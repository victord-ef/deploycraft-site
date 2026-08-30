---
title: "How mTLS Actually Works End to End"
date: 2026-08-06
author: "Victor D"
description: "Most explanations of mutual TLS stop at 'both sides present certificates'. This article walks through the full handshake, what each party actually verifies, and why mTLS is the foundation of zero-trust networking."
tags: ["mtls", "tls", "security", "certificates", "kubernetes", "service-mesh", "zero-trust"]
categories: ["article"]
draft: false
toc: true
---

TLS is everywhere. You use it every time you open a browser. But standard TLS only authenticates one side — the server proves its identity to the client, and the client is taken on trust.

**Mutual TLS** (mTLS) closes that gap. Both parties present certificates. Both verify each other before a single byte of application data is exchanged. It is the cryptographic foundation of zero-trust networking and the mechanism behind service mesh security in Kubernetes.

Most explanations stop at "both sides present certificates." This article goes deeper — walking through the full handshake, explaining what each party actually checks, and clarifying what mTLS does and does not protect against.

---

## Standard TLS first — what you already know

In a standard TLS 1.3 handshake, the client connects to the server and the server presents a certificate. The client verifies:

1. The certificate is signed by a trusted Certificate Authority (CA)
2. The certificate has not expired
3. The certificate's Subject Alternative Name (SAN) matches the hostname the client connected to
4. The certificate has not been revoked (via OCSP or CRL)

If all checks pass, the client trusts the server. A symmetric session key is derived and the connection is encrypted.

The server, however, knows nothing about who the client is. Any client that can complete a TCP connection can initiate a TLS session.

---

## What mTLS adds

mTLS extends the handshake so the **client also presents a certificate** and the **server verifies it** using the same chain of trust logic.

The result: both parties are cryptographically authenticated before the connection proceeds. Neither side can impersonate a trusted party without holding a valid private key signed by the trusted CA.

---

## The mTLS handshake, step by step

Here is the full TLS 1.3 handshake with mutual authentication. Both parties have certificates issued by the same CA (or a CA chain the other side trusts).

### Step 1 — ClientHello

The client sends:

- Supported TLS versions (1.3 in modern deployments)
- Supported cipher suites (e.g. `TLS_AES_256_GCM_SHA384`)
- A random nonce (`client_random`)
- Key share for key agreement (e.g. X25519 ECDH public key)

No certificates yet. This is pure negotiation.

```
Client → Server: ClientHello
  version: TLS 1.3
  random: a3f2...
  cipher_suites: [TLS_AES_256_GCM_SHA384, TLS_CHACHA20_POLY1305_SHA256]
  key_share: X25519 pubkey
```

### Step 2 — ServerHello

The server responds with:

- Selected cipher suite
- A random nonce (`server_random`)
- Its own X25519 key share

At this point both parties have enough material to derive the **handshake traffic keys** using HKDF. All subsequent messages in the handshake are encrypted.

```
Server → Client: ServerHello
  cipher_suite: TLS_AES_256_GCM_SHA384
  random: 9b1e...
  key_share: X25519 pubkey
```

### Step 3 — Server Certificate and CertificateVerify

The server sends its certificate chain and proves it holds the private key:

```
Server → Client: Certificate
  [server cert → intermediate CA cert → root CA cert]

Server → Client: CertificateVerify
  signature over handshake transcript using server's private key
```

The **CertificateVerify** message is critical — it is a digital signature over everything exchanged so far, proving the server holds the private key corresponding to the public key in its certificate. Presenting the certificate alone is not proof of identity.

### Step 4 — CertificateRequest (the mTLS addition)

In a standard TLS handshake this step does not exist. In mTLS, the server sends a **CertificateRequest** message asking the client to prove its identity:

```
Server → Client: CertificateRequest
  acceptable_certificate_types: RSA, ECDSA
  certificate_authorities: [CN=Internal CA]
```

This is what makes the handshake mutual.

### Step 5 — Server Finished

The server sends a Finished message — an HMAC over the entire handshake transcript using the derived handshake keys. This confirms the server's view of the negotiation.

```
Server → Client: Finished
  verify_data: HMAC(handshake_traffic_key, transcript_hash)
```

### Step 6 — Client Certificate and CertificateVerify

The client now presents its certificate and proves it holds the private key, mirroring what the server did in Step 3:

```
Client → Server: Certificate
  [client cert → intermediate CA cert → root CA cert]

Client → Server: CertificateVerify
  signature over handshake transcript using client's private key
```

### Step 7 — Client Finished

The client sends its Finished message:

```
Client → Server: Finished
  verify_data: HMAC(handshake_traffic_key, transcript_hash)
```

Both Finished messages have been verified. Both parties have authenticated each other. The **application traffic keys** are derived and the connection is established.

---

## What each side actually verifies

### The client verifies the server certificate

1. **Chain of trust** — the server cert is signed by an intermediate, which is signed by a root CA the client trusts
2. **Expiry** — `notBefore` and `notAfter` fields are valid
3. **SAN match** — the Subject Alternative Name matches the hostname (or SPIFFE URI in Kubernetes)
4. **Key usage** — the certificate has the `serverAuth` extended key usage
5. **CertificateVerify signature** — the server actually holds the private key

### The server verifies the client certificate

1. **Chain of trust** — the client cert is signed by a CA the server trusts (often the same internal CA)
2. **Expiry** — validity period is current
3. **Key usage** — the certificate has the `clientAuth` extended key usage
4. **CertificateVerify signature** — the client actually holds the private key
5. **Optionally: identity** — the server may extract the client's identity from the certificate's Subject or SAN and apply authorisation policy

