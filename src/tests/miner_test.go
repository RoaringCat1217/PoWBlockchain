package tests

import (
	"blockchain/blockchain"
	Miner "blockchain/miner"
	Tracker "blockchain/tracker"
	User "blockchain/user"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestMaliciousUserPost tests the behavior of the system when a malicious user tries to submit a tampered post
func TestMaliciousUserPost(t *testing.T) {
	// Setup mock miners to capture posts sent to them
	miners := []int{8001, 8002, 8003}
	mockTracker := newMockTracker(miners)

	trackerServer := httptest.NewServer(http.HandlerFunc(mockTracker.handleGetMiners))
	defer trackerServer.Close()

	newUser := User.NewUser(extractPort(trackerServer.URL))

	var receivedPosts []blockchain.PostBase64

	minerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var post blockchain.PostBase64
		err := json.NewDecoder(r.Body).Decode(&post)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		// Attempt to decode the post and verify its integrity
		decodedPost, err := post.DecodeBase64()
		if err != nil || !decodedPost.Verify() {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		receivedPosts = append(receivedPosts, post)
		w.WriteHeader(http.StatusOK)
	}))
	defer minerServer.Close()

	// Create a valid post and tamper with it
	validPost := blockchain.Post{
		User: &blockchain.GenerateKey().PublicKey,
		Body: blockchain.PostBody{
			Content:   "Legitimate content",
			Timestamp: time.Now().UnixNano(),
		},
	}
	validPost.Signature = blockchain.Sign(blockchain.GenerateKey(), validPost.Body) // Sign with a different key

	tamperedPostContent := "Malicious content"
	// Intentionally breaking the integrity by not updating the signature
	validPost.Body.Content = tamperedPostContent

	// Send tampered post
	err := newUser.WritePost(tamperedPostContent)
	if err == nil {
		t.Errorf("Expected error when writing a tampered post, but got nil")
	}

	// Verify no post was accepted by any miner
	if len(receivedPosts) > 0 {
		t.Errorf("Expected no posts to be accepted, but %d were accepted", len(receivedPosts))
	}
}

// TestMaliciousMiner - test whether the system rejects a block when a worker fakes or replays a user's post
func TestMaliciousMiner(t *testing.T) {
	tracker := Tracker.NewTracker(8080)
	tracker.Start()

	// Create one legitimate miner
	miner := Miner.NewMiner(3000, 8080)
	miner.Start()
	// wait for everything to start
	time.Sleep(1000 * time.Millisecond)
	// post one message
	privateKey := blockchain.GenerateKey()
	post := blockchain.Post{
		User: &privateKey.PublicKey,
		Body: blockchain.PostBody{
			Content:   "Legitimate content",
			Timestamp: time.Now().UnixNano(),
		},
	}
	post.Signature = blockchain.Sign(privateKey, post.Body)
	postBase64 := post.EncodeBase64()
	postJSON, _ := json.Marshal(postBase64)
	resp, err := http.Post("http://localhost:3000/write", "application/json", bytes.NewReader(postJSON))
	if err != nil {
		t.Fatalf("error when writing blockchain: %v\n", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("error when writing blockchain: %v\n", err)
	}
	resp.Body.Close()
	// wait for a block to be mined
	time.Sleep(10000 * time.Millisecond)

	// tries to attack miner's /sync API with a fake post
	fakePost, _ := postBase64.DecodeBase64()
	fakePost.Body.Content = "Malicious content"
	fakePostBase64 := fakePost.EncodeBase64()
	syncReq := Miner.PostsJson{}
	syncReq.Posts = append(syncReq.Posts, fakePostBase64)
	fakePostJson, _ := json.Marshal(syncReq)
	resp, _ = http.Post("http://localhost:3000/sync", "application/json", bytes.NewReader(fakePostJson))
	resp.Body.Close()

	// tries to attack miner's /sync API with a replayed post
	resp, _ = http.Post("http://localhost:3000/sync", "application/json", bytes.NewReader(postJSON))
	resp.Body.Close()

	// tries to attack miner's /broadcast API with a very long, fake blockchain
	fakeBlockchain := make([]blockchain.BlockBase64, 100)
	fakeBroadcastReq := Miner.BlockChainJson{Blockchain: fakeBlockchain}
	fakeBroadcastJson, _ := json.Marshal(fakeBroadcastReq)
	resp, _ = http.Post("http://localhost:3000/broadcast", "application/json", bytes.NewReader(fakeBroadcastJson))
	resp.Body.Close()

	time.Sleep(10000 * time.Millisecond)
	user := User.NewUser(8080)
	posts, err := user.ReadPosts()
	if err != nil {
		t.Fatalf("error when reading posts: %v\n", err)
	}
	// should have only 1 legitimate post
	if len(posts) != 1 {
		t.Fatalf("wrong number of posts\n")
	}
	if posts[0].Body.Content != "Legitimate content" {
		t.Fatalf("wrong content of posts\n")
	}

	// clean up
	miner.Shutdown()
	tracker.Shutdown()
}
