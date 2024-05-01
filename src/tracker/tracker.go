package tracker

import (
	"context"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

// EntryTimeout - A miner entry expires after EntryTimeout, if no heartbeats are received.
const EntryTimeout = 500 * time.Millisecond

type RegisterRequest struct {
	Port int `json:"port"`
}

type RegisterResponse struct {
	Ports []int `json:"ports"`
}

type GetMinerResponse struct {
	Port int `json:"port"`
}

type Tracker struct {
	miners map[int]*time.Timer // maps each miner's port to its expiration timer
	lock   sync.Mutex          // protects miners
	router *gin.Engine
	server *http.Server
}

func NewTracker(port int) *Tracker {
	tracker := &Tracker{
		miners: make(map[int]*time.Timer),
		router: gin.Default(),
	}

	// register APIs
	tracker.router.POST("/register", func(ctx *gin.Context) {
		var request RegisterRequest
		if err := ctx.BindJSON(&request); err != nil {
			ctx.JSON(http.StatusBadRequest, nil)
			return
		}
		statusCode, response := tracker.registerHandler(request)
		ctx.JSON(statusCode, response)
	})
	tracker.router.GET("/get_miner", func(ctx *gin.Context) {
		statusCode, response := tracker.getMinerHandler()
		ctx.JSON(statusCode, response)
	})

	tracker.server = &http.Server{
		Addr:    fmt.Sprintf("localhost:%d", port),
		Handler: tracker.router,
	}

	return tracker
}

func (t *Tracker) Start() {
	go func() {
		if err := t.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("listen: %s\n", err)
		}
	}()
}

func (t *Tracker) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := t.server.Shutdown(ctx); err != nil {
		log.Println("error when shutting down server: ", err)
	}
	select {
	case <-ctx.Done():
		log.Println("shutting down server timeout")
	}
}

func (t *Tracker) registerHandler(request RegisterRequest) (int, any) {
	port := request.Port
	t.lock.Lock()
	defer t.lock.Unlock()
	timer, ok := t.miners[port]
	if ok {
		// stop timer
		timer.Stop()
	}
	// register a new timer
	t.miners[port] = time.AfterFunc(EntryTimeout, func() {
		t.lock.Lock()
		defer t.lock.Unlock()
		delete(t.miners, port)
	})
	var response RegisterResponse
	for port := range t.miners {
		response.Ports = append(response.Ports, port)
	}
	return http.StatusOK, response
}

func (t *Tracker) getMinerHandler() (int, any) {
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
	i := rand.Intn(len(ports))
	response := GetMinerResponse{Port: ports[i]}
	return http.StatusOK, response
}
