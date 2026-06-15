// camo-gossip implements the CAMO node discovery and announcement protocol.
//
// Spec reference: Section 9 — Node Discovery — Signed Gossip Protocol
//
// On startup:
//   - Loads configuration from /etc/camo/gossip.json
//   - Loads or generates the node Ed25519 keypair (Section 5.4)
//   - Builds the own NodeRecord and signs it (Section 5.5)
//   - Connects to bootstrap peers and announces own record (Section 9.2)
//   - Starts the TCP gossip listener
//   - Starts the heartbeat broadcaster (Section 9.2)
//   - Starts the local Unix socket API (for camo-circuit to query peers)
//
// Other components query the gossip agent via /run/camo/gossip.sock
package main

import (
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const configPath = "/etc/camo/gossip.json"

func main() {
	slog.Info("camo-gossip starting", "version", protocolVersion)

	cfg, err := loadConfig(configPath)
	if err != nil {
		slog.Error("failed to load config", "path", configPath, "err", err)
		os.Exit(1)
	}

	// Load or generate the node's persistent Ed25519 keypair.
	// Spec reference: Section 5.4
	key, err := loadOrGenerateKey(cfg.DataDir)
	if err != nil {
		slog.Error("failed to load node key", "err", err)
		os.Exit(1)
	}
	slog.Info("node identity loaded", "server_id", key.serverID())

	// Build this node's NodeRecord.
	// Spec reference: Section 5.5, Appendix A.1
	ownRecord := &NodeRecord{
		Version:      protocolVersion,
		ServerID:     key.serverID(),
		WireGuardKey: cfg.WireGuardPubKey,
		Endpoints: Endpoints{
			WireGuard: cfg.WireGuardAddr,
			GTP:       cfg.GTPAddr,
		},
		Capabilities: Capabilities{
			Regions:       cfg.Regions,
			Jurisdictions: cfg.Jurisdictions,
			ExitNode:      cfg.ExitNode,
			MaxCircuits:   cfg.MaxCircuits,
			Uptime30d:     1.0, // updated by monitoring in production
		},
		Timestamp: time.Now().UTC(),
	}
	if err := signRecord(ownRecord, key); err != nil {
		slog.Error("failed to sign own record", "err", err)
		os.Exit(1)
	}

	st := newStore()
	pm := newPeerManager(cfg.BootstrapPeers)

	// Store own record so it is returned in list_peers responses.
	st.put(ownRecord)

	// Connect to bootstrap peers and announce own record.
	// Spec reference: Section 9.2 — peer initialization
	payload, _ := json.Marshal(ownRecord)
	msg := Message{Type: msgNodeRecord, Payload: payload}
	encoded, _ := json.Marshal(msg)
	for _, addr := range cfg.BootstrapPeers {
		slog.Info("announcing to bootstrap peer", "addr", addr)
		sendLine(addr, encoded)
	}

	srv := newServer(cfg, key, st, pm)
	bc := newBroadcaster(cfg, key, st, pm, ownRecord)
	api := newAPIServer(st, ownRecord)

	// Start gossip TCP listener.
	go func() {
		if err := srv.listen(); err != nil {
			slog.Error("gossip server error", "err", err)
			os.Exit(1)
		}
	}()

	// Start heartbeat broadcaster and expiry checker.
	go bc.run()

	// Start local API server for other components.
	go func() {
		if err := api.listen(); err != nil {
			slog.Error("gossip API error", "err", err)
			os.Exit(1)
		}
	}()

	slog.Info("camo-gossip running",
		"server_id", key.serverID(),
		"listen", cfg.ListenAddr,
		"bootstrap_peers", len(cfg.BootstrapPeers),
	)

	// Wait for termination signal.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	slog.Info("camo-gossip shutting down")
}
