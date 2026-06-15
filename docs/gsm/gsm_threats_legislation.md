# Threats & Legislation

**Author:** Pablo Chacon  
**Published:** June 2026  
**License:** CC BY 4.0

**Part of series:**  
→ [GSM Routing Chains](./gsm_routing_chains.md) *(main)*  
→ [Protocol & SS7](./gsm_protocol_ss7.md)

---

## Overview

This document maps the threat landscape against mobile subscribers — the actors, the techniques, and the legal frameworks that enable or constrain them. The Swedish legislative context is covered in detail as a concrete example of a liberal democracy's surveillance infrastructure. The mitigations referenced here are developed fully in [GSM Routing Chains](./gsm_routing_chains.md).

---

## 1. Threat Actor Taxonomy

| Actor | SS7 access | IMSI catchers | STK/OTA | Legal intercept |
|---|---|---|---|---|
| Domestic nation-state | Yes — via carrier obligation | Yes — confirmed operational | Via carrier cooperation | Yes — LEK/FRA |
| Foreign nation-state | Via partner agencies or grey market | Deployable | No | No |
| Licensed domestic operator | Yes — inherent | N/A | Yes — direct SIM access | Legally obligated to enable |
| Criminal / grey market | Purchasable via intermediaries | Available commercially | No | No |
| Corporate intelligence | SS7 access purchasable | Rentable | No | No |

The distinction between "nation-state" and "licensed operator" is partially illusory in practice — operators are legally obligated to provide lawful intercept interfaces and to cooperate with authorized requests. The operator's infrastructure is the state's intercept infrastructure by legal design.

---

## 2. SS7 Attacks

### 2.1 Location tracking

MAP `sendRoutingInfo` queries the HLR for a subscriber's current serving MSC. The MSC address resolves to a geographic area. MAP `provideSubscriberInfo` can return cell-level location — accurate to the serving cell's coverage area, typically hundreds of meters in urban environments.

These are standard MAP operations. Any SS7 node can issue them. The network does not validate whether the requesting node has a legitimate reason to query a given subscriber's location.

Real-world demonstration: Tobias Engel's 2014 CCC presentation tracked a German politician's movements in real time using documented MAP operations, with the subject's knowledge and consent, to demonstrate the capability publicly. No exploit was involved. The protocol behaved as designed.

### 2.2 Call interception

MAP `insertSubscriberData` can write new data into a subscriber's HLR entry — including call forwarding rules. An attacker with SS7 access can silently add an unconditional forward that routes the subscriber's calls through an attacker-controlled node before delivery.

From the subscriber's perspective: calls ring normally. No indication of interception exists at the device level. The interception node receives the call, records it, and forwards it to the original destination.

### 2.3 SMS interception

The same forwarding mechanism applies to SMS. MAP `sendRoutingInfoForSM` routes an SMS by returning the subscriber's current MSC. An attacker can manipulate this routing to intercept SMS before delivery.

Practical consequence: SMS-based two-factor authentication is not a reliable second factor. An attacker with SS7 access can intercept the OTP without any access to the device. This has been confirmed in documented attacks against bank accounts and cryptocurrency exchanges.

### 2.4 Who has SS7 access

Access is theoretically restricted to licensed telecommunications operators. Practically:

- **Intelligence agencies** of major nations hold SS7 access through carrier relationships and direct infrastructure
- **Grey market brokers** have operated SS7 interconnects, documented in investigative journalism and academic research
- **Smaller carriers** in jurisdictions with weak regulatory oversight have been used as access points — a carrier in an obscure jurisdiction with loose oversight can be used as an SS7 node
- SS7 access has been offered commercially, with pricing documented in leaked documents from private intelligence vendors

The barrier to SS7 access is regulatory and financial, not technical. The protocol has no mechanism to enforce legitimate use.

---

## 3. IMSI Catchers

### 3.1 Operation

An IMSI catcher impersonates a legitimate cellular base station. GSM devices connect to the strongest available signal automatically — there is no subscriber opt-in and no visible indication that the device has connected to a non-legitimate tower.

From that position:

