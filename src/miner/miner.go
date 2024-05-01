package miner

import (
	"blockchain/blockchain"
)

type Miner struct {
	blockChain []blockchain.Block // current blockchain
	posts      []blockchain.Post  // all posts on the current blockchain, sorted by timestamp
}
