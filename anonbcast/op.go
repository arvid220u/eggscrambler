package anonbcast

import "github.com/google/uuid"

type OpType string

const (
	PublicKeyOpType OpType = "publicKey"
	StartOpType     OpType = "start"
	MessageOpType   OpType = "message"
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
}

// PublicKeyOp indicates participation for a participant
// in a round, and submits their encrypt key pair's public key.
// If Round is not equal to current round, or the current phase isn't PreparePhase, it is a no-op.
// If the public key for this user has already been submitted, it is overwritten.
type PublicKeyOp struct {
	Id        uuid.UUID
	Round     int
	PublicKey string // TODO: update with public key type
}

func (op PublicKeyOp) Type() OpType {
	return PublicKeyOpType
}

// StartOp is transitions from the PreparePhase to the EncryptPhase.
// If Round is not equal to the current round, or the current phase isn't PreparePhase, it is a no-op.
type StartOp struct {
	Id    uuid.UUID
	Round int
}

func (op StartOp) Type() OpType {
	return StartOpType
}

// MessageOp submits a message that has been encrypted once with each participant's public key.
// If Round is not equal to the current round, or the current phase isn't EncryptPhase, it is a no-op.
// If a participant with the given Id does not exist, it is a no-op (and logs a warning, because it should never happen with a legal client).
// If a message for this user has already been submitted, it is overwritten.
// If this is the last participant to submit a message, the state machine will transition to the ScramblePhase.
type MessageOp struct {
	Id      uuid.UUID
	Round   int
	Message Msg
}

func (op MessageOp) Type() OpType {
	return MessageOpType
}

// ScrambledOp announces that a participant has scrambled all messages.
// If Round is not equal to the current round, or the current phase isn't ScramblePhase, it is a no-op.
// If a participant with the given Id does not exist, it is a no-op (and logs a warning, because it should never happen with a legal client).
// If Prev is not the previous number of participants who have scrambled, it is a no-op.
// If this participant has already scrambled, it is a no-op.
// If this is the last participant to scramble, the state machine will transition to the DecryptPhase.
type ScrambledOp struct {
	Id       uuid.UUID
	Round    int
	Messages []Msg
	// Prev is the number of participants who have previously submitted a scrambled.
	// This supports the test-and-set behavior.
	Prev int
}

func (op ScrambledOp) Type() OpType {
	return ScrambledOpType
}

// DecryptedOp announces that a participant has decrypted all messages.
// If Round is not equal to current round, or the current phase isn't DecryptPhase, it is a no-op.
// If a participant with the given Id does not exist, it is a no-op (and logs a warning, because it should never happen with a legal client).
// If Prev is not the previous number of participants who have decrypted, it is a no-op.
// If this participant has already decrypted, it is a no-op.
// If this is the last participant to decrypt, the state machine will transition to the RevealPhase.
type DecryptedOp struct {
	Id       uuid.UUID
	Round    int
	Messages []Msg
	// Prev is the number of participants who have previously submitted a scrambled.
	// This supports the test-and-set behavior.
	Prev int
}

func (op DecryptedOp) Type() OpType {
	return DecryptedOpType
}

// RevealOp is reveals a participant's reveal key pair.
// If Round is not equal to the current round, or the current phase isn't RevealPhase, it is a no-op.
// If a participant with the given Id does not exist, it is a no-op (and logs a warning, because it should never happen with a legal client).
// If a reveal key pair for this participant has already been submitted, it is overwritten.
// If this is the last participant to submit a reveal key pair, the state machine will transition to the DonePhase,
// as well as increment its current round.
type RevealOp struct {
	Id        uuid.UUID
	Round     int
	RevealKey string // TODO: update to crypto keypair type
}

func (op RevealOp) Type() OpType {
	return RevealOpType
}

// AbortOp aborts the current round.
// If Round is not equal to the current round, it is a no-op.
// The state machine will transition to the FailedPhase, and increment its current round.
type AbortOp struct {
	Round int
}

func (op AbortOp) Type() OpType {
	return AbortOpType
}
