package masseyomura

import (
	"crypto/rand"
	"io"
	"math/big"
)

type SystemKey struct { // systemwide value
	P *big.Int
}

type PrivateKey struct { // user's private encryption/decryption keys
	SystemKey
	E, D *big.Int
}

// generate a number from 1 to Q - 1, inclusive
// alt not shift here and just go for 1 to Q - 1 might be more efficient
// TODO: shift likely unnecessary; just panic if generates 0?
func randHelp(random io.Reader, Q *big.Int) (r *big.Int, err error) {
	ONE := new(big.Int).SetInt64(1)
	r, err = rand.Int(random, new(big.Int).Sub(Q, ONE))
	r.Add(r, ONE)
	return
}
