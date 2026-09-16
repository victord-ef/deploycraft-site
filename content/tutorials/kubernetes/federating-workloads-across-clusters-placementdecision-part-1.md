---
title: "Federating Workloads Across Clusters Using PlacementDecision — Part 1: Adopting the Placement API"
date: 2026-08-28
description: "Multi-cluster Kubernetes is no longer niche — it is the default architecture for resilient, regulated, or globally distributed platforms. The OCM Placement API gives you a declarative, scheduler-driven model for expressing where workloads should run. This is how to adopt it."
cluster: "Kubernetes"
series: "Multi-cluster Scheduling"
part: 1
difficulty: "advanced"
duration: "45 min"
tags: ["kubernetes", "multi-cluster", "open-cluster-management", "placement", "platform-engineering", "federation"]
categories: ["tutorial"]
draft: false
toc: true
---

Running a single Kubernetes cluster is straightforward. Running three clusters in different regions, for different environments, with different compliance requirements, is a different discipline entirely. The question that defines whether you are operating multi-cluster infrastructure or just managing multiple clusters manually is this: **do you have a system that decides where workloads run, or do you decide yourself every time?**

Open Cluster Management (OCM) is the CNCF project that answers this question with a structured API. At its centre is the `Placement` resource — a declarative specification of where workloads should run — and the `PlacementDecision` resource, which is the scheduler's answer. This two-part series covers how to adopt the Placement API and how to use `PlacementDecision` to drive real workload scheduling.

Part 1 covers the foundation: hub and spoke architecture, cluster registration, and how to write `Placement` objects that express meaningful scheduling intent. [Part 2](/tutorials/kubernetes/federating-workloads-across-clusters-placementdecision-part-2/) covers what happens next — reading `PlacementDecision`, integrating with GitOps, handling cluster churn, and monitoring the scheduling layer.

---

## The hub and spoke model

OCM uses a hub and spoke architecture. One cluster — the hub — runs the OCM control plane and holds the global state of your fleet. Every other cluster — the managed clusters — runs a lightweight agent (`klusterlet`) that maintains a secure connection back to the hub.

```
              ┌─────────────────────┐
              │      Hub Cluster    │
              │                     │
              │  Placement API      │
              │  ManifestWork API   │
              │  ManagedCluster API │
              └────────┬────────────┘
                       │
         ┌─────────────┼─────────────┐
         │             │             │
    ┌────┴────┐   ┌────┴────┐   ┌────┴────┐
    │Managed  │   │Managed  │   │Managed  │
    │Cluster A│   │Cluster B│   │Cluster C│
    │(EU)     │   │(US)     │   │(APAC)   │
    └─────────┘   └─────────┘   └─────────┘
```

The hub never pushes workloads directly — it writes `ManifestWork` objects, and the klusterlet agent on each managed cluster pulls and applies them. This pull model means managed clusters do not need inbound connectivity from the hub, only outbound. It also means a hub outage does not immediately affect running workloads — agents continue applying their last known state until the connection recovers.

The hub cluster itself is just a Kubernetes cluster. It is common to run it on a small, dedicated cluster or on an existing management plane cluster. It should not run application workloads.

---

## Installing OCM

The quickest path to a working hub is `clusteradm`:

```bash
# Install clusteradm CLI
curl -L https://raw.githubusercontent.com/open-cluster-management-io/clusteradm/main/install.sh | bash

# Initialise the hub (run against your hub cluster context)
clusteradm init --wait

# Output includes a join command with a bootstrap token — save it
# Example output:
# clusteradm join --hub-apiserver https://hub.example.com:6443 \
#   --hub-token <token> \
#   --cluster-name <cluster-name>
```

Register a managed cluster by running the join command against its kubeconfig:

```bash
# Run against managed cluster context
clusteradm join \
  --hub-apiserver https://hub.example.com:6443 \
  --hub-token <bootstrap-token> \
  --cluster-name prod-eu-west-1 \
  --wait

# Back on the hub — approve the join request
clusteradm accept --clusters prod-eu-west-1
```

