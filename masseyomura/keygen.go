package masseyomura

import (
	"crypto/rand"
	"errors"
	"io"
	"math/big"
)

// generates prime p to base the system off of
// given a bits p: bitsp is the number of bits in p
// note that size of p is limit on message size
func GenSysKey(random io.Reader, bitsp int) (syskey *SystemKey, err error) {
	var p *big.Int
	isprimep := false
	for !isprimep { // while p not prime
		p, err = rand.Prime(random, bitsp)
		if err != nil {
			return
		}
		isprimep = p.ProbablyPrime(40) // TODO: is 40 safe enough?
	}
	syskey = &SystemKey{P: p}
	return
}

// given system p generate a user's public/private keypair
// random 0 < e < p-1, gcd (e, p-1) = 1
// calculate d such that e*d = 1 (mod p-1)
func GenUserKey(random io.Reader, syskey *SystemKey) (privkey *PrivateKey, err error) {
	tmp := new(big.Int).SetInt64(1)
	p1 := tmp.Sub(syskey.P, tmp) // p - 1
	gcd := new(big.Int).SetInt64(2)
	var e *big.Int
	for gcd.BitLen() != 1 {
		e, err = randHelp(random, p1) // random 1 <= x < P - 1
		if err != nil {
			return
		}
		gcd.GCD(nil, nil, e, p1) // gcd(e, p1) = 1 => e is valid
	}
	d := p1.ModInverse(e, p1)
	if d == nil {
		return nil, errors.New("impossible no inverse")
	}
	privkey = &PrivateKey{
		SystemKey: *syskey,
		E:         e,
		D:         d,
	}
	return
}
