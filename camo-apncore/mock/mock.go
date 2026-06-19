// Package mock provides a mock APNAdapter for development and testing.
// It simulates APN core behavior without requiring real carrier infrastructure.
//
// Spec reference: Section 6 — implementation-agnostic interface
// All behaviors of the real adapter are simulated in memory.
package mock

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Session mirrors the main package Session type to avoid import cycles.
type Session struct {
	IMSI      string    `json:"imsi"`
	GTPTEID   uint32    `json:"gtp_teid"`
	DeviceIP  string    `json:"device_ip"`
	CreatedAt time.Time `json:"created_at"`
}

// RouteRule mirrors the main package RouteRule type.
type RouteRule struct {
	IMSI               string `json:"imsi"`
	NextHopWireGuardIP string `json:"next_hop_wireguard_ip"`
	SessionToken       string `json:"session_token"`
}

// SessionEventType mirrors the main package type.
type SessionEventType string

const (
	EventSessionCreated    SessionEventType = "session_created"
	EventSessionTerminated SessionEventType = "session_terminated"
)

// SessionEvent mirrors the main package type.
type SessionEvent struct {
	Type    SessionEventType
	Session Session
}

// Adapter is the mock APN core adapter.
// Thread-safe. All state is in-memory.
type Adapter struct {
	mu       sync.RWMutex
	sessions map[string]*Session // keyed by IMSI
	routes   map[string]*RouteRule
	events   chan SessionEvent
	teidSeq  uint32
	ipSeq    int
}

// New creates a new mock adapter.
func New() *Adapter {
	return &Adapter{
		sessions: make(map[string]*Session),
		routes:   make(map[string]*RouteRule),
		events:   make(chan SessionEvent, 64),
	}
}

// SetRoute stores a routing rule in memory and logs it.
func (a *Adapter) SetRoute(rule RouteRule) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.routes[rule.IMSI] = &rule
	slog.Info("[mock] route set", "imsi", rule.IMSI, "next_hop", rule.NextHopWireGuardIP)
	return nil
}

// ClearRoute removes a routing rule.
func (a *Adapter) ClearRoute(imsi string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.routes, imsi)
	slog.Info("[mock] route cleared", "imsi", imsi)
	return nil
}

// ListSessions returns all currently active sessions.
func (a *Adapter) ListSessions() ([]Session, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	var out []Session
	for _, s := range a.sessions {
		out = append(out, *s)
	}
	return out, nil
}

// SessionEvents returns the event channel.
func (a *Adapter) SessionEvents() <-chan SessionEvent {
	return a.events
}

// SimulateConnect simulates a device connecting to the APN.
// Used in tests to trigger session_created events.
func (a *Adapter) SimulateConnect(imsi string) {
	a.mu.Lock()
	a.teidSeq++
	a.ipSeq++
	s := &Session{
		IMSI:      imsi,
		GTPTEID:   a.teidSeq,
		DeviceIP:  fmt.Sprintf("10.0.%d.%d", a.ipSeq/256, a.ipSeq%256),
		CreatedAt: time.Now(),
	}
	a.sessions[imsi] = s
	a.mu.Unlock()

	slog.Info("[mock] device connected", "imsi", imsi, "ip", s.DeviceIP)
	a.events <- SessionEvent{Type: EventSessionCreated, Session: *s}
}

// SimulateDisconnect simulates a device disconnecting from the APN.
// Used in tests to trigger session_terminated events.
func (a *Adapter) SimulateDisconnect(imsi string) {
	a.mu.Lock()
	s, ok := a.sessions[imsi]
	if !ok {
		a.mu.Unlock()
		return
	}
	delete(a.sessions, imsi)
	a.mu.Unlock()

	slog.Info("[mock] device disconnected", "imsi", imsi)
	a.events <- SessionEvent{Type: EventSessionTerminated, Session: *s}
}
