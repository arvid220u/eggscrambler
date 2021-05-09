package commutencrypt

import (
	"crypto/rand"
	"math/big"
  "errors"
  "io"
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
