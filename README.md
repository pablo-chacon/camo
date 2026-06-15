# CAMO
### Cellular Anonymization and Mobile Onion-routing

A protocol specification and reference implementation for decentralized anonymization at the cellular radio layer.

**Author:** Pablo Chacon  
**Status:** Specification complete — implementation in progress  
**Version:** 0.1  
**License:** CC BY 4.0

---

## What CAMO is

Every existing anonymization network operates above the cellular radio layer. They protect data content and IP routing. None of them address the layer where a device's identity, physical location, and traffic metadata are exposed to the carrier network — regardless of what software runs on the device.

CAMO closes that gap. It defines a permissionless network of distribution servers — each running a containerized mobile core and a pool of M2M SIM cards — that collectively provide onion routing at the radio layer. The carrier network provides radio access. Routing, encryption, and chain topology are defined by the CAMO protocol, independent of carrier infrastructure.

No single node has visibility of both origin and destination. The protocol is free. Anyone can run a node. No central authority exists.

---

## How it works

```
Device (SIM + APN config)
  → Carrier (radio access only)
    → Entry distribution server
      → WireGuard tunnel → Middle hop(s)
        → WireGuard tunnel → Exit node
          → Destination
```

Each distribution server sees only its adjacent hops. The carrier sees an encrypted tunnel to the entry server and nothing beyond it. The destination sees the exit node's IP. No party has the full picture.

Chains are constructed dynamically, rotated every 10 minutes, and enforced to span multiple jurisdictions. Anyone can operate a distribution server with commercially available M2M SIM cards and a private APN agreement.

---

## Repository structure

```
camo/
├── README.md                         — this file
├── LICENSE                           — CC BY 4.0
├── LEGAL.md                          — legal considerations
├── THREADS.md                        — roadmap and open threads
│
├── spec/
│   ├── mobile_onion_routing_spec.md  — protocol specification v0.1
│   ├── gsm_routing_chains.md         — foundational architecture analysis
│   ├── gsm_protocol_ss7.md           — protocol and SS7 background
│   └── gsm_threats_legislation.md    — threat landscape and legislation
│
├── camo-gossip/                      — gossip agent (node discovery)
├── camo-circuit/                     — circuit controller
├── camo-apncore/                     — APN core interface adapter
├── camo-simpool/                     — SIM pool manager
├── camo-wireguard/                   — WireGuard peer management
│
├── deploy/
│   ├── docker-compose.yml            — single-node reference deployment
│   └── helm/                         — Kubernetes deployment charts
│
└── tools/
    └── camo-cli/                     — node operator tooling
```

---

## Specification

| Document | Description |
|---|---|
| [Protocol Specification](./spec/mobile_onion_routing_spec.md) | Complete protocol — architecture, data structures, interface contracts, threat model |
| [GSM Routing Chains](./spec/gsm_routing_chains.md) | Foundational analysis — forwarding chains, M2M breaks, defensive stack |
| [Protocol & SS7](./spec/gsm_protocol_ss7.md) | Technical background — USSD/MMI, SS7 architecture, SIM Toolkit |
| [Threats & Legislation](./spec/gsm_threats_legislation.md) | Threat landscape — SS7 attacks, IMSI catchers, Swedish legislative context |

Start with the [Protocol Specification](./spec/mobile_onion_routing_spec.md).

---

## Implementation components

### camo-gossip
Node discovery and announcement. Implements the signed gossip protocol defined in Section 9 of the specification. Handles NodeRecord signing and verification, heartbeat management, peer tracking, and liveness monitoring.

### camo-circuit
Circuit construction and rotation. Implements the chain building algorithm defined in Section 10. Manages WireGuard peer programming for active circuits, rotation scheduling, and session isolation.

### camo-apncore
APN core interface adapter. Implements the management API contract defined in Appendix B of the specification. Provides a compliant adapter layer over a chosen APN core implementation (Open5GS, free5GC, or other). The adapter is the only component that changes when switching APN core software.

### camo-simpool
SIM pool management. Tracks available M2M SIMs, monitors liveness, manages allocation to active sessions, and reports pool capacity to the gossip agent for NodeRecord announcements.

### camo-wireguard
WireGuard peer lifecycle management. Adds and removes peers dynamically as circuits are constructed and torn down. Manages the WireGuard interface configuration independently of the circuit controller.

---

## Running a node

Full requirements are in Section 13 of the specification. Summary:

**Infrastructure:**
- Linux server, kernel 5.6+, public IP
- Docker or Kubernetes
- At least one M2M carrier agreement with private APN routing to your server IP

**SIM pool:**
- Minimum 4 M2M SIMs (12+ recommended)
- At least one carrier, one jurisdiction

**Software:**
- WireGuard (kernel-native, no additional install on 5.6+)
- Docker Compose (single node) or Helm (Kubernetes)
- CAMO components from this repository

No registration. No fee. No permission required.

---

## Design principles

- **Permissionless** — anyone can run a node
- **No central authority** — no directory server, no single point whose unavailability affects the network
- **Implementation agnostic** — the spec defines interfaces; implementations are interchangeable
- **Free** — no payment mechanism, no token, no fee at the protocol level
- **Layered** — carrier provides radio access; CAMO provides everything above it
- **Honest** — the threat model documents limitations as clearly as protections

---

## Contributing

Contributions to the specification, implementation, and research are welcome.

See [THREADS.md](./THREADS.md) for current work items and open research questions.

- **Specification issues** — describe the problem, the affected section, and a proposed resolution
- **Implementation** — pick up any item from THREADS.md or open a new thread
- **Research** — formal analysis of anonymity properties is particularly needed
- **Threat model gaps** — document the attack, conditions, and potential mitigations

All contributions are subject to CC BY 4.0.

---

## Related work

- [Tor Project](https://torproject.org) — foundational onion routing; CAMO complements Tor at the radio layer
- [Open5GS](https://open5gs.org) — open source 4G/5G core, reference APN core option
- [free5GC](https://free5gc.org) — Go-based 5G core, Kubernetes-native
- [WireGuard](https://wireguard.com) — inter-node encryption protocol

---

## Citation

```
Chacon, P. (2026). CAMO: Cellular Anonymization and Mobile Onion-routing.
Protocol Specification v0.1. https://github.com/[repo]
License: CC BY 4.0
```

---

*Pablo Chacon — June 2026*
