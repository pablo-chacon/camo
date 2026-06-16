# CAMO
### Cellular Anonymization and Mobile Onion-routing

A discovery: standard GSM and IoT industry infrastructure, composed in a specific architecture, produces onion routing at the cellular radio layer.

**Author:** Pablo Chacon  
**Status:** Specification complete — reference implementation v0.1  
**License:** CC BY 4.0

---

## The discovery

This protocol was not designed toward a known goal. It was found by following a chain of reasoning through GSM signaling architecture — from how carriers route calls, to how forwarding chains produce partial visibility at each node, to how M2M private APNs create jurisdictional breaks, to how these properties compose into a distributed anonymization network.

The result: GSM call forwarding, private APN routing, M2M eSIM pools, and WireGuard inter-node encryption, assembled into a permissionless distributed network, produce onion routing at the cellular radio layer. No single node has visibility of both origin and destination. The carrier is reduced to a radio access substrate.

None of the components are new:

| Component | Standard / Product | Age |
|---|---|---|
| GSM call forwarding | ITU-T / 3GPP | 1987 |
| Private APN routing | Standard M2M carrier product | ~2000s |
| GTP-U tunneling | 3GPP TS 29.281 | GSM era |
| M2M eSIM provisioning | GSMA SGP.02 | 2016 |
| WireGuard | Linux kernel 5.6+ | 2017 |
| Signed gossip protocol | Distributed systems standard | — |

CAMO is the recognition that these components, composed in a specific way, produce mobile onion routing as an emergent property — and the formal specification of that architecture so anyone can implement it.

---

## Why this matters

Existing anonymization networks, including Tor, operate above the cellular radio layer. They protect data content and IP routing. They do not address what the carrier sees: IMSI, IMEI, physical location, and traffic metadata — regardless of what software runs on the device.

CAMO closes that gap. The carrier provides radio access and nothing more. Routing, encryption, chain topology, and node discovery are defined by the protocol, independent of carrier infrastructure.

The two are complementary. Tor protects the IP layer. CAMO protects the radio layer. Running both covers the full stack.

---

## The enforcement-resistance property

CAMO is built exclusively from infrastructure the global carrier and IoT industry already operates and depends on commercially:

- Private APNs are standard enterprise products sold for IoT fleet management
- M2M eSIM remote provisioning is a GSMA standard deployed at scale globally
- GTP-U tunneling is how mobile data has functioned for decades
- WireGuard is in the Linux kernel

Disabling CAMO requires disabling these components. Disabling these components collapses enterprise IoT, M2M carrier infrastructure, and the mobile data layer itself.

This enforcement-resistance is not a feature that was engineered. It is a structural consequence of what CAMO is made from.

---

## How it works

```
Device (any SIM + APN config pointing at entry server)
  → Carrier (radio access only — sees encrypted tunnel, nothing beyond)
    → Entry distribution server
      → WireGuard tunnel → Middle hop(s)
        → WireGuard tunnel → Exit node
          → Destination
```

Each distribution server sees only its adjacent hops. Chains rotate every 10 minutes. eSIM pools at each server rotate independently on a non-synchronized timer. No stable identifier exists at any layer across rotation cycles.

Anyone can run a distribution server. No registration. No permission. No fee.

---

## Repository structure

```
camo/
├── README.md
├── LICENSE                           — CC BY 4.0
├── LEGAL.md                          — legal considerations
│
├── spec/
│   ├── mobile_onion_routing_spec.md  — protocol specification v0.1
│   ├── gsm_routing_chains.md         — foundational architecture analysis
│   ├── gsm_protocol_ss7.md           — GSM and SS7 background
│   └── gsm_threats_legislation.md    — threat landscape and legislation
│
├── camo-gossip/                      — node discovery (signed gossip protocol)
├── camo-circuit/                     — circuit construction and rotation
├── camo-apncore/                     — APN core interface adapter
├── camo-simpool/                     — eSIM pool management
├── camo-wireguard/                   — WireGuard peer lifecycle
│
└── deploy/
    ├── docker-compose.yml            — single-node reference deployment
    ├── Dockerfile.go                 — shared build image
    └── config.example                — annotated configuration examples
```

---

## Specification

| Document | Description |
|---|---|
| [Protocol Specification](https://github.com/pablo-chacon/camo/blob/main/docs/mobile_onion_routing_spec.md) | Complete protocol — architecture, data structures, interface contracts, threat model |
| [GSM Routing Chains](https://github.com/pablo-chacon/camo/blob/main/docs/gsm/gsm_routing_chains.md) | Foundational analysis — forwarding chains, M2M breaks, defensive stack |
| [Protocol & SS7](https://github.com/pablo-chacon/camo/blob/main/docs/gsm/gsm_protocol_ss7.md) | Technical background — USSD/MMI, SS7 architecture, SIM Toolkit |
| [Threats & Legislation](https://github.com/pablo-chacon/camo/blob/main/docs/gsm/gsm_threats_legislation.md) | Threat landscape — SS7 attacks, IMSI catchers, legislative context |

---

## Running a node

Full requirements in Section 13 of the specification. In brief:

- Linux server, kernel 5.6+, public IP
- Docker or Kubernetes
- M2M carrier agreement with private APN routing to your server IP
- Pool of M2M eSIM cards (minimum 8, recommended 24+)

```
docker compose up -d
```

No registration. No fee. No central authority.

---

## Design principles

- **Permissionless** — anyone can run a node
- **No central authority** — no single point whose unavailability affects the network
- **Implementation agnostic** — the spec defines interfaces, not software
- **Free** — no payment mechanism, no token, no fee at the protocol level
- **Built from standards** — every component is an existing industry standard or open protocol
- **Honest** — the threat model documents limitations as clearly as protections

---

## Related work

- [Tor Project](https://torproject.org) — foundational onion routing; CAMO complements Tor at the radio layer
- [Open5GS](https://open5gs.org) — open source 4G/5G core, one compliant APN core option
- [free5GC](https://free5gc.org) — Go-based 5G core, Kubernetes-native
- [WireGuard](https://wireguard.com) — inter-node encryption

---

## Citation

```
Chacon, P. (2026). CAMO: Cellular Anonymization and Mobile Onion-routing.
Protocol Specification v0.1. https://github.com/[repo]
License: CC BY 4.0
```

---

*Pablo Chacon — June 2026*