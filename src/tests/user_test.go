package tests

import (
	"blockchain/blockchain"
	"blockchain/tracker"
	"blockchain/user"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// N - Number of miners to select for writing posts
const (
	N = 5
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

// TestReadPosts tests the retrieval of posts from a miner
func TestReadPosts(t *testing.T) {
	posts := []blockchain.Post{
		{
			Body: blockchain.PostBody{
				Content:   "Post 1",
				Timestamp: time.Now().UnixNano(),
			},
		},
		{
			Body: blockchain.PostBody{
				Content:   "Post 2",
				Timestamp: time.Now().UnixNano(),
			},
		},
	}

	minerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := json.NewEncoder(w).Encode(posts)
		if err != nil {
			return
		}
	}))
	defer minerServer.Close()

	newUser := user.NewUser(0)
	minerPort := extractPort(minerServer.URL)

	retrievedPosts, err := newUser.ReadPosts(minerPort)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// We cannot compare the entire Post struct since it contains unexported fields
	// Instead, we can compare the exported fields that we expect to be the same
	for i := range posts {
		if posts[i].Body != retrievedPosts[i].Body {
			t.Errorf("Expected post body %v, but got %v", posts[i].Body, retrievedPosts[i].Body)
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

	minerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var post blockchain.PostBase64
		err := json.NewDecoder(r.Body).Decode(&post)
		if err != nil {
			return
		}
		receivedPosts = append(receivedPosts, post)
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
