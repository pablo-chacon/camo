package main

import (
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

const simpoolConfigPath = "/etc/camo/simpool.json"

// config holds runtime configuration for the SIM pool manager.
type config struct {
	// SIMRotationInterval in seconds. Default 480.
	// Spec reference: Section 7.5
	SIMRotationInterval int `json:"sim_rotation_interval"`

	// SIMs is the initial pool inventory loaded from config.
	// In production these are added by the operator after SIM provisioning.
	SIMs []SIM `json:"sims"`

	// UseMockChecker disables real carrier liveness checks.
	// Set to true in development environments without M2M infrastructure.
	UseMockChecker bool `json:"use_mock_checker"`
}

func loadSimpoolConfig(path string) (*config, error) {
	cfg := &config{
		SIMRotationInterval: 480,
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// camo-simpool main entry point.
//
// Spec reference: Section 7 — SIM Pool Management
//
// Starts:
//   - SIM pool with inventory from config
//   - Active subset rotator (Section 7.5)
//   - Liveness monitor (Section 7.9)
//   - Unix socket API for camo-circuit (Section 7.6)
func main() {
	slog.Info("camo-simpool starting")

	cfg, err := loadSimpoolConfig(simpoolConfigPath)
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	p := newPool()

	// Load SIM inventory from config.
	for i := range cfg.SIMs {
		sim := cfg.SIMs[i]
		p.add(&sim)
		slog.Info("registered SIM", "imsi", sim.IMSI, "carrier", sim.Carrier, "jurisdiction", sim.Jurisdiction)
	}

	total, _, _ := p.stats()
	slog.Info("pool loaded", "total_sims", total)

	// Select liveness checker.
	var checker SIMChecker
	if cfg.UseMockChecker {
		slog.Warn("using mock SIM checker — not suitable for production")
		checker = &noopChecker{}
	} else {
		checker = &noopChecker{} // replace with real carrier checker implementation
	}

	rot := newRotator(p, cfg)
	liv := newLiveness(p, checker)
	api := newSIMAPIServer(p)

	go rot.run()
	go liv.run()
	go func() {
		if err := api.listen(); err != nil {
			slog.Error("simpool API error", "err", err)
			os.Exit(1)
		}
	}()

	slog.Info("camo-simpool running", "total_sims", total)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	slog.Info("camo-simpool shutting down")
}
