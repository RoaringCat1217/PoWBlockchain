package tests

import (
	"blockchain/blockchain"
	Miner "blockchain/miner"
	Tracker "blockchain/tracker"
	User "blockchain/user"
	"crypto/rsa"
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"time"
)

// TestPostSafety - test whether tampering a Post can be detected by signature
func TestPostSafety(t *testing.T) {
	privateKey := blockchain.GenerateKey()
	post := blockchain.Post{
		User: &privateKey.PublicKey,
		Body: blockchain.PostBody{
			Content:   "Hello World",
			Timestamp: time.Now().UnixNano(),
		},
	}
	post.Signature = blockchain.Sign(privateKey, post.Body)
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
	posts := make([]blockchain.Post, 0)
	for i := 0; i < 3; i++ {
		privateKey := blockchain.GenerateKey()
		post := blockchain.Post{
			User: &privateKey.PublicKey,
			Body: blockchain.PostBody{
				Content:   fmt.Sprintf("Hello from %d", i),
				Timestamp: time.Now().UnixNano(),
			},
		}
		post.Signature = blockchain.Sign(privateKey, post.Body)
		users = append(users, privateKey)
		posts = append(posts, post)
	}
	block := blockchain.Block{
		Header: blockchain.BlockHeader{
			PrevHash:  make([]byte, 32),
			Summary:   blockchain.Hash(posts),
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
		hash := blockchain.Hash(block.Header)
		zeroBytes := blockchain.TARGET / 8
		zeroBits := blockchain.TARGET % 8
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

// TestCompleteInteractions - orchestrate complete interactions between a tracker, users and miners
func TestCompleteInteractions(t *testing.T) {
	tracker := Tracker.NewTracker(8080)
	tracker.Start()
	// register 6 miners
	miners := make([]*Miner.Miner, 0)
	for i := 0; i < 6; i++ {
		miner := Miner.NewMiner(3000+i, 8080)
		miner.Start()
		miners = append(miners, miner)
	}
	// register 6 users
	users := make([]*User.User, 0)
	for i := 0; i < 6; i++ {
		users = append(users, User.NewUser(8080))
	}
	// wait for everything to be ready
	time.Sleep(500 * time.Millisecond)

	// each user posts something
	for i := 0; i < 6; i++ {
		err := users[i].WritePost(fmt.Sprintf("Hello world from %d", i))
		if err != nil {
			t.Fatalf("error when posting: %v", err)
		}
	}

	// wait for the blockchain to reach consensus
	time.Sleep(20000 * time.Millisecond)
	posts, err := users[0].ReadPosts()
	if err != nil {
		t.Fatalf("error when reading: %v", err)
	}
	if len(posts) != 6 {
		t.Fatalf("not enough posts on the blockchain")
	}
	for i := 0; i < 6; i++ {
		if posts[i].Body.Content != fmt.Sprintf("Hello world from %d", i) {
			t.Fatalf("wrong body for post %d: %s", i, posts[i].Body.Content)
		}
	}
	// gracefully shutdown everything
	for _, miner := range miners {
		miner.Shutdown()
	}
	tracker.Shutdown()
}
