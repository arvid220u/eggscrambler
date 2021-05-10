package anonbcast

import (
	"math/big"
)

type Msg struct {
	//M *big.Int // cannot be used because it cannot be sent over rpc
	M []byte
}

func (m Msg) DeepCopy() Msg {
	m2 := Msg{make([]byte, len(m.M))}
	copy(m2.M, m.M)
	return m2
}

func (m Msg) Nil() bool {
	return m.M == nil || len(m.M) == 0
}
func NilMsg() Msg {
	return Msg{}
}

type PrivateKey struct {
	Pk int64
}

func (pk PrivateKey) DeepCopy() PrivateKey {
	pk2 := PrivateKey{pk.Pk}
	return pk2
}
func NilPrivateKey() PrivateKey {
	return PrivateKey{}
}
func (pk PrivateKey) Nil() bool {
	return pk.Pk == 0
}

type CommutativeCrypto interface {
	// PrepareMsg converts a plaintext message to a plaintext Msg.
	// Returns an error if the message is too long.
	PrepareMsg(msg []byte) (Msg, error)

	// Encrypt encrypts a message
	Encrypt(key PrivateKey, m Msg) Msg

	// Decrypt decrypts a message
	Decrypt(key PrivateKey, m Msg) Msg

	// ExtractMsg reverses PrepareMsg
	ExtractMsg(m Msg) []byte

	// GenKey generates a private key
	GenKey() PrivateKey

	// DeepCopy copies itself, to avoid race conditions
	DeepCopy() CommutativeCrypto
}

// replace later! just to mock, and probably not a useful mock in the future
type fakeCommutativeCrypto struct {
	P int64
}

func (f fakeCommutativeCrypto) PrepareMsg(msg []byte) (Msg, error) {
	var m Msg
	m.M = msg
	return m, nil
}

func (f fakeCommutativeCrypto) Encrypt(key PrivateKey, m Msg) Msg {
	// + is commutative lolol
	b := new(big.Int)
	b2 := new(big.Int)
	b2.SetBytes(m.M)
	b.Add(b2, big.NewInt(key.Pk))
	return Msg{b.Bytes()}
}

func (f fakeCommutativeCrypto) Decrypt(key PrivateKey, m Msg) Msg {
	b := new(big.Int)
	b2 := new(big.Int)
	b2.SetBytes(m.M)
	b.Sub(b2, big.NewInt(key.Pk))
	return Msg{b.Bytes()}
}

func (f fakeCommutativeCrypto) ExtractMsg(m Msg) []byte {
	return m.M
}

func (f fakeCommutativeCrypto) GenKey() PrivateKey {
	return PrivateKey{f.P}
}

func (f fakeCommutativeCrypto) DeepCopy() CommutativeCrypto {
	return f
}

func newFakeCommutativeCrypto() CommutativeCrypto {
	return fakeCommutativeCrypto{P: 17}
}
