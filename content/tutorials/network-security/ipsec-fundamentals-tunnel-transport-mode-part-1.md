---
title: "IPSec Fundamentals — IKEv2, Security Associations, and Tunnel vs Transport Mode — Part 1"
date: 2026-09-01
description: "Understand how IPSec works end-to-end: the IKEv2 handshake phases, Security Association negotiation, AH vs ESP protocols, and the architectural difference between tunnel and transport mode — when each applies and why."
cluster: "Network Security"
series: "IPSec"
part: 1
difficulty: "intermediate"
duration: "45 min"
tags: ["ipsec", "vpn", "network-security", "ikev2", "cryptography", "networking", "devsecops"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

By the end of this tutorial you will understand how IPSec establishes encrypted tunnels at the network layer: the two-phase IKEv2 negotiation, what a Security Association is and how it maps to a unidirectional encrypted channel, and the difference between tunnel and transport mode. Part 2 configures IPSec site-to-site and road-warrior tunnels using strongSwan on Linux.

## Prerequisites

- TCP/IP fundamentals — IP headers, routing, NAT
- Basic understanding of symmetric and asymmetric cryptography (see [Pair 79](/tutorials/network-security/cryptography-algorithms-symmetric-asymmetric-hash-part-1/))

---

## What IPSec is

IPSec (Internet Protocol Security) is a suite of protocols that secure IP communications at the network layer (Layer 3). Unlike TLS, which secures a single application connection, IPSec encrypts every IP packet between two endpoints — protecting all traffic regardless of the application above.

IPSec operates at the kernel level. There is no application-level wrapper — the OS intercepts packets matching a Security Policy, applies encryption/authentication, and forwards the result. The receiving OS decrypts and authenticates before passing the packet to the application.

**The three components of IPSec:**

| Component | Function |
|---|---|
| **IKE (Internet Key Exchange)** | Negotiates and manages Security Associations — the control plane |
| **AH (Authentication Header)** | Authenticates packet origin and integrity, no encryption |
| **ESP (Encapsulating Security Payload)** | Encrypts and authenticates packet payload — used in practice |

AH is rarely used alone today because it does not encrypt payload and is incompatible with NAT. ESP provides both encryption and authentication and is the standard choice.

---

## Step 1 — Security Associations and Security Policies

Before any encrypted traffic flows, IPSec establishes a **Security Association (SA)**. An SA is a one-directional agreement between two endpoints that defines:

- Which encryption algorithm to use (AES-256-GCM)
- Which authentication algorithm to use (SHA-384)
- The encryption key
- The SA lifetime (in bytes and seconds)
- An SPI (Security Parameter Index) — a 32-bit identifier in each packet header that references this SA

Because an SA is unidirectional, a bidirectional tunnel requires two SAs — one per direction. A VPN between host A and host B creates:
- SA1: A→B (A encrypts with SA1, B decrypts with SA1)
- SA2: B→A (B encrypts with SA2, A decrypts with SA2)

SAs are stored in the **Security Association Database (SAD)** in the kernel.

**Security Policies (SP)** define which traffic should be processed by IPSec. An SP matches traffic by source/destination IP and port, and specifies the action: bypass (no IPSec), discard (drop), or protect (apply IPSec with a specified SA). Security Policies are stored in the **Security Policy Database (SPD)**.

```
Outbound packet from 10.0.1.5 → 10.0.2.10

1. Kernel checks SPD:
   Match: src=10.0.1.0/24, dst=10.0.2.0/24 → protect
   
2. Kernel looks up SA in SAD:
   SPI: 0x12345678, algorithm: AES-256-GCM, key: <256-bit key>
   
3. Kernel applies ESP encapsulation and encryption

4. Encrypted packet sent to 10.0.2.1 (IPSec gateway)
```

---

## Step 2 — IKEv2 handshake phases

IKEv2 establishes SAs through a two-exchange protocol. All IKE communication uses UDP port 500 (or 4500 when NAT is involved).

### Phase 1 — IKE_SA_INIT

The first exchange establishes a secure channel for the IKE negotiation itself. It is not yet securing data traffic.

```
Initiator                                    Responder
    │                                             │
    │── IKE_SA_INIT request ──────────────────▶  │
    │   • Proposed IKE algorithms                 │
    │     (cipher, PRF, DH group, integrity)      │
    │   • DH public value (g^i mod p)             │
    │   • Nonce Ni                                │
    │                                             │
    │  ◀────────────────── IKE_SA_INIT response ──│
    │   • Selected algorithms                     │
    │   • DH public value (g^r mod p)             │
    │   • Nonce Nr                                │
    │   • Certificate (optional)                  │
    │                                             │
    Both sides compute shared secret:             │
    DH(g^i, g^r) = g^(ir) mod p                  │
    → SKEYSEED derived using Ni, Nr               │
    → Keys generated: SK_e (encrypt), SK_a (auth) │
```

After IKE_SA_INIT, both sides have a shared secret and can communicate securely. All subsequent IKE messages are encrypted with the negotiated keys.

### Phase 2 — IKE_AUTH

The second exchange authenticates the peers and establishes the first Child SA (which will carry data traffic).

```
Initiator                                    Responder
    │                                             │
    │── IKE_AUTH request (encrypted) ──────────▶ │
    │   • IDi (initiator identity)               │
    │   • AUTH (signature over Phase 1 messages) │
    │   • Certificate (if cert-based auth)       │
    │   • Proposed Child SA parameters:          │
    │     - Traffic selectors (what traffic)     │
    │     - ESP algorithms (AES-256-GCM + SHA384)│
    │                                             │
    │  ◀─────────────────── IKE_AUTH response ── │
    │   • IDr (responder identity)               │
    │   • AUTH (signature)                       │
    │   • Certificate                            │
    │   • Accepted Child SA parameters           │
    │   • SA1, SA2 (the actual data-plane SAs)   │
    │                                             │
    Data traffic now flows:                       │
    Encrypted with Child SA keys (not IKE keys)  │
```

The IKE_AUTH exchange authenticates using one of:
- **Pre-shared key (PSK):** Both sides compute an AUTH value using the PSK and phase 1 data. Simple, does not scale.
- **RSA/ECDSA certificate:** Each side signs the IKE Phase 1 data with its private key. The peer verifies the signature using the CA certificate. Scales — each peer only needs the CA cert.
- **EAP (Extensible Authentication Protocol):** Road-warrior (mobile user) authentication against RADIUS or LDAP. The server authenticates with a certificate; the client uses EAP.

### CHILD_SA rekeying

A CHILD_SA has a finite lifetime (e.g., 1 hour or 1 GB of data). When it nears expiry, IKEv2 automatically negotiates a new CHILD_SA with fresh keying material — **Perfect Forward Secrecy (PFS)**. Compromise of the current session key does not compromise past or future sessions.

```bash
# View established SAs on Linux (strongSwan)
sudo ipsec statusall
# Security Associations (1 up, 0 connecting):
#   site-b[1]: ESTABLISHED 5 minutes ago
#   site-b{1}:  INSTALLED, TUNNEL, reqid 1, ESP in UDP SPIs: ...
#     local:  10.0.1.0/24
#     remote: 10.0.2.0/24
```

---

## Step 3 — Tunnel mode vs transport mode

This is the most important architectural decision in an IPSec deployment.

### Tunnel mode

In tunnel mode, the **entire original IP packet** is encapsulated inside a new IP packet. The original IP header is encrypted along with the payload. The outer IP header carries the gateway addresses.

```
Original packet:
┌──────────────────────────────────────────────────────┐
│ IP Header (src: 10.0.1.5, dst: 10.0.2.10) │ Payload │
└──────────────────────────────────────────────────────┘

After ESP tunnel-mode encapsulation:
┌──────────────────────────────────────────────────────────────────────┐
│ Outer IP Header           │ ESP Header │ Encrypted:                  │
│ (src: GW-A, dst: GW-B)   │            │ [ Inner IP Hdr │ Payload ]  │
└──────────────────────────────────────────────────────────────────────┘
```

**The outer header carries gateway IPs — the original source and destination are hidden inside the encrypted payload.** An observer on the internet sees traffic between the two gateways but cannot determine the actual communicating hosts.

**When to use tunnel mode:**
- Site-to-site VPN: traffic between two networks, gateways are the IPSec endpoints
- Road-warrior VPN: client IP is the IPSec initiator, server IP is the responder
- Any scenario where the IPSec endpoints differ from the communicating hosts

Tunnel mode is the overwhelmingly common choice for VPN deployments.

### Transport mode

In transport mode, only the payload is encrypted. The original IP header remains in plaintext — only an ESP header is inserted between the IP header and the transport payload.

```
Original packet:
┌──────────────────────────────────────────────────────┐
│ IP Header (src: 10.0.1.5, dst: 10.0.2.10) │ Payload │
└──────────────────────────────────────────────────────┘

After ESP transport-mode encapsulation:
┌──────────────────────────────────────────────────────────────┐
│ IP Header (src: 10.0.1.5, dst: 10.0.2.10) │ ESP Hdr │ Encrypted Payload │
└──────────────────────────────────────────────────────────────┘
```

**The original source and destination IPs are visible** to observers. The payload content is encrypted, but the metadata (who is talking to whom) is exposed.

**When to use transport mode:**
- Host-to-host encryption between two specific servers that are also the IPSec endpoints
- L2TP over IPSec: transport mode protects the L2TP tunnel, which in turn carries the client traffic
- GRE over IPSec: a GRE tunnel carries multi-protocol traffic; IPSec transport mode encrypts it

**Transport mode limitation:** Incompatible with NAT. NAT rewrites IP addresses; if the original IP header changes, the IPSec authentication check (which covers the IP header in AH, and the addresses in the IKE negotiation) fails. In transport mode, if either endpoint is behind NAT, NAT-T (NAT Traversal) is required — which encapsulates ESP in UDP/4500.

---

## Step 4 — NAT traversal (NAT-T)

ESP is a Layer 3 protocol — it has no port numbers, which NAT devices require to maintain state. When an IPSec endpoint is behind NAT, IKEv2 detects this via NAT Detection payloads in IKE_SA_INIT and switches to UDP port 4500:

```
Without NAT-T:
Client (behind NAT) ──[ESP packet]──▶ NAT device ──❌ NAT drops ESP (no port)

With NAT-T:
Client (behind NAT) ──[UDP:4500 + ESP]──▶ NAT device ──✓ NAT maps UDP port ──▶ Server
```

IKEv2 performs NAT detection automatically. Keepalive packets (empty UDP/4500 datagrams) maintain the NAT binding. NAT-T is transparent to configuration on modern implementations.

---

## Step 5 — Cryptographic algorithm selection

IKEv2 negotiates separate algorithm sets for:
- **The IKE SA** (protecting the IKE control channel)
- **The Child SA (ESP)** (protecting data traffic)

### Recommended algorithm suite (2026)

```
IKE SA:
  Encryption:    AES-256-GCM (AEAD — combined encryption+auth)
  Integrity:     SHA-384 (only needed if not using AEAD)
  PRF:           PRF-HMAC-SHA-384
  DH group:      ECP-384 (Curve P-384) or DH-21 (MODP-8192)

Child SA (ESP):
  Encryption:    AES-256-GCM or ChaCha20-Poly1305
  Integrity:     SHA-384 (implicit with AEAD)
  DH group:      ECP-384 (for PFS)
```

### What to avoid

| Algorithm | Why to avoid |
|---|---|
| 3DES | 64-bit block cipher, Sweet32 birthday attack risk |
| DES | Single 56-bit key, trivially brute-forced |
| MD5, SHA-1 | Collision vulnerabilities |
| DH group 1, 2 (768/1024-bit MODP) | Insufficient key size — broken by nation-state adversaries |
| RC4 | Multiple known attacks |
| AES-CBC without integrity | Does not provide authenticated encryption — padding oracle attacks possible |

---

## Step 6 — Dead Peer Detection and failover

IKEv2 includes Dead Peer Detection (DPD) to identify when the remote peer has crashed or become unreachable. Periodic R_U_THERE messages test reachability; if unanswered, the SA is torn down.

```
# strongSwan DPD configuration
connections:
  site-b:
    dpd_timeout: 90s    # declare peer dead after 90s of no response
    dpd_delay: 30s      # send DPD probe every 30s
```

For high-availability site-to-site, configure two tunnels with different paths and use routing metrics or ECMP to prefer one:

```
Primary tunnel:  GW-A (203.0.113.1) ──▶ GW-B (198.51.100.1)   metric 100
Failover tunnel: GW-A (203.0.113.1) ──▶ GW-B2 (198.51.100.5)  metric 200
```

When DPD declares GW-B dead, the route to the primary tunnel is withdrawn and traffic fails over to the backup.

---

## Step 7 — IPSec packet walkthrough

Tracing a single packet through an IPSec tunnel reinforces every concept:

```
1. App on 10.0.1.5 sends TCP SYN to 10.0.2.10:443

2. Kernel routing: next hop is 10.0.1.1 (local gateway)

3. SPD lookup:
   src=10.0.1.5, dst=10.0.2.10 matches policy → protect
   → look up CHILD_SA for this traffic selector

4. SAD lookup:
   SPI=0xABCD1234, algo=AES-256-GCM, key=<256-bit>

5. ESP encapsulation (tunnel mode):
   a. Prepend ESP header with SPI=0xABCD1234 and sequence number
   b. Encrypt inner IP packet (src: 10.0.1.5 → dst: 10.0.2.10)
   c. Compute GCM authentication tag
   d. Prepend outer IP header (src: GW-A=203.0.113.1 → dst: GW-B=198.51.100.1)

6. Packet leaves as: [Outer IP: GW-A → GW-B][ESP Header][Encrypted: [Inner IP: 10.0.1.5 → 10.0.2.10][TCP SYN]]

7. GW-B receives packet:
   a. SAD lookup by SPI=0xABCD1234
   b. Decrypt and verify authentication tag
   c. Strip outer IP and ESP headers
   d. Route inner packet to 10.0.2.10 on the internal LAN

8. 10.0.2.10 receives TCP SYN from 10.0.1.5 — unaware of the VPN
```

---

## What you have built

- How IPSec operates at the kernel level — SPD, SAD, SA lifecycle
- IKEv2 two-phase handshake: IKE_SA_INIT (key agreement) and IKE_AUTH (authentication + Child SA)
- Security Association structure — unidirectional channels, SPI, algorithm negotiation
- Tunnel mode vs transport mode — full packet encapsulation vs payload-only protection
- NAT traversal and when it is needed
- Recommended and deprecated cryptographic algorithm suites
- Dead Peer Detection for HA failover
- Complete packet walkthrough through an IPSec tunnel

In [Part 2](/tutorials/network-security/configuring-ipsec-tunnels-strongswan-part-2/) you will deploy strongSwan on Linux and configure both a site-to-site tunnel and a road-warrior remote access VPN, with certificate-based authentication and PFS.
