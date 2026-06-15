# Protocol & SS7

**Author:** Pablo Chacon  
**Published:** June 2026  
**License:** CC BY 4.0

**Part of series:**  
→ [GSM Routing Chains](./gsm_routing_chains.md) *(main)*  
→ [Threats & Legislation](./gsm_threats_legislation.md)

---

## Overview

This document describes the protocol stack underlying GSM mobile communications — the layers that enable call routing, SMS delivery, location services, and remote SIM management. Understanding these layers is prerequisite to understanding both the attack surface documented in [Threats & Legislation](./gsm_threats_legislation.md) and the routing architecture described in [GSM Routing Chains](./gsm_routing_chains.md).

---

## 1. USSD, MMI, and the Code Taxonomy

Three categories of `*` and `#` dialer codes are commonly conflated. They are distinct in origin, execution path, and what they expose.

### 1.1 OEM / Device Codes

Defined by the device manufacturer and hardcoded into firmware. Execute locally on the device. No network connection required. No SIM required.

Examples:
- `*2767*3855#` — Samsung factory reset (executes immediately, no confirmation)
- `*#0*#` — Samsung hardware diagnostic menu
- `*#1234#` — Samsung firmware version

These are Samsung-defined, not carrier-defined. They function identically regardless of which SIM or carrier is present.

### 1.2 GSM Standard Codes

Defined by 3GPP and ITU-T specifications. Intended to be portable across devices and carriers, though carriers can override or block them.

Examples:
- `*#06#` — Display IMEI (universal)
- `*21*[number]#` — Activate call forwarding to number
- `##002#` — Cancel all forwarding rules
- `*#43#` — Check call waiting status
- `*#31#` — Check caller ID suppression

These sit at the interface between device and network — some execute locally, some initiate a network transaction.

### 1.3 Operator USSD

Defined by each carrier independently. Execute as real-time sessions over the SS7 signaling channel. The device sends the code, the carrier's USSD gateway processes it and returns a response displayed on screen. Entirely carrier-specific — the same code means different things on different networks.

Examples:
- `*123#` — balance check (varies by carrier)
- `*101#` — data usage (carrier-specific)

Require a live SIM from the relevant operator. Response goes to the device screen — not to any external system.

| Type | Defined by | Needs SIM | Hits network |
|---|---|---|---|
| OEM/device | Manufacturer | No | No |
| GSM standard | 3GPP / ITU-T | Sometimes | Sometimes |
| Operator USSD | Each carrier | Yes | Yes |

---

## 2. SS7 — Signaling System 7

### 2.1 What it is

Signaling System 7 is the protocol stack that underpins the global telephone network. Standardized by ITU-T in the Q.700 series, it handles the signaling traffic that sets up calls, routes SMS, manages roaming, and supports location services — separately from the actual voice/data payload.

SS7 operates on a dedicated signaling plane. When you make a call, the voice travels on one channel. The signaling that sets up that call — negotiating routing, checking subscriber status, reserving capacity — travels on the SS7 plane independently.

### 2.2 Architecture

Key components:

**SSP — Service Switching Point**  
The telephone exchange. Originates and terminates SS7 messages. Interfaces between the SS7 network and the PSTN.

**STP — Signal Transfer Point**  
Packet switches for SS7 messages. Routes signaling between SSPs and SCPs.

**SCP — Service Control Point**  
Database node. Responds to queries from SSPs — number portability lookups, HLR queries, service logic.

**HLR — Home Location Register**  
The definitive subscriber database for a carrier. Maps each IMSI to: current serving network, current VLR address, active service profile, call forwarding rules. When a call arrives for a subscriber, the originating network queries the HLR to find out where they currently are.

**VLR — Visitor Location Register**  
Temporary subscriber registry. When a device roams onto a network, that network's VLR registers the device and notifies the home HLR. The VLR holds a local copy of the subscriber profile to avoid querying the HLR on every transaction.

**MSC — Mobile Switching Center**  
Manages call routing, handoffs between cells, and interfaces to the SS7 network.

### 2.3 MAP — Mobile Application Part

MAP is the SS7 application layer most directly relevant to mobile subscriber surveillance. Defined in 3GPP TS 29.002.

MAP enables:

- **`sendRoutingInfo`** — query HLR for a subscriber's current location (returns serving MSC address and IMSI)
- **`provideSubscriberInfo`** — request current location information including cell ID
- **`insertSubscriberData`** — write new service profile data into a subscriber's HLR/VLR entry — including call forwarding rules
- **`sendRoutingInfoForSM`** — route an SMS, returns subscriber location
- **`registerSS`** — register supplementary services including call forwarding

These are the MAP operations exploited in SS7 attacks. `sendRoutingInfo` is the location tracking primitive. `insertSubscriberData` is the call interception primitive — it can silently add a forwarding rule to route calls through an attacker-controlled node.

### 2.4 The authentication problem

SS7 was designed in 1975. Authentication between network nodes is based on network membership — a node on the SS7 network is trusted because it is on the SS7 network. There is no cryptographic verification of node identity. Any node with SS7 network access can issue MAP queries and commands as if it were a legitimate carrier.

SS7 access is theoretically restricted to licensed telecommunications operators. In practice access has been sold through grey markets, obtained through smaller carriers in jurisdictions with weak regulatory oversight, and directly held by intelligence agencies. The public demonstration of SS7 location tracking and call interception by Tobias Engel at the 2014 Chaos Communication Congress used documented, legitimate MAP operations — not exploits in the conventional sense. The protocol behaves as designed. The design is the problem.

---

## 3. SIM Toolkit (STK)

### 3.1 Architecture

