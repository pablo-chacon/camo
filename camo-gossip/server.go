package main

import (
	"bufio"
	"encoding/json"
	"log/slog"
	"net"
)

// server listens for incoming gossip connections and processes
// NodeRecord and Heartbeat messages from peers.
// Spec reference: Section 9.2 — gossip propagation
type server struct {
	cfg   *config
	key   *nodeKey
	store *store
	peers *peerManager
}

func newServer(cfg *config, key *nodeKey, st *store, pm *peerManager) *server {
	return &server{cfg: cfg, key: key, store: st, peers: pm}
}

// listen starts the TCP gossip listener.
func (s *server) listen() error {
	ln, err := net.Listen("tcp", s.cfg.ListenAddr)
	if err != nil {
		return err
	}
	slog.Info("gossip listener started", "addr", s.cfg.ListenAddr)
	for {
		conn, err := ln.Accept()
		if err != nil {
			slog.Error("accept error", "err", err)
			continue
		}
		go s.handleConn(conn)
	}
}

// handleConn processes an inbound gossip connection.
// Reads newline-delimited JSON messages until the connection closes.
func (s *server) handleConn(conn net.Conn) {
	defer conn.Close()
	remoteAddr := conn.RemoteAddr().String()
	s.peers.add(remoteAddr)
	s.peers.seen(remoteAddr)

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Bytes()
		var msg Message
		if err := json.Unmarshal(line, &msg); err != nil {
			slog.Warn("malformed message", "addr", remoteAddr, "err", err)
			continue
		}
		switch msg.Type {
		case msgNodeRecord:
			s.handleNodeRecord(msg.Payload, remoteAddr)
		case msgHeartbeat:
			s.handleHeartbeat(msg.Payload, remoteAddr)
		default:
			slog.Warn("unknown message type", "type", msg.Type, "addr", remoteAddr)
		}
	}
}

// handleNodeRecord validates and stores an incoming NodeRecord,
// then forwards it to all known peers except the sender.
// Spec reference: Section 9.2 — propagation rules
func (s *server) handleNodeRecord(payload json.RawMessage, fromAddr string) {
	var r NodeRecord
	if err := json.Unmarshal(payload, &r); err != nil {
		slog.Warn("invalid node_record payload", "err", err)
		return
	}

	// Discard our own records.
	if r.ServerID == s.key.serverID() {
		return
	}

	// Verify signature before storing or forwarding.
	if err := verifyRecord(&r); err != nil {
		slog.Warn("node_record signature invalid", "server_id", r.ServerID, "err", err)
		return
	}

	// Deduplication check.
	key := recordKey(&r)
	if s.store.isSeen(key) {
		return
	}

	// Store the record.
	if !s.store.put(&r) {
		return // already stored by a concurrent goroutine
	}

	slog.Info("received node_record", "server_id", r.ServerID, "from", fromAddr)

	// Forward to all known peers except the sender.
	// Spec reference: Section 9.2 — forward to all known peers except P
	go s.forwardRecord(&r, fromAddr)
}

// handleHeartbeat updates liveness for the sending node.
// Spec reference: Section 9.2 — heartbeat handling
func (s *server) handleHeartbeat(payload json.RawMessage, fromAddr string) {
	var h Heartbeat
	if err := json.Unmarshal(payload, &h); err != nil {
		slog.Warn("invalid heartbeat payload", "err", err)
		return
	}

	// Retrieve the known public key for this server_id.
	record := s.store.get(h.ServerID)
	if record == nil {
		// Unknown node — cannot verify. Discard.
		return
	}

	if err := verifyHeartbeat(&h, record.ServerID); err != nil {
		slog.Warn("heartbeat signature invalid", "server_id", h.ServerID, "err", err)
		return
	}

	s.store.heartbeat(h.ServerID)
	s.peers.seen(fromAddr)
}

// forwardRecord sends a NodeRecord to all active peers except the excluded address.
func (s *server) forwardRecord(r *NodeRecord, excludeAddr string) {
	payload, err := json.Marshal(r)
	if err != nil {
		return
	}
	msg := Message{Type: msgNodeRecord, Payload: payload}
	encoded, err := json.Marshal(msg)
	if err != nil {
		return
	}

	for _, addr := range s.peers.activeAddrs() {
		if addr == excludeAddr {
			continue
		}
		sendLine(addr, encoded)
	}
}

// sendLine opens a short-lived TCP connection to addr and sends one JSON line.
func sendLine(addr string, line []byte) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.Write(append(line, '\n'))
}
