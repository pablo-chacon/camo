package main

import (
	"log/slog"
	"time"
)

// simRotationInterval is the default SIM subset rotation interval.
// Deliberately NOT a simple multiple of chain rotation (600s) to prevent
// predictable alignment between the two rotation cycles.
// Spec reference: Section 7.5
const simRotationInterval = 480 * time.Second

// targetActiveRatio is the target fraction of the total pool to keep active.
// Spec reference: Section 7.4 — recommended 30-40% of total pool
const targetActiveRatio = 0.35

// minActiveCount is the minimum number of active SIMs regardless of pool size.
// Spec reference: Section 7.4
const minActiveCount = 4

// drainWindow is how long to wait for in-flight sessions to complete
// before retiring a SIM. Spec reference: Section 7.5
const drainWindow = 30 * time.Second

// rotator manages the active subset rotation.
// Spec reference: Section 7.5 — active subset rotation algorithm
type rotator struct {
	pool    *pool
	cfg     *config
	ticker  *time.Ticker
}

func newRotator(p *pool, cfg *config) *rotator {
	interval := simRotationInterval
	if cfg.SIMRotationInterval > 0 {
		interval = time.Duration(cfg.SIMRotationInterval) * time.Second
	}
	return &rotator{pool: p, cfg: cfg, ticker: time.NewTicker(interval)}
}

// run starts the rotation loop. Blocks until the process exits.
func (r *rotator) run() {
	// On startup, initialize the active subset.
	r.initActiveSubset()

	for range r.ticker.C {
		r.rotate()
	}
}

// initActiveSubset promotes SIMs from dormant to active until
// the target active count is reached. Run once at startup.
func (r *rotator) initActiveSubset() {
	r.pool.mu.Lock()
	defer r.pool.mu.Unlock()

	target := r.targetCount()
	promoted := 0

	for _, sim := range r.pool.sims {
		if promoted >= target {
			break
		}
		if sim.State == SIMDormant {
			sim.State = SIMActive
			promoted++
		}
	}
	slog.Info("active subset initialized", "active", promoted, "target", target)
}

// rotate performs one rotation cycle:
//  1. Selects replacement_count SIMs from dormant pool to promote
//  2. Selects replacement_count SIMs from active pool to retire
//  3. Applies promotions and retirements with drain window
//
// Spec reference: Section 7.5 — rotation algorithm
func (r *rotator) rotate() {
	r.pool.mu.Lock()

	target := r.targetCount()
	// Replacement count: random in [1, active_size/2]
	// Partial replacement prevents complete active set turnover.
	activeCount := 0
	for _, sim := range r.pool.sims {
		if sim.State == SIMActive {
			activeCount++
		}
	}

	maxReplace := activeCount / 2
	if maxReplace < 1 {
		maxReplace = 1
	}
	replaceCount, err := randomInt(maxReplace)
	if err != nil || replaceCount == 0 {
		replaceCount = 1
	}

	// Select dormant SIMs to promote.
	var toPromote []string
	for imsi, sim := range r.pool.sims {
		if len(toPromote) >= replaceCount {
			break
		}
		if sim.isDormantCandidate() {
			toPromote = append(toPromote, imsi)
		}
	}

	// Select active SIMs to retire. Cannot retire SIMs handling sessions
	// or SIMs active since the last rotation cycle.
	lastRotation := r.pool.lastRotation
	var toRetire []string
	for imsi, sim := range r.pool.sims {
		if len(toRetire) >= len(toPromote) {
			break
		}
		if sim.isRetirementCandidate(lastRotation) {
			toRetire = append(toRetire, imsi)
		}
	}

	// Promote dormant SIMs to active.
	for _, imsi := range toPromote {
		if sim, ok := r.pool.sims[imsi]; ok {
			sim.State = SIMActive
		}
	}

	// Ensure we have at least minActiveCount active SIMs.
	activeAfter := 0
	for _, sim := range r.pool.sims {
		if sim.State == SIMActive {
			activeAfter++
		}
	}
	if activeAfter < target {
		// Promote more dormant SIMs if needed.
		for _, sim := range r.pool.sims {
			if activeAfter >= target {
				break
			}
			if sim.State == SIMDormant {
				sim.State = SIMActive
				activeAfter++
			}
		}
	}

	r.pool.lastRotation = time.Now()
	r.pool.mu.Unlock()

	// Retire SIMs after drain window.
	// Drain window allows in-flight sessions to complete gracefully.
	// Spec reference: Section 7.5 — drain window
	if len(toRetire) > 0 {
		go func() {
			time.Sleep(drainWindow)
			r.pool.mu.Lock()
			defer r.pool.mu.Unlock()
			for _, imsi := range toRetire {
				if sim, ok := r.pool.sims[imsi]; ok {
					// Only retire if session has ended during drain window.
					if sim.CurrentSession == "" {
						sim.State = SIMDormant
					}
				}
			}
		}()
	}

	_, active, available := r.pool.stats()
	slog.Info("SIM subset rotated",
		"promoted", len(toPromote),
		"retiring", len(toRetire),
		"active", active,
		"available", available,
	)
}

// targetCount returns the target number of active SIMs.
func (r *rotator) targetCount() int {
	total := len(r.pool.sims)
	target := int(float64(total) * targetActiveRatio)
	if target < minActiveCount {
		target = minActiveCount
	}
	return target
}
