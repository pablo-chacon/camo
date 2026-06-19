package main

import (
	"encoding/json"
	"errors"
	"time"
)

// Protocol version this node announces.
const protocolVersion = "0.1"

// NodeRecord is the signed announcement a distribution server broadcasts
// to the gossip network. All fields are included in the signature.
// Spec reference: Section 5.5, Appendix A.1
type NodeRecord struct {
	Version      string       `json:"version"`
	ServerID     string       `json:"server_id"`
	WireGuardKey string       `json:"wireguard_pubkey"`
	Endpoints    Endpoints    `json:"endpoints"`
	Capabilities Capabilities `json:"capabilities"`
	Timestamp    time.Time    `json:"timestamp"`
	Signature    string       `json:"signature,omitempty"`
}

// Endpoints describes the network addresses the node listens on.
type Endpoints struct {
	WireGuard string `json:"wireguard"` // ip:port
	GTP       string `json:"gtp"`       // ip:port
}

// Capabilities describes what this node offers to the network.
// Used during circuit construction for node selection.
// Spec reference: Section 10.2
type Capabilities struct {
	SIMPoolSize   int      `json:"sim_pool_size"`
	Regions       []string `json:"regions"`
	Jurisdictions []string `json:"jurisdictions"`
	Uptime30d     float64  `json:"uptime_30d"`
	ExitNode      bool     `json:"exit_node"`
	MaxCircuits   int      `json:"max_circuits"`
}

// Heartbeat is a liveness signal sent every 60 seconds.
// Spec reference: Section 9.2, Appendix A.2
type Heartbeat struct {
	ServerID         string    `json:"server_id"`
	Timestamp        time.Time `json:"timestamp"`
	ActiveCircuits   int       `json:"active_circuits"`
	SIMPoolAvailable int       `json:"sim_pool_available"`
	Signature        string    `json:"signature,omitempty"`
}

// Message wraps a gossip protocol payload with a type discriminator.
// Used for framing over the TCP gossip connection.
type Message struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

const (
	msgNodeRecord = "node_record"
	msgHeartbeat  = "heartbeat"
)

// canonicalJSON returns the JSON representation of v with the signature
// field zeroed out, suitable for signing or verification.
// Signing is over the canonical form — signature field excluded.
func canonicalJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// signRecord signs a NodeRecord using the node's keypair.
// The signature covers all fields except the signature field itself.
func signRecord(r *NodeRecord, key *nodeKey) error {
	// Temporarily clear signature for canonical form.
	r.Signature = ""
	data, err := canonicalJSON(r)
	if err != nil {
		return err
	}
	r.Signature = key.sign(data)
	return nil
}

// verifyRecord verifies the signature on a NodeRecord.
// Returns an error if the signature is invalid or missing.
func verifyRecord(r *NodeRecord) error {
	if r.Signature == "" {
		return errors.New("missing signature")
	}
	sig := r.Signature
	r.Signature = ""
	data, err := canonicalJSON(r)
	r.Signature = sig
	if err != nil {
		return err
	}
	return verify(r.ServerID, sig, data)
}

// signHeartbeat signs a Heartbeat using the node's keypair.
func signHeartbeat(h *Heartbeat, key *nodeKey) error {
	h.Signature = ""
	data, err := canonicalJSON(h)
	if err != nil {
		return err
	}
	h.Signature = key.sign(data)
	return nil
}

// verifyHeartbeat verifies the signature on a Heartbeat against
// the known public key for that server_id.
func verifyHeartbeat(h *Heartbeat, pubKeyB64 string) error {
	if h.Signature == "" {
		return errors.New("missing signature")
	}
	sig := h.Signature
	h.Signature = ""
	data, err := canonicalJSON(h)
	h.Signature = sig
	if err != nil {
		return err
	}
	return verify(pubKeyB64, sig, data)
}

// recordKey returns the deduplication key for a NodeRecord.
// Spec reference: Section 9.2 — seen-record deduplication by (server_id, timestamp)
func recordKey(r *NodeRecord) string {
	return r.ServerID + "|" + r.Timestamp.UTC().Format(time.RFC3339)
}
