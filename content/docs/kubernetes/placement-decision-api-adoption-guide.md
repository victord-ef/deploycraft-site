---
title: "PlacementDecision API Adoption Guide"
description: "A vendor-neutral standard for publishing multi-cluster scheduling results — how producers write PlacementDecision objects and how consumers read, filter, and act on them."
weight: 10
toc: true
draft: true
---

The `PlacementDecision` API (KEP-5313) is a vendor-neutral standard for publishing multi-cluster scheduling results. It decouples schedulers from consumers: any scheduler that writes `PlacementDecision` objects can be read by any consumer without custom integrations.

---

## Prerequisites

- A Kubernetes hub cluster with the cluster-inventory-api CRDs installed.
- `ClusterProfile` objects already exist for each managed cluster.
- Go 1.21+ if building a custom producer or consumer.

---

## Installing the CRD

Apply the `PlacementDecision` CRD from the cluster-inventory-api repository:

```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/cluster-inventory-api/main/config/crd/bases/multicluster.x-k8s.io_placementdecisions.yaml
```

Verify the CRD is registered:

```bash
kubectl get crd placementdecisions.multicluster.x-k8s.io
```

---

## Producer Guide (Scheduler Implementers)

A **producer** is a scheduler or placement controller that selects clusters and writes the result as one or more `PlacementDecision` objects.

### Importing the Client

Add the module to your `go.mod`:

```bash
go get sigs.k8s.io/cluster-inventory-api
```

Import the generated clientset and API types:

```go
import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    ciaclient "sigs.k8s.io/cluster-inventory-api/client/clientset/versioned"
    ciav1alpha1 "sigs.k8s.io/cluster-inventory-api/apis/v1alpha1"
)
```

Build the clientset from a `*rest.Config` (hub cluster):

```go
client, err := ciaclient.NewForConfig(hubConfig)
if err != nil {
    return fmt.Errorf("building cluster-inventory client: %w", err)
}
```

### Creating a PlacementDecision

A `PlacementDecision` holds up to 100 `ClusterDecision` entries (following the EndpointSlice convention). Each entry points to a `ClusterProfile` by name.

```go
pd := &ciav1alpha1.PlacementDecision{
    ObjectMeta: metav1.ObjectMeta{
        // Name should be deterministic and unique within the namespace.
        // A common pattern: <placement-name>-<index>
        Name:      "my-placement-1",
        Namespace: "default",
        Labels: map[string]string{
            // Required when more than one slice exists for the same logical placement.
            ciav1alpha1.DecisionKeyLabel: "my-placement",
            // Slice index (zero-based) for ordering across slices.
            ciav1alpha1.DecisionIndexLabel: "0",
            // Optional: link this decision to a higher-level workload or placement request.
            ciav1alpha1.PlacementKeyLabel: "my-workload",
        },
    },
    SchedulerName: "my-scheduler",
    Decisions: []ciav1alpha1.ClusterDecision{
        {
            ClusterProfileRef: ciav1alpha1.ClusterProfileReference{
                Name:      "cluster-us-east-1",
                Namespace: "default", // omit to inherit the PlacementDecision's namespace
            },
            Reason: "lowest-latency region for this workload",
        },
        {
            ClusterProfileRef: ciav1alpha1.ClusterProfileReference{
                Name: "cluster-eu-west-1",
            },
            Reason: "GDPR-compliant region",
        },
    },
}

_, err = client.ApisV1alpha1().PlacementDecisions("default").Create(
    ctx, pd, metav1.CreateOptions{},
)
```

**Field reference:**

| Field | Required | Description |
|-------|----------|-------------|
| `decisions` | Yes | List of selected clusters (max 100 per object). |
| `decisions[].clusterProfileRef.name` | Yes | Name of the target `ClusterProfile`. |
| `decisions[].clusterProfileRef.namespace` | No | Namespace of the `ClusterProfile`; defaults to the `PlacementDecision`'s namespace. |
| `decisions[].reason` | No | Human-readable explanation for why this cluster was chosen. Useful for auditing. |
| `schedulerName` | No | Name of the scheduler that created this decision. Consumers can filter on this. |

**Label reference:**

| Label | Description |
|-------|-------------|
| `multicluster.x-k8s.io/decision-key` | Groups all slices that belong to the same logical placement decision. **Required when more than one slice exists.** |
| `multicluster.x-k8s.io/decision-index` | Zero-based integer index indicating slice order within a multi-slice decision. |
| `multicluster.x-k8s.io/placement-key` | Links this decision to an upstream placement request or workload object. Optional. |

