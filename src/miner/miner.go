package miner

import (
	"blockchain/blockchain"
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/emirpasic/gods/sets/treeset"
	"github.com/emirpasic/gods/utils"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"sync"
	"time"
)

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
