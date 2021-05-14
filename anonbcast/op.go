package anonbcast

import (
	"github.com/google/uuid"
)

type OpType string

const (
	JoinOpType      OpType = "join"
	StartOpType     OpType = "start"
	MessageOpType   OpType = "message"
	EncryptedOpType OpType = "encrypted"
	ScrambledOpType OpType = "scrambled"
	DecryptedOpType OpType = "decrypted"
	RevealOpType    OpType = "reveal"
	AbortOpType     OpType = "abort"
)

// Op represents an operation to be applied to the state machine. The
// operation is submitted by an RPC call to the leader of the Raft cluster,
// and gets stored in the Raft log. An Op is immutable and thread safe.
type Op interface {
	Type() OpType
	Round() int
}

// JoinOp indicates participation for a participant in a round, submitting their UUID.
// If Round is not equal to current round, or the current phase isn't PreparePhase, it is a no-op.
// If the participant Id has already been submitted, it is a no-op.
type JoinOp struct {
	Id uuid.UUID
	R  int
}

func (op JoinOp) Type() OpType {
	return JoinOpType
}
func (op JoinOp) Round() int {
	return op.R
}

// StartOp transitions from the PreparePhase to the SubmitPhase.
// It also submits Crypto to the round, overwriting the value if it exists.
// If Round is not equal to the current round, or the current phase isn't PreparePhase, it is a no-op.
type StartOp struct {
	Id     uuid.UUID
	R      int
	Crypto CommutativeCrypto
}

func (op StartOp) Type() OpType {
	return StartOpType
}
func (op StartOp) Round() int {
	return op.R
}

// MessageOp submits a message that has been encrypted once with the participant's own encryption key.
// If Round is not equal to the current round, or the current phase isn't SubmitPhase, it is a no-op.
// If a participant with the given Id does not exist, it is a no-op (and logs a warning, because it should never happen with a legal client).
// If a message for this user has already been submitted, it is overwritten.
// If a reveal key hash for this user has already been submitted, it is overwritten.
// If this is the last participant to submit a message, the state machine will transition to the EncryptPhase.
type MessageOp struct {
	Id            uuid.UUID
	R             int
	Message       Msg
	RevealKeyHash []byte
}

func (op MessageOp) Type() OpType {
	return MessageOpType
}
func (op MessageOp) Round() int {
	return op.R
}

// EncryptedOp announces that a participant has encrypted all messages exactly once with its encryption key.
// If Round is not equal to the current round, or the current phase isn't EncryptPhase, it is a no-op.
// If a participant with the given Id does not exist, it is a no-op (and logs a warning, because it should never happen with a legal client).
// If Prev is not the previous number of participants who have encrypted, it is a no-op.
// If this participant has already encrypted, it is a no-op.
// If this is the last participant to encrypt, the state machine will transition to the ScramblePhase.
type EncryptedOp struct {
	Id uuid.UUID
	R  int
	// Messages need to be in same order as before.
	Messages []Msg
	// Prev is the number of participants who have previously encrypted.
	// This supports the test-and-set behavior.
	Prev int
}

func (op EncryptedOp) Type() OpType {
	return EncryptedOpType
}
func (op EncryptedOp) Round() int {
	return op.R
}

// ScrambledOp announces that a participant has scrambled all messages.
// If Round is not equal to the current round, or the current phase isn't ScramblePhase, it is a no-op.
// If a participant with the given Id does not exist, it is a no-op (and logs a warning, because it should never happen with a legal client).
// If not all participants with index < this participant's index have scrambled, it is a no-op.
// If this participant has already scrambled, it is a no-op.
// If this is the last participant to scramble, the state machine will transition to the DecryptPhase.
type ScrambledOp struct {
	Id       uuid.UUID
	R        int
	Messages []Msg
}

func (op ScrambledOp) Type() OpType {
	return ScrambledOpType
}
func (op ScrambledOp) Round() int {
	return op.R
}

// DecryptedOp announces that a participant has decrypted all messages.
// If Round is not equal to current round, or the current phase isn't DecryptPhase, it is a no-op.
// If a participant with the given Id does not exist, it is a no-op (and logs a warning, because it should never happen with a legal client).
// If Prev is not the previous number of participants who have decrypted, it is a no-op.
// If this participant has already decrypted, it is a no-op.
// If this is the last participant to decrypt, the state machine will transition to the RevealPhase.
type DecryptedOp struct {
	Id       uuid.UUID
	R        int
	Messages []Msg
	// Prev is the number of participants who have previously submitted a scrambled.
	// This supports the test-and-set behavior.
	Prev int
}

func (op DecryptedOp) Type() OpType {
	return DecryptedOpType
}
func (op DecryptedOp) Round() int {
	return op.R
}

// RevealOp is reveals a participant's reveal key pair.
// If Round is not equal to the current round, or the current phase isn't RevealPhase, it is a no-op.
// If a participant with the given Id does not exist, it is a no-op (and logs a warning, because it should never happen with a legal client).
// If a reveal key pair for this participant has already been submitted, it is overwritten.
// If this is the last participant to submit a reveal key pair, the state machine will transition to the DonePhase,
// as well as increment its current round.
type RevealOp struct {
	Id        uuid.UUID
	R         int
	RevealKey PrivateKey
}

func (op RevealOp) Type() OpType {
	return RevealOpType
}
func (op RevealOp) Round() int {
	return op.R
}

// AbortOp aborts the current round.
// If Round is not equal to the current round, it is a no-op.
// The state machine will transition to the FailedPhase, and increment its current round.
type AbortOp struct {
	R int
}

func (op AbortOp) Type() OpType {
	return AbortOpType
}
func (op AbortOp) Round() int {
	return op.R
}
