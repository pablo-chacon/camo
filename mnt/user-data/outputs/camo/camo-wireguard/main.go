// Package main implements the CAMO WireGuard peer lifecycle manager.
//
// Spec reference: Section 8 — Inter-Node Encryption — WireGuard
//
// Manages the WireGuard interface used for inter-node tunnels.
// Peers are added dynamically when circuits are constructed and
// removed when circuits are torn down.
//
// Exposes a Unix socket API for camo-circuit to add/remove peers.
package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// wgSocket is the Unix socket API path for this component.
const wgSocket = "/run/camo/wireguard.sock"

// wgAPIRequest is a command from camo-circuit.
type wgAPIRequest struct {
	// Command is one of: add_peer, remove_peer, list_peers, status
	Command   string `json:"command"`
	PublicKey string `json:"public_key,omitempty"` // base64 WireGuard pubkey
	Endpoint  string `json:"endpoint,omitempty"`   // ip:port
	AllowedIP string `json:"allowed_ip,omitempty"` // CIDR
	PeerID    string `json:"peer_id,omitempty"`    // circuit identifier
}

// wgAPIResponse is the reply.
type wgAPIResponse struct {
	Status  string     `json:"status"`
	Command string     `json:"command"`
	Peers   []wgPeer   `json:"peers,omitempty"`
	Message string     `json:"message,omitempty"`
}

// wgPeer is a summary of a configured WireGuard peer.
type wgPeer struct {
	PublicKey  string    `json:"public_key"`
	Endpoint   string    `json:"endpoint"`
	AllowedIPs []string  `json:"allowed_ips"`
	LastHandshake time.Time `json:"last_handshake"`
}

// wgConfig holds runtime configuration.
type wgConfig struct {
	// InterfaceName is the WireGuard interface to manage.
	InterfaceName string `json:"interface_name"`
	// ListenPort is the WireGuard UDP listen port. Spec reference: Section 8.2
	ListenPort int `json:"listen_port"`
	// PrivateKeyPath is the path to the WireGuard private key file.
	PrivateKeyPath string `json:"private_key_path"`
}

