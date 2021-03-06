package masseyomura

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"testing"
)

func TestBasicMasseyOmura(t *testing.T) {
	MOTest(t, 1027, "i am a squid", []int{0})
}

func TestMasseyOmuraCommute(t *testing.T) {
	MOTest(t, 1027, "i am a squid", []int{0, 1})
}

func TestMasseyOmuraCommuteMany(t *testing.T) {
	MOTest(t, 1027, "hello", []int{0, 1, 2, 3, 4, 5})
}

func TestMasseyOmuraVerify(t *testing.T) {
	byteLen := 200
	syskey, err := GenSysKey(rand.Reader, BitsMaxSize(byteLen))
	if err != nil {
		t.Fatalf("%v", err)
	}
	err = VerifySysKey(syskey, BitsMaxSize(byteLen))
	if err != nil {
		t.Fatalf("%v", err)
	}
	err = VerifySysKey(syskey, BitsMaxSize(byteLen+1))
	if err == nil {
		t.Fatalf("expected verify to fail")
	}
	newsyskey := SystemKey{}
	newsyskey.P = new(big.Int)
	newsyskey.P.Add(syskey.P, big.NewInt(1))
	err = VerifySysKey(&newsyskey, BitsMaxSize(byteLen))
	if err == nil {
		t.Fatalf("expected to fail because not a prime")
	}
}

func TestMasseyOmuraLengths(t *testing.T) {
	lengths := []struct{ keyBits, msgBytes int }{
		{BitsMaxSize(100), 100},
		{BitsMaxSize(10), 10},
	}
	for _, l := range lengths {
		syskey, err := GenSysKey(rand.Reader, l.keyBits)
		if err != nil {
			t.Fatalf("%v", err)
		}
		var b []byte
		for i := 0; i < l.msgBytes; i++ {
			b = append(b, 13)
		}
		_, err = PrepareMsg(b, syskey.P.BitLen())
		if err != nil {
			t.Fatalf("%+v: %v", l, err)
		}
		b = append(b, 13)
		_, err = PrepareMsg(b, syskey.P.BitLen())
		if err == nil {
			t.Fatalf("%+v: error is nil but expected message too big", l)
		}
	}
}

// decrypt order is a permutation of {0, ..., n-1} for a system of n users.
// sample usage:
// MOTest(1027, "i am a very purple squid", []int{1, 3, 2, 5, 4, 0})
// MOTest(1027, "Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua", []int{2, 3, 6, 5, 0, 4, 1})
func MOTest(t *testing.T, bitlen int, message string, decryptorder []int) {
	syskey, err := GenSysKey(rand.Reader, bitlen)
	if err != nil {
		t.Fatalf("%v", err)
	}
	msg, err := PrepareMsg([]byte(message), syskey.P.BitLen())
	if err != nil {
		t.Fatalf("%v", err)
	}
	keys := make([]*PrivateKey, len(decryptorder))
	for i := 0; i < len(decryptorder); i++ {
		k, err := GenUserKey(rand.Reader, syskey)
		keys[i] = k
		if err != nil {
			t.Fatalf("%v", err)
		}
		msg = Encrypt(keys[i], msg)
	}
	for _, user := range decryptorder {
		msg = Decrypt(keys[user], msg)
	}
	result := string(ExtractMsg(msg))
	fmt.Println(result)
	if result != message {
		t.Fatalf("result (%v) != message (%v)", result, message)
	}
}
