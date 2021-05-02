package anonbcast

import "github.com/google/uuid"

type Phase int

const (
	PreparePhase Phase = iota
	EncryptPhase
	ScramblePhase
	DecryptPhase
	RevealPhase
	DonePhase
	FailedPhase
)

const NumRoundsPersisted = 3

// Msg is the type of the message
// TODO: change this to the output of the crypto algo!
type Msg string

// StateMachine is the shared state machine that records the state of
// the anonymous broadcasting protocol. It is NOT thread safe.
type StateMachine struct {
	// Round is the current round that the state machine is in. It increases
	// monotonically.
	Round int
	// Rounds store the data for the NumRoundsPersisted last rounds, at
	// index round # mod 3
	Rounds [NumRoundsPersisted]RoundInfo
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
	assertf(len(ri.Participants) == len(ri.PublicKeys), "must be equally many participants as public keys!")
	assertf(len(ri.Participants) == len(ri.Messages), "must be equally many participants for each field!")
	assertf(len(ri.Participants) == len(ri.Scrambled), "must be equally many participants for each field!")
	assertf(len(ri.Participants) == len(ri.Decrypted), "must be equally many participants for each field!")
	assertf(len(ri.Participants) == len(ri.RevealedKeys), "must be equally many participants for each field!")
}

func (sm *StateMachine) Apply(op Op) {
	panic("implement this")
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
func (sm *StateMachine) DeepCopy() *StateMachine {
	panic("implement this")
}

// Snapshot returns a snapshot of the state machine, from which it
// can be deterministically recreated using NewStateMachine.
func (sm *StateMachine) Snapshot() []byte {
	panic("implement this")
}

// NewStateMachine returns a new state machine. If snapshot is nil, it
// creates the state machine from the initial state. Otherwise, it creates
// the state machine from the given snapshot.
func NewStateMachine(snapshot []byte) *StateMachine {
	if snapshot != nil {
		panic("we can only do nil snapshots for now!")
	}
	return &StateMachine{}
}
