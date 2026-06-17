# CAMO — Cellular Anonymization and Mobile Onion-routing
## Protocol Specification v0.1

**Author:** Pablo Chacon  
**Published:** June 2026  
**Status:** Draft Specification  
**License:** CC BY 4.0

**Related research:**  
→ [GSM Routing Chains](./gsm_routing_chains.md)  
→ [Protocol & SS7](./gsm_protocol_ss7.md)  
→ [Threats & Legislation](./gsm_threats_legislation.md)

---

## Abstract

This document describes a discovery: the fundamental operating properties of GSM call forwarding, M2M private APN infrastructure, and eSIM remote provisioning — when composed — produce onion routing at the cellular radio layer as an emergent architectural property. This is not a novel technology. Every component described in this specification is a decades-old standard or a current commercial product. What is described here is the recognition that these components, assembled in a specific way, close the one gap in mobile privacy infrastructure that no existing solution addresses.

Existing anonymization networks, including Tor, operate above the cellular radio layer. They protect data content and routing topology within the IP stack but leave the radio identity layer — IMSI, IMEI, carrier metadata, physical location — entirely exposed. This gap has been documented. It has not been closed.

CAMO closes it using infrastructure that already exists everywhere.

The protocol defines a permissionless network of distribution servers — each running a containerized mobile core and a pool of M2M eSIM cards — interconnected via WireGuard tunnels and discovered through a signed gossip protocol. Participating devices route their traffic through dynamically constructed, rotating chains of distribution servers. Each server has visibility only of its adjacent hops. No single node can reconstruct the full circuit.

Every component is a GSM standard, an IoT industry standard, or an open protocol in wide commercial deployment: GTP-U (3GPP TS 29.281), private APN routing (standard M2M carrier product), eSIM remote provisioning (GSMA SGP.02), WireGuard (Linux kernel 5.6+). CAMO composes these components. It does not modify them.

This has a structural consequence that is worth stating explicitly: CAMO cannot be banned or disabled without disabling the underlying infrastructure it is built from. Private APNs cannot be prohibited without collapsing enterprise IoT. eSIM remote provisioning cannot be prohibited without dismantling the M2M carrier industry. GTP-U tunneling cannot be prohibited without breaking how mobile data works. The protocol is enforcement-resistant not by design but by composition — it is indistinguishable from normal commercial use of standard telecommunications infrastructure.

This specification is complete and intended for implementation.

---

## Table of Contents

