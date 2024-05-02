package miner

import (
	"blockchain/blockchain"
	"bytes"
	"github.com/emirpasic/gods/sets/treeset"
	"net/http"
)

func (m *Miner) readHandler() (int, any) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	resp := BlockChainJson{}
	for _, block := range m.blockChain {
		resp.Blockchain = append(resp.Blockchain, block.EncodeBase64())
	}
	return http.StatusOK, resp
}

func (m *Miner) writeHandler(post blockchain.Post) (int, any) {
	if !post.Verify() {
		return http.StatusBadRequest, map[string]string{"error": "invalid post"}
	}
	m.lock.Lock()
	defer m.lock.Unlock()

	// the new post must not be on the blockchain already
	if m.posts.Contains(post) {
		return http.StatusBadRequest, map[string]string{"error": "duplicated post on the blockchain"}
	}
	// the new post must not be in the pool already
	if m.pool.Contains(post) {
		return http.StatusBadRequest, map[string]string{"error": "duplicated post in the post"}
	}
	m.pool.Add(post)
	return http.StatusOK, nil
}

func (m *Miner) syncHandler(posts []blockchain.Post) (int, any) {
	m.lock.Lock()
	defer m.lock.Unlock()

	// all posts must be valid
	for _, post := range posts {
		if !post.Verify() {
			return http.StatusBadRequest, map[string]string{"error": "posts are invalid"}
		}
	}
	// add all posts that are not duplicated
	for _, post := range posts {
		// the new post must not be in the blockchain or pool already
		if m.posts.Contains(post) || m.pool.Contains(post) {
			continue
		}
		// accept the post
		m.pool.Add(post)
	}
	return http.StatusOK, nil
}

func (m *Miner) broadcastHandler(newChain []blockchain.Block) (int, any) {
	m.lock.Lock()
	defer m.lock.Unlock()

	if len(newChain) <= len(m.blockChain) {
		// shorter or equal than mine, just ignore it
		return http.StatusOK, nil
	}
	// each block must be valid
	for _, block := range newChain {
		if !block.Verify() {
			return http.StatusOK, nil
		}
	}
	// their hash value must form a chain
	if !bytes.Equal(newChain[0].Header.PrevHash, make([]byte, 256)) {
		return http.StatusOK, nil
	}
	for i := 1; i < len(newChain); i++ {
		if !bytes.Equal(newChain[i].Header.PrevHash, blockchain.Hash(newChain[i-1].Header)) {
			return http.StatusOK, nil
		}
	}
	// no duplicated posts
	posts := treeset.NewWith(m.cmp)
	for _, block := range newChain {
		for _, post := range block.Posts {
			if posts.Contains(post) {
				return http.StatusOK, nil
			}
			posts.Add(post)
		}
	}
	// all checks passed, compute the new pool
	pool := treeset.NewWith(m.cmp)
	iter := m.pool.Iterator()
	for iter.Next() {
		post := iter.Value().(blockchain.Post)
		if !posts.Contains(post) {
			pool.Add(post)
		}
	}
	// any blocks that are discarded will return to the pool
	i := 0
	for ; i < len(m.blockChain); i++ {
		if !bytes.Equal(blockchain.Hash(m.blockChain[i].Header), blockchain.Hash(newChain[i].Header)) {
			break
		}
	}
	// blocks from i to the end are discarded
	for ; i < len(m.blockChain); i++ {
		for _, post := range m.blockChain[i].Posts {
			if !posts.Contains(post) {
				pool.Add(post)
			}
		}
	}
	// update everything
	m.blockChain = newChain
	m.posts = posts
	m.pool = pool
	return http.StatusOK, nil
}
