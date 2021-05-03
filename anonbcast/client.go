package anonbcast

import (
	"errors"
	"fmt"
	"github.com/arvid220u/6.824-project/labrpc"
	"github.com/davecgh/go-spew/spew"
	"github.com/google/uuid"
)

// Client performs the anonymous broadcasting protocol, and exposes the bare minimum of
// communication necessary for an application to broadcast anonymous messages. A Client
// interacts with a local Server instance to get information, and a possibly remote Server
// instance to write information.
type Client struct {
	// Id is the id of this client. Each client has its own unique id.
	Id uuid.UUID

	// m is an object that provides a message to send on a given round.
	m Messager

	updCh <-chan StateMachine

	// TODO: this leader needs to be updated whenever the leader fails
	// 	think about how to deal with leader failures
	leader *labrpc.ClientEnd
}

type Messager interface {
	// Message returns the message for this participant in the given round.
	// It may block. If it takes too much time, its return value may be ignored.
	// (i.e., the protocol has timed out and moved on to the next round)
	Message(round int) string
}

// Start indicates the intent of the user to start the round.
func (c *Client) Start(round int) {
	op := StartOp{
		Id:    c.Id,
		Round: round,
	}
	err := c.submitOp(op)
	assertf(err == nil, "idk: %v", err) // TODO: handle this error!
}

func (c *Client) submitOp(op Op) error {
	var reply RpcReply
	ok := c.leader.Call("Server.SubmitOp", &op, &reply)
	if !ok {
		panic("idk what to do here. is this because of timeout?") // TODO: figure out what to do here! retry?
		return errors.New("idk")
	}
	if reply.Err == ErrWrongLeader {
		panic("update leader here and then retry?") // TODO: retry with new leader here
		return errors.New("wrong leader")
	}
	return nil
}

func (c *Client) readUpdates() {
	// TODO: none of this is thought through
	// 	there's probably a ton of problems here in terms of consistency
	for { // TODO: add killed() ?
		sm := <-c.updCh
		c.logf("received state machine update! %+v", sm)
		round := sm.Round
		ri := sm.CurrentRoundInfo()
		me := -1
		for i, p := range ri.Participants {
			if p == c.Id {
				me = i
			}
		}
		switch ri.Phase {
		case PreparePhase:
			// TODO: handle Done and Failed phases of previous rounds maybe?
			// if we are not in the participants list, we wanna submit ourselves!
			if me == -1 {
				op := PublicKeyOp{
					Id:        c.Id,
					Round:     round,
					PublicKey: "public key here!", // TODO: generate a new public key here, and probably all other keys
				}
				err := c.submitOp(op)
				c.assertf(err == nil, "err: %v", err) // TODO: handle this error
			}
		case EncryptPhase:
			// TODO: implement timeout here??
			msg := c.m.Message(round) // TODO: encrypt!
			op := MessageOp{
				Id:      c.Id,
				Round:   round,
				Message: Msg(msg),
			}
			err := c.submitOp(op)
			c.assertf(err == nil, "err: %v", err) // TODO: handle this error
		case ScramblePhase:
			// TODO: timeout?
			prev := 0
			for _, scrambled := range ri.Scrambled {
				if scrambled {
					prev++
				}
			}
			op := ScrambledOp{
				Id:       c.Id,
				Round:    round,
				Messages: ri.Messages, // TODO: scramble and encrypt!
				Prev:     prev,
			}
			err := c.submitOp(op)
			c.assertf(err == nil, "err: %v", err) // TODO: handle this error.
		case DecryptPhase:
			// TODO: timeout?
			prev := 0
			for _, decrypted := range ri.Decrypted {
				if decrypted {
					prev++
				}
			}
			op := DecryptedOp{
				Id:       c.Id,
				Round:    round,
				Messages: ri.Messages, // TODO: decrypt!
				Prev:     prev,
			}
			err := c.submitOp(op)
			c.assertf(err == nil, "err: %v", err) // TODO: handle this error.
		case RevealPhase:
			op := RevealOp{
				Id:        c.Id,
				Round:     round,
				RevealKey: "reveal key here lol", // TODO: add reveal key
			}
			err := c.submitOp(op)
			c.assertf(err == nil, "err: %v", err) // TODO: handle this error.
		}
	}
}

func (c *Client) logf(format string, a ...interface{}) {
	logHeader := fmt.Sprintf("[client %s] ", c.Id.String())
	DPrintf(logHeader+format, a...)
}

func (c *Client) assertf(condition bool, format string, a ...interface{}) {
	logHeader := fmt.Sprintf("[client %s] ", c.Id.String())
	dump := ""
	if EnableDump {
		dump = "\n\n" + spew.Sdump(c)
	}
	assertf(condition, logHeader+format+dump, a...)
}
func (c *Client) dump() {
	if Debug && EnableDump {
		// TODO: lock here?
		c.logf(spew.Sdump(c))
	}
}

func NewClient(s *Server, m Messager, leader *labrpc.ClientEnd) *Client {
	c := new(Client)
	c.Id = uuid.New()
	c.updCh = s.GetUpdCh()
	c.m = m
	c.leader = leader

	go c.readUpdates()

	return c
}
