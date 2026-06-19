package main

import (
	"crypto/rand"
	"errors"
	"math/big"
	"sync"
	"time"
)

var (
	errNoAvailableSIM    = errors.New("no available SIM in active subset")
	errSessionNotFound   = errors.New("session not found")
	errSIMNotFound       = errors.New("SIM not found")
)

// pool manages the full collection of M2M SIM cards and their state.
// Thread-safe. Implements the two-axis rotation model from Section 7.
type pool struct {
	mu              sync.RWMutex
	sims            map[string]*SIM // keyed by IMSI
	lastRotation    time.Time
	// sessions maps session token → IMSI for active sessions.
	sessions        map[string]string
}

func newPool() *pool {
	return &pool{
		sims:         make(map[string]*SIM),
		sessions:     make(map[string]string),
		lastRotation: time.Now(),
	}
}

// add registers a new SIM in the pool. Initial state is dormant.
func (p *pool) add(sim *SIM) {
	p.mu.Lock()
	defer p.mu.Unlock()
	sim.State = SIMDormant
	p.sims[sim.IMSI] = sim
}

// allocate assigns an active, idle SIM to a new session.
// Returns the assigned IMSI and records the session.
// Spec reference: Section 7.6 — per-session SIM allocation
func (p *pool) allocate(sessionToken string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Find available SIMs. Prefer those not recently used.
	var candidates []*SIM
	for _, sim := range p.sims {
		if sim.isAvailable() {
			candidates = append(candidates, sim)
		}
	}
	if len(candidates) == 0 {
		return "", errNoAvailableSIM
	}

	// Randomized selection from candidates.
	// Spec reference: Section 7.6 — randomized assignment
	idx, err := randomInt(len(candidates))
	if err != nil {
		return "", err
	}
	chosen := candidates[idx]
	chosen.CurrentSession = sessionToken
	chosen.LastActive = time.Now()
	p.sessions[sessionToken] = chosen.IMSI
	return chosen.IMSI, nil
}

// release frees the SIM associated with a session.
// Called when a session terminates.
func (p *pool) release(sessionToken string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	imsi, ok := p.sessions[sessionToken]
	if !ok {
		return errSessionNotFound
	}
	delete(p.sessions, sessionToken)

	sim, ok := p.sims[imsi]
	if !ok {
		return errSIMNotFound
	}
	sim.CurrentSession = ""
	return nil
}

// stats returns a snapshot of pool statistics for gossip announcements.
// Spec reference: Section 7.6 — reports available active SIM count
func (p *pool) stats() (total, active, available int) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, sim := range p.sims {
		total++
		if sim.State == SIMActive {
			active++
			if sim.CurrentSession == "" {
				available++
			}
		}
	}
	return
}

// markUnavailable marks a SIM as unavailable due to liveness failure.
// Spec reference: Section 7.9
func (p *pool) markUnavailable(imsi string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if sim, ok := p.sims[imsi]; ok {
		sim.State = SIMUnavailable
		sim.FailCount++
	}
}

// markDormant moves an unavailable SIM back to dormant after recovery.
func (p *pool) markDormant(imsi string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if sim, ok := p.sims[imsi]; ok {
		sim.State = SIMDormant
		sim.FailCount = 0
	}
}

// activeSIMs returns a copy of all active SIM IMSIs.
func (p *pool) activeSIMs() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var out []string
	for imsi, sim := range p.sims {
		if sim.State == SIMActive {
			out = append(out, imsi)
		}
	}
	return out
}

// allSIMs returns all SIMs for inspection (e.g. liveness checks).
func (p *pool) allSIMs() []*SIM {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var out []*SIM
	for _, sim := range p.sims {
		out = append(out, sim)
	}
	return out
}

// randomInt returns a cryptographically random integer in [0, n).
func randomInt(n int) (int, error) {
	if n <= 0 {
		return 0, errors.New("n must be positive")
	}
	max := big.NewInt(int64(n))
	v, err := rand.Int(rand.Reader, max)
	if err != nil {
		return 0, err
	}
	return int(v.Int64()), nil
}
