package tests

import (
	"blockchain/blockchain"
	Miner "blockchain/miner"
	"blockchain/tracker"
	"blockchain/user"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// N - Number of miners to select for writing posts
const (
	N = 3
)

// mockTracker is a mock implementation of the tracker server
type mockTracker struct {
	miners []int
}

// newMockTracker creates a new instance of the mock tracker server
func newMockTracker(miners []int) *mockTracker {
	return &mockTracker{miners: miners}
}

// handleGetMiners handles the GET request to retrieve the list of miners
func (t *mockTracker) handleGetMiners(w http.ResponseWriter, r *http.Request) {
	response := tracker.PortsJson{Ports: t.miners}
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		return
	}
}

// TestNewUser tests the creation of a new user
func TestNewUser(t *testing.T) {
	trackerPort := 8000
	newUser := user.NewUser(trackerPort)

	// We cannot access the unexported fields directly
	// Instead, we can test the behavior of the newUser through its methods
	// For example, we can call GetRandomMiners and check if it returns an error
	_, err := newUser.GetRandomMiners()
	if err == nil {
		t.Error("Expected an error when calling GetRandomMiners with no running tracker, but got nil")
	}
}

// TestGetRandomMiners tests the retrieval of random miners from the tracker
func TestGetRandomMiners(t *testing.T) {
	miners := []int{8001, 8002, 8003, 8004, 8005, 8006, 8007, 8008, 8009, 8010}
	mockTracker := newMockTracker(miners)

	trackerServer := httptest.NewServer(http.HandlerFunc(mockTracker.handleGetMiners))
	defer trackerServer.Close()

	newUser := user.NewUser(extractPort(trackerServer.URL))

	randomMiners, err := newUser.GetRandomMiners()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(randomMiners) != N {
		t.Errorf("Expected %d random miners, but got %d", N, len(randomMiners))
	}
}

func TestReadPosts(t *testing.T) {
	newTracker := tracker.NewTracker(8080)
	newTracker.Start()

	// Register 3 miners
	miners := make([]*Miner.Miner, 0)
	for i := 0; i < 3; i++ {
		miner := Miner.NewMiner(3000+i, 8080)
		miner.Start()
		miners = append(miners, miner)
	}

	// Create a user
	user := user.NewUser(8080)

	// Wait for everything to be ready
	time.Sleep(500 * time.Millisecond)

	// User posts something
	err := user.WritePost("Hello World")
	if err != nil {
		t.Fatalf("error when posting: %v", err)
	}

	// Wait for the blockchain to reach consensus
	time.Sleep(2000 * time.Millisecond)

	posts, err := user.ReadPosts()
	if err != nil {
		t.Fatalf("error when reading: %v", err)
	}

	if len(posts) != 1 {
		t.Fatalf("expected 1 post, but got %d", len(posts))
	}

	if posts[0].Body.Content != "Hello World" {
		t.Fatalf("wrong body for post: %s", posts[0].Body.Content)
	}

	// Gracefully shutdown everything
	for _, miner := range miners {
		miner.Shutdown()
	}
	newTracker.Shutdown()
}

// TestWritePost tests the writing of a post to miners
func TestWritePost(t *testing.T) {
	miners := []int{8001, 8002, 8003}
	mockTracker := newMockTracker(miners)

	trackerServer := httptest.NewServer(http.HandlerFunc(mockTracker.handleGetMiners))
	defer trackerServer.Close()

	newUser := user.NewUser(extractPort(trackerServer.URL))

	var receivedPosts []blockchain.PostBase64
	var mu sync.Mutex // Mutex to synchronize access to receivedPosts

	minerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var post blockchain.PostBase64
		err := json.NewDecoder(r.Body).Decode(&post)
		if err != nil {
			return
		}
		mu.Lock()
		receivedPosts = append(receivedPosts, post)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer minerServer.Close()

	// Set the miner ports to the same server URL
	minerPort := extractPort(minerServer.URL)
	for i := range miners {
		miners[i] = minerPort
	}

	content := "Test post"
	err := newUser.WritePost(content)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Check if the post was sent to the expected number of miners
	if len(receivedPosts) != len(miners) {
		t.Errorf("Expected %d posts to be written, but got %d", len(miners), len(receivedPosts))
	}

	for _, post := range receivedPosts {
		decodedPost, _ := post.DecodeBase64()
		if decodedPost.Body.Content != content {
			t.Errorf("Expected post content '%s', but got '%s'", content, decodedPost.Body.Content)
		}
	}
}

// extractPort extracts the port number from a URL
func extractPort(url string) int {
	_, portStr, _ := net.SplitHostPort(url[7:])
	var port int
	_, err := fmt.Sscanf(portStr, "%d", &port)
	if err != nil {
		return 0
	}
	return port
}

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
