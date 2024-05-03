package tests

import (
	"blockchain/user"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
