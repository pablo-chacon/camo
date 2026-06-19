// Package main implements the CAMO SIM pool manager.
// Spec reference: Section 7 — SIM Pool Management
package main

import "time"

// SIMState represents the operational state of a SIM card in the pool.
// Spec reference: Section 7.4 — active/dormant distinction
type SIMState int

const (
	// SIMActive means the SIM is in the active subset and available
	// for session allocation. Carrier-registered and routing enabled.
	SIMActive SIMState = iota

	// SIMDormant means the SIM is carrier-registered but not in the
	// active subset. Not available for session allocation.
	// Spec reference: Section 7.8 — dormant SIM behavior
	SIMDormant

	// SIMUnavailable means the SIM has failed liveness checks.
	// Excluded from both active subset and rotation candidates.
	// Spec reference: Section 7.9 — liveness monitoring
	SIMUnavailable
)

func (s SIMState) String() string {
	switch s {
	case SIMActive:
		return "active"
	case SIMDormant:
		return "dormant"
	case SIMUnavailable:
		return "unavailable"
	default:
		return "unknown"
	}
}

// SIM represents a single M2M SIM card in the pool.
// Spec reference: Section 7.2 — pool composition
type SIM struct {
	// IMSI is the International Mobile Subscriber Identity.
	// Unique identifier for this SIM on the carrier network.
	IMSI string `json:"imsi"`

	// Carrier is the M2M carrier providing this SIM's radio access.
	Carrier string `json:"carrier"`

	// Jurisdiction is the legal jurisdiction where this SIM is registered.
	// ISO 3166-1 alpha-2. Used for diversity enforcement in circuit construction.
	// Spec reference: Section 10.2 — jurisdictional diversity
	Jurisdiction string `json:"jurisdiction"`

	// APN is the private APN configuration for this SIM.
	APN string `json:"apn"`

	// State is the current operational state of this SIM.
	State SIMState `json:"state"`

	// LastHealthCheck is the time of the most recent liveness check.
	// Spec reference: Section 7.9
	LastHealthCheck time.Time `json:"last_health_check"`

	// LastActive is the last time this SIM was used in an active session.
	// Used to enforce no consecutive-reuse constraint.
	// Spec reference: Section 7.6
	LastActive time.Time `json:"last_active"`

	// FailCount tracks consecutive liveness failures.
	// Spec reference: Section 7.9 — exponential backoff on failure
	FailCount int `json:"fail_count"`

	// CurrentSession holds the session token if this SIM is currently
	// handling an active session. Empty if idle.
	CurrentSession string `json:"current_session,omitempty"`
}

// isAvailable returns true if the SIM is active and not currently
// handling a session — available for new session allocation.
func (s *SIM) isAvailable() bool {
	return s.State == SIMActive && s.CurrentSession == ""
}

// isDormantCandidate returns true if the SIM is eligible to be promoted
// to the active subset during a rotation.
// Spec reference: Section 7.5 — dormant SIMs promoted during rotation
func (s *SIM) isDormantCandidate() bool {
	return s.State == SIMDormant
}

// isRetirementCandidate returns true if the SIM is eligible to be moved
// from active to dormant during a rotation.
// Spec reference: Section 7.5 — constraints on retirement
func (s *SIM) isRetirementCandidate(lastRotationTime time.Time) bool {
	if s.State != SIMActive {
		return false
	}
	// Cannot retire a SIM currently handling a session.
	if s.CurrentSession != "" {
		return false
	}
	// Cannot retire a SIM that was last active during the immediately
	// preceding rotation cycle — prevents too-rapid reuse.
	if s.LastActive.After(lastRotationTime) {
		return false
	}
	return true
}
