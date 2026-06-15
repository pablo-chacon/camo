// camo-apncore implements the CAMO APN core interface.
//
// Spec reference: Section 6 — APN Core Interface, Appendix B
//
// This component bridges the CAMO protocol to the underlying APN core
// software. The APNAdapter interface (adapter.go) is the only thing
// that changes when switching APN core implementations.
//
// Configuration selects the adapter:
//   - "mock"   — in-memory mock, no real APN core required
//   - "open5gs" — adapter for Open5GS (implementation provided separately)
//   - "free5gc" — adapter for free5GC (implementation provided separately)
package main

import (
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/pablochacon/camo/apncore/mock"
)

const apnConfigPath = "/etc/camo/apncore.json"

type apnConfig struct {
	// Adapter selects the APN core implementation.
	// Valid values: "mock", "open5gs", "free5gc"
	Adapter string `json:"adapter"`
}

func loadAPNConfig(path string) (*apnConfig, error) {
	cfg := &apnConfig{Adapter: "mock"}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func main() {
	slog.Info("camo-apncore starting")

	cfg, err := loadAPNConfig(apnConfigPath)
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	var adapter APNAdapter

	switch cfg.Adapter {
	case "mock":
		slog.Warn("using mock APN adapter — not suitable for production")
		m := mock.New()
		// Wrap mock adapter to satisfy the main package interface.
		adapter = &mockAdapterWrapper{m}

	case "open5gs":
		// Replace with: adapter = open5gs.New(cfg)
		slog.Error("open5gs adapter not yet implemented — use 'mock' for development")
		os.Exit(1)

	case "free5gc":
		// Replace with: adapter = free5gc.New(cfg)
		slog.Error("free5gc adapter not yet implemented — use 'mock' for development")
		os.Exit(1)

	default:
		slog.Error("unknown adapter", "adapter", cfg.Adapter)
		os.Exit(1)
	}

	srv := newAPIServer(adapter)
	go func() {
		if err := srv.listen(); err != nil {
			slog.Error("apncore API error", "err", err)
			os.Exit(1)
		}
	}()

	slog.Info("camo-apncore running", "adapter", cfg.Adapter)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	slog.Info("camo-apncore shutting down")
}

// mockAdapterWrapper adapts the mock package types to the main package interface.
// This indirection keeps the mock package self-contained without import cycles.
type mockAdapterWrapper struct {
	m *mock.Adapter
}

func (w *mockAdapterWrapper) SetRoute(rule RouteRule) error {
	return w.m.SetRoute(mock.RouteRule{
		IMSI:               rule.IMSI,
		NextHopWireGuardIP: rule.NextHopWireGuardIP,
		SessionToken:       rule.SessionToken,
	})
}

func (w *mockAdapterWrapper) ClearRoute(imsi string) error {
	return w.m.ClearRoute(imsi)
}

func (w *mockAdapterWrapper) ListSessions() ([]Session, error) {
	ms, err := w.m.ListSessions()
	if err != nil {
		return nil, err
	}
	out := make([]Session, len(ms))
	for i, s := range ms {
		out[i] = Session{
			IMSI:      s.IMSI,
			GTPTEID:   s.GTPTEID,
			DeviceIP:  s.DeviceIP,
			CreatedAt: s.CreatedAt,
		}
	}
	return out, nil
}

func (w *mockAdapterWrapper) SessionEvents() <-chan SessionEvent {
	in := w.m.SessionEvents()
	out := make(chan SessionEvent, 64)
	go func() {
		for e := range in {
			out <- SessionEvent{
				Type: SessionEventType(e.Type),
				Session: Session{
					IMSI:      e.Session.IMSI,
					GTPTEID:   e.Session.GTPTEID,
					DeviceIP:  e.Session.DeviceIP,
					CreatedAt: e.Session.CreatedAt,
				},
			}
		}
	}()
	return out
}
