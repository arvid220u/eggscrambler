package commutencrypt

import (
	"crypto/rand"
	"math/big"
  "errors"
  "io"
)

// generates p, q, g to base the system off of
//   g generates a group of order q (mod p)
//   this is a schnorr group
// given a bits q: bitsq is a strict lower bound on number of bits in p
// number of bits in p limits max size of message
func GenSysKey(random io.Reader, bitsq int) (syskey *SystemKey, err error) {
  var p big.Int
  var q *big.Int
  var r *big.Int
  isprimep := false
  for !isprimep { // while p not prime
    isprimeq := false
    for !isprimeq { // while q not prime
      q, err = rand.Prime(random, bitsq)
      if err != nil {
        return
      }
      isprimeq = q.ProbablyPrime(40)
    }
    r, err = randHelp(random, new(big.Int).SetInt64(10)) // TODO: CHOOSE UPPER BOUND ON r THAT IS NOT JUST 2 <= r < 20???
    if err != nil {
      return
    }
    p = *p.Mul(r, q)
    p.Mul(&p, new(big.Int).SetInt64(2)) // make r even
    p.Add(&p, new(big.Int).SetInt64(1)) // p = r * q + 1
    isprimep = p.ProbablyPrime(40)
  }
  var h *big.Int
  var g *big.Int
  validg := false
  for !validg {
    h, err = randHelp(random, q) //pick random 1 < h < p
    if err != nil {
      return
    }
    g = new(big.Int).Exp(h, r, &p) //g = h^r mod p
    validg = g.BitLen() > 1 // check g != 1 using number of bits
  }
  syskey = &SystemKey{P: &p, Q: q, G: g}
  return
}

// given system p, q, g generate a user's public/private keypair
func GenUserKey(random io.Reader, syskey *SystemKey) (privkey *PrivateKey, err error) {
  x, err := randHelp(random, syskey.Q) // random 1 <= x < Q
  if err != nil {
    return
  }
  privkey = &PrivateKey{ // TODO: is pointers correct
		PublicKey: PublicKey{
      P: syskey.P, Q: syskey.Q, G: syskey.G, // save for convenience
      Y: new(big.Int).Exp(syskey.G, x, syskey.P), // y = g^x mod p
		},
		X: x,
	}
  return
}
