package anonbcast

import (
	"errors"
	"github.com/davecgh/go-spew/spew"

	"github.com/google/uuid"
)

type Phase string

const (
	PreparePhase  Phase = "PREPARE"
	SubmitPhase   Phase = "SUBMIT"
	EncryptPhase  Phase = "ENCRYPT"
	ScramblePhase Phase = "SCRAMBLE"
	DecryptPhase  Phase = "DECRYPT"
	RevealPhase   Phase = "REVEAL"
	DonePhase     Phase = "DONE"
	FailedPhase   Phase = "FAILED"
)

const NumRoundsPersisted = 3

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

func (sm *StateMachine) logHeader() string {
	return "state machine" // it may make sense to reference the server, although state machines exist everywhere
}
func (sm *StateMachine) logf(topic logTopic, format string, a ...interface{}) {
	logf(topic, sm.logHeader(), format, a...)
}
func (sm *StateMachine) assertf(condition bool, format string, a ...interface{}) {
	sm.dump()
	assertf(condition, sm.logHeader(), format, a...)
}
func (sm *StateMachine) dump() {
	if IsDebug() && IsDump() {
		sm.logf(dDump, spew.Sdump(sm))
	}
}

func (sm *StateMachine) checkRep() {
	if !IsDebug() {
		return
	}
	sm.assertf(sm.Round >= 0, "round must be non-negative")
	for i := 0; i < NumRoundsPersisted; i++ {
		sm.Rounds[i].checkRep()
	}
}

type RoundInfo struct {
	// Phase is the phase that the round is in.
	Phase Phase
	// Crypto is an object that allows us to do consistent commutative encryption. It is typically a big prime.
	Crypto CommutativeCrypto
	// Participants is a list of uuids for every participant, uniquely identifying them
	Participants []uuid.UUID
	// Messages is a list of messages, should have same length as Participants. If participant i
	// hasn't sent in a message yet, Messages[i] is the null value (i.e. "")
	// The messages change over the course of the progress of the protocol.
	Messages []Msg
	// Encrypted[i] is true if and only if participant Participants[i] has encrypted the messages
	Encrypted []bool
	// Scrambled[i] is true if and only if participant Participants[i] has scrambled the messages
	Scrambled []bool
	// Decrypted[i] is true if and only if participant Participants[i] has decrypted the messages
	Decrypted []bool
	// RevealedKeys[i] is the public/private reveal keypair of participant Participants[i],
	// or the nil value of the type if the participant has yet to submit it
	RevealedKeys []PrivateKey
	// RevealKeyHashes[i] is a SHA256 hash of the reveal key of participant Participants[i],
	// or a length 0 or nil slice if the participant has yet to submit it
	RevealKeyHashes [][]byte
}

func (ri *RoundInfo) logHeader() string {
	return "state machine" // it may make sense to reference the server, although state machines exist everywhere
}
func (ri *RoundInfo) logf(topic logTopic, format string, a ...interface{}) {
	logf(topic, ri.logHeader(), format, a...)
}
func (ri *RoundInfo) assertf(condition bool, format string, a ...interface{}) {
	ri.dump()
	assertf(condition, ri.logHeader(), format, a...)
}
func (ri *RoundInfo) dump() {
	if IsDebug() && IsDump() {
		ri.logf(dDump, spew.Sdump(ri))
	}
}

