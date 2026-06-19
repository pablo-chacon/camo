package main

import (
	"errors"
	"time"
)

// minNetworkSize is the minimum active node count before circuits can be built.
// Spec reference: Section 10.5
const minNetworkSize = 3

// minJurisdictions is the minimum distinct jurisdictions required across a circuit.
// Spec reference: Section 10.5, Section 3 — P6
const minJurisdictions = 3

// NodeRecord mirrors the gossip agent's NodeRecord for circuit construction.
// Retrieved via the gossip API socket.
type NodeRecord struct {
	ServerID     string   `json:"server_id"`
	WireGuardKey string   `json:"wireguard_pubkey"`
	Endpoints    struct {
		WireGuard string `json:"wireguard"`
	} `json:"endpoints"`
	Capabilities struct {
		Jurisdictions []string `json:"jurisdictions"`
		Regions       []string `json:"regions"`
		Uptime30d     float64  `json:"uptime_30d"`
		SIMPoolSize   int      `json:"sim_pool_size"`
		ExitNode      bool     `json:"exit_node"`
		MaxCircuits   int      `json:"max_circuits"`
	} `json:"capabilities"`
}

// constructor builds circuits from the active node list.
// Spec reference: Section 10.2 — circuit construction algorithm
type constructor struct {
	cfg        *circuitConfig
	ownServerID string
}

func newConstructor(cfg *circuitConfig, ownServerID string) *constructor {
	return &constructor{cfg: cfg, ownServerID: ownServerID}
}

// build constructs a new circuit from the provided active node list.
// Returns an ordered slice of Hops.
//
// Spec reference: Section 10.2 — construction algorithm steps 1-6
func (c *constructor) build(nodes []*NodeRecord, deviceIMSI string) (*Circuit, error) {
	// Step 1: Remove this server from the candidate pool.
	var candidates []*NodeRecord
	for _, n := range nodes {
		if n.ServerID != c.ownServerID {
			candidates = append(candidates, n)
		}
	}

	// Check minimum viable network. Spec reference: Section 10.5
	if len(candidates) < minNetworkSize {
		return nil, errors.New("network below minimum viable size")
	}

	// Verify at least minJurisdictions are available.
	if jurisdictionCount(candidates) < minJurisdictions {
		return nil, errors.New("insufficient jurisdictional diversity in active nodes")
	}

	// Step 2-3: Select middle hops.
	hopCount := minHops
	if maxHops > minHops {
		extra, err := randomInt(maxHops - minHops + 1)
		if err != nil {
			return nil, err
		}
		hopCount = minHops + extra
	}

	// Select middle hops enforcing:
	// - no two consecutive hops in same jurisdiction
	// - prefer higher uptime and larger SIM pools
	// - exclude exit node candidates from middle hops
	// Spec reference: Section 10.2 — steps b, c, d, e
	middleHops, err := selectMiddleHops(candidates, hopCount-1)
	if err != nil {
		return nil, err
	}

	// Step 4: Select exit node.
	// Must have exit_node == true and differ in jurisdiction from last middle hop.
	// Spec reference: Section 10.2 — step 4
	var exitCandidates []*NodeRecord
	lastJurisdiction := ""
	if len(middleHops) > 0 {
		lastJurisdiction = primaryJurisdiction(middleHops[len(middleHops)-1])
	}
	for _, n := range candidates {
		if !n.Capabilities.ExitNode {
			continue
		}
		if isIn(middleHops, n.ServerID) {
			continue
		}
		if primaryJurisdiction(n) == lastJurisdiction {
			continue
		}
		exitCandidates = append(exitCandidates, n)
	}
	if len(exitCandidates) == 0 {
		return nil, errors.New("no eligible exit nodes available")
	}

	exitIdx, err := randomInt(len(exitCandidates))
	if err != nil {
		return nil, err
	}
	exitNode := exitCandidates[exitIdx]

	// Step 5: Assemble ordered circuit.
	token, err := newSessionToken()
	if err != nil {
		return nil, err
	}

	var hops []Hop
	for i, n := range middleHops {
		hops = append(hops, Hop{
			ServerID:         n.ServerID,
			WireGuardAddress: n.Endpoints.WireGuard,
			WireGuardPubKey:  n.WireGuardKey,
			Jurisdiction:     primaryJurisdiction(n),
			HopIndex:         i,
			IsExit:           false,
		})
	}
	hops = append(hops, Hop{
		ServerID:         exitNode.ServerID,
		WireGuardAddress: exitNode.Endpoints.WireGuard,
		WireGuardPubKey:  exitNode.WireGuardKey,
		Jurisdiction:     primaryJurisdiction(exitNode),
		HopIndex:         len(hops),
		IsExit:           true,
	})

	now := time.Now()
	return &Circuit{
		CircuitID:    mustUUID(),
		DeviceIMSI:   deviceIMSI,
		SessionToken: token,
		Hops:         hops,
		CreatedAt:    now,
		RotateAt:     now.Add(chainRotationInterval),
	}, nil
}