After acceptance, a `ManagedCluster` object appears on the hub representing the registered cluster. Repeat the join and accept cycle for every cluster in your fleet.

---

## ManagedCluster: the scheduling primitive

Every registered cluster is represented as a `ManagedCluster` object on the hub. This object carries two categories of information that the placement scheduler uses: **labels** and **cluster claims**.

Labels are applied by platform operators and represent stable, intentional attributes of the cluster:

```yaml
apiVersion: cluster.open-cluster-management.io/v1
kind: ManagedCluster
metadata:
  name: prod-eu-west-1
  labels:
    environment: production
    region: eu-west-1
    cloud: aws
    compliance: gdpr
spec:
  hubAcceptsClient: true
```

Cluster claims are key-value pairs reported by the klusterlet agent from inside the managed cluster. They represent facts the cluster knows about itself — Kubernetes version, node count, installed operators, available capacity:

```yaml
status:
  clusterClaims:
    - name: platform.open-cluster-management.io/product
      value: OpenShift
    - name: version.open-cluster-management.io/kubernetes
      value: v1.30.1
    - name: id.k8s.io
      value: a1b2c3d4-e5f6-7890-abcd-ef1234567890
```

You can define custom claims. A common pattern is to have the klusterlet report available GPU count, a compliance certification level, or the last successful audit timestamp. These claims then become first-class inputs to your placement policies.

---

## ManagedClusterSet: grouping clusters for governance

Before writing a `Placement`, you need to understand `ManagedClusterSet`. A `ClusterSet` is a named group of clusters used to scope placement decisions and enforce access boundaries.

```yaml
apiVersion: cluster.open-cluster-management.io/v1beta2
kind: ManagedClusterSet
metadata:
  name: production-clusters
```

Clusters are assigned to a set via label:

```yaml
# On each ManagedCluster
metadata:
  labels:
    cluster.open-cluster-management.io/clusterset: production-clusters
```

A `ManagedClusterSetBinding` grants a namespace on the hub access to schedule against clusters in a given set:

```yaml
apiVersion: cluster.open-cluster-management.io/v1beta2
kind: ManagedClusterSetBinding
metadata:
  name: production-clusters
  namespace: my-workload-namespace
spec:
  clusterSet: production-clusters
```

This binding is a governance control. A `Placement` in `my-workload-namespace` can only target clusters that are bound to that namespace via a `ManagedClusterSetBinding`. Teams cannot accidentally (or intentionally) schedule workloads to clusters they have not been granted access to through this mechanism.

The typical model:
- **global** ClusterSet: all clusters, hub-admin namespace only
- **production-clusters** ClusterSet: prod clusters, bound to production namespaces
- **non-production-clusters** ClusterSet: dev/staging, bound to developer namespaces

---

## Writing your first Placement

A `Placement` object is a scheduling policy. It declares constraints (which clusters are eligible) and, optionally, prioritisation (which eligible clusters are preferred). The placement controller evaluates it continuously and writes the result to `PlacementDecision` objects.

### Label selector predicates

The simplest placement: all clusters in a bound cluster set that carry the `environment=production` label.

```yaml
apiVersion: cluster.open-cluster-management.io/v1beta1
kind: Placement
metadata:
  name: production-placement
  namespace: my-workload-namespace
spec:
  clusterSets:
    - production-clusters
  predicates:
    - requiredClusterSelector:
        labelSelector:
          matchLabels:
            environment: production
```

Add a region constraint for data residency:

```yaml
spec:
  predicates:
    - requiredClusterSelector:
        labelSelector:
          matchExpressions:
            - key: environment
              operator: In
              values: [production]
            - key: region
              operator: In
              values: [eu-west-1, eu-central-1]
```

### Cluster claim predicates

