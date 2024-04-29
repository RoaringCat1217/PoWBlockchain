package tracker

import (
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
	miners  map[int]*time.Timer // maps each miner's port to its expiration timer
	lock    sync.Mutex          // protects miners
	port    int
	service *gin.Engine
}

func NewTracker(port int) *Tracker {
	tracker := &Tracker{
		miners:  make(map[int]*time.Timer),
		port:    port,
		service: gin.Default(),
	}

	// register APIs
	tracker.service.POST("/register", func(ctx *gin.Context) {
		var request RegisterRequest
		if err := ctx.BindJSON(&request); err != nil {
			ctx.JSON(http.StatusBadRequest, nil)
			return
		}
		statusCode, response := tracker.registerHandler(request)
		ctx.JSON(statusCode, response)
	})
	tracker.service.GET("/get_miner", func(ctx *gin.Context) {
		statusCode, response := tracker.getMinerHandler()
		ctx.JSON(statusCode, response)
	})

	return tracker
}

func (t *Tracker) Start() {
	go func() {
		err := t.service.Run(fmt.Sprintf("localhost:%d", t.port))
		if err != nil {
			log.Print(err)
		}
	}()
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
