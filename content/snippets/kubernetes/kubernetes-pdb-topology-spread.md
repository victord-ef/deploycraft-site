---
title: "Kubernetes — PodDisruptionBudget and Topology Spread Constraints"
date: 2026-11-12
description: "PodDisruptionBudget to protect availability during node drains and cluster upgrades, combined with topology spread constraints to distribute replicas across zones and nodes — the two manifests every production Deployment needs alongside its HPA."
lang: "Kubernetes"
tags: ["kubernetes", "pdb", "topology-spread", "high-availability", "disruption", "zones"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-11-12"
draft: false
---

## When to use this

For any workload where downtime costs money or violates an SLA. A PodDisruptionBudget prevents node maintenance from evicting all pods simultaneously. Topology spread constraints ensure replicas are spread across failure domains so a single zone outage or node failure does not take the whole workload offline.

## Without it

```
Cluster upgrade scenario — 3 replicas, all on the same node:
  Node drains → all 3 pods evicted simultaneously → service unavailable
  New pods schedule → readiness probe delay → 15–30s of downtime per drain
```

## Snippet

### PodDisruptionBudget

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: my-app
  namespace: my-app
spec:
  # Guarantee at least 2 pods are available during voluntary disruptions
  # (node drain, cluster upgrade, manual eviction)
  minAvailable: 2

  # Alternatively: maxUnavailable — allow at most 1 pod to be unavailable
  # maxUnavailable: 1

  selector:
    matchLabels:
      app: my-app            # must match the pod labels in the Deployment
```

### Topology spread constraints — spread across zones and nodes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: my-app
spec:
  replicas: 6
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      # ── Topology spread across availability zones ───────────────────────────
      topologySpreadConstraints:
        - maxSkew: 1                        # max difference in pod count between zones
          topologyKey: topology.kubernetes.io/zone
          whenUnsatisfiable: DoNotSchedule  # hard constraint — pod stays Pending if violated
          labelSelector:
            matchLabels:
              app: my-app

        # ── Also spread across individual nodes within each zone ──────────────
        - maxSkew: 1
          topologyKey: kubernetes.io/hostname
          whenUnsatisfiable: ScheduleAnyway  # soft constraint — schedule even if skewed
          labelSelector:
            matchLabels:
              app: my-app

      # ── Anti-affinity as a fallback for clusters without zone labels ─────────
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - weight: 100
              podAffinityTerm:
                labelSelector:
                  matchLabels:
                    app: my-app
                topologyKey: kubernetes.io/hostname

      containers:
        - name: app
          image: my-app:1.0.0
          resources:
            requests:
              cpu: 250m
              memory: 256Mi
            limits:
              cpu: 500m
              memory: 512Mi
```

### Combined — full high-availability configuration

```yaml
# PDB + topology spread for a 3-replica minimum-viable HA setup
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: my-app
  namespace: my-app
spec:
  minAvailable: 2
  selector:
    matchLabels:
      app: my-app

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: my-app
spec:
  replicas: 3
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      topologySpreadConstraints:
        - maxSkew: 1
          topologyKey: topology.kubernetes.io/zone
          whenUnsatisfiable: DoNotSchedule
          labelSelector:
            matchLabels:
              app: my-app
      containers:
        - name: app
          image: my-app:1.0.0
          resources:
            requests:
              cpu: 250m
              memory: 256Mi
            limits:
              cpu: 500m
              memory: 512Mi
          readinessProbe:
            httpGet:
              path: /healthz
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 5
```

**Key decisions:**

| Decision | Why it matters |
|---|---|
| `minAvailable: 2` over `maxUnavailable: 1` | For a 3-replica deployment, both express the same constraint. But if the HPA scales down to 2 replicas, `minAvailable: 2` blocks all evictions while `maxUnavailable: 1` still allows one pod to be drained — taking the deployment to 1 replica. Use `minAvailable` as an absolute floor. |
| `DoNotSchedule` on zone spread, `ScheduleAnyway` on node spread | Zone spread is a hard availability requirement — a pod that can only schedule in one zone defeats the purpose of multi-zone. Node spread is best-effort — blocking scheduling because one node has one more pod than another is too restrictive. |
| `maxSkew: 1` | Allows at most a 1-pod difference between the most-loaded and least-loaded zone. `maxSkew: 2` with 3 zones and 3 replicas allows all 3 replicas to land in one zone — useless for HA. |
| `topology.kubernetes.io/zone` key | Standard label applied by cloud providers (EKS, GKE, AKS) to every node. Confirm your nodes have this label before using it — otherwise all pods land on the first node and the constraint is silently satisfied. |
| PDB and HPA together | The HPA can scale down to `minReplicas`. If `minReplicas: 2` and `minAvailable: 2`, any node drain is blocked — there are no pods to spare. Set `minAvailable` to `minReplicas - 1` or use a percentage: `minAvailable: 50%`. |
| `readinessProbe` is mandatory | The PDB protects pods that are `Ready`. A pod with no readiness probe is considered ready immediately on start — the new pod may replace a drained pod before it can actually serve traffic, causing a brief gap despite the PDB. |

## Verify it worked

```bash
# Confirm pods are spread across zones
kubectl get pods -n my-app -o wide \
  --label-columns topology.kubernetes.io/zone

# Simulate a node drain and confirm PDB blocks it when at minimum
kubectl drain <node-name> --ignore-daemonsets --delete-emptydir-data

# If PDB is protecting the pods, the drain will output:
# error: cannot evict pod as it would violate the pod's disruption budget

# View PDB status
kubectl describe pdb my-app -n my-app
# Check: Allowed disruptions — must be > 0 for drains to proceed

# Check zone distribution via node labels
kubectl get nodes --label-columns topology.kubernetes.io/zone
```
