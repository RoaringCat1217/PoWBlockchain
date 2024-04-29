package tracker

import (
	"github.com/gin-gonic/gin"
	"sync"
	"time"
)

const Timeout =

type RegisterRequest struct {
	Port int `json:"port"`
}

type RegisterResponse struct {
	Miners []int `json:"miners"`
}

type Tracker struct {
	miners  map[int]time.Timer // maps each miner's port to its expiration timer
	lock    sync.Mutex         // protects miners
	service *gin.Engine
}

func NewTracker(regPort int, userPort int) *Tracker {
	tracker := &Tracker{
		miners:  make(map[int]time.Timer),
		service: gin.Default(),
	}

}

func (t *Tracker) registerHandler(request RegisterRequest) (int, any) {
	port := request.Port
	t.lock.Lock()
	defer t.lock.Unlock()
	_, ok :=
}
