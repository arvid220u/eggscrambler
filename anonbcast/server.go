package anonbcast

import (
	"context"
	"fmt"
	"github.com/arvid220u/eggscrambler/libraft"
	"sync"
	"sync/atomic"
	"time"

	"github.com/arvid220u/eggscrambler/labgob"
	"github.com/arvid220u/eggscrambler/network"
	"github.com/davecgh/go-spew/spew"
)

type UpdateMsg struct {
	ConfigurationValid bool
	Configuration      map[string]bool

	StateMachineValid bool
	StateMachine      StateMachine
}

// Server implements the shared state machine on top of Raft, that is used
// for performing the anonymous broadcasting protocol. It exposes RPCs that
// can be called by a Client on a different machine, and exposes a channel
// that can be consumed by a local Client for reading the latest state.
type Server struct {
	Me string
	rf libraft.Raft

	dead int32 // set by Kill(), not protected by mu

	mu sync.Mutex
	sm StateMachine // protected by mu

	// applyCh is for communication from raft to the server
	applyCh <-chan libraft.ApplyMsg
	// updChs stores all channels on which state machine updates should be sent.
	// Each update should be sent to each channel.
	updChs     map[int]chan UpdateMsg // map protected by mu
	updChIndex int                    // protected by mu

	// resps stores one channel for every pending request, indexed by Raft term and log index
	resps RespChannels // protected by mu
}

func NewServer(me string, rf libraft.Raft) *Server {
	labgob.Register(masseyOmuraCrypto{})
	labgob.Register(JoinOp{})
	labgob.Register(StartOp{})
	labgob.Register(MessageOp{})
	labgob.Register(EncryptedOp{})
	labgob.Register(ScrambledOp{})
	labgob.Register(DecryptedOp{})
	labgob.Register(RevealOp{})
	labgob.Register(AbortOp{})

	s := new(Server)
	s.Me = me
	s.rf = rf
	s.sm = NewStateMachine(nil)
	s.applyCh = rf.GetApplyCh()
	s.resps = NewRespChannels()
	s.updChs = make(map[int]chan UpdateMsg)
	s.updChIndex = 0

	go s.applier()

	return s
}

// Creates and starts new anonbcast server using a real raft instance
func MakeServer(cp network.ConnectionProvider, initialCfg map[string]bool, persister *libraft.Persister, maxraftstate int) (*Server, libraft.Raft) {
	applyCh := make(chan libraft.ApplyMsg, 1)
	// TODO: is it ok if initialCfg only contains part of some configuration (I think so???)
	rf := makeRaft(cp, initialCfg, persister, applyCh, false)
	return NewServer(cp.Me(), rf), rf
}

type Err string

const (
	OK             = "OK"
	ErrWrongLeader = "ErrWrongLeader"
)

type OpRpcReply struct {
	Err Err
}

func (s *Server) IsLeader(args interface{}, reply *OpRpcReply) {
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
func (s *Server) SubmitOp(ctx context.Context, args *Op, reply *OpRpcReply) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logf(dInfo, "SubmitOp called with args: %v", *args)

	if s.sm.GuaranteedNoEffect(*args) {
		// even if this server is not the leader, there is no need to retry
		reply.Err = OK
		return nil
	}

	index, term, ok := s.rf.Start(*args)

	s.logf(dInfo, "started consensus. index %d, term %d, ok %v", index, term, ok)

	if !ok {
		reply.Err = ErrWrongLeader
		return nil
	}

	ch := s.resps.Create(term, index)

	s.mu.Unlock()
	resp := false
	brk := false
	for !s.killed() && !brk {
		select {
		case resp = <-ch:
			brk = true
		case <-time.NewTimer(time.Second).C:
		}
	}
	s.mu.Lock()

	if !resp {
		reply.Err = ErrWrongLeader
		return nil
	}

	reply.Err = OK
	return nil
}

func (s *Server) applier() {
	for !s.killed() {
		msg := <-s.applyCh

		s.logf(dInfo, "received msg %v from raft!", msg)

		s.mu.Lock()

		if msg.CommandValid {
			op := msg.Command.(Op)

			s.logf(dInfo, "applying operation %+v to the state machine", op)
			updated := s.sm.Apply(op)
			s.logf(dInfo, "state machine is now: %+v", s.sm)
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

func (s *Server) logHeader() string {
	return fmt.Sprintf("server %v", s.Me)
}

func (s *Server) logf(topic logTopic, format string, a ...interface{}) {
	logf(topic, s.logHeader(), format, a...)
}

func (s *Server) assertf(condition bool, format string, a ...interface{}) {
	s.dump()
	assertf(condition, s.logHeader(), format, a...)
}
func (s *Server) dump() {
	if IsDebug() && IsDump() {
		// this still has race conditions because we use atomic int for killed
		s.mu.Lock()
		defer s.mu.Unlock()
		s.logf(dDump, spew.Sdump(s))
	}
}

// GetUpdCh returns a channel on which the server will send a copy of the state machine
// every time it updates. This method can be called multiple times; in that case, it will
// return distinct channels, each of which will receive all updates. At the time this method
// is called, the current state of the state machine will be immediately sent on the channel.
// All updates are guaranteed to come, and they are guaranteed to come in order.
//
// The client MUST almost always be reading from the channel. It may never block for an extended
// amount of time not reading on the returned channel. Before the client stops reading from the channel,
// it must call CloseUpdCh.
func (s *Server) GetUpdCh() (<-chan UpdateMsg, int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// make it buffered with buffer 1 so that we can send the first state machine right here
	ch := make(chan UpdateMsg, 1)
	index := s.updChIndex
	s.updChs[index] = ch
	s.updChIndex++

	ch <- UpdateMsg{StateMachineValid: true, StateMachine: s.sm.DeepCopy()}

	return ch, index
}
func (s *Server) CloseUpdCh(index int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	close(s.updChs[index])
	delete(s.updChs, index)
}

//Assumes lock on
// s.mu is HELD.
func (s *Server) sendStateMachineUpdate() {
	for _, ch := range s.updChs {
		ch <- UpdateMsg{StateMachineValid: true, StateMachine: s.sm.DeepCopy()}
	}
}

// Assumes lock on s.mu is HELD.
func (s *Server) sendConfigurationUpdate(conf map[string]bool) {
	for _, ch := range s.updChs {
		ch <- UpdateMsg{ConfigurationValid: true, Configuration: conf}
	}
}