- **IMSI harvesting** — all devices in range transmit their IMSI during the connection process, identifying every subscriber physically present in the area
- **Real-time interception** — voice and SMS in active mode
- **Location pinpointing** — signal strength triangulation, accurate to meters
- **2G downgrade** — historic IMSI catchers forced a 2G connection because 2G lacks mutual authentication (the network does not prove its identity to the device). 2G voice is trivially decryptable.
- **STK injection** — in active mode, some implementations can push SIM Toolkit commands to connected devices

### 3.2 Commercial availability

IMSI catchers are commercially sold as "lawful intercept" tools. Harris Corporation's Stingray product line is the most documented. DRTBOX, Cell-Hawk, and numerous foreign equivalents exist. Second-hand units appear on grey markets. Research-grade IMSI catchers can be assembled from software-defined radio hardware for under $1,000.

Swedish law enforcement and SÄPO have confirmed operational use. The classification of IMSI catchers as "technical aids" under Swedish law rather than wiretapping equipment has, in certain interpretations, allowed deployment without the same judicial authorization threshold required for conventional wiretapping.

### 3.3 Modern evasion of detection

Historically IMSI catchers were detectable through:

- Forced 2G downgrade (no mutual authentication in 2G)
- Unknown Cell ID not present in crowdsourced databases
- Absence of neighboring cells (real towers have overlapping coverage)
- Loss of connectivity despite strong signal (early devices terminated rather than relaying)

Modern deployments address these:

- **4G/LTE capable** — no longer require 2G downgrade
- **Passive mode** — monitor without transmitting; no detectable radio anomaly
- **Tower parameter cloning** — copy Cell IDs and parameters from legitimate neighboring towers exactly
- **Relay mode** — man-in-the-middle architecture; connectivity is maintained normally

A passive LTE IMSI catcher operating in relay mode with cloned tower parameters is essentially undetectable from device software.

### 3.4 Detection tools

Software detection approaches work against unsophisticated or older deployments:

- **SnoopSnitch** (SecUpwN) — requires root, accesses Qualcomm diagnostic modem interface directly, reads lower-level radio data than standard Android APIs. Most reliable software option. Available on F-Droid.
- **AIMSICD** — Android IMSI-Catcher Detector, active 2014–2017, now largely dormant. Forks maintained on GitHub. Less reliable than SnoopSnitch.
- **Android 12+ native alerts** — 2G downgrade notification and unencrypted network warning available in some builds under Settings → Location.

Hardware detection:

- **RTL-SDR dongle** (~$25) — passive software-defined radio. Monitors GSM frequencies independently of the device being monitored. Anomalous signals in the spectrum are visible regardless of what the device OS reports.
- **YARD Stick One + GNU Radio** — more capable SDR setup for active radio environment analysis.
- **OsmocomBB** — open source GSM baseband implementation running on specific legacy hardware (Motorola C123). Enables inspection of raw GSM signaling independent of a commercial modem.

Hardware detection is the architecturally sound approach — a second independent radio cannot be deceived by a compromise of the primary device.

---

## 4. SIM Toolkit Exploitation — Simjacker

### 4.1 Discovery and disclosure

In September 2019, Adaptive Mobile Security published research on a vulnerability they named Simjacker. The attack used a single OTA binary SMS containing STK commands to silently:

1. Execute `PROVIDE LOCAL INFORMATION` — harvest the device's cell ID and IMEI
2. Execute `SEND SHORT MESSAGE` — transmit the harvested data to an attacker-controlled number

No user interaction required. No notification displayed. No indication at the device or SIM level visible to the subscriber.

### 4.2 Affected infrastructure

The vulnerability was present in SIMs running the S@T Browser (SIM Alliance Toolbox Browser) applet — a legacy STK application originally designed for WAP-era carrier services, still present on an estimated one billion SIMs at time of disclosure across at least 30 countries.

### 4.3 Active exploitation

Adaptive Mobile Security confirmed the vulnerability was being actively exploited in the wild, attributed to a private surveillance company operating on behalf of government clients. Targets in multiple countries had their locations tracked over extended periods via repeated automated Simjacker queries — multiple times per day in documented cases.

The attack required no device compromise. No malware. No user interaction. Only a SIM supporting the S@T applet and a sender with access to the OTA SMS infrastructure.

### 4.4 Structural implication

