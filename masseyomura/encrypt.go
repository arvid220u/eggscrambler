package masseyomura

import (
	"fmt"
	"math/big"
)

// PrepareMsg prepares a []byte message for encryption
// plen = PublicKey.P.BitLen()
func PrepareMsg(msg []byte, plen int) (*big.Int, error) {
	maxsize := (plen-1)/8 - 1 // message has strictly fewer bits than p
	if len(msg) > maxsize {
		return nil, fmt.Errorf("message size %d too long for %d bit key", len(msg), plen)
	}
	return new(big.Int).SetBytes(pad(msg, maxsize)), nil
}

// Encrypt encrypts using privkey.E: result = message ^ privkey.E (mod p)
func Encrypt(privkey *PrivateKey, prevC *big.Int) *big.Int {
	return new(big.Int).Exp(prevC, privkey.E, privkey.P)
}

// Decrypt decrypts using privkey.D: result = ciphertext ^ privkey.D (mod p)
func Decrypt(privkey *PrivateKey, prevD *big.Int) *big.Int {
	return new(big.Int).Exp(prevD, privkey.D, privkey.P)
}

// ExtractMsg extracts a message from fully decrypted
func ExtractMsg(D *big.Int) []byte {
	msg := D.Bytes()
	return unPad(msg)
}

// pad pads to reach targetlen
func pad(msg []byte, targetlen int) []byte {
	if len(msg) > targetlen {
		panic("cannot make message shorter through padding")
	}
	if len(msg) == targetlen {
		panic("message too long to encrypt")
	}
	padded := make([]byte, targetlen-1)
	for i := 0; i < targetlen-2-len(msg); i++ {
		padded[i] = 1
	}
	padded[targetlen-2-len(msg)] = 0
	copy(padded[targetlen-len(msg)-1:], msg)
	return padded
}

// unPad undoes pad
func unPad(msg []byte) []byte {
	for i := 0; i < len(msg); i++ {
		if msg[i] == 0 {
			return msg[i+1:]
		}
	}
	panic("no 0 in message")
}
