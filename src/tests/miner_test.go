package tests

import (
	"blockchain/blockchain"
	"blockchain/user"
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
