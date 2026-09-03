---
title: "Securing Service-to-Service Communication with Linkerd mTLS and Authorization Policies — Part 2"
date: 2026-09-03
description: "Configure Linkerd's policy model to enforce zero-trust service communication: Server resources, MeshTLSAuthentication, HTTPRoute-based AuthorizationPolicy, per-route retries and timeouts via ServiceProfile, and policy observability in the viz dashboard."
cluster: "Service Mesh"
series: "Linkerd"
part: 2
difficulty: "intermediate"
duration: "45 min"
tags: ["service-mesh", "linkerd", "kubernetes", "mtls", "authorization", "zero-trust", "networking", "devsecops", "policy"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

In [Part 1](/tutorials/service-mesh/installing-meshing-workloads-linkerd-part-1/) you installed Linkerd and verified mTLS across all workloads. In Part 2 you will configure Linkerd's authorization policy to enforce deny-by-default service-to-service communication: `Server` resources that define what ports accept traffic, `MeshTLSAuthentication` to restrict callers by SPIFFE identity, `HTTPRoute`-scoped `AuthorizationPolicy` for method and path-level rules, per-route retries and timeouts via `ServiceProfile`, and how to observe all of this in the viz dashboard and CLI.

## Prerequisites

- Completed [Part 1](/tutorials/service-mesh/installing-meshing-workloads-linkerd-part-1/) — Linkerd installed, all workloads injected, mTLS verified
- At least two communicating services in the mesh (e.g., `checkout-svc` calling `payment-svc`)
- `linkerd` CLI and `kubectl` configured

---

## Step 1 — Linkerd's policy model

Linkerd's authorization policy uses four resources working together:

```
Server              — defines a port on a set of pods that accepts inbound traffic
      │
MeshTLSAuthentication  — identifies which SPIFFE identities (ServiceAccounts) are trusted callers
NetworkAuthentication  — identifies callers by CIDR (for non-mesh traffic, e.g. monitoring scrapers)
      │
HTTPRoute           — (optional) narrows policy to specific HTTP methods and paths
      │
AuthorizationPolicy — binds a Server (+ optional HTTPRoute) to one or more Authentications
                      Result: ALLOW or DENY
```

Without any policy resources, Linkerd's default mode is **`all-unauthenticated`**: all traffic is allowed regardless of identity. Configuring a `Server` on a port switches that port to **deny-by-default** — only explicitly authorised identities can connect.

---

## Step 2 — Set the default policy

Configure Linkerd to default to deny for all meshed workloads:

```bash
# Set cluster-wide default to deny (requires all Servers to be explicitly authorised)
linkerd upgrade \
  --set proxy.defaultInboundPolicy=deny \
  | kubectl apply -f -
```

Or patch the control plane ConfigMap directly:

```yaml
# linkerd-config ConfigMap patch
data:
  values: |
    proxy:
      defaultInboundPolicy: deny
```

Available default policies:

| Policy | Behaviour |
|---|---|
| `all-unauthenticated` | Allow all traffic (default — equivalent to no policy) |
| `all-authenticated` | Allow only mTLS-authenticated traffic (any mesh identity) |
| `cluster-authenticated` | Allow only identities within this cluster's trust domain |
| `deny` | Deny all traffic — require explicit `AuthorizationPolicy` per `Server` |

For a zero-trust posture, set `deny` and authorise each service explicitly. For a transitional posture during mesh adoption, use `all-authenticated` to ensure all traffic is mTLS-encrypted while building out per-service policies.

---

## Step 3 — Define a Server resource

A `Server` selects pods and a port, converting that port from the default policy to `deny`. All inbound traffic to that port is blocked until an `AuthorizationPolicy` explicitly allows it:

```yaml
# payment-server.yaml
apiVersion: policy.linkerd.io/v1beta3
kind: Server
metadata:
  name: payment-svc-http
  namespace: payments
spec:
  podSelector:
    matchLabels:
      app: payment-svc    # targets pods with this label
  port: 8080
  proxyProtocol: HTTP/2   # HTTP/1, HTTP/2, gRPC, opaque, TLS, unknown
```

Apply it:

```bash
kubectl apply -f payment-server.yaml

# Immediately after: all traffic to payment-svc:8080 is denied
# Callers will see connection refused or 403 until AuthorizationPolicy is added

# Verify the Server is created
kubectl get servers -n payments
# NAME                 PORT   PROXY PROTOCOL
# payment-svc-http     8080   HTTP/2
```

---

## Step 4 — Authenticate callers by SPIFFE identity

`MeshTLSAuthentication` declares which SPIFFE identities (Kubernetes ServiceAccounts) are permitted to call a `Server`:

```yaml
# checkout-authn.yaml
apiVersion: policy.linkerd.io/v1beta3
kind: MeshTLSAuthentication
metadata:
  name: checkout-svc-identity
  namespace: payments
spec:
  identities:
    # SPIFFE identity format: <serviceaccount>.<namespace>.serviceaccount.identity.linkerd.cluster.local
    - "checkout-svc.checkout.serviceaccount.identity.linkerd.cluster.local"
```

Bind the authentication to the `Server` via an `AuthorizationPolicy`:

```yaml
# checkout-to-payment-authz.yaml
apiVersion: policy.linkerd.io/v1beta3
kind: AuthorizationPolicy
metadata:
  name: allow-checkout-to-payment
  namespace: payments
spec:
  targetRef:
    group: policy.linkerd.io
    kind: Server
    name: payment-svc-http
  requiredAuthenticationRefs:
    - name: checkout-svc-identity
      kind: MeshTLSAuthentication
      group: policy.linkerd.io
```

```bash
kubectl apply -f checkout-authn.yaml
kubectl apply -f checkout-to-payment-authz.yaml

# Verify checkout-svc can now reach payment-svc
kubectl exec -n checkout deploy/checkout-svc -- \
  curl -s http://payment-svc.payments:8080/health
# {"status":"ok"}   ← allowed

# Verify another service cannot (no AuthorizationPolicy for it)
kubectl exec -n my-app deploy/my-app -- \
  curl -s http://payment-svc.payments:8080/health
# curl: (52) Empty reply from server   ← denied by Server policy
```

---

## Step 5 — Allow non-mesh traffic (monitoring scrapers)

Prometheus scrapes metrics endpoints from outside the mesh (or without a sidecar). Use `NetworkAuthentication` to allow traffic from specific CIDRs:

```yaml
# prometheus-authn.yaml
apiVersion: policy.linkerd.io/v1beta3
kind: NetworkAuthentication
metadata:
  name: prometheus-scraper
  namespace: payments
spec:
  networks:
    - cidr: 10.0.0.0/8       # cluster pod CIDR — covers Prometheus pods
      except:
        - 10.0.1.0/24        # exclude specific ranges if needed
```

```yaml
# prometheus-authz.yaml
apiVersion: policy.linkerd.io/v1beta3
kind: AuthorizationPolicy
metadata:
  name: allow-prometheus-scrape
  namespace: payments
spec:
  targetRef:
    group: policy.linkerd.io
    kind: Server
    name: payment-svc-admin    # a separate Server on the metrics port (4191)
  requiredAuthenticationRefs:
    - name: prometheus-scraper
      kind: NetworkAuthentication
      group: policy.linkerd.io
```

Define a separate `Server` for the admin/metrics port:

```yaml
# payment-server-admin.yaml
apiVersion: policy.linkerd.io/v1beta3
kind: Server
metadata:
  name: payment-svc-admin
  namespace: payments
spec:
  podSelector:
    matchLabels:
      app: payment-svc
  port: 4191    # Linkerd admin port (metrics, readiness, liveness)
  proxyProtocol: HTTP/1
```

Separating the application port (`Server` on 8080) from the admin port (`Server` on 4191) lets you apply different policies to each: strict mTLS identity for application traffic, CIDR-based network authentication for Prometheus.

---

## Step 6 — HTTPRoute-scoped authorization

For HTTP services, scope authorization to specific methods and paths rather than the entire port. This enables fine-grained rules: the `checkout-svc` may call `POST /charge` but not `DELETE /refund`.

Define an `HTTPRoute` targeting the `Server`:

```yaml
# payment-routes.yaml
apiVersion: policy.linkerd.io/v1beta3
kind: HTTPRoute
metadata:
  name: payment-charge-route
  namespace: payments
spec:
  parentRefs:
    - name: payment-svc-http    # references the Server
      kind: Server
      group: policy.linkerd.io
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /api/v1/charge
          method: POST
---
apiVersion: policy.linkerd.io/v1beta3
kind: HTTPRoute
metadata:
  name: payment-refund-route
  namespace: payments
spec:
  parentRefs:
    - name: payment-svc-http
      kind: Server
      group: policy.linkerd.io
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /api/v1/refund
          method: POST
```

Bind each route to a different set of authenticated identities:

```yaml
# charge-authz.yaml — only checkout-svc can charge
apiVersion: policy.linkerd.io/v1beta3
kind: AuthorizationPolicy
metadata:
  name: allow-checkout-charge
  namespace: payments
spec:
  targetRef:
    group: policy.linkerd.io
    kind: HTTPRoute
    name: payment-charge-route
  requiredAuthenticationRefs:
    - name: checkout-svc-identity
      kind: MeshTLSAuthentication
      group: policy.linkerd.io
---
# refund-authz.yaml — only refund-svc can trigger refunds
apiVersion: policy.linkerd.io/v1beta3
kind: AuthorizationPolicy
metadata:
  name: allow-refund-service
  namespace: payments
spec:
  targetRef:
    group: policy.linkerd.io
    kind: HTTPRoute
    name: payment-refund-route
  requiredAuthenticationRefs:
    - name: refund-svc-identity
      kind: MeshTLSAuthentication
      group: policy.linkerd.io
```

Now `checkout-svc` can `POST /api/v1/charge` but any attempt to `POST /api/v1/refund` from checkout is denied at the proxy — the payment service never sees the request.

---

## Step 7 — ServiceProfile: retries and timeouts

`ServiceProfile` defines per-route traffic policies for a service: timeouts, retries, and which routes are idempotent (safe to retry). Unlike Istio `VirtualService`, `ServiceProfile` is scoped to a Kubernetes Service and applied to callers of that service.

```yaml
# payment-svc-profile.yaml
apiVersion: linkerd.io/v1alpha2
kind: ServiceProfile
metadata:
  name: payment-svc.payments.svc.cluster.local
  namespace: checkout    # applied to callers of payment-svc in the checkout namespace
spec:
  routes:
    - name: POST /api/v1/charge
      condition:
        method: POST
        pathRegex: /api/v1/charge
      responseClasses:
        - condition:
            status:
              min: 500
              max: 599
          isFailure: true    # 5xx counts as a failure for retry and circuit breaking
      timeout: 10s
      isRetryable: false     # POST is not idempotent — do not retry

    - name: GET /api/v1/status
      condition:
        method: GET
        pathRegex: /api/v1/status(/.*)?
      timeout: 3s
      isRetryable: true      # GET is idempotent — safe to retry

    - name: POST /api/v1/refund
      condition:
        method: POST
        pathRegex: /api/v1/refund
      timeout: 30s           # refunds may take longer
      isRetryable: false
```

Apply in the **caller's namespace** (not the server's) — the ServiceProfile is used by the caller's proxy when routing outbound requests to `payment-svc`:

```bash
kubectl apply -f payment-svc-profile.yaml -n checkout

# Verify it's recognised
linkerd viz routes deployment/checkout-svc -n checkout
# ROUTE                        SERVICE          SUCCESS   RPS   LATENCY_P50
# POST /api/v1/charge          payment-svc      99.8%     5.2   8ms
# GET  /api/v1/status          payment-svc     100.0%     2.1   2ms
# [DEFAULT]                    payment-svc      98.2%     0.4   12ms
```

---

## Step 8 — Retries with ServiceProfile

Enable retries for idempotent routes and configure the retry budget:

```yaml
spec:
  retryBudget:
    retryRatio: 0.2          # allow up to 20% additional requests as retries
    minRetriesPerSecond: 10  # always allow at least 10 retries/second
    ttl: 10s                 # retry budget window
  routes:
    - name: GET /api/v1/status
      condition:
        method: GET
        pathRegex: /api/v1/status
      isRetryable: true
      timeout: 3s
```

The retry budget is a global rate limit on retries across all routes in the profile. With `retryRatio: 0.2` and 100 RPS: Linkerd allows up to 20 retry requests per second. If retries exceed this, they are dropped — preventing retry storms from cascading into overload.

Verify retries are working:

```bash
# Inject a fault and watch Linkerd retry transparently
linkerd viz tap deployment/checkout-svc -n checkout \
  --to deployment/payment-svc \
  --to-namespace payments | grep "retry"
# req id=0:5 proxy=out [retry attempt 1]
```