### The Multi-Slice Pattern

When a scheduler selects more than 100 clusters, the decisions must be split across multiple `PlacementDecision` objects. This mirrors the EndpointSlice pattern used by Kubernetes Services.

**Rules for producers:**

1. Set the same `multicluster.x-k8s.io/decision-key` label on every slice.
2. Set `multicluster.x-k8s.io/decision-index` to `"0"`, `"1"`, `"2"`, ... on each slice.
3. Each slice holds at most 100 decisions.
4. Consumers MUST union all slices with the same `decision-key` to obtain the full cluster set.

```go
const decisionKey = "my-large-placement"

func createSlices(ctx context.Context, client ciaclient.Interface, namespace string, allClusters []string) error {
    const maxPerSlice = 100

    for i := 0; i < len(allClusters); i += maxPerSlice {
        end := i + maxPerSlice
        if end > len(allClusters) {
            end = len(allClusters)
        }
        batch := allClusters[i:end]
        index := i / maxPerSlice

        decisions := make([]ciav1alpha1.ClusterDecision, 0, len(batch))
        for _, clusterName := range batch {
            decisions = append(decisions, ciav1alpha1.ClusterDecision{
                ClusterProfileRef: ciav1alpha1.ClusterProfileReference{Name: clusterName},
            })
        }

        pd := &ciav1alpha1.PlacementDecision{
            ObjectMeta: metav1.ObjectMeta{
                Name:      fmt.Sprintf("%s-%d", decisionKey, index),
                Namespace: namespace,
                Labels: map[string]string{
                    ciav1alpha1.DecisionKeyLabel:   decisionKey,
                    ciav1alpha1.DecisionIndexLabel: strconv.Itoa(index),
                },
            },
            SchedulerName: "my-scheduler",
            Decisions:     decisions,
        }

        _, err := client.ApisV1alpha1().PlacementDecisions(namespace).Create(
            ctx, pd, metav1.CreateOptions{},
        )
        if err != nil {
            return fmt.Errorf("creating slice %d: %w", index, err)
        }
    }
    return nil
}
```

### Updating Decisions

When the set of selected clusters changes (e.g., a cluster becomes unhealthy), update the relevant slices in place. For small changes, patch individual slices. For large reshuffles, replace all slices atomically.

```go
// Fetch the existing slice
existing, err := client.ApisV1alpha1().PlacementDecisions(namespace).Get(
    ctx, "my-placement-0", metav1.GetOptions{},
)
if err != nil {
    return err
}

// Replace the decisions list
existing.Decisions = newDecisions

_, err = client.ApisV1alpha1().PlacementDecisions(namespace).Update(
    ctx, existing, metav1.UpdateOptions{},
)
```

> **Tip:** Use `resourceVersion` optimistic locking (already present in `existing.ResourceVersion`) to avoid lost-update races when multiple reconcilers run concurrently.

### Cleaning Up Decisions

Delete all slices when a placement is retired. Use label selectors to batch-delete:

```bash
kubectl delete placementdecisions \
  -l multicluster.x-k8s.io/decision-key=my-placement \
  -n default
```

In Go:

```go
err = client.ApisV1alpha1().PlacementDecisions(namespace).DeleteCollection(
    ctx,
    metav1.DeleteOptions{},
    metav1.ListOptions{
        LabelSelector: fmt.Sprintf("%s=%s", ciav1alpha1.DecisionKeyLabel, decisionKey),
    },
)
```

---

## Consumer Guide

A **consumer** is any tool (operator, GitOps controller, batch system) that reads `PlacementDecision` objects to determine where to deploy workloads.

### Listing and Watching PlacementDecisions

```go
// List all PlacementDecisions in a namespace
list, err := client.ApisV1alpha1().PlacementDecisions(namespace).List(
    ctx, metav1.ListOptions{},
)

// Watch for changes
watcher, err := client.ApisV1alpha1().PlacementDecisions(namespace).Watch(
    ctx, metav1.ListOptions{},
)
for event := range watcher.ResultChan() {
    pd, ok := event.Object.(*ciav1alpha1.PlacementDecision)
    if !ok {
        continue
    }
    // handle ADDED, MODIFIED, DELETED
    fmt.Printf("event=%s name=%s decisions=%d\n", event.Type, pd.Name, len(pd.Decisions))
}
```

