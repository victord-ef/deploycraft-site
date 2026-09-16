---
title: "Introduction to Service Mesh Concepts and Architecture in Kubernetes — Part 1"
date: 2026-09-03
description: "Understand what a service mesh is, why the sidecar proxy model exists, what problems mTLS, traffic management, and observability solve that Kubernetes alone cannot, and how the control plane and data plane divide responsibilities in Istio and Linkerd."
cluster: "Service Mesh"
series: "Service Mesh Foundations"
part: 1
difficulty: "intermediate"
duration: "35 min"
tags: ["service-mesh", "istio", "linkerd", "kubernetes", "mtls", "observability", "networking", "devsecops"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

By the end of this tutorial you will have a clear mental model of what a service mesh does, why it exists as a distinct infrastructure layer, and how its components — sidecar proxies, control plane, and data plane — interact. You will understand the core capabilities (mTLS, traffic management, observability) and when adding a service mesh is the right decision for a Kubernetes platform. Part 2 compares Istio and Linkerd directly so you can make an informed choice for your cluster.

## Prerequisites

- Solid understanding of Kubernetes Deployments, Services, and namespaces
- Familiarity with basic networking concepts (TCP/IP, TLS, HTTP/2)
- No service mesh installation required for this tutorial — it is conceptual

---

## Step 1 — The problem a service mesh solves

A Kubernetes cluster provides pod networking: any pod can reach any other pod. That is the extent of the built-in guarantee. What the cluster does not provide:

- **Authentication between services.** Any pod can call any other pod without proving its identity.
- **Encryption in transit.** Pod-to-pod traffic crosses the cluster network in plaintext by default.
- **Fine-grained traffic control.** You cannot route 10% of requests to a new version of a service, retry on 503s, or circuit-break a failing dependency at the network layer without modifying application code.
- **Uniform observability.** Request rate, error rate, and latency between services require per-service instrumentation — no standard metric set exists at the infrastructure layer.
- **Policy enforcement.** Preventing service A from calling service B requires NetworkPolicy, which is coarse (port-level) and cannot be tied to service identity.

These gaps are acceptable for a handful of services. For a platform with dozens or hundreds of services, they create significant operational, security, and reliability risk. The service mesh addresses all of them without modifying application code.

---

## Step 2 — The sidecar proxy model

A service mesh works by injecting a proxy container (sidecar) into every Pod alongside the application container. The proxy intercepts all inbound and outbound network traffic:

```
┌─────────────────────── Pod ──────────────────────────┐
│                                                       │
│  ┌─────────────────┐        ┌──────────────────────┐ │
│  │  App container  │◄──────►│  Sidecar proxy       │ │
│  │  (port 8080)    │        │  (Envoy / Linkerd2)  │ │
│  └─────────────────┘        └──────────┬───────────┘ │
│                                        │             │
└────────────────────────────────────────┼─────────────┘
                                         │
                              network traffic
                              (mTLS, HTTP/2, gRPC)
                                         │
                              ┌──────────▼───────────┐
                              │  Sidecar proxy in    │
                              │  the destination Pod │
                              └──────────────────────┘
```

The sidecar is injected automatically via a mutating admission webhook when a namespace is labelled for injection:

```bash
kubectl label namespace my-app istio-injection=enabled
# or for Linkerd:
kubectl annotate namespace my-app linkerd.io/inject=enabled
```

From the application's perspective, nothing changes. It listens on the same port, makes the same outbound connections. The sidecar transparently intercepts via `iptables` rules installed in the Pod's network namespace.

### The ambient mesh alternative

Istio's ambient mode (stable in Istio 1.22+) removes the sidecar entirely. A per-node `ztunnel` proxy handles L4 traffic (mTLS, authorisation policy) and an optional per-namespace `waypoint` proxy handles L7 (retries, circuit breaking, header manipulation). This reduces memory overhead significantly — relevant for large clusters where sidecar memory per pod accumulates to gigabytes.

---

## Step 3 — Control plane and data plane

A service mesh has two logical layers:

### Data plane

The collection of all sidecar proxies running in the cluster. This is where actual traffic flows. The data plane:
- Terminates and originates mTLS connections
- Applies routing rules (weight-based traffic splitting, header matching, retries)
- Collects telemetry (request counts, latencies, error rates) and exports to Prometheus
- Enforces authorisation policies (allow/deny decisions per request)

The data plane runs distributed across every Pod. It processes millions of requests without contacting the control plane — configuration is pushed down once and cached.

### Control plane

Manages and configures the data plane proxies. The control plane:
- Issues and rotates TLS certificates (the mesh CA)
- Distributes service discovery information (endpoints, routes)
- Translates mesh configuration (CRDs) into proxy-native config (Envoy xDS for Istio, Linkerd's SMI config)
- Exposes telemetry aggregation and the CLI/dashboard

```
┌──────────────────────────────────────────────────────────────┐
│                      Control Plane                            │
│                                                               │
│  Istiod (Istio)           Linkerd Control Plane              │
│  ────────────────         ──────────────────────             │
│  Pilot (service          identity (cert issuance)            │
│    discovery + routing)  destination (routing)               │
│  Citadel (CA / certs)    proxy-injector                      │
│  Galley (config          sp-validator                        │
│    validation)                                               │
└─────────────────────────────┬────────────────────────────────┘
                              │ push xDS config / certs
                              ▼
┌──────────────────────────────────────────────────────────────┐
│                       Data Plane                              │
│                                                               │
│   Pod A: [app] + [sidecar] ◄──mTLS──► Pod B: [app]+[sidecar]│
│   Pod C: [app] + [sidecar] ◄──mTLS──► Pod D: [app]+[sidecar]│
│                                                               │
└──────────────────────────────────────────────────────────────┘
```

The control plane is not in the request path. A control plane outage does not stop existing traffic — proxies continue operating on their last-known configuration.

---

## Step 4 — Mutual TLS (mTLS)

mTLS is the foundational security capability of a service mesh. In standard TLS, only the server presents a certificate (the client validates the server). In mTLS, both sides present certificates:

```
Service A sidecar              Service B sidecar
─────────────────              ─────────────────
"I am service-a               "I am service-b
 in namespace payments,        in namespace orders,
 here is my cert               here is my cert
 signed by the mesh CA"        signed by the mesh CA"

Both verify → connection established
One fails verification → connection rejected
```

The mesh CA (Istiod's Citadel component, or Linkerd's identity controller) issues short-lived certificates (default: 24 hours) to each workload using the SPIFFE identity standard:

```
spiffe://cluster.local/ns/payments/sa/checkout-service
```

This identity is tied to the Kubernetes ServiceAccount, not to the IP address or hostname — which change across Pod restarts and rollouts.

### mTLS modes

| Mode | Behaviour | Use when |
|---|---|---|
| `DISABLE` | No mTLS | Never in production |
| `PERMISSIVE` | Accepts both plaintext and mTLS | During migration — allows non-mesh services to call mesh services |
| `STRICT` | mTLS required — rejects plaintext | Target state for all namespaces |

Enable strict mTLS cluster-wide in Istio:

```yaml
apiVersion: security.istio.io/v1
kind: PeerAuthentication
metadata:
  name: default
  namespace: istio-system    # mesh-wide when in the root namespace
spec:
  mtls:
    mode: STRICT
```

---

## Step 5 — Authorisation policies

mTLS proves identity. Authorisation policies use that identity to control which services can talk to which:

```yaml
# Allow only the checkout service to call the payment service
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: payment-service-authz
  namespace: payments
spec:
  selector:
    matchLabels:
      app: payment-service
  action: ALLOW
  rules:
    - from:
        - source:
            principals:
              - "cluster.local/ns/checkout/sa/checkout-service"
      to:
        - operation:
            methods: ["POST"]
            paths: ["/api/v1/charge"]
```

This policy allows only the `checkout-service` ServiceAccount (in the `checkout` namespace) to make POST requests to `/api/v1/charge` on the payment service. All other callers — even those with valid mTLS certificates — are denied.

NetworkPolicy operates at the IP/port level and cannot express this kind of identity-based, HTTP-method-scoped rule. AuthorizationPolicy is what zero-trust networking in Kubernetes actually looks like in practice.

---

## Step 6 — Traffic management

The service mesh data plane handles traffic management features that would otherwise require application-level code or external load balancers:

### Retries

```yaml
apiVersion: networking.istio.io/v1
kind: VirtualService
metadata:
  name: payment-service
  namespace: payments
spec:
  hosts:
    - payment-service
  http:
    - retries:
        attempts: 3
        perTryTimeout: 5s
        retryOn: "gateway-error,connect-failure,retriable-4xx"
      route:
        - destination:
            host: payment-service
```

The sidecar retries failed requests transparently — the calling service sees a single successful response.

### Circuit breaking

```yaml
apiVersion: networking.istio.io/v1
kind: DestinationRule
metadata:
  name: payment-service
  namespace: payments
spec:
  host: payment-service
  trafficPolicy:
    outlierDetection:
      consecutiveGatewayErrors: 5      # eject an endpoint after 5 consecutive failures
      interval: 10s
      baseEjectionTime: 30s            # keep ejected for at least 30s
      maxEjectionPercent: 50           # never eject more than 50% of endpoints
```

When an upstream endpoint returns 5 consecutive gateway errors, the sidecar stops sending traffic to it for 30 seconds. The application never sees the failing endpoint.

### Timeouts

```yaml
http:
  - timeout: 10s
    route:
      - destination:
          host: slow-service
```

A deadline applied at the mesh layer — without changing the application or the SDK it uses to make HTTP calls.

---

## Step 7 — Observability: the golden signals for free

Because all traffic flows through the sidecar, the mesh can emit the four golden signals for every service-to-service call without any application instrumentation:

| Signal | What the mesh provides |
|---|---|
| **Latency** | p50, p90, p99, p999 request duration per source/destination pair |
| **Traffic** | Request rate (RPS) per source/destination/status code |
| **Errors** | 4xx and 5xx rates, broken down by route and method |
| **Saturation** | Active connections, pending requests per endpoint |

These metrics are emitted in Prometheus format by every sidecar. With a standard Grafana dashboard, you get a service topology map showing request rates and error rates on every edge — no instrumentation required from application teams.

For distributed tracing, Istio and Linkerd both propagate the `b3` and `W3C Trace Context` headers automatically. Applications need only forward the trace headers when making downstream calls (not generate them) — a much lighter instrumentation requirement than full OpenTelemetry instrumentation.

---

## Step 8 — When to add a service mesh

A service mesh adds operational complexity. It is not the right choice for every cluster:

**Add a service mesh when:**
- You have multiple services calling each other and need uniform observability across all of them
- Compliance or security requirements mandate encryption in transit between services (PCI DSS, HIPAA, SOC 2)
- You need zero-trust: deny-by-default service-to-service communication with identity-based allow rules
- You want to implement canary/blue-green traffic splitting at the mesh layer (Flagger, covered in [Progressive Delivery Part 1](/tutorials/gitops/implementing-canary-deployments-flagger-flux-part-1/))
- You need per-request retries, timeouts, and circuit breaking without modifying application code

**Do not add a service mesh when:**
- You have fewer than five services and all are operated by the same team
- Your workload is batch jobs or event-driven consumers with no synchronous service-to-service calls
- Your team does not yet have the operational capacity to troubleshoot proxy-related networking issues
- Performance is critical and you cannot absorb the latency added by the sidecar (typically 1–5ms per hop)

The decision is a maturity trade-off: the mesh pays for itself when the operational overhead of debugging service-to-service connectivity, rotating certificates manually, and adding retry logic to every service exceeds the overhead of operating the mesh itself.

---

## Step 9 — SPIFFE and workload identity

The mesh uses the SPIFFE (Secure Production Identity Framework For Everyone) standard for workload identity. Each workload receives a SPIFFE Verifiable Identity Document (SVID) — a short-lived X.509 certificate encoding:

```
Subject Alternative Name:
  URI: spiffe://cluster.local/ns/<namespace>/sa/<serviceaccount>
```

The SPIFFE URI is the identity that AuthorizationPolicy rules reference. It is:
- **Tied to the ServiceAccount**, not the IP or hostname — survives Pod restarts
- **Short-lived** (default 24h, Linkerd uses 24h, Istio configurable down to minutes) — compromised certificates expire quickly
- **Rotated automatically** — no manual certificate management
- **Interoperable** — SPIFFE is a CNCF standard; the same identity model works across Istio, Linkerd, Consul Connect, and SPIRE

The cluster's root CA (held by Istiod or Linkerd's identity controller) signs each workload certificate. For production, replace the self-signed root CA with one signed by your organisation's PKI:

```bash
# Istio: plug in an intermediate CA from your PKI
kubectl create secret generic cacerts -n istio-system \
  --from-file=ca-cert.pem \
  --from-file=ca-key.pem \
  --from-file=root-cert.pem \
  --from-file=cert-chain.pem
```

---

## What you have built

- The operational gaps in vanilla Kubernetes that a service mesh closes: identity, encryption, traffic control, and observability
- The sidecar proxy model — how `iptables` interception gives the mesh full visibility without application changes
- Istio ambient mode as a sidecar-free alternative for large clusters
- Control plane vs data plane: what each layer does, how configuration is pushed, and why a control plane outage does not stop traffic
- mTLS: both-sides certificate validation using SPIFFE identities tied to Kubernetes ServiceAccounts
- `PeerAuthentication` (STRICT/PERMISSIVE/DISABLE) for cluster-wide or per-namespace mTLS enforcement
- `AuthorizationPolicy`: identity-scoped, method-scoped, and path-scoped allow/deny rules beyond what NetworkPolicy can express
- Traffic management at the mesh layer: retries, circuit breaking, and timeouts via `VirtualService` and `DestinationRule`
- The four golden signals emitted automatically by every sidecar — request rate, error rate, latency histograms, saturation
- SPIFFE workload identity: short-lived X.509 SVIDs, automatic rotation, and external PKI integration
- When to add a service mesh — the operational maturity threshold

In [Part 2](/tutorials/service-mesh/choosing-between-istio-linkerd-kubernetes-part-2/) you will compare Istio and Linkerd directly across architecture, performance, operational complexity, and feature surface — with a decision framework for choosing the right mesh for your platform.