---

## Step 9 — Observe policy decisions in the dashboard

The Linkerd viz dashboard and CLI surface policy enforcement:

```bash
# Check which routes are being allowed vs denied per deployment
linkerd viz routes deployment/payment-svc -n payments --from deployment/checkout-svc -n checkout
# ROUTE                      SUCCESS   RPS   LATENCY_P50   LATENCY_P95   LATENCY_P99
# POST /api/v1/charge        99.9%     5.2   8ms           15ms          22ms
# GET /api/v1/status        100.0%     2.1   2ms           4ms           6ms
# [DEFAULT]                  ---       ---   ---           ---           ---

# Show all Servers and their policy status
kubectl get servers -A
# NAMESPACE   NAME                    PORT    PROXY PROTOCOL
# payments    payment-svc-http        8080    HTTP/2
# payments    payment-svc-admin       4191    HTTP/1

# Describe a Server to see bound AuthorizationPolicies
kubectl describe server payment-svc-http -n payments
# Name:         payment-svc-http
# Namespace:    payments
# Spec:
#   Pod Selector: app=payment-svc
#   Port:         8080
# Events:
#   AuthorizationPolicy allow-checkout-to-payment: ALLOW checkout-svc.checkout.*

# Live tap to confirm mTLS and identity on each request
linkerd viz tap deployment/payment-svc -n payments --method POST
# req id=0:1 proxy=in
#   :method=POST :authority=payment-svc.payments:8080 :path=/api/v1/charge
#   tls=true
#   client-id=checkout-svc.checkout.serviceaccount.identity.linkerd.cluster.local
```