func (ri *RoundInfo) checkRep() {
	if !IsDebug() {
		return
	}
	ri.assertf(len(ri.Participants) == len(ri.Messages), "must be equally many participants for each field!")
	ri.assertf(len(ri.Participants) == len(ri.Encrypted), "must be equally many participants for each field!")
	ri.assertf(len(ri.Participants) == len(ri.Scrambled), "must be equally many participants for each field!")
	ri.assertf(len(ri.Participants) == len(ri.Decrypted), "must be equally many participants for each field!")
	ri.assertf(len(ri.Participants) == len(ri.RevealedKeys), "must be equally many participants for each field!")
	ri.assertf(len(ri.Participants) == len(ri.RevealKeyHashes), "must be equally many participants for each field!")
	ri.assertf(ri.Phase == "" || ri.Phase == PreparePhase || ri.Phase == FailedPhase || ri.Crypto != nil, "crypto must not be nil if left prepare phase")
	ri.assertf(ri.Phase != PreparePhase || ri.Crypto == nil, "crypto must be nil if not left prepare phase")

	// scrambled must grow from left to right
	for i, s := range ri.Scrambled {
		if i > 0 && s {
			ri.assertf(ri.Scrambled[i-1], "if i is scrambled, so must i-1 be")
		}
	}

	for i1, p1 := range ri.Participants {
		for i2, p2 := range ri.Participants {
			if p1 == p2 && i1 != i2 {
				ri.assertf(false, "all participants must be distinct!")
			}
		}
	}
	for i, m := range ri.Messages {
		if !m.Nil() {
			ri.assertf(ri.RevealKeyHashes[i] != nil && len(ri.RevealKeyHashes[i]) > 0, "reveal key hash must be submitted iff message is submitted")
		} else {
			ri.assertf(ri.RevealKeyHashes[i] == nil || len(ri.RevealKeyHashes[i]) == 0, "reveal key hash must be submitted iff message is submitted")
		}
	}
}

// Apply applies an operation to the state machine. Note that operations may not always
// have an effect, for example if the round number is out of date. If an operation has
// an effect, this method must return true; otherwise, it may return false.
func (sm *StateMachine) Apply(op Op) bool {
	sm.checkRep()
	defer sm.checkRep()

	if op.Round() != sm.Round {
		return false
	}

	switch op.Type() {
	case JoinOpType:
		return sm.join(op.(JoinOp))
	case StartOpType:
		return sm.start(op.(StartOp))
	case MessageOpType:
		return sm.message(op.(MessageOp))
	case EncryptedOpType:
		return sm.encrypted(op.(EncryptedOp))
	case ScrambledOpType:
		return sm.scrambled(op.(ScrambledOp))
	case DecryptedOpType:
		return sm.decrypted(op.(DecryptedOp))
	case RevealOpType:
		return sm.reveal(op.(RevealOp))
	case AbortOpType:
		return sm.abort(op.(AbortOp))
	default:
		sm.assertf(false, "this should never happen")
		return false
	}
}

func (sm *StateMachine) join(op JoinOp) bool {
	ri := sm.CurrentRoundInfo()
	if ri.Phase != PreparePhase {
		return false
	}

	_, err := ri.participantIndex(op.Id)
	if err != nil {
		ri.Participants = append(ri.Participants, op.Id)
		ri.Messages = append(ri.Messages, NilMsg())
		ri.Encrypted = append(ri.Encrypted, false)
		ri.Scrambled = append(ri.Scrambled, false)
		ri.Decrypted = append(ri.Decrypted, false)
		ri.RevealedKeys = append(ri.RevealedKeys, NilPrivateKey())
		ri.RevealKeyHashes = append(ri.RevealKeyHashes, nil)
		return true
	} else {
		return false
	}
}

func (sm *StateMachine) start(op StartOp) bool {
	ri := sm.CurrentRoundInfo()
	if ri.Phase != PreparePhase {
		return false
	}

	ri.Phase = SubmitPhase
	ri.Crypto = op.Crypto
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
	ri := sm.CurrentRoundInfo()
	if ri.Phase != SubmitPhase {
		return false
	}
	p, err := ri.participantIndex(op.Id)
	if err != nil {
		sm.logf(dWarning, "participant %v not found for message op, which should never happen with a legal client", op.Id)
		return false
	}

	ri.Messages[p] = op.Message
	ri.RevealKeyHashes[p] = make([]byte, len(op.RevealKeyHash))
	copy(ri.RevealKeyHashes[p], op.RevealKeyHash)

	for _, m := range ri.Messages {
		if m.Nil() {
			return true
		}
	}

	ri.Phase = EncryptPhase
	return true
}

func (ri *RoundInfo) numEncrypted() int {
	n := 0
	for _, s := range ri.Encrypted {
		if s {
			n++
		}
	}
	return n
}

