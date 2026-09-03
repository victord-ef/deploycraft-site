---
title: "Automating Certificate Rotation and Renewal in Kubernetes — Part 2"
date: 2026-09-01
description: "Configure cert-manager for automatic certificate rotation, monitor expiry with Prometheus alerts, debug stuck renewals, and implement a multi-issuer fallback strategy."
cluster: "Security & Hardening"
series: "Certificates"
part: 2
difficulty: "intermediate"
duration: "40 min"
tags: ["kubernetes", "cert-manager", "tls", "certificates", "security", "prometheus", "monitoring"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

In [Part 1](/tutorials/security/issuing-tls-certificates-cert-manager-part-1/) you installed cert-manager and issued certificates from self-signed, private CA, and Let's Encrypt Issuers. In Part 2 you will configure automatic certificate rotation, add Prometheus monitoring for expiry and renewal failures, debug the most common stuck renewal scenarios, and set up a multi-issuer fallback so a single Issuer failure does not block certificate issuance cluster-wide.

## Prerequisites

- Completed [Part 1](/tutorials/security/issuing-tls-certificates-cert-manager-part-1/) — cert-manager installed, at least one ClusterIssuer configured, at least one Certificate issued
- Prometheus and Grafana running in the cluster (or access to a metrics endpoint)

---

## How cert-manager renewal works

cert-manager computes a renewal window from two Certificate fields:

```
renewBefore = how long before expiry to start renewing
```

If a certificate has `duration: 2160h` (90 days) and `renewBefore: 360h` (15 days), cert-manager will trigger renewal 75 days after issuance — 15 days before it expires. This is the renewal window.

cert-manager checks all managed certificates on a 10-second polling loop. When a certificate enters its renewal window, cert-manager creates a new `CertificateRequest` and an `Order` (for ACME) or contacts the CA directly. On success, the certificate Secret is updated in-place. Workloads that mount the Secret receive the new certificate without restart — provided they watch for Secret changes or the kubelet refreshes the projected volume.

---

## Step 1 — Verify rotation policy

Set `rotationPolicy: Always` on the private key to ensure the private key is rotated with every renewal, not just the certificate:

```yaml
spec:
  privateKey:
    rotationPolicy: Always
    algorithm: ECDSA
    size: 256
```

Without `rotationPolicy: Always`, cert-manager reuses the existing private key on renewal. This is acceptable but means long-lived key material. For high-security environments, rotating the key with every renewal is best practice.

Apply to an existing Certificate:

```bash
kubectl patch certificate internal-service-cert -n default \
  --type merge \
  -p '{"spec":{"privateKey":{"rotationPolicy":"Always"}}}'
```

---

## Step 2 — Trigger a manual renewal

To test the renewal path without waiting for the renewal window:

```bash
# Trigger immediate renewal using cmctl
kubectl cert-manager renew internal-service-cert -n default
```

If `cmctl` is not installed:

```bash
# Install cmctl
curl -LO https://github.com/cert-manager/cert-manager/releases/latest/download/cmctl_linux_amd64
chmod +x cmctl_linux_amd64
mv cmctl_linux_amd64 /usr/local/bin/cmctl
```

Watch the renewal complete:

```bash
kubectl get certificate internal-service-cert -n default -w
```

The certificate will briefly show `READY: False` while the new certificate is being issued, then return to `READY: True` with an updated `Not After` date.

Verify the new expiry:

```bash
kubectl get secret internal-service-tls -n default \
  -o jsonpath='{.data.tls\.crt}' | base64 -d | \
  openssl x509 -noout -dates
```

---

## Step 3 — Monitor certificate expiry with Prometheus

cert-manager exposes Prometheus metrics on port 9402 of the cert-manager controller pod. Key metrics for expiry monitoring:

| Metric | Description |
|---|---|
| `certmanager_certificate_expiration_timestamp_seconds` | Unix timestamp of certificate expiry |
| `certmanager_certificate_ready_status` | 1 if ready, 0 if not |
| `certmanager_certificate_renewal_timestamp_seconds` | When cert-manager will next attempt renewal |

Scrape these metrics by adding a `ServiceMonitor` (if using the Prometheus Operator):

```yaml
# cert-manager-servicemonitor.yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: cert-manager
  namespace: cert-manager
  labels:
    release: prometheus
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: cert-manager
      app.kubernetes.io/component: controller
  endpoints:
    - port: tcp-prometheus-servicemonitor
      interval: 60s
      path: /metrics
```

```bash
kubectl apply -f cert-manager-servicemonitor.yaml
```

---

## Step 4 — Set up expiry alerts in Prometheus

Add alerting rules that fire before a certificate expires. These two alerts cover the critical windows:

```yaml
# cert-manager-alerts.yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: cert-manager-alerts
  namespace: cert-manager
  labels:
    release: prometheus
spec:
  groups:
    - name: cert-manager.certificates
      rules:
        - alert: CertificateExpiryWarning
          expr: |
            (certmanager_certificate_expiration_timestamp_seconds - time()) < (21 * 24 * 3600)
          for: 1h
          labels:
            severity: warning
          annotations:
            summary: "Certificate expiring within 21 days"
            description: >
              Certificate {{ $labels.name }} in namespace {{ $labels.namespace }}
              expires in {{ $value | humanizeDuration }}.

        - alert: CertificateExpiryCritical
          expr: |
            (certmanager_certificate_expiration_timestamp_seconds - time()) < (7 * 24 * 3600)
          for: 15m
          labels:
            severity: critical
          annotations:
            summary: "Certificate expiring within 7 days"
            description: >
              Certificate {{ $labels.name }} in namespace {{ $labels.namespace }}
              expires in {{ $value | humanizeDuration }}. Immediate action required.

        - alert: CertificateNotReady
          expr: certmanager_certificate_ready_status == 0
          for: 10m
          labels:
            severity: warning
          annotations:
            summary: "Certificate not ready"
            description: >
              Certificate {{ $labels.name }} in namespace {{ $labels.namespace }}
              has not been ready for over 10 minutes.
```

```bash
kubectl apply -f cert-manager-alerts.yaml
```

The `CertificateExpiryWarning` at 21 days gives your team time to investigate if renewal is failing before reaching the critical window. `CertificateNotReady` catches certificates stuck in a non-ready state — which is the primary symptom of a renewal failure.

---

## Step 5 — Grafana dashboard queries

Useful Grafana panel queries for certificate visibility:

```promql
# Days until expiry for all certificates
(certmanager_certificate_expiration_timestamp_seconds - time()) / 86400

# Certificates not ready
certmanager_certificate_ready_status == 0

# Certificates in renewal window (renewal timestamp passed but not yet renewed)
certmanager_certificate_renewal_timestamp_seconds < time()
and certmanager_certificate_ready_status == 1

# Count of ready vs not-ready certificates
count by (ready) (certmanager_certificate_ready_status)
```

Build a table panel using the expiry query, sorted ascending — the certificates closest to expiry appear at the top. This is your daily certificate health view.

---

## Step 6 — Debug stuck renewals

When a certificate is not renewing, the diagnostic sequence is:

**1. Check the Certificate status**

```bash
kubectl describe certificate <cert-name> -n <namespace>
```

Look at the `Status.Conditions` block. Common conditions and their meaning:

| Condition | Status | Meaning |
|---|---|---|
| `Ready` | `False` | Certificate is not issued or has failed |
| `Issuing` | `True` | Renewal is in progress |

**2. Check the CertificateRequest**

```bash
kubectl get certificaterequests -n <namespace>
kubectl describe certificaterequest <cr-name> -n <namespace>
```

The CertificateRequest is created by cert-manager for each issuance attempt. Its `Status.Conditions` shows whether the Issuer accepted or rejected the request.

**3. Check Orders and Challenges (ACME only)**

```bash
kubectl get orders --all-namespaces
kubectl get challenges --all-namespaces
kubectl describe challenge <challenge-name> -n <namespace>
```

A Challenge stuck in `pending` state means the ACME server could not verify domain ownership. Common causes:

| Cause | Fix |
|---|---|
| HTTP-01: Ingress not reachable | Verify the Ingress is publicly accessible on port 80 |
| HTTP-01: Wrong ingressClassName | Match the `ingressClassName` to your actual Ingress controller |
| DNS-01: API credentials expired | Rotate the Cloudflare/Route53 API token and update the Secret |
| DNS-01: Propagation timeout | Increase `dns01RecursiveNameserversOnly` or add propagation wait |
| Rate limit hit | Check Let's Encrypt rate limit dashboard; use staging to test |

**4. Check cert-manager controller logs**

```bash
kubectl logs -n cert-manager \
  -l app.kubernetes.io/component=controller \
  --tail=100 -f | grep -i "error\|warning\|failed"
```

The controller logs include the Issuer response, the challenge URL, and the specific error from the CA. This is the definitive source for diagnosing issuance failures.

**5. Force a retry after fixing the root cause**

```bash
# Delete the stuck CertificateRequest to trigger a fresh issuance attempt
kubectl delete certificaterequest <cr-name> -n <namespace>

# Or use cmctl to force renewal
kubectl cert-manager renew <cert-name> -n <namespace>
```

---

## Step 7 — Multi-issuer fallback strategy

If your primary Issuer fails — Let's Encrypt is unavailable, Vault is down, the CA cert has expired — certificates entering their renewal window will be stuck. A fallback strategy uses your private CA Issuer as a backup.

The pattern: define the primary Issuer on the Certificate, and maintain a runbook (or automated process) to patch the `issuerRef` to the fallback Issuer if the primary is unhealthy.

A more automated approach uses a custom controller or a Kyverno mutating policy that watches Certificate objects and substitutes the Issuer based on cluster conditions. For most teams, a manual runbook is sufficient — the alert fires 21 days before expiry, which is enough time to diagnose and switch issuers.

To switch an existing Certificate to a different Issuer:

```bash
kubectl patch certificate <cert-name> -n <namespace> \
  --type merge \
  -p '{"spec":{"issuerRef":{"name":"private-ca-issuer","kind":"ClusterIssuer"}}}'
```

cert-manager immediately triggers a new issuance using the updated Issuer. The workload receives an internally-signed certificate instead of a Let's Encrypt certificate — acceptable for continuity while the primary Issuer is restored.

---

## Step 8 — Ensuring workloads pick up renewed certificates

cert-manager updates the TLS Secret in-place on renewal. Whether the workload picks up the new certificate depends on how it consumes the Secret:

| Mount type | Behaviour |
|---|---|
| Volume mount (projected) | kubelet refreshes the file automatically (within `syncFrequency`, default 1 min) |
| Volume mount (subPath) | Not automatically refreshed — requires pod restart |
| Environment variable | Never refreshed — requires pod restart |
| Direct Secret read via API | Application controls refresh timing |

For workloads that use `subPath` mounts or environment variables, trigger a rolling restart after renewal:

```bash
kubectl rollout restart deployment <deployment-name> -n <namespace>
```

Automate this with a `CronJob` that watches for Secret updates and triggers restarts, or use a tool like `Reloader` which watches Secrets and ConfigMaps and automatically restarts Deployments when they change:

```bash
helm install reloader stakater/reloader \
  --namespace reloader \
  --create-namespace
```

Annotate your Deployment to enable automatic restart on Secret change:

```yaml
metadata:
  annotations:
    secret.reloader.stakater.com/reload: "internal-service-tls"
```

---

## What you have built

- `rotationPolicy: Always` configured for private key rotation on every renewal
- Manual renewal tested with `cmctl`
- Prometheus `ServiceMonitor` scraping cert-manager metrics
- Alerting rules for 21-day warning, 7-day critical, and not-ready conditions
- Grafana queries for daily certificate health visibility
- A structured diagnostic sequence for stuck renewals — certificate → request → order → challenge → logs
- Multi-issuer fallback strategy with Issuer switching procedure
- Workload restart automation with Reloader for `subPath` mounted secrets

Your certificate lifecycle is now fully automated: cert-manager issues, rotates, and renews certificates without manual intervention, alerts fire before expiry windows become critical, and your team has a clear diagnostic path when something goes wrong.
