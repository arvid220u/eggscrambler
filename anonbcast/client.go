package anonbcast

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/arvid220u/6.824-project/network"
	"github.com/davecgh/go-spew/spew"
	"github.com/google/uuid"
)

// Client performs the anonymous broadcasting protocol, and exposes the bare minimum of
// communication necessary for an application to broadcast anonymous messages. A Client
// interacts with a local Server instance to get information, and a possibly remote Server
// instance to write information.
type Client struct {
	// Id is the id of this client. Each client has its own unique id.
	// Immutable.
	Id uuid.UUID

	// dead indicates whether the client is alive. Set by Kill()
	dead int32
	mu   sync.Mutex

	// m is an object that provides a message to send on a given round.
	m Messager

	// lastUpdate stores the last update received on the state machine
	lastUpdate *lastStateMachine

	updCh <-chan UpdateMsg

	cp                 network.ConnectionProvider
	currConf           []int
	lastKnownLeaderInd int

	resCh chan RoundResult

	// pending stores the operations that are pending submission to the leader
	pending *eliminationQueue
}

// RoundResult represents the final outcome of a round.
type RoundResult struct {
	// Round is the round we're looking at.
	Round int
	// Succeeded is true if the round resulted in the DonePhase
	// and false if in FailedPhase.
	Succeeded bool
	// Messages contains all the plaintext messages from this round.
	Messages []string
}

type Messager interface {
	// Message returns the message for this participant in the given round.
	// It may block. If it takes too much time, its return value may be ignored.
	// (i.e., the protocol has timed out and moved on to the next round)
	Message(round int) string
}

// submitOp submits the op to the leader, returning an error if the op is not successfully committed to the
// leader's log. It does not retry on failure.
func (c *Client) submitOp(op Op) error {
	var reply RpcReply
	ok := c.cp.Call(c.currConf[c.lastKnownLeaderInd], "Server.SubmitOp", &op, &reply)
	if !ok {
		return errors.New("No response")
	}
	if reply.Err != OK {
		return errors.New(string(reply.Err))
	}
	return nil
}

// sendRoundResult is a helper method to readUpdates
func (c *Client) sendRoundResult(sm StateMachine, round int) {
	ri, err := sm.GetRoundInfo(round - 1)
	if err != nil {
		assertf(false, "must be able to get previous round, error is %v", err)
	}
	assertf(ri.Phase == DonePhase || ri.Phase == FailedPhase, "must either fail or done")
	var messages []string
	for _, m := range ri.Messages {
		messages = append(messages, string(m)) // TODO: do a ton of decryption here using reveal keys
	}
	result := RoundResult{
		Round:     round - 1,
		Succeeded: ri.Phase == DonePhase,
		Messages:  messages,
	}
	if c.resCh != nil {
		c.resCh <- result
	}
}

// prepare is a helper method to readUpdates
func (c *Client) prepare(round int, ri *RoundInfo, me int) {
	if me == -1 {
		op := PublicKeyOp{
			Id:        c.Id,
			R:         round,
			PublicKey: "public key here!", // TODO: generate a new public key here, and probably all other keys
		}
		c.pending.add(op)
	}
}

// encrypt is a helper method to readUpdates
func (c *Client) encrypt(round int, ri *RoundInfo, me int) {
	if ri.Messages[me] == Msg("") { // TODO: update with real nil type for Msg
		// TODO: time out here! if Message does not return within X seconds, then use a dummy message
		msg := c.m.Message(round) // TODO: encrypt!
		op := MessageOp{
			Id:      c.Id,
			R:       round,
			Message: Msg(msg),
		}
		c.pending.add(op)
	}
}

// scramble is a helper method to readUpdates
func (c *Client) scramble(round int, ri *RoundInfo, me int) {
	if !ri.Scrambled[me] {
		prev := 0
		for _, scrambled := range ri.Scrambled {
			if scrambled {
				prev++
			}
		}
		op := ScrambledOp{
			Id:       c.Id,
			R:        round,
			Messages: ri.Messages, // TODO: scramble and encrypt!
			Prev:     prev,
		}
		c.pending.add(op)
	}
}

// decrypt is a helper method to readUpdates
func (c *Client) decrypt(round int, ri *RoundInfo, me int) {
	if !ri.Decrypted[me] {
		prev := 0
		for _, decrypted := range ri.Decrypted {
			if decrypted {
				prev++
			}
		}
		op := DecryptedOp{
			Id:       c.Id,
			R:        round,
			Messages: ri.Messages, // TODO: decrypt!
			Prev:     prev,
		}
		c.pending.add(op)
	}
}

// reveal is a helper method to readUpdates
func (c *Client) reveal(round int, ri *RoundInfo, me int) {
	if ri.RevealedKeys[me] == "" { // TODO: check actual nil type here
		op := RevealOp{
			Id:        c.Id,
			R:         round,
			RevealKey: "reveal key here lol", // TODO: add reveal key
		}
		c.pending.add(op)
	}
}