The `client-id` in the tap output is the SPIFFE identity of the caller — confirmed by the mTLS handshake. This is the identity that `MeshTLSAuthentication` evaluates.

---

## Step 10 — Audit and troubleshoot policy denials

When a caller is denied, the connection is reset without an HTTP response. Diagnose denials:

```bash
# Check Linkerd proxy logs for the destination pod (look for policy deny messages)
kubectl logs -n payments \
  $(kubectl get pod -n payments -l app=payment-svc -o jsonpath='{.items[0].metadata.name}') \
  -c linkerd-proxy | grep -i "deny\|unauthorized\|policy"

# Use linkerd diagnostics to check which policy applies to a specific port
linkerd diagnostics policy -n payments \
  deploy/payment-svc 8080

# Output shows:
# Server: payment-svc-http
# Default policy: deny
# AuthorizationPolicies:
#   allow-checkout-to-payment → MeshTLSAuthentication: checkout-svc-identity
#     Identities: checkout-svc.checkout.serviceaccount.identity.linkerd.cluster.local

# Confirm the caller's SPIFFE identity
kubectl exec -n checkout deploy/checkout-svc -c checkout-svc -- \
  cat /var/run/secrets/kubernetes.io/serviceaccount/token | \
  cut -d. -f2 | base64 -d 2>/dev/null | python3 -m json.tool | grep "sub"
# "sub": "system:serviceaccount:checkout:checkout-svc"
```

Common causes of unexpected denials:
- **Wrong ServiceAccount name** in `MeshTLSAuthentication` — the SPIFFE identity uses the ServiceAccount name, not the Deployment name
- **`Server` created before `AuthorizationPolicy`** — there is a brief window where all traffic is denied
- **Caller pod not injected** — un-injected callers present no mTLS identity and are denied by `MeshTLSAuthentication`
- **Wrong namespace in `MeshTLSAuthentication`** — the identity includes both name and namespace

---

## What you have built

- Linkerd's four-resource policy model: `Server` → `MeshTLSAuthentication`/`NetworkAuthentication` → `HTTPRoute` → `AuthorizationPolicy`
- Cluster-wide default policy set to `deny` — the zero-trust starting point
- `Server` resources converting specific ports from default-allow to deny-by-default
- `MeshTLSAuthentication` restricting callers by SPIFFE ServiceAccount identity
- `NetworkAuthentication` allowing CIDR-based access for Prometheus scrapers on the admin port
- `HTTPRoute`-scoped `AuthorizationPolicy`: `checkout-svc` allowed to `POST /charge`, `refund-svc` allowed to `POST /refund`, all other callers denied at the proxy
- `ServiceProfile` with per-route timeouts, `isRetryable` flags, and a `retryBudget` to prevent retry storms
- viz dashboard route-level observability: success rate, RPS, and latency broken down by named route
- `linkerd viz tap` for live request inspection including caller SPIFFE identity and mTLS status
- `linkerd diagnostics policy` for auditing which policies apply to a port and diagnosing unexpected denials

With Linkerd's policy model fully configured, every service-to-service call in the cluster is authenticated by SPIFFE identity, encrypted by mTLS, and scoped to explicitly permitted routes — without any changes to application code.
