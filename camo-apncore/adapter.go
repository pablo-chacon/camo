// Package main implements the CAMO APN core interface adapter.
//
// Spec reference: Section 6 — APN Core Interface, Appendix B
//
// This component translates between the CAMO protocol's management
// API (Appendix B) and the underlying APN core implementation.
// The adapter interface allows any compliant APN core to be used
// without modifying any other CAMO component.
//
// To integrate a new APN core implementation:
//  1. Implement the APNAdapter interface in a new file
//  2. Select it in main.go based on configuration
//  3. No other changes are required
package main

import "time"

// Session represents an active device PDN session on the APN core.
// Spec reference: Appendix B.1 — session_created event fields
type Session struct {
	IMSI      string    `json:"imsi"`
	GTPTEID   uint32    `json:"gtp_teid"`
	DeviceIP  string    `json:"device_ip"`
	CreatedAt time.Time `json:"created_at"`
}

// RouteRule defines per-IMSI next-hop routing.
// Spec reference: Appendix B.2 — set_route command
type RouteRule struct {
	IMSI              string `json:"imsi"`
	NextHopWireGuardIP string `json:"next_hop_wireguard_ip"`
	SessionToken      string `json:"session_token"`
}

// APNAdapter is the interface any APN core implementation must satisfy.
// Spec reference: Section 6.2 — required behaviors, Appendix B
type APNAdapter interface {
	// SetRoute programs a per-IMSI next-hop routing rule.
	// Spec reference: Appendix B.2 — set_route command, behavior B2
	SetRoute(rule RouteRule) error

	// ClearRoute removes the routing rule for an IMSI.
	// Spec reference: Appendix B.2 — clear_route command
	ClearRoute(imsi string) error

	// ListSessions returns all currently active PDN sessions.
	// Spec reference: Appendix B.2 — list_sessions command, behavior B3
	ListSessions() ([]Session, error)

	// SessionEvents returns a channel that emits session lifecycle events.
	// Events are session_created and session_terminated.
	// Spec reference: Appendix B.1 — session events, behavior B3
	SessionEvents() <-chan SessionEvent
}

// SessionEventType distinguishes creation from termination.
type SessionEventType string

const (
	EventSessionCreated    SessionEventType = "session_created"
	EventSessionTerminated SessionEventType = "session_terminated"
)

// SessionEvent is emitted by the APNAdapter when a device session changes state.
// Spec reference: Appendix B.1
type SessionEvent struct {
	Type    SessionEventType `json:"event"`
	Session Session          `json:"session"`
}