func (sm *StateMachine) encrypted(op EncryptedOp) bool {
	ri := sm.CurrentRoundInfo()
	if ri.Phase != EncryptPhase {
		return false
	}
	p, err := ri.participantIndex(op.Id)
	if err != nil {
		sm.logf(dWarning, "participant %v not found for encrypted op, which should never happen with a legal client", op.Id)
		return false
	}

	if ri.numEncrypted() != op.Prev {
		return false
	}

	if ri.Encrypted[p] {
		return false
	}

	ri.Messages = op.Messages
	ri.Encrypted[p] = true

	if ri.numEncrypted() == len(ri.Encrypted) {
		ri.Phase = ScramblePhase
	}

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
	ri := sm.CurrentRoundInfo()
	if ri.Phase != ScramblePhase {
		return false
	}
	p, err := ri.participantIndex(op.Id)
	if err != nil {
		sm.logf(dWarning, "participant %v not found for scrambled op, which should never happen with a legal client", op.Id)
		return false
	}

	if ri.numScrambled() != p {
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
	ri := sm.CurrentRoundInfo()
	if ri.Phase != DecryptPhase {
		return false
	}
	p, err := ri.participantIndex(op.Id)
	if err != nil {
		sm.logf(dWarning, "participant %v not found for decrypted op, which should never happen with a legal client", op.Id)
		return false
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
	ri := sm.CurrentRoundInfo()
	if ri.Phase != RevealPhase {
		return false
	}
	p, err := ri.participantIndex(op.Id)
	if err != nil {
		sm.logf(dWarning, "participant %v not found for reveal op, which should never happen with a legal client", op.Id)
		return false
	}

	ri.RevealedKeys[p] = op.RevealKey

	for _, k := range ri.RevealedKeys {
		if k.Nil() {
			return true
		}
	}

	ri.Phase = DonePhase
	sm.Round++
	sm.initRound(sm.Round)

	return true
}

func (sm *StateMachine) abort(op AbortOp) bool {
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
	sm.assertf(err == nil, "current round should always exist: %v", err)
	return ri
}

// GuaranteedNoEffect returns true only if the supplied operation
// is a no-op on the state machine in its current state AND all possible
// future states. For example, if the state machine is in round 10 and
// op has round 9, this should return true. It is always safe to return
// false, but it may improve performance to return true when allowed.
func (sm *StateMachine) GuaranteedNoEffect(op Op) bool {
	// TODO: potentially add more optimizations here, looking at the phase of the op
	if op.Round() < sm.Round {
		return true
	}
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
		Phase:           ri.Phase,
		Crypto:          nil,
		Participants:    make([]uuid.UUID, n),
		Messages:        make([]Msg, n),
		Encrypted:       make([]bool, n),
		Scrambled:       make([]bool, n),
		Decrypted:       make([]bool, n),
		RevealedKeys:    make([]PrivateKey, n),
		RevealKeyHashes: make([][]byte, n),
	}
	if ri.Crypto != nil {
		riCopy.Crypto = ri.Crypto.DeepCopy()
	}
	copy(riCopy.Participants, ri.Participants)
	for i, msg := range ri.Messages {
		riCopy.Messages[i] = msg.DeepCopy()
	}
	copy(riCopy.Encrypted, ri.Encrypted)
	copy(riCopy.Scrambled, ri.Scrambled)
	copy(riCopy.Decrypted, ri.Decrypted)
	for i, pk := range ri.RevealedKeys {
		riCopy.RevealedKeys[i] = pk.DeepCopy()
	}
	for i, b := range ri.RevealKeyHashes {
		riCopy.RevealKeyHashes[i] = make([]byte, len(b))
		copy(riCopy.RevealKeyHashes[i], b)
	}
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
	sm.assertf(sm.Round == round, "can only init current round!")
	sm.Rounds[round%NumRoundsPersisted] = RoundInfo{
		Phase: PreparePhase,
	}
}

// MayPrecede returns true if and only if the phase and round of sm
// may come before the phase and round of sm2.
func (sm *StateMachine) MayPrecede(sm2 StateMachine) bool {
	if sm.Round < sm2.Round {
		return true
	}
	if sm.Round > sm2.Round {
		return false
	}
	phaseOrder := map[Phase]int{
		PreparePhase:  0,
		SubmitPhase:   1,
		EncryptPhase:  2,
		ScramblePhase: 3,
		DecryptPhase:  4,
		RevealPhase:   5,
		DonePhase:     6,
		FailedPhase:   7,
	}
	if phaseOrder[sm.CurrentRoundInfo().Phase] > phaseOrder[sm2.CurrentRoundInfo().Phase] {
		return false
	}
	return true
}
