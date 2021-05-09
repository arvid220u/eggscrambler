package commutencrypt

import (
	"crypto/rand"
	"math/big"
  "errors"
  "io"
  "fmt"
)

// prepare []byte message for encryption
// plen = PublicKey.P.BitLen()
func PrepareMsg(msg []byte, plen int) (*big.Int, error) {
  maxsize := (plen + 7) / 8 - 2 // TODO: check correctness of relation
  if len(msg) > maxsize {
    return nil, fmt.Errorf("Message size %d too long for %d bit key", len(msg), plen)
  }
  return new(big.Int).SetBytes(pad(msg, maxsize)), nil
}

// scheme: user saves K (for decrypt). forward C and T to the next encrypter
// return: (K, C, error)
func Encrypt(random io.Reader, pubkey *PublicKey, prevC *big.Int) (*big.Int, *big.Int, error) {
  //if _, ok := T[me]; ok {
  //  return nil, nil, fmt.Errorf("User %d has already encrypted this message", me)
  //}
  r, err := randHelp(random, pubkey.Q)
  if err != nil {
    return nil, nil, err
  }
  K := new(big.Int).Exp(pubkey.G, r, pubkey.P) // K = g^r mod p
  C := new(big.Int).Exp(pubkey.Y, r, pubkey.P)
  C.Mul(C, prevC) // C = prevC * y^r (mod p)
  //T[me] = Row{Y: pubkey.Y, K: K, C: C} // TODO: what info is actually necessary lol
  return K, C, nil
}

// scheme: user forward D and T to the next decrypter
// return: (D, error)
func Decrypt(privkey *PrivateKey, K *big.Int, prevD *big.Int) (*big.Int, error) {
  //if len(T) == 0 {
  //  return nil, errors.New("Message already fully decrypted")
  //}
  //if _, ok := T[me]; ok {
    denom:= new(big.Int).Exp(K, privkey.X, privkey.P)
    if denom.ModInverse(denom, privkey.P) == nil {
  		return nil, errors.New("Decrypt received invalid private key")
  	}
    D := new(big.Int).Mul(prevD, denom)
    D.Mod(D, privkey.P) // D = prevD / K^X. K is saved K from encrypt
    //delete(T, me)
    return D, nil
  //} else {
  //  return nil, fmt.Errorf("User %d has already decrypted this message", me)
  //}
}

// extract message from fully decrypted
func ExtractMsg(D *big.Int) []byte {
  msg := D.Bytes()
  return unpad(msg)
}

// dummy pad to reach targetlen
func pad(msg []byte, targetlen int) []byte {
  fmt.Println(msg)
  if len(msg) > targetlen {
    panic("cannot make message shorter through padding")
  }
  if len(msg) == targetlen {
    panic("message too long to encrypt")
  }
  padded := make([]byte, targetlen - 1)
  for i:= 0; i < targetlen-2-len(msg); i++ {
    padded[i] = 1
  }
  padded[targetlen-2-len(msg)] = 0
  copy(padded[targetlen-len(msg)-1:], msg)
  fmt.Println(padded)
  return padded
}

// dummy unpad
func unpad(msg []byte) []byte {
  fmt.Println(msg)
  for i:= 0; i < len(msg); i++ {
    if msg[i] == 0 {
      fmt.Println(msg[i+1:])
      return msg[i+1:]
    }
  }
  panic("no 0 in message")
}
