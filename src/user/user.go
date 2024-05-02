package user

import (
	"blockchain/blockchain"
	"bytes"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// User represents a user in the blockchain system
type User struct {
	privateKey *rsa.PrivateKey
	trackerURL string
}

// NewUser creates a new user with the given tracker URL
func NewUser(trackerURL string) *User {
	privateKey := blockchain.GenerateKey()
	return &User{
		privateKey: privateKey,
		trackerURL: trackerURL,
	}
}

// GetRandomMiner retrieves a random miner from the tracker
func (u *User) GetRandomMiner() (string, error) {
	// Send a GET request to the tracker's "/miner" endpoint
	resp, err := http.Get(fmt.Sprintf("%s/miner", u.trackerURL))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Check the response status code
	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("no miner found")
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get miner: %s", resp.Status)
	}

	// Decode the response body to get the miner's address
	var miner string
	err = json.NewDecoder(resp.Body).Decode(&miner)
	if err != nil {
		return "", err
	}

	return miner, nil
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

// WritePost writes a new post to the specified miner
func (u *User) WritePost(minerURL string, content string) error {
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

	// Encode the post to base64 and marshal it to JSON
	postBytes, err := json.Marshal(post.EncodeBase64())
	if err != nil {
		return err
	}

	// Send a POST request to the miner's "/post" endpoint with the post data
	resp, err := http.Post(fmt.Sprintf("%s/post", minerURL), "application/json", bytes.NewReader(postBytes))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check the response status code
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to write post: %s", resp.Status)
	}

	return nil
}
