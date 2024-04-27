package blockchain

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/binary"
	"encoding/gob"
	"math/big"
)

func Hash(object any) []byte {
	// first serialize object to bytes
	gob.Register(object)
	var buffer bytes.Buffer
	encoder := gob.NewEncoder(&buffer)
	err := encoder.Encode(object)
	if err != nil {
		panic(err)
	}
	// next use SHA256 to hash the bytes
	hash := sha256.Sum256(buffer.Bytes())
	return hash[:]
}

func GenerateKey() *rsa.PrivateKey {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	return privateKey
}

func PublicKeyToBytes(publicKey rsa.PublicKey) []byte {
	buffer := make([]byte, 4)
	binary.LittleEndian.PutUint32(buffer, uint32(publicKey.E))
	buffer = append(buffer, publicKey.N.Bytes()...)
	return buffer
}

func PublicKeyFromBytes(buffer []byte) *rsa.PublicKey {
	E := int(binary.LittleEndian.Uint32(buffer[:4]))
	N := new(big.Int)
	N.SetBytes(buffer[4:])
	return &rsa.PublicKey{N: N, E: E}
}

func Sign(privateKey *rsa.PrivateKey, object any) []byte {
	hash := Hash(object)
	signature, err := rsa.SignPKCS1v15(nil, privateKey, crypto.SHA256, hash)
	if err != nil {
		panic(err)
	}
	return signature
}

func Verify(publicKey *rsa.PublicKey, object any, signature []byte) bool {
	hash := Hash(object)
	err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, hash, signature)
	return err == nil
}
