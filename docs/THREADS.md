# CAMO — Threads & Roadmap

**Project:** Cellular Anonymization and Mobile Onion-routing  
**Updated:** June 2026

This document tracks immediate work items and open research threads. Immediate items are actionable now. Research threads require investigation or design decisions before implementation can begin.

---

## Immediate — Actionable Now

### Specification

- [ ] **Update spec with CAMO name**  
  Replace working title throughout `mobile_onion_routing_spec.md` with CAMO. Update citation block, header, and all self-references.

- [ ] **Add infrastructure migration section to spec**  
  Document the broader architectural implication: CAMO reduces carriers to commodity radio access, enabling independent evolution of communication protocols above the radio layer. Connects CAMO to the broader trajectory of decentralization across digital infrastructure.

- [ ] **Formalize protocol versioning scheme**  
  Define how version numbers work, what constitutes a breaking change, and how nodes running different versions negotiate compatibility. Currently referenced in Section 13.4 but not fully specified.

---

### camo-gossip

- [ ] **Implement NodeRecord signing and verification**  
  Ed25519 keypair generation at first launch, persistent storage, canonical JSON serialization for signing, signature verification on receipt. Per Section 5.4 and 9.2.

- [ ] **Implement gossip propagation loop**  
  Seen-record deduplication by `(server_id, timestamp)` tuple, forward-to-all-except-sender logic, configurable peer list management. Per Section 9.2.

- [ ] **Implement heartbeat sender and receiver**  
  60-second signed heartbeat emission, receipt handling, 300-second liveness timeout, inactive/active state transitions. Per Section 9.2.

- [ ] **Implement NodeRecord expiry**  
  Records older than 24 hours without update are expired and removed. Re-announcement interval enforcement. Per Section 9.2.

- [ ] **Bootstrap node handling**  
  On first launch with no known peers, connect to at least one bootstrap node. Once connected, operate independently. Bootstrap node list configurable, not hardcoded.

- [ ] **Peer persistence**  
  Known peer list persists across restarts. Node does not lose its peer graph on reboot.

---

### camo-circuit

- [ ] **Implement chain construction algorithm**  
  Full implementation of the algorithm in Section 10.2: candidate filtering, jurisdictional diversity enforcement, operator diversity preference, exit node selection, uptime and pool size weighting.

- [ ] **Implement circuit rotation**  
  600-second rotation timer, new circuit construction, session migration with overlap window, old circuit teardown. Per Section 10.3.

- [ ] **Implement session isolation**  
  Each device IMSI gets an independent circuit. No two devices share a circuit. Per Section 10.4.

- [ ] **Implement session token generation and distribution**  
  16-byte random session tokens, encrypted distribution to each hop at circuit construction time. Per Section 11.1.

- [ ] **Implement minimum viable network check**  
  Before constructing a circuit, verify the gossip network has minimum viable node count and jurisdictional diversity. Refuse construction and notify device if below threshold. Per Section 10.5.

---

### camo-apncore

- [ ] **Define Unix socket management API**  
  Implement the full API contract from Appendix B: session_created and session_terminated events, set_route and clear_route commands, list_sessions response. Newline-delimited JSON on `/run/apncore.sock`.

- [ ] **Open5GS adapter**  
  Implement the Appendix B interface contract as an adapter layer over Open5GS. Translates CAMO management commands into Open5GS API calls and translates Open5GS session events into CAMO event format.

- [ ] **free5GC adapter**  
  Same adapter for free5GC. The adapter interface is identical — only the underlying API calls differ.

- [ ] **Per-IMSI routing rule management**  
  Implement set/clear routing rules per IMSI, translating CAMO route commands into the APN core's native routing configuration.

---

### camo-simpool

- [ ] **SIM registration and inventory**  
  Data model for SIM pool entries: IMSI, carrier, jurisdiction, APN, active/inactive status, last health check timestamp.

- [ ] **SIM liveness monitoring**  
  Periodic health checks against each SIM in the pool. Mark unresponsive SIMs inactive. Restore to active on recovery.

- [ ] **Session allocation**  
  Randomized SIM assignment to active sessions. Enforce no consecutive reuse of same SIM across sessions from the same circuit entry. Load distribution across pool.

- [ ] **Pool capacity reporting**  
  Expose current available pool size to gossip agent for NodeRecord `sim_pool_size` field. Updates on allocation/release events.

---

### camo-wireguard

- [ ] **Dynamic peer management**  
  Add WireGuard peers on circuit construction, remove on circuit teardown. Manage AllowedIPs per peer. Per Section 8.2.

- [ ] **WireGuard key management**  
  Separate WireGuard keypair from Ed25519 identity keypair. WireGuard public key included in signed NodeRecord. Per Section 8.3.

- [ ] **Interface health monitoring**  
  Monitor WireGuard interface state. Alert circuit controller on peer connectivity failures.

---

### Deployment

- [ ] **Docker Compose single-node reference deployment**  
  Compose file bringing up all five CAMO components plus a chosen APN core. Volume mounts for keypair persistence. Network configuration for WireGuard and GTP interfaces. Documented setup procedure for a first node.

- [ ] **Helm chart for Kubernetes deployment**  
  Production-grade deployment. One pod per component. ConfigMap for node configuration. Secret management for keypairs. Horizontal scaling for circuit controller and gossip agent.

- [ ] **Node operator setup guide**  
  Step-by-step: carrier agreement, APN configuration, SIM pool provisioning, software deployment, gossip network join, verification that the node is announcing correctly.

---

### camo-cli

