---
title: "Defence-in-Depth with Layered Firewall Zones and East-West Traffic Controls — Part 2"
date: 2026-09-01
description: "Implement defence-in-depth: build a layered firewall rule set for perimeter, DMZ, and internal zones, control east-west traffic between application tiers with micro-segmentation, and audit firewall policies for gaps and overly permissive rules."
cluster: "Network Security"
series: "Firewall"
part: 2
difficulty: "advanced"
duration: "50 min"
tags: ["firewall", "network-security", "defence-in-depth", "micro-segmentation", "iptables", "nftables", "devsecops"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

In [Part 1](/tutorials/network-security/firewall-deployment-architecture-ngfw-dmz-part-1/) you learned firewall generations and DMZ design. In Part 2 you will implement a complete layered defence using nftables on Linux: perimeter rules, DMZ-to-internal controls, east-west micro-segmentation between application tiers, management plane isolation, and a policy audit script.

## Prerequisites

- Completed [Part 1](/tutorials/network-security/firewall-deployment-architecture-ngfw-dmz-part-1/)
- A Linux host (Ubuntu 22.04) acting as the firewall/gateway
- Three network interfaces: `eth0` (internet), `eth1` (DMZ), `eth2` (internal)
- `sudo` access

---

## Why nftables over iptables

This tutorial uses nftables — the modern Linux packet filtering framework that replaced iptables in the Linux 3.13 kernel. nftables advantages:

- Single tool for IPv4, IPv6, ARP, and bridge filtering
- Atomic rule updates (entire ruleset applied in one transaction)
- Better performance (maps/sets avoid long linear rule chains)
- Cleaner syntax — all rules in a single file

iptables rules still work (via the `iptables-nft` compatibility layer), but nftables should be used for new configurations.

---

## Step 1 — Interface and network design

```
eth0: 203.0.113.1/24  (internet-facing, untrusted)
eth1: 192.168.2.1/24  (DMZ — web/mail/VPN servers)
eth2: 10.0.0.1/8      (internal LAN — app servers, databases, workstations)

DMZ hosts:
  192.168.2.5  — web server (nginx + WAF)
  192.168.2.6  — mail relay
  192.168.2.7  — VPN gateway

Internal tiers:
  10.0.1.0/24  — application servers
  10.0.2.0/24  — database servers
  10.0.3.0/24  — workstations
  10.0.10.0/24 — management (jump hosts, monitoring)
```

---

## Step 2 — Complete nftables ruleset

```nft
#!/usr/sbin/nft -f
# /etc/nftables/firewall.nft — complete firewall configuration

flush ruleset

define INT_NET  = 10.0.0.0/8
define DMZ_NET  = 192.168.2.0/24
define APP_NET  = 10.0.1.0/24
define DB_NET   = 10.0.2.0/24
define WKS_NET  = 10.0.3.0/24
define MGMT_NET = 10.0.10.0/24

define WEB_SRV  = 192.168.2.5
define MAIL_SRV = 192.168.2.6
define VPN_GW   = 192.168.2.7

# ─────────────────────────────────────────────────────────────────────────────
# NAT table
# ─────────────────────────────────────────────────────────────────────────────
table ip nat {

  chain prerouting {
    type nat hook prerouting priority dstnat; policy accept;

    # DNAT: forward inbound HTTPS to DMZ web server
    iif "eth0" tcp dport 443 dnat to $WEB_SRV:443

    # DNAT: forward inbound SMTP to DMZ mail relay
    iif "eth0" tcp dport 25  dnat to $MAIL_SRV:25

    # DNAT: forward inbound OpenVPN to VPN gateway
    iif "eth0" udp dport 1194 dnat to $VPN_GW:1194
  }

  chain postrouting {
    type nat hook postrouting priority srcnat; policy accept;

    # Masquerade internal and DMZ traffic leaving on internet interface
    oif "eth0" masquerade
  }
}

# ─────────────────────────────────────────────────────────────────────────────
# Filter table
# ─────────────────────────────────────────────────────────────────────────────
table ip filter {

  # Reusable sets (O(log n) lookup — faster than long rule chains)
  set allowed_mgmt_hosts {
    type ipv4_addr
    elements = { 10.0.10.5, 10.0.10.6 }    # jump host IPs
  }

  # ─── INPUT chain (traffic destined for the firewall itself) ──────────────
  chain input {
    type filter hook input priority filter; policy drop;

    # Allow loopback
    iif lo accept

    # Allow established/related return traffic
    ct state established,related accept

    # Drop invalid connection states
    ct state invalid drop

    # Allow ICMP (ping) — rate limit to prevent ICMP flood
    ip protocol icmp limit rate 10/second accept
    ip protocol icmp drop

    # Allow SSH from management hosts only
    iif "eth2" ip saddr @allowed_mgmt_hosts tcp dport 22 ct state new accept

    # Log and drop everything else to the firewall
    limit rate 20/minute log prefix "FW-INPUT-DROP: " level warn
    drop
  }

  # ─── FORWARD chain (routed traffic through the firewall) ─────────────────
  chain forward {
    type filter hook forward priority filter; policy drop;

    # Allow established/related return traffic (stateful)
    ct state established,related accept

    # Drop invalid
    ct state invalid drop

    # ── Internet → DMZ (inbound to public services) ──────────────────────
    iif "eth0" oif "eth1" ip daddr $WEB_SRV  tcp dport 443  ct state new accept
    iif "eth0" oif "eth1" ip daddr $MAIL_SRV tcp dport 25   ct state new accept
    iif "eth0" oif "eth1" ip daddr $VPN_GW   udp dport 1194 ct state new accept

    # ── DMZ → Internal (DMZ servers connecting to app tier only) ─────────
    # Web server → app servers (application API)
    iif "eth1" oif "eth2" ip saddr $WEB_SRV  ip daddr $APP_NET tcp dport 8080 ct state new accept

    # Mail relay → internal mail server
    iif "eth1" oif "eth2" ip saddr $MAIL_SRV ip daddr 10.0.1.20 tcp dport 25 ct state new accept

    # VPN clients → internal network (via VPN gateway)
    iif "eth1" oif "eth2" ip saddr $VPN_GW ct state new accept

    # DENY DMZ → database tier directly (must go via app tier)
    iif "eth1" oif "eth2" ip daddr $DB_NET drop

    # ── Internal → DMZ (management access only) ──────────────────────────
    iif "eth2" oif "eth1" ip saddr @allowed_mgmt_hosts tcp dport 22 ct state new accept

    # ── Internal → Internet (via proxy — not direct) ──────────────────────
    # All workstation internet access must go via proxy on 10.0.10.100
    iif "eth2" oif "eth0" ip saddr $WKS_NET ip daddr != 10.0.10.100 drop
    iif "eth2" oif "eth0" ip saddr $WKS_NET ip daddr 10.0.10.100 tcp dport 3128 ct state new accept

    # App servers may reach internet for package updates (on specific IPs)
    # Add specific IPs as needed rather than allowing all internet

    # ── East-West: Internal tier-to-tier controls ─────────────────────────
    # Workstations → app servers (users accessing internal apps)
    iif "eth2" oif "eth2" ip saddr $WKS_NET ip daddr $APP_NET tcp dport { 80, 443, 8080 } ct state new accept

    # App servers → database (only from app tier, only on DB port)
    iif "eth2" oif "eth2" ip saddr $APP_NET ip daddr $DB_NET tcp dport 5432 ct state new accept

    # Workstations → databases (DENY — apps mediate all DB access)
    iif "eth2" oif "eth2" ip saddr $WKS_NET ip daddr $DB_NET drop

    # Management → everything (monitoring, SSH, SNMP)
    iif "eth2" oif "eth2" ip saddr $MGMT_NET accept
    iif "eth2" oif "eth1" ip saddr $MGMT_NET accept

    # ── Log all drops ─────────────────────────────────────────────────────
    limit rate 20/minute log prefix "FW-FORWARD-DROP: " level warn
    drop
  }

  # ─── OUTPUT chain (traffic originating from the firewall) ────────────────
  chain output {
    type filter hook output priority filter; policy accept;
    # Allow all outbound from firewall — restrict as needed
  }
}
```

Apply the ruleset atomically:

```bash
# Validate syntax
nft -c -f /etc/nftables/firewall.nft

# Apply (atomic — either all rules apply or none)
nft -f /etc/nftables/firewall.nft

# Verify
nft list ruleset

# Enable at boot
systemctl enable nftables
```

---

## Step 3 — East-west micro-segmentation

The rules above enforce that application servers access databases but workstations do not. This east-west control limits lateral movement — an attacker who compromises a workstation cannot pivot directly to databases.

Micro-segmentation extends this to service-level granularity. Implement it using nftables sets to group servers by role:

```nft
table ip microseg {

  set app_servers {
    type ipv4_addr
    elements = { 10.0.1.10, 10.0.1.11, 10.0.1.12 }
  }

  set db_servers {
    type ipv4_addr
    elements = { 10.0.2.10, 10.0.2.11 }
  }

  set auth_servers {
    type ipv4_addr
    elements = { 10.0.1.50 }    # LDAP/AD server
  }

  chain forward {
    type filter hook forward priority filter + 10; policy accept;

    # App servers → specific DB instances only
    iif "eth2" oif "eth2" \
      ip saddr @app_servers ip daddr @db_servers \
      tcp dport 5432 ct state new accept

    # Workstations → auth server (LDAP/Kerberos)
    iif "eth2" oif "eth2" \
      ip saddr $WKS_NET ip daddr @auth_servers \
      tcp dport { 389, 636, 88 } ct state new accept

    # DENY any workstation → app server on management port
    iif "eth2" oif "eth2" \
      ip saddr $WKS_NET ip daddr @app_servers \
      tcp dport 22 drop

    # Management only → SSH to app and DB servers
    iif "eth2" oif "eth2" \
      ip saddr $MGMT_NET ip daddr @app_servers \
      tcp dport 22 ct state new accept

    iif "eth2" oif "eth2" \
      ip saddr $MGMT_NET ip daddr @db_servers \
      tcp dport 22 ct state new accept
  }
}
```

---

## Step 4 — Management plane isolation

The management plane (SSH, SNMP, monitoring agents) must be isolated from the data plane. Compromise of a web server should not provide SSH access to other systems.

```nft
# Dedicated management VLAN (eth2.10) for all SSH/management traffic
# Only management hosts can SSH to servers — enforced at the firewall

table ip mgmt_isolation {

  set mgmt_sources {
    type ipv4_addr
    elements = { 10.0.10.5, 10.0.10.6 }    # jump hosts
  }

  chain forward {
    type filter hook forward priority filter + 20; policy accept;

    # SSH only from management sources
    tcp dport 22 ip saddr != @mgmt_sources drop

    # SNMP only from monitoring server
    udp dport 161 ip saddr != 10.0.10.20 drop

    # Prometheus scraping only from monitoring server
    tcp dport 9090 ip saddr != 10.0.10.20 drop
  }
}
```

---

## Step 5 — Connection rate limiting (DDoS mitigation)

Protect public-facing services from SYN flood and connection exhaustion:

```nft
table ip ratelimit {

  chain prerouting {
    type filter hook prerouting priority raw; policy accept;

    # SYN flood protection — limit new TCP connections per source
    tcp flags & (fin|syn|rst|ack) == syn \
      limit rate over 100/second burst 200 packets \
      log prefix "SYN-FLOOD: " drop

    # ICMP flood protection
    ip protocol icmp limit rate over 20/second burst 50 packets drop

    # Port scan detection — new connections to many ports from single source
    # (Requires connlimit or recent module — see iptables-nft extension)
  }

  chain input {
    type filter hook input priority filter - 5; policy accept;

    # Limit SSH connection attempts per source IP
    iif "eth0" tcp dport 22 ct state new \
      add @ssh_candidates { ip saddr timeout 60s }
    iif "eth0" tcp dport 22 ct state new \
      ip saddr @ssh_candidates limit rate over 5/minute \
      log prefix "SSH-BRUTE: " drop
  }

  set ssh_candidates {
    type ipv4_addr
    flags dynamic,timeout
  }
}
```

---

## Step 6 — Firewall policy audit script

Regular audits detect policy drift — rules added for temporary access that were never removed, overly broad rules, and missing deny-log entries.

```bash
#!/bin/bash
# firewall-audit.sh

echo "=== Firewall Policy Audit ==="
echo "Date: $(date)"
echo ""

# 1. Verify default policy is DROP on all chains
echo "--- Default policies ---"
nft list ruleset | grep "policy" | while read line; do
    if echo "$line" | grep -q "accept"; then
        echo "WARNING: Non-DROP default policy: $line"
    else
        echo "OK: $line"
    fi
done

# 2. Find overly broad ACCEPT rules (any source or any destination)
echo ""
echo "--- Broad ACCEPT rules (review these) ---"
nft list ruleset | grep -E "accept" | grep -v "established|related|loopback|@" \
  | grep -v "ip saddr\|ip daddr" \
  | grep -v "#"

# 3. Check for rules without state tracking (stateless ACCEPT)
echo ""
echo "--- Rules accepting new connections without state tracking ---"
nft list ruleset | grep "accept" | grep "ct state new" | grep -v "established"

# 4. Verify log-before-drop rules exist
echo ""
echo "--- Checking for log-before-drop ---"
if nft list ruleset | grep -q "log prefix.*FW-FORWARD-DROP"; then
    echo "OK: Forward drop logging present"
else
    echo "MISSING: No forward chain drop logging"
fi

# 5. Count rules per chain (rule sprawl indicator)
echo ""
echo "--- Rule count per chain ---"
nft list ruleset | grep "^    [a-z]" | awk '{print $1}' | sort | uniq -c | sort -rn

# 6. Check for unused sets (defined but not referenced in rules)
echo ""
echo "--- Sets defined (verify all are referenced in rules) ---"
nft list ruleset | grep "^  set "

# 7. Test connectivity matrix against policy
echo ""
echo "--- Connectivity test (expected: BLOCKED) ---"
# Test that workstations cannot reach databases directly
if nc -z -w2 10.0.2.10 5432 2>/dev/null; then
    echo "FAIL: Workstation → database TCP/5432 is OPEN (should be blocked)"
else
    echo "OK: Workstation → database TCP/5432 is blocked"
fi

echo ""
echo "Audit complete."
```

```bash
chmod +x firewall-audit.sh
sudo ./firewall-audit.sh
```

---

## Step 7 — Logging and monitoring integration

Forward firewall logs to your SIEM or log aggregator:

```bash
# Configure rsyslog to forward kernel firewall logs
cat >> /etc/rsyslog.conf << 'EOF'
# Forward kernel/firewall messages to SIEM
if $syslogfacility-text == 'kern' and $msg contains 'FW-' then {
    action(type="omfwd" target="siem.internal" port="514" protocol="udp")
}
EOF

systemctl restart rsyslog
```

Create a Prometheus alert for sudden spikes in dropped packets (potential attack):

```yaml
# prometheus-alerts-firewall.yaml
groups:
  - name: firewall
    rules:
      - alert: HighDropRate
        expr: |
          rate(node_nf_conntrack_entries_limit[5m]) > 0.9
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "Connection table near capacity — possible SYN flood"

      - alert: SYNFloodDetected
        expr: |
          increase(node_netstat_Tcp_InSegs[1m]) > 50000
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Possible SYN flood attack — high inbound TCP segment rate"
```

---

## What you have built

- Complete nftables ruleset for a three-interface gateway (internet / DMZ / internal) with named zones and reusable sets
- DNAT port forwarding for web, mail, and VPN services into the DMZ
- DMZ-to-internal rules enforcing that DMZ servers cannot reach the database tier directly
- East-west micro-segmentation preventing workstations from directly accessing databases
- Management plane isolation restricting SSH and monitoring to designated management hosts
- SYN flood and connection rate limiting with per-source tracking
- A firewall policy audit script detecting overly broad rules, missing logging, and connectivity violations
- Syslog forwarding to SIEM and Prometheus alerting for traffic anomalies
