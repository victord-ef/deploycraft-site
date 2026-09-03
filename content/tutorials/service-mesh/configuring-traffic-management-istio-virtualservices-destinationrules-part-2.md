---
title: "Configuring Traffic Management with Istio VirtualServices and DestinationRules — Part 2"
date: 2026-09-03
description: "Configure Istio traffic management in depth: weighted routing between service versions, header-based A/B routing, fault injection for chaos testing, circuit breaking with outlier detection, request mirroring, and locality-aware load balancing."
cluster: "Service Mesh"
series: "Installing & Configuring Istio"
part: 2
difficulty: "intermediate"
duration: "50 min"
tags: ["service-mesh", "istio", "kubernetes", "traffic-management", "canary", "circuit-breaking", "chaos", "networking"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

In [Part 1](/tutorials/service-mesh/installing-istio-kubernetes-cluster-istio-operator-part-1/) you installed Istio and verified mTLS. In Part 2 you will use `VirtualService` and `DestinationRule` to configure the full Istio traffic management stack: weighted routing between Deployment versions for canary releases, header-based routing for A/B testing, fault injection to validate resilience, circuit breaking with outlier detection, traffic mirroring for shadow testing, and locality-aware load balancing for multi-zone deployments.

## Prerequisites

- Completed [Part 1](/tutorials/service-mesh/installing-istio-kubernetes-cluster-istio-operator-part-1/) — Istio installed, sidecar injection enabled, mTLS verified
- Two versions of a test application deployed (`v1` and `v2` — instructions below)
- `istioctl` and `kubectl` configured

---

## Step 1 — The VirtualService and DestinationRule relationship

These two CRDs work together. Separating their concerns:

**`DestinationRule`** — defines the *subsets* of a service (groups of pods, typically by version label) and the *connection-level policy* applied to each (load balancing algorithm, TLS settings, connection pool limits, outlier detection).

**`VirtualService`** — defines *routing rules* for traffic arriving at a host. Routes to the subsets defined in the `DestinationRule`. One `VirtualService` per host — multiple routes within it.

```
Request → VirtualService (routing decision: which subset?)
                │
                ▼
          DestinationRule (what connection policy applies to that subset?)
                │
                ▼
          Selected pods
```

Without a `DestinationRule` defining subsets, you cannot route to specific versions. Without a `VirtualService`, all traffic uses Kubernetes' default round-robin `Service` load balancing.

---

## Step 2 — Deploy two versions of a service

Create `v1` and `v2` Deployments, both selected by the same Service:

```yaml
# app-v1.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app-v1
  namespace: my-app
spec:
  replicas: 3
  selector:
    matchLabels:
      app: my-app
      version: v1
  template:
    metadata:
      labels:
        app: my-app
        version: v1
    spec:
      containers:
        - name: my-app
          image: ghcr.io/my-org/my-app:v1.0.0
          ports:
            - containerPort: 8080
---
# app-v2.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app-v2
  namespace: my-app
spec:
  replicas: 1
  selector:
    matchLabels:
      app: my-app
      version: v2
  template:
    metadata:
      labels:
        app: my-app
        version: v2
    spec:
      containers:
        - name: my-app
          image: ghcr.io/my-org/my-app:v2.0.0
          ports:
            - containerPort: 8080
---
# service.yaml — selects all pods with app: my-app (both versions)
apiVersion: v1
kind: Service
metadata:
  name: my-app
  namespace: my-app
spec:
  selector:
    app: my-app
  ports:
    - port: 8080
      targetPort: 8080
```

Without any Istio configuration, Kubernetes sends traffic to all pods (both v1 and v2) in round-robin. The next steps progressively control this.

---

## Step 3 — Define subsets with DestinationRule

```yaml
# destination-rule.yaml
apiVersion: networking.istio.io/v1
kind: DestinationRule
metadata:
  name: my-app
  namespace: my-app
spec:
  host: my-app    # matches the Kubernetes Service name
  trafficPolicy:
    loadBalancer:
      simple: LEAST_CONN    # default for all subsets
    connectionPool:
      tcp:
        maxConnections: 100
      http:
        h2UpgradePolicy: UPGRADE    # upgrade to HTTP/2 when possible
  subsets:
    - name: v1
      labels:
        version: v1    # selects pods with version: v1 label
      trafficPolicy:
        loadBalancer:
          simple: ROUND_ROBIN
    - name: v2
      labels:
        version: v2
      trafficPolicy:
        loadBalancer:
          simple: LEAST_CONN    # use least-connections for the canary subset
```

Apply it:

```bash
kubectl apply -f destination-rule.yaml

# Verify subsets are recognised
istioctl x describe svc my-app -n my-app
# Service: my-app/my-app
# Subsets:
#   v1: version=v1
#   v2: version=v2
```

---

## Step 4 — Weighted routing (canary deployment)

Route 90% of traffic to v1 and 10% to v2:

```yaml
# virtual-service-weighted.yaml
apiVersion: networking.istio.io/v1
kind: VirtualService
metadata:
  name: my-app
  namespace: my-app
spec:
  hosts:
    - my-app
  http:
    - route:
        - destination:
            host: my-app
            subset: v1
          weight: 90
        - destination:
            host: my-app
            subset: v2
          weight: 10
```

```bash
kubectl apply -f virtual-service-weighted.yaml

# Generate traffic and observe the split
for i in $(seq 1 20); do
  kubectl exec -n my-app deploy/my-app-v1 -- \
    curl -s http://my-app:8080/version
done
# v1 v1 v1 v1 v1 v1 v1 v1 v1 v2 v1 v1 v1 v1 v1 v1 v1 v1 v1 v1
# ≈ 90% v1, 10% v2 ✔
```

Adjust the weights progressively as confidence grows:

```bash
# Update weights to 80/20
kubectl patch virtualservice my-app -n my-app \
  --type merge \
  -p '{"spec":{"http":[{"route":[{"destination":{"host":"my-app","subset":"v1"},"weight":80},{"destination":{"host":"my-app","subset":"v2"},"weight":20}]}]}}'
```

For GitOps-managed canary progression, use Flagger (covered in [Progressive Delivery Part 1](/tutorials/gitops/implementing-canary-deployments-flagger-flux-part-1/)) — it automates the weight changes based on metric analysis.

---

## Step 5 — Header-based routing (A/B testing)

Route requests containing specific headers to v2, all others to v1:

```yaml
# virtual-service-header.yaml
apiVersion: networking.istio.io/v1
kind: VirtualService
metadata:
  name: my-app
  namespace: my-app
spec:
  hosts:
    - my-app
  http:
    # Rule 1: header match → v2
    - match:
        - headers:
            x-user-segment:
              exact: "beta"
        - headers:
            cookie:
              regex: "^(.*; )?preview=true(;.*)?$"
      route:
        - destination:
            host: my-app
            subset: v2
    # Rule 2: all other traffic → v1 (catch-all, no match block)
    - route:
        - destination:
            host: my-app
            subset: v1
```

```bash
# Test header-based routing
kubectl exec -n my-app deploy/my-app-v1 -- \
  curl -H "x-user-segment: beta" http://my-app:8080/version
# v2 ← routed to v2 via header match

kubectl exec -n my-app deploy/my-app-v1 -- \
  curl http://my-app:8080/version
# v1 ← routed to v1 (no matching header)
```

Match conditions support:
- `exact` — exact string match
- `prefix` — string prefix match
- `regex` — RE2 regular expression match
- Header name matching is case-insensitive

Multiple `match` blocks within a single rule are evaluated as OR. Multiple `match` conditions within one block are evaluated as AND.

---

## Step 6 — Fault injection (chaos testing)

Fault injection validates that your application handles upstream failures correctly — before those failures happen in production. Istio injects faults at the proxy level without any application changes.

### Inject a delay

```yaml
# virtual-service-delay.yaml
apiVersion: networking.istio.io/v1
kind: VirtualService
metadata:
  name: my-app
  namespace: my-app
spec:
  hosts:
    - my-app
  http:
    - fault:
        delay:
          percentage:
            value: 50.0    # inject the delay for 50% of requests
          fixedDelay: 5s
      route:
        - destination:
            host: my-app
            subset: v1
```

Apply this and observe whether upstream callers handle the 5-second delay: do they time out gracefully? Do they surface an error to the user? Does circuit breaking engage?

### Inject HTTP errors

```yaml
- fault:
    abort:
      percentage:
        value: 10.0    # return a 503 for 10% of requests
      httpStatus: 503
  route:
    - destination:
        host: my-app
        subset: v1
```

Combine delay and abort to simulate a degraded upstream:

```yaml
- fault:
    delay:
      percentage:
        value: 20.0
      fixedDelay: 3s
    abort:
      percentage:
        value: 5.0
      httpStatus: 503
```

Always remove fault injection rules after testing — apply the clean `VirtualService` from Git:

```bash
kubectl apply -f virtual-service-weighted.yaml
```

---

## Step 7 — Retries and timeouts

Configure retries and timeouts in the `VirtualService` for automatic resilience without application-level retry logic:

```yaml
# virtual-service-resilience.yaml
apiVersion: networking.istio.io/v1
kind: VirtualService
metadata:
  name: payment-service
  namespace: payments
spec:
  hosts:
    - payment-service
  http:
    - timeout: 10s    # total deadline for the request
      retries:
        attempts: 3
        perTryTimeout: 3s      # each attempt has its own 3s deadline
        retryOn: "gateway-error,connect-failure,retriable-4xx,reset"
      route:
        - destination:
            host: payment-service
            subset: v1
```

`retryOn` conditions:
- `gateway-error` — 502, 503, 504
- `connect-failure` — upstream connection failure
- `retriable-4xx` — 409 Conflict (safe to retry)
- `reset` — upstream closed the connection unexpectedly
- `5xx` — any 5xx response (use carefully — only if the upstream operation is idempotent)

The combination of `timeout` and `perTryTimeout` matters: with 3 attempts and a 3s per-try timeout, the maximum total time is 10s (the outer timeout governs). Without an outer timeout, retries could take 3 × 3s = 9s.

---

## Step 8 — Circuit breaking with outlier detection

Circuit breaking prevents cascading failures by stopping traffic to unhealthy endpoints:

```yaml
# destination-rule-circuit-breaker.yaml
apiVersion: networking.istio.io/v1
kind: DestinationRule
metadata:
  name: payment-service
  namespace: payments
spec:
  host: payment-service
  trafficPolicy:
    connectionPool:
      tcp:
        maxConnections: 50          # max concurrent TCP connections
      http:
        http1MaxPendingRequests: 10 # queue depth before returning 503
        http2MaxRequests: 100       # max concurrent HTTP/2 requests
        maxRequestsPerConnection: 5 # max requests before closing a connection
    outlierDetection:
      consecutiveGatewayErrors: 5   # eject after 5 consecutive 502/503/504
      consecutive5xxErrors: 5       # or 5 consecutive 5xx
      interval: 10s                 # evaluation window
      baseEjectionTime: 30s         # initial ejection duration
      maxEjectionPercent: 50        # never eject more than 50% of endpoints
      minHealthPercent: 30          # stop ejecting if fewer than 30% are healthy
```

When an endpoint (a specific pod) returns `consecutiveGatewayErrors` failures within the `interval`, the sidecar ejects it for `baseEjectionTime`. Each subsequent ejection doubles the ejection time (exponential backoff) up to `maxEjectionPercent` of the total endpoint pool.

Verify circuit breaker state:

```bash
# Check which endpoints are ejected
istioctl x proxy-status
kubectl exec -n payments \
  $(kubectl get pod -n payments -l app=payment-service -o jsonpath='{.items[0].metadata.name}') \
  -c istio-proxy -- curl localhost:15000/clusters | grep payment-service | grep ejected
```

---

## Step 9 — Traffic mirroring (shadow testing)

Traffic mirroring sends a copy of live production traffic to a new version simultaneously, without affecting the response the client receives. The client always gets the response from the primary version — the mirrored copy is fire-and-forget:

```yaml
# virtual-service-mirror.yaml
apiVersion: networking.istio.io/v1
kind: VirtualService
metadata:
  name: my-app
  namespace: my-app
spec:
  hosts:
    - my-app
  http:
    - route:
        - destination:
            host: my-app
            subset: v1
          weight: 100    # all responses come from v1
      mirror:
        host: my-app
        subset: v2       # v2 receives a copy of every request
      mirrorPercentage:
        value: 100.0     # mirror 100% — reduce to 10% for high-traffic services
```

When `mirror` is configured:
- Every request is forwarded to `v1` (primary) — the caller gets v1's response
- A copy of the same request is sent to `v2` simultaneously
- v2's response is discarded
- v2 sees the real production request headers and body — realistic load without impacting users

Use mirroring to:
- Validate that v2 handles production request shapes correctly before routing any live traffic to it
- Compare v1 and v2 response times using Prometheus metrics on both subsets
- Identify v2 failures before exposing them to users

---

## Step 10 — Locality-aware load balancing

In multi-zone clusters, locality-aware load balancing prefers endpoints in the same zone as the caller before routing cross-zone — reducing inter-zone data transfer costs and latency:

```yaml
# destination-rule-locality.yaml
apiVersion: networking.istio.io/v1
kind: DestinationRule
metadata:
  name: my-app
  namespace: my-app
spec:
  host: my-app
  trafficPolicy:
    loadBalancer:
      localityLbSetting:
        enabled: true
        distribute:
          # From eu-west-1a, send 80% to eu-west-1a, 20% to eu-west-1b
          - from: eu-west-1/eu-west-1a/*
            to:
              "eu-west-1/eu-west-1a/*": 80
              "eu-west-1/eu-west-1b/*": 20
          # From eu-west-1b, prefer eu-west-1b
          - from: eu-west-1/eu-west-1b/*
            to:
              "eu-west-1/eu-west-1b/*": 80
              "eu-west-1/eu-west-1a/*": 20
        failover:
          # If eu-west-1 is down, fail over to eu-central-1
          - from: eu-west-1
            to: eu-central-1
    outlierDetection:
      consecutiveGatewayErrors: 3
      interval: 10s
      baseEjectionTime: 30s
```

Locality load balancing requires `outlierDetection` to be configured — Istio uses health information from outlier detection to determine when to fail over to a different locality.

Node zone labels are used for locality inference:

```bash
# Verify nodes have zone labels (set automatically by cloud providers)
kubectl get nodes --show-labels | grep topology.kubernetes.io/zone
# node-1   topology.kubernetes.io/zone=eu-west-1a
# node-2   topology.kubernetes.io/zone=eu-west-1b
```

---

## What you have built

- The `VirtualService` / `DestinationRule` division: routing decisions vs connection policy and subset definitions
- Two-version Deployment setup with version labels enabling Istio subset selection
- `DestinationRule` subsets with per-subset load balancing policies (ROUND_ROBIN, LEAST_CONN)
- Weighted routing at 90/10, 80/20 splits for progressive canary rollouts
- Header-based and cookie-based routing for A/B test cohort separation
- Fault injection: fixed delays and HTTP abort codes for 50% and 10% of requests — chaos validation without application changes
- Retry configuration: `attempts`, `perTryTimeout`, outer `timeout`, and `retryOn` conditions
- Circuit breaking: `connectionPool` limits (max connections, pending requests) and `outlierDetection` (consecutive error threshold, ejection time, max ejection percent)
- Traffic mirroring: 100% of production requests shadowed to v2 asynchronously — live validation without user impact
- Locality-aware load balancing: zone-preferring distribution with cross-zone failover and outlier detection integration

With these primitives in place, Istio's data plane handles the full range of traffic management concerns — retries, timeouts, circuit breaking, canary routing, shadow testing — transparently, without any modification to application code.
