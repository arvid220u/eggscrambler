package elgamal

// This is an implementation of a commutative version of the ElGamal encryption scheme, based on
// this paper: https://doi.org/10.1109/ISIC.2012.6449730
// It turned out that ElGamal does not suit our purposes, since it requires users to record a table T
// for each message, which would reveal identities in our case. The implementation is kept here for reference.
// Usage is demonstrated in encrypt_test.go.

import (
	"crypto/rand"
	"io"
	"math/big"
)

type SystemKey struct { // systemwide values
	P, Q, G *big.Int
}

type PublicKey struct { // user's public key
	P, Q, G, Y *big.Int
}

type PrivateKey struct { // user's private key
	PublicKey
	X *big.Int // Y = G^X mod P. 1 <= X < Q
}

//type Table map[int]Row

//type Row struct { // table row for encryption scheme
//  Y, K, C *big.Int // TODO
//}

// generate a number from 1 to Q - 1, inclusive
// alt not shift here and just go for 1 to Q - 1 might be more efficient
// TODO: shift likely unnecessary; just panic if generates 0?
func randHelp(random io.Reader, Q *big.Int) (r *big.Int, err error) {
	ONE := new(big.Int).SetInt64(1)
	r, err = rand.Int(random, new(big.Int).Sub(Q, ONE))
	r.Add(r, ONE)
	return
}
