package anonbcast

import (
	"errors"
	"github.com/google/uuid"
)

type Phase string

const (
	PreparePhase  Phase = "PREPARE"
	EncryptPhase  Phase = "ENCRYPT"
	ScramblePhase Phase = "SCRAMBLE"
	DecryptPhase  Phase = "DECRYPT"
	RevealPhase   Phase = "REVEAL"
	DonePhase     Phase = "DONE"
	FailedPhase   Phase = "FAILED"
)

const NumRoundsPersisted = 3

// Msg is the type of the message
// TODO: change this to the output of the crypto algo!
// MUST exhibit pass-by-value semantics, or else DeepCopy need to be updated
type Msg string

// StateMachine is the shared state machine that records the state of
// the anonymous broadcasting protocol. It is NOT thread safe.
type StateMachine struct {
	// Round is the current round that the state machine is in. It increases
	// monotonically, starting at 0.
	Round int
	// Rounds store the data for the NumRoundsPersisted last rounds, at
	// index round # mod 3
	Rounds [NumRoundsPersisted]RoundInfo
}

func (sm *StateMachine) checkRep() {
	if !Debug {
		return
	}
	assertf(sm.Round >= 0, "round must be non-negative")
	for i := 0; i < NumRoundsPersisted; i++ {
		sm.Rounds[i].checkRep()
	}
}

type RoundInfo struct {
	// Phase is the phase that the round is in.
	Phase Phase
	// Participants is a list of uuids for every participant, uniquely identifying them
	Participants []uuid.UUID
	// PublicKeys[i] is the public key of the encrypt keypair of participant Participants[i]
	PublicKeys []string // TODO: update this to the actual type of public keys
	// Messages is a list of messages, should have same length as Participants. If participant i
	// hasn't sent in a message yet, Messages[i] is the null value (i.e. "")
	// The messages change over the course of the progress of the protocol.
	Messages []Msg
	// Scrambled[i] is true if and only if participant Participants[i] has scrambled the messages
	Scrambled []bool
	// Decrypted[i] is true if and only if participant Participants[i] has decrypted the messages
	Decrypted []bool
	// RevealedKeys[i] is the public/private reveal keypair of participant Participants[i],
	// or the nil value of the type if the participant has yet to submit it
	RevealedKeys []string // TODO: update this to the actual type of the public/private keypair
}

func (ri *RoundInfo) checkRep() {
	if !Debug {
		return
	}
	assertf(len(ri.Participants) == len(ri.PublicKeys), "must be equally many participants as public keys!")
	assertf(len(ri.Participants) == len(ri.Messages), "must be equally many participants for each field!")
	assertf(len(ri.Participants) == len(ri.Scrambled), "must be equally many participants for each field!")
	assertf(len(ri.Participants) == len(ri.Decrypted), "must be equally many participants for each field!")
	assertf(len(ri.Participants) == len(ri.RevealedKeys), "must be equally many participants for each field!")

	for i1, p1 := range ri.Participants {
		for i2, p2 := range ri.Participants {
			if p1 == p2 && i1 != i2 {
				assertf(false, "all participants must be distinct!")
			}
		}
	}
}

// Apply applies an operation to the state machine. Note that operations may not always
// have an effect, for example if the round number is out of date. If an operation has
// an effect, this method must return true; otherwise, it may return false.
func (sm *StateMachine) Apply(op Op) bool {
	sm.checkRep()
	defer sm.checkRep()
	// TODO: tie this to the server's logger somehow?
	DPrintf("applying operation %+v to the state machine", op)

	switch op.Type() {
	case PublicKeyOpType:
		return sm.publicKey(op.(PublicKeyOp))
	case StartOpType:
		return sm.start(op.(StartOp))
	case MessageOpType:
		return sm.message(op.(MessageOp))
	case ScrambledOpType:
		return sm.scrambled(op.(ScrambledOp))
	case DecryptedOpType:
		return sm.decrypted(op.(DecryptedOp))
	case RevealOpType:
		return sm.reveal(op.(RevealOp))
	case AbortOpType:
		return sm.abort(op.(AbortOp))
	default:
		assertf(false, "this should never happen")
		return false
	}
}