Simjacker is not an anomaly. It is an instance of the general vulnerability: STK executes below the device OS, commands arrive via a channel invisible to the user, and the attack surface is defined by whatever applets are present on the SIM. S@T Browser was the specific vector in 2019. Other applets, other carriers, other implementations of the same architecture represent the continuing surface.

---

## 5. Legislative Context — Sweden

### 5.1 FRA-lagen (2008:717)

The Swedish Defence Radio Authority (Försvarets radioanstalt) is authorized under FRA-lagen to conduct signals intelligence on cable-based communications traffic crossing Sweden's borders. The law was passed in 2008 after significant public controversy and has been amended, but bulk collection authority on cross-border traffic remains in force.

FRA operates collection points at major international cable crossing points within Sweden. Traffic transiting these points — which includes a significant portion of northern European internet traffic due to Sweden's geography and infrastructure role — is subject to collection and analysis under the framework.

### 5.2 LEK — Lag om elektronisk kommunikation (2022:482)

The Electronic Communications Act governs licensed telecommunications operators in Sweden. Relevant provisions:

- **Lawful intercept obligation** — licensed operators must maintain technical capability for real-time interception of communications content and metadata, accessible to authorized authorities. This is not optional. It is a licensing condition.
- **Assistance obligation** — operators must cooperate with law enforcement requests for subscriber data, call records, and real-time interception under judicial authorization.

Telia, Tele2, Tre, and Telenor SE are all licensed Swedish operators subject to LEK. The lawful intercept interface is built into their infrastructure by legal requirement.

### 5.3 Metadata retention

Sweden has maintained domestic metadata retention requirements following the EU Court of Justice's invalidation of the EU Data Retention Directive (C-293/12, Digital Rights Ireland, 2014). Swedish operators are required to retain:

- Call records (origin, destination, duration, time)
- SMS records
- Cell tower location data for each call/SMS event
- Internet session metadata

Retention periods vary by data category. This retained dataset is the foundation for retroactive chain reconstruction — it is what a legal request to a Swedish carrier produces.

### 5.4 The M2M jurisdictional gap

Foreign M2M carriers are not subject to LEK. They have no Swedish metadata retention obligation. They are not parties to automatic data-sharing arrangements with Swedish operators. Accessing their records requires:

- A formal Mutual Legal Assistance Treaty (MLAT) request to the relevant jurisdiction
- Processing through that jurisdiction's legal system under its own laws and timelines
- Assuming the jurisdiction has a relevant MLAT with Sweden at all

MLAT processes are measured in months, not days. Swedish metadata retention windows are measured in months. The intersection of these timelines is the practical constraint.

### 5.5 RIPA parallel — UK context

For reference: the UK's Investigatory Powers Act 2016 (successor to RIPA) requires UK operators to maintain bulk interception capability, retain metadata for 12 months, and provide access under warrant. The architecture is identical to Sweden's — operator infrastructure is legally required lawful intercept infrastructure. This is the standard model across EU/EEA states and most liberal democracies.

The implication: domestic operators in any jurisdiction operating under this model are, by design, lawful intercept nodes. This is not a vulnerability in the conventional sense. It is the intended architecture.

---

## 6. References

- Engel, T. (2014). *SS7: Locate. Track. Manipulate.* Chaos Communication Congress. CCC media archive.
- Nohl, K. & Melette, L. (2014). *SS7: Locate. Track. Manipulate.* SRLabs. srlabs.de
- Positive Technologies. (2018). *Vulnerabilities in Mobile Networks.* ptsecurity.com
- Adaptive Mobile Security. (2019). *Simjacker — Next Generation Spying Over Mobile.* adaptivemobile.com
- CJEU C-293/12 — Digital Rights Ireland. Struck down EU Data Retention Directive.
- FRA-lagen (2008:717) — Swedish signals intelligence law. riksdagen.se
- Lag (2022:482) om elektronisk kommunikation — Swedish electronic communications act. riksdagen.se
- SnoopSnitch — f-droid.org / github.com/SecUpwN/Android-IMSI-Catcher-Detector
- RTL-SDR — rtl-sdr.com
- OsmocomBB — osmocom.org/projects/baseband

---

*Pablo Chacon — June 2026 — CC BY 4.0*
