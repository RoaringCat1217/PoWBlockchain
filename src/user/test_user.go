package user

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	blockchain "github.com/cmu14736/s24-lab4-stilllearninggo"
)

func TestNewUser(t *testing.T) {
	trackerURL := "http://tracker.example.com"
	user := NewUser(trackerURL)

	if user.trackerURL != trackerURL {
		t.Errorf("Expected trackerURL to be %s, but got %s", trackerURL, user.trackerURL)
	}

	if user.privateKey == nil {
		t.Error("Expected privateKey to be set, but got nil")
	}
}

func TestGetRandomMiner(t *testing.T) {
	trackerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/miner" {
			http.Error(w, "Invalid request path", http.StatusBadRequest)
			return
		}

		if r.Method != http.MethodGet {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode("miner1")
	}))
	defer trackerServer.Close()

	user := &User{
		trackerURL: trackerServer.URL,
	}

	miner, err := user.GetRandomMiner()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expectedMiner := "miner1"
	if miner != expectedMiner {
		t.Errorf("Expected miner to be %s, but got %s", expectedMiner, miner)
	}

	// Test case for 404 Not Found
	trackerServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer trackerServer.Close()

	user.trackerURL = trackerServer.URL
	miner, err = user.GetRandomMiner()
	if err == nil {
		t.Error("Expected error, but got nil")
	}

	expectedError := "no miner found"
	if err.Error() != expectedError {
		t.Errorf("Expected error to be %s, but got %s", expectedError, err.Error())
	}

	// Test case for non-200 status code
	trackerServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer trackerServer.Close()

	user.trackerURL = trackerServer.URL
	miner, err = user.GetRandomMiner()
	if err == nil {
		t.Error("Expected error, but got nil")
	}

	expectedError = "failed to get miner: 500 Internal Server Error"
	if err.Error() != expectedError {
		t.Errorf("Expected error to be %s, but got %s", expectedError, err.Error())
	}
}

func TestReadPosts(t *testing.T) {
	minerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/posts" {
			http.Error(w, "Invalid request path", http.StatusBadRequest)
			return
		}

		if r.Method != http.MethodGet {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		posts := []Post{
			{
				User: &rsa.PublicKey{},
				body: PostBody{
					Content:   "Post 1",
					Timestamp: time.Now().Unix(),
				},
				Signature: []byte("signature1"),
			},
			{
				User: &rsa.PublicKey{},
				body: PostBody{
					Content:   "Post 2",
					Timestamp: time.Now().Unix(),
				},
				Signature: []byte("signature2"),
			},
		}
		json.NewEncoder(w).Encode(posts)
	}))
	defer minerServer.Close()

	user := &User{}
	posts, err := user.ReadPosts(minerServer.URL)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(posts) != 2 {
		t.Errorf("Expected 2 posts, but got %d", len(posts))
	}

	expectedContent := []string{"Post 1", "Post 2"}
	for i, post := range posts {
		if post.body.Content != expectedContent[i] {
			t.Errorf("Expected post content to be %s, but got %s", expectedContent[i], post.body.Content)
		}
	}
}

func TestWritePost(t *testing.T) {
	minerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/post" {
			http.Error(w, "Invalid request path", http.StatusBadRequest)
			return
		}

		if r.Method != http.MethodPost {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		var postBase64 PostBase64
		err := json.NewDecoder(r.Body).Decode(&postBase64)
		if err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		post, err := postBase64.DecodeBase64()
		if err != nil {
			http.Error(w, "Invalid post data", http.StatusBadRequest)
			return
		}

		if post.body.Content != "Test post" {
			http.Error(w, "Invalid post content", http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer minerServer.Close()

	privateKey := GenerateKey()
	user := &User{
		privateKey: privateKey,
	}

	err := user.WritePost(minerServer.URL, "Test post")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Test case for non-200 status code
	minerServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer minerServer.Close()

	err = user.WritePost(minerServer.URL, "Test post")
	if err == nil {
		t.Error("Expected error, but got nil")
	}

	expectedError := "failed to write post: 500 Internal Server Error"
	if err.Error() != expectedError {
		t.Errorf("Expected error to be %s, but got %s", expectedError, err.Error())
	}
}