func (sm *StateMachine) publicKey(op PublicKeyOp) bool {
	if op.Round != sm.Round {
		return false
	}
	ri := sm.CurrentRoundInfo()
	if ri.Phase != PreparePhase {
		return false
	}

	p, err := ri.participantIndex(op.Id)
	if err != nil {
		p = len(ri.Participants)
		ri.Participants = append(ri.Participants, op.Id)
		ri.PublicKeys = append(ri.PublicKeys, "")
		ri.Messages = append(ri.Messages, Msg(""))
		ri.Scrambled = append(ri.Scrambled, false)
		ri.Decrypted = append(ri.Decrypted, false)
		ri.RevealedKeys = append(ri.RevealedKeys, "")
	}

	ri.PublicKeys[p] = op.PublicKey
	return true
}

func (sm *StateMachine) start(op StartOp) bool {
	if op.Round != sm.Round {
		return false
	}
	ri := sm.CurrentRoundInfo()
	if ri.Phase != PreparePhase {
		return false
	}

	ri.Phase = EncryptPhase
	return true
}

func (ri *RoundInfo) participantIndex(id uuid.UUID) (int, error) {
	for i, p := range ri.Participants {
		if p == id {
			return i, nil
		}
	}
	return -1, errors.New("participant not found")
}

func (sm *StateMachine) message(op MessageOp) bool {
	if op.Round != sm.Round {
		return false
	}
	ri := sm.CurrentRoundInfo()
	if ri.Phase != EncryptPhase {
		return false
	}
	p, err := ri.participantIndex(op.Id)
	if err != nil {
		DPrintf("WARNING: participant %v not found for message op, which should never happen with a legal client", op.Id)
	}

	ri.Messages[p] = op.Message

	for _, m := range ri.Messages {
		if m == Msg("") {
			return true
		}
	}

	ri.Phase = ScramblePhase
	return true
}

func (ri *RoundInfo) numScrambled() int {
	n := 0
	for _, s := range ri.Scrambled {
		if s {
			n++
		}
	}
	return n
}

func (sm *StateMachine) scrambled(op ScrambledOp) bool {
	if op.Round != sm.Round {
		return false
	}
	ri := sm.CurrentRoundInfo()
	if ri.Phase != ScramblePhase {
		return false
	}
	p, err := ri.participantIndex(op.Id)
	if err != nil {
		DPrintf("WARNING: participant %v not found for scrambled op, which should never happen with a legal client", op.Id)
	}

	if ri.numScrambled() != op.Prev {
		return false
	}

	if ri.Scrambled[p] {
		return false
	}

	ri.Messages = op.Messages
	ri.Scrambled[p] = true

	if ri.numScrambled() == len(ri.Scrambled) {
		ri.Phase = DecryptPhase
	}

	return true
}

func (ri *RoundInfo) numDecrypted() int {
	n := 0
	for _, s := range ri.Decrypted {
		if s {
			n++
		}
	}
	return n
}

func (sm *StateMachine) decrypted(op DecryptedOp) bool {
	if op.Round != sm.Round {
		return false
	}
	ri := sm.CurrentRoundInfo()
	if ri.Phase != DecryptPhase {
		return false
	}
	p, err := ri.participantIndex(op.Id)
	if err != nil {
		DPrintf("WARNING: participant %v not found for scrambled op, which should never happen with a legal client", op.Id)
	}

	if ri.numDecrypted() != op.Prev {
		return false
	}

	if ri.Decrypted[p] {
		return false
	}

	ri.Messages = op.Messages
	ri.Decrypted[p] = true

	if ri.numDecrypted() == len(ri.Decrypted) {
		ri.Phase = RevealPhase
	}

	return true
}

func (sm *StateMachine) reveal(op RevealOp) bool {
	if op.Round != sm.Round {
		return false
	}
	ri := sm.CurrentRoundInfo()
	if ri.Phase != RevealPhase {
		return false
	}
	p, err := ri.participantIndex(op.Id)
	if err != nil {
		DPrintf("WARNING: participant %v not found for scrambled op, which should never happen with a legal client", op.Id)
	}

	ri.RevealedKeys[p] = op.RevealKey

	for _, k := range ri.RevealedKeys {
		if k == "" { // TODO: update to actual nil value of revealed key
			return true
		}
	}

	ri.Phase = DonePhase
	sm.Round++
	sm.initRound(sm.Round)

	return true
}

