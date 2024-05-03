package miner

import (
	"blockchain/blockchain"
	"blockchain/tracker"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

const HeartbeatMin = 200
const HeartbeatMax = 400
const SyncMin = 300
const SyncMax = 600
const MiningIterations = 10000
const PostsPerBlock = 2

func (m *Miner) routine() {
	heartbeatInterval := time.Duration(HeartbeatMin+rand.Intn(HeartbeatMax-HeartbeatMin)) * time.Millisecond
	syncInterval := time.Duration(SyncMin+rand.Intn(SyncMax-SyncMin)) * time.Millisecond

	// register to the tracker immediately
	peers := m.register()
	// set up timers
	heartbeatTimer := time.NewTimer(heartbeatInterval)
	syncTimer := time.NewTimer(syncInterval)

loop:
	for {
	timerLoop:
		for {
			select {
			case <-heartbeatTimer.C:
				// send heartbeat to tracker
				peers = m.register()
				heartbeatTimer.Reset(heartbeatInterval)
			case <-syncTimer.C:
				// sync my pool with all peers, if I have at least one post
				request := PostsJson{}
				// gather all posts to send
				m.lock.RLock()
				iter := m.pool.Iterator()
				for iter.Next() {
					post := iter.Value().(blockchain.Post)
					request.Posts = append(request.Posts, post.EncodeBase64())
				}
				m.lock.RUnlock()
				if len(request.Posts) == 0 {
					// no need to sync empty requests
					syncTimer.Reset(syncInterval)
					continue timerLoop
				}
				reqBytes, err := json.Marshal(request)
				if err != nil {
					log.Fatalf("failed to encode sync request")
				}
				wg := sync.WaitGroup{}
				// sync in parallel
				for _, peer := range peers {
					peer := peer
					wg.Add(1)
					go m.syncWith(peer, reqBytes, &wg)
				}
				wg.Wait()
				syncTimer.Reset(syncInterval)
			case <-m.quit:
				break loop
			default:
				break timerLoop
			}
		}
		// mine
		m.mine(peers)
	}
	// stop all timers
	if !heartbeatTimer.Stop() {
		<-heartbeatTimer.C
	}
	if !syncTimer.Stop() {
		<-syncTimer.C
	}
	m.quit <- struct{}{}
}

func (m *Miner) register() []int {
	request := tracker.PortJson{Port: m.port}
	reqBytes, err := json.Marshal(request)
	if err != nil {
		log.Fatal("failed to encode register request to tracker")
	}
	url := fmt.Sprintf("http://localhost:%d/register", m.trackerPort)
	resp, err := http.Post(url, "application/json", bytes.NewReader(reqBytes))
	if err != nil {
		log.Println("failed to send register request to tracker")
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Println("failed to register to server")
		return nil
	}
	var response tracker.PortsJson
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		log.Printf("failed to decode registration response: %s", err.Error())
		return nil
	}
	peers := response.Ports
	// delete myself from the response
	i := 0
	for ; i < len(peers); i++ {
		if peers[i] == m.port {
			break
		}
	}
	if i < len(peers) {
		peers = append(peers[:i], peers[i+1:]...)
	}
	return peers
}

func (m *Miner) syncWith(peer int, data []byte, wg *sync.WaitGroup) {
	defer wg.Done()
	url := fmt.Sprintf("http://localhost:%d/sync", peer)
	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		log.Printf("error when syncing with peer %d: %s\n", peer, err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("failed to sync with peer %d\n", peer)
	}
}

func (m *Miner) mine(peers []int) {
	m.lock.RLock()
	length := len(m.blockChain)
	// fill in the block that is to be mined
	posts := make([]blockchain.Post, 0)
	iter := m.pool.Iterator()
	count := 0
	for iter.Next() {
		post := iter.Value().(blockchain.Post)
		posts = append(posts, post)
		count++
		if count >= PostsPerBlock {
			break
		}
	}
	block := blockchain.Block{
		Header: blockchain.BlockHeader{
			PrevHash:  make([]byte, 32),
			Summary:   blockchain.Hash(posts),
			Timestamp: time.Now().UnixNano(),
		},
		Posts: posts,
	}
	if len(m.blockChain) > 0 {
		hash := blockchain.Hash(m.blockChain[len(m.blockChain)-1].Header)
		copy(block.Header.PrevHash, hash)
	}

	success := false
MineIter:
	for i := 0; i < MiningIterations; i++ {
		block.Header.Nonce = rand.Uint32()
		hash := blockchain.Hash(block.Header)
		zeroBytes := blockchain.TARGET / 8
		zeroBits := blockchain.TARGET % 8
		// the first zeroBytes bytes of hash must be zero
		for i := 0; i < zeroBytes; i++ {
			if hash[i] != 0 {
				continue MineIter
			}
		}
		// and then zeroBits bits of hash must be zero
		if zeroBits > 0 {
			nextByte := hash[zeroBytes]
			nextByte = nextByte >> (8 - zeroBits)
			if nextByte != 0 {
				continue MineIter
			}
		}
		success = true
		break
	}
	m.lock.RUnlock()
	if !success {
		return
	}

	// append the new block to my blockchain
	m.lock.Lock()
	if len(m.blockChain) != length {
		// accepted other broadcasts between unlock and lock
		// abort
		m.lock.Unlock()
		return
	}
	m.blockChain = append(m.blockChain, block)
	for _, post := range block.Posts {
		m.posts.Add(post)
		m.pool.Remove(post)
	}
	request := BlockChainJson{}
	for _, block := range m.blockChain {
		request.Blockchain = append(request.Blockchain, block.EncodeBase64())
	}
	m.lock.Unlock()

	contents := make([]string, 0)
	for _, post := range block.Posts {
		contents = append(contents, post.Body.Content)
	}
	log.Printf("%d: Mined a block with contents (%v), chain length %d\n", m.port, contents, len(request.Blockchain))
	// broadcast the new block in parallel
	reqBytes, err := json.Marshal(request)
	if err != nil {
		log.Fatalf("failed to encode broadcast request")
	}
	wg := sync.WaitGroup{}
	for _, peer := range peers {
		peer := peer
		wg.Add(1)
		go m.broadcastTo(peer, reqBytes, &wg)
	}
	wg.Wait()
}

func (m *Miner) broadcastTo(peer int, data []byte, wg *sync.WaitGroup) {
	defer wg.Done()
	url := fmt.Sprintf("http://localhost:%d/broadcast", peer)
	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		log.Printf("error when broadcasting to peer %d: %s\n", peer, err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("failed to broadcast to peer %d\n", peer)
	}
}