Using a controller-runtime informer cache is recommended for production consumers since it handles reconnection and list/watch bookkeeping automatically.

### Filtering by Scheduler

If multiple schedulers write decisions into the same namespace, consumers can filter by `schedulerName` after listing. The `schedulerName` field is not a label, so filter in-process:

```go
func filterByScheduler(list *ciav1alpha1.PlacementDecisionList, scheduler string) []ciav1alpha1.PlacementDecision {
    var result []ciav1alpha1.PlacementDecision
    for _, pd := range list.Items {
        if pd.SchedulerName == scheduler {
            result = append(result, pd)
        }
    }
    return result
}
```

### Correlating Multi-Slice Decisions

When a placement spans more than one slice, all slices share the same `decision-key` label. Union them to get the complete cluster set:

```go
func collectAllDecisions(
    ctx context.Context,
    client ciaclient.Interface,
    namespace, decisionKey string,
) ([]ciav1alpha1.ClusterDecision, error) {
    list, err := client.ApisV1alpha1().PlacementDecisions(namespace).List(
        ctx,
        metav1.ListOptions{
            LabelSelector: fmt.Sprintf("%s=%s",
                ciav1alpha1.DecisionKeyLabel, decisionKey),
        },
    )
    if err != nil {
        return nil, err
    }

    // Sort slices by decision-index to preserve insertion order.
    sort.Slice(list.Items, func(i, j int) bool {
        idxI, _ := strconv.Atoi(list.Items[i].Labels[ciav1alpha1.DecisionIndexLabel])
        idxJ, _ := strconv.Atoi(list.Items[j].Labels[ciav1alpha1.DecisionIndexLabel])
        return idxI < idxJ
    })

    var all []ciav1alpha1.ClusterDecision
    for _, pd := range list.Items {
        all = append(all, pd.Decisions...)
    }
    return all, nil
}
```

### Resolving Cluster References

Each `ClusterDecision` holds a `ClusterProfileRef`. Fetch the corresponding `ClusterProfile` to obtain connection details:

```go
decisions, err := collectAllDecisions(ctx, client, namespace, decisionKey)
if err != nil {
    return err
}

for _, d := range decisions {
    ns := d.ClusterProfileRef.Namespace
    if ns == "" {
        ns = namespace
    }
    cp, err := client.ApisV1alpha1().ClusterProfiles(ns).Get(
        ctx, d.ClusterProfileRef.Name, metav1.GetOptions{},
    )
    if err != nil {
        return fmt.Errorf("fetching ClusterProfile %s: %w", d.ClusterProfileRef.Name, err)
    }

    // Use the access package to build a *rest.Config for the spoke cluster.
    spokeConfig, err := accessCfg.BuildConfigFromCP(cp)
    if err != nil {
        return err
    }
    // Now you can create a client for that cluster.
    _ = spokeConfig
}
```

### Integration Patterns

**Deploying a ConfigMap to all selected clusters:**

```go
func deployConfigMap(ctx context.Context, decisions []ciav1alpha1.ClusterDecision,
    namespace string, accessCfg *access.Config, hubClient ciaclient.Interface) error {

    cm := &corev1.ConfigMap{
        ObjectMeta: metav1.ObjectMeta{Name: "app-config", Namespace: "default"},
        Data:       map[string]string{"key": "value"},
    }

    for _, d := range decisions {
        ns := d.ClusterProfileRef.Namespace
        if ns == "" {
            ns = namespace
        }
        cp, err := hubClient.ApisV1alpha1().ClusterProfiles(ns).Get(
            ctx, d.ClusterProfileRef.Name, metav1.GetOptions{},
        )
        if err != nil {
            return fmt.Errorf("get ClusterProfile %s: %w", d.ClusterProfileRef.Name, err)
        }

        spokeConfig, err := accessCfg.BuildConfigFromCP(cp)
        if err != nil {
            return err
        }

        spokeClient, err := k8sclient.NewForConfig(spokeConfig)
        if err != nil {
            return err
        }

        _, err = spokeClient.CoreV1().ConfigMaps("default").Apply(
            ctx, cm, metav1.ApplyOptions{FieldManager: "my-consumer"},
        )
        if err != nil {
            return fmt.Errorf("apply ConfigMap on %s: %w", d.ClusterProfileRef.Name, err)
        }
        log.Printf("applied ConfigMap on cluster %s", d.ClusterProfileRef.Name)
    }
    return nil
}
```