func (sm *StateMachine) abort(op AbortOp) bool {
	if op.Round != sm.Round {
		return false
	}
	ri := sm.CurrentRoundInfo()

	ri.Phase = FailedPhase
	sm.Round++
	sm.initRound(sm.Round)

	return true
}

// GetRoundInfo returns the round info associated with the given round,
// or an error if that round is either (1) too old or (2) to new to have an associated round info.
func (sm *StateMachine) GetRoundInfo(round int) (*RoundInfo, error) {
	if round > sm.Round {
		return &RoundInfo{}, errors.New("cannot get round info from the future")
	}
	if round < 0 {
		return &RoundInfo{}, errors.New("cannot get round info < 0 because rounds start at 0")
	}
	if round <= sm.Round-NumRoundsPersisted {
		return &RoundInfo{}, errors.New("cannot get round info <= current round minus num rounds persisted")
	}
	return &sm.Rounds[round%NumRoundsPersisted], nil
}

func (sm *StateMachine) CurrentRoundInfo() *RoundInfo {
	ri, err := sm.GetRoundInfo(sm.Round)
	assertf(err == nil, "current round should always exist: %v", err)
	return ri
}

// GuaranteedNoEffect returns true only if the supplied operation
// is a no-op on the state machine in its current state AND all possible
// future states. For example, if the state machine is in round 10 and
// op has round 9, this should return true. It is always safe to return
// false, but it may improve performance to return true when allowed.
func (sm *StateMachine) GuaranteedNoEffect(op Op) bool {
	// TODO: return true if round number is wrong
	return false
}

// DeepCopy returns a deep copy of this state machine, sharing no
// data with the original state machine. This means that it is fine
// to use a deep-copied state machine concurrently with its original.
func (sm *StateMachine) DeepCopy() StateMachine {
	sm.checkRep()
	defer sm.checkRep()
	smCopy := StateMachine{
		Round:  sm.Round,
		Rounds: [3]RoundInfo{},
	}
	smCopy.checkRep()
	defer smCopy.checkRep()
	for i, ri := range sm.Rounds {
		smCopy.Rounds[i] = ri.DeepCopy()
	}
	return smCopy
}

func (ri RoundInfo) DeepCopy() RoundInfo {
	n := len(ri.Participants)
	riCopy := RoundInfo{
		Phase:        ri.Phase,
		Participants: make([]uuid.UUID, n),
		PublicKeys:   make([]string, n),
		Messages:     make([]Msg, n),
		Scrambled:    make([]bool, n),
		Decrypted:    make([]bool, n),
		RevealedKeys: make([]string, n),
	}
	copy(riCopy.Participants, ri.Participants)
	copy(riCopy.PublicKeys, ri.PublicKeys)
	copy(riCopy.Messages, ri.Messages)
	copy(riCopy.Scrambled, ri.Scrambled)
	copy(riCopy.Decrypted, ri.Decrypted)
	copy(riCopy.RevealedKeys, ri.RevealedKeys)
	return riCopy
}

// Snapshot returns a snapshot of the state machine, from which it
// can be deterministically recreated using NewStateMachine.
func (sm *StateMachine) Snapshot() []byte {
	panic("implement this")
}

// NewStateMachine returns a new state machine. If snapshot is nil, it
// creates the state machine from the initial state. Otherwise, it creates
// the state machine from the given snapshot.
func NewStateMachine(snapshot []byte) StateMachine {
	if snapshot != nil {
		panic("we can only do nil snapshots for now!")
	}
	sm := StateMachine{
		Round:  0,
		Rounds: [3]RoundInfo{},
	}
	sm.initRound(0)
	return sm
}

func (sm *StateMachine) initRound(round int) {
	assertf(sm.Round == round, "can only init current round!")
	sm.Rounds[round%NumRoundsPersisted] = RoundInfo{
		Phase: PreparePhase,
	}
}
