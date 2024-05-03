package tests

import (
	"blockchain/blockchain"
	"blockchain/miner"
	"blockchain/tracker"
	Tracker "blockchain/tracker"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type PartitionTracker struct {
	miners      map[int]*time.Timer // maps each miner's port to its expiration timer
	lock        sync.Mutex          // protects miners
	router      *gin.Engine
	server      *http.Server
	partitioned atomic.Bool
}

func NewPartitionTracker(port int) *PartitionTracker {
	tracker := &PartitionTracker{
		miners: make(map[int]*time.Timer),
		router: gin.New(),
	}

	// register APIs
	tracker.router.POST("/register", func(ctx *gin.Context) {
		var request Tracker.PortJson
		if err := ctx.BindJSON(&request); err != nil {
			ctx.JSON(http.StatusBadRequest, nil)
			return
		}
		statusCode, response := tracker.registerHandler(request)
		ctx.JSON(statusCode, response)
	})
	tracker.router.GET("/get_miners", func(ctx *gin.Context) {
		statusCode, response := tracker.getMinersHandler()
		ctx.JSON(statusCode, response)
	})

	tracker.server = &http.Server{
		Addr:    fmt.Sprintf("localhost:%d", port),
		Handler: tracker.router,
	}

	return tracker
}

func (t *PartitionTracker) Start() {
	go func() {
		if err := t.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("listen: %s\n", err)
		}
	}()
}

func (t *PartitionTracker) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := t.server.Shutdown(ctx); err != nil {
		log.Println("error when shutting down server: ", err)
	}
	select {
	case <-ctx.Done():
		log.Println("shutting down server timeout")
	default:
		break
	}
}

func (t *PartitionTracker) Partition(partition bool) {
	t.partitioned.Store(partition)
}

func (t *PartitionTracker) registerHandler(request Tracker.PortJson) (int, any) {
	port := request.Port
	r := port % 2
	t.lock.Lock()
	defer t.lock.Unlock()
	timer, ok := t.miners[port]
	if ok {
		// stop timer
		timer.Stop()
	}
	// register a new timer
	t.miners[port] = time.AfterFunc(Tracker.EntryTimeout, func() {
		t.lock.Lock()
		defer t.lock.Unlock()
		delete(t.miners, port)
	})
	var response Tracker.PortsJson
	if t.partitioned.Load() {
		// do partitioning
		for port := range t.miners {
			if port%2 == r {
				response.Ports = append(response.Ports, port)
			}
		}
	} else {
		// act normally
		for port := range t.miners {
			response.Ports = append(response.Ports, port)
		}
	}

	return http.StatusOK, response
}

func (t *PartitionTracker) getMinersHandler() (int, any) {
	t.lock.Lock()
	defer t.lock.Unlock()
	if len(t.miners) == 0 {
		// no miners currently
		return http.StatusNotFound, nil
	}
	ports := make([]int, 0)
	for port := range t.miners {
		ports = append(ports, port)
	}
	response := Tracker.PortsJson{Ports: ports}
	return http.StatusOK, response
}

func ReadBlockchain(port int) []blockchain.Block {
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/read", port))
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	var respJson miner.BlockChainJson
	err = json.NewDecoder(resp.Body).Decode(&respJson)
	if err != nil {
		return nil
	}
	// retrieve blockchain
	chain := make([]blockchain.Block, 0)
	for _, encoded := range respJson.Blockchain {
		decoded, err := encoded.DecodeBase64()
		if err != nil {
			return nil
		}
		chain = append(chain, decoded)
	}
	return chain
}

func WriteBlockchain(port int, content string) error {
	privateKey := blockchain.GenerateKey()
	post := blockchain.Post{
		User: &privateKey.PublicKey,
		Body: blockchain.PostBody{
			Content:   content,
			Timestamp: time.Now().UnixNano(),
		},
	}

	// Sign the post using the private key
	post.Signature = blockchain.Sign(privateKey, post.Body)

	// Encode the post to base64
	postBase64 := post.EncodeBase64()

	postJSON, _ := json.Marshal(postBase64)
	resp, err := http.Post(fmt.Sprintf("http://localhost:%d/write", port), "application/json", bytes.NewReader(postJSON))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("miner rejected post: status code %d", resp.StatusCode)
	}
	resp.Body.Close()

	return nil
}

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
