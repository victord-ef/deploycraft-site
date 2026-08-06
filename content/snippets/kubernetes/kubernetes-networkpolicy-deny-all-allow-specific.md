---
title: "NetworkPolicy — Deny All, Allow Specific"
date: 2026-07-25
description: "Namespace-scoped default-deny policy that blocks all ingress and egress, then selectively re-opens only the traffic your workload actually needs."
lang: "Kubernetes"
tags: ["kubernetes", "networkpolicy", "zero-trust", "namespace", "security"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-07-25"
draft: false
---

## When to use this

When you deploy a workload into a shared cluster and need to enforce that pods can only talk to explicitly permitted peers — not every other pod in the cluster by default.

## Without it

```yaml
# No NetworkPolicy — every pod in the cluster can reach every other pod
# on any port, by default
```

A compromised pod can freely scan and connect to databases, internal APIs, or other tenants' services. There is no network-layer boundary between workloads unless you create one explicitly.

## Snippet

Apply these two manifests together. The first locks the namespace down; the second punches only the holes your app needs.

```yaml
# 1. Default deny — all ingress and egress blocked for every pod in the namespace
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny-all
  namespace: my-app
spec:
  podSelector: {}       # matches every pod in the namespace
  policyTypes:
    - Ingress
    - Egress
```

```yaml
# 2. Allow rules — ingress from the frontend, egress to the database and DNS only
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: my-app-allow
  namespace: my-app
spec:
  podSelector:
    matchLabels:
      app: my-app
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - podSelector:
            matchLabels:
              app: frontend
      ports:
        - protocol: TCP
          port: 8080
  egress:
    - to:
        - podSelector:
            matchLabels:
              app: postgres
      ports:
        - protocol: TCP
          port: 5432
    - to:                  # allow DNS resolution — without this, all lookups fail
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: kube-system
      ports:
        - protocol: UDP
          port: 53
```

**Key decisions:**

| Field | Why it matters |
|---|---|
| `podSelector: {}` in deny policy | Empty selector matches every pod — this is intentional. Any pod without an explicit allow rule is isolated. |
| `policyTypes: [Ingress, Egress]` | Omitting `Egress` blocks inbound only; pods can still dial out freely. Always specify both for true isolation. |
| DNS egress rule | Forgetting UDP/53 is the most common mistake. Without it, your app cannot resolve any hostname and fails in non-obvious ways. |
| `namespaceSelector` for DNS | `kube-dns` lives in `kube-system`. A `podSelector` alone won't reach across namespaces. |
| Separate deny and allow manifests | Keeping them in two files makes it easy to apply the deny globally and manage per-app allows independently in GitOps. |

## Verify it worked

```bash
# Apply both manifests
kubectl apply -f default-deny-all.yaml
kubectl apply -f my-app-allow.yaml

# Confirm both policies exist in the namespace
kubectl get networkpolicy -n my-app

# Test ingress is blocked from an unauthorised pod
kubectl run test-blocked --rm -it --image=busybox --restart=Never -n my-app \
  -- wget -qO- --timeout=3 http://my-app:8080

# Test ingress is allowed from a pod with the frontend label
kubectl run test-allowed --rm -it --image=busybox --restart=Never -n my-app \
  --labels="app=frontend" -- wget -qO- --timeout=3 http://my-app:8080
```

Expected: the blocked pod times out; the labelled pod gets a response.

> **Note:** NetworkPolicy enforcement requires a CNI plugin that supports it (Calico, Cilium, Weave). Vanilla `kubenet` ignores these rules silently.

## Full walkthrough

Step-by-step breakdown of selectors, namespace isolation, and testing strategies → **Tutorial Pair 8: Networking** *(coming soon)*.
