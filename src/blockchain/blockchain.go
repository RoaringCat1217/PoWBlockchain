package blockchain

import (
	"bytes"
	"crypto/rsa"
	"encoding/base64"
)

// TARGET - A valid block hash has its first TARGET bits be zero.
const TARGET = 20

type PostBody struct {
	Content   string
	Timestamp int64
}

type Post struct {
	User      *rsa.PublicKey
	Signature []byte
	Body      PostBody
}

func (p *Post) Verify() bool {
	return Verify(p.User, p.Body, p.Signature)
}

type BlockHeader struct {
	PrevHash  []byte
	Summary   []byte
	Timestamp int64
	Nonce     uint32
}

type Block struct {
	Header BlockHeader
	Posts  []Post
}

func (b *Block) Verify() bool {
	hash := Hash(b.Header)
	zeroBytes := TARGET / 8
	zeroBits := TARGET % 8
	// the first zeroBytes bytes of hash must be zero
	for i := 0; i < zeroBytes; i++ {
		if hash[i] != 0 {
			return false
		}
	}
	// and then zeroBits bits of hash must be zero
	if zeroBits > 0 {
		nextByte := hash[zeroBytes]
		nextByte = nextByte >> (8 - zeroBits)
		if nextByte != 0 {
			return false
		}
	}
	// verify the summary
	if !bytes.Equal(b.Header.Summary, Hash(b.Posts)) {
		return false
	}
	// verify all posts
	for _, post := range b.Posts {
		if !post.Verify() {
			return false
		}
	}
	return true
}

// PostBase64 base64-encoded Post to support marshalling to json
type PostBase64 struct {
	User      string `json:"user"`
	Content   string `json:"content"`
	Timestamp int64  `json:"timestamp"`
	Signature string `json:"signature"`
}

func (p *Post) EncodeBase64() PostBase64 {
	encoded := PostBase64{
		User:      base64.StdEncoding.EncodeToString(PublicKeyToBytes(p.User)),
		Content:   p.Body.Content,
		Timestamp: p.Body.Timestamp,
		Signature: base64.StdEncoding.EncodeToString(p.Signature),
	}
	return encoded
}

func (p *PostBase64) DecodeBase64() (Post, error) {
	decoded := Post{
		Body: PostBody{
			Content:   p.Content,
			Timestamp: p.Timestamp,
		},
	}
	// decode public key
	bytes, err := base64.StdEncoding.DecodeString(p.User)
	if err != nil {
		return Post{}, err
	}
	publicKey, err := PublicKeyFromBytes(bytes)
	if err != nil {
		return Post{}, err
	}
	decoded.User = publicKey
	// decode signature
	bytes, err = base64.StdEncoding.DecodeString(p.Signature)
	if err != nil {
		return Post{}, err
	}
	decoded.Signature = bytes
	return decoded, nil
}

// BlockBase64 base64-encoded Block to support marshalling to json
type BlockBase64 struct {
	PrevHash  string       `json:"prev-hash"`
	Summary   string       `json:"summary"`
	Timestamp int64        `json:"timestamp"`
	NPosts    int          `json:"n-posts"`
	Nonce     uint32       `json:"nonce"`
	Posts     []PostBase64 `json:"posts"`
}

func (b *Block) EncodeBase64() BlockBase64 {
	encoded := BlockBase64{
		PrevHash:  base64.StdEncoding.EncodeToString(b.Header.PrevHash),
		Summary:   base64.StdEncoding.EncodeToString(b.Header.Summary),
		Timestamp: b.Header.Timestamp,
		Nonce:     b.Header.Nonce,
	}
	for _, post := range b.Posts {
		encoded.Posts = append(encoded.Posts, post.EncodeBase64())
	}
	return encoded
}

func (b *BlockBase64) DecodeBase64() (Block, error) {
	decoded := Block{
		Header: BlockHeader{
			Timestamp: b.Timestamp,
			Nonce:     b.Nonce,
		},
	}

	bytes, err := base64.StdEncoding.DecodeString(b.PrevHash)
	if err != nil {
		return Block{}, err
	}
	decoded.Header.PrevHash = bytes

	bytes, err = base64.StdEncoding.DecodeString(b.Summary)
	if err != nil {
		return Block{}, err
	}
	decoded.Header.Summary = bytes

	for _, post := range b.Posts {
		decodedPost, err := post.DecodeBase64()
		if err != nil {
			return Block{}, err
		}
		decoded.Posts = append(decoded.Posts, decodedPost)
	}
	return decoded, nil
}
