package main

import (
	"sync"
	"time"
)

// peer represents a known gossip peer.
type peer struct {
	Addr     string    // host:port
	LastSeen time.Time
	Active   bool
}

// peerManager tracks known gossip peers.
// Thread-safe. Peers are added on first contact and updated on heartbeat.
type peerManager struct {
	mu    sync.RWMutex
	peers map[string]*peer // keyed by addr
}

func newPeerManager(bootstrapAddrs []string) *peerManager {
	pm := &peerManager{
		peers: make(map[string]*peer),
	}
	for _, addr := range bootstrapAddrs {
		pm.peers[addr] = &peer{Addr: addr, Active: true, LastSeen: time.Now()}
	}
	return pm
}

// add registers a peer address. No-op if already known.
func (pm *peerManager) add(addr string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if _, ok := pm.peers[addr]; !ok {
		pm.peers[addr] = &peer{Addr: addr, Active: true, LastSeen: time.Now()}
	}
}

// seen marks a peer as recently contacted.
func (pm *peerManager) seen(addr string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if p, ok := pm.peers[addr]; ok {
		p.LastSeen = time.Now()
		p.Active = true
	}
}

// activeAddrs returns addresses of all currently active peers.
func (pm *peerManager) activeAddrs() []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	var out []string
	for addr, p := range pm.peers {
		if p.Active {
			out = append(out, addr)
		}
	}
	return out
}

// allAddrs returns all known peer addresses.
func (pm *peerManager) allAddrs() []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	var out []string
	for addr := range pm.peers {
		out = append(out, addr)
	}
	return out
}
