package user

import (
	"blockchain/blockchain"
	"blockchain/tracker"
	"bytes"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

// N - Number of miners to select for writing posts
const (
	N = 5
)

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

// GetRandomMiners retrieves all miners from the tracker and selects a random subset
func (u *User) GetRandomMiners() ([]int, error) {
	// Send a GET request to the tracker's "/get_miners" endpoint
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/get_miners", u.trackerPort))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Check the response status code
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to retrieve miners from the tracker")
	}

	// Decode the response body to get the list of miner ports
	var response tracker.PortsJson
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return nil, errors.New("tracker sends invalid response")
	}
	ports := response.Ports

	// Select a random subset of miners
	if len(ports) <= N {
		// If the number of miners is less than or equal to N, use all miners
		return ports, nil
	}

	// Shuffle the miner ports randomly
	rand.Shuffle(len(ports), func(i, j int) {
		ports[i], ports[j] = ports[j], ports[i]
	})

	// Select the first N miners from the shuffled list
	return ports[:N], nil
}

// ReadPosts retrieves all posts from the specified miner
func (u *User) ReadPosts(minerPort int) ([]blockchain.Post, error) {
	// Send a GET request to the miner's "/posts" endpoint
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/posts", minerPort))
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

// WritePost writes a new post to the specified miners concurrently and handles errors.
func (u *User) WritePost(content string) error {
	// Create a new post with the given content and the user's public key
	post := blockchain.Post{
		User: &u.privateKey.PublicKey,
		Body: blockchain.PostBody{
			Content:   content,
			Timestamp: time.Now().UnixNano(),
		},
	}

	// Sign the post using the user's private key
	post.Signature = blockchain.Sign(u.privateKey, post.Body)

	// Encode the post to base64
	postBase64 := post.EncodeBase64()

	// Determine the number of miners to use
	miners, err := u.GetRandomMiners()
	if err != nil {
		return err
	}

	// Create a wait group to wait for concurrent requests to finish
	var wg sync.WaitGroup
	errChan := make(chan error, len(miners)) // Channel to collect errors

	// Send POST requests to the selected miners concurrently
	for _, port := range miners {
		port := port
		wg.Add(1)
		go func(port int) {
			defer wg.Done()

			// Send a POST request to the miner's "/write" endpoint with the post data
			postJSON, _ := json.Marshal(postBase64)
			resp, err := http.Post(fmt.Sprintf("http://localhost:%d/write", port), "application/json", bytes.NewReader(postJSON))
			if err != nil {
				errChan <- err
				return
			}
			if resp.StatusCode != http.StatusOK {
				errChan <- fmt.Errorf("miner rejected post: status code %d", resp.StatusCode)
			}
			resp.Body.Close()
		}(port)
	}

	// Wait for all concurrent requests to finish
	wg.Wait()
	close(errChan) // Close channel to finish range iteration

	// Check for errors from the error channel
	for e := range errChan {
		if e != nil {
			return e // Return the first error encountered
		}
	}

	return nil
}
