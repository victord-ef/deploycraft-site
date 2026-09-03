---
title: "IDS/IPS System Architecture and Detection Models — Part 1"
date: 2026-09-01
description: "Understand how intrusion detection and prevention systems work: signature, anomaly, and behaviour-based detection models, inline vs passive sensor placement, and the architectural tradeoffs of NIDS, HIDS, and hybrid deployments."
cluster: "Network Security"
series: "IDS/IPS"
part: 1
difficulty: "intermediate"
duration: "45 min"
tags: ["ids", "ips", "network-security", "intrusion-detection", "suricata", "snort", "devsecops", "threat-analysis"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

By the end of this tutorial you will understand how IDS and IPS systems are architected, how each detection model works and where it fails, and how to position sensors correctly in a real network. Part 2 deploys Suricata as a working network IPS with rule management, tuning, and alert pipeline.

## Prerequisites

- Familiarity with TCP/IP networking (packets, ports, protocols)
- Basic understanding of Linux networking (`tcpdump`, network interfaces)
- Access to a Linux host or VM for lab exercises

---

## IDS vs IPS — the operational distinction

An **Intrusion Detection System (IDS)** observes a copy of traffic and raises alerts. It cannot block anything. An **Intrusion Prevention System (IPS)** sits inline in the traffic path and can drop packets before they reach their destination.

| Property | IDS | IPS |
|---|---|---|
| Placement | Out-of-band (passive tap or SPAN port) | Inline (bridge or router hop) |
| Traffic impact | None — sees a copy | Adds latency on every packet |
| Blocking capability | No — alert only | Yes — drop, reset, or redirect |
| False positive risk | Alert fatigue | Service disruption |
| Failure mode | Silent (traffic continues) | Can block legitimate traffic |

The right choice depends on where you are in your security maturity. IDS is safer to deploy first — it gives visibility without risk of disruption. IPS is added once the ruleset is tuned and false positive rates are acceptable.

---

## Step 1 — Detection models

Every IDS/IPS uses one or more of three detection approaches. Understanding which model is active for a given rule explains why it fires and how to tune it.

### Signature-based detection

Signature detection matches known attack patterns against packet content. Each signature is a precise description of a known exploit, malware payload, or protocol anomaly.

```
alert tcp any any -> $HOME_NET 22 (
  msg:"SSH brute force attempt";
  flow:to_server,established;
  content:"SSH-";
  detection_filter:track by_src, count 10, seconds 60;
  sid:1000001;
  rev:1;
)
```

This Suricata rule fires when a single source IP sends more than 10 SSH connection attempts in 60 seconds.

**Strengths:** Very low false positive rate for known attacks. Fast to evaluate. Deterministic.

**Weaknesses:** Zero-day attacks have no signature. Encrypted traffic hides payloads. Attackers slightly mutate payloads to evade fixed signatures.

### Anomaly-based detection

Anomaly detection builds a statistical model of normal behaviour — baseline traffic volume, connection rates, protocol distribution — and alerts on deviations.

Examples:
- DNS query rate exceeds 3 standard deviations above the hourly mean
- A host that normally receives 100 connections/min suddenly receives 50,000/min
- An internal host begins sending traffic to countries it has never communicated with

**Strengths:** Can detect novel attacks with no known signature. Detects zero-days by their effect on traffic patterns.

**Weaknesses:** High false positive rate during baseline changes (deployments, batch jobs, business events). Requires weeks of training data to establish a meaningful baseline. Baseline drift over time degrades accuracy.

### Behaviour-based detection

Behaviour detection models the expected sequence of actions for a protocol or application and alerts when the sequence is violated.

Example: a legitimate HTTP client sends a request, receives a response, and either closes the connection or sends another request. An HTTP client that sends a request and then immediately begins scanning other ports is violating expected behaviour — likely post-exploitation lateral movement.

**Strengths:** Catches multi-stage attacks and post-compromise activity that has no signature. Less sensitive to payload encryption than signature detection.

**Weaknesses:** Requires accurate protocol models. Application behaviour varies enough that model coverage is incomplete.

### How modern systems combine all three

Production IDS/IPS engines like Suricata and Snort 3 use all three approaches simultaneously on different rule layers. A single alert may be triggered by a signature match on the initial payload, reinforced by anomaly scoring on connection volume, and confirmed by protocol state violation.

---

## Step 2 — Network topology placement

Where you place a sensor determines what it can and cannot see.

### SPAN port (passive tap)

A managed switch mirrors all traffic on selected ports to a dedicated SPAN (Switched Port Analyzer) port. The IDS connects to the SPAN port and receives a copy of all traffic.

```
┌─────────────┐    ┌──────────────────┐    ┌─────────────┐
│  Internet   │───▶│  Core Switch     │───▶│  Servers    │
│  (WAN)      │    │                  │    │             │
└─────────────┘    │  SPAN port ──────│    └─────────────┘
                   └──────────────────┘
                              │
                              ▼
                   ┌──────────────────┐
                   │   IDS Sensor     │
                   │  (passive, no    │
                   │   blocking)      │
                   └──────────────────┘
```

**SPAN limitations:**
- Drops packets under high load (switch prioritises forwarding over mirroring)
- Cannot mirror encrypted traffic at the switch level
- Cannot block — alert only

### Inline (IPS mode)

The sensor is placed in the physical or logical traffic path. All packets traverse the sensor before being forwarded.

```
┌─────────────┐    ┌──────────────────┐    ┌─────────────┐
│  Internet   │───▶│   IPS Sensor     │───▶│  Firewall   │───▶ Internal
│  (WAN)      │    │  (inline bridge) │    │             │
└─────────────┘    └──────────────────┘    └─────────────┘
                            │
                    Drop malicious
                    packets here
```

**Inline requirements:**
- High-availability pair with hardware bypass (if sensor fails, traffic must pass — fail-open — or stop — fail-closed)
- Low-latency processing: an IPS adding more than 1–2ms to every packet is operationally unacceptable
- Sufficient throughput: must match or exceed peak line rate

### Network TAP

A passive optical or electrical TAP physically splits the signal and sends a copy to the sensor. Unlike a SPAN port, a TAP cannot drop packets under load — it is a hardware split.

```
┌─────────────┐    ┌──────────────────┐    ┌─────────────┐
│  Internet   │───▶│  Network TAP     │───▶│  Firewall   │
└─────────────┘    └──────────────────┘    └─────────────┘
                            │
                     (copy of traffic)
                            ▼
                   ┌──────────────────┐
                   │   IDS Sensor     │
                   └──────────────────┘
```

TAPs are preferred over SPAN ports for production IDS deployments because they guarantee full packet delivery without drops.

---

## Step 3 — NIDS, HIDS, and hybrid architecture

### Network IDS (NIDS)

A NIDS sensor monitors traffic at a network chokepoint — typically between the perimeter firewall and the internal network. It sees all inter-zone traffic but cannot inspect encrypted TLS sessions (unless combined with TLS inspection).

**Best for:** Perimeter monitoring, detecting lateral movement, C2 beacon detection, DNS exfiltration.

**Cannot see:** Traffic encrypted end-to-end at the application layer. Host-level events (process execution, file changes).

### Host IDS (HIDS)

A HIDS agent runs on each endpoint and monitors system-level activity: file integrity, process creation, network connections from the host, system calls, and log events.

Tools: OSSEC, Wazuh, Falco (container-aware HIDS), auditd.

**Best for:** Detecting post-exploitation activity after a perimeter bypass. File integrity monitoring (FIM). Container escape detection.

**Cannot see:** Network traffic from other hosts. Encrypted network streams (sees the plaintext before encryption).

### Hybrid architecture

Production security operations centres run both:

```
                    ┌──────────────────────────────────────┐
                    │             SIEM / SOC                │
                    │   (alert correlation & enrichment)    │
                    └────────────┬─────────────────────────┘
                                 │
               ┌─────────────────┼──────────────────┐
               │                 │                  │
    ┌──────────▼─────┐  ┌────────▼───────┐  ┌──────▼──────────┐
    │  NIDS (network │  │  HIDS (host    │  │  EDR (endpoint  │
    │  sensor at     │  │  agent on each │  │  detection &    │
    │  perimeter)    │  │  server/VM)    │  │  response)      │
    └────────────────┘  └────────────────┘  └─────────────────┘
```

The SIEM correlates alerts from all three sources. A NIDS alert for a port scan followed 5 minutes later by a HIDS alert for a new user account creation on the scanned host is a high-confidence incident — neither alert alone would warrant immediate escalation.

---

## Step 4 — The IPS alert pipeline

Understanding where alerts go after the sensor fires is as important as the detection itself.

```
Packet captured
      │
      ▼
Rule evaluation (signature + anomaly + protocol state)
      │
      ├── No match → forward packet
      │
      └── Match →
            │
            ├── IDS mode: log alert + forward packet
            │
            └── IPS mode: log alert + DROP packet
                    │
                    ▼
            Alert written to:
            ├── EVE JSON log (Suricata's structured output format)
            ├── Syslog forwarded to SIEM
            └── Alert database (Elasticsearch, Splunk)
                    │
                    ▼
            SIEM correlates with:
            ├── Other IDS/IPS alerts
            ├── Firewall logs
            ├── Authentication logs
            └── Threat intelligence feeds
                    │
                    ▼
            SOC analyst reviews:
            ├── True positive → incident response
            └── False positive → rule tuning
```

The EVE JSON format (Suricata's native output) produces structured records that are directly ingestible by Elasticsearch, Splunk, and Loki:

```json
{
  "timestamp": "2026-09-01T10:34:22.123456+0000",
  "flow_id": 1234567890,
  "event_type": "alert",
  "src_ip": "203.0.113.42",
  "src_port": 54321,
  "dest_ip": "10.0.1.5",
  "dest_port": 22,
  "proto": "TCP",
  "alert": {
    "action": "blocked",
    "gid": 1,
    "signature_id": 1000001,
    "rev": 1,
    "signature": "SSH brute force attempt",
    "category": "Attempted Information Leak",
    "severity": 2
  }
}
```

---

## Step 5 — Common evasion techniques and countermeasures

Understanding evasion helps you evaluate your IDS ruleset coverage.

### Fragmentation evasion

An attacker splits a malicious payload across multiple IP fragments. A naive IDS checking each fragment independently sees only partial data and misses the pattern. **Countermeasure:** IP reassembly at the sensor before inspection (all modern IDS engines do this).

### Protocol-level evasion

Attackers exploit ambiguity in protocol implementations. A web server and an IDS may interpret the same malformed HTTP request differently — the IDS sees a benign request, the server processes the exploit. **Countermeasure:** Protocol normalisation — the IDS applies the same parsing rules as the target application before running detection.

### Encryption

TLS 1.3 encrypts everything except the SNI and basic handshake metadata. Payload signature detection is impossible without TLS inspection. **Countermeasure:** TLS inspection (terminating proxy decrypts, passes plaintext to IDS, re-encrypts). Alternatively, use JA3/JA3S fingerprinting of TLS handshakes — the client hello parameters identify known malware C2 clients without decrypting content.

### Timing evasion

A slow port scan that sends one probe every 10 minutes evades rate-based detection rules. **Countermeasure:** Long-window detection rules with persistent state (track by source IP over hours, not seconds).

---

## Step 6 — Deploying a lab NIDS with tcpdump

Before deploying Suricata, verify your sensor sees the traffic you expect using a packet capture:

```bash
# Install tcpdump
apt-get install -y tcpdump

# Capture all traffic on the monitored interface
tcpdump -i eth0 -w /tmp/capture.pcap -c 10000

# Verify traffic volume
tcpdump -r /tmp/capture.pcap | wc -l

# Check that both directions are captured (src and dst)
tcpdump -r /tmp/capture.pcap 'tcp port 80' | head -20

# Verify you see traffic from multiple hosts (SPAN/TAP working)
tcpdump -r /tmp/capture.pcap -n | awk '{print $3}' | cut -d. -f1-4 | sort -u | head -20
```

If the capture only shows traffic destined for the sensor host itself (not traffic between other hosts), the SPAN port or TAP is misconfigured — promiscuous mode is required on the capture interface:

```bash
ip link set eth0 promisc on
```

---

## Step 7 — Sizing and performance

An IDS/IPS sensor must process packets faster than they arrive. Under-sizing causes packet drops — missed attacks.

**Key metrics to size against:**

| Metric | How to measure | Typical threshold |
|---|---|---|
| Peak throughput (Gbps) | `iftop`, `nethogs`, or switch counters | Sensor must exceed peak by 20% |
| Packets per second (PPS) | `sar -n DEV 1` | CPU-bound at high PPS |
| Active flows | `ss -s` on gateway | Affects state table size |
| Rule count | `suricata --list-runmodes` | More rules = more CPU per packet |

**Quick sizing guide:**

- 1 Gbps link, 5,000 rules: 2–4 CPU cores, 4GB RAM
- 10 Gbps link, 10,000 rules: 8–16 CPU cores, 16GB RAM, dedicated NIC with RSS
- 40+ Gbps: hardware-accelerated packet capture (DPDK or AF_XDP), dedicated appliance

---

## What you have built

- A clear model of IDS vs IPS operational tradeoffs — when to use each
- Understanding of signature, anomaly, and behaviour detection models with their strengths and failure modes
- Sensor placement options — SPAN port, inline, network TAP — and when to use each
- NIDS vs HIDS architecture and how they compose in a production SOC
- The alert pipeline from packet capture through SIEM correlation to analyst review
- Common evasion techniques and countermeasures for each
- Lab verification of sensor placement with tcpdump
- Sizing guidance for real network throughput

In [Part 2](/tutorials/network-security/deploying-suricata-network-ids-ips-part-2/) you will deploy Suricata as a working NIDS/IPS: install it on a Linux host, load the Emerging Threats ruleset, configure EVE JSON output, tune rules to suppress false positives, and build a Suricata alert pipeline to a log aggregator.
