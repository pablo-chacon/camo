package main

import (
	"sync"
	"time"
)

// recordExpiry is the maximum age of a NodeRecord before it is expired.
// Spec reference: Section 9.2 — Records older than 24 hours are expired.
const recordExpiry = 24 * time.Hour

// livenessTimeout is how long without a heartbeat before a node is inactive.
// Spec reference: Section 9.2 — 300 seconds without heartbeat → inactive.
const livenessTimeout = 300 * time.Second

// entry is an internal store entry combining a NodeRecord with tracking state.
type entry struct {
	Record      *NodeRecord
	LastSeen    time.Time // time of last heartbeat or record update
	Active      bool
}

// store is a thread-safe in-memory store for NodeRecords.
// Provides deduplication, expiry, and liveness tracking.
type store struct {
	mu      sync.RWMutex
	records map[string]*entry // keyed by server_id
	seen    map[string]bool   // deduplication: keyed by recordKey()
}

func newStore() *store {
	return &store{
		records: make(map[string]*entry),
		seen:    make(map[string]bool),
	}
}

// isSeen returns true if a record with this deduplication key has been seen.
// Spec reference: Section 9.2
func (s *store) isSeen(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.seen[key]
}

// put stores a NodeRecord, marking it as seen and active.
// Returns false if the record was already seen (duplicate).
func (s *store) put(r *NodeRecord) bool {
	key := recordKey(r)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.seen[key] {
		return false
	}
	s.seen[key] = true
	s.records[r.ServerID] = &entry{
		Record:   r,
		LastSeen: time.Now(),
		Active:   true,
	}
	return true
}

// heartbeat updates the liveness timestamp for a server_id.
// Restores an inactive node to active. Spec reference: Section 9.2
func (s *store) heartbeat(serverID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e, ok := s.records[serverID]; ok {
		e.LastSeen = time.Now()
		e.Active = true
	}
}

// get returns the NodeRecord for a server_id, or nil if not found.
func (s *store) get(serverID string) *NodeRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if e, ok := s.records[serverID]; ok {
		return e.Record
	}
	return nil
}

// activeRecords returns all currently active NodeRecords.
// Used by circuit construction to get the candidate node list.
func (s *store) activeRecords() []*NodeRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*NodeRecord
	for _, e := range s.records {
		if e.Active {
			out = append(out, e.Record)
		}
	}
	return out
}

// allRecords returns all NodeRecords regardless of active state.
func (s *store) allRecords() []*NodeRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*NodeRecord
	for _, e := range s.records {
		out = append(out, e.Record)
	}
	return out
}

// expireAndCheck runs periodic maintenance:
//   - Marks nodes inactive if no heartbeat within livenessTimeout
//   - Removes records older than recordExpiry
//
// Spec reference: Section 9.2
func (s *store) expireAndCheck() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for serverID, e := range s.records {
		age := now.Sub(e.Record.Timestamp)
		if age > recordExpiry {
			delete(s.records, serverID)
			continue
		}
		if now.Sub(e.LastSeen) > livenessTimeout {
			e.Active = false
		}
	}
}
