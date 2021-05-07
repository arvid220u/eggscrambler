package anonbcast

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/arvid220u/6.824-project/labgob"
	"github.com/arvid220u/6.824-project/raft"
	"github.com/davecgh/go-spew/spew"
)

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
	updChs []chan StateMachine // slice protected by mu

	// resps stores one channel for every pending request, indexed by Raft term and log index
	resps RespChannels // protected by mu
}

func NewServer(rf Raft) *Server {
	labgob.Register(PublicKeyOp{})
	labgob.Register(StartOp{})
	labgob.Register(MessageOp{})
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

type Err string

const (
	OK             = "OK"
	ErrWrongLeader = "ErrWrongLeader"
)

type RpcReply struct {
	Err Err
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
	for s.killed() == false {
		msg := <-s.applyCh

		s.logf("received msg %v from raft!", msg)

		s.mu.Lock()

		if msg.CommandValid {
			op := msg.Command.(Op)

			updated := s.sm.Apply(op)
			if updated {
				s.sendUpdate()
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
// Updates are NOT guaranteed to come in order.  TODO: does it make it easier to implement the client if updates are guaranteed in order?
func (s *Server) GetUpdCh() <-chan StateMachine {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch := make(chan StateMachine)
	s.updChs = append(s.updChs, ch)

	go func(ch chan StateMachine, sm StateMachine) {
		ch <- sm
	}(ch, s.sm.DeepCopy())

	return ch
}

// sendUpdate sends updates to all updCh. Assumes lock on
// s.mu is HELD.
func (s *Server) sendUpdate() {
	for _, ch := range s.updChs {
		go func(ch chan StateMachine, sm StateMachine) {
			ch <- sm
		}(ch, s.sm.DeepCopy())
	}
}
