package miner

import (
	"blockchain/blockchain"
	"blockchain/tracker"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/emirpasic/gods/sets/treeset"
	"github.com/emirpasic/gods/utils"
	"github.com/gin-gonic/gin"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

const HeartbeatMin = 200
const HeartbeatMax = 400
const SyncMin = 500
const SyncMax = 1000
const MiningIterations = 10000
const PostsPerBlock = 2

type PostsJson struct {
	Posts []blockchain.PostBase64 `json:"posts"`
}

type BlockChainJson struct {
	Blockchain []blockchain.BlockBase64 `json:"blockchain"`
}

type Miner struct {
	blockChain  []blockchain.Block // current blockchain
	cmp         utils.Comparator   // comparator for posts and pool
	posts       *treeset.Set       // all posts on the current blockchain, sorted by timestamp
	pool        *treeset.Set       // posts to be posted to the blockchain
	port        int                // http port
	trackerPort int                // tracker's http port
	router      *gin.Engine        // http router
	server      *http.Server       // http server
	lock        sync.RWMutex       // protects all writable fields
	quit        chan struct{}      // notify the background routine to quit
}

func NewMiner(port int) *Miner {
	miner := &Miner{
		router: gin.Default(),
	}
	miner.cmp = func(a, b any) int {
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
	miner.posts = treeset.NewWith(miner.cmp, nil)
	miner.pool = treeset.NewWith(miner.cmp, nil)

	miner.registerAPIs()
	miner.server = &http.Server{
		Addr:    fmt.Sprintf("localhost:%d", port),
		Handler: miner.router,
	}
	return miner
}

func (m *Miner) Start() {
	go func() {
		if err := m.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("listen: %s\n", err)
		}
	}()
	go m.routine()
}

func (m *Miner) Shutdown() {
	// first shutdown background routine
	m.quit <- struct{}{}
	<-m.quit
	// then shutdown server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := m.server.Shutdown(ctx); err != nil {
		log.Println("error when shutting down server: ", err)
	}
	select {
	case <-ctx.Done():
		log.Println("shutting down server timeout")
	}
}

func (m *Miner) registerAPIs() {
	// register APIs
	m.router.GET("/read", func(ctx *gin.Context) {
		statusCode, response := m.readHandler()
		ctx.JSON(statusCode, response)
	})
	m.router.POST("/write", func(ctx *gin.Context) {
		var encoded blockchain.PostBase64
		if err := ctx.BindJSON(&encoded); err != nil {
			ctx.JSON(http.StatusBadRequest, map[string]string{"error": "post has invalid format"})
			return
		}
		post, err := encoded.DecodeBase64()
		if err != nil {
			ctx.JSON(http.StatusBadRequest, map[string]string{"error": "post has invalid base64 string"})
			return
		}
		statusCode, response := m.writeHandler(post)
		ctx.JSON(statusCode, response)
	})
	m.router.POST("/sync", func(ctx *gin.Context) {
		var request PostsJson
		if err := ctx.BindJSON(&request); err != nil {
			ctx.JSON(http.StatusBadRequest, map[string]string{"error": "request has invalid format"})
			return
		}
		posts := make([]blockchain.Post, 0)
		for _, encoded := range request.Posts {
			post, err := encoded.DecodeBase64()
			if err != nil {
				ctx.JSON(http.StatusBadRequest, map[string]string{"error": "post has invalid base64 string"})
				return
			}
			posts = append(posts, post)
		}
		statusCode, response := m.syncHandler(posts)
		ctx.JSON(statusCode, response)
	})
	m.router.POST("/broadcast", func(ctx *gin.Context) {
		var request BlockChainJson
		if err := ctx.BindJSON(&request); err != nil {
			ctx.JSON(http.StatusBadRequest, map[string]string{"error": "request has invalid format"})
			return
		}
		chain := make([]blockchain.Block, 0)
		for _, encoded := range request.Blockchain {
			block, err := encoded.DecodeBase64()
			if err != nil {
				ctx.JSON(http.StatusBadRequest, map[string]string{"error": "block has invalid base64 string"})
				return
			}
			chain = append(chain, block)
		}
		statusCode, response := m.broadcastHandler(chain)
		ctx.JSON(statusCode, response)
	})
}

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
				// sync with all peers
				request := PostsJson{}
				// gather all posts to send
				m.lock.RLock()
				iter := m.posts.Iterator()
				for iter.Next() {
					post := iter.Value().(blockchain.Post)
					request.Posts = append(request.Posts, post.EncodeBase64())
				}
				m.lock.RUnlock()
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
	peers = append(peers[:i], peers[i+1:]...)
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
			PrevHash:  make([]byte, 256),
			Summary:   blockchain.Hash(posts),
			Timestamp: time.Now().UnixNano(),
		},
		Posts: posts,
	}
	if len(m.blockChain) > 0 {
		copy(block.Header.PrevHash, blockchain.Hash(m.blockChain[len(m.blockChain)-1].Header))
	}

	success := false
	for i := 0; i < MiningIterations; i++ {
		block.Header.Nonce = rand.Uint32()
		hash := blockchain.Hash(block.Header)
		zeroBytes := blockchain.TARGET / 8
		zeroBits := blockchain.TARGET % 8
		// the first zeroBytes bytes of hash must be zero
		for i := 0; i < zeroBytes; i++ {
			if hash[i] != 0 {
				continue
			}
		}
		// and then zeroBits bits of hash must be zero
		if zeroBits > 0 {
			nextByte := hash[zeroBytes]
			nextByte = nextByte >> (8 - zeroBits)
			if nextByte != 0 {
				continue
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
