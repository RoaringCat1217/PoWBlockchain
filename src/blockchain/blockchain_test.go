package blockchain

import (
	"crypto/rsa"
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"time"
)

// TestPostSafety - test whether tampering a Post can be detected by signature
func TestPostSafety(t *testing.T) {
	privateKey := GenerateKey()
	post := Post{
		User: &privateKey.PublicKey,
		Body: PostBody{
			Content:   "Hello World",
			Timestamp: time.Now().UnixNano(),
		},
	}
	post.Signature = Sign(privateKey, post.Body)
	if !post.Verify() {
		t.Fatal("Body is not signed correctly")
	}

	// encoding and then decoding should return the identical block
	encoded := post.EncodeBase64()
	decoded, _ := encoded.DecodeBase64()
	if !reflect.DeepEqual(post, decoded) {
		t.Fatal("post is not encoded or decoded correctly")
	}

	// tamper the content of post
	post.Body.Content = "Bye World"
	if post.Verify() {
		t.Fatal("signature fails to detect a tamper of content")
	}

	// tamper the timestamp of post
	post.Body.Content = "Hello World"
	post.Body.Timestamp = time.Now().UnixNano()
	if post.Verify() {
		t.Fatal("signature fails to detect a tamper of content")
	}
}

// TestBlockSafety - test whether tampering a Block can be detected by signature
func TestBlockSafety(t *testing.T) {
	users := make([]*rsa.PrivateKey, 0)
	posts := make([]Post, 0)
	for i := 0; i < 3; i++ {
		privateKey := GenerateKey()
		post := Post{
			User: &privateKey.PublicKey,
			Body: PostBody{
				Content:   fmt.Sprintf("Hello from %d", i),
				Timestamp: time.Now().UnixNano(),
			},
		}
		post.Signature = Sign(privateKey, post.Body)
		users = append(users, privateKey)
		posts = append(posts, post)
	}
	block := Block{
		Header: BlockHeader{
			PrevHash:  make([]byte, 256),
			Summary:   Hash(posts),
			Timestamp: time.Now().UnixNano(),
		},
		Posts: posts,
	}
	start := time.Now().UnixMilli()
	count := 0
mine:
	for {
		count++
		block.Header.Nonce = rand.Uint32()
		hash := Hash(block.Header)
		zeroBytes := TARGET / 8
		zeroBits := TARGET % 8
		// the first zeroBytes bytes of hash must be zero
		for i := 0; i < zeroBytes; i++ {
			if hash[i] != 0 {
				continue mine
			}
		}
		// and then zeroBits bits of hash must be zero
		if zeroBits > 0 {
			nextByte := hash[zeroBytes]
			nextByte = nextByte >> (8 - zeroBits)
			if nextByte != 0 {
				continue mine
			}
		}
		break
	}
	end := time.Now().UnixMilli()
	t.Logf("used %d ms (%d iterations) to mine a block", end-start, count)

	if !block.Verify() {
		t.Fatalf("the mined block is not valid")
	}

	// encoding and then decoding should return the identical block
	encoded := block.EncodeBase64()
	decoded, _ := encoded.DecodeBase64()
	if !reflect.DeepEqual(block, decoded) {
		t.Fatal("block is not encoded or decoded correctly")
	}

	// delete a post
	block.Posts = posts[:2]
	if block.Verify() {
		t.Fatalf("fails to detect a tamper of posts")
	}

	// tamper PrevHash
	block.Header.PrevHash[0] = 1
	if block.Verify() {
		t.Fatalf("fails to detect a tamper of previous block's hash")
	}
}
