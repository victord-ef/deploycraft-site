---
title: "Installing and Meshing Workloads with Linkerd — Part 1"
date: 2026-09-03
description: "Install Linkerd via the CLI, inject the proxy into existing workloads with zero downtime, verify mTLS with the viz dashboard, configure the multicluster extension, and harden the installation with an external trust anchor and production-grade control plane settings."
cluster: "Service Mesh"
series: "Linkerd"
part: 1
difficulty: "intermediate"
duration: "40 min"
tags: ["service-mesh", "linkerd", "kubernetes", "mtls", "observability", "networking", "devsecops"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

By the end of this tutorial you will have Linkerd installed in a Kubernetes cluster with the viz and multicluster extensions, all application workloads injected with the linkerd2-proxy sidecar, mTLS verified between every service pair using the dashboard and CLI, and the installation hardened with an external trust anchor and production resource settings. Part 2 builds on this by configuring Linkerd's authorization policy model, per-route traffic policies, and retries.

## Prerequisites

- A Kubernetes cluster (1.28+) with `kubectl` configured
- `step` CLI installed — used to generate the trust anchor certificate (`brew install step`)
- Familiarity with [Service Mesh Foundations Part 1](/tutorials/service-mesh/introduction-service-mesh-concepts-architecture-kubernetes-part-1/) — sidecar model, mTLS, SPIFFE identity

---

## Step 1 — Install the Linkerd CLI

```bash
# Official install script (Linux / macOS)
curl --proto '=https' --tlsv1.2 -sSfL https://run.linkerd.io/install | sh

# Add to PATH
export PATH=$HOME/.linkerd2/bin:$PATH

# Or install a specific version
curl --proto '=https' --tlsv1.2 -sSfL \
  https://github.com/linkerd/linkerd2/releases/download/stable-2.15.0/linkerd2-cli-stable-2.15.0-linux-amd64 \
  -o linkerd
chmod +x linkerd
sudo mv linkerd /usr/local/bin/

# macOS via Homebrew
brew install linkerd

# Verify
linkerd version --client
# Client version: stable-2.15.0
```

---

## Step 2 — Generate a trust anchor (external CA)

Linkerd's identity controller signs workload certificates using a trust anchor. In production, this anchor must be an offline root CA — not the default self-signed certificate that expires in one year.

Using `step` to generate a long-lived offline root CA:

```bash
# Generate the root CA (trust anchor) — keep the key offline after this step
step certificate create root.linkerd.cluster.local ca.crt ca.key \
  --profile root-ca \
  --no-password \
  --insecure \
  --not-after 87600h    # 10 years — the trust anchor is offline, not rotated frequently

# Generate an intermediate CA signed by the root — this is what Linkerd uses
step certificate create identity.linkerd.cluster.local issuer.crt issuer.key \
  --profile intermediate-ca \
  --not-after 8760h \    # 1 year — rotate annually
  --no-password \
  --insecure \
  --ca ca.crt \
  --ca-key ca.key

# Verify the chain
step certificate verify issuer.crt --roots ca.crt
# Certificate is valid

ls -1
# ca.crt       ← trust anchor (root CA) — distribute to all clusters, keep key offline
# ca.key       ← root CA private key — store in hardware security module or offline vault
# issuer.crt   ← intermediate CA certificate — provide to Linkerd
# issuer.key   ← intermediate CA private key — provide to Linkerd
```

---

## Step 3 — Preflight check and install the CRDs

```bash
# Run preflight checks with the external trust anchor
linkerd check --pre

# Install Linkerd CRDs first (separate step from control plane)
linkerd install --crds | kubectl apply -f -

# Verify CRDs
kubectl get crds | grep linkerd.io
# authorizationpolicies.policy.linkerd.io
# httproutes.policy.linkerd.io
# meshtlsauthentications.policy.linkerd.io
# networkauthentications.policy.linkerd.io
# serverauthorizations.policy.linkerd.io
# servers.policy.linkerd.io
# serviceprofiles.linkerd.io
```

---

## Step 4 — Install the Linkerd control plane

```bash
# Install with the external trust anchor
linkerd install \
  --identity-trust-anchors-file ca.crt \
  --identity-issuer-certificate-file issuer.crt \
  --identity-issuer-key-file issuer.key \
  | kubectl apply -f -

# Wait for the control plane to become ready
linkerd check

# ── linkerd-config ──────────────────────────────────────
# √ control plane namespace exists
# √ control plane ClusterRoles exist
# √ control plane ClusterRoleBindings exist
# √ control plane ServiceAccounts exist
# √ control plane CustomResourceDefinitions exist
# √ control plane MutatingWebhookConfigurations exist
# √ control plane ValidatingWebhookConfigurations exist
# ── linkerd-identity ────────────────────────────────────
# √ certificate config is valid
# √ trust anchors are using supported crypto algorithm
# √ trust anchor certificate is within its validity period
# √ issuer certificate is within its validity period
# √ issuer cert is signed by the trust anchor
# ── linkerd-control-plane-proxy ─────────────────────────
# √ control plane proxies are healthy
# √ control plane proxies are up-to-date
# Status check results are √
```

---

## Step 5 — Install the viz extension

The viz extension installs Prometheus, the dashboard, tap (request-level inspection), and the metrics API:

```bash
# Install viz
linkerd viz install | kubectl apply -f -

# Wait and verify
linkerd viz check

# Open the dashboard (opens a browser window)
linkerd viz dashboard &

# The dashboard shows:
# - Service topology graph with RPS, success rate, and latency on each edge
# - mTLS lock icons confirming encrypted connections
# - Per-namespace and per-deployment drill-down
```

For production, replace the built-in Prometheus with your existing cluster Prometheus:

```bash
linkerd viz install \
  --set prometheus.enabled=false \
  --set prometheusUrl=http://prometheus-operated.monitoring:9090 \
  | kubectl apply -f -
```

---

## Step 6 — Inject workloads

Linkerd injection is controlled by a namespace annotation or a pod annotation. Annotate at the namespace level to inject all pods automatically:

```bash
# Inject an entire namespace (rolling restart required for existing pods)
kubectl annotate namespace my-app linkerd.io/inject=enabled
kubectl annotate namespace payments linkerd.io/inject=enabled
kubectl annotate namespace checkout linkerd.io/inject=enabled

# Explicitly exclude system namespaces
kubectl annotate namespace kube-system linkerd.io/inject=disabled
kubectl annotate namespace kube-public linkerd.io/inject=disabled

# Restart existing pods to receive the sidecar
kubectl rollout restart deployment -n my-app
kubectl rollout restart deployment -n payments
kubectl rollout restart deployment -n checkout
```

Verify injection:

```bash
# Each pod should show 2/2 (app + linkerd-proxy)
kubectl get pods -n my-app
# NAME            READY   STATUS    RESTARTS
# my-app-xxx      2/2     Running   0

# Confirm the proxy container is present
kubectl get pods -n my-app -o jsonpath='{.items[*].spec.initContainers[*].name}'
# linkerd-init   ← iptables setup init container

kubectl get pods -n my-app -o jsonpath='{.items[*].spec.containers[*].name}'
# my-app linkerd-proxy   ← sidecar present
```

Check injection status across the cluster:

```bash
linkerd check --proxy -n my-app
# √ all pods are running
# √ all pods are injected
# √ all proxies are healthy
# √ all proxies are up-to-date
```

---

## Step 7 — Verify mTLS between services

```bash
# Show mTLS status for all edges in a namespace
linkerd viz edges deployment -n my-app
# SRC            DST          SRC_NS   DST_NS    SECURED
# checkout-svc   my-app       checkout my-app    ✔
# my-app         payment-svc  my-app   payments  ✔

# Live tap — inspect individual requests including mTLS metadata
linkerd viz tap deployment/my-app -n my-app
# req id=0:1 proxy=in  src=10.0.1.5:52431 dst=10.0.2.3:8080
#   :method=GET :authority=my-app.my-app:8080 :path=/api/health
# rsp id=0:1 proxy=in  src=10.0.1.5:52431 dst=10.0.2.3:8080
#   :status=200 latency=2ms

# Get per-second stats for a deployment
linkerd viz stat deployment -n my-app
# NAME      MESHED   SUCCESS   RPS     LATENCY_P50   LATENCY_P95   LATENCY_P99
# my-app    3/3      100.00%   12.3    1ms           4ms           8ms
```

---

## Step 8 — Control plane high availability

For production, run the control plane in HA mode with multiple replicas and pod disruption budgets:

```bash
linkerd install \
  --identity-trust-anchors-file ca.crt \
  --identity-issuer-certificate-file issuer.crt \
  --identity-issuer-key-file issuer.key \
  --set controllerReplicas=3 \
  --set identity.issuer.issuanceLifetime=24h0m0s \
  --set identity.issuer.clockSkewAllowance=20s \
  | kubectl apply -f -
```

Or use a Helm values file for full control:

```yaml
# linkerd-values.yaml
controllerReplicas: 3

proxy:
  resources:
    cpu:
      request: 100m
      limit: 1000m
    memory:
      request: 20Mi
      limit: 250Mi
  logLevel: warn

identity:
  issuer:
    issuanceLifetime: 24h0m0s
    clockSkewAllowance: 20s
    scheme: kubernetes.io/tls

proxyInit:
  resources:
    cpu:
      request: 10m
      limit: 100m
    memory:
      request: 10Mi
      limit: 50Mi

enablePodAntiAffinity: true    # spread control plane pods across nodes
```

```bash
helm repo add linkerd https://helm.linkerd.io/stable
helm repo update

helm install linkerd-crds linkerd/linkerd-crds -n linkerd --create-namespace

helm install linkerd-control-plane linkerd/linkerd-control-plane \
  -n linkerd \
  -f linkerd-values.yaml \
  --set-file identityTrustAnchorsPEM=ca.crt \
  --set-file identity.issuer.tls.crtPEM=issuer.crt \
  --set-file identity.issuer.tls.keyPEM=issuer.key
```

---

## Step 9 — Install the multicluster extension

The multicluster extension enables service mirroring across clusters — services in cluster A can call services in cluster B as if they were local:

```bash
# Install on both clusters
linkerd multicluster install | kubectl apply -f -

# Link cluster B to cluster A
# (run on cluster A with cluster B's kubeconfig available)
linkerd multicluster link \
  --context=cluster-b \
  --cluster-name=cluster-b \
  | kubectl apply -f -

# Verify the link
linkerd multicluster check

# Export a service from cluster B so cluster A can reach it
kubectl label svc payment-svc -n payments \
  mirror.linkerd.io/exported=true \
  --context=cluster-b

# Cluster A now has a mirrored Service:
kubectl get svc -n payments --context=cluster-a
# NAME                        TYPE        CLUSTER-IP
# payment-svc                 ClusterIP   10.96.1.10     ← local
# payment-svc-cluster-b       ClusterIP   10.96.1.11     ← mirrored from cluster B
```

Traffic to `payment-svc-cluster-b` is automatically mTLS-secured end-to-end between clusters using the shared trust anchor.

---

## Step 10 — Upgrade Linkerd

Linkerd upgrades use the same CLI/Helm flow as installation:

```bash
# Check the current version and available upgrades
linkerd version

# CLI upgrade
curl --proto '=https' --tlsv1.2 -sSfL https://run.linkerd.io/install | \
  LINKERD2_VERSION=stable-2.15.1 sh

# Upgrade CRDs first
linkerd upgrade --crds | kubectl apply -f -

# Upgrade the control plane
linkerd upgrade \
  --identity-trust-anchors-file ca.crt \
  --identity-issuer-certificate-file issuer.crt \
  --identity-issuer-key-file issuer.key \
  | kubectl apply -f -

# Verify upgrade
linkerd check

# Restart injected workloads to pick up the new proxy version
kubectl rollout restart deployment -n my-app
kubectl rollout restart deployment -n payments
```

For Helm-managed installs:

```bash
helm repo update
helm upgrade linkerd-control-plane linkerd/linkerd-control-plane \
  -n linkerd \
  -f linkerd-values.yaml \
  --set-file identityTrustAnchorsPEM=ca.crt \
  --set-file identity.issuer.tls.crtPEM=issuer.crt \
  --set-file identity.issuer.tls.keyPEM=issuer.key
```

---

## What you have built

- Linkerd CLI installed (script, specific version, and Homebrew paths)
- A 10-year offline root CA and 1-year intermediate issuer CA using `step` — the correct trust anchor model for production
- Linkerd CRDs installed separately, then control plane with external trust anchor
- `linkerd check` health verification covering config, identity, and proxy health
- Viz extension installed with built-in and external Prometheus options
- Namespace-level injection via `linkerd.io/inject=enabled` annotation with system namespace exclusions
- mTLS verification via `linkerd viz edges`, `linkerd viz tap`, and `linkerd viz stat`
- HA control plane: 3 replicas, pod anti-affinity, proxy resource limits via Helm values file
- Multicluster extension: cross-cluster service mirroring with end-to-end mTLS using the shared trust anchor
- CLI and Helm upgrade paths with CRD-first ordering and post-upgrade proxy rollout

In [Part 2](/tutorials/service-mesh/securing-service-communication-linkerd-mtls-authorization-policies-part-2/) you will configure Linkerd's authorization policy model — `Server`, `HTTPRoute`, `MeshTLSAuthentication`, and `AuthorizationPolicy` — to enforce deny-by-default zero-trust communication between services, add per-route retries and timeouts via `ServiceProfile`, and observe policy decisions in the viz dashboard.
