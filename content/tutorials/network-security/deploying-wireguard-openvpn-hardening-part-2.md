---
title: "Deploying WireGuard and OpenVPN — Configuration, Certificate Auth, and Hardening — Part 2"
date: 2026-09-01
description: "Deploy WireGuard and OpenVPN on Linux: generate a CA and certificates with EasyRSA, configure server and client for certificate-based auth, enable MFA, apply hardening, and verify tunnel connectivity."
cluster: "Network Security"
series: "VPN"
part: 2
difficulty: "intermediate"
duration: "55 min"
tags: ["vpn", "wireguard", "openvpn", "network-security", "pki", "certificate-auth", "hardening", "devsecops"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

In [Part 1](/tutorials/network-security/vpn-architecture-site-to-site-remote-access-part-1/) you learned VPN design patterns and protocol selection. In Part 2 you will deploy WireGuard for site-to-site and remote access, then deploy OpenVPN with a full PKI for certificate-based authentication, and apply production hardening to both.

## Prerequisites

- Completed [Part 1](/tutorials/network-security/vpn-architecture-site-to-site-remote-access-part-1/)
- Two Linux hosts (Ubuntu 22.04) — one as server/gateway, one as client
- Public IP or reachable address on the server
- `sudo` access on both hosts

---

## Part A — WireGuard

### Step 1 — Install WireGuard

WireGuard is built into the Linux kernel since 5.6. Install the userspace tools:

```bash
# On both server and client
sudo apt-get update && sudo apt-get install -y wireguard wireguard-tools
```

### Step 2 — Generate keypairs

Each peer needs a private/public keypair. Keys are Curve25519.

```bash
# On the server
wg genkey | sudo tee /etc/wireguard/server_private.key | \
  wg pubkey | sudo tee /etc/wireguard/server_public.key
sudo chmod 600 /etc/wireguard/server_private.key

SERVER_PRIVATE=$(sudo cat /etc/wireguard/server_private.key)
SERVER_PUBLIC=$(sudo cat /etc/wireguard/server_public.key)
echo "Server public key: $SERVER_PUBLIC"

# On the client
wg genkey | sudo tee /etc/wireguard/client_private.key | \
  wg pubkey | sudo tee /etc/wireguard/client_public.key
sudo chmod 600 /etc/wireguard/client_private.key

CLIENT_PRIVATE=$(sudo cat /etc/wireguard/client_private.key)
CLIENT_PUBLIC=$(sudo cat /etc/wireguard/client_public.key)
echo "Client public key: $CLIENT_PUBLIC"
```

### Step 3 — Configure the server

```bash
# /etc/wireguard/wg0.conf on the server
sudo cat > /etc/wireguard/wg0.conf << EOF
[Interface]
Address = 10.8.0.1/24
ListenPort = 51820
PrivateKey = ${SERVER_PRIVATE}

# Enable routing and NAT for VPN clients to reach internet
PostUp = iptables -A FORWARD -i wg0 -j ACCEPT; iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
PostDown = iptables -D FORWARD -i wg0 -j ACCEPT; iptables -t nat -D POSTROUTING -o eth0 -j MASQUERADE

[Peer]
# Client 1
PublicKey = ${CLIENT_PUBLIC}
AllowedIPs = 10.8.0.2/32    # only this IP from this peer
EOF

sudo chmod 600 /etc/wireguard/wg0.conf
```

Enable IP forwarding (required for routing between VPN and LAN):

```bash
echo "net.ipv4.ip_forward=1" | sudo tee -a /etc/sysctl.d/99-wireguard.conf
sudo sysctl -p /etc/sysctl.d/99-wireguard.conf
```

Start WireGuard:

```bash
sudo systemctl enable wg-quick@wg0
sudo systemctl start wg-quick@wg0
sudo wg show
```

### Step 4 — Configure the client

```bash
# /etc/wireguard/wg0.conf on the client
SERVER_PUBLIC="<paste server public key here>"

sudo cat > /etc/wireguard/wg0.conf << EOF
[Interface]
Address = 10.8.0.2/24
PrivateKey = ${CLIENT_PRIVATE}
DNS = 10.8.0.1           # use server-side DNS resolver

[Peer]
PublicKey = ${SERVER_PUBLIC}
Endpoint = <server-public-ip>:51820
AllowedIPs = 10.0.0.0/8   # split tunnel: only route internal traffic through VPN
# AllowedIPs = 0.0.0.0/0  # full tunnel: route all traffic through VPN
PersistentKeepalive = 25   # keep NAT mapping alive
EOF

sudo chmod 600 /etc/wireguard/wg0.conf
sudo wg-quick up wg0
```

Verify connectivity:

```bash
# From client: ping the server VPN IP
ping -c 3 10.8.0.1

# Show WireGuard status — check handshake timestamp and transfer stats
sudo wg show
# peer: <server-public-key>
#   endpoint: <server-ip>:51820
#   allowed ips: 10.0.0.0/8
#   latest handshake: 3 seconds ago
#   transfer: 92 B received, 180 B sent
```

### Step 5 — Add a pre-shared key for post-quantum resistance

WireGuard's Curve25519 keys are vulnerable to future quantum computers. Add a symmetric pre-shared key (PSK) as a second layer:

```bash
# Generate PSK (do this once, share securely to all peers)
wg genpsk | sudo tee /etc/wireguard/psk.key
sudo chmod 600 /etc/wireguard/psk.key

PSK=$(sudo cat /etc/wireguard/psk.key)
```

Add `PresharedKey = ${PSK}` to the `[Peer]` section on both server and client.

---

## Part B — OpenVPN with PKI

### Step 6 — Build a Certificate Authority with EasyRSA

```bash
# Install EasyRSA
apt-get install -y easy-rsa
mkdir -p /etc/openvpn/pki
cp -r /usr/share/easy-rsa/* /etc/openvpn/pki/
cd /etc/openvpn/pki

# Initialise the PKI
./easyrsa init-pki

# Build the CA (non-interactive)
./easyrsa --batch \
  --req-cn="DeployCraft VPN CA" \
  --keysize=4096 \
  --digest=sha256 \
  build-ca nopass

# Generate server certificate and key
./easyrsa --batch \
  --req-cn="vpn-server" \
  gen-req vpn-server nopass

./easyrsa --batch sign-req server vpn-server

# Generate Diffie-Hellman parameters
./easyrsa gen-dh

# Generate TLS auth key (protects against DoS on TLS handshake)
openvpn --genkey secret /etc/openvpn/ta.key
```

Generate a client certificate:

```bash
cd /etc/openvpn/pki

# Each client gets a unique certificate
./easyrsa --batch --req-cn="client-alice" gen-req client-alice nopass
./easyrsa --batch sign-req client client-alice
```

### Step 7 — Configure the OpenVPN server

```bash
cat > /etc/openvpn/server/server.conf << 'EOF'
# Network
port 1194
proto udp
dev tun

# Certificates
ca   /etc/openvpn/pki/pki/ca.crt
cert /etc/openvpn/pki/pki/issued/vpn-server.crt
key  /etc/openvpn/pki/pki/private/vpn-server.key
dh   /etc/openvpn/pki/pki/dh.pem
tls-auth /etc/openvpn/ta.key 0

# IP pool for clients
server 10.8.0.0 255.255.255.0
ifconfig-pool-persist /var/log/openvpn/ipp.txt

# Push routes to clients (split tunnel)
push "route 10.0.0.0 255.0.0.0"
push "dhcp-option DNS 10.0.0.53"

# Hardened cipher selection (TLS 1.3 only)
tls-version-min 1.3
tls-cipher TLS_AES_256_GCM_SHA384:TLS_CHACHA20_POLY1305_SHA256
cipher AES-256-GCM
auth SHA512

# Security
user nobody
group nogroup
persist-key
persist-tun
verify-client-cert require    # enforce cert auth (no password fallback)
remote-cert-tls client

# Prevent client-to-client traffic (isolate clients from each other)
client-to-client               # remove this line to isolate clients

# Logging
status /var/log/openvpn/status.log 30
log-append /var/log/openvpn/openvpn.log
verb 3
mute 20

# Revocation list (update when revoking client certs)
crl-verify /etc/openvpn/pki/pki/crl.pem
EOF
```

Enable and start:

```bash
sudo systemctl enable openvpn-server@server
sudo systemctl start openvpn-server@server
sudo systemctl status openvpn-server@server
```

### Step 8 — Generate a client configuration file

Bundle all certificates into a single `.ovpn` file for easy distribution:

```bash
#!/bin/bash
# gen-client-config.sh
CLIENT="$1"
PKI_DIR="/etc/openvpn/pki/pki"
SERVER_IP="<your-server-ip>"

cat > "${CLIENT}.ovpn" << EOF
client
dev tun
proto udp
remote ${SERVER_IP} 1194
resolv-retry infinite
nobind
persist-key
persist-tun
remote-cert-tls server
tls-version-min 1.3
cipher AES-256-GCM
auth SHA512
verb 3

<ca>
$(cat ${PKI_DIR}/ca.crt)
</ca>

<cert>
$(cat ${PKI_DIR}/issued/${CLIENT}.crt)
</cert>

<key>
$(cat ${PKI_DIR}/private/${CLIENT}.key)
</key>

<tls-auth>
$(cat /etc/openvpn/ta.key)
</tls-auth>
key-direction 1
EOF

echo "Generated ${CLIENT}.ovpn"
```

```bash
chmod +x gen-client-config.sh
./gen-client-config.sh client-alice
# Distribute client-alice.ovpn securely
```

Import on the client:

```bash
# Linux
sudo apt-get install -y openvpn
sudo openvpn --config client-alice.ovpn
```

---

## Step 9 — Revoke a client certificate

When a user leaves or a device is compromised, revoke their certificate:

```bash
cd /etc/openvpn/pki
./easyrsa revoke client-alice
./easyrsa gen-crl

# Copy the updated CRL to where the server reads it
cp pki/crl.pem /etc/openvpn/pki/pki/crl.pem

# Reload the server (picks up new CRL without restart)
sudo kill -HUP $(pidof openvpn)
```

The revoked client certificate is now rejected — the client cannot reconnect even with the same `.ovpn` file.

---

## Step 10 — Hardening checklist

Apply these settings to both WireGuard and OpenVPN deployments before production:

### Firewall rules (server)

```bash
# Allow VPN port (UDP 51820 for WireGuard, 1194 for OpenVPN)
ufw allow 51820/udp comment "WireGuard"
ufw allow 1194/udp comment "OpenVPN"

# Allow established VPN client traffic to internal resources
ufw allow in on wg0 to 10.0.0.0/8
ufw allow in on tun0 to 10.0.0.0/8

# Block everything else inbound
ufw default deny incoming
ufw default allow outgoing
ufw enable
```

### Harden SSH on the VPN gateway

```bash
cat >> /etc/ssh/sshd_config << 'EOF'
PermitRootLogin no
PasswordAuthentication no
AllowUsers vpnadmin
MaxAuthTries 3
LoginGraceTime 20
EOF
sudo systemctl reload sshd
```

### Monitor active VPN sessions

```bash
# WireGuard: show all peers and last handshake times
watch -n 5 sudo wg show

# Alert if a peer has not had a handshake in over 5 minutes (disconnected client)
# Integrate with your monitoring stack:
sudo wg show all dump | awk '$5 != "0" && (systime() - $5) > 300 {print "Stale peer:", $2}'

# OpenVPN: watch live connections
sudo cat /var/log/openvpn/status.log
```

### Rotate WireGuard keys periodically

```bash
#!/bin/bash
# rotate-wg-key.sh — run monthly via cron
NEW_PRIVATE=$(wg genkey)
NEW_PUBLIC=$(echo $NEW_PRIVATE | wg pubkey)

# Update server config with new client pubkey
# (client must be notified and rotate their endpoint config simultaneously)
echo "New public key for peer rotation: $NEW_PUBLIC"
```

---

## Comparison: WireGuard vs OpenVPN in production

| Dimension | WireGuard | OpenVPN |
|---|---|---|
| Config complexity | Very low — key exchange only | Moderate — PKI, cipher suite, options |
| Performance | ~3× faster than OpenVPN | Adequate for <1 Gbps |
| Audit surface | ~4,000 lines of code | Much larger codebase |
| Authentication | Public keys only (no cert revocation) | Full PKI with CRL/OCSP |
| Client revocation | Remove peer from config + reload | Revoke cert + update CRL |
| Logging | Minimal (by design) | Verbose, configurable |
| Enterprise features | Limited | LDAP/RADIUS auth, SAML, MFA plugins |
| OS support | Linux 5.6+, macOS, Windows, iOS, Android | All platforms, 15+ years of clients |

**Recommendation:** Use WireGuard for infrastructure-to-infrastructure (server to cloud, site-to-site between Linux gateways). Use OpenVPN for remote access with certificate revocation requirements, LDAP authentication, or enterprise user management.

---

## What you have built

- WireGuard server and client configured with Curve25519 keypairs and optional PSK for quantum resistance
- OpenVPN PKI built with EasyRSA — CA, server cert, client certs, DH params, TLS auth key
- OpenVPN server hardened with TLS 1.3 only, AES-256-GCM, SHA-512 auth
- Client `.ovpn` bundle generator script for easy distribution
- Certificate revocation workflow — revoke, generate CRL, reload server
- Firewall rules for both WireGuard and OpenVPN gateways
- Session monitoring and stale peer detection
- A clear comparison of when to use each implementation in production
