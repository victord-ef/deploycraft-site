---
title: "Kubernetes Ingress with TLS via cert-manager"
date: 2026-11-12
description: "nginx Ingress manifest with TLS termination, automated Let's Encrypt certificate provisioning via cert-manager annotations, HTTP-to-HTTPS redirect, and security headers."
lang: "Kubernetes"
tags: ["kubernetes", "ingress", "tls", "cert-manager", "lets-encrypt", "nginx", "https"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-11-12"
draft: false
---

## When to use this

When exposing a service to the internet over HTTPS with an automatically renewed Let's Encrypt certificate. cert-manager handles certificate issuance and renewal — no manual `certbot` runs, no cron jobs, no expiry surprises.

## Prerequisites

- `ingress-nginx` controller installed in the cluster.
- `cert-manager` installed (`kubectl apply -f https://github.com/cert-manager/cert-manager/releases/latest/download/cert-manager.yaml`).
- A DNS A record pointing your domain to the Ingress controller's external IP.

## Snippet

### ClusterIssuer — Let's Encrypt production

```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: ops@example.com          # receives expiry warnings if renewal fails
    privateKeySecretRef:
      name: letsencrypt-prod-account-key
    solvers:
      - http01:
          ingress:
            ingressClassName: nginx
```

### ClusterIssuer — Let's Encrypt staging (use first to avoid rate limits)

```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-staging
spec:
  acme:
    server: https://acme-staging-v02.api.letsencrypt.org/directory
    email: ops@example.com
    privateKeySecretRef:
      name: letsencrypt-staging-account-key
    solvers:
      - http01:
          ingress:
            ingressClassName: nginx
```

### Ingress with automatic TLS certificate

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app
  namespace: my-app
  annotations:
    # Trigger cert-manager to provision a certificate for the listed hosts
    cert-manager.io/cluster-issuer: letsencrypt-prod

    # nginx-specific — redirect HTTP to HTTPS
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    nginx.ingress.kubernetes.io/force-ssl-redirect: "true"

    # Security headers
    nginx.ingress.kubernetes.io/configuration-snippet: |
      add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
      add_header X-Content-Type-Options "nosniff" always;
      add_header X-Frame-Options "DENY" always;
      add_header Referrer-Policy "strict-origin-when-cross-origin" always;

    # Rate limiting — 100 requests per minute per IP
    nginx.ingress.kubernetes.io/limit-rps: "100"

spec:
  ingressClassName: nginx

  tls:
    - hosts:
        - app.example.com
      secretName: my-app-tls      # cert-manager writes the certificate here

  rules:
    - host: app.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: my-app
                port:
                  number: 80
```

### Multi-host Ingress — separate certificates per domain

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app-multi
  namespace: my-app
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
spec:
  ingressClassName: nginx
  tls:
    - hosts:
        - app.example.com
      secretName: app-example-com-tls
    - hosts:
        - api.example.com
      secretName: api-example-com-tls
  rules:
    - host: app.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: frontend
                port:
                  number: 80
    - host: api.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: api
                port:
                  number: 8080
```

### Certificate resource — explicit (without Ingress annotation)

```yaml
# Use when you need more control over certificate properties
# or when the workload terminates TLS itself (not at the Ingress)
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: my-app-tls
  namespace: my-app
spec:
  secretName: my-app-tls
  issuerRef:
    name: letsencrypt-prod
    kind: ClusterIssuer
  commonName: app.example.com
  dnsNames:
    - app.example.com
    - www.app.example.com
  duration: 2160h       # 90 days (Let's Encrypt maximum)
  renewBefore: 360h     # renew 15 days before expiry
```

**Key decisions:**

| Decision | Why it matters |
|---|---|
| Use staging issuer first | Let's Encrypt production has a rate limit of 5 duplicate certificate requests per week. Using staging validates the entire ACME flow with no rate limit risk — switch to production only after staging succeeds. |
| `ClusterIssuer` over `Issuer` | `Issuer` is namespace-scoped — a separate Issuer resource per namespace. `ClusterIssuer` is available to all namespaces in the cluster from one resource. Use `Issuer` only if different namespaces need different ACME accounts. |
| `secretName` in `tls:` matches cert-manager output | cert-manager watches for Ingress resources with its annotation, creates a `Certificate` resource automatically, and writes the resulting TLS secret to the name in `secretName`. The name must be unique per namespace. |
| `force-ssl-redirect: "true"` in addition to `ssl-redirect` | `ssl-redirect` redirects HTTP to HTTPS when TLS is configured. `force-ssl-redirect` redirects unconditionally, even without TLS configured — prevents accidentally serving HTTP if TLS is misconfigured. |
| HSTS `max-age: 31536000` | Tells browsers to only connect over HTTPS for 1 year. Set `includeSubDomains` only if all subdomains have valid certificates — a missing certificate on a subdomain blocks access to it entirely. |
| `renewBefore: 360h` (15 days) | Let's Encrypt certificates last 90 days. cert-manager's default is to renew 30 days before expiry. 15 days is tighter but provides two full renewal windows before expiry if the first attempt fails. |

## Verify it worked

```bash
# Watch certificate issuance
kubectl describe certificate my-app-tls -n my-app
kubectl get certificaterequest -n my-app
kubectl get order -n my-app              # ACME challenge order

# Confirm the TLS secret was created
kubectl get secret my-app-tls -n my-app

# Check certificate expiry
kubectl get certificate my-app-tls -n my-app \
  -o jsonpath='{.status.notAfter}'

# Test HTTPS from outside the cluster
curl -v https://app.example.com
# Expected: TLS handshake with Let's Encrypt certificate, HTTP 200

# Confirm HTTP redirects to HTTPS
curl -I http://app.example.com
# Expected: 308 Permanent Redirect to https://
```
