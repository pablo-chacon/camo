package main

import (
	"bufio"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// circuitConfig holds runtime configuration.
type circuitConfig struct {
	// OwnServerID is this node's server_id from camo-gossip.
	// Read from gossip API at startup.
	OwnServerID string `json:"own_server_id"`

	// RotationInterval overrides the default circuit rotation interval.
	RotationInterval int `json:"rotation_interval"`

	// GossipSocket is the path to the camo-gossip Unix socket.
	GossipSocket string `json:"gossip_socket"`

	// WireGuardSocket is the path to the camo-wireguard Unix socket.
	WireGuardSocket string `json:"wireguard_socket"`

	// APNSocket is the path to the camo-apncore Unix socket.
	APNSocket string `json:"apn_socket"`

	// SIMPoolSocket is the path to the camo-simpool Unix socket.
	SIMPoolSocket string `json:"simpool_socket"`
}

func loadCircuitConfig(path string) (*circuitConfig, error) {
	cfg := &circuitConfig{
		GossipSocket:    "/run/camo/gossip.sock",
		WireGuardSocket: "/run/camo/wireguard.sock",
		APNSocket:       "/run/apncore.sock",
		SIMPoolSocket:   "/run/camo/simpool.sock",
		RotationInterval: int(chainRotationInterval.Seconds()),
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	return cfg, json.Unmarshal(data, cfg)
}

// rotationManager handles circuit lifecycle for all active device sessions.
// Spec reference: Section 10.3 — circuit rotation
type rotationManager struct {
	cfg         *circuitConfig
	store       *circuitStore
	constructor *constructor
}

func newRotationManager(cfg *circuitConfig, st *circuitStore, con *constructor) *rotationManager {
	return &rotationManager{cfg: cfg, store: st, constructor: con}
}

// run starts the rotation ticker. Checks each circuit's RotateAt time.
func (r *rotationManager) run() {
	ticker := time.NewTicker(30 * time.Second) // check every 30s, rotate when due
	defer ticker.Stop()
	for range ticker.C {
		r.checkRotations()
	}
}

// checkRotations rotates any circuit past its RotateAt time.
func (r *rotationManager) checkRotations() {
	for _, c := range r.store.all() {
		if time.Now().After(c.RotateAt) {
			if err := r.rotate(c); err != nil {
				slog.Error("circuit rotation failed",
					"imsi", c.DeviceIMSI,
					"circuit_id", c.CircuitID,
					"err", err,
				)
			}
		}
	}
}

// rotate replaces a circuit with a fresh one.
// Spec reference: Section 10.3 — rotation steps
func (r *rotationManager) rotate(old *Circuit) error {
	nodes, err := queryGossipPeers(r.cfg.GossipSocket)
	if err != nil {
		return fmt.Errorf("gossip query failed: %w", err)
	}

	newCircuit, err := r.constructor.build(nodes, old.DeviceIMSI)
	if err != nil {
		return fmt.Errorf("circuit construction failed: %w", err)
	}

	// Program new WireGuard peers.
	for _, hop := range newCircuit.Hops {
		if err := addWGPeer(r.cfg.WireGuardSocket, hop.WireGuardPubKey, hop.WireGuardAddress, hop.WireGuardAddress+"/32"); err != nil {
			slog.Warn("failed to add WireGuard peer", "server_id", hop.ServerID, "err", err)
		}
	}

	// Store new circuit. Old circuit remains active briefly for overlap.
	r.store.put(newCircuit)

	// Brief overlap window — allow in-flight packets to drain.
	go func() {
		time.Sleep(5 * time.Second)
		// Remove old WireGuard peers.
		for _, hop := range old.Hops {
			// Only remove if not also in the new circuit.
			if !hopInCircuit(newCircuit, hop.ServerID) {
				_ = removeWGPeer(r.cfg.WireGuardSocket, hop.WireGuardPubKey)
			}
		}
	}()

	slog.Info("circuit rotated",
		"imsi", old.DeviceIMSI,
		"old_circuit", old.CircuitID,
		"new_circuit", newCircuit.CircuitID,
		"hops", len(newCircuit.Hops),
	)
	return nil
}

// hopInCircuit returns true if serverID appears in any hop of the circuit.
func hopInCircuit(c *Circuit, serverID string) bool {
	for _, h := range c.Hops {
		if h.ServerID == serverID {
			return true
		}
	}
	return false
}

// queryGossipPeers retrieves the active node list from camo-gossip.
func queryGossipPeers(socketPath string) ([]*NodeRecord, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	req := map[string]string{"command": "list_peers"}
	enc := json.NewEncoder(conn)
	if err := enc.Encode(req); err != nil {
		return nil, err
	}

	var resp struct {
		Status  string        `json:"status"`
		Records []*NodeRecord `json:"records"`
	}
	dec := json.NewDecoder(conn)
	if err := dec.Decode(&resp); err != nil {
		return nil, err
	}
	return resp.Records, nil
}

// addWGPeer sends an add_peer command to camo-wireguard.
func addWGPeer(socketPath, pubKey, endpoint, allowedIP string) error {
	return wgCommand(socketPath, map[string]string{
		"command":    "add_peer",
		"public_key": pubKey,
		"endpoint":   endpoint,
		"allowed_ip": allowedIP,
	})
}

// removeWGPeer sends a remove_peer command to camo-wireguard.
func removeWGPeer(socketPath, pubKey string) error {
	return wgCommand(socketPath, map[string]string{
		"command":    "remove_peer",
		"public_key": pubKey,
	})
}

func wgCommand(socketPath string, cmd map[string]string) error {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return err
	}
	defer conn.Close()
	enc := json.NewEncoder(conn)
	if err := enc.Encode(cmd); err != nil {
		return err
	}
	var resp map[string]string
	scanner := bufio.NewScanner(conn)
	if scanner.Scan() {
		_ = json.Unmarshal(scanner.Bytes(), &resp)
	}
	if resp["status"] != "ok" {
		return fmt.Errorf("wg command failed: %s", resp["message"])
	}
	return nil
}

// randomInt returns a cryptographically random integer in [0, n).
func randomInt(n int) (int, error) {
	if n <= 0 {
		return 0, fmt.Errorf("n must be positive")
	}
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return 0, err
	}
	v := int(b[0])<<24 | int(b[1])<<16 | int(b[2])<<8 | int(b[3])
	if v < 0 {
		v = -v
	}
	return v % n, nil
}