- [ ] **Node status command**  
  Display current node state: gossip peer count, active circuits, SIM pool availability, WireGuard peer count, uptime.

- [ ] **Peer list command**  
  List known gossip peers with last-seen timestamp, jurisdiction, pool size, exit status.

- [ ] **Circuit inspect command**  
  Display active circuits: hop count, jurisdictions in circuit, rotation timer, session count.

- [ ] **SIM pool command**  
  List SIM pool inventory with health status, allocation state, carrier, jurisdiction.

---

## Research — Requires Investigation

### Onion encryption layer

**Status:** Identified as a known limitation in Section 14.4  
**Problem:** Current spec uses hop-by-hop WireGuard encryption. Each middle node decrypts and re-encrypts. A compromised middle node can read the inner IP packet. Tor's layered onion encryption eliminates this — the circuit initiator applies all layers before transmission; each hop peels one layer without seeing inner content.  
**Question:** Can onion encryption be implemented at the cellular routing layer without requiring device-side software beyond APN configuration? If device-side software is required, what is the minimal footprint?  
**Relevant prior work:** Tor circuit construction, Sphinx packet format (Danezis & Goldberg, 2009)

---

### Formal anonymity analysis

**Status:** Open — no formal analysis exists for this protocol  
**Problem:** The spec makes informal anonymity claims based on structural reasoning. A formal analysis would establish what anonymity properties the protocol actually provides, under what adversary models, and at what network sizes.  
**Question:** What are the k-anonymity properties of the SIM pool model? What is the minimum network size for meaningful anonymity under a passive global adversary? Under a partial adversary controlling k% of nodes?  
**Relevant prior work:** Tor anonymity analysis (Overlier & Syverson, 2006), k-anonymity in location privacy literature

---

### Sybil resistance

**Status:** Documented limitation in Section 9.3  
**Problem:** A fully permissionless gossip network has no inherent Sybil resistance. A well-resourced adversary registering many nodes gains disproportionate circuit presence. Jurisdictional diversity enforcement limits concentrated Sybil attacks but does not eliminate the risk.  
**Question:** What Sybil resistance mechanisms can be applied without introducing a central authority or registration requirement? Can proof-of-work on NodeRecord announcements provide sufficient resistance without creating a compute barrier to legitimate operators?  
**Relevant prior work:** Sybil attacks in P2P networks (Douceur, 2002), Tor's approach to relay diversity

---

### Latency characterization

**Status:** Acknowledged in Section 14.1, not quantified  
**Problem:** Multi-hop cellular routing introduces latency. The acceptable ceiling depends on use case. The spec currently states voice is likely unacceptable but provides no measurements.  
**Question:** What is the actual latency profile of a 3–5 hop CAMO circuit using M2M SIMs across typical carrier configurations? What use cases are feasible at that latency?  
**Approach:** Build minimal testnet with 3 nodes across 2 jurisdictions, benchmark round-trip latency under varying load, characterize against Tor latency benchmarks for comparison.

---

### Bootstrap node governance

**Status:** Referenced in Section 9.2, governance not specified  
**Problem:** The gossip protocol requires at least one known peer to enter the network. Bootstrap nodes are the entry point. Who operates them, how they are maintained, and how their addresses are published are governance questions with technical implications.  
**Question:** What is the minimum viable bootstrap node set? How should bootstrap node addresses be distributed — hardcoded, published in the repository, discoverable via DNS? What prevents a bootstrap node operator from censoring new entrants?

---

### CAMO as Tor entry layer

**Status:** Unexplored  
**Problem:** CAMO and Tor address different layers — CAMO addresses the radio layer, Tor addresses the IP layer. The two protocols are complementary and could potentially be combined: CAMO circuit to exit node, then Tor circuit from exit node to destination.  
**Question:** What are the threat model implications of combining CAMO and Tor? Does the combination provide additive protection or do the trust boundaries interact in ways that reduce overall protection? What is the latency cost?

---

### IPv6

**Status:** Deferred in Section 14.5  
**Problem:** The current spec focuses on IPv4. IPv6 requires consideration for WireGuard tunnel address allocation and GTP-U encapsulation of IPv6 packets.  
**Question:** What changes to the data structures and routing logic are required for full IPv6 support? Are there IPv6-specific considerations for the M2M carrier layer?

---

### 5G Standalone core integration

**Status:** Spec references 5G but focuses on 4G EPC architecture  
**Problem:** 5G Standalone (SA) core uses a different architecture — Service Based Architecture (SBA), AMF replacing MMC, SMF replacing SGW-C/PGW-C. The APN concept maps to DNN (Data Network Name) in 5G SA.  
**Question:** What changes to the APN core interface contract (Appendix B) are required for 5G SA compliance? Are the CAMO routing properties preserved in 5G SA, or does the SBA architecture introduce new considerations?

---

### Node operator sustainability

**Status:** Out of scope for the protocol, relevant to network viability  
**Problem:** The protocol defines no payment mechanism. Node operators incur real costs: M2M SIM fees, compute, bandwidth, carrier agreements. Long-term network health requires operators to find the operation worthwhile.  
**Question:** What is the realistic cost of operating a minimum viable node? What motivates node operation in a fee-free network — privacy advocacy, institutional funding, organizational use? What is the minimum operator community size for network resilience?  
**Note:** This is a social and economic research question, not a protocol design question. The protocol will not be modified to include payment as a result of this research. The question is about network viability, not protocol design.

---

## Completed

*Nothing yet — this is v0.1*

---

*Pablo Chacon — June 2026*  
*CAMO is released under CC BY 4.0*
