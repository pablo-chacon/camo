# GSM Routing Chains

**Author:** Pablo Chacon  
**Published:** June 2026  
**License:** CC BY 4.0

**Related reading:**  
→ [Protocol & SS7](./gsm_protocol_ss7.md)  
→ [Threats & Legislation](./gsm_threats_legislation.md)

---

## Overview

GSM call forwarding is a standard network feature. A subscriber instructs their carrier to redirect incoming calls to a different number. The carrier writes a forwarding rule into the subscriber's Home Location Register (HLR) entry and executes it transparently on every incoming call.

When multiple SIM cards are chained through forwarding rules, a routing property emerges from the protocol's normal operation: no single carrier in the chain has visibility of both the origin and the final destination simultaneously. Each node knows only its immediate predecessor and successor.

This document describes that property, its implications, and how it can be structured into a deliberate routing architecture using standard GSM infrastructure and commercially available M2M SIM cards.

---

## 1. The Forwarding Chain

### 1.1 Visibility per node

A three-hop forwarding chain across separate carriers:

```
Origin → Tele2(SIM_A) → Telenor(SIM_B) → Tre(SIM_C) → Destination
```

What each carrier observes:

| Carrier | Sees | Does not see |
|---|---|---|
| Tele2 | Origin IMSI, forward to Telenor number | Telenor→Tre hop, destination |
| Telenor | Incoming from Tele2 number, forward to Tre number | Origin, destination |
| Tre | Incoming from Telenor number, deliver to destination | Origin, Tele2→Telenor hop |

Reconstructing the full chain requires querying each carrier independently with separate legal instruments, in sequence, within the data retention window of each carrier. Each query depends on the result of the previous one. The process is serial, not parallel.

### 1.2 What this property is

This is not a vulnerability. It is not an exploit. It is the normal, intended behavior of GSM call forwarding operating across carrier boundaries. The protocol was designed for reliability and interoperability — the partial visibility at each node is an inherent consequence of a distributed network architecture where no central routing authority exists.

For protocol background see [Protocol & SS7](./gsm_protocol_ss7.md).

---

## 2. M2M Cards as Jurisdictional Breaks

### 2.1 What M2M SIM cards are

Machine-to-Machine (M2M) SIM cards are standard GSM subscriber cards designed for IoT and industrial deployments. They operate on private APNs — traffic routes through a private gateway rather than the public carrier infrastructure. M2M cards are available from carriers across multiple jurisdictions and are not linked to personal identity in the way consumer SIMs are. Blank programmable variants exist for development purposes with no operator locks.

For practical purposes in a forwarding chain: an M2M card from a non-domestic carrier, operating on a private APN, represents a point at which:

- Traffic leaves the originating carrier's infrastructure entirely
- The observable origin resets to the M2M card's number
- The carrier governing that node operates under a different legal jurisdiction
- No automatic data-sharing obligation exists between that carrier and domestic operators

### 2.2 The jurisdictional break in practice

```
Tele2(SIM_A) → M2M_NL(private APN) → Telenor(SIM_B)
```

Tele2 sees a forward to an M2M number and loses the thread. Telenor sees an incoming call originating from an M2M number — the Swedish origin is not visible. A legal request from Swedish authorities to Tele2 yields a forward destination. Pursuing that destination requires initiating a separate legal process in the Netherlands under Dutch law. The two processes are independent and cannot be compelled simultaneously through a single domestic instrument.

### 2.3 Dual M2M architecture

Placing M2M nodes at both entry and exit:

```
M2M_entry(NL) → domestic hops → M2M_exit(DE) → destination
```

Entry and exit are now in separate foreign jurisdictions. The domestic hops in the middle are accessible to domestic authorities but contain no origin or destination information — only the two adjacent M2M numbers, which are each governed by separate foreign legal processes.

```
To reconstruct this chain:
  Dutch legal process  →  identifies entry node forward
  Swedish queries      →  middle hops (accessible, uninformative)
  German legal process →  identifies exit node forward
  
All three must complete before retention windows expire.
Processes cannot be compelled through a single legal instrument.
```

---

## 3. Rotating Chain Architecture

### 3.1 The static chain problem

A fixed forwarding chain — same SIMs, same sequence, used persistently — can be retroactively reconstructed from carrier metadata as long as records are retained. The chain is a persistent structure.

### 3.2 Rotation

A pool of forwarding SIMs with regularly updated forwarding rules produces a different chain topology each session:

```
Pool: [Tele2_A, Tele2_B, Telenor_A, Telenor_B, Tre_A, M2M_NL, M2M_DE, M2M_UK, M2M_FR]

Session 1:  M2M_NL → Telenor_A → Tre_A   → M2M_DE
Session 2:  M2M_DE → Tele2_A   → Telenor_B → M2M_UK
Session 3:  M2M_FR → Tre_A     → Tele2_B  → M2M_NL
```

No two sessions share the same chain. No persistent structure exists for retroactive analysis. Reconstruction requires identifying which chain was active during which session, querying every carrier that appeared across all sessions, and correlating results across multiple jurisdictions — all before retention windows expire.

