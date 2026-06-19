package main

import (
	"encoding/json"
	"log/slog"
	"time"
)

// heartbeatInterval is how often this node sends a heartbeat to all peers.
// Spec reference: Section 9.2 — servers send heartbeat every 60 seconds.
const heartbeatInterval = 60 * time.Second

// expiryCheckInterval is how often the store runs its expiry pass.
const expiryCheckInterval = 60 * time.Second

// announceInterval is how often this node re-announces its own NodeRecord.
// Spec reference: Section 9.2 — re-announce at intervals not exceeding 12 hours.
const announceInterval = 6 * time.Hour

// broadcaster handles outbound heartbeats and periodic re-announcements.
type broadcaster struct {
	cfg    *config
	key    *nodeKey
	store  *store
	peers  *peerManager
	record *NodeRecord
}

func newBroadcaster(cfg *config, key *nodeKey, st *store, pm *peerManager, r *NodeRecord) *broadcaster {
	return &broadcaster{cfg: cfg, key: key, store: st, peers: pm, record: r}
}

// run starts the heartbeat ticker and announcement ticker.
// Blocks until the process exits.
func (b *broadcaster) run() {
	heartbeatTicker := time.NewTicker(heartbeatInterval)
	expiryTicker := time.NewTicker(expiryCheckInterval)
	announceTicker := time.NewTicker(announceInterval)
	defer heartbeatTicker.Stop()
	defer expiryTicker.Stop()
	defer announceTicker.Stop()

	for {
		select {
		case <-heartbeatTicker.C:
			b.sendHeartbeat()
		case <-expiryTicker.C:
			b.store.expireAndCheck()
		case <-announceTicker.C:
			b.announce()
		}
	}
}

// sendHeartbeat broadcasts a signed heartbeat to all active peers.
// Spec reference: Section 9.2, Appendix A.2
func (b *broadcaster) sendHeartbeat() {
	h := &Heartbeat{
		ServerID:  b.key.serverID(),
		Timestamp: time.Now().UTC(),
		// ActiveCircuits and SIMPoolAvailable would be queried
		// from camo-circuit and camo-simpool in production.
		// Set to 0 here; the gossip component does not own this state.
	}
	if err := signHeartbeat(h, b.key); err != nil {
		slog.Error("failed to sign heartbeat", "err", err)
		return
	}

	payload, err := json.Marshal(h)
	if err != nil {
		return
	}
	msg := Message{Type: msgHeartbeat, Payload: payload}
	encoded, err := json.Marshal(msg)
	if err != nil {
		return
	}

	for _, addr := range b.peers.activeAddrs() {
		sendLine(addr, encoded)
	}
	slog.Debug("heartbeat sent", "peers", len(b.peers.activeAddrs()))
}

// announce re-broadcasts this node's own NodeRecord to all active peers.
// Spec reference: Section 9.2 — re-announce at intervals not exceeding 12 hours.
func (b *broadcaster) announce() {
	// Update timestamp so peers do not expire our record.
	b.record.Timestamp = time.Now().UTC()
	if err := signRecord(b.record, b.key); err != nil {
		slog.Error("failed to sign record for announcement", "err", err)
		return
	}

	payload, err := json.Marshal(b.record)
	if err != nil {
		return
	}
	msg := Message{Type: msgNodeRecord, Payload: payload}
	encoded, err := json.Marshal(msg)
	if err != nil {
		return
	}

	for _, addr := range b.peers.activeAddrs() {
		sendLine(addr, encoded)
	}
	slog.Info("re-announced node record", "peers", len(b.peers.activeAddrs()))
}
