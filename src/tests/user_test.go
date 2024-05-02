package tests

import (
	"blockchain/blockchain"
	"blockchain/miner"
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

// TestReadPosts tests the retrieval of posts from miners
func TestReadPosts(t *testing.T) {
	miners := []int{8001, 8002, 8003}
	mockTracker := newMockTracker(miners)

	trackerServer := httptest.NewServer(http.HandlerFunc(mockTracker.handleGetMiners))
	defer trackerServer.Close()

	newUser := user.NewUser(extractPort(trackerServer.URL))

	var blockchains [][]blockchain.Block
	for i := 0; i < len(miners); i++ {
		posts := []blockchain.Post{
			{
				User: &blockchain.GenerateKey().PublicKey,
				Body: blockchain.PostBody{
					Content:   fmt.Sprintf("Post %d-1", i),
					Timestamp: time.Now().UnixNano(),
				},
				Signature: []byte{},
			},
			{
				User: &blockchain.GenerateKey().PublicKey,
				Body: blockchain.PostBody{
					Content:   fmt.Sprintf("Post %d-2", i),
					Timestamp: time.Now().UnixNano(),
				},
				Signature: []byte{},
			},
		}
		// Sign the posts
		for j := range posts {
			posts[j].Signature = blockchain.Sign(blockchain.GenerateKey(), posts[j].Body)
		}

		blocks := []blockchain.Block{
			{
				Header: blockchain.BlockHeader{
					PrevHash:  []byte{},
					Summary:   blockchain.Hash(posts),
					Timestamp: time.Now().UnixNano(),
					Nonce:     0,
				},
				Posts: posts,
			},
		}
		blockchains = append(blockchains, blocks)
	}

	minerServers := make([]*httptest.Server, len(miners))
	for i := range miners {
		blocks := blockchains[i]
		minerServers[i] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var encoded []blockchain.BlockBase64
			for _, block := range blocks {
				encoded = append(encoded, block.EncodeBase64())
			}
			err := json.NewEncoder(w).Encode(miner.BlockChainJson{Blockchain: encoded})
			if err != nil {
				return
			}
		}))
		miners[i] = extractPort(minerServers[i].URL)
	}
	defer func() {
		for _, server := range minerServers {
			server.Close()
		}
	}()

	posts, err := newUser.ReadPosts()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Check if the retrieved posts match the expected posts
	var expectedPosts []blockchain.Post
	for _, blocks := range blockchains {
		for _, block := range blocks {
			expectedPosts = append(expectedPosts, block.Posts...)
		}
	}

	if len(posts) != len(expectedPosts) {
		t.Errorf("Expected %d posts, but got %d", len(expectedPosts), len(posts))
	}

	for i := range posts {
		if posts[i].Body != expectedPosts[i].Body {
			t.Errorf("Expected post body %v, but got %v", expectedPosts[i].Body, posts[i].Body)
		}
	}
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