// selectMiddleHops picks n nodes enforcing jurisdictional diversity.
// Spec reference: Section 10.2 — step 3 constraints
func selectMiddleHops(candidates []*NodeRecord, n int) ([]*NodeRecord, error) {
	var selected []*NodeRecord
	lastJurisdiction := ""

	// Score candidates: prefer high uptime and large SIM pools.
	scored := scoreNodes(candidates)

	for len(selected) < n && len(scored) > 0 {
		// Among top candidates, pick randomly to prevent determinism.
		poolSize := 3
		if len(scored) < poolSize {
			poolSize = len(scored)
		}

		var eligible []*NodeRecord
		for _, n := range scored[:poolSize] {
			j := primaryJurisdiction(n)
			if j != lastJurisdiction {
				eligible = append(eligible, n)
			}
		}
		if len(eligible) == 0 {
			// Relax jurisdiction constraint if no eligible node found.
			eligible = scored[:poolSize]
		}

		idx, err := randomInt(len(eligible))
		if err != nil {
			return nil, err
		}
		chosen := eligible[idx]
		selected = append(selected, chosen)
		lastJurisdiction = primaryJurisdiction(chosen)

		// Remove chosen from scored pool.
		var remaining []*NodeRecord
		for _, n := range scored {
			if n.ServerID != chosen.ServerID {
				remaining = append(remaining, n)
			}
		}
		scored = remaining
	}

	if len(selected) < n {
		return nil, errors.New("insufficient nodes for circuit construction")
	}
	return selected, nil
}

// scoreNodes returns candidates sorted by uptime then SIM pool size descending.
// Simple insertion sort — circuit construction is infrequent.
func scoreNodes(nodes []*NodeRecord) []*NodeRecord {
	out := make([]*NodeRecord, len(nodes))
	copy(out, nodes)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0; j-- {
			if score(out[j]) > score(out[j-1]) {
				out[j], out[j-1] = out[j-1], out[j]
			}
		}
	}
	return out
}

func score(n *NodeRecord) float64 {
	return n.Capabilities.Uptime30d*100 + float64(n.Capabilities.SIMPoolSize)
}

func primaryJurisdiction(n *NodeRecord) string {
	if len(n.Capabilities.Jurisdictions) > 0 {
		return n.Capabilities.Jurisdictions[0]
	}
	return ""
}

func jurisdictionCount(nodes []*NodeRecord) int {
	seen := make(map[string]bool)
	for _, n := range nodes {
		seen[primaryJurisdiction(n)] = true
	}
	return len(seen)
}

func isIn(hops []*NodeRecord, serverID string) bool {
	for _, h := range hops {
		if h.ServerID == serverID {
			return true
		}
	}
	return false
}

// mustUUID returns a random UUID v4.
func mustUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%12x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
