---
title: "VPN Architecture — Site-to-Site, Remote Access, and Split Tunneling — Part 1"
date: 2026-09-01
description: "Understand the core VPN design patterns: site-to-site connectivity, remote access, split vs full tunneling, hub-and-spoke vs mesh topologies, and the tradeoffs between SSL/TLS VPN and IPSec VPN for each use case."
cluster: "Network Security"
series: "VPN"
part: 1
difficulty: "intermediate"
duration: "40 min"
tags: ["vpn", "network-security", "wireguard", "openvpn", "ipsec", "networking", "devsecops"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

By the end of this tutorial you will understand how VPN architectures are designed for different operational needs — connecting offices together, enabling remote workers, and securing cloud connectivity. Part 2 deploys WireGuard and OpenVPN on Linux with certificate-based authentication and hardened configurations.

## Prerequisites

- Familiarity with TCP/IP — routing, subnets, NAT
- Basic Linux networking knowledge

---

## What a VPN does and what it does not

A VPN creates an encrypted tunnel between two endpoints. Traffic inside the tunnel is confidential from observers on the path between them. A VPN does **not**:

- Make a compromised endpoint safe
- Encrypt traffic beyond the VPN endpoints
- Provide anonymity if the VPN provider logs traffic
- Substitute for application-layer security (HTTPS, mTLS)

The value of a VPN is the encrypted channel between specific endpoints — typically to extend a trusted network across an untrusted one (the public internet).

---

## Step 1 — Site-to-site VPN

A site-to-site VPN connects two or more private networks across the public internet. Once established, hosts on each network can communicate with hosts on the other as if they were on the same LAN.

```
  Site A (10.0.1.0/24)          Site B (10.0.2.0/24)
  ┌──────────────────────┐      ┌──────────────────────┐
  │  Hosts               │      │  Hosts               │
  │  10.0.1.10           │      │  10.0.2.10           │
  │  10.0.1.11           │      │  10.0.2.11           │
  └──────────┬───────────┘      └───────────┬──────────┘
             │                              │
  ┌──────────▼───────────┐      ┌───────────▼──────────┐
  │  VPN Gateway A       │======│  VPN Gateway B       │
  │  Public: 203.0.113.1 │ VPN  │  Public: 198.51.100.1│
  └──────────────────────┘ tunnel└──────────────────────┘
```

**Key properties:**
- The VPN tunnel is between the gateway devices, not individual hosts
- Hosts on each site are unaware of the VPN — they route to the other site's subnet via their default gateway
- The gateways handle encryption, decryption, and routing

**Use cases:** Office-to-office connectivity, connecting on-premises infrastructure to cloud VPCs, branch office connectivity.

**Common implementations:** IPSec (industry standard for site-to-site, hardware router support), WireGuard (modern, simple, lower overhead), OpenVPN (flexible, widely supported).

---

## Step 2 — Remote access VPN

A remote access VPN connects individual clients to a central gateway. The gateway acts as a concentrator — many clients, one gateway.

```
  Remote Worker            Corporate Gateway
  ┌──────────────┐         ┌──────────────────────────────┐
  │ Laptop       │=========│ VPN Concentrator             │
  │ 192.168.1.5  │  tunnel │ Assigns: 10.8.0.2 to client  │
  └──────────────┘         │                              │
                           │   Internal network:          │
  Remote Worker            │   10.0.0.0/8                 │
  ┌──────────────┐         │                              │
  │ Phone        │=========│ Internal services:           │
  │ 192.168.2.10 │  tunnel │   - Active Directory         │
  └──────────────┘         │   - File shares              │
                           │   - Internal apps            │
                           └──────────────────────────────┘
```

**Key properties:**
- Each client receives a virtual IP from a pool managed by the gateway
- The client's traffic to internal resources routes through the tunnel
- Gateway handles authentication (certificates, MFA, LDAP/RADIUS)

**Use cases:** Work-from-home access to corporate resources, contractor access to specific internal services, developer access to cloud infrastructure.

---

## Step 3 — Split tunneling vs full tunneling

How a VPN client routes traffic is the most consequential design decision for remote access.

### Full tunnel

All client traffic — including general internet browsing — routes through the VPN gateway.

```
Client ──(all traffic)──▶ VPN Gateway ──▶ Internet
                                     └──▶ Internal resources
```

**Advantages:**
- Corporate security controls (proxies, DLP, DNS filtering) apply to all client traffic
- Prevents split DNS attacks where a malicious local DNS server directs corporate hostnames to attacker IPs
- Simpler to reason about — all traffic is inspected

**Disadvantages:**
- Gateway becomes a bandwidth bottleneck for internet traffic
- Higher latency for internet browsing (all traffic hairpins through corporate network)
- Gateway must handle total client internet bandwidth, not just internal traffic

### Split tunnel

Only traffic destined for internal resources routes through the VPN. Internet traffic exits the client's local network directly.

```
Client ──(10.0.0.0/8)──▶ VPN Gateway ──▶ Internal resources
      └──(everything else)──▶ Local ISP ──▶ Internet
```

**Advantages:**
- Internet traffic is not bottlenecked at the gateway
- Lower gateway capacity requirement — only internal traffic traverses the tunnel
- Better user experience for internet browsing

**Disadvantages:**
- Internet traffic is not inspected by corporate controls — endpoint compromise can exfiltrate data directly
- DNS split-brain: client must resolve internal hostnames via VPN DNS server but public hostnames via local DNS
- More complex routing configuration

**Split tunnel configuration in OpenVPN:**

```
# Include only these routes through the VPN
route 10.0.0.0 255.0.0.0      # corporate network
route 172.16.0.0 255.240.0.0  # RFC 1918
# Everything else uses local routing table
```

**Full tunnel in OpenVPN:**

```
# Push default route through VPN (overrides all routing)
push "redirect-gateway def1 bypass-dhcp"
```

### Inverse split tunnel

A hybrid: all traffic routes through the VPN except specific destinations (video conferencing, SaaS tools). Used when full tunnel is required by policy but specific high-bandwidth services need local exit.

---

## Step 4 — VPN topologies

### Hub-and-spoke

All sites connect to a central hub. Traffic between spokes travels through the hub.

```
        ┌─────────────┐
        │  Hub (HQ)   │
        └──┬───┬───┬──┘
           │   │   │
   ┌───────┘   │   └───────┐
   │           │           │
┌──▼───┐    ┌──▼───┐    ┌──▼───┐
│Spoke1│    │Spoke2│    │Spoke3│
│ (NY) │    │ (LA) │    │ (LN) │
└──────┘    └──────┘    └──────┘
```

**Advantages:** Simple to manage. All routing decisions at the hub. Security controls (firewall, IDS) applied centrally at the hub.

**Disadvantages:** Hub is a single point of failure. Spoke-to-spoke traffic takes two hops (spoke → hub → spoke), adding latency. Hub bandwidth must accommodate all inter-spoke traffic.

**Best for:** Classic enterprise with strong central control, compliance requirements for traffic inspection at a single point.

### Full mesh

Every site has a direct tunnel to every other site.

```
┌──────┐──────────┌──────┐
│Site A│          │Site B│
└──┬───┘          └──┬───┘
   │  ╲          ╱   │
   │    ╲      ╱     │
   │      ╲  ╱       │
   │        ╳        │
   │      ╱  ╲       │
   │    ╱      ╲     │
┌──▼───┐          ┌──▼───┐
│Site C│──────────│Site D│
└──────┘          └──────┘
```

**Advantages:** Optimal latency — direct paths between all sites. No hub bottleneck or single point of failure.

**Disadvantages:** Number of tunnels grows as n(n-1)/2 — 4 sites needs 6 tunnels, 10 sites needs 45. Management complexity grows quadratically. WireGuard handles this well with its simple peer configuration.

**Best for:** Branch office networks where inter-branch latency matters. Cloud multi-region connectivity.

### Dynamic mesh (SD-WAN / DSVPN)

A dynamic routing protocol builds and tears down tunnels on demand. Hubs maintain static tunnels; spoke-to-spoke tunnels are created dynamically when needed and torn down when idle.

**Best for:** Large enterprises with many branch offices. Cloud-native SD-WAN solutions (Cisco SD-WAN, Cloudflare WARP, AWS Transit Gateway).

---

## Step 5 — SSL/TLS VPN vs IPSec VPN

The two dominant protocol families have different tradeoffs:

| Property | SSL/TLS VPN (OpenVPN, WireGuard) | IPSec VPN |
|---|---|---|
| Transport | UDP or TCP (user space) | ESP protocol (Layer 3, kernel) |
| Firewall traversal | Easy — uses standard HTTPS/UDP ports | Harder — ESP may be blocked; needs NAT-T |
| Client software | Requires client app | Built into OS (Windows, macOS, Linux, iOS, Android) |
| Performance | Good; WireGuard approaches native | Excellent — kernel implementation, hardware offload |
| Configuration complexity | Low (OpenVPN config file or WireGuard peer keys) | High (IKE policy, SA proposals, transform sets) |
| Hardware router support | Limited | Universal — every enterprise router speaks IPSec |
| Use case fit | Remote access, cloud VPN, container workloads | Site-to-site, router-to-router, carrier-grade |

**Choose SSL/TLS VPN when:**
- Remote access for end users (OpenVPN/WireGuard clients are easy to deploy)
- Environments with restrictive firewalls (UDP/443 almost always passes)
- Cloud-native infrastructure (WireGuard is built into the Linux kernel since 5.6)

**Choose IPSec when:**
- Site-to-site between hardware routers (Cisco, Juniper, Palo Alto all speak IKEv2)
- Compliance requirements specify IPSec (FIPS, some government frameworks)
- High-throughput links with hardware encryption offload

---

## Step 6 — Authentication models

### Pre-shared key (PSK)

Both sides share a secret key configured statically. Simple but does not scale — rotating the key requires touching every peer.

```
peer A: psk = "s3cr3t"
peer B: psk = "s3cr3t"
```

**Use only for:** Lab environments, small static site-to-site with two peers.

### Certificate-based (PKI)

Each peer has a certificate signed by a shared Certificate Authority. Authentication is mutual — both sides verify each other's certificate.

```
CA (root)
├── Server cert (gateway)
├── Client cert (user-1)
├── Client cert (user-2)
└── Client cert (site-b-gateway)
```

**Revocation:** Compromised clients are removed by revoking their certificate (CRL or OCSP). No key rotation required for other peers.

**Use for:** Production remote access VPNs, site-to-site where scaling matters.

### MFA + certificates

Certificate proves device identity; MFA (TOTP, push notification) proves user presence. Defense against stolen client certificates.

```
Client presents cert → gateway verifies cert is valid → prompts for TOTP
                                                    → user enters code
                                                    → session established
```

---

## Step 7 — VPN in cloud environments

Cloud providers offer managed VPN services that integrate with their routing infrastructure:

**AWS:**
- Site-to-site VPN: connects on-premises to VPC via IPSec, 1.25 Gbps per tunnel
- Client VPN: OpenVPN-compatible remote access to VPC
- Transit Gateway: hub for multiple VPCs and on-premises connections

**Azure:**
- VPN Gateway: IPSec site-to-site and point-to-site (OpenVPN or SSTP)
- ExpressRoute: dedicated private circuit, not a VPN

**GCP:**
- Cloud VPN: IPSec with dynamic routing (BGP)
- Cloud Interconnect: dedicated connectivity

**Self-managed WireGuard on cloud VMs:**
A single WireGuard instance on a cloud VM costs ~$5/month and gives you full control over routing, authentication, and logging. Many organisations prefer this over managed VPN services for simplicity and auditability.

---

## What you have built

- The operational distinction between IDS and IPS — when each is appropriate
- Site-to-site vs remote access VPN — which topology fits which use case
- Split tunnel vs full tunnel — the security and performance tradeoffs
- Hub-and-spoke, full mesh, and dynamic mesh topologies
- SSL/TLS VPN vs IPSec VPN — protocol selection criteria
- Authentication models — PSK, certificate-based, MFA-augmented
- Cloud VPN integration patterns for AWS, Azure, and GCP

In [Part 2](/tutorials/network-security/deploying-wireguard-openvpn-hardening-part-2/) you will deploy both WireGuard and OpenVPN on Linux: generate a CA and client certificates, configure server and client configs, verify tunnel connectivity, and apply production hardening.
