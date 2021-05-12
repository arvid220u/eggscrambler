package anonbcast

import (
	crand "crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"

	"github.com/arvid220u/6.824-project/network"
	"github.com/davecgh/go-spew/spew"
	"github.com/google/uuid"
)

// inspiration: https://stackoverflow.com/a/54491783
func init() {
	var b [8]byte
	_, err := crand.Read(b[:])
	if err != nil {
		assertf(false, "cannot seed math/rand, err: %v", err)
	}
	rand.Seed(int64(binary.LittleEndian.Uint64(b[:])))
}

// Client performs the anonymous broadcasting protocol, and exposes the bare minimum of
// communication necessary for an application to broadcast anonymous messages. A Client
// interacts with a local Server instance to get information, and a possibly remote Server
// instance to write information.
type Client struct {
	// Id is the id of this client. Each client has its own unique id.
	// Immutable.
	Id uuid.UUID

	// Id of the underlying state machine server.
	// Used for the client to make configuration changes.
	serverId int

	// dead indicates whether the client is alive. Set by Kill()
	dead int32
	mu   sync.Mutex

	// m is an object that provides a message to send on a given round.
	m Messager

	// lastUpdate stores the last update received on the state machine
	lastUpdate *lastStateMachine

	updCh <-chan UpdateMsg

	// Indicates whether the underlying raft server
	// is in the configuration or not
	active bool
	//Indicates whether client is actively trying to leave configuration
	leaving            bool
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
	Messages [][]byte
}

type Messager interface {
	// Message returns the message for this participant in the given round.
	// It may block. If it takes too much time, its return value may be ignored.
	// (i.e., the protocol has timed out and moved on to the next round)
	Message(round int) []byte
}

// submitOp submits the op to the leader, returning an error if the op is not successfully committed to the
// leader's log. It does not retry on failure.
func (c *Client) submitOp(op Op) error {
	var reply OpRpcReply
	c.mu.Lock()
	ok := c.cp.Call(c.currConf[c.lastKnownLeaderInd], "Server.SubmitOp", &op, &reply)
	c.mu.Unlock()
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
	var messages [][]byte
	for _, m := range ri.Messages {
		om := m
		for _, revealKey := range ri.RevealedKeys {
			om = ri.Crypto.Decrypt(revealKey, om)
		}
		messages = append(messages, ri.Crypto.ExtractMsg(om))
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
		op := JoinOp{
			Id: c.Id,
			R:  round,
		}
		c.pending.add(op)
	}
}

// submit is a helper method to readUpdates
func (c *Client) submit(round int, ri *RoundInfo, me int, encryptKey PrivateKey) {
	if ri.Messages[me].Nil() {
		// TODO: time out here! if Message does not return within X seconds, then use a dummy message
		msg := c.m.Message(round)
		m, err := ri.Crypto.PrepareMsg(msg)
		if err != nil {
			c.logf("message size too long, using dummy message, %v", err)
			m, err = ri.Crypto.PrepareMsg([]byte("dummy"))
			c.assertf(err == nil, "dummy cannot be too long, err: %v", err)
		}
		encryptedM := ri.Crypto.Encrypt(encryptKey, m)
		op := MessageOp{
			Id:      c.Id,
			R:       round,
			Message: encryptedM,
		}
		c.pending.add(op)
	}
}

// encrypt is a helper method to readUpdates
func (c *Client) encrypt(round int, ri *RoundInfo, me int, encryptKey PrivateKey) {
	if !ri.Encrypted[me] {
		prev := 0
		for _, encrypted := range ri.Encrypted {
			if encrypted {
				prev++
			}
		}
		var encryptedMessages []Msg
		for i, m := range ri.Messages {
			if i == me {
				encryptedMessages = append(encryptedMessages, m.DeepCopy())
			} else {
				encryptedMessages = append(encryptedMessages, ri.Crypto.Encrypt(encryptKey, m))
			}
		}
		op := EncryptedOp{
			Id:       c.Id,
			R:        round,
			Messages: encryptedMessages,
			Prev:     prev,
		}
		c.pending.add(op)
	}
}

// scramble is a helper method to readUpdates
func (c *Client) scramble(round int, ri *RoundInfo, me int, scrambleKey PrivateKey, revealKey PrivateKey) {
	if !ri.Scrambled[me] {
		prev := 0
		for _, scrambled := range ri.Scrambled {
			if scrambled {
				prev++
			}
		}
		var scrambledMessages []Msg
		// first encrypt each message once with each key
		for _, m := range ri.Messages {
			m1 := ri.Crypto.Encrypt(scrambleKey, m)
			m2 := ri.Crypto.Encrypt(revealKey, m1)
			scrambledMessages = append(scrambledMessages, m2)
		}
		// now scramble!
		rand.Shuffle(len(scrambledMessages), func(i, j int) {
			scrambledMessages[i], scrambledMessages[j] = scrambledMessages[j], scrambledMessages[i]
		})
		op := ScrambledOp{
			Id:       c.Id,
			R:        round,
			Messages: scrambledMessages,
			Prev:     prev,
		}
		c.pending.add(op)
	}
}