func loadWGConfig(path string) (*wgConfig, error) {
	cfg := &wgConfig{
		InterfaceName:  "camo0",
		ListenPort:     51820,
		PrivateKeyPath: "/var/lib/camo/wg.key",
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

// manager wraps wgctrl to manage the WireGuard interface.
// Spec reference: Section 8.2 — tunnel configuration
type manager struct {
	cfg    *wgConfig
	client *wgctrl.Client
}

func newManager(cfg *wgConfig) (*manager, error) {
	client, err := wgctrl.New()
	if err != nil {
		return nil, err
	}
	return &manager{cfg: cfg, client: client}, nil
}

// addPeer adds a WireGuard peer dynamically.
// Called by camo-circuit when constructing a new circuit hop.
// Spec reference: Section 8.2 — dynamic peer configuration
func (m *manager) addPeer(pubKeyB64, endpoint, allowedIP string) error {
	pubKeyBytes, err := base64.StdEncoding.DecodeString(pubKeyB64)
	if err != nil {
		return err
	}
	var pubKey wgtypes.Key
	copy(pubKey[:], pubKeyBytes)

	keepalive := 25 * time.Second
	peerCfg := wgtypes.PeerConfig{
		PublicKey:                   pubKey,
		ReplaceAllowedIPs:           true,
		PersistentKeepaliveInterval: &keepalive,
	}

	if endpoint != "" {
		addr, err := net.ResolveUDPAddr("udp", endpoint)
		if err != nil {
			return err
		}
		peerCfg.Endpoint = addr
	}

	if allowedIP != "" {
		_, ipNet, err := net.ParseCIDR(allowedIP)
		if err != nil {
			return err
		}
		peerCfg.AllowedIPs = []net.IPNet{*ipNet}
	}

	return m.client.ConfigureDevice(m.cfg.InterfaceName, wgtypes.Config{
		Peers: []wgtypes.PeerConfig{peerCfg},
	})
}

// removePeer removes a WireGuard peer.
// Called by camo-circuit when tearing down a circuit hop.
// Spec reference: Section 10.3 — old circuit teardown
func (m *manager) removePeer(pubKeyB64 string) error {
	pubKeyBytes, err := base64.StdEncoding.DecodeString(pubKeyB64)
	if err != nil {
		return err
	}
	var pubKey wgtypes.Key
	copy(pubKey[:], pubKeyBytes)

	return m.client.ConfigureDevice(m.cfg.InterfaceName, wgtypes.Config{
		Peers: []wgtypes.PeerConfig{
			{PublicKey: pubKey, Remove: true},
		},
	})
}

// listPeers returns all configured peers on the interface.
func (m *manager) listPeers() ([]wgPeer, error) {
	device, err := m.client.Device(m.cfg.InterfaceName)
	if err != nil {
		return nil, err
	}
	var out []wgPeer
	for _, p := range device.Peers {
		var allowedIPs []string
		for _, ip := range p.AllowedIPs {
			allowedIPs = append(allowedIPs, ip.String())
		}
		ep := ""
		if p.Endpoint != nil {
			ep = p.Endpoint.String()
		}
		out = append(out, wgPeer{
			PublicKey:     base64.StdEncoding.EncodeToString(p.PublicKey[:]),
			Endpoint:      ep,
			AllowedIPs:    allowedIPs,
			LastHandshake: p.LastHandshakeTime,
		})
	}
	return out, nil
}

// apiServer serves the Unix socket API for camo-circuit.
type apiServer struct {
	mgr *manager
}

func newWGAPIServer(mgr *manager) *apiServer {
	return &apiServer{mgr: mgr}
}

func (s *apiServer) listen() error {
	_ = os.Remove(wgSocket)
	if err := os.MkdirAll("/run/camo", 0700); err != nil {
		return err
	}
	ln, err := net.Listen("unix", wgSocket)
	if err != nil {
		return err
	}
	slog.Info("wireguard API listening", "socket", wgSocket)
	for {
		conn, err := ln.Accept()
		if err != nil {
			slog.Error("wireguard API accept", "err", err)
			continue
		}
		go s.handleConn(conn)
	}
}

func (s *apiServer) handleConn(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	enc := json.NewEncoder(conn)

	for scanner.Scan() {
		var req wgAPIRequest
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			_ = enc.Encode(wgAPIResponse{Status: "error", Message: "invalid request"})
			continue
		}
		switch req.Command {
		case "add_peer":
			if err := s.mgr.addPeer(req.PublicKey, req.Endpoint, req.AllowedIP); err != nil {
				_ = enc.Encode(wgAPIResponse{Status: "error", Command: req.Command, Message: err.Error()})
				continue
			}
			slog.Info("peer added", "pubkey", req.PublicKey[:8]+"...", "endpoint", req.Endpoint)
			_ = enc.Encode(wgAPIResponse{Status: "ok", Command: req.Command})

		case "remove_peer":
			if err := s.mgr.removePeer(req.PublicKey); err != nil {
				_ = enc.Encode(wgAPIResponse{Status: "error", Command: req.Command, Message: err.Error()})
				continue
			}
			slog.Info("peer removed", "pubkey", req.PublicKey[:8]+"...")
			_ = enc.Encode(wgAPIResponse{Status: "ok", Command: req.Command})

		case "list_peers":
			peers, err := s.mgr.listPeers()
			if err != nil {
				_ = enc.Encode(wgAPIResponse{Status: "error", Command: req.Command, Message: err.Error()})
				continue
			}
			_ = enc.Encode(wgAPIResponse{Status: "ok", Command: req.Command, Peers: peers})

		default:
			_ = enc.Encode(wgAPIResponse{Status: "error", Command: req.Command, Message: "unknown command"})
		}
	}
}

const wgConfigPath = "/etc/camo/wireguard.json"

func main() {
	slog.Info("camo-wireguard starting")

	cfg, err := loadWGConfig(wgConfigPath)
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	mgr, err := newManager(cfg)
	if err != nil {
		slog.Error("failed to create WireGuard manager", "err", err)
		os.Exit(1)
	}
	defer mgr.client.Close()

	srv := newWGAPIServer(mgr)
	go func() {
		if err := srv.listen(); err != nil {
			slog.Error("wireguard API error", "err", err)
			os.Exit(1)
		}
	}()

	slog.Info("camo-wireguard running",
		"interface", cfg.InterfaceName,
		"port", cfg.ListenPort,
	)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	slog.Info("camo-wireguard shutting down")
}