Target clusters by facts they report about themselves — useful when labels are operator-managed but claims are self-reported:

```yaml
spec:
  predicates:
    - requiredClusterSelector:
        claimSelector:
          matchExpressions:
            - key: version.open-cluster-management.io/kubernetes
              operator: In
              values: ["v1.29.0", "v1.30.0", "v1.30.1"]
```

This targets only clusters running a specific Kubernetes version range — useful during rolling upgrades where you want to stage workload migration.

### Limiting cluster count

By default, a `Placement` targets all eligible clusters. To target a specific number:

```yaml
spec:
  numberOfClusters: 3
  predicates:
    - requiredClusterSelector:
        labelSelector:
          matchLabels:
            environment: production
```

If more than three clusters match, the prioritisers (covered in Part 2) determine which three are selected.

---

## Taints and tolerations at the cluster level

OCM mirrors the pod taint/toleration model at the cluster level, giving platform operators a way to temporarily exclude clusters from scheduling without deleting them or modifying labels.

Apply a taint to a cluster undergoing maintenance:

```yaml
apiVersion: cluster.open-cluster-management.io/v1
kind: ManagedCluster
metadata:
  name: prod-eu-west-1
spec:
  taints:
    - key: maintenance
      value: "true"
      effect: NoSelect
      timeAdded: "2026-08-28T02:00:00Z"
```

`NoSelect` prevents any new `Placement` from selecting this cluster. Existing decisions are unaffected unless the placement policy specifically re-evaluates. `NoSelectIfNew` is the softer variant — the cluster remains in existing decisions but is excluded from new ones.

A `Placement` can tolerate specific taints, allowing privileged workloads (monitoring agents, log shippers) to schedule even to tainted clusters:

```yaml
spec:
  tolerations:
    - key: maintenance
      operator: Equal
      value: "true"
      effect: NoSelect
```

This is the correct mechanism for deploying cluster-level infrastructure that must run regardless of maintenance state.

---

## Verifying placement decisions

After creating a `Placement`, inspect what the scheduler decided:

```bash
# List all PlacementDecision objects for a placement
kubectl get placementdecisions \
  -n my-workload-namespace \
  -l cluster.open-cluster-management.io/placement=production-placement

# Describe a decision to see the cluster list
kubectl describe placementdecision production-placement-decision-1 \
  -n my-workload-namespace
```

A healthy decision looks like:

```
Status:
  Decisions:
    Cluster Name:  prod-eu-west-1
    Reason:
    Cluster Name:  prod-us-east-1
    Reason:
    Cluster Name:  prod-ap-southeast-1
    Reason:
```

An empty `reason` field means the cluster was selected cleanly. A non-empty reason signals a constraint or scoring issue — the scheduler documents why a cluster scored lower or was excluded.

If the decision list is shorter than expected, check:

```bash
# View placement status conditions
kubectl get placement production-placement \
  -n my-workload-namespace \
  -o jsonpath='{.status.conditions}' | jq .
```

`PlacementSatisfied=False` with a message indicates no clusters matched — usually a label mismatch, a missing `ManagedClusterSetBinding`, or all matching clusters are tainted.

---

## What comes next

You now have a `Placement` that continuously evaluates your cluster fleet and writes `PlacementDecision` objects reflecting the current scheduling result. The placement controller handles additions, removals, taint changes, and cluster health transitions automatically — your policy stays static, the decisions adapt.

What you do not yet have is anything acting on those decisions. A `PlacementDecision` sitting unread is just metadata.

[Part 2](/tutorials/kubernetes/federating-workloads-across-clusters-placementdecision-part-2/) covers exactly this: how to read `PlacementDecision` correctly across pagination boundaries, how to wire it to GitOps engines and `ManifestWork` controllers, how prioritisers shape which clusters are selected when the eligible set is larger than `numberOfClusters`, and how to handle cluster churn in production without manual intervention.