**GitOps tool integration:** A GitOps controller (e.g., Argo CD ApplicationSet, Flux) can watch `PlacementDecisions` via informers and generate one `Application`/`Kustomization` per cluster in `decisions`. Use the `decision-key` label to scope the watch to a specific placement.

**Batch job integration:** A batch controller can fan out jobs using the same pattern — list decisions, resolve each `ClusterProfile`, and submit a job to each spoke cluster's API server.

---

## End-to-End Walkthrough

This section shows the complete lifecycle: a cluster manager registers clusters, a scheduler selects them, and a consumer deploys a workload.

### Step 1 — Cluster Manager Creates ClusterProfiles

A cluster manager (e.g., Open Cluster Management, Karmada) creates a `ClusterProfile` for each managed cluster on the hub:

```yaml
apiVersion: multicluster.x-k8s.io/v1alpha1
kind: ClusterProfile
metadata:
  name: cluster-us-east-1
  namespace: default
  labels:
    multicluster.x-k8s.io/cluster-manager: my-cluster-manager
spec:
  displayName: "US East 1"
  clusterManager:
    name: my-cluster-manager
```

```bash
kubectl apply -f cluster-us-east-1.yaml
kubectl apply -f cluster-eu-west-1.yaml
```

### Step 2 — Scheduler Selects Clusters and Writes a PlacementDecision

The scheduler evaluates placement constraints (affinity, resource availability, region) and writes its result:

```yaml
apiVersion: multicluster.x-k8s.io/v1alpha1
kind: PlacementDecision
metadata:
  name: my-placement-0
  namespace: default
  labels:
    multicluster.x-k8s.io/decision-key: my-placement
    multicluster.x-k8s.io/decision-index: "0"
    multicluster.x-k8s.io/placement-key: my-workload
schedulerName: my-scheduler
decisions:
  - clusterProfileRef:
      name: cluster-us-east-1
    reason: "lowest latency for US users"
  - clusterProfileRef:
      name: cluster-eu-west-1
    reason: "GDPR compliance"
```

```bash
kubectl apply -f my-placement-0.yaml
```

### Step 3 — Consumer Reads the Decision and Deploys

```bash
# Inspect the current decision
kubectl get placementdecisions -n default -l multicluster.x-k8s.io/decision-key=my-placement -o yaml
```

The consumer code (see [Deploying a ConfigMap](#deploying-a-configmap-to-all-selected-clusters) above) reads the `PlacementDecision`, resolves each `ClusterProfile`, and applies the workload:

```
2026/06/06 12:00:01 applied ConfigMap on cluster cluster-us-east-1
2026/06/06 12:00:01 applied ConfigMap on cluster cluster-eu-west-1
```

### Step 4 — Scheduler Updates the Decision

If `cluster-eu-west-1` becomes unavailable, the scheduler replaces it with `cluster-ap-southeast-1`:

```bash
kubectl patch placementdecision my-placement-0 -n default --type=json \
  -p='[{"op": "replace", "path": "/decisions/1", "value": {"clusterProfileRef": {"name": "cluster-ap-southeast-1"}, "reason": "failover region"}}]'
```

The consumer's watch loop receives a `MODIFIED` event and re-deploys to the new cluster set automatically.

### Step 5 — Cleanup

When the placement is retired, delete all slices:

```bash
kubectl delete placementdecisions \
  -l multicluster.x-k8s.io/decision-key=my-placement \
  -n default
```

---

## Summary

| Role | Action |
|------|--------|
| **Producer** | Create `PlacementDecision` objects; use multi-slice pattern for >100 clusters; set `decision-key`, `decision-index`, and `schedulerName`. |
| **Consumer** | List/watch `PlacementDecisions`; union slices by `decision-key`; resolve `ClusterProfile` references to obtain spoke credentials. |

The shared contract — the `PlacementDecision` CRD — means any compliant scheduler works with any compliant consumer without bespoke integration code.

The [second part](/docs/kubernetes/federating-workloads-placementdecision/) builds on this foundation to construct a federation controller that reconciles a `Deployment` across all selected clusters, handles unreachable spokes, and reports per-cluster rollout status back to the hub.