const circuitConfigPath = "/etc/camo/circuit.json"

func main() {
	slog.Info("camo-circuit starting")

	cfg, err := loadCircuitConfig(circuitConfigPath)
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	// Retrieve own server_id from gossip agent.
	if cfg.OwnServerID == "" {
		conn, err := net.Dial("unix", cfg.GossipSocket)
		if err != nil {
			slog.Error("cannot connect to gossip agent", "err", err)
			os.Exit(1)
		}
		enc := json.NewEncoder(conn)
		_ = enc.Encode(map[string]string{"command": "node_info"})
		var resp struct {
			Record struct {
				ServerID string `json:"server_id"`
			} `json:"record"`
		}
		dec := json.NewDecoder(conn)
		_ = dec.Decode(&resp)
		conn.Close()
		cfg.OwnServerID = resp.Record.ServerID
	}

	slog.Info("circuit controller identity", "server_id", cfg.OwnServerID)

	st := newCircuitStore()
	con := newConstructor(cfg, cfg.OwnServerID)
	rot := newRotationManager(cfg, st, con)

	go rot.run()

	slog.Info("camo-circuit running",
		"rotation_interval", cfg.RotationInterval,
		"gossip", cfg.GossipSocket,
	)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	slog.Info("camo-circuit shutting down")
}
