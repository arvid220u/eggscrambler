package masseyomura

import (
	"fmt"
	"math/big"
)

func bytesMaxSize(bits int) int {
	// TODO: is this actually correct?
	maxsize := (bits-1)/8 - 1 // message has strictly fewer bits than p
	return maxsize
}

// BitsMaxSize reverse bytesMaxSize
func BitsMaxSize(bytes int) int {
	return 8*(bytes+1) + 1
}

// PrepareMsg prepares a []byte message for encryption
// plen = PublicKey.P.BitLen()
func PrepareMsg(msg []byte, plen int) (*big.Int, error) {
	maxsize := bytesMaxSize(plen)
	if len(msg) > maxsize {
		return nil, fmt.Errorf("message size %d too long for %d bit key", len(msg), plen)
	}
	return new(big.Int).SetBytes(pad(msg, maxsize+1)), nil // TODO: is maxsize+1 allowed here?
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
	padded := make([]byte, targetlen)
	for i := 0; i < targetlen-1-len(msg); i++ {
		padded[i] = 1
	}
	padded[targetlen-1-len(msg)] = 0
	copy(padded[targetlen-len(msg):], msg)
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
