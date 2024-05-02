package user

import (
	"blockchain/blockchain"
	"blockchain/tracker"
	"bytes"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

// N Number of miners the user will prompt for
var N int = 3

// User represents a user in the blockchain system
type User struct {
	privateKey  *rsa.PrivateKey
	trackerPort int
}

// NewUser creates a new user with the given tracker port
func NewUser(trackerPort int) *User {
	privateKey := blockchain.GenerateKey()
	return &User{
		privateKey:  privateKey,
		trackerPort: trackerPort,
	}
}

// GetRandomMiner retrieves a random miner from the tracker
func (u *User) GetRandomMiner() (int, error) {
	// Send a GET request to the tracker's "/get_miners" endpoint
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/get_miners", u.trackerPort))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	// Check the response status code
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("failed to get miners: %s", resp.Status)
	}

	// Decode the response body to get the list of miner ports
	var response tracker.PortsJson
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return 0, err
	}

	// Return a random miner port from the list
	if len(response.Ports) == 0 {
		return 0, fmt.Errorf("no miners available")
	}

	randomIndex := rand.Intn(len(response.Ports))
	return response.Ports[randomIndex], nil
}

// ReadPosts retrieves all posts from the specified miner
func (u *User) ReadPosts(minerURL string) ([]blockchain.Post, error) {
	// Send a GET request to the miner's "/posts" endpoint
	resp, err := http.Get(fmt.Sprintf("%s/posts", minerURL))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Decode the response body to get the list of posts
	var posts []blockchain.Post
	err = json.NewDecoder(resp.Body).Decode(&posts)
	if err != nil {
		return nil, err
	}

	return posts, nil
}

// WritePost writes a new post to the specified miners concurrently
func (u *User) WritePost(minerPorts []int, content string) error {
	// Create a new post with the given content and the user's public key
	post := blockchain.Post{
		User: &u.privateKey.PublicKey,
		Body: blockchain.PostBody{
			Content:   content,
			Timestamp: time.Now().Unix(),
		},
	}

	// Sign the post using the user's private key
	post.Signature = blockchain.Sign(u.privateKey, post.Body)

	// Encode the post to base64
	postBase64 := post.EncodeBase64()

	// Determine the number of miners to use
	numMiners := len(minerPorts)
	if numMiners > N {
		numMiners = N
	}

	// Create a wait group to wait for concurrent requests to finish
	var wg sync.WaitGroup
	wg.Add(numMiners)

	// Create a channel to collect errors from concurrent requests
	errChan := make(chan error, numMiners)

	// Send POST requests to the selected miners concurrently
	for i := 0; i < numMiners; i++ {
		go func(port int) {
			defer wg.Done()

			// Send a POST request to the miner's "/write" endpoint with the post data
			postJSON, _ := json.Marshal(postBase64)
			resp, err := http.Post(fmt.Sprintf("http://localhost:%d/write", port), "application/json", bytes.NewReader(postJSON))
			if err != nil {
				errChan <- err
				return
			}
			defer resp.Body.Close()

			// Check the response status code
			if resp.StatusCode != http.StatusOK {
				errChan <- fmt.Errorf("failed to write post to miner %d: %s", port, resp.Status)
			}
		}(minerPorts[i])
	}

	// Wait for all concurrent requests to finish
	wg.Wait()

	// Check if any errors occurred during concurrent requests
	select {
	case err := <-errChan:
		return err
	default:
		return nil
	}
}
