package tests

import (
	Miner "blockchain/miner"
	Tracker "blockchain/tracker"
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

// TestMinerDiscovery - test whether miners can register to the tracker and discover each other correctly
func TestMinerDiscovery(t *testing.T) {
	tracker := Tracker.NewTracker(8080)
	tracker.Start()
	miners := make([]*Miner.Miner, 0)
	for i := 0; i < 2; i++ {
		miner := Miner.NewMiner(3000+i, 8080)
		miner.Start()
		miners = append(miners, miner)
	}
	time.Sleep(500 * time.Millisecond)
	// initialize a mock miner at 3002
	request := Tracker.PortJson{Port: 3002}
	reqBytes, _ := json.Marshal(request)
	url := "http://localhost:8080/register"
	resp, err := http.Post(url, "application/json", bytes.NewReader(reqBytes))
	if err != nil {
		t.Fatalf("failed to connect to tracker")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("failed to register to tracker")
	}
	var response Tracker.PortsJson
	_ = json.NewDecoder(resp.Body).Decode(&response)
	peers := response.Ports
	// should have 3 peers (including the mock miner)
	if len(peers) != 3 {
		t.Fatalf("wrong number of peers: %d\n", len(peers))
	}

	// wait for 3002 miner to timeout
	time.Sleep(1000 * time.Millisecond)
	// initialize a mock miner at 3003
	request = Tracker.PortJson{Port: 3003}
	reqBytes, _ = json.Marshal(request)
	resp, err = http.Post(url, "application/json", bytes.NewReader(reqBytes))
	if err != nil {
		t.Fatalf("failed to connect to tracker")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("failed to register to tracker")
	}
	_ = json.NewDecoder(resp.Body).Decode(&response)
	peers = response.Ports
	// should still have 10 peers (including the mock miner)
	if len(peers) != 3 {
		t.Fatalf("wrong number of peers: %d\n", len(peers))
	}
	// 3009 should not be in peers
	for _, peer := range peers {
		if peer == 3002 {
			t.Fatalf("3002 does not time out")
		}
	}
	// cleanup everything
	for _, miner := range miners {
		miner.Shutdown()
	}
	tracker.Shutdown()
}
