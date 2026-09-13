---
title: "Configuring IPSec Site-to-Site and Road-Warrior Tunnels with strongSwan — Part 2"
date: 2026-09-01
description: "Deploy strongSwan on Linux to build an IPSec site-to-site tunnel and a road-warrior remote access VPN. Configure IKEv2 with certificate authentication, PFS, and NAT traversal, and verify SA negotiation."
cluster: "Network Security"
series: "IPSec"
part: 2
difficulty: "advanced"
duration: "55 min"
tags: ["ipsec", "strongswan", "vpn", "network-security", "ikev2", "pki", "networking", "devsecops"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

In [Part 1](/tutorials/network-security/ipsec-fundamentals-tunnel-transport-mode-part-1/) you learned how IPSec works: IKEv2 phases, Security Associations, and tunnel vs transport mode. In Part 2 you will deploy strongSwan on Linux and configure two real scenarios — a site-to-site tunnel connecting two networks, and a road-warrior VPN for remote clients using EAP-TLS certificate authentication.

## Prerequisites

- Completed [Part 1](/tutorials/network-security/ipsec-fundamentals-tunnel-transport-mode-part-1/)
- Two Linux hosts (Ubuntu 22.04) — one per site for the site-to-site scenario
- A third client machine (Linux, macOS, or Windows) for the road-warrior scenario
- Public IPs or routable addresses on the VPN gateways
- `sudo` access

---

## Step 1 — Install strongSwan

```bash
# On all gateway hosts
sudo apt-get update
sudo apt-get install -y strongswan strongswan-pki libcharon-extra-plugins libcharon-extauth-plugins

# Verify version
ipsec version
# Linux strongSwan 5.x.x
```

strongSwan uses two configuration locations:
- `/etc/ipsec.conf` — legacy format (still supported)
- `/etc/swanctl/swanctl.conf` — modern VICI-based format (recommended)

This tutorial uses `swanctl.conf` (the modern format).

---

## Part A — Site-to-Site Tunnel

### Step 2 — Build a PKI for certificate authentication

```bash
# Create a CA on a secure host (not the gateway)
mkdir -p ~/pki/{ca,certs,keys}
chmod 700 ~/pki

# Generate CA private key
ipsec pki --gen --type ecdsa --size 384 \
  --outform pem > ~/pki/ca/ca-key.pem

# Self-sign the CA certificate (10-year validity)
ipsec pki --self --ca --lifetime 3650 \
  --in ~/pki/ca/ca-key.pem \
  --type ecdsa \
  --dn "CN=IPSec CA, O=DeployCraft, C=GB" \
  --outform pem > ~/pki/ca/ca-cert.pem

# Generate server key and certificate for Gateway A
ipsec pki --gen --type ecdsa --size 384 \
  --outform pem > ~/pki/keys/gw-a.pem

ipsec pki --pub --in ~/pki/keys/gw-a.pem --type ecdsa \
  | ipsec pki --issue --lifetime 730 \
  --cacert ~/pki/ca/ca-cert.pem \
  --cakey ~/pki/ca/ca-key.pem \
  --dn "CN=gw-a.example.com, O=DeployCraft, C=GB" \
  --san "gw-a.example.com" \
  --san "203.0.113.1" \
  --flag serverAuth \
  --outform pem > ~/pki/certs/gw-a-cert.pem

# Repeat for Gateway B
ipsec pki --gen --type ecdsa --size 384 \
  --outform pem > ~/pki/keys/gw-b.pem

ipsec pki --pub --in ~/pki/keys/gw-b.pem --type ecdsa \
  | ipsec pki --issue --lifetime 730 \
  --cacert ~/pki/ca/ca-cert.pem \
  --cakey ~/pki/ca/ca-key.pem \
  --dn "CN=gw-b.example.com, O=DeployCraft, C=GB" \
  --san "gw-b.example.com" \
  --san "198.51.100.1" \
  --flag serverAuth \
  --outform pem > ~/pki/certs/gw-b-cert.pem
```

Distribute to each gateway:

```bash
# On Gateway A
sudo cp ~/pki/ca/ca-cert.pem /etc/swanctl/x509ca/
sudo cp ~/pki/certs/gw-a-cert.pem /etc/swanctl/x509/
sudo cp ~/pki/keys/gw-a.pem /etc/swanctl/private/
sudo chmod 600 /etc/swanctl/private/gw-a.pem

# On Gateway B (scp from CA host)
sudo cp ca-cert.pem /etc/swanctl/x509ca/
sudo cp gw-b-cert.pem /etc/swanctl/x509/
sudo cp gw-b.pem /etc/swanctl/private/
sudo chmod 600 /etc/swanctl/private/gw-b.pem
```

### Step 3 — Configure Gateway A

```hcl
# /etc/swanctl/swanctl.conf — Gateway A (203.0.113.1)

connections {
  site-b {
    # IKE (control plane) configuration
    version = 2
    local_addrs  = 203.0.113.1
    remote_addrs = 198.51.100.1

    # IKE SA algorithm proposals
    proposals = aes256gcm16-prfsha384-ecp384

    # Rekey IKE SA every 4 hours
    rekey_time = 14400s
    dpd_delay  = 30s
    dpd_timeout = 90s

    local {
      auth = pubkey
      certs = gw-a-cert.pem
      id = "CN=gw-a.example.com, O=DeployCraft, C=GB"
    }

    remote {
      auth = pubkey
      cacerts = ca-cert.pem
      id = "CN=gw-b.example.com, O=DeployCraft, C=GB"
    }

    children {
      # Child SA (data plane) — defines what traffic to protect
      net-b {
        local_ts  = 10.0.1.0/24   # Site A subnet
        remote_ts = 10.0.2.0/24   # Site B subnet
        mode      = tunnel

        # ESP (data) algorithm proposals — AES-256-GCM with PFS
        esp_proposals = aes256gcm16-ecp384

        # Rekey Child SA every 1 hour with fresh DH exchange (PFS)
        rekey_time = 3600s
        life_time  = 3900s
        rand_time  = 300s

        # Automatically start the tunnel and restart if it drops
        start_action = start
        close_action = restart
        dpd_action    = restart
      }
    }
  }
}

secrets {
  # No PSK — using certificate authentication
}
```

### Step 4 — Configure Gateway B

```hcl
# /etc/swanctl/swanctl.conf — Gateway B (198.51.100.1)

connections {
  site-a {
    version = 2
    local_addrs  = 198.51.100.1
    remote_addrs = 203.0.113.1

    proposals = aes256gcm16-prfsha384-ecp384
    rekey_time = 14400s
    dpd_delay  = 30s
    dpd_timeout = 90s

    local {
      auth = pubkey
      certs = gw-b-cert.pem
      id = "CN=gw-b.example.com, O=DeployCraft, C=GB"
    }

    remote {
      auth = pubkey
      cacerts = ca-cert.pem
      id = "CN=gw-a.example.com, O=DeployCraft, C=GB"
    }

    children {
      net-a {
        local_ts  = 10.0.2.0/24
        remote_ts = 10.0.1.0/24
        mode      = tunnel
        esp_proposals = aes256gcm16-ecp384
        rekey_time = 3600s
        life_time  = 3900s
        rand_time  = 300s
        start_action = start
        close_action = restart
        dpd_action   = restart
      }
    }
  }
}
```

### Step 5 — Start and verify the tunnel

```bash
# On both gateways
sudo systemctl enable strongswan
sudo systemctl start strongswan

# Load the swanctl configuration
sudo swanctl --load-all

# On Gateway A: initiate the tunnel
sudo swanctl --initiate --child net-b

# Check SA status
sudo swanctl --list-sas
# site-b: #1, ESTABLISHED, IKEv2, ...
#   net-b: #1, reqid 1, INSTALLED, TUNNEL, ESP in UDP
#     local  10.0.1.0/24
#     remote 10.0.2.0/24

# Test traffic through the tunnel
ping -c 4 10.0.2.10    # from a host on Site A, targeting Site B
traceroute 10.0.2.10   # should show direct path (1 hop via tunnel)

# Verify packets are being encrypted
sudo ip xfrm state     # shows active SA with bytes transferred
sudo ip xfrm policy    # shows SPD entries matching the tunnel traffic
```

---

## Part B — Road-Warrior Remote Access

### Step 6 — Generate client certificate for road-warrior

```bash
# For each remote user/device, issue a client certificate
ipsec pki --gen --type ecdsa --size 384 \
  --outform pem > ~/pki/keys/alice.pem

ipsec pki --pub --in ~/pki/keys/alice.pem --type ecdsa \
  | ipsec pki --issue --lifetime 365 \
  --cacert ~/pki/ca/ca-cert.pem \
  --cakey ~/pki/ca/ca-key.pem \
  --dn "CN=alice@example.com, O=DeployCraft, C=GB" \
  --san "alice@example.com" \
  --flag clientAuth \
  --outform pem > ~/pki/certs/alice-cert.pem

# Package for distribution (PKCS#12 bundle)
openssl pkcs12 -export \
  -in ~/pki/certs/alice-cert.pem \
  -inkey ~/pki/keys/alice.pem \
  -certfile ~/pki/ca/ca-cert.pem \
  -out alice.p12 \
  -passout pass:"$(openssl rand -base64 16)"
```

### Step 7 — Configure the road-warrior gateway

```hcl
# /etc/swanctl/swanctl.conf — VPN gateway for road warriors

connections {
  road-warrior {
    version = 2
    local_addrs  = 203.0.113.1

    # Road warriors use NAT-T — expect connections from any IP
    remote_addrs = %any

    proposals = aes256gcm16-prfsha384-ecp384

    local {
      auth = pubkey
      certs = gw-a-cert.pem
      id = "gw-a.example.com"
    }

    remote {
      auth = pubkey
      cacerts = ca-cert.pem
      # Clients identified by their cert's CN/SAN
    }

    children {
      road-warrior {
        local_ts  = 0.0.0.0/0   # gateway allows access to all (filter with firewall)
        mode      = tunnel
        esp_proposals = aes256gcm16-ecp384
        rekey_time = 3600s
        dpd_action = clear       # release SA when client disconnects
      }
    }

    # Assign virtual IP from this pool to connected clients
    pools = rw-pool
  }
}

pools {
  rw-pool {
    addrs = 10.8.0.0/24           # pool of virtual IPs for clients
    dns   = 10.0.0.53             # push DNS server to clients
  }
}
```

### Step 8 — Connect a Linux road-warrior client

```bash
# Install strongSwan on client
sudo apt-get install -y strongswan strongswan-pki

# Install certificates
sudo cp ca-cert.pem /etc/swanctl/x509ca/
sudo cp alice-cert.pem /etc/swanctl/x509/
sudo cp alice.pem /etc/swanctl/private/
sudo chmod 600 /etc/swanctl/private/alice.pem
```

```hcl
# /etc/swanctl/swanctl.conf — Alice's laptop

connections {
  corporate {
    version = 2
    local_addrs  = %any
    remote_addrs = 203.0.113.1

    proposals = aes256gcm16-prfsha384-ecp384

    local {
      auth = pubkey
      certs = alice-cert.pem
      id = "alice@example.com"
    }

    remote {
      auth = pubkey
      cacerts = ca-cert.pem
      id = "gw-a.example.com"
    }

    children {
      corp {
        remote_ts = 10.0.0.0/8    # split tunnel: only corporate traffic
        # remote_ts = 0.0.0.0/0   # full tunnel: all traffic
        mode = tunnel
        esp_proposals = aes256gcm16-ecp384
        start_action = none        # client initiates on demand
        dpd_action   = clear
      }
    }
  }
}
```

```bash
sudo swanctl --load-all
sudo swanctl --initiate --child corp

sudo swanctl --list-sas
# corporate: ESTABLISHED
#   corp: INSTALLED, TUNNEL, 10.8.0.2 → 10.0.0.0/8

# Verify virtual IP assigned
ip addr show | grep 10.8.0
# inet 10.8.0.2/32 scope global
```

---

## Step 9 — Monitor and troubleshoot

```bash
# Real-time IKE log
sudo journalctl -u strongswan -f

# Show all active IKE and Child SAs
sudo swanctl --list-sas --raw

# Show SA counters — bytes in/out per tunnel
sudo swanctl --list-sas | grep -E "bytes|packets"

# Show SPD (Security Policy Database)
sudo ip xfrm policy

# Show SAD (Security Association Database) with current byte counts
sudo ip xfrm state | grep -E "src|dst|proto|spi|enc|auth|lifetime|stats"

# Force rekeying a specific Child SA
sudo swanctl --rekey --child net-b

# Terminate a specific SA (triggers reconnect if start_action=restart)
sudo swanctl --terminate --ike site-b

# Packet-level verification: capture ESP on the gateway interface
sudo tcpdump -i eth0 esp -n -v
# Should see ESP packets (not readable — they are encrypted)
```

### Common failure modes

| Symptom | Likely cause | Fix |
|---|---|---|
| `NO_PROPOSAL_CHOSEN` | Algorithm mismatch | Align `proposals` on both sides |
| `AUTHENTICATION_FAILED` | Cert not in CA chain, wrong `id` | Verify cert SAN matches configured `id` |
| IKE established but no traffic | SPD missing or traffic selectors don't match | Check `local_ts`/`remote_ts` and `ip xfrm policy` |
| Tunnel drops every hour | Child SA lifetime expiry + rekeying failure | Check logs at rekey time, verify DH group compatibility |
| Client behind NAT cannot connect | NAT-T not active | Verify UDP/4500 is permitted; check `nat_traversal` |

---

## Step 10 — Firewall rules for IPSec

```bash
# Allow IKE (UDP/500 and UDP/4500 for NAT-T)
ufw allow 500/udp comment "IKEv2"
ufw allow 4500/udp comment "IKEv2 NAT-T"

# Allow ESP (IP protocol 50) — needed when not using NAT-T
iptables -A INPUT -p esp -j ACCEPT
iptables -A OUTPUT -p esp -j ACCEPT

# Allow forwarding between VPN and LAN
iptables -A FORWARD -i eth0 -o eth0 -m policy --dir in --pol ipsec -j ACCEPT
iptables -A FORWARD -i eth0 -o eth0 -m policy --dir out --pol ipsec -j ACCEPT

# NAT for road-warrior clients (so they can reach internet via gateway)
iptables -t nat -A POSTROUTING -s 10.8.0.0/24 -o eth0 -j MASQUERADE
```

---

## What you have built

- A full PKI with CA, gateway certificates, and client certificates using ECDSA P-384
- A site-to-site IPSec tunnel between two Linux gateways with AES-256-GCM and PFS via ECP-384
- A road-warrior VPN with virtual IP assignment, split tunnel, and certificate-based client authentication
- SA monitoring with `swanctl --list-sas` and `ip xfrm state`
- Troubleshooting workflow for the most common IKEv2 failure modes
- Firewall rules for IKE, ESP, and road-warrior client NAT