SIM Toolkit is an application execution environment embedded in the SIM card OS. SIM cards run JavaCard — a subset of Java designed for constrained hardware. STK is a set of proactive commands the SIM can issue to the device handset and a set of commands the network can issue to the SIM.

The SIM is a separate computer. It has its own processor, its own OS, its own application layer. It communicates with the device via the SIM-ME interface. STK commands issued by the SIM are processed by the device before reaching Android or iOS.

### 3.2 Proactive commands

STK commands the SIM can issue to the device:

| Command | Function |
|---|---|
| `PROVIDE LOCAL INFORMATION` | Request IMEI, cell ID, battery, time |
| `SEND SHORT MESSAGE` | Transmit SMS without user action |
| `SET UP CALL` | Initiate a call without user action |
| `LAUNCH BROWSER` | Open a URL |
| `DISPLAY TEXT` | Display text on screen |
| `GET INKEY / GET INPUT` | Request user input |
| `SET UP EVENT LIST` | Register for device events (call, location change) |

Commands arrive via OTA (Over-The-Air) SMS — a specially formatted binary SMS processed by the SIM before the device OS. In implementations that do not validate incoming OTA commands, these execute silently with no user notification.

### 3.3 OTA update mechanism

Carriers can push updates to SIM applets over-the-air using the OTA mechanism. OTA messages are encrypted and authenticated using keys provisioned on the SIM at manufacture — the carrier holds the corresponding keys. This mechanism is used legitimately for carrier settings updates, service provisioning, and applet installation.

The security of OTA channels varies between carriers and SIM generations. Older SIMs used DES encryption for OTA channels. Modern SIMs use 3DES or AES. Vulnerabilities in OTA channel implementation have been documented historically.

### 3.4 SIM Toolkit on Android

Android includes a `com.android.stk` system package — one per SIM slot on multi-SIM devices. This package is the interface between the device OS and STK commands from the SIM. It can be disabled via ADB without root:

```bash
adb shell pm disable-user --user 0 com.android.stk
adb shell pm disable-user --user 0 com.android.stk2
```

`disable-user` disables for the current user profile without modifying the system partition. Reversible with `pm enable`.

Legitimate uses of STK on Swedish consumer devices: carrier menus (rarely used), legacy visual voicemail on some carriers. GrapheneOS disables STK by default.

### 3.5 JavaCard and programmable SIMs

Standard operator SIMs have a locked applet partition — writing custom applets requires the carrier's Administrative Key (ADM), which subscribers do not have access to. Blank programmable JavaCard SIMs exist for M2M and IoT development without operator locks. These accept custom applet installation via GlobalPlatformPro or equivalent tools using the default transport key.

A custom applet on a programmable SIM can intercept incoming STK commands before execution — inspecting, logging, or blocking specific command types. This is architecturally sound but requires a blank SIM; it is not applicable to standard operator SIMs.

---

## 4. GSM Call Forwarding — Protocol Detail

### 4.1 How forwarding is stored

Call forwarding rules are stored in the subscriber's HLR entry as supplementary service data. When an incoming call arrives for a subscriber, the originating network queries the HLR via MAP `sendRoutingInfo`. The HLR response includes the current serving MSC — or, if unconditional forwarding is active, the forwarding destination number instead.

The originating network then routes the call to the forwarding destination without any further involvement from the subscriber's device.

### 4.2 Setting forwarding rules

Forwarding rules are written via GSM standard MMI codes, which translate to MAP `registerSS` operations on the network:

```
*21*[destination]#   — activate unconditional call forwarding
*21#                 — deactivate unconditional call forwarding
**21*[destination]#  — register without activating
*#21#                — query current forwarding destination
```

These are processed by the carrier as legitimate subscriber-initiated service changes. They are indistinguishable at the network level from any other forwarding configuration.

### 4.3 SMS forwarding

SMS forwarding is handled differently from voice — it is not a standard GSM supplementary service in the same way. Practical SMS forwarding in a chain requires either:

- Carrier-provided SMS forwarding features (available on some carriers via portal or USSD)
- A device receiving and re-forwarding programmatically
- Virtual number providers with API-based forwarding rules

---

## 5. IMSI and Subscriber Identity

### 5.1 IMSI structure

The International Mobile Subscriber Identity is a 15-digit number stored on the SIM:

```
MCC (3 digits) — Mobile Country Code
MNC (2-3 digits) — Mobile Network Code  
MSIN (remaining digits) — subscriber identifier within the network
```

Example: `24007XXXXXXXXX` — MCC 240 (Sweden), MNC 07 (Tele2 SE)

The IMSI is never transmitted in plaintext over the radio interface in normal operation — a temporary TMSI (Temporary Mobile Subscriber Identity) is used instead, refreshed periodically. The IMSI is transmitted during initial registration or when the network cannot resolve the TMSI.

### 5.2 IMEI

The International Mobile Equipment Identity identifies the device hardware, not the SIM. A 15-digit number burned into the device at manufacture. `*#06#` displays the IMEI on any device.

IMEI and IMSI together allow correlation of device and subscriber. Changing SIM does not change IMEI. Changing device does not change IMSI.

---

## 6. References

- ITU-T Q.700 series — SS7 specifications. itu.int
- 3GPP TS 29.002 — MAP specification
- 3GPP TS 51.014 — SIM Application Toolkit specification
- IETF RFC 4666 — SCCP over IP (SIGTRAN)
- GSMA SGP.02 — M2M Embedded SIM Remote Provisioning Architecture
- Engel, T. (2014). *SS7: Locate. Track. Manipulate.* CCC media archive.
- GlobalPlatformPro — github.com/martinpaljak/GlobalPlatformPro

---

*Pablo Chacon — June 2026 — CC BY 4.0*
