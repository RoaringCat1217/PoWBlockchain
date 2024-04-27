package blockchain

import "crypto/rsa"

type Post struct {
	User      *rsa.PublicKey
	Content   string
	Timestamp int64
	Signature []byte
}

type BlockHeader struct {
	PrevHash  []byte
	Summary   []byte
	Timestamp int64
	Nonce     uint32
}

type Block struct {
	header BlockHeader
	NPosts int
	Posts  []Post
}
