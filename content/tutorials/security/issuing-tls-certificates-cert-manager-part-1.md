---
title: "Issuing TLS Certificates for Workloads Using cert-manager — Part 1"
date: 2026-09-01
description: "Install cert-manager, configure Issuers and ClusterIssuers, and issue TLS certificates for Kubernetes workloads using self-signed, CA, and ACME sources."
cluster: "Security & Hardening"
series: "Certificates"
part: 1
difficulty: "intermediate"
duration: "45 min"
tags: ["kubernetes", "cert-manager", "tls", "certificates", "security", "ingress"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

By the end of this tutorial you will have cert-manager installed in your cluster, a self-signed CA Issuer for internal workload certificates, and an ACME (Let's Encrypt) ClusterIssuer for internet-facing TLS. You will issue certificates for an Ingress resource and verify the full chain. Part 2 covers automating certificate rotation, monitoring expiry, and handling renewal failures.

## Prerequisites

- A running Kubernetes cluster (v1.25+)
- `kubectl` with cluster-admin access
- `helm` v3 installed
- A domain name you control (for the ACME section)
- An Ingress controller installed (NGINX or equivalent)

---

## Why cert-manager

Managing TLS certificates manually in Kubernetes — creating Secrets, tracking expiry dates, renewing before they lapse — does not scale. cert-manager automates the full lifecycle: issuance, storage in Kubernetes Secrets, and renewal well before expiry.

cert-manager introduces three core concepts:

| Object | Purpose |
|---|---|
| `Issuer` | A namespaced certificate authority or ACME account |
| `ClusterIssuer` | A cluster-scoped Issuer — usable from any namespace |
| `Certificate` | A request for a TLS certificate from an Issuer |

cert-manager watches `Certificate` objects, requests certificates from the configured Issuer, and stores the result as a Kubernetes Secret containing `tls.crt` and `tls.key`. When a certificate approaches expiry (default: 30 days before), cert-manager automatically renews it.

---

## Step 1 — Install cert-manager

```bash
helm repo add jetstack https://charts.jetstack.io
helm repo update

helm install cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --create-namespace \
  --version v1.15.0 \
  --set crds.enabled=true
```

Verify all pods are running:

```bash
kubectl get pods -n cert-manager
```

Expected output:

```
NAME                                       READY   STATUS    RESTARTS
cert-manager-xxxxxxxxx-xxxxx              1/1     Running   0
cert-manager-cainjector-xxxxxxxxx-xxxxx   1/1     Running   0
cert-manager-webhook-xxxxxxxxx-xxxxx      1/1     Running   0
```

Three components:

- **cert-manager** — core controller, watches Certificate and Issuer objects
- **cainjector** — injects CA bundles into webhook configurations and CRDs
- **webhook** — validates and mutates cert-manager resources at admission

Confirm the CRDs are installed:

```bash
kubectl get crd | grep cert-manager.io
```

---

## Step 2 — Create a self-signed Issuer

A self-signed Issuer generates certificates signed by themselves — useful for internal services, development environments, and as a bootstrap step for creating a private CA.

```yaml
# selfsigned-issuer.yaml
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: selfsigned-issuer
  namespace: default
spec:
  selfSigned: {}
```

```bash
kubectl apply -f selfsigned-issuer.yaml
kubectl get issuer selfsigned-issuer -n default
```

---

## Step 3 — Create a private CA using the self-signed Issuer

A private CA lets you issue trusted certificates for all internal workloads without reaching out to an external authority. The pattern: use the self-signed Issuer to issue a CA certificate, then create a CA Issuer backed by that certificate.

```yaml
# private-ca-cert.yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: private-ca
  namespace: cert-manager
spec:
  isCA: true
  commonName: deploycraft-internal-ca
  secretName: private-ca-secret
  duration: 87600h    # 10 years
  renewBefore: 720h   # renew 30 days before expiry
  privateKey:
    algorithm: ECDSA
    size: 256
  issuerRef:
    name: selfsigned-issuer
    kind: Issuer
    group: cert-manager.io
```

```bash
kubectl apply -f private-ca-cert.yaml -n cert-manager
kubectl get certificate private-ca -n cert-manager
```

Wait for `READY: True`, then create a ClusterIssuer backed by this CA:

```yaml
# ca-clusterissuer.yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: private-ca-issuer
spec:
  ca:
    secretName: private-ca-secret
```

```bash
kubectl apply -f ca-clusterissuer.yaml
kubectl get clusterissuer private-ca-issuer
```

---

## Step 4 — Issue a certificate for an internal workload

With the private CA ClusterIssuer ready, issue a certificate for an internal service:

```yaml
# internal-service-cert.yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: internal-service-cert
  namespace: default
spec:
  secretName: internal-service-tls
  duration: 2160h      # 90 days
  renewBefore: 360h    # renew 15 days before expiry
  commonName: my-service.default.svc.cluster.local
  dnsNames:
    - my-service.default.svc.cluster.local
    - my-service.default.svc
    - my-service
  privateKey:
    algorithm: ECDSA
    size: 256
    rotationPolicy: Always
  issuerRef:
    name: private-ca-issuer
    kind: ClusterIssuer
    group: cert-manager.io
```

```bash
kubectl apply -f internal-service-cert.yaml
kubectl get certificate internal-service-cert -n default
```

Wait for `READY: True`, then inspect the resulting Secret:

```bash
kubectl get secret internal-service-tls -n default
kubectl describe secret internal-service-tls -n default
```

The Secret contains three keys:

- `tls.crt` — the signed certificate (PEM)
- `tls.key` — the private key (PEM)
- `ca.crt` — the CA certificate that signed it

Mount this Secret in your workload as a volume or use it directly in an Ingress TLS configuration.

---

## Step 5 — Configure a Let's Encrypt ACME ClusterIssuer

For internet-facing services you need a publicly trusted certificate. cert-manager supports the ACME protocol, which Let's Encrypt uses. There are two challenge types:

- **HTTP-01** — Let's Encrypt verifies domain ownership by fetching a token over HTTP. Requires your Ingress to be publicly reachable.
- **DNS-01** — Let's Encrypt verifies domain ownership via a DNS TXT record. Works for private clusters and wildcard certificates.

### HTTP-01 ClusterIssuer (staging — for testing)

Always test with the Let's Encrypt staging environment first. Staging has much higher rate limits and will not issue browser-trusted certificates — but it validates your setup before hitting production rate limits.

```yaml
# letsencrypt-staging.yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-staging
spec:
  acme:
    server: https://acme-staging-v02.api.letsencrypt.org/directory
    email: you@yourdomain.com
    privateKeySecretRef:
      name: letsencrypt-staging-key
    solvers:
      - http01:
          ingress:
            ingressClassName: nginx
```

```bash
kubectl apply -f letsencrypt-staging.yaml
kubectl get clusterissuer letsencrypt-staging
```

### HTTP-01 ClusterIssuer (production)

Once staging works, switch to production:

```yaml
# letsencrypt-prod.yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: you@yourdomain.com
    privateKeySecretRef:
      name: letsencrypt-prod-key
    solvers:
      - http01:
          ingress:
            ingressClassName: nginx
```

```bash
kubectl apply -f letsencrypt-prod.yaml
```

---

## Step 6 — Issue a certificate via Ingress annotation

The simplest way to issue a Let's Encrypt certificate is via the cert-manager Ingress annotation. cert-manager watches for the annotation and creates a `Certificate` object automatically:

```yaml
# app-ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: app-ingress
  namespace: default
  annotations:
    cert-manager.io/cluster-issuer: "letsencrypt-staging"
spec:
  ingressClassName: nginx
  tls:
    - hosts:
        - app.yourdomain.com
      secretName: app-tls
  rules:
    - host: app.yourdomain.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: app-service
                port:
                  number: 80
```

```bash
kubectl apply -f app-ingress.yaml
```

cert-manager detects the annotation, creates a `Certificate` object named `app-tls` in the `default` namespace, solves the HTTP-01 challenge, and populates the `app-tls` Secret with the signed certificate. This typically takes 60–90 seconds.

Watch the progress:

```bash
kubectl get certificate app-tls -n default -w
kubectl describe certificate app-tls -n default
```

Check the challenge status if the certificate is not becoming ready:

```bash
kubectl get challenges --all-namespaces
kubectl describe challenge <challenge-name> -n default
```

---

## Step 7 — Verify the issued certificate

Confirm the certificate is valid and inspect its details:

```bash
# Decode the certificate from the Secret
kubectl get secret app-tls -n default \
  -o jsonpath='{.data.tls\.crt}' | base64 -d | \
  openssl x509 -noout -text | grep -A2 "Validity\|Subject\|DNS"
```

Expected output shows:

```
Subject: CN=app.yourdomain.com
Validity
    Not Before: Sep  1 00:00:00 2026 GMT
    Not After : Nov 30 00:00:00 2026 GMT
X509v3 Subject Alternative Names:
    DNS:app.yourdomain.com
```

For production certificates, also verify the chain is trusted:

```bash
kubectl get secret app-tls -n default \
  -o jsonpath='{.data.tls\.crt}' | base64 -d | \
  openssl verify -CAfile /etc/ssl/certs/ca-certificates.crt
```

---

## Step 8 — DNS-01 challenge for wildcard certificates

Wildcard certificates (`*.yourdomain.com`) cover all subdomains with a single certificate. HTTP-01 cannot issue wildcards — DNS-01 is required.

Example using Cloudflare as the DNS provider:

```bash
# Create a Secret with your Cloudflare API token
kubectl create secret generic cloudflare-api-token \
  --from-literal=api-token=<your-cloudflare-api-token> \
  -n cert-manager
```

```yaml
# letsencrypt-dns01.yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-dns01
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: you@yourdomain.com
    privateKeySecretRef:
      name: letsencrypt-dns01-key
    solvers:
      - dns01:
          cloudflare:
            apiTokenSecretRef:
              name: cloudflare-api-token
              key: api-token
```

```bash
kubectl apply -f letsencrypt-dns01.yaml
```

Issue a wildcard certificate:

```yaml
# wildcard-cert.yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: wildcard-cert
  namespace: default
spec:
  secretName: wildcard-tls
  duration: 2160h
  renewBefore: 360h
  dnsNames:
    - "*.yourdomain.com"
    - "yourdomain.com"
  issuerRef:
    name: letsencrypt-dns01
    kind: ClusterIssuer
    group: cert-manager.io
```

```bash
kubectl apply -f wildcard-cert.yaml
kubectl get certificate wildcard-cert -n default -w
```

---

## What you have built

- cert-manager installed and verified with all three components healthy
- A private CA ClusterIssuer for internal workload certificates
- Let's Encrypt staging and production ClusterIssuers with HTTP-01 challenge
- A Let's Encrypt DNS-01 ClusterIssuer for wildcard certificates
- Certificates issued via both explicit `Certificate` objects and Ingress annotations
- Certificate verification via `openssl`

## Next steps

In [Part 2](/tutorials/security/automating-certificate-rotation-renewal-kubernetes-part-2/) you will configure cert-manager for automatic rotation, monitor certificate expiry with Prometheus alerts, handle renewal failures and stuck challenges, and implement a multi-issuer fallback strategy for high-availability certificate issuance.
