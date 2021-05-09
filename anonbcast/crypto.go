package anonbcast

import (
	"crypto/rand"
	"github.com/arvid220u/6.824-project/commutencrypt"
	"math/big"
)

type CommutativeCrypto interface {
	// PrepareMsg converts a plaintext message to a plaintext big.Int.
	// Returns an error if the message is too long.
	PrepareMsg(msg []byte) (*big.Int, error)

	// Encrypt encrypts prevC using pubkey, returning (K, C, error).
	Encrypt(pubkey *commutencrypt.PublicKey, prevC *big.Int) (*big.Int, *big.Int)

	// Decrypt decrypts prevD using privkey and K, returning the plaintext.
	// Returns an error if the private key does not work.
	Decrypt(privkey *commutencrypt.PrivateKey, K *big.Int, prevD *big.Int) (*big.Int, error)

	// ExtractMsg reverses PrepareMsg
	ExtractMsg(D *big.Int) []byte

	// GenKey generates a private key
	GenKey() *commutencrypt.PrivateKey
}

type commutEncrypt struct {
	syskey *commutencrypt.SystemKey
}

func (c *commutEncrypt) PrepareMsg(msg []byte) (*big.Int, error) {
	return commutencrypt.PrepareMsg(msg, c.syskey.P.BitLen())
}

func (c *commutEncrypt) Encrypt(pubkey *commutencrypt.PublicKey, prevC *big.Int) (*big.Int, *big.Int) {
	K, C, err := commutencrypt.Encrypt(rand.Reader, pubkey, prevC)
	if err != nil {
		assertf(false, "don't know what to do when encrypt fails: %v", err)
	}
	return K, C
}

func (c *commutEncrypt) Decrypt(privkey *commutencrypt.PrivateKey, K *big.Int, prevD *big.Int) (*big.Int, error) {
	return commutencrypt.Decrypt(privkey, K, prevD)
}

func (c *commutEncrypt) ExtractMsg(D *big.Int) []byte {
	return commutencrypt.ExtractMsg(D)
}

func (c *commutEncrypt) GenKey() *commutencrypt.PrivateKey {
	privkey, err := commutencrypt.GenUserKey(rand.Reader, c.syskey)
	if err != nil {
		assertf(false, "don't know what to do when key gen fails: %v", err)
	}
	return privkey
}

func newCommutEncrypt() *commutEncrypt {
	// TODO: set max length?
	syskey, err := commutencrypt.GenSysKey(rand.Reader, 140)
	if err != nil {
		assertf(false, "error is %v, don't know what to do", err)
	}
	return &commutEncrypt{syskey}
}
