---
title: "Applied Cryptography — TLS 1.3, Certificate Chains, and Key Management — Part 2"
date: 2026-09-01
description: "Apply cryptographic primitives in practice: trace the TLS 1.3 handshake step by step, validate X.509 certificate chains, configure strong cipher suites, and implement key rotation and HSM-backed key management."
cluster: "Network Security"
series: "Cryptography"
part: 2
difficulty: "advanced"
duration: "50 min"
tags: ["cryptography", "tls", "pki", "certificates", "key-management", "network-security", "openssl", "devsecops"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

In [Part 1](/tutorials/network-security/cryptography-algorithms-symmetric-asymmetric-hash-part-1/) you learned the cryptographic primitives — AES, RSA, ECC, SHA-2, HMAC, HKDF. In Part 2 you will see them in action: trace the TLS 1.3 handshake packet by packet, understand X.509 certificate chain validation, configure server TLS to enforce strong cipher suites, and apply key management best practices for production systems.

## Prerequisites

- Completed [Part 1](/tutorials/network-security/cryptography-algorithms-symmetric-asymmetric-hash-part-1/)
- OpenSSL installed (`apt-get install -y openssl`)
- A Linux host for exercises

---

## Step 1 — The TLS 1.3 handshake

TLS 1.3 reduces the handshake to 1 round-trip (1-RTT). Here is exactly what happens when a browser connects to an HTTPS server:

### Client Hello

The client sends its capabilities and starts the key exchange:

```
Client → Server: ClientHello
  - TLS version: 1.3
  - Random: 32 bytes of client randomness
  - Cipher suites (in preference order):
      TLS_AES_256_GCM_SHA384
      TLS_CHACHA20_POLY1305_SHA256
      TLS_AES_128_GCM_SHA256
  - Supported groups (key exchange): x25519, P-256, P-384
  - Key share: ECDH public key for x25519 (pre-computed to save 1 RTT)
  - Signature algorithms: ecdsa_secp256r1_sha256, rsa_pss_rsae_sha256, ...
  - Server name (SNI): example.com
```

The client sends its ECDH public key immediately — without knowing which curve the server will select — to enable 1-RTT. If the server selects a different group, it requests a retry (HelloRetryRequest), costing an extra round trip.

### Server Hello and key derivation

```
Server → Client: ServerHello
  - Selected cipher suite: TLS_AES_256_GCM_SHA384
  - Selected group: x25519
  - Key share: server's ECDH public key for x25519
```

At this point, both sides can compute the shared secret:

```
shared_secret = ECDH(client_private, server_public)
             = ECDH(server_private, client_public)

# Key schedule (HKDF-SHA384):
early_secret      = HKDF-Extract(0, 0)
handshake_secret  = HKDF-Extract(derived_secret, shared_secret)
  client_hs_key   = HKDF-Expand(handshake_secret, "c hs traffic", 32)
  server_hs_key   = HKDF-Expand(handshake_secret, "s hs traffic", 32)
master_secret     = HKDF-Extract(derived_secret, 0)
  client_app_key  = HKDF-Expand(master_secret, "c ap traffic", 32)
  server_app_key  = HKDF-Expand(master_secret, "s ap traffic", 32)
```

All subsequent handshake and application data is encrypted with these derived keys.

### Server certificate and Finished

```
Server → Client: (encrypted with server_hs_key)
  Certificate:
    - Server certificate chain (leaf + intermediates, NOT root)
    - Chain must be valid for SNI "example.com"
  CertificateVerify:
    - Signature: ECDSA-P256(server_private_key, transcript_hash)
    - Client verifies: server holds the private key for the leaf cert
  Finished:
    - HMAC of the entire handshake transcript
    - Ensures neither side tampered with the handshake messages

Client → Server: (encrypted with client_hs_key)
  Finished:
    - HMAC of handshake transcript from client side
```

### Application data

From this point, all data uses `client_app_key` (client→server) and `server_app_key` (server→client). Each record includes an authentication tag — tampering is detected immediately.

---

## Step 2 — X.509 certificate chain validation

A TLS certificate is not trusted on its own — it is trusted because it is signed by a CA the client already trusts.

### Certificate chain structure

```
Root CA (self-signed, in OS/browser trust store)
  └── Intermediate CA (signed by Root CA)
        └── Leaf certificate (signed by Intermediate CA)
              - Subject: CN=example.com
              - SAN: example.com, www.example.com
              - Key usage: Digital Signature, Key Encipherment
              - Extended key usage: TLS Web Server Authentication
              - Valid: 2026-01-01 to 2026-12-31
              - Public key: ECDSA P-256
```

### Validation steps

When a TLS client validates a server certificate:

1. **Chain building:** Assemble a chain from leaf to a trusted root. The server must send all intermediates (not the root).
2. **Signature verification:** Verify each cert is signed by the next CA in the chain.
3. **Validity period:** Each cert's `notBefore` ≤ now ≤ `notAfter`.
4. **Revocation check:** Verify the cert has not been revoked — via CRL (Certificate Revocation List) or OCSP (Online Certificate Status Protocol).
5. **SAN matching:** The server's hostname matches the Subject Alternative Name list on the leaf cert. CN is ignored for hostname matching in modern clients.
6. **Key usage:** `Extended Key Usage` must include `id-kp-serverAuth` for TLS server certificates.

```bash
# Inspect a certificate chain
openssl s_client -connect example.com:443 -showcerts 2>/dev/null \
  | openssl x509 -noout -text | grep -A3 "Subject\|Issuer\|Validity\|SAN"

# Verify a certificate against a CA bundle
openssl verify -CAfile /etc/ssl/certs/ca-certificates.crt server.crt

# Check OCSP status
openssl s_client -connect example.com:443 -status 2>/dev/null \
  | grep -A5 "OCSP response"

# Extract the full chain
openssl s_client -connect example.com:443 -showcerts 2>/dev/null \
  | awk '/BEGIN CERT/,/END CERT/' > chain.pem
```

### Certificate pinning

For high-security applications, pin the expected certificate or public key hash. If the server presents a different certificate (even a valid one), the client rejects the connection — defeating CA compromise attacks.

```python
import ssl, hashlib, socket

def get_cert_hash(hostname, port=443):
    ctx = ssl.create_default_context()
    with socket.create_connection((hostname, port)) as sock:
        with ctx.wrap_socket(sock, server_hostname=hostname) as ssock:
            cert_der = ssock.getpeercert(binary_form=True)
            return hashlib.sha256(cert_der).hexdigest()

# Pin the known-good hash
PINNED_HASH = "abc123..."    # pre-computed from known good cert
actual_hash = get_cert_hash("api.example.com")
assert actual_hash == PINNED_HASH, f"Certificate mismatch: {actual_hash}"
```

---

## Step 3 — Configure strong server TLS

### nginx

```nginx
server {
    listen 443 ssl http2;
    server_name example.com;

    ssl_certificate     /etc/ssl/certs/example.com.crt;
    ssl_certificate_key /etc/ssl/private/example.com.key;

    # TLS 1.3 only (drop 1.2 for maximum security)
    ssl_protocols TLSv1.3;

    # TLS 1.3 cipher suites are fixed — this line is informational only
    ssl_ciphers TLS_AES_256_GCM_SHA384:TLS_CHACHA20_POLY1305_SHA256:TLS_AES_128_GCM_SHA256;
    ssl_prefer_server_ciphers off;    # client preference in TLS 1.3

    # ECDH curve for key exchange
    ssl_ecdh_curve X25519:P-384:P-256;

    # OCSP stapling — server fetches and caches OCSP response
    ssl_stapling on;
    ssl_stapling_verify on;
    ssl_trusted_certificate /etc/ssl/certs/ca-chain.crt;
    resolver 9.9.9.9 valid=300s;

    # Session tickets — disable for perfect forward secrecy
    ssl_session_tickets off;

    # HSTS — tell browsers to always use HTTPS (max-age 2 years)
    add_header Strict-Transport-Security "max-age=63072000; includeSubDomains; preload" always;
}
```

### Apache

```apache
<VirtualHost *:443>
    SSLEngine on
    SSLCertificateFile    /etc/ssl/certs/example.com.crt
    SSLCertificateKeyFile /etc/ssl/private/example.com.key
    SSLCertificateChainFile /etc/ssl/certs/ca-chain.crt

    SSLProtocol -all +TLSv1.3
    SSLHonorCipherOrder off
    SSLUseStapling on
    SSLStaplingCache "shmcb:logs/ssl_stapling(32768)"

    Header always set Strict-Transport-Security "max-age=63072000; includeSubDomains"
</VirtualHost>
```

### Verify with testssl.sh

```bash
# Install testssl.sh
git clone --depth 1 https://github.com/drwetter/testssl.sh.git
cd testssl.sh

# Run a full TLS assessment
./testssl.sh --full example.com

# Check specifically for weak protocols and ciphers
./testssl.sh --protocols --ciphers example.com
```

---

## Step 4 — Key generation best practices

### Generate keys with adequate entropy

```bash
# ECDSA P-256 server key (preferred for performance)
openssl ecparam -name prime256v1 -genkey -noout -out server.key
openssl ec -in server.key -pubout -out server.pub

# ECDSA P-384 for higher security requirements
openssl ecparam -name secp384r1 -genkey -noout -out server-p384.key

# RSA 3072 (if RSA required for legacy compatibility)
openssl genrsa -out server-rsa.key 3072

# Verify key type and size
openssl ec -in server.key -noout -text | head -5
# read EC key
# Private-Key: (256 bit)

# Protect the key — encrypt at rest
openssl ec -in server.key -out server-encrypted.key -aes256
```

### Key permissions

```bash
# Private keys must be readable only by the service user
chmod 600 /etc/ssl/private/server.key
chown root:ssl-cert /etc/ssl/private/server.key

# Never commit private keys to git
echo "*.key" >> .gitignore
echo "*.pem" >> .gitignore
```

---

## Step 5 — Key rotation

Keys and certificates have finite lifetimes. Rotation ensures that a compromised key has limited exposure.

### Certificate rotation workflow

```bash
#!/bin/bash
# rotate-cert.sh

DOMAIN="example.com"
CERT_DIR="/etc/ssl"

# 1. Generate a new key (keep old key until new cert is deployed)
openssl ecparam -name prime256v1 -genkey -noout -out ${CERT_DIR}/private/${DOMAIN}-new.key

# 2. Generate CSR
openssl req -new \
  -key ${CERT_DIR}/private/${DOMAIN}-new.key \
  -out ${CERT_DIR}/certs/${DOMAIN}-new.csr \
  -subj "/CN=${DOMAIN}/O=Example/C=GB" \
  -addext "subjectAltName=DNS:${DOMAIN},DNS:www.${DOMAIN}"

# 3. Submit CSR to CA (e.g., Let's Encrypt via certbot, or internal CA)
# certbot certonly --csr ${CERT_DIR}/certs/${DOMAIN}-new.csr

# 4. Once signed cert received, swap atomically
mv ${CERT_DIR}/private/${DOMAIN}.key ${CERT_DIR}/private/${DOMAIN}-old.key
mv ${CERT_DIR}/private/${DOMAIN}-new.key ${CERT_DIR}/private/${DOMAIN}.key
mv ${CERT_DIR}/certs/${DOMAIN}-new.crt ${CERT_DIR}/certs/${DOMAIN}.crt

# 5. Reload web server (not restart — avoids downtime)
nginx -s reload

echo "Certificate rotated. Old key retained at ${CERT_DIR}/private/${DOMAIN}-old.key"
```

**Rotation schedule:**
- TLS server certificates: 90 days (Let's Encrypt) or 1 year
- CA certificates: 5–10 years (with planned succession)
- Symmetric data-at-rest encryption keys: annually or on-demand after compromise
- Session tokens / API keys: 90 days maximum, or event-driven (breach, employee departure)

---

## Step 6 — HSM-backed key management

For keys that protect high-value data (CA private keys, data encryption keys), use a Hardware Security Module (HSM). An HSM stores keys in tamper-resistant hardware — the private key never leaves the HSM in plaintext.

### Cloud HSM options

```bash
# AWS KMS — envelope encryption
aws kms create-key --description "data-encryption-key" \
  --key-spec SYMMETRIC_DEFAULT \
  --key-usage ENCRYPT_DECRYPT

# Encrypt data using KMS
aws kms encrypt \
  --key-id alias/my-key \
  --plaintext fileb://data.bin \
  --output text --query CiphertextBlob | base64 -d > data.encrypted

# Decrypt (KMS does the decryption inside the HSM — plaintext key never exposed)
aws kms decrypt \
  --ciphertext-blob fileb://data.encrypted \
  --output text --query Plaintext | base64 -d > data.decrypted
```

### SoftHSM for development

```bash
# Install SoftHSM2 for local HSM simulation
apt-get install -y softhsm2 opensc

# Initialise a token
softhsm2-util --init-token --slot 0 --label "dev-ca" --pin 1234 --so-pin 5678

# Generate a key inside the HSM
pkcs11-tool --module /usr/lib/softhsm/libsofthsm2.so \
  --login --pin 1234 \
  --keypairgen --key-type EC:prime256v1 \
  --label "ca-key" --id 01

# Use the HSM key with OpenSSL via PKCS#11 engine
openssl req -new -engine pkcs11 -keyform engine \
  -key "pkcs11:token=dev-ca;object=ca-key;type=private" \
  -out server.csr -subj "/CN=example.com"
```

---

## Step 7 — Common cryptographic implementation mistakes

| Mistake | Risk | Correct approach |
|---|---|---|
| Comparing MACs with `==` | Timing attack leaks how many bytes matched | Use `hmac.compare_digest()` or `crypto/subtle.ConstantTimeCompare` |
| Reusing a nonce with AES-GCM | Catastrophic key recovery — same nonce + key = keystream reuse | Generate a fresh random nonce for every message; use a counter with a unique key per connection |
| ECB mode | Pattern leakage (same plaintext → same ciphertext) | Use AES-256-GCM |
| Storing passwords with SHA-256 | Fast hashing = fast brute force | Use bcrypt, scrypt, or Argon2id with appropriate work factor |
| Encrypting without authenticating | Bit-flip attacks modify ciphertext without detection | Use AEAD (AES-GCM or ChaCha20-Poly1305) — authentication is built in |
| Small RSA key for long-lived CA | Key becomes breakable before cert expires | Use ECDSA P-384 for CA keys; RSA minimum 3072-bit |
| Hardcoded keys or IVs | Attacker reads source, decrypts all data | Generate keys from secure random source; store in HSM or secret manager |
| MD5 or SHA-1 in new code | Collision vulnerabilities | SHA-256 minimum; SHA-384 for TLS |

---

## What you have built

- The complete TLS 1.3 handshake — ECDH key agreement, HKDF key schedule, certificate authentication, Finished MAC
- X.509 certificate chain validation — building the chain, signature verification, SANs, OCSP, revocation
- Certificate pinning for high-security application connections
- nginx and Apache TLS hardening — TLS 1.3 only, OCSP stapling, HSTS, session ticket disablement
- Key generation with `openssl` and correct permission hardening
- Certificate rotation script with atomic swap and server reload
- AWS KMS and SoftHSM2 for HSM-backed key management
- A table of common cryptographic mistakes and their correct alternatives