// readUpdates is a long-running goroutine that reads from the updCh and takes
// action to follow the protocol.
func (c *Client) readUpdates() {
	lastResChSend := -1
	lastMessageRound := -1
	version := 0

	// in this loop, all heavy work is done in goroutines. this is because the server always expects
	// to be able to send on the updCh.
	for !c.killed() {
		updMsg := <-c.updCh

		if updMsg.StateMachineValid {
			sm := updMsg.StateMachine
			c.logf("received state machine update! %+v", sm)

			go c.lastUpdate.set(sm, version)
			version++

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
				if round-1 > lastResChSend {
					// send the result of the previous round!
					assertf(lastResChSend == round-2, "required to be true because updates are in order")
					go c.sendRoundResult(sm, round)
					lastResChSend = round - 1
				}
				go c.prepare(round, ri, me)
			case EncryptPhase:
				if me == -1 {
					// we don't have any business in this round
					continue
				}
				// we want to make sure that we only ask the user for a message once per round
				if round > lastMessageRound {
					go c.encrypt(round, ri, me)
					lastMessageRound = round
				}
			case ScramblePhase:
				if me == -1 {
					// we don't have any business in this round
					continue
				}
				go c.scramble(round, ri, me)
			case DecryptPhase:
				if me == -1 {
					// we don't have any business in this round
					continue
				}
				go c.decrypt(round, ri, me)
			case RevealPhase:
				if me == -1 {
					// we don't have any business in this round
					continue
				}
				go c.reveal(round, ri, me)
			}
		} else if updMsg.ConfigurationValid {
			c.logf("Received configuration update: %v", updMsg.Configuration)
			c.mu.Lock()
			c.currConf = mapToSlice(updMsg.Configuration)
			assertf(len(c.currConf) > 0, "Expected configuration with at least 1 server, but none found: %v", updMsg.Configuration)
			c.lastKnownLeaderInd = 0 // reset this to avoid OOB errors
			c.mu.Unlock()
		}

	}
}

// submitOps is a long-running goroutine that gets pending ops and submits them to the leader
func (c *Client) submitOps() {
	for !c.killed() {
		op := c.pending.get()

		// TODO maybe a switch case to handle other types of errors?
		if err := c.submitOp(op); err != nil {
			c.updateLeader()
		} else {
			// op is now done, so remove it from the pending ops if it is there
			c.pending.finish(op)
		}
	}
}

// Cycles through the servers in the known configuration
func (c *Client) updateLeader() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastKnownLeaderInd = (c.lastKnownLeaderInd + 1) % len(c.currConf)
}

// Kill kills all long-running goroutines and releases any memory
// used by the Client instance. After calling Kill no other methods
// may be called.
func (c *Client) Kill() {
	atomic.StoreInt32(&c.dead, 1)
}
func (c *Client) killed() bool {
	z := atomic.LoadInt32(&c.dead)
	return z == 1
}

func (c *Client) logf(format string, a ...interface{}) {
	logHeader := fmt.Sprintf("[client %s] ", c.Id.String())
	DPrintf(logHeader+format, a...)
}

func (c *Client) assertf(condition bool, format string, a ...interface{}) {
	logHeader := fmt.Sprintf("[client %s] ", c.Id.String())
	dump := ""
	if IsDump() {
		dump = "\n\n" + spew.Sdump(c)
	}
	assertf(condition, logHeader+format+dump, a...)
}
func (c *Client) dump() {
	if IsDebug() && IsDump() {
		// ideally we would lock before we dump to avoid races, but we don't have a global lock so we can't :(
		c.logf(spew.Sdump(c))
	}
}

func mapToSlice(mp map[int]bool) []int {
	sl := make([]int, 0)
	for k := range mp {
		sl = append(sl, k)
	}

	return sl
}

// GetResCh returns a channel on which the results of rounds are sent.
// sent. Each round result will be sent exactly once, but it is not
// guaranteed that the results will be sent in order, if, for example,
// the user application does not continuously read from the channel.
//
// This method may only be called once.
func (c *Client) GetResCh() <-chan RoundResult {
	c.resCh = make(chan RoundResult)
	return c.resCh
}

// GetLastStateMachine returns the last known version of the state machine.
func (c *Client) GetLastStateMachine() StateMachine {
	return c.lastUpdate.get()
}

// Start indicates the intent of the user to start the round.
func (c *Client) Start(round int) error {
	sm := c.GetLastStateMachine()
	if sm.Round != round {
		return errors.New("can only start the current round")
	}
	me := false
	for _, p := range sm.CurrentRoundInfo().Participants {
		if p == c.Id {
			me = true
		}
	}
	if !me {
		return errors.New("can only start a round after submitting the public key. please wait")
	}
	op := StartOp{
		Id: c.Id,
		R:  round,
	}
	go c.pending.add(op)
	return nil
}

// TODO remove Server from this argument list, but will require a discussion
// about how to replace the updCh, since we can't use channels over
// labrpc or the real network.
func NewClient(s *Server, m Messager, cp network.ConnectionProvider) *Client {
	c := new(Client)
	c.Id = uuid.New()
	c.m = m
	c.pending = newEliminationQueue()
	c.lastUpdate = newLastStateMachine()
	c.cp = cp
	c.currConf = make([]int, 0)
	c.updCh = s.GetUpdCh()

	// Assume the configuration has all possible servers, until we get notified otherwise
	peers := cp.NumPeers()
	for i := 0; i < peers; i++ {
		c.currConf = append(c.currConf, i)
	}

	go c.readUpdates()
	go c.submitOps()

	return c
}
