package main

import (
	"log/slog"
	"time"
)

// activeLivenessInterval is how often active SIMs are health-checked.
const activeLivenessInterval = 60 * time.Second

// dormantLivenessInterval is how often dormant SIMs are health-checked.
// Spec reference: Section 7.9 — intervals not exceeding 3600 seconds
const dormantLivenessInterval = 3600 * time.Second

// maxBackoffInterval is the maximum retry interval for unavailable SIMs.
const maxBackoffInterval = 4 * time.Hour

// liveness monitors SIM health and updates pool state accordingly.
// Spec reference: Section 7.9 — liveness monitoring
type liveness struct {
	pool    *pool
	checker SIMChecker
}

// SIMChecker is the interface for checking whether a SIM is reachable.
// In production this pings the carrier APN to confirm the SIM is registered
// and routing. In tests this is a mock.
type SIMChecker interface {
	Check(imsi string) error
}

func newLiveness(p *pool, checker SIMChecker) *liveness {
	return &liveness{pool: p, checker: checker}
}

// run starts the liveness monitoring loop.
func (l *liveness) run() {
	activeTicker := time.NewTicker(activeLivenessInterval)
	dormantTicker := time.NewTicker(dormantLivenessInterval)
	defer activeTicker.Stop()
	defer dormantTicker.Stop()

	for {
		select {
		case <-activeTicker.C:
			l.checkActive()
		case <-dormantTicker.C:
			l.checkDormant()
		}
	}
}

// checkActive verifies all active SIMs are reachable.
// Marks unresponsive SIMs as unavailable.
// Spec reference: Section 7.9 — continuous monitoring for active SIMs
func (l *liveness) checkActive() {
	for _, sim := range l.pool.allSIMs() {
		if sim.State != SIMActive {
			continue
		}
		if err := l.checker.Check(sim.IMSI); err != nil {
			slog.Warn("active SIM liveness check failed",
				"imsi", sim.IMSI,
				"carrier", sim.Carrier,
				"err", err,
			)
			l.pool.markUnavailable(sim.IMSI)
		} else {
			sim.LastHealthCheck = time.Now()
		}
	}
}

// checkDormant verifies dormant SIMs are still carrier-registered.
// Restores unavailable SIMs to dormant if they recover.
// Spec reference: Section 7.9 — periodic checks for dormant SIMs
func (l *liveness) checkDormant() {
	for _, sim := range l.pool.allSIMs() {
		switch sim.State {
		case SIMDormant:
			if err := l.checker.Check(sim.IMSI); err != nil {
				slog.Warn("dormant SIM liveness check failed",
					"imsi", sim.IMSI,
					"err", err,
				)
				l.pool.markUnavailable(sim.IMSI)
			} else {
				sim.LastHealthCheck = time.Now()
			}

		case SIMUnavailable:
			// Retry unavailable SIMs at exponential backoff.
			// Spec reference: Section 7.9 — exponential backoff
			backoff := backoffDuration(sim.FailCount)
			if time.Since(sim.LastHealthCheck) < backoff {
				continue
			}
			if err := l.checker.Check(sim.IMSI); err == nil {
				slog.Info("unavailable SIM recovered", "imsi", sim.IMSI)
				l.pool.markDormant(sim.IMSI)
			} else {
				sim.LastHealthCheck = time.Now()
				l.pool.markUnavailable(sim.IMSI) // increments FailCount
			}
		}
	}
}

// backoffDuration returns the retry wait time for a given failure count.
// Doubles each failure, capped at maxBackoffInterval.
func backoffDuration(failCount int) time.Duration {
	if failCount <= 0 {
		return dormantLivenessInterval
	}
	d := dormantLivenessInterval
	for i := 0; i < failCount; i++ {
		d *= 2
		if d > maxBackoffInterval {
			return maxBackoffInterval
		}
	}
	return d
}

// noopChecker is a SIMChecker that always succeeds.
// Used in development and testing when no real carrier is available.
type noopChecker struct{}

func (n *noopChecker) Check(_ string) error { return nil }
