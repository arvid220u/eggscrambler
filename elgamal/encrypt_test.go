package elgamal

import (
	"crypto/rand"
	"fmt"
	"testing"
)

func TestBasic(t *testing.T) {
	syskey, err := GenSysKey(rand.Reader, 140)
	if err != nil {
		panic(err)
	}
	privkey, err := GenUserKey(rand.Reader, syskey)
	if err != nil {
		panic(err)
	}
	msg, err := PrepareMsg([]byte("i am a squid"), syskey.P.BitLen())
	if err != nil {
		panic(err)
	}
	//var T Table
	K, C, err := Encrypt(rand.Reader, &privkey.PublicKey, msg)
	if err != nil {
		panic(err)
	}
	D, err := Decrypt(privkey, K, C)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(ExtractMsg(D)))
}

func TestBasicCommute(t *testing.T) {
	syskey, err := GenSysKey(rand.Reader, 140)
	if err != nil {
		panic(err)
	}
	privkey1, err := GenUserKey(rand.Reader, syskey)
	if err != nil {
		panic(err)
	}
	privkey2, err := GenUserKey(rand.Reader, syskey)
	if err != nil {
		panic(err)
	}
	msg, err := PrepareMsg([]byte("i am a squid"), syskey.P.BitLen())
	if err != nil {
		panic(err)
	}
	K1, C1, err := Encrypt(rand.Reader, &privkey1.PublicKey, msg)
	if err != nil {
		panic(err)
	}
	K2, C, err := Encrypt(rand.Reader, &privkey2.PublicKey, C1)
	if err != nil {
		panic(err)
	}
	D2, err := Decrypt(privkey1, K1, C)
	if err != nil {
		panic(err)
	}
	D, err := Decrypt(privkey2, K2, D2)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(ExtractMsg(D)))
}