The number of required legal requests scales with pool size and rotation frequency. Retention window expiry is a hard deadline that cannot be extended retroactively.

### 3.3 Chain construction constraints

Effective chain construction should enforce:

- Minimum 3 hops, maximum 6
- Minimum 2 M2M hops per chain, from different jurisdictions
- No consecutive chains sharing the same entry or exit node
- Domestic carrier hops between M2M nodes
- No identical chain repeated across sessions

### 3.4 Entry and exit from the M2M pool

The strongest form of the architecture uses M2M cards at both entry and exit, with the full pool rotating:

```
Random M2M entry →
  Random domestic middle hops (2–3) →
    Random M2M exit →
      destination
```

No fixed identifier exists at any point in the chain across sessions.

---

## 4. Full Architecture

### 4.1 Layered model

The forwarding chain provides routing topology obfuscation. It does not encrypt content — each carrier in the chain can read the traffic passing through it. Content encryption and transport encryption are handled at separate layers:

```
Layer 1 — Content:    End-to-end encrypted VOIP (Signal, Matrix)
Layer 2 — Transport:  Tor or equivalent
Layer 3 — Routing:    GSM rotating forwarding chain
```

Each layer is orthogonal — solving a different part of the problem independently. The GSM chain does not need to provide encryption. It only needs to obscure routing topology. Content and transport are handled above it.

### 4.2 Physical separation

M2M cards physically register on towers. Geographic co-location of M2M cards with the operator creates a potential correlation point. The mitigated architecture separates the operator's device from the forwarding infrastructure entirely:

```
Operator device (no SIM, data-only)
  → Tor
    → Remote controller (separate physical location)
      → Manages M2M pool and forwarding rules
        → Rotating GSM chain
          → Encrypted VOIP endpoint
            → Destination
```

The operator's device is absent from the GSM network. No IMSI. No tower registration. No presence on the forwarding chain. The chain operates as a remotely managed resource.

### 4.3 Infrastructure requirements

The full architecture requires only:

- Standard consumer SIM cards from domestic carriers, registered normally
- M2M SIM cards from carriers in multiple jurisdictions — commercially available
- A mechanism to update forwarding rules — carrier USSD commands or web portals
- An encrypted VOIP client
- Tor

No custom hardware. No software exploits. No protocol modifications. Every component is standard, commercially available telecommunications infrastructure operating within its normal designed parameters.

---

## 5. Defensive Stack

Organized by layer, descending impact:

**1. Exit the cellular signaling plane**  
Data-only SIM eliminates the SS7 attack surface. Voice and SMS travel over the signaling plane — removing that plane removes the primary interception surface. Encrypted VOIP over data replaces voice. See [Threats & Legislation](./gsm_threats_legislation.md) for what the signaling plane exposes.

**2. Harden the device OS**  
GrapheneOS: STK disabled by default, granular per-app network permissions, hardened memory allocator, verified boot. Meaningful reduction in attack surface accessible from userspace and from the baseband interface.

**3. Encrypt transport**  
All data over Tor. No single point of trust. Origin obfuscated from network-level observers.

**4. Forwarding chain for cellular identity**  
Where a cellular identity is required: rotating M2M chain as described above. No persistent identifier. Jurisdictional fragmentation. Rotation window shorter than legal process timelines.

**5. Device-level reduction**  
STK disabled via ADB. Telemetry packages disabled. OTA update mechanism disabled. Minimum installed application surface.

---

## 6. Limitations

### 6.1 The baseband

The baseband processor runs its own isolated OS with direct hardware access, below Android. Userspace software — including a hardened OS — cannot fully audit baseband behavior. A baseband-level compromise is largely invisible to the device OS. This is an architectural constraint with no current consumer-available software solution.

### 6.2 Timing correlation

A passive observer with simultaneous visibility of both chain entry and exit can correlate sessions through timing analysis. Call enters the chain at time T, exits at time T+n — the temporal signature can link origin and destination even across multiple hops. M2M private APNs introduce variable latency that partially disrupts timing signatures but does not eliminate the attack under a sufficiently capable observer.

### 6.3 Trust boundary displacement

Every architectural layer moves the trust boundary rather than eliminating it. The M2M carrier becomes the trusted party. The VOIP provider becomes a trust point. Tor exit nodes are trust points for unencrypted destinations. Jurisdiction selection, payment method, and operational security at each trust boundary are outside the scope of the technical architecture.

### 6.4 Nation-state ceiling

These architectures raise the cost and complexity of surveillance for most realistic threat actors. They do not provide absolute protection against a well-resourced state actor with SS7 access, carrier cooperation, deployed IMSI catchers, and the legal instruments to compel simultaneous multi-jurisdictional disclosure. The honest framing: appropriate for commercial surveillance, corporate intelligence gathering, opportunistic interception, and constrained-resource state actors. Not a complete solution against a dedicated high-resource adversary targeting a specific individual.

---

*Pablo Chacon — June 2026*  
*CC BY 4.0 — See [Protocol & SS7](./gsm_protocol_ss7.md) and [Threats & Legislation](./gsm_threats_legislation.md)*
