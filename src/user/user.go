package user

import (
	"blockchain/blockchain"
	"blockchain/miner"
	"blockchain/tracker"
	"bytes"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/emirpasic/gods/sets/treeset"
	"math/rand"
	"net/http"
	"sort"
	"sync"
	"time"
)

// RWCount - Number of miners to select for writing posts
const RWCount = 5

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
	if len(ports) <= RWCount {
		// If the number of miners is less than or equal to RWCount, use all miners
		return ports, nil
	}

	// Shuffle the miner ports randomly
	rand.Shuffle(len(ports), func(i, j int) {
		ports[i], ports[j] = ports[j], ports[i]
	})

	// Select the first RWCount miners from the shuffled list
	return ports[:RWCount], nil
}

// ReadPosts retrieves all posts from the specified miner
func (u *User) ReadPosts() ([]blockchain.Post, error) {
	miners, err := u.GetRandomMiners()
	if err != nil {
		return nil, err
	}

	// send concurrent requests to get each miner's blockchain
	respChan := make(chan []blockchain.Block)
	for _, port := range miners {
		port := port
		go func(port int) {
			resp, err := http.Get(fmt.Sprintf("http://localhost:%d/read", port))
			if err != nil {
				respChan <- nil
				return
			}
			defer resp.Body.Close()

			var respJson miner.BlockChainJson
			err = json.NewDecoder(resp.Body).Decode(&respJson)
			if err != nil {
				respChan <- nil
				return
			}
			// retrieve blockchain
			chain := make([]blockchain.Block, 0)
			for _, encoded := range respJson.Blockchain {
				decoded, err := encoded.DecodeBase64()
				if err != nil {
					respChan <- nil
					return
				}
				chain = append(chain, decoded)
			}
			respChan <- chain
		}(port)
	}
	chains := make([][]blockchain.Block, 0)
	for i := 0; i < len(miners); i++ {
		chains = append(chains, <-respChan)
	}
	// sort the chains from longest to shortest
	sort.Slice(chains, func(i, j int) bool {
		return len(chains[i]) > len(chains[j])
	})

	// find the first valid chain
	cmp := func(a, b any) int {
		post1 := a.(blockchain.Post)
		post2 := b.(blockchain.Post)
		if post1.Body.Timestamp != post2.Body.Timestamp {
			if post1.Body.Timestamp < post2.Body.Timestamp {
				return -1
			} else {
				return 1
			}
		}
		key1 := blockchain.PublicKeyToBytes(post1.User)
		key2 := blockchain.PublicKeyToBytes(post2.User)
		return bytes.Compare(key1, key2)
	}
	var posts *treeset.Set
VerifyChains:
	for _, chain := range chains {
		if len(chain) == 0 {
			continue VerifyChains
		}
		// each block must be valid
		for _, block := range chain {
			if !block.Verify() {
				continue VerifyChains
			}
		}
		// their hash value must form a chain
		if !bytes.Equal(chain[0].Header.PrevHash, make([]byte, 256)) {
			continue VerifyChains
		}
		for i := 1; i < len(chain); i++ {
			if !bytes.Equal(chain[i].Header.PrevHash, blockchain.Hash(chain[i-1].Header)) {
				continue VerifyChains
			}
		}
		// no duplicated posts
		posts = treeset.NewWith(cmp)
		for _, block := range chain {
			for _, post := range block.Posts {
				if posts.Contains(post) {
					posts = nil
					continue VerifyChains
				}
				posts.Add(post)
			}
		}
		// done
		break
	}
	if posts == nil {
		return nil, errors.New("failed to receive a valid blockchain")
	}
	postsList := make([]blockchain.Post, 0)
	iter := posts.Iterator()
	for iter.Next() {
		postsList = append(postsList, iter.Value().(blockchain.Post))
	}
	return postsList, nil
}

// WritePost writes a new post to the specified miners concurrently
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
				return
			}
			resp.Body.Close()
		}(port)
	}
	// Wait for all concurrent requests to finish
	wg.Wait()
	return nil
}
