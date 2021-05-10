package anonbcast

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/arvid220u/6.824-project/labgob"
	"github.com/arvid220u/6.824-project/network"
	"github.com/arvid220u/6.824-project/raft"
	"github.com/davecgh/go-spew/spew"
)

type UpdateMsg struct {
	ConfigurationValid bool
	Configuration      map[int]bool

	StateMachineValid bool
	StateMachine      StateMachine
}

// Server implements the shared state machine on top of Raft, that is used
// for performing the anonymous broadcasting protocol. It exposes RPCs that
// can be called by a Client on a different machine, and exposes a channel
// that can be consumed by a local Client for reading the latest state.
type Server struct {
	rf Raft

	dead int32 // set by Kill(), not protected by mu

	mu sync.Mutex
	sm StateMachine // protected by mu

	// applyCh is for communication from raft to the server
	applyCh <-chan raft.ApplyMsg
	// updChs stores all channels on which state machine updates should be sent.
	// Each update should be sent to each channel.
	updChs []chan UpdateMsg // slice protected by mu

	// resps stores one channel for every pending request, indexed by Raft term and log index
	resps RespChannels // protected by mu
}

func NewServer(rf Raft) *Server {
	labgob.Register(fakeCommutativeCrypto{}) // TODO: remove the fake crypto
	labgob.Register(JoinOp{})
	labgob.Register(StartOp{})
	labgob.Register(MessageOp{})
	labgob.Register(EncryptedOp{})
	labgob.Register(ScrambledOp{})
	labgob.Register(DecryptedOp{})
	labgob.Register(RevealOp{})
	labgob.Register(AbortOp{})

	s := new(Server)
	s.rf = rf
	s.sm = NewStateMachine(nil)
	s.applyCh = rf.GetApplyCh()
	s.resps = NewRespChannels()

	go s.applier()

	return s
}

// Creates and starts new anonbcast server using a real raft instance
func MakeServer(cp network.ConnectionProvider, me int, initialCfg map[int]bool, persister *raft.Persister, maxraftstate int) *Server {
	applyCh := make(chan raft.ApplyMsg, 1)
	rf := raft.Make(cp, me, initialCfg, persister, applyCh, false)
	return NewServer(rf)
}

type Err string

const (
	OK             = "OK"
	ErrWrongLeader = "ErrWrongLeader"
)

type RpcReply struct {
	Err Err
}

func (s *Server) IsLeader(args interface{}, reply *RpcReply) {
	if _, isLeader := s.rf.GetState(); isLeader {
		reply.Err = OK
	} else {
		reply.Err = ErrWrongLeader
	}
}

// SubmitOp is an RPC for clients to submit operations to the state machine.
// Returns OK if this operation shouldn't be retried (because it has been committed or
// because it is a no-op), and ErrWrongLeader if the operation should be submitted
// to another server that might be the leader.
func (s *Server) SubmitOp(args *Op, reply *RpcReply) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logf("SubmitOp called with args: %v", *args)

	if s.sm.GuaranteedNoEffect(*args) {
		// even if this server is not the leader, there is no need to retry
		reply.Err = OK
		return
	}

	index, term, ok := s.rf.Start(*args)

	s.logf("started consensus. index %d, term %d, ok %v", index, term, ok)

	if !ok {
		reply.Err = ErrWrongLeader
		return
	}

	ch := s.resps.Create(term, index)

	s.mu.Unlock()
	resp := <-ch
	s.mu.Lock()

	if !resp {
		reply.Err = ErrWrongLeader
		return
	}

	reply.Err = OK
}

func (s *Server) applier() {
	for !s.killed() {
		msg := <-s.applyCh

		s.logf("received msg %v from raft!", msg)

		s.mu.Lock()

		if msg.CommandValid {
			op := msg.Command.(Op)

			updated := s.sm.Apply(op)
			if updated {
				s.sendStateMachineUpdate()
			}

			ch := s.resps.Get(msg.CommandTerm, msg.CommandIndex)
			if ch != nil {
				s.mu.Unlock()
				ch <- true // we applied it successfully!
				s.mu.Lock()
			}
		} else if msg.RaftUpdateValid {
			// cancel all requests if we're not the leader anymore
			// this may mean that we cancel requests that could've succeeded
			// but that is fine — the client will retry with the real leader
			if !msg.IsLeader {
				s.resps.Clear()
			}
		} else if msg.SnapshotValid {
			s.assertf(false, "snapshots not implemented yet")
		} else if msg.ConfigValid {
			s.sendConfigurationUpdate(msg.Configuration)
		} else {
			s.assertf(false, "a message has to be either a command, an update or a snapshot!")
		}

		s.mu.Unlock()
	}
}

// Kill kills all long-running goroutines and releases any memory
// used by the Server instance. After calling Kill no other methods
// may be called.
func (s *Server) Kill() {
	atomic.StoreInt32(&s.dead, 1)
	s.rf.Kill()
}
func (s *Server) killed() bool {
	z := atomic.LoadInt32(&s.dead)
	return z == 1
}

func (s *Server) logf(format string, a ...interface{}) {
	logHeader := fmt.Sprintf("[server ?] ")
	DPrintf(logHeader+format, a...)
}

func (s *Server) assertf(condition bool, format string, a ...interface{}) {
	logHeader := fmt.Sprintf("[server ?] ")
	dump := ""
	if IsDump() {
		dump = "\n\n" + spew.Sdump(s)
	}
	assertf(condition, logHeader+format+dump, a...)
}
func (s *Server) dump() {
	if IsDebug() && IsDump() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.logf(spew.Sdump(s))
	}
}

// TODO: add some RPCs for configuration changes?

// GetUpdCh returns a channel on which the server will send a copy of the state machine
// every time it updates. This method can be called multiple times; in that case, it will
// return distinct channels, each of which will receive all updates. At the time this method
// is called, the current state of the state machine will be immediately sent on the channel.
// All updates are guaranteed to come, and they are guaranteed to come in order.
//
// The client MUST almost always be reading from the channel. It may never block for an extended
// amount of time not reading on the returned channel.
func (s *Server) GetUpdCh() <-chan UpdateMsg {
	s.mu.Lock()
	defer s.mu.Unlock()

	// make it buffered with buffer 1 so that we can send the first state machine right here
	ch := make(chan UpdateMsg, 1)
	s.updChs = append(s.updChs, ch)

	ch <- UpdateMsg{StateMachineValid: true, StateMachine: s.sm.DeepCopy()}

	return ch
}

//Assumes lock on
// s.mu is HELD.
func (s *Server) sendStateMachineUpdate() {
	for _, ch := range s.updChs {
		ch <- UpdateMsg{StateMachineValid: true, StateMachine: s.sm.DeepCopy()}
	}
}

// Assumes lock on s.mu is HELD.
func (s *Server) sendConfigurationUpdate(conf map[int]bool) {
	for _, ch := range s.updChs {
		ch <- UpdateMsg{ConfigurationValid: true, Configuration: conf}
	}
}