1. [Introduction](#1-introduction)
2. [Problem Statement](#2-problem-statement)
3. [Design Principles](#3-design-principles)
4. [Protocol Overview](#4-protocol-overview)
5. [Distribution Server Specification](#5-distribution-server-specification)
6. [APN Core Interface](#6-apn-core-interface)
7. [SIM Pool Management](#7-sim-pool-management)
8. [Inter-Node Encryption — WireGuard](#8-inter-node-encryption--wireguard)
9. [Node Discovery — Signed Gossip Protocol](#9-node-discovery--signed-gossip-protocol)
10. [Circuit Construction](#10-circuit-construction)
11. [Packet Routing](#11-packet-routing)
12. [Threat Model](#12-threat-model)
13. [Implementation Requirements](#13-implementation-requirements)
14. [Limitations and Open Questions](#14-limitations-and-open-questions)
15. [References](#15-references)
16. [Appendix A — Data Structures](#appendix-a--data-structures)
17. [Appendix B — APN Core Interface Contract](#appendix-b--apn-core-interface-contract)

---

## 1. Introduction

This specification documents a discovery made while investigating the attack surface of USSD codes and GSM signaling. The investigation followed a chain of reasoning from how carriers route calls, to how forwarding chains produce partial visibility at each node, to how M2M private APNs create jurisdictional breaks, to how these properties — when composed deliberately — produce onion routing at the cellular radio layer using only standard telecommunications infrastructure.

The discovery is this: GSM call forwarding, private APN routing, M2M eSIM pools, and WireGuard inter-node encryption, assembled into a distributed network architecture, produce a system with the same fundamental anonymization properties as Tor — but operating at the radio layer where Tor does not reach, and built entirely from infrastructure that the global carrier industry already operates and depends on.

The relationship between a mobile device and the cellular network is asymmetric by design. The subscriber registers with the carrier. The carrier assigns identity. The carrier routes traffic. The carrier retains metadata. Carrier infrastructure collects this data as a structural property of how cellular networks operate — call records, tower location, session timing — regardless of what software runs on the device.

Existing privacy tools operate above this layer. A device running Tor still has a SIM. That SIM registers on towers. The carrier knows the device's IMSI, its physical location to cell-level granularity, and the timing and volume of all traffic — even when content is protected. A device using Signal still exposes its cellular identity to its carrier. A device running GrapheneOS still participates in the carrier's metadata collection.

This is not a failure of those tools. It is a consequence of where they operate. None of them were designed to address the radio layer because the radio layer was considered fixed infrastructure — something you use, not something you architect around.

The premise of this protocol is that the radio layer is not fixed. The carrier provides radio access. Everything above the radio layer is configurable. A private APN moves the gateway from carrier infrastructure to operator-defined infrastructure. A distributed network of private APN gateways, running containerized mobile cores and rotating eSIM pools, interconnected via WireGuard and discovered through a signed gossip protocol, produces at the radio layer the same property that Tor produces at the IP layer: no single node has full visibility of both origin and destination.

The components are not new. Private APNs are sold to enterprises today for IoT fleet management. M2M eSIM remote provisioning is a GSMA standard deployed at scale. WireGuard is in the Linux kernel. GTP-U is how mobile data has worked for decades. CAMO is the recognition that these components, composed in a specific architecture, produce mobile onion routing as an emergent property — and the formal specification of that architecture so anyone can implement it.

---

## 2. Problem Statement

### 2.1 The mobile privacy gap

Tor's threat model explicitly excludes the radio layer. The Tor design document states that Tor protects against traffic analysis at the network level — it does not address the physical layer identity of the device initiating traffic. For desktop users this is an acceptable constraint. For mobile users it is a fundamental exposure.

A mobile device in normal operation continuously exposes:

- **IMSI** — the subscriber's permanent identity, mapped to their carrier contract
- **IMEI** — the device's hardware identity, persistent across SIM changes
- **Cell tower registration** — physical location to serving cell granularity, logged by the carrier as metadata
- **Traffic timing and volume** — visible to the carrier even when content is encrypted
- **SS7 signaling** — call setup, SMS routing, location updates, all traversing a network with no node authentication

These exposures exist regardless of what software runs on the device. They are properties of cellular network participation, not of any particular application or OS configuration.

### 2.2 Why existing solutions are insufficient

| Solution | What it protects | Radio layer |
|---|---|---|
| Tor | IP routing, data content | Unaddressed |
| VPN | Data content, destination | Unaddressed |
| Signal | Message content | Unaddressed |
| GrapheneOS | OS attack surface | Partially reduced |
| Data-only SIM | Removes voice/SMS surface | Partial |
| None | Radio identity, carrier metadata | — |

No existing solution addresses the combination of radio identity exposure and carrier metadata collection at the network infrastructure level.

### 2.3 The gap this protocol fills

This protocol moves the point of control from the carrier to a permissionless distributed network. The carrier retains radio access provision. All routing decisions, all encryption, all chain topology are determined by the protocol — not by carrier infrastructure, carrier policy, or carrier legal obligation.

A device participating in this protocol exposes to its carrier:

- IMSI and IMEI (unavoidable at the radio layer)
- Physical location (unavoidable at the radio layer)
- An encrypted GTP tunnel to a distribution server endpoint

The carrier cannot observe the destination of traffic. The carrier cannot observe the chain topology. The carrier cannot observe what services are being accessed. The encrypted tunnel is the carrier's visibility ceiling.

---

## 3. Design Principles

**P1 — Permissionless**  
Any operator with appropriate infrastructure can run a distribution server. There is no admission authority, no vetting process, no central registration. The network is open by design.

**P2 — No single point of trust**  
No node in the network can reconstruct a complete circuit. No directory authority can be compelled to deanonymize users. No central coordinator exists.

**P3 — Implementation agnostic**  
This specification defines interfaces and behaviors. It does not mandate specific APN core software. Compliant implementations may use any software that satisfies the interface contract defined in Appendix B.

**P4 — Carrier agnostic**  
This protocol operates above the radio access layer. It requires only that the carrier support private APN routing to a customer-defined endpoint — a standard feature of M2M carrier offerings. No carrier-specific features, APIs, or cooperation beyond standard private APN provisioning are required.

**P5 — Free to use**  
This protocol defines no payment mechanism, no token, no fee structure. Access to the network carries no cost at the protocol level. Node operators determine their own operational model. The protocol is silent on economics.

**P6 — Jurisdictional diversity by construction**  
Circuit construction algorithms enforce geographic and jurisdictional distribution across hops. No circuit may traverse multiple consecutive hops within the same legal jurisdiction.

**P7 — Honest threat model**  
This specification documents what the protocol protects against and what it does not. No privacy claim is made beyond what the architecture supports.

---

## 4. Protocol Overview

### 4.1 Components

**Device**  
Any device with a SIM card configured with a participating distribution server's APN. Requires no special software beyond APN configuration. The privacy properties are provided by the network, not by software on the device.

**Distribution Server (DS)**  
A node in the network. Runs a containerized mobile core (APN gateway), maintains a pool of M2M SIM cards, participates in the gossip protocol, and routes traffic through the chain. Anyone may operate a distribution server.

**M2M SIM Pool**  
A collection of M2M SIM cards held by a distribution server. Each SIM provides a cellular identity that can serve as a chain hop. SIM pools are registered as server capabilities in the gossip network.

**Circuit**  
A ordered sequence of distribution servers through which a session's traffic is routed. Constructed by the entry distribution server at session initiation. Rotated periodically.

**Gossip Network**  
The peer-to-peer network through which distribution servers discover each other, announce capabilities, and verify node liveness.

### 4.2 Traffic flow

```
Device
  │ (APN config: entry DS endpoint)
  │ GTP tunnel, carrier-encrypted radio layer
  ▼
Entry Distribution Server (DS_1)
  │ Decapsulates GTP
  │ Constructs circuit: [DS_2, DS_3, DS_4]
  │ Re-encapsulates in WireGuard tunnel
  ▼
DS_2 (middle hop)
  │ Receives on WireGuard interface
  │ Knows: came from DS_1, goes to DS_3
  │ Does not know: device identity, DS_4, destination
  │ Forwards on WireGuard tunnel
  ▼
DS_3 (middle hop)
  │ Knows: came from DS_2, goes to DS_4
  │ Does not know: device, DS_1, DS_2, destination
  │ Forwards on WireGuard tunnel
  ▼
DS_4 (exit node)
  │ Knows: came from DS_3
  │ Does not know: device, DS_1, DS_2, DS_3
  │ Routes to destination via its own internet egress
  ▼
Destination
```

### 4.3 What each party observes

| Party | Observes | Does not observe |
|---|---|---|
| Device's carrier | IMSI, IMEI, location, GTP tunnel to DS_1 | Chain topology, destination, content |
| DS_1 (entry) | Device's carrier tunnel, constructs circuit | Destination (via exit), DS_3, DS_4 |
| DS_2 (middle) | DS_1 address, DS_3 address | Device, DS_4, destination, content |
| DS_3 (middle) | DS_2 address, DS_4 address | Device, DS_1, DS_2, destination, content |
| DS_4 (exit) | DS_3 address, destination | Device, DS_1, DS_2, DS_3 |
| Destination | DS_4's egress IP | Device, chain, carrier |

---

## 5. Distribution Server Specification

### 5.1 Required components

A compliant distribution server MUST run:

- A containerized mobile core implementing the APN Core Interface (Appendix B)
- A WireGuard interface for inter-node tunneling
- A gossip protocol agent for node discovery and announcement
- A circuit controller managing chain construction and rotation
- A SIM pool manager tracking available M2M SIM identities

### 5.2 Minimum hardware requirements

```
CPU:     2 cores (4 recommended for multi-session)
RAM:     4GB minimum (8GB recommended)
Storage: 20GB
Network: Public IP address with stable routing
         Low-latency connectivity (< 50ms to nearest exchange recommended)
OS:      Linux kernel 5.6+ (WireGuard native kernel support)
Runtime: Docker or Kubernetes
```

### 5.3 Network requirements

The distribution server MUST have:

- A publicly reachable IP address
- UDP port 51820 open (WireGuard, configurable)
- Inbound GTP-U port 2152 open for device tunnel termination (entry nodes)
- Outbound connectivity to other distribution servers
- An APN agreement with at least one M2M carrier routing GTP tunnels to the server's IP

### 5.4 Identity

Each distribution server has a persistent Ed25519 keypair. The public key is the server's canonical identity across the gossip network. The keypair is generated at first launch and persists across restarts.

```
Key generation:
  keypair = ed25519.generate()
  server_id = base64url(keypair.public_key)
```

The server_id is used in gossip announcements, circuit construction, and WireGuard peer configuration.

### 5.5 Node announcement

A distribution server announces itself to the gossip network with a signed NodeRecord:

```json
{
  "server_id": "<base64url ed25519 public key>",
  "version": "0.1",
  "endpoints": {
    "wireguard": "<ip>:<port>",
    "gtp": "<ip>:2152"
  },
  "capabilities": {
    "sim_pool_size": 12,
    "regions": ["SE", "NL", "DE"],
    "jurisdictions": ["SE", "NL", "DE"],
    "uptime_30d": 0.997,
    "exit_node": true
  },
  "timestamp": "<RFC3339>",
  "signature": "<ed25519 signature over canonical JSON>"
}
```

Fields:

- `sim_pool_size` — number of active M2M SIMs in the pool
- `regions` — ISO 3166-1 alpha-2 country codes where SIMs are registered
- `jurisdictions` — legal jurisdictions governing the server's data
- `exit_node` — whether this server will route traffic to the open internet
- `signature` — Ed25519 signature over the canonical JSON representation, using the server's keypair

### 5.6 Containerization

The reference deployment model is Docker Compose or Kubernetes. Each component runs in an isolated container:

```yaml
services:
  apn-core:       # APN core implementation (operator's choice)
  wireguard:      # WireGuard interface management
  gossip-agent:   # Peer discovery and announcement
  circuit-ctrl:   # Chain construction and rotation
  sim-manager:    # SIM pool tracking and allocation
  proxy:          # Internal routing between components
```

Components communicate via a local Unix socket or loopback interface. No component exposes an external management API without explicit configuration.

---

## 6. APN Core Interface

### 6.1 Principle

This specification does not mandate a specific APN core implementation. Operators may use Open5GS, free5GC, or any other implementation that satisfies the interface contract. The interface is defined in terms of observable behaviors and API calls — not internal implementation.

### 6.2 Required behaviors

A compliant APN core MUST:

**B1 — GTP tunnel termination**  
Accept inbound GTP-U (UDP/2152) tunnels from carrier infrastructure. Decapsulate GTP packets and deliver the inner IP traffic to the routing layer.

**B2 — Per-IMSI routing**  
Support routing rules configurable per IMSI. Traffic from each IMSI must be independently routable to a specified next-hop WireGuard interface.

**B3 — Session management**  
Manage PDN (Packet Data Network) sessions for connected devices. Report session state (active, idle, terminated) to the circuit controller.

**B4 — No logging by default**  
The APN core MUST NOT log subscriber traffic content by default. Session metadata logging (IMSI, session start/end, bytes transferred) is configurable and defaults to off.

**B5 — Management API**  
Expose a local management API (Unix socket, localhost only) implementing the contract defined in Appendix B.

### 6.3 Carrier requirements

The distribution server operator must have an agreement with at least one M2M carrier that:

- Supports private APN configuration
- Routes GTP-U tunnels to the operator's server IP
- Does not inspect or log tunnel content (verify contractually)
- Operates in the jurisdiction the operator intends to declare

Standard M2M carrier offerings from providers including Hologram, 1NCE, Eseye, and Transatel support this configuration. This is a commercial product category, not a custom arrangement.

---

## 7. SIM Pool Management

### 7.1 Purpose

A distribution server's SIM pool provides the cellular identities used when the server acts as a middle or exit hop in a circuit. When traffic is routed through a server, it appears to originate from a SIM in that server's pool — not from the device's actual SIM. The pool is the mechanism by which the device's cellular identity is decoupled from the routing chain.

The pool operates on two independent rotation axes: per-session SIM allocation (which SIM handles a given session) and active subset rotation (which SIMs are active at all). Together these ensure no stable SIM identifier persists across rotation cycles at either layer.

### 7.2 Pool composition

A SIM pool consists of M2M SIM cards from one or more carriers. Each SIM in the pool:

- Is registered with a carrier in a specific jurisdiction
- Has a private APN routing GTP traffic to the distribution server
- Receives traffic forwarded from the previous hop in the circuit
- Re-transmits that traffic over the cellular network to the next hop
- Has an active or dormant state managed programmatically by the SIM manager

### 7.3 Pool size recommendations

```
Minimum viable:    8 SIMs  (4 active, 4 dormant — provides basic subset rotation)
Recommended:       24 SIMs across 2-3 carriers (8 active at any time)
Production:        64+ SIMs across 3+ carriers, 2+ jurisdictions (16+ active)
```

Larger pools reduce correlation between any specific SIM and any specific session. The ratio of active to total pool size determines the anonymity set at the SIM layer — a larger dormant reserve increases the unpredictability of which SIMs will be active at any given time.

### 7.4 Active subset

At any point in time only a subset of the full pool is active. Active SIMs are registered on the carrier network and available for session allocation. Dormant SIMs are provisioned and carrier-registered but not currently in use — they are not deregistered or cancelled, simply inactive at the protocol layer.

```
Active subset size:
  Minimum:      4 SIMs
  Recommended:  30-40% of total pool size
  Maximum:      60% of total pool size

Remaining SIMs: dormant, available for next rotation cycle
```

Keeping a majority of SIMs dormant at any time ensures that an observer tracking active SIMs cannot identify the full pool — the dormant SIMs are invisible to traffic analysis.

### 7.5 Active subset rotation

The active subset rotates on an independent timer, separate from circuit chain rotation (Section 10.3). The two rotation timers MUST be independent and SHOULD NOT be synchronized.

```
Default rotation intervals:
  Chain rotation:       600 seconds  (Section 10.3)
  SIM subset rotation:  configurable, default 480 seconds

Recommended: SIM rotation interval is NOT a simple multiple of chain
rotation interval — prevents predictable alignment between the two cycles.
```

Rotation algorithm:

```
On SIM subset rotation trigger:

1. Select replacement_count SIMs from dormant pool
   replacement_count = random integer between 1 and (active_subset_size / 2)
   Partial replacement prevents complete active set turnover at once

2. For each selected dormant SIM:
   a. Promote to active state
   b. Register with APN core for traffic routing

3. Select replacement_count SIMs from current active pool for retirement
   Enforce: no SIM currently handling an active session is retired
   Enforce: no SIM used in the immediately preceding rotation is retired

4. For each selected active SIM:
   a. Wait for in-flight sessions to complete (max 30 second drain window)
   b. Demote to dormant state
   c. Remove from APN core routing

5. Update available pool count reported to gossip agent
```

Partial replacement rather than full subset swap ensures continuity — active sessions are not disrupted and the transition between subsets is smooth.

### 7.6 Per-session SIM allocation

Within the active subset, SIM allocation to individual sessions uses randomized assignment:

- Does not reuse the same SIM for consecutive sessions from the same circuit entry
- Distributes load across the active subset to prevent usage pattern correlation
- Tracks SIM liveness within the active subset and removes unresponsive SIMs from allocation
- Reports available active SIM count to the gossip agent for node announcements

### 7.7 The combined rotation property

Chain rotation (Section 10.3) and SIM subset rotation (Section 7.5) operate independently on non-synchronized timers. The combined effect:

```
Observer attempting correlation across sessions:

  Session 1:  Chain [DS_A -> DS_B -> DS_C]  |  Active SIMs at DS_B: {3, 7, 11}
  Session 2:  Chain [DS_A -> DS_D -> DS_C]  |  Active SIMs at DS_D: {2, 9, 14}
  Session 3:  Chain [DS_A -> DS_B -> DS_E]  |  Active SIMs at DS_B: {5, 1, 8}
                                                        ^ subset has rotated
```

No stable identifier exists at either layer. The chain changes which servers are involved. The active subset changes which SIMs those servers present. An observer tracking any specific SIM sees it go dormant on an unpredictable schedule. An observer tracking chain topology finds a different SIM layer each time even on familiar hops.

### 7.8 Dormant SIM behavior

Dormant SIMs MUST:

- Remain provisioned and carrier-registered (not cancelled or deactivated at the carrier level)
- Maintain their APN configuration pointing to the distribution server
- Be available for promotion to active state at the next rotation
- Not appear in active session allocation

Dormant SIMs MUST NOT:

- Be deregistered from the carrier network
- Have their APN configuration removed
- Generate outbound traffic

The carrier sees dormant SIMs as idle registered subscribers — indistinguishable from a device that is powered off or out of coverage. This is the intended behavior.

### 7.9 Liveness monitoring

The SIM manager monitors liveness of both active and dormant SIMs:

- Active SIMs: continuous monitoring via session activity and periodic APN core health checks
- Dormant SIMs: periodic liveness checks at intervals not exceeding 3600 seconds
- SIMs failing liveness checks are flagged as unavailable and excluded from rotation
- Unavailable SIMs are retried at exponential backoff intervals before being flagged for operator attention

### 7.10 eSIM profiles as the recommended pool implementation

Physical M2M SIM cards satisfy the requirements of this specification. eSIM (embedded SIM) profiles are the recommended implementation where hardware supports them.

An eSIM chip stores multiple carrier profiles. Each profile is a complete subscriber identity — IMSI, authentication keys, APN configuration — downloadable and manageable over the air via the GSMA Remote SIM Provisioning (RSP) standard (SGP.02 for M2M, SGP.22 for consumer).

The mapping to this specification's pool model is direct:

| Section 7 concept | Physical SIM | eSIM implementation |
|---|---|---|
| Active SIM | SIM inserted, carrier-registered | Profile enabled via RSP API |
| Dormant SIM | SIM idle, carrier-registered | Profile disabled, reactivatable |
| Pool expansion | Physical SIM procurement and insertion | Profile download via RSP API call |
| Carrier switch | Physical SIM swap | Profile replacement via RSP |

**Operational advantages over physical SIMs:**

- Pool expansion is an API call — no physical access to the distribution server required
- Profile enable/disable maps directly to active/dormant rotation (Section 7.5) — disabled profiles are invisible to the carrier, stronger than physical SIM dormancy where the SIM remains passively registered
- Multiple profiles on a single eSIM chip reduce hardware requirements for large pools
- Carrier and jurisdiction diversity can be managed remotely — download profiles from carriers in different jurisdictions without touching the server
- Profile switching is programmable — the SIM manager can enable/disable profiles via RSP API as part of the rotation algorithm

**Compatibility:**

eSIM RSP is standardized. Any M2M eSIM carrier implementing GSMA SGP.02 exposes compatible profile management. The CAMO SIM manager treats eSIM profile state as equivalent to physical SIM active/dormant state — no protocol-level distinction exists between the two. Implementations supporting eSIM SHOULD use the RSP API for profile state management in place of software-only active/dormant tracking.

---

## 8. Inter-Node Encryption — WireGuard

### 8.1 Rationale

WireGuard was selected as the inter-node encryption protocol for the following properties:

- **Minimal codebase** — approximately 4,000 lines of code, audited, with a small attack surface
- **Modern cryptography** — Curve25519 for key exchange, ChaCha20-Poly1305 for symmetric encryption, BLAKE2s for hashing
- **Kernel-native** — integrated into the Linux kernel since 5.6, eliminating userspace overhead
- **Stateless design** — no session negotiation overhead, connection establishment is fast
- **Proven deployment** — widely used in production VPN infrastructure

### 8.2 Tunnel configuration

Each distribution server maintains a WireGuard interface. Peers are added dynamically as circuits are constructed.

```
Interface configuration (per server):
  PrivateKey = <server WireGuard private key, distinct from Ed25519 identity key>
  ListenPort = 51820
  Address    = <server's internal WireGuard IP from allocated range>

Peer configuration (added per circuit hop):
  PublicKey  = <peer server's WireGuard public key>
  AllowedIPs = <peer's WireGuard IP/32>
  Endpoint   = <peer's public IP>:51820
```

### 8.3 Key exchange

WireGuard public keys for inter-node tunnels are distributed via the gossip protocol as part of NodeRecord announcements. A server's WireGuard public key is included in its signed NodeRecord — verifiable against the server's Ed25519 identity key.

```json
{
  "server_id": "...",
  "wireguard_pubkey": "<base64 WireGuard public key>",
  ...
  "signature": "<ed25519 signature covering all fields including wireguard_pubkey>"
}
```

A server receiving a NodeRecord can verify the WireGuard public key is authentically associated with that server_id before adding it as a peer.

### 8.4 Per-hop encryption

Traffic between adjacent hops in a circuit is encrypted by the WireGuard tunnel between those two servers. A middle node can decrypt the WireGuard layer to receive the packet — and re-encrypt it for the next hop. This provides hop-by-hop encryption.

This differs from Tor's onion encryption model, where layers are pre-applied by the circuit initiator and each hop peels one layer without seeing inner content. The implications of this distinction are addressed in Section 12.

### 8.5 Forward secrecy

WireGuard performs a Diffie-Hellman handshake every 180 seconds and rekeying every 60 seconds. Compromising a session key does not compromise past sessions. This property holds for all inter-node tunnels in the circuit.

---

## 9. Node Discovery — Signed Gossip Protocol

### 9.1 Design rationale

A signed gossip protocol was selected over a DHT or centralized directory for the following reasons:

- **No directory authority** — no central server can be compelled to produce the node list or taken offline to disable the network
- **Resilience** — the network continues to function if any subset of nodes is unreachable
- **Verifiability** — all announcements are cryptographically signed; forged or tampered records are detectable
- **Simplicity** — gossip protocols are well-understood and straightforward to implement correctly

### 9.2 Protocol operation

**Peer initialization**  
A new distribution server requires at least one known peer to enter the gossip network — a bootstrap node. Bootstrap nodes are well-known community-operated servers whose addresses are published in this specification's companion repository. The use of bootstrap nodes does not create a central authority — they are entry points, not authorities. Once connected, a server discovers the full network through gossip without further reliance on bootstrap nodes.

**Announcement propagation**  
When a server comes online or updates its NodeRecord, it sends the signed record to all known peers. Each peer that receives a valid record it has not seen before forwards it to all of its known peers. A record is considered "seen" if its `(server_id, timestamp)` tuple matches a previously received record.

```
On receipt of NodeRecord R from peer P:
  if R.server_id == self.server_id: discard
  if not verify_signature(R): discard
  if (R.server_id, R.timestamp) in seen_records: discard
  store(R)
  update peer list if R is newer than stored record for R.server_id
  forward R to all known peers except P
```

**Liveness verification**  
Servers send a signed heartbeat to all known peers every 60 seconds:

```json
{
  "server_id": "...",
  "timestamp": "<RFC3339>",
  "signature": "..."
}
```

A server that fails to send a heartbeat for 300 seconds is marked inactive and excluded from circuit construction. Its NodeRecord is retained but flagged inactive. If heartbeats resume, the server is restored to active status.

**Record expiry**  
NodeRecords older than 24 hours without a corresponding update are expired and removed. Servers MUST re-announce at intervals not exceeding 12 hours.

### 9.3 Sybil resistance

A fully permissionless gossip network has no inherent Sybil resistance — an adversary can register many nodes to gain disproportionate circuit presence. This is a known and documented limitation.

Mitigations within this specification:

- Circuit construction enforces jurisdictional diversity — a Sybil attack concentrated in one jurisdiction does not fully compromise a well-constructed circuit
- Circuit construction enforces operator diversity where operator identity can be inferred from network prefix or announced jurisdiction
- The exit node flag requires explicit declaration — an operator claiming exit capability is asserting internet egress, which requires real infrastructure

This specification acknowledges that Sybil resistance in a fully permissionless network is an open research problem. Implementations SHOULD implement additional heuristics appropriate to their deployment context. A companion document on Sybil mitigation strategies is planned.

### 9.4 Privacy of the gossip network

Participation in the gossip network reveals a server's IP address and capabilities to all other participants. This is intentional — distribution servers are infrastructure operators, not users. Their IP addresses are necessarily public to function as network nodes.

User devices do not participate in the gossip network. A device's IP address is never announced to the gossip network.

---

## 10. Circuit Construction

### 10.1 Responsibility

Circuit construction is the responsibility of the entry distribution server — the server that accepts the device's GTP tunnel. The device is not involved in circuit construction beyond its APN configuration pointing at an entry server.

This differs from Tor, where the client constructs the circuit. The tradeoff: the entry server knows the device and constructs the circuit, which requires trusting the entry server with circuit topology knowledge. The entry server knows the circuit it constructed but not the device's ultimate destination (which is visible only to the exit node). This is documented in the threat model (Section 12).

### 10.2 Construction algorithm

```
Input:
  active_nodes      = gossip network's current active node list
  device_jurisdiction = jurisdiction of device's carrier
  entry_server      = self

Parameters:
  MIN_HOPS = 3
  MAX_HOPS = 5
  ROTATION_INTERVAL = 600 seconds

Algorithm:

1. Remove entry_server from candidate pool
2. Remove nodes in same jurisdiction as entry_server
3. Select middle hops:
   a. Randomly select (MIN_HOPS - 1) to (MAX_HOPS - 1) nodes
   b. Enforce: no two consecutive hops in same jurisdiction
   c. Enforce: no two consecutive hops with same announced operator
   d. Prefer: nodes with higher uptime_30d
   e. Prefer: nodes with larger sim_pool_size
4. Select exit node:
   a. Filter candidates to exit_node == true
   b. Remove any node already selected as middle hop
   c. Select randomly from remaining candidates
   d. Enforce: exit node jurisdiction differs from last middle hop jurisdiction
5. Construct ordered circuit: [entry, middle_1, ..., middle_n, exit]
6. Program WireGuard peers and routing rules for each hop
7. Schedule rotation at ROTATION_INTERVAL
```

### 10.3 Circuit rotation

At each rotation interval:

1. Construct a new circuit using the algorithm above
2. Program new WireGuard peers and routing rules
3. Migrate active sessions to the new circuit (with brief overlap period)
4. Tear down old WireGuard peers
5. Schedule next rotation

Rotation is continuous and transparent to the device. The device's GTP tunnel to the entry server is not affected by circuit rotation.

### 10.4 Circuit isolation

Each device session MUST use an isolated circuit. Two devices connecting to the same entry server MUST NOT share a circuit. Circuit isolation prevents correlation of traffic from different devices through shared hop observation.

### 10.5 Minimum viable network size

Circuit construction with MIN_HOPS=3 and jurisdictional diversity enforcement requires at minimum:

- 3 active nodes in at least 3 distinct jurisdictions
- At least 1 node with exit_node=true

Below this threshold the entry server MUST NOT construct circuits and MUST inform connecting devices that the network is below minimum viable size.

Recommended minimum for meaningful anonymity: 50 active nodes across 10+ jurisdictions, with 10+ exit nodes.

---

## 11. Packet Routing

### 11.1 Entry node processing

On receipt of a GTP-U packet from carrier infrastructure:

```
1. Decapsulate GTP-U header
2. Extract inner IP packet
3. Identify device IMSI from GTP tunnel endpoint
4. Look up active circuit for this IMSI
5. Re-encapsulate inner IP packet in WireGuard tunnel to circuit[1] (first middle hop)
6. Transmit on WireGuard interface
```

The entry node adds a circuit header identifying the session — an opaque session token that allows middle nodes to route the packet to the correct next hop without exposing device identity:

```
Circuit header (prepended to inner IP packet before WireGuard encapsulation):
  session_token: 16 bytes random, generated at circuit construction
  next_hop_index: 1 byte (position in circuit, 0-indexed)
```

The session token is shared with each hop in the circuit at construction time via a direct encrypted message to each node. Middle nodes use the session token to look up the next hop without needing to know the circuit's full topology.

### 11.2 Middle node processing

On receipt of a WireGuard packet:

```
1. WireGuard decryption (automatic, kernel layer)
2. Extract circuit header
3. Look up session_token in local session table
4. Retrieve next hop address from session table entry
5. Re-encapsulate in WireGuard tunnel to next hop
6. Transmit
```

A middle node's session table entry contains only:

```
session_token → next_hop_wireguard_address
```

The node does not store the previous hop address beyond what WireGuard provides for the incoming tunnel. The node does not store the device identity. The node does not store the circuit topology beyond its own next-hop.

### 11.3 Exit node processing

On receipt of a WireGuard packet at the exit node:

```
1. WireGuard decryption
2. Extract circuit header
3. Verify session_token is registered as an exit session
4. Strip circuit header
5. Route inner IP packet to destination via exit node's internet egress
6. Return traffic: encapsulate response in WireGuard tunnel back to previous hop
```

Return traffic traverses the circuit in reverse using the same session token. Each node's session table entry stores both next-hop (forward) and previous-hop (return) WireGuard addresses.

### 11.4 GTP-U encapsulation detail

GTP-U (GPRS Tunneling Protocol — User Plane) is the carrier protocol used to tunnel device traffic from the radio network to the APN gateway. Defined in 3GPP TS 29.281.

A distribution server acting as entry node terminates GTP-U tunnels from the carrier. The GTP-U header is stripped at this point. Inner IP traffic enters the protocol's routing layer. GTP-U is not used between distribution servers — WireGuard tunnels carry inter-node traffic.

### 11.5 Device-to-entry encryption gap

GTP-U provides tunneling, not confidentiality. The protocol does not encrypt the inner IP payload. This is a property of GTP-U itself, not a CAMO design choice, and it is a known characteristic of carrier transport infrastructure generally — any party with visibility into the GTP-U transport segment between the device's radio access and the entry distribution server can in principle inspect the inner payload.

This stands in contrast to every other segment of a CAMO circuit. Inter-node traffic between distribution servers is protected by WireGuard (Section 8). The device-to-entry leg, carried over carrier GTP-U, is not protected by any encryption this specification mandates.

**Recommendation.** Devices SHOULD apply their own encryption on top of the GTP-U leg specifically to close this gap. This can take the form of:

- A VPN client on the device, terminating beyond the CAMO exit node or at a separate trusted endpoint
- TLS at the application layer for all traffic (standard practice regardless of CAMO)
- An encrypted overlay (e.g. WireGuard) from the device directly to the entry distribution server, if the entry server offers such an endpoint in addition to its GTP-U interface

This recommendation is independent of which encryption the application layer already provides. Even where application traffic is itself encrypted (TLS web traffic, Signal, etc.), an unencrypted GTP-U leg still exposes traffic metadata — destination IPs, timing, and volume — to anything observing that transport segment. Device-side encryption closes this independent of the application's own protections.

**Where the VPN terminates matters.** A VPN client active on the device before traffic enters the GTP-U tunnel protects the device-to-entry leg specifically. A VPN terminating after the CAMO exit node protects the exit-to-destination leg instead — and additionally removes destination visibility from the exit node operator, since the exit node then sees only encrypted VPN traffic rather than the final destination. These are not equivalent and operators with a specific threat model should be precise about which leg they intend to protect.

This is documented as a specification gap rather than a defect: closing it is a SHOULD-level recommendation for device configuration, not a MUST-level requirement on the protocol, because the GTP-U leg is carrier transport that CAMO does not control. A future revision of this specification may define an optional device-to-entry encrypted overlay as part of the protocol itself rather than leaving it to operator discretion.

---

## 12. Threat Model

### 12.1 What this protocol protects

**Against a passive observer watching the device's carrier:**  
The carrier observes IMSI, IMEI, physical location, and an encrypted GTP tunnel to the entry distribution server. The carrier cannot observe traffic destination, content, or chain topology. This is the primary protection this protocol provides that existing solutions do not.

**Against a passive observer at any single distribution server:**  
A compromised middle node observes its upstream and downstream WireGuard peer addresses and the volume and timing of traffic passing through it. It cannot observe the device, the destination, or the rest of the circuit. This holds as long as the node is not both the entry and exit of the same circuit — which circuit construction prevents by design.

**Against a passive observer at the destination:**  
The destination observes the exit node's egress IP address. It cannot observe the device, the carrier, the chain topology, or any prior hop.

**Against retroactive metadata analysis:**  
Distribution servers do not log by default. Carriers log metadata for the GTP tunnel to the entry server. Reconstructing the full circuit requires legal process against every server in the circuit simultaneously — across multiple jurisdictions — before any server's logs (if any exist) expire.

### 12.2 What this protocol does not protect

**Radio layer identity:**  
The device's IMSI and IMEI are visible to its carrier. Physical location to cell-level granularity is logged by the carrier. This protocol does not change these properties — they are inherent to cellular network participation and cannot be addressed at the protocol layer. Mitigation: data-only SIM from a privacy-focused carrier; GrapheneOS for device hardening.

**Entry server knowledge:**  
The entry distribution server knows the device's carrier tunnel (and therefore can infer the carrier and approximate location) and constructs the full circuit. It does not know the destination. It does not know what traffic the device is sending. But it is a more privileged position than middle or exit nodes. Users with high threat models should consider this when selecting entry servers.

**Timing correlation:**  
A global passive observer with simultaneous visibility of the device's carrier traffic and the exit node's egress traffic can perform timing correlation — matching the timing of packets entering the entry server's GTP endpoint with packets exiting the exit node. This is the same fundamental attack that threatens Tor under global observer conditions. Variable latency introduced by the cellular network and SIM pool routing partially mitigates this — it is not eliminated.

**Sybil attacks:**  
A well-resourced adversary operating many distribution servers could gain disproportionate circuit presence. See Section 9.3. Jurisdictional diversity enforcement in circuit construction limits the effectiveness of a concentrated Sybil attack but does not eliminate the risk.

**Exit node traffic:**  
Traffic from the exit node to the destination is in plaintext unless the application provides end-to-end encryption. This is identical to Tor's exit node property. Applications SHOULD use TLS or end-to-end encryption independently of this protocol. The protocol does not inspect or modify application-layer content.

**Device-to-entry transport:**  
GTP-U, the carrier protocol carrying traffic from the device to the entry distribution server, provides tunneling but not payload confidentiality. This segment is not covered by the WireGuard encryption that protects all inter-node traffic in the rest of the circuit (Section 8). See Section 11.5 for the gap in full and recommended device-side mitigations.

**Baseband:**  
The device's baseband processor operates independently of the device OS with direct hardware access. A baseband-level compromise is outside this protocol's threat model and outside any current software solution's scope.

### 12.3 Honest summary

This protocol provides meaningful protection against:
- Carrier-level traffic analysis
- Destination visibility to the carrier
- Single-server compromise revealing circuit topology or device identity
- Retroactive chain reconstruction under normal (non-nation-state) threat models

This protocol does not provide protection against:
- Global passive adversaries with simultaneous visibility of entry and exit
- Well-resourced Sybil attacks without additional mitigations
- Baseband-level device compromise
- Application-layer traffic analysis (correlation via content fingerprinting)
- Legal compulsion of the entry distribution server operator

---

## 13. Implementation Requirements

### 13.1 Distribution server operator requirements

An operator running a compliant distribution server MUST:

- Generate and persist an Ed25519 keypair at first launch
- Run a compliant APN core satisfying the interface in Appendix B
- Maintain at least one active M2M SIM carrier agreement with private APN routing
- Participate in the gossip protocol and maintain accurate NodeRecord announcements
- Run WireGuard with the configuration specified in Section 8
- Not log subscriber traffic content
- Declare accurate jurisdiction information in NodeRecord announcements

An operator SHOULD:

- Maintain a SIM pool of at least 12 SIMs
- Deploy in a jurisdiction with strong data protection laws
- Use a carrier with contractual no-logging commitments
- Implement monitoring for SIM pool liveness
- Publish their operational security practices

### 13.2 Client configuration

A device participates in the network by configuring the APN to point at a chosen entry distribution server. No application installation is required. No OS modification is required.

APN configuration:

```
APN name:     [entry server's announced APN name]
APN type:     default
Protocol:     IPv4 or IPv4/IPv6
MCC/MNC:      [device's carrier values, unchanged]
Username:     [optional, per entry server configuration]
Password:     [optional, per entry server configuration]
```

The device obtains an IP address from the entry server's DHCP pool. All traffic from the device routes through the circuit from that point.

### 13.3 Implementation languages

This specification places no constraints on implementation language. The reference gossip agent and circuit controller are expected to be implemented in Go or Rust given their suitability for network infrastructure software. APN core implementation is operator's choice per Section 6.

### 13.4 Versioning

This document is version 0.1. The version field in NodeRecord announcements allows nodes running different specification versions to coexist and negotiate compatibility. Nodes MUST reject circuit construction with nodes running incompatible versions.

Version negotiation:
- Major version mismatch: incompatible, refuse circuit construction
- Minor version mismatch: compatible with documented caveats, log warning

---

## 14. Limitations and Open Questions

### 14.1 Latency

Multi-hop cellular routing introduces latency. Each hop adds the round-trip time across a WireGuard tunnel plus processing time at each distribution server. In testing comparable to Tor's latency characteristics (100–300ms additional), this is acceptable for data applications and unacceptable for real-time voice. Voice applications should use Tor or a direct VPN rather than this protocol.

Latency optimization is an open area for future work.

### 14.2 Bootstrapping problem

The network requires sufficient node density before it provides meaningful anonymity. A network of 3 nodes provides weak protection regardless of the protocol's design. Growth from zero requires coordinated effort. The minimum viable threshold (Section 10.5) must be reached before the network can be recommended for production use.

### 14.3 Carrier relationship at scale

Private APN routing requires a carrier agreement. At individual scale, M2M carrier agreements are commercially available products. At network scale — if the protocol achieves significant adoption — carriers may face regulatory pressure to refuse private APN agreements to distribution server operators. This is a non-technical risk with no purely technical mitigation. Jurisdictional diversity of node operators distributes this risk.

### 14.4 Onion encryption

This specification uses hop-by-hop WireGuard encryption rather than Tor's layered onion encryption. In Tor's model, the circuit initiator applies all encryption layers before transmission — each hop peels one layer without seeing inner content, and no intermediate node can read the full packet. In this specification's model, each hop decrypts the WireGuard layer and re-encrypts for the next hop. A compromised middle node could in principle read and modify the inner IP packet.

The practical risk is partially mitigated by:
- Application-layer encryption (TLS, end-to-end encrypted applications)
- The fact that a middle node cannot determine whether it is seeing real traffic or is being tested
- The entry node constructing the circuit without middle nodes having advance knowledge

Full onion encryption at the cellular routing layer is technically achievable but requires pre-computing encryption layers at circuit construction time and distributing keys accordingly. This is identified as a significant enhancement for a future version of this specification.

### 14.5 IPv6

This specification focuses on IPv4. IPv6 support requires additional consideration for address allocation within the WireGuard tunnel range and GTP-U encapsulation of IPv6 packets. IPv6 support is planned for a future revision.

### 14.6 Open research questions

- Optimal circuit length versus latency tradeoffs for cellular routing
- Sybil mitigation without introducing centralization
- Onion encryption layer design for cellular routing substrate
- Economic sustainability models for node operators in a fee-free network
- Formal anonymity analysis (k-anonymity properties of the SIM pool model)
- Interaction with 5G standalone (SA) core architecture

---

## 15. References

### Protocol specifications
- 3GPP TS 29.281 — GTP-U specification
- 3GPP TS 29.002 — MAP specification  
- 3GPP TS 23.401 — GPRS enhancements for E-UTRAN access (EPC architecture)
- 3GPP TS 23.501 — System architecture for 5G (5GC)
- GSMA SGP.02 — Remote Provisioning Architecture for Embedded UICC (M2M eSIM)
- GSMA SGP.22 — RSP Technical Specification (consumer eSIM)
- WireGuard: Next Generation Kernel Network Tunnel — Donenfeld, J.A. (2017). NDSS.

### Anonymization research
- Dingledine, R., Mathewson, N., Syverson, P. (2004). *Tor: The Second-Generation Onion Router.* USENIX Security.
- Syverson, P., Reed, M., Goldschlag, D. (1997). *Onion Routing for Anonymous and Private Internet Connections.* Communications of the ACM.
- Chaum, D. (1981). *Untraceable Electronic Mail, Return Addresses, and Digital Pseudonyms.* Communications of the ACM.

### SS7 and mobile security
- Engel, T. (2014). *SS7: Locate. Track. Manipulate.* Chaos Communication Congress.
- Adaptive Mobile Security. (2019). *Simjacker — Next Generation Spying Over Mobile.*
- Nohl, K. (2014). *Mobile Self-Defense.* Chaos Communication Congress.

### Reference implementations
- Open5GS — open5gs.org
- free5GC — free5gc.org
- srsRAN — srsran.com
- WireGuard — wireguard.com

### Related work in this series
- Chacon, P. (2026). *GSM Routing Chains.* [gsm_routing_chains.md]
- Chacon, P. (2026). *Protocol & SS7.* [gsm_protocol_ss7.md]
- Chacon, P. (2026). *Threats & Legislation.* [gsm_threats_legislation.md]

---

## Appendix A — Data Structures

### A.1 NodeRecord

```json
{
  "version": "0.1",
  "server_id": "string (base64url ed25519 public key)",
  "wireguard_pubkey": "string (base64 WireGuard public key)",
  "endpoints": {
    "wireguard": "string (ip:port)",
    "gtp": "string (ip:port)"
  },
  "capabilities": {
    "sim_pool_size": "integer",
    "regions": ["string (ISO 3166-1 alpha-2)"],
    "jurisdictions": ["string (ISO 3166-1 alpha-2)"],
    "uptime_30d": "float (0.0–1.0)",
    "exit_node": "boolean",
    "max_circuits": "integer"
  },
  "timestamp": "string (RFC3339)",
  "signature": "string (base64 ed25519 signature)"
}
```

### A.2 Heartbeat

```json
{
  "server_id": "string",
  "timestamp": "string (RFC3339)",
  "active_circuits": "integer",
  "sim_pool_available": "integer",
  "signature": "string (base64 ed25519 signature)"
}
```

### A.3 CircuitRecord (entry server internal)

```json
{
  "circuit_id": "string (uuid)",
  "device_imsi": "string",
  "session_token": "string (base64, 16 bytes random)",
  "hops": [
    {
      "server_id": "string",
      "wireguard_address": "string",
      "hop_index": "integer"
    }
  ],
  "created_at": "string (RFC3339)",
  "rotate_at": "string (RFC3339)"
}
```

### A.4 SessionTableEntry (middle/exit node internal)

```json
{
  "session_token": "string (base64)",
  "forward_peer": "string (WireGuard peer address)",
  "return_peer": "string (WireGuard peer address)",
  "created_at": "string (RFC3339)",
  "is_exit": "boolean"
}
```

---

## Appendix B — APN Core Interface Contract

The following API MUST be exposed by any compliant APN core implementation on a Unix socket at `/run/apncore.sock`. All messages are newline-delimited JSON.

### B.1 Session events (APN core → circuit controller)

```json
// New device session established
{
  "event": "session_created",
  "imsi": "string",
  "gtp_teid": "integer",
  "device_ip": "string",
  "timestamp": "string (RFC3339)"
}

// Device session terminated
{
  "event": "session_terminated", 
  "imsi": "string",
  "timestamp": "string (RFC3339)"
}
```

### B.2 Routing commands (circuit controller → APN core)

```json
// Set per-IMSI next hop
{
  "command": "set_route",
  "imsi": "string",
  "next_hop_wireguard_ip": "string",
  "session_token": "string"
}

// Remove per-IMSI routing
{
  "command": "clear_route",
  "imsi": "string"
}

// Query active sessions
{
  "command": "list_sessions"
}
```

### B.3 Responses

```json
// Acknowledgement
{
  "status": "ok",
  "command": "string"
}

// Error
{
  "status": "error",
  "command": "string",
  "message": "string"
}

// Session list response
{
  "status": "ok",
  "command": "list_sessions",
  "sessions": [
    {
      "imsi": "string",
      "device_ip": "string",
      "active_since": "string (RFC3339)"
    }
  ]
}
```

---

*Pablo Chacon — June 2026*  
*This specification is released under Creative Commons Attribution 4.0 International (CC BY 4.0)*  
*Implementations, derivative works, and distributions are permitted with attribution.*