// decrypt is a helper method to readUpdates
func (c *Client) decrypt(round int, ri *RoundInfo, me int, scrambleKey PrivateKey, encryptKey PrivateKey) {
	if !ri.Decrypted[me] {
		prev := 0
		for _, decrypted := range ri.Decrypted {
			if decrypted {
				prev++
			}
		}
		// decrypt each message once with each key
		var decryptedMessages []Msg
		for _, m := range ri.Messages {
			m1 := ri.Crypto.Decrypt(scrambleKey, m)
			m2 := ri.Crypto.Decrypt(encryptKey, m1)
			decryptedMessages = append(decryptedMessages, m2)
		}
		op := DecryptedOp{
			Id:       c.Id,
			R:        round,
			Messages: decryptedMessages,
			Prev:     prev,
		}
		c.pending.add(op)
	}
}

// reveal is a helper method to readUpdates
func (c *Client) reveal(round int, ri *RoundInfo, me int, revealKey PrivateKey) {
	if ri.RevealedKeys[me].Nil() {
		op := RevealOp{
			Id:        c.Id,
			R:         round,
			RevealKey: revealKey,
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
	var encryptKey PrivateKey
	var scrambleKey PrivateKey
	var revealKey PrivateKey
	lastKeyGen := -1

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

				//only join round if we're in the raft configuration and not actively trying to leave it
				if c.active && !c.leaving {
					go c.prepare(round, ri, me)
				} else {
					continue
				}
			case SubmitPhase:
				if !c.active || me == -1 {
					// we don't have any business in this round
					continue
				}
				if round > lastKeyGen {
					assertf(lastKeyGen == round-1, "required true because updates are in order")
					// TODO: verify that ri.Crypto is a sane generator. We may have gotten it from a malicious user!!
					// 	so verify that the prime is big enough
					encryptKey = ri.Crypto.GenKey()
					scrambleKey = ri.Crypto.GenKey()
					revealKey = ri.Crypto.GenKey()
					lastKeyGen = round
				}
				// we want to make sure that we only ask the user for a message once per round
				if round > lastMessageRound {
					go c.submit(round, ri, me, encryptKey.DeepCopy())
					lastMessageRound = round
				}
			case EncryptPhase:
				if !c.active || me == -1 {
					// we don't have any business in this round
					continue
				}
				go c.encrypt(round, ri, me, encryptKey.DeepCopy())
			case ScramblePhase:
				if !c.active || me == -1 {
					// we don't have any business in this round
					continue
				}
				go c.scramble(round, ri, me, scrambleKey.DeepCopy(), revealKey.DeepCopy())
			case DecryptPhase:
				if !c.active || me == -1 {
					// we don't have any business in this round
					continue
				}
				go c.decrypt(round, ri, me, scrambleKey.DeepCopy(), encryptKey.DeepCopy())
			case RevealPhase:
				if !c.active || me == -1 {
					// we don't have any business in this round
					continue
				}
				go c.reveal(round, ri, me, revealKey.DeepCopy())
			}
		} else if updMsg.ConfigurationValid {
			c.logf("Received configuration update: %v", updMsg.Configuration)
			c.mu.Lock()
			if _, ok := updMsg.Configuration[c.serverId]; ok {
				// we're in the configuration
				c.active = true
			} else {
				// we're not in the configuration
				c.active = false
				c.leaving = false
			}

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
	// TODO: submit an abort operation here if the last state machine includes this user
	// 	and the round isn't done or failed
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
		Id:     c.Id,
		R:      round,
		Crypto: newFakeCommutativeCrypto(), // TODO: get real commutative crypto here
	}
	go c.pending.add(op)
	return nil
}

func NewClient(s *Server, m Messager, cp network.ConnectionProvider) *Client {
	c := new(Client)
	c.Id = uuid.New()
	c.m = m
	c.pending = newEliminationQueue()
	c.lastUpdate = newLastStateMachine()
	c.cp = cp
	c.currConf = make([]int, 0)
	c.updCh = s.GetUpdCh()
	c.serverId = s.Me
	c.active = true // TODO set this to false on init after testing.
	c.leaving = false

	// Assume the configuration has all possible servers, until we get notified otherwise
	peers := cp.NumPeers()
	for i := 0; i < peers; i++ {
		c.currConf = append(c.currConf, i)
	}

	// TODO uncomment this after testing
	/* c.setActive()
	if !c.active {
		c.BecomeActive()
	} */

	go c.readUpdates()
	go c.submitOps()

	return c
}
