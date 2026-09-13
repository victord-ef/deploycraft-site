---
title: "Firewall Deployment Architecture — Packet Filtering, NGFW, WAF, and DMZ Design — Part 1"
date: 2026-09-01
description: "Understand how firewalls work and how to design layered network security zones: packet filtering, stateful inspection, next-generation firewalls, web application firewalls, and DMZ architecture for separating public-facing services from internal networks."
cluster: "Network Security"
series: "Firewall"
part: 1
difficulty: "intermediate"
duration: "45 min"
tags: ["firewall", "network-security", "ngfw", "waf", "dmz", "network-architecture", "devsecops", "security-engineering"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

By the end of this tutorial you will understand how each firewall generation works technically, what each adds over the last, and how to design a multi-zone network with a DMZ that separates internet-facing services from internal resources. Part 2 implements defence-in-depth with layered firewall zones, east-west traffic controls, and policy auditing.

## Prerequisites

- TCP/IP fundamentals (IP headers, TCP state machine, ports, NAT)
- Basic networking — routing, subnets, VLANs

---

## Firewall generations

### Generation 1 — Packet filtering

The original firewall inspects individual packets and makes allow/deny decisions based on packet headers alone:
- Source IP address
- Destination IP address
- Source port
- Destination port
- Protocol (TCP, UDP, ICMP)

```
# iptables packet filter rules
iptables -A INPUT  -p tcp --dport 22 -s 10.0.0.0/8 -j ACCEPT
iptables -A INPUT  -p tcp --dport 22 -j DROP
iptables -A INPUT  -p tcp --dport 443 -j ACCEPT
iptables -A OUTPUT -p tcp --sport 443 -j ACCEPT
```

**Limitations:** Stateless — no awareness of connection state. An attacker can bypass a rule allowing established connections by sending packets with the ACK flag set (mimicking an already-established session). Cannot distinguish legitimate traffic from spoofed packets without inspecting full connection state.

### Generation 2 — Stateful inspection

Stateful firewalls track the state of each TCP connection and UDP pseudo-connection in a connection table. A rule that allows outbound connections automatically permits the return traffic — no explicit inbound rule needed.

```
Connection table entry:
  src: 10.0.1.5:54321
  dst: 8.8.8.8:53
  proto: UDP
  state: ESTABLISHED
  timeout: 30s
```

When the DNS response arrives from 8.8.8.8:53 to 10.0.1.5:54321, the firewall looks up the connection table, finds the matching entry, and permits it — even though no inbound rule for UDP/53 exists.

**What stateful inspection catches that packet filtering misses:**
- ACK-spoofing attacks (no matching connection entry → DROP)
- TCP state machine violations (SYN-FIN simultaneously → DROP)
- Port scanning using half-open connections

**Still cannot inspect:** Application-layer content. A connection to TCP/80 looks identical whether it's legitimate HTTP or a C2 beacon — both are "established" connections.

### Generation 3 — Application-layer gateway (proxy firewall)

A proxy firewall terminates the connection from the client and establishes a new connection to the server. It can inspect the full application-layer protocol:

```
Client → [Proxy firewall] → Server
        (terminates TCP)  (new TCP)
        (inspects HTTP)
```

Application proxies can:
- Block specific URLs or content
- Detect protocol violations (HTTP smuggling, malformed headers)
- Decrypt and inspect TLS (as a MITM)
- Apply content filtering

**Cost:** Full proxy termination adds latency. Does not scale to high-speed networks without dedicated hardware.

### Generation 4 — NGFW (Next-Generation Firewall)

NGFW integrates application identification, user identity awareness, IPS, TLS inspection, and threat intelligence into a single platform:

| Feature | Traditional firewall | NGFW |
|---|---|---|
| Application control | No — port-based only | Yes — identifies app regardless of port |
| User identity | No — IP-based | Yes — integrates with Active Directory |
| IPS | No | Yes — inline signature + anomaly detection |
| TLS inspection | No | Yes — man-in-the-middle with cert re-signing |
| Threat intelligence | No | Yes — IP/domain reputation feeds |
| URL filtering | No | Yes — categorised web filtering |
| File inspection | No | Yes — sandbox execution of downloaded files |

NGFW identifies application traffic using deep packet inspection — recognising Facebook, Dropbox, or Tor regardless of the port they use. Rules become: "Block Tor regardless of port" instead of "Block TCP/9001".

---

## Step 1 — Web Application Firewall (WAF)

A WAF is a specialised firewall that operates at Layer 7, specifically for HTTP/HTTPS traffic to web applications. It protects against OWASP Top 10 attacks:

| Attack | WAF protection |
|---|---|
| SQL injection | Detect and block `' OR 1=1--` patterns in request parameters |
| XSS | Strip `<script>` tags from user-supplied input |
| Path traversal | Block `../../../etc/passwd` in URL paths |
| HTTP smuggling | Normalise conflicting `Content-Length`/`Transfer-Encoding` |
| Large request bodies | Rate limit and size-check file uploads |
| Bot traffic | Challenge JavaScript execution (Proof of Work, CAPTCHA) |

### WAF deployment modes

**Inline (reverse proxy):**
```
Internet → WAF → Web server
```
All traffic passes through the WAF. Can block attacks. Adds latency. Single point of failure (mitigated with HA pair).

**Out-of-band (mirrored traffic):**
```
Internet → Web server
         ↘ WAF (copy)
```
WAF sees a copy — can alert but not block. Zero performance impact.

**Cloud WAF (CDN-integrated):**
Traffic routed through a cloud provider's WAF layer (Cloudflare, AWS WAF, Akamai). Absorbs DDoS at the edge. Origin server IP must be hidden from public DNS.

### ModSecurity OWASP core ruleset

```bash
# Install ModSecurity with nginx
apt-get install -y libmodsecurity3 nginx-module-security

# Download and configure OWASP Core Rule Set
git clone https://github.com/coreruleset/coreruleset /etc/nginx/modsecurity-crs
cp /etc/nginx/modsecurity-crs/crs-setup.conf.example /etc/nginx/modsecurity-crs/crs-setup.conf

# nginx.conf
modsecurity on;
modsecurity_rules_file /etc/nginx/modsecurity-crs/crs-setup.conf;
modsecurity_rules_file /etc/nginx/modsecurity-crs/rules/*.conf;
```

---

## Step 2 — DMZ architecture

A DMZ (Demilitarised Zone) is a network segment that sits between the internet and the internal network. It hosts services that must be accessible from the internet (web servers, mail servers, VPN gateways) while keeping them isolated from internal systems.

### Classic three-legged DMZ

```
                        ┌─────────────────────┐
                        │   Firewall          │
                        │   (three interfaces)│
                        └────┬──────┬─────────┘
                             │      │
              ┌──────────────┘      └──────────────┐
              │                                     │
   ┌──────────▼──────────┐             ┌────────────▼────────┐
   │        DMZ           │             │   Internal LAN      │
   │   (192.168.2.0/24)   │             │   (10.0.0.0/8)      │
   │                      │             │                     │
   │  ┌────────────────┐  │             │  ┌───────────────┐  │
   │  │ Web server     │  │             │  │ App server    │  │
   │  │ (public-facing)│  │             │  │ (private)     │  │
   │  └────────────────┘  │             │  └───────────────┘  │
   │  ┌────────────────┐  │             │  ┌───────────────┐  │
   │  │ Mail server    │  │             │  │ Database      │  │
   │  └────────────────┘  │             │  │ (private)     │  │
   │  ┌────────────────┐  │             │  └───────────────┘  │
   │  │ VPN gateway    │  │             └────────────────────┘
   │  └────────────────┘  │
   └──────────────────────┘
```

### Traffic flow rules

The firewall enforces these policies across the three zones:

```
Internet → DMZ:
  ALLOW TCP/443 to web server
  ALLOW TCP/25  to mail server
  ALLOW UDP/1194 to VPN gateway
  DROP all other traffic

DMZ → Internal:
  ALLOW TCP/8080 from web server to app server (application API)
  ALLOW TCP/25   from mail server to internal mail relay
  DENY all direct database access from DMZ
  DROP all other traffic

Internal → DMZ:
  ALLOW established connections (return traffic only)
  ALLOW TCP/22 from admin hosts to DMZ servers (management)
  DROP all other traffic

Internal → Internet:
  ALLOW HTTP/HTTPS via proxy
  DROP all other direct internet access

Internet → Internal (DENY):
  DROP all (no direct path — must go through DMZ or VPN)
```

### Key principle

**The DMZ web server should never initiate connections directly to the internal database.** The application server in the internal zone handles database queries. If the web server is compromised, the attacker has access to the DMZ only — they cannot directly reach the database.

```
Browser → [WAF] → Web server (DMZ) → App server (Internal) → Database (Internal)
                                          ↑
                             DMZ server connects TO internal
                             (not: internal accepts connections FROM DMZ)
```

---

## Step 3 — Firewall rule design principles

### Default deny

Start with an implicit deny-all. Every allowed flow is an explicit decision. Every undocumented flow is automatically blocked.

```
# iptables default deny
iptables -P INPUT DROP
iptables -P FORWARD DROP
iptables -P OUTPUT DROP
# Then add explicit ACCEPT rules for required traffic
```

### Least privilege

Rules should be as specific as possible:

```bash
# Bad — too broad
iptables -A FORWARD -s 192.168.2.0/24 -d 10.0.0.0/8 -j ACCEPT

# Good — specific source, destination, protocol, and port
iptables -A FORWARD \
  -s 192.168.2.5 \          # specific web server IP
  -d 10.0.1.10 \            # specific app server IP
  -p tcp --dport 8080 \     # specific application port
  -m state --state NEW,ESTABLISHED \
  -j ACCEPT
```

### Stateful by default

Always prefer stateful rules (using connection tracking) over stateless. Allow outbound connections and automatically permit their return traffic:

```bash
# Allow established/related traffic inbound
iptables -A INPUT -m state --state ESTABLISHED,RELATED -j ACCEPT
iptables -A FORWARD -m state --state ESTABLISHED,RELATED -j ACCEPT
```

### Log before drop

Log dropped traffic with rate limiting to avoid log flooding:

```bash
iptables -A INPUT -m limit --limit 10/min -j LOG --log-prefix "IPTABLES-DROP: " --log-level 6
iptables -A INPUT -j DROP
```

---

## Step 4 — NAT and port forwarding in the DMZ

The DMZ web server typically has a private IP. External traffic is forwarded from the firewall's public interface via DNAT (Destination NAT):

```bash
# Forward inbound HTTPS to the DMZ web server
iptables -t nat -A PREROUTING \
  -i eth0 \                         # public interface
  -p tcp --dport 443 \
  -j DNAT --to-destination 192.168.2.5:443

# Masquerade outbound traffic from DMZ (if DMZ hosts need internet access)
iptables -t nat -A POSTROUTING \
  -s 192.168.2.0/24 \
  -o eth0 \
  -j MASQUERADE
```

### Source NAT for internal outbound

All internal traffic leaving via the firewall appears to originate from the firewall's public IP:

```bash
iptables -t nat -A POSTROUTING \
  -s 10.0.0.0/8 \
  -o eth0 \                         # public-facing interface
  -j MASQUERADE
```

This hides the internal addressing scheme from external observers.

---

## Step 5 — High availability firewall pair

A single firewall is a single point of failure. Production deployments use an active-passive HA pair:

```
               ┌─────────────────────────┐
               │   Active Firewall       │ ◀── All traffic
               │   (processes all flows) │
               └─────────────┬───────────┘
                             │ State sync (connection table, VRRP)
               ┌─────────────▼───────────┐
               │   Passive Firewall      │ ◀── Standby (takes over on failure)
               │   (mirrors state)       │
               └─────────────────────────┘
```

**VRRP (Virtual Router Redundancy Protocol):** The pair shares a virtual IP address. The active firewall announces it via VRRP. On failure, the passive firewall takes over the virtual IP within 1–3 seconds — existing sessions are preserved because the connection state was replicated.

**Session synchronisation:** The active firewall replicates its connection table to the passive in real time. When failover occurs, existing TCP sessions continue without interruption from the client's perspective.

---

## Step 6 — Firewall zones in cloud environments

Cloud environments require explicit security groups / VPC firewall rules in place of physical firewalls:

### AWS Security Groups + NACLs

```hcl
# Terraform: three-zone architecture on AWS

# DMZ subnet — public-facing
resource "aws_security_group" "dmz" {
  vpc_id = aws_vpc.main.id

  ingress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port       = 8080
    to_port         = 8080
    protocol        = "tcp"
    security_groups = [aws_security_group.app.id]    # only to app tier
  }
}

# App tier — internal, reachable only from DMZ
resource "aws_security_group" "app" {
  vpc_id = aws_vpc.main.id

  ingress {
    from_port       = 8080
    to_port         = 8080
    protocol        = "tcp"
    security_groups = [aws_security_group.dmz.id]    # only from DMZ
  }

  egress {
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [aws_security_group.db.id]    # only to DB
  }
}

# Database tier — reachable only from app tier
resource "aws_security_group" "db" {
  vpc_id = aws_vpc.main.id

  ingress {
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [aws_security_group.app.id]
  }
}
```

---

## What you have built

- Packet filtering, stateful inspection, application-layer gateway, and NGFW — how each generation works and what it adds
- WAF operation and OWASP Core Rule Set deployment with ModSecurity
- DMZ three-legged architecture — traffic flows and the principle that DMZ servers initiate connections inward, not the reverse
- Firewall rule design — default deny, least privilege, stateful, log-before-drop
- NAT and DNAT for port forwarding and internal traffic masquerading
- HA active-passive firewall pair with VRRP and session synchronisation
- Cloud firewall equivalents — AWS Security Groups and NACLs in a three-tier Terraform model

In [Part 2](/tutorials/network-security/defence-in-depth-firewall-zones-east-west-part-2/) you will implement defence-in-depth: build the full layered firewall rule set, control east-west (internal) traffic between application tiers, and audit the firewall policy for gaps.