This last point is important: **authentication and authorisation are separate**. mTLS proves identity. What you do with that identity — allow, deny, rate-limit — is an authorisation decision made by the application or a policy engine.

---

## Certificates in Kubernetes: SPIFFE and SVID

In a Kubernetes service mesh (Istio, Linkerd, Cilium), certificates are not issued per hostname — they use the **SPIFFE** (Secure Production Identity Framework for Everyone) standard.

Each workload gets a **SVID** (SPIFFE Verifiable Identity Document) — a certificate whose SAN is a SPIFFE URI:

```
spiffe://cluster.local/ns/production/sa/api-server
```

This encodes the trust domain, namespace, and service account. When two services establish an mTLS connection, they exchange SVIDs. The receiving side verifies:

1. The SVID is signed by the cluster's root CA (managed by Istio's istiod or Linkerd's identity service)
2. The SPIFFE URI is for a workload in a namespace the receiver trusts
3. The certificate is current (SVIDs are short-lived — typically 24 hours, rotated automatically)

```bash
# Inspect a workload's SVID in Istio
istioctl proxy-config secret <pod-name> -n production

# Output shows:
# RESOURCE NAME    TYPE    STATUS    VALID CERT    SERIAL NUMBER    NOT AFTER
# default          Cert    ACTIVE    true          a1b2c3...        2026-08-07T12:00:00Z
```

Short-lived certificates are a key security property — if a certificate is compromised, it expires quickly. There is no need to revoke it.

---

## What mTLS protects against

**Man-in-the-middle attacks** — an attacker cannot intercept or modify traffic between two services because they cannot present a valid certificate signed by the trusted CA.

**Spoofing** — a malicious service cannot impersonate `payment-service` without holding a private key for a certificate signed by the cluster CA.

**Eavesdropping** — all application traffic is encrypted with symmetric keys derived during the handshake. Capturing packets yields ciphertext.

**Replay attacks** — the random nonces (`client_random`, `server_random`) ensure that each handshake produces unique session keys. Replaying a captured handshake produces a different key and fails.

---

## What mTLS does not protect against

mTLS is a transport-layer mechanism. It does not protect against:

**Compromised workloads** — if an attacker gains code execution inside a Pod, they have access to its certificate and private key. mTLS authenticates the workload identity, not the code running inside it.

**Authorisation failures** — mTLS proves who you are, not what you are allowed to do. A misconfigured authorisation policy can still allow a legitimate workload to do things it should not.

**Application-layer attacks** — SQL injection, XSS, and SSRF happen above the TLS layer. mTLS does not protect your application from itself.

**Certificate theft** — private keys must be protected at rest. In Kubernetes, Istio stores workload keys in memory inside the Envoy sidecar, never writing them to disk. But a host-level compromise can still expose them.

---

## mTLS in practice: what the sidecar does

In an Istio-managed cluster, mTLS is transparent to your application. The Envoy sidecar proxy intercepts all inbound and outbound connections and handles the TLS handshake.

```
Pod A (app container)
  ↓ plaintext on localhost
Envoy sidecar (Pod A)
  ↓ mTLS (SVID cert, encrypted)
Envoy sidecar (Pod B)
  ↓ plaintext on localhost
Pod B (app container)
```

Your application code never sees certificates. It connects to localhost as if no encryption exists. The sidecar enforces mTLS transparently.

You can verify that mTLS is active between two services:

```bash
# Check PeerAuthentication policy
kubectl get peerauthentication -n production

# Check that a connection between services uses mTLS
istioctl authn tls-check <pod-name>.<namespace> <service>.<namespace>.svc.cluster.local
```

---

## Certificate rotation without downtime

One operational advantage of short-lived certificates and automatic rotation is that you never perform a manual certificate renewal that risks downtime.

In Istio, istiod rotates SVIDs before they expire. The new certificate is pushed to the Envoy sidecar, which begins using it for new connections while existing connections complete on the old certificate. The rotation is seamless.

```bash
# Check when the root CA expires (Istio)
kubectl get secret istio-ca-secret -n istio-system \
  -o jsonpath='{.data.ca-cert\.pem}' | base64 -d \
  | openssl x509 -noout -enddate
```

The root CA itself has a longer lifetime (typically 10 years) and rotation requires more careful planning — but intermediate certificates and SVIDs rotate continuously without operator intervention.

---

## The bottom line

mTLS is not complicated in concept — it is TLS with the handshake extended in one direction. But the details matter: the CertificateVerify signature, the SPIFFE identity model, the separation of authentication from authorisation, and the operational guarantees of short-lived automatic rotation.

When you see "zero-trust networking" in a Kubernetes context, mTLS is almost always the mechanism underneath it. Every service proves who it is before the connection proceeds, and no service is trusted by virtue of network position alone.

That is the guarantee mTLS provides — and understanding the handshake in detail makes it much easier to debug when things go wrong.

---

## Related reading

- Enabling mTLS between services with Istio PeerAuthentication → **Tutorial: SM-02**
- Securing service-to-service communication with Linkerd → **Tutorial: SM-03**
- [SPIFFE specification](https://spiffe.io/docs/latest/spiffe-about/spiffe-concepts/)
- [TLS 1.3 RFC 8446](https://datatracker.ietf.org/doc/html/rfc8446)
