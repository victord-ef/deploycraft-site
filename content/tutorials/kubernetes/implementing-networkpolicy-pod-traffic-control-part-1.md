---
title: "Implementing NetworkPolicy for Pod-to-Pod Traffic Control — Part 1"
date: 2026-09-01
description: "Write Kubernetes NetworkPolicy resources to control ingress and egress traffic between pods. Build a layered policy model that isolates namespaces and restricts inter-service communication."
cluster: "Kubernetes"
series: "Networking"
part: 1
difficulty: "intermediate"
duration: "45 min"
tags: ["kubernetes", "networkpolicy", "networking", "security", "cni", "calico", "devsecops"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

By the end of this tutorial you will have a working NetworkPolicy model that isolates namespaces by default, permits only explicitly defined inter-service communication, and restricts egress to known endpoints. Part 2 builds on this by applying zero-trust principles cluster-wide — denying all traffic by default and allowing only what is explicitly declared.

## Prerequisites

- A Kubernetes cluster with a CNI plugin that enforces NetworkPolicy (Calico, Cilium, Weave Net, or Antrea)
- `kubectl` with cluster-admin access
- Basic familiarity with Kubernetes Services and namespaces

> **CNI requirement:** NetworkPolicy objects are valid in any Kubernetes cluster, but they are only enforced if a CNI plugin that supports NetworkPolicy is running. Flannel does not enforce NetworkPolicy. If you are on EKS, GKE, or AKS, the managed CNI (VPC CNI, Dataplane V2, Azure CNI) supports NetworkPolicy.

---

## How NetworkPolicy works

A `NetworkPolicy` selects pods using a `podSelector` and defines ingress (inbound) and egress (outbound) rules. Rules specify which sources or destinations are allowed — everything else is denied for that direction once a policy applies.

Three important behaviours:

1. **Additive.** Multiple NetworkPolicies selecting the same pod are unioned. A pod is allowed traffic if any matching policy permits it.
2. **Direction-specific.** A policy that defines `ingress` rules only affects inbound traffic. Egress is unrestricted unless a policy explicitly defines `egress` rules.
3. **Default-allow.** A pod with no NetworkPolicy selecting it has unrestricted ingress and egress. Policies only restrict pods they select.

Traffic sources and destinations are expressed as:

| Selector type | Matches |
|---|---|
| `podSelector` | Pods in the same namespace matching labels |
| `namespaceSelector` | All pods in namespaces matching labels |
| `podSelector` + `namespaceSelector` | Pods matching labels in namespaces matching labels |
| `ipBlock` | A CIDR range (for external traffic) |

---

## Step 1 — Verify NetworkPolicy enforcement

Before writing policies, confirm your CNI enforces them:

```bash
# Check which CNI is running
kubectl get pods -n kube-system | grep -E "calico|cilium|weave|antrea|flannel"

# Deploy two test pods in different namespaces
kubectl create namespace app
kubectl create namespace db

kubectl run client --image=nicolaka/netshoot -n app \
  --labels="role=client" -- sleep infinity
kubectl run server --image=nginx -n db \
  --labels="role=server" --expose --port=80

# Verify connectivity (should succeed before any policy)
kubectl exec -n app client -- curl -s --max-time 3 server.db.svc.cluster.local
```

If the curl succeeds, traffic is unrestricted — as expected before any NetworkPolicy is applied.

---

## Step 2 — Default deny ingress for a namespace

The foundation of any NetworkPolicy model is a default-deny rule. This selects all pods in the namespace with an empty `podSelector` and defines no ingress rules — blocking all inbound traffic:

```yaml
# default-deny-ingress.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny-ingress
  namespace: db
spec:
  podSelector: {}
  policyTypes:
    - Ingress
```

```bash
kubectl apply -f default-deny-ingress.yaml

# Verify — should now fail (connection timeout)
kubectl exec -n app client -- curl -s --max-time 3 server.db.svc.cluster.local
```

The empty `podSelector: {}` matches all pods in the `db` namespace. No `ingress` rules means no inbound traffic is permitted. The `server` pod in `db` is now unreachable from anywhere.

---

## Step 3 — Allow specific ingress from a namespace

Now explicitly permit traffic from the `app` namespace to the `server` pod:

```yaml
# allow-app-to-server.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-app-to-server
  namespace: db
spec:
  podSelector:
    matchLabels:
      role: server
  policyTypes:
    - Ingress
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: app
          podSelector:
            matchLabels:
              role: client
      ports:
        - protocol: TCP
          port: 80
```

```bash
kubectl apply -f allow-app-to-server.yaml

# Should succeed — explicitly permitted
kubectl exec -n app client -- curl -s --max-time 3 server.db.svc.cluster.local

# Test from a different pod — should still be blocked
kubectl run attacker --image=nicolaka/netshoot -n default \
  --labels="role=attacker" -- sleep infinity
kubectl exec -n default attacker -- curl -s --max-time 3 server.db.svc.cluster.local
```

The combined `namespaceSelector` + `podSelector` in a single `from` entry means: pods labelled `role=client` **in** namespaces labelled `kubernetes.io/metadata.name=app`. This is an AND condition. If they were separate list entries, it would be an OR — a common source of overly permissive policies.

---

## Step 4 — Allow ingress from multiple sources

A real service typically needs to accept traffic from multiple callers. Add a frontend namespace:

```bash
kubectl create namespace frontend
kubectl run frontend --image=nicolaka/netshoot -n frontend \
  --labels="role=frontend" -- sleep infinity
```

Update the policy to allow both `app` and `frontend` as ingress sources:

```yaml
# allow-multiple-ingress.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-app-to-server
  namespace: db
spec:
  podSelector:
    matchLabels:
      role: server
  policyTypes:
    - Ingress
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: app
          podSelector:
            matchLabels:
              role: client
      ports:
        - protocol: TCP
          port: 80
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: frontend
          podSelector:
            matchLabels:
              role: frontend
      ports:
        - protocol: TCP
          port: 80
```

```bash
kubectl apply -f allow-multiple-ingress.yaml

# Both should succeed
kubectl exec -n app client -- curl -s --max-time 3 server.db.svc.cluster.local
kubectl exec -n frontend frontend -- curl -s --max-time 3 server.db.svc.cluster.local
```

---

## Step 5 — Controlling egress traffic

By default, adding only `Ingress` to `policyTypes` leaves egress unrestricted. To restrict what pods can connect to outbound, add `Egress` rules.

Lock down the `client` pod so it can only connect to the `server` in `db` and DNS:

```yaml
# client-egress-policy.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: client-egress
  namespace: app
spec:
  podSelector:
    matchLabels:
      role: client
  policyTypes:
    - Egress
  egress:
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: db
          podSelector:
            matchLabels:
              role: server
      ports:
        - protocol: TCP
          port: 80
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: kube-system
      ports:
        - protocol: UDP
          port: 53
        - protocol: TCP
          port: 53
```

```bash
kubectl apply -f client-egress-policy.yaml

# Should succeed — explicitly allowed
kubectl exec -n app client -- curl -s --max-time 3 server.db.svc.cluster.local

# Should fail — not in the egress allow list
kubectl exec -n app client -- curl -s --max-time 3 https://example.com
```

> **Always allow DNS egress.** Port 53 to `kube-system` (where CoreDNS runs) must be permitted in any egress policy, or DNS resolution will fail and all hostname-based connections will break — including connections to Services that are explicitly permitted by IP.

---

## Step 6 — Restricting egress to external IPs with ipBlock

To allow a pod to reach a specific external API but nothing else on the internet:

```yaml
# external-api-egress.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-external-api
  namespace: app
spec:
  podSelector:
    matchLabels:
      role: client
  policyTypes:
    - Egress
  egress:
    - to:
        - ipBlock:
            cidr: 203.0.113.0/24      # external API IP range
            except:
              - 203.0.113.5/32        # exclude a specific IP within the range
      ports:
        - protocol: TCP
          port: 443
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: kube-system
      ports:
        - protocol: UDP
          port: 53
```

`ipBlock` rules apply to traffic leaving the cluster. They do not apply to pod-to-pod traffic (which uses pod IPs, not external CIDRs).

---

## Step 7 — Labelling namespaces for policy selectors

NetworkPolicy `namespaceSelector` matches namespace labels. The built-in label `kubernetes.io/metadata.name` is automatically applied to every namespace and equals the namespace name — use it for precise namespace matching without adding custom labels.

For group-based matching, add labels to namespaces:

```bash
kubectl label namespace app environment=production team=backend
kubectl label namespace frontend environment=production team=frontend
kubectl label namespace db environment=production tier=data
```

Now write policies using environment labels instead of individual namespace names:

```yaml
# Allow all production pods to reach the db namespace
ingress:
  - from:
      - namespaceSelector:
          matchLabels:
            environment: production
```

This approach scales better than listing individual namespace names — new namespaces in `production` automatically match without policy updates.

---

## Step 8 — Verifying policies with network testing

Always verify NetworkPolicy behaviour with live traffic tests rather than relying on `kubectl describe`:

```bash
# Quick connectivity test script
test_connectivity() {
  local src_ns=$1 src_pod=$2 dst=$3 expected=$4
  result=$(kubectl exec -n $src_ns $src_pod -- \
    curl -s --max-time 3 -o /dev/null -w "%{http_code}" $dst 2>/dev/null)
  if [[ "$expected" == "allow" && "$result" != "000" ]]; then
    echo "PASS: $src_ns/$src_pod -> $dst ($result)"
  elif [[ "$expected" == "deny" && "$result" == "000" ]]; then
    echo "PASS: $src_ns/$src_pod -> $dst (blocked)"
  else
    echo "FAIL: $src_ns/$src_pod -> $dst (expected $expected, got $result)"
  fi
}

test_connectivity app client server.db.svc.cluster.local allow
test_connectivity default attacker server.db.svc.cluster.local deny
test_connectivity frontend frontend server.db.svc.cluster.local allow
```

Run this suite after every policy change to catch regressions.

---

## What you have built

- Default-deny ingress for the `db` namespace
- Explicit ingress allow from specific pods in specific namespaces
- Multi-source ingress policy for a service with multiple callers
- Egress policy restricting outbound to specific services and external CIDRs
- DNS egress correctly permitted to prevent resolution failures
- Namespace labelling strategy for scalable policy selectors
- Connectivity verification script

## Next steps

In [Part 2](/tutorials/kubernetes/zero-trust-networking-kubernetes-networkpolicy-part-2/) you will apply zero-trust principles cluster-wide: default-deny all traffic in every namespace, implement a systematic allow-list model, handle system namespaces and health check traffic, and use Cilium's network policy tooling for visibility into policy decisions.
