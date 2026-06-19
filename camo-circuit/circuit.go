// Package main implements the CAMO circuit controller.
//
// Spec reference: Section 10 — Circuit Construction
//                 Section 11 — Packet Routing
package main

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

// chainRotationInterval is how often circuits are replaced.
// Spec reference: Section 10.2 — ROTATION_INTERVAL = 600 seconds
const chainRotationInterval = 600 * time.Second

// minHops is the minimum number of hops in a circuit (excluding entry).
// Spec reference: Section 10.2 — MIN_HOPS = 3
const minHops = 3

// maxHops is the maximum number of hops in a circuit (excluding entry).
// Spec reference: Section 10.2 — MAX_HOPS = 5
const maxHops = 5

// sessionTokenLen is the byte length of a session token.
// Spec reference: Section 11.1 — 16 bytes random
const sessionTokenLen = 16

// Hop represents one node in a circuit.
// Spec reference: Appendix A.3 — CircuitRecord hops
type Hop struct {
	ServerID         string `json:"server_id"`
	WireGuardAddress string `json:"wireguard_address"`
	WireGuardPubKey  string `json:"wireguard_pubkey"`
	Jurisdiction     string `json:"jurisdiction"`
	HopIndex         int    `json:"hop_index"`
	IsExit           bool   `json:"is_exit"`
}

// Circuit is the active routing path for a device session.
// Spec reference: Appendix A.3 — CircuitRecord
type Circuit struct {
	CircuitID    string    `json:"circuit_id"`
	DeviceIMSI   string    `json:"device_imsi"`
	SessionToken string    `json:"session_token"` // base64, 16 bytes
	Hops         []Hop     `json:"hops"`
	CreatedAt    time.Time `json:"created_at"`
	RotateAt     time.Time `json:"rotate_at"`
}

// SessionTableEntry is what middle/exit nodes store.
// Spec reference: Appendix A.4
type SessionTableEntry struct {
	SessionToken string `json:"session_token"`
	ForwardPeer  string `json:"forward_peer"`  // WireGuard address of next hop
	ReturnPeer   string `json:"return_peer"`   // WireGuard address of previous hop
	CreatedAt    time.Time `json:"created_at"`
	IsExit       bool   `json:"is_exit"`
}

// circuitStore is a thread-safe store for active circuits.
// Keyed by device IMSI — each device has exactly one active circuit.
// Spec reference: Section 10.4 — circuit isolation
type circuitStore struct {
	mu       sync.RWMutex
	circuits map[string]*Circuit // keyed by IMSI
	byToken  map[string]*Circuit // keyed by session token
}

func newCircuitStore() *circuitStore {
	return &circuitStore{
		circuits: make(map[string]*Circuit),
		byToken:  make(map[string]*Circuit),
	}
}

// put stores a circuit, replacing any existing circuit for the same IMSI.
func (s *circuitStore) put(c *Circuit) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Remove old token index if replacing.
	if old, ok := s.circuits[c.DeviceIMSI]; ok {
		delete(s.byToken, old.SessionToken)
	}
	s.circuits[c.DeviceIMSI] = c
	s.byToken[c.SessionToken] = c
}

// getByIMSI returns the active circuit for a device IMSI.
func (s *circuitStore) getByIMSI(imsi string) (*Circuit, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.circuits[imsi]
	return c, ok
}

// remove deletes a circuit for a device IMSI.
func (s *circuitStore) remove(imsi string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.circuits[imsi]; ok {
		delete(s.byToken, c.SessionToken)
		delete(s.circuits, imsi)
	}
}

// all returns all active circuits.
func (s *circuitStore) all() []*Circuit {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*Circuit
	for _, c := range s.circuits {
		out = append(out, c)
	}
	return out
}

// count returns the number of active circuits.
func (s *circuitStore) count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.circuits)
}

// newSessionToken generates a cryptographically random 16-byte session token.
// Spec reference: Section 11.1
func newSessionToken() (string, error) {
	b := make([]byte, sessionTokenLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}
