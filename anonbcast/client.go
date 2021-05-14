package anonbcast

import (
	crand "crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/arvid220u/6.824-project/network"
	"github.com/davecgh/go-spew/spew"
	"github.com/google/uuid"
)

// inspiration: https://stackoverflow.com/a/54491783
func init() {
	var b [8]byte
	_, err := crand.Read(b[:])
	if err != nil {
		assertf(false, "init package", "cannot seed math/rand, err: %v", err)
	}
	rand.Seed(int64(binary.LittleEndian.Uint64(b[:])))

	log.SetFlags(log.Lmicroseconds)
}

// Client performs the anonymous broadcasting protocol, and exposes the bare minimum of
// communication necessary for an application to broadcast anonymous messages. A Client
// interacts with a local Server instance to get information, and a possibly remote Server
// instance to write information.
type Client struct {
	// Id is the id of this client. Each client has its own unique id.
	// Immutable.
	Id uuid.UUID

	// Config contains an immutable store of the configuration parameters for this particular instance
	Config ClientConfig

	// Id of the underlying state machine server.
	// Used for the client to make configuration changes.
	serverId int // never modified concurrently, so don't need lock on mu

	// dead indicates whether the client is alive. Set by Kill()
	dead int32
	mu   sync.Mutex

	// m is an object that provides a message to send on a given round.
	m Messager

	// lastUpdate stores the last update received on the state machine
	lastUpdate *lastStateMachine

	updCh      <-chan UpdateMsg
	closeUpdCh func()

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

type ClientConfig struct {
	// MessageTimeout is the amount of time that the user has to return from when
	// the client calls the user's Message method. The client will abort the submit
	// phase if more than MessageTimeout + ProtocolTimeout seconds have passed.
	// A reasonable value is 30 seconds, allowing the user to come up with a message and send it.
	MessageTimeout time.Duration

	// ProtocolTimeout is the amount of time that the client will wait for another
	// client j to complete its task. Specifically, if this client has not heard of
	// an update from client j in the last ProtocolTimeout time, and client j should
	// have sent an update at the start of that interval, then this client will abort the round.
	// A reasonable value is 10 seconds, which may allow clients to crash and restart.
	ProtocolTimeout time.Duration

	// MessageSize is the maximum size, in bytes, of the message that the user can send in a round.
	// Note that to make sure that each message has indistinguishable length, each message will
	// need to be padded to this length, so in terms of cost analysis the messages will always have
	// the maximum length.
	MessageSize int
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
		c.assertf(false, "must be able to get previous round, error is %v", err)
	}
	c.assertf(ri.Phase == DonePhase || ri.Phase == FailedPhase, "must either fail or done")

	if ri.Phase == FailedPhase {
		result := RoundResult{
			Round:     round - 1,
			Succeeded: false,
		}
		if c.resCh != nil {
			c.resCh <- result
		}
		return
	}

	var messages [][]byte
	for _, m := range ri.Messages {
		om := m
		for _, revealKey := range ri.RevealedKeys {
			om = ri.Crypto.Decrypt(revealKey, om)
		}
		messages = append(messages, ri.Crypto.ExtractMsg(om))
	}
	// now verify that messages have the right structure
	var verifiedMessages [][]byte
	for _, m := range messages {
		err = c.verifyHeader(m[:c.messageHeaderLength()])
		if err != nil {
			c.logf(dWarning, "header is incorrect! something bad happened this round: %v", err)
			// TODO: transition this round into the failed phase, somehow
		}
		if len(m) > c.messageHeaderLength() {
			verifiedMessages = append(verifiedMessages, m[c.messageHeaderLength():])
		}
	}
	result := RoundResult{
		Round:     round - 1,
		Succeeded: ri.Phase == DonePhase,
		Messages:  verifiedMessages,
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

// abort aborts the given round!
func (c *Client) abort(round int) {
	op := AbortOp{R: round}
	c.pending.add(op)
}

func (c *Client) messageHeaderLength() int {
	return 16
}
func (c *Client) messageHeader() []byte {
	// header format: 13 * 8, then random bytes * 8
	// the goal is: probability that random message has this format is miniscule (1/256)^8
	// as well as: probability that two messages generated by different people in the same round has
	// 	the same random bytes is miniscule (1/256)^8
	var h []byte
	for i := 0; i < 8; i++ {
		h = append(h, 13)
	}
	r := make([]byte, 8)
	_, err := crand.Read(r)
	if err != nil {
		c.assertf(false, "could not read from crand: %v", err)
	}
	h = append(h, r...)
	c.assertf(len(h) == c.messageHeaderLength(), "header length should be 16")
	return h
}
func (c *Client) verifyHeader(h []byte) error {
	c.assertf(len(h) == 16, "header length should be 16")
	for i := 0; i < 8; i++ {
		if h[i] != 13 {
			return fmt.Errorf("first 8 bytes must be 13, h[%d] = %d", i, h[i])
		}
	}
	return nil
}
func (c *Client) messageWithHeader(m []byte) []byte {
	h := c.messageHeader()
	h = append(h, m...)
	return h
}

// submit is a helper method to readUpdates
func (c *Client) submit(round int, ri *RoundInfo, me int, encryptKey PrivateKey) {
	if ri.Messages[me].Nil() {
		msgChan := make(chan []byte)
		go func(msgChan chan []byte, round int) {
			msgChan <- c.m.Message(round)
		}(msgChan, round)
		msg := []byte("")
		select {
		case msg = <-msgChan:
		case <-time.NewTimer(c.Config.MessageTimeout).C:
			c.logf(dWarning, "message function timed out, sending empty message")
		}
		m, err := ri.Crypto.PrepareMsg(c.messageWithHeader(msg))
		if err != nil {
			c.assertf(len(msg) > c.Config.MessageSize, "prepare message threw but message is within bounds, %v", err)
			c.logf(dWarning, "message size too long, sending empty message, %v", err)
			m, err = ri.Crypto.PrepareMsg(c.messageWithHeader([]byte("")))
			c.assertf(err == nil, "empty message cannot be too long, err: %v", err)
		} else {
			c.assertf(len(msg) <= c.Config.MessageSize, "prepare message didn't throw but message is too big,len(msg)=%d, MessageSize=%d", len(msg), c.Config.MessageSize)
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
	c.setActive()
	if !c.getActiveUnlocked() {
		go c.BecomeActive()
	}

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
			c.logf(dInfo, "received state machine update! %+v", sm)

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
					c.assertf(lastResChSend == round-2, "required to be true because updates are in order")
					go c.sendRoundResult(sm, round)
					lastResChSend = round - 1
				}

				//only join round if we're in the raft configuration and not actively trying to leave it
				if c.getActiveUnlocked() && !c.getLeavingUnlocked() {
					go c.prepare(round, ri, me)
				} else {
					continue
				}
			case SubmitPhase:
				c.logf(dInfo, "Active: %v, Me: %d", c.getActiveUnlocked(), me)
				if !c.getActiveUnlocked() || me == -1 {
					// we don't have any business in this round
					continue
				}
				if round > lastKeyGen {
					// we don't necessarily know that lastKeyGen = round-1 here, because this client
					// may not have participated in the last round. that is okay, though.
					err := ri.Crypto.Verify(c.Config.MessageSize + c.messageHeaderLength())
					if err != nil {
						c.logf(dWarning, "bad crypto received! %v", err)
						go c.abort(round)
						continue
					}
					encryptKey = ri.Crypto.GenKey()
					scrambleKey = ri.Crypto.GenKey()
					revealKey = ri.Crypto.GenKey()
					lastKeyGen = round
				}
				// we want to make sure that we only ask the user for a message once per round
				if round > lastMessageRound {
					c.logf(dInfo, "Going to call submit for round %d", round)
					go c.submit(round, ri, me, encryptKey.DeepCopy())
					lastMessageRound = round
				} else {
					c.logf(dInfo, "Already submitted message for round %d. Waiting", round)
				}
			case EncryptPhase:
				if !c.getActiveUnlocked() || me == -1 {
					// we don't have any business in this round
					continue
				}
				go c.encrypt(round, ri, me, encryptKey.DeepCopy())
			case ScramblePhase:
				if !c.getActiveUnlocked() || me == -1 {
					// we don't have any business in this round
					continue
				}
				go c.scramble(round, ri, me, scrambleKey.DeepCopy(), revealKey.DeepCopy())
			case DecryptPhase:
				if !c.getActiveUnlocked() || me == -1 {
					// we don't have any business in this round
					continue
				}
				go c.decrypt(round, ri, me, scrambleKey.DeepCopy(), encryptKey.DeepCopy())
			case RevealPhase:
				if !c.getActiveUnlocked() || me == -1 {
					// we don't have any business in this round
					continue
				}
				go c.reveal(round, ri, me, revealKey.DeepCopy())
			default:
				c.assertf(false, "in phase %v which should never happen", ri.Phase)
			}
		} else if updMsg.ConfigurationValid {
			c.logf(dInfo, "Received configuration update: %v", updMsg.Configuration)
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
			c.assertf(len(c.currConf) > 0, "Expected configuration with at least 1 server, but none found: %v", updMsg.Configuration)
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

func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	} else {
		return b
	}
}

// aborter checks the timestamp of the latest update that was received, and times out
// if it is longer than the allowed timeout value.
func (c *Client) aborter() {
	for !c.killed() {
		// make a copy of the last update because we want to read the time and state atomically
		lastUpdate := c.lastUpdate.deepCopy()

		// sleepAmount should be the maximum amount of time in the future during which
		// we can GUARANTEE that there should be no timeout abort
		var sleepAmount time.Duration = -1

		switch lastUpdate.sm.CurrentRoundInfo().Phase {
		case PreparePhase:
			// no timeout in the prepare phase; the user could simply call start()
			// so just wait protocol timeout amount of time
			sleepAmount = c.Config.ProtocolTimeout
		case SubmitPhase:
			// we use message + protocol timeout so that each user has time to submit a message, and the
			// protocol has time to propagate it
			submitTimeout := c.Config.MessageTimeout + c.Config.ProtocolTimeout
			timeSince := time.Since(lastUpdate.phaseTimestamp)
			if timeSince > submitTimeout {
				c.abort(lastUpdate.sm.Round)
				sleepAmount = c.Config.ProtocolTimeout
			} else {
				sleepAmount = min(c.Config.ProtocolTimeout, submitTimeout-timeSince)
			}
		case EncryptPhase, ScramblePhase, DecryptPhase:
			// we use the time since the last update, since the test-and-set means a client needs to recompute
			// after a new update
			timeSince := time.Since(lastUpdate.timestamp)
			if timeSince > c.Config.ProtocolTimeout {
				c.abort(lastUpdate.sm.Round)
				sleepAmount = c.Config.ProtocolTimeout
			} else {
				sleepAmount = c.Config.ProtocolTimeout - timeSince
			}
		case RevealPhase:
			// this is like the submit phase
			timeSince := time.Since(lastUpdate.phaseTimestamp)
			if timeSince > c.Config.ProtocolTimeout {
				c.abort(lastUpdate.sm.Round)
				sleepAmount = c.Config.ProtocolTimeout
			} else {
				sleepAmount = c.Config.ProtocolTimeout - timeSince
			}
		default:
			c.assertf(false, "in phase %v which should never happen", lastUpdate.sm.CurrentRoundInfo().Phase)
		}

		c.assertf(sleepAmount >= 0, "sleep amount should never be < 0, but it is %d", sleepAmount)

		time.Sleep(sleepAmount)
	}
}

// Cycles through the servers in the known configuration
func (c *Client) updateLeader() {
	c.mu.Lock()
	c.lastKnownLeaderInd = (c.lastKnownLeaderInd + 1) % len(c.currConf)
	c.mu.Unlock()
}

// Kill kills all long-running goroutines and releases any memory
// used by the Client instance. After calling Kill no other methods
// may be called.
func (c *Client) Kill() {
	// submit an abort operation here if the last state machine includes this user
	// and the round isn't done or failed
	sm := c.lastUpdate.get()
	if sm.CurrentRoundInfo().Phase != DonePhase && sm.CurrentRoundInfo().Phase != FailedPhase {
		me := false
		for _, p := range sm.CurrentRoundInfo().Participants {
			if p == c.Id {
				me = true
			}
		}
		if me {
			op := AbortOp{R: sm.Round}
			// we don't care if the operation succeeds or not; it's fine if it does not. in that case,
			// the protocol will simply time out.
			_ = c.submitOp(op)
		}
	}
	// close the upd channel
	c.closeUpdCh()
	// now we're dead :'(
	atomic.StoreInt32(&c.dead, 1)
}
func (c *Client) killed() bool {
	z := atomic.LoadInt32(&c.dead)
	return z == 1
}

func (c *Client) logf(topic logTopic, format string, a ...interface{}) {
	logf(topic, c.logHeader(), format, a...)
}

func (c *Client) logHeader() string {
	return fmt.Sprintf("client %s (on server %d)", c.Id.String(), c.serverId)
}

func (c *Client) assertf(condition bool, format string, a ...interface{}) {
	c.dump()
	assertf(condition, c.logHeader(), format, a...)
}
func (c *Client) dump() {
	if IsDebug() && IsDump() {
		// this still has race conditions since we use atomic ints for killed
		c.mu.Lock()
		defer c.mu.Unlock()
		c.logf(dDump, spew.Sdump(c))
	}
}

func mapToSlice(mp map[int]bool) []int {
	sl := make([]int, 0)
	for k := range mp {
		sl = append(sl, k)
	}

	return sl
}

func (c *Client) getActiveUnlocked() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	tempActive := c.active
	return tempActive
}

func (c *Client) getLeavingUnlocked() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	tempLeaving := c.leaving
	return tempLeaving
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
		Crypto: newMasseyOmuraCrypto(c.Config.MessageSize + c.messageHeaderLength()),
	}
	go c.pending.add(op)
	return nil
}

func NewClient(s *Server, m Messager, cp network.ConnectionProvider, conf ClientConfig) *Client {
	c := new(Client)
	c.Id = uuid.New()
	c.m = m
	c.pending = newEliminationQueue()
	c.lastUpdate = newLastStateMachine()
	c.cp = cp
	c.currConf = make([]int, 0)
	var i int
	c.updCh, i = s.GetUpdCh()
	c.closeUpdCh = func() {
		s.CloseUpdCh(i)
	}
	c.serverId = s.Me
	c.active = false
	c.leaving = false
	c.Config = conf

	// Assume the configuration has all possible servers, until we get notified otherwise
	peers := cp.NumPeers()
	for i := 0; i < peers; i++ {
		c.currConf = append(c.currConf, i)
	}

	go c.readUpdates()
	go c.submitOps()
	go c.aborter()

	return c
}
