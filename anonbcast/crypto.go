package anonbcast

import (
	"crypto/rand"
	"github.com/arvid220u/6.824-project/masseyomura"
	"math/big"
)

func bigInt(bytes []byte) *big.Int {
	b := new(big.Int)
	b.SetBytes(bytes)
	return b
}

type Msg struct {
	// M stores the raw value of the big.Int.
	// Ideally we would have used a *big.Int here, but unfortunately it cannot be sent over RPC.
	M []byte
}

func (m Msg) getM() *big.Int {
	return bigInt(m.M)
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
func NewMsg(m *big.Int) Msg {
	return Msg{m.Bytes()}
}

type PrivateKey struct {
	E, D []byte
}

func (pk PrivateKey) getE() *big.Int {
	return bigInt(pk.E)
}
func (pk PrivateKey) getD() *big.Int {
	return bigInt(pk.D)
}

func (pk PrivateKey) DeepCopy() PrivateKey {
	pk2 := PrivateKey{make([]byte, len(pk.E)), make([]byte, len(pk.D))}
	copy(pk2.E, pk.E)
	copy(pk2.D, pk.D)
	return pk2
}
func NilPrivateKey() PrivateKey {
	return PrivateKey{}
}
func (pk PrivateKey) Nil() bool {
	return pk.E == nil || len(pk.E) == 0
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

	// Verify returns nil if and only if this crypto is safe for message sizes
	// up to and including messageSize bytes
	Verify(messageSize int) error
}

type masseyOmuraCrypto struct {
	// P is masseyomura.SystemKey.P.Bytes()
	P []byte
}

func (mo masseyOmuraCrypto) getP() *big.Int {
	return bigInt(mo.P)
}

// Verify verifies that this crypto is indeed safe for the given message size
func (mo masseyOmuraCrypto) Verify(messageSize int) error {
	return masseyomura.VerifySysKey(mo.createSysKey(), masseyomura.BitsMaxSize(messageSize))
}

func (mo masseyOmuraCrypto) PrepareMsg(msg []byte) (Msg, error) {
	m, err := masseyomura.PrepareMsg(msg, mo.getP().BitLen())
	if err != nil {
		return NilMsg(), err
	}
	return NewMsg(m), nil
}

func (mo masseyOmuraCrypto) createSysKey() *masseyomura.SystemKey {
	return &masseyomura.SystemKey{P: mo.getP()}
}

func (mo masseyOmuraCrypto) createPrivateKey(key PrivateKey) *masseyomura.PrivateKey {
	return &masseyomura.PrivateKey{
		SystemKey: *mo.createSysKey(),
		E:         key.getE(),
		D:         key.getD(),
	}
}

func (mo masseyOmuraCrypto) Encrypt(key PrivateKey, m Msg) Msg {
	b := masseyomura.Encrypt(mo.createPrivateKey(key), m.getM())
	return NewMsg(b)
}

func (mo masseyOmuraCrypto) Decrypt(key PrivateKey, m Msg) Msg {
	b := masseyomura.Decrypt(mo.createPrivateKey(key), m.getM())
	return NewMsg(b)
}

func (mo masseyOmuraCrypto) ExtractMsg(m Msg) []byte {
	return masseyomura.ExtractMsg(m.getM())
}

func (mo masseyOmuraCrypto) GenKey() PrivateKey {
	pk, err := masseyomura.GenUserKey(rand.Reader, mo.createSysKey())
	if err != nil {
		assertf(false, "crypto", "something is wrong in crypto: %v", err)
	}
	return PrivateKey{
		E: pk.E.Bytes(),
		D: pk.D.Bytes(),
	}
}

func (mo masseyOmuraCrypto) DeepCopy() CommutativeCrypto {
	mo2 := masseyOmuraCrypto{make([]byte, len(mo.P))}
	copy(mo2.P, mo.P)
	return mo2
}

// newMasseyOmuraCrypto creates a new instance, where messageSize is the maximum
// message size in bytes.
func newMasseyOmuraCrypto(messageSize int) masseyOmuraCrypto {
	bitlen := masseyomura.BitsMaxSize(messageSize)
	syskey, err := masseyomura.GenSysKey(rand.Reader, bitlen)
	if err != nil {
		assertf(false, "crypto", "something is wrong with rand, not sure what to do: %v", err)
	}
	return masseyOmuraCrypto{P: syskey.P.Bytes()}
}
