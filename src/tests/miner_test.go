package tests

import (
	"blockchain/blockchain"
	Miner "blockchain/miner"
	Tracker "blockchain/tracker"
	"blockchain/user"
	User "blockchain/user"
	"bytes"
	"encoding/json"
	"fmt"
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

	newUser := user.NewUser(extractPort(trackerServer.URL))

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
	tracker := Tracker.NewTracker(8081)
	tracker.Start()
	defer tracker.Shutdown()

	// Create a legitimate miner
	legitimateMiner := Miner.NewMiner(3001, 8081)
	legitimateMiner.Start()
	defer legitimateMiner.Shutdown()

	// Create a malicious miner
	maliciousMiner := Miner.NewMiner(3002, 8081)
	maliciousMiner.Start()
	defer maliciousMiner.Shutdown()

	// Create a user
	user := User.NewUser(8081)

	// User posts a legitimate message
	legitimateContent := "Legitimate post"
	err := user.WritePost(legitimateContent)
	if err != nil {
		t.Fatalf("error when posting: %v", err)
	}

	// Wait for the legitimate miner to mine the block
	time.Sleep(5000 * time.Millisecond)

	// Malicious miner attempts to replay the user's post
	posts, err := user.ReadPosts()
	if err != nil {
		t.Fatalf("error reading user's posts: %v", err)
	}
	if len(posts) == 0 {
		t.Fatalf("user has no posts")
	}
	lastPost := posts[len(posts)-1]
	lastPostEncoded := lastPost.EncodeBase64()
	lastPostJSON, _ := json.Marshal(lastPostEncoded)
	resp, err := http.Post(fmt.Sprintf("http://localhost:%d/write", 3002), "application/json", bytes.NewBuffer(lastPostJSON))
	if err != nil {
		t.Fatalf("error replaying user's post: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status Bad Request, but got %d", resp.StatusCode)
	}

	// Malicious miner attempts to fake the user's post
	privateKey := blockchain.GenerateKey()
	fakePost := blockchain.Post{
		User: &privateKey.PublicKey,
		Body: blockchain.PostBody{
			Content:   "Fake post",
			Timestamp: time.Now().UnixNano(),
		},
	}
	fakePost.Signature = blockchain.Sign(blockchain.GenerateKey(), fakePost.Body)
	fakePostEncoded := fakePost.EncodeBase64()
	fakePostJSON, _ := json.Marshal(fakePostEncoded)
	resp, err = http.Post(fmt.Sprintf("http://localhost:%d/write", 3002), "application/json", bytes.NewBuffer(fakePostJSON))
	if err != nil {
		t.Fatalf("error faking user's post: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status Bad Request, but got %d", resp.StatusCode)
	}

	// Wait for the miners to sync
	time.Sleep(5000 * time.Millisecond)

	// Check that the legitimate post is on the blockchain
	posts, err = user.ReadPosts()
	if err != nil {
		t.Fatalf("error when reading: %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("expected 1 post on the blockchain, got %d", len(posts))
	}
	if posts[0].Body.Content != legitimateContent {
		t.Fatalf("wrong body for post: got %s, expected %s", posts[0].Body.Content, legitimateContent)
	}
}
