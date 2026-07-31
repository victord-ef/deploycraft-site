---
title: "Federating Workloads Across Clusters Using PlacementDecision"
description: "Build a federation controller that reads PlacementDecision objects and reconciles a workload across multiple Kubernetes clusters, with namespace parity, failure handling, and cross-cluster observability."
weight: 20
toc: true
draft: true
---

The
[first part](/docs/kubernetes/placement-decision-api-adoption-guide/)
of this series covered the `PlacementDecision` API — how schedulers write
decisions and how consumers read them. This post takes the next step:
building a controller that turns those decisions into running workloads
across clusters.

By the end of this post you will have a federation controller that watches
`PlacementDecision` objects, reconciles a `Deployment` on every selected
cluster, handles cluster failures gracefully, and reports per-cluster
rollout status back to the hub.

## What federation adds on top of placement

A `PlacementDecision` answers the question _where_ a workload should run.
Federation answers the question _how_ to get it there and keep it there.

The gap between the two includes:

- Creating and maintaining the target namespace on each spoke cluster.
- Applying the `Deployment`, `Service`, and supporting resources on each
  cluster independently.
- Detecting when a spoke cluster becomes unreachable and stopping
  reconciliation without removing the workload.
- Propagating rollout status from each spoke back to the hub so operators
  have a single view of the fleet.

A federation controller closes this gap by sitting between the
`PlacementDecision` (produced by a scheduler) and the spoke clusters
(where the workload runs).

## Prerequisites

- A hub cluster with the `PlacementDecision` CRD installed and
  `ClusterProfile` objects for each spoke.
- Spoke clusters reachable from the hub via kubeconfig or in-cluster
  credentials stored as Secrets.
- Go 1.21+ and `controller-runtime` v0.17+.
- The `cluster-inventory-api` module from Part 1.

## Designing the federation controller

The controller watches two resource types on the hub:

1. `PlacementDecision` — to know which clusters should run the workload.
2. A `FederatedDeployment` custom resource — the workload descriptor that
   references a placement by label selector.

On every reconcile loop the controller:

1. Lists all `PlacementDecision` objects matching the `FederatedDeployment`'s
   placement key.
2. Unions the cluster set across all slices.
3. For each cluster: ensures the namespace exists, applies the `Deployment`,
   and records rollout status.
4. For clusters that were previously in the set but are no longer: removes
   the `Deployment` (or leaves it in place, depending on the eviction
   policy).

## Defining the FederatedDeployment CRD

A minimal `FederatedDeployment` spec ties a placement key to a workload
template:

```go
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type FederatedDeployment struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec              FederatedDeploymentSpec   `json:"spec"`
    Status            FederatedDeploymentStatus `json:"status,omitempty"`
}

type FederatedDeploymentSpec struct {
    // PlacementKey selects PlacementDecisions by the decision-key label.
    PlacementKey string `json:"placementKey"`
    // Template is the Deployment to apply on each selected cluster.
    Template appsv1.DeploymentSpec `json:"template"`
    // EvictOnRemoval controls whether the Deployment is deleted when a
    // cluster is removed from the placement. Defaults to true.
    EvictOnRemoval *bool `json:"evictOnRemoval,omitempty"`
}

type ClusterRolloutStatus struct {
    ClusterName     string `json:"clusterName"`
    ReadyReplicas   int32  `json:"readyReplicas"`
    DesiredReplicas int32  `json:"desiredReplicas"`
    // Phase is one of: Pending, Progressing, Available, Degraded, Unreachable.
    Phase string `json:"phase"`
}

type FederatedDeploymentStatus struct {
    Clusters   []ClusterRolloutStatus `json:"clusters,omitempty"`
    TotalReady int32                  `json:"totalReady"`
}
```

## Ensuring namespace parity

Before applying any workload resources, the controller must ensure the
target namespace exists on the spoke cluster. A missing namespace causes
the `Deployment` apply to fail with a not-found error that would otherwise
be indistinguishable from a permission error.

```go
func ensureNamespace(ctx context.Context, client kubernetes.Interface, name string) error {
    _, err := client.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
    if err == nil {
        return nil
    }
    if !errors.IsNotFound(err) {
        return fmt.Errorf("checking namespace %s: %w", name, err)
    }

    ns := &corev1.Namespace{
        ObjectMeta: metav1.ObjectMeta{
            Name: name,
            Labels: map[string]string{
                "app.kubernetes.io/managed-by": "federation-controller",
            },
        },
    }
    _, err = client.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
    if errors.IsAlreadyExists(err) {
        return nil
    }
    return err
}
```

Call this before every apply. The `IsAlreadyExists` guard handles the race
between a concurrent Create from another reconciler goroutine.

## Applying the workload with server-side apply

Use server-side apply rather than a Create/Update pattern. SSA handles
the three-way merge correctly, avoids last-write-wins conflicts, and lets
the controller declare its field manager so other tools can coexist on
the same object.

```go
func applyDeployment(
    ctx context.Context,
    client kubernetes.Interface,
    namespace string,
    spec appsv1.DeploymentSpec,
    name string,
) error {
    deploy := &appsv1.Deployment{
        TypeMeta: metav1.TypeMeta{
            APIVersion: "apps/v1",
            Kind:       "Deployment",
        },
        ObjectMeta: metav1.ObjectMeta{
            Name:      name,
            Namespace: namespace,
            Labels: map[string]string{
                "app.kubernetes.io/managed-by": "federation-controller",
            },
        },
        Spec: spec,
    }

    data, err := json.Marshal(deploy)
    if err != nil {
        return err
    }

    _, err = client.AppsV1().Deployments(namespace).Patch(
        ctx,
        name,
        types.ApplyPatchType,
        data,
        metav1.PatchOptions{FieldManager: "federation-controller", Force: pointer.Bool(true)},
    )
    return err
}
```

The `Force: true` flag allows the controller to take ownership of fields
that were previously managed by another field manager (for example, a
manually applied Deployment during initial setup).

## Handling unreachable clusters

A spoke cluster that becomes unreachable returns network errors on every
API call. The controller must distinguish between a transient error (retry)
and a cluster that has been unavailable long enough to mark as `Unreachable`
(skip and surface in status).

```go
const unreachableThreshold = 3 * time.Minute

func (r *Reconciler) reconcileCluster(
    ctx context.Context,
    cluster ciav1alpha1.ClusterDecision,
    spec appsv1.DeploymentSpec,
    namespace, deployName string,
) ClusterRolloutStatus {
    status := ClusterRolloutStatus{ClusterName: cluster.ClusterProfileRef.Name}

    spokeClient, err := r.clientForCluster(ctx, cluster)
    if err != nil {
        status.Phase = "Unreachable"
        return status
    }

    // Check if the cluster has been unreachable long enough to skip.
    if r.unreachableSince[cluster.ClusterProfileRef.Name].Add(unreachableThreshold).Before(time.Now()) {
        status.Phase = "Unreachable"
        return status
    }

    if err := ensureNamespace(ctx, spokeClient, namespace); err != nil {
        r.recordUnreachable(cluster.ClusterProfileRef.Name)
        status.Phase = "Unreachable"
        return status
    }

    if err := applyDeployment(ctx, spokeClient, namespace, spec, deployName); err != nil {
        status.Phase = "Degraded"
        return status
    }

    // Clear the unreachable timestamp on success.
    delete(r.unreachableSince, cluster.ClusterProfileRef.Name)

    deploy, err := spokeClient.AppsV1().Deployments(namespace).Get(ctx, deployName, metav1.GetOptions{})
    if err != nil {
        status.Phase = "Pending"
        return status
    }

    status.ReadyReplicas = deploy.Status.ReadyReplicas
    status.DesiredReplicas = *deploy.Spec.Replicas
    status.Phase = rolloutPhase(deploy)
    return status
}

func rolloutPhase(d *appsv1.Deployment) string {
    if d.Status.ReadyReplicas == *d.Spec.Replicas {
        return "Available"
    }
    if d.Status.UpdatedReplicas < *d.Spec.Replicas {
        return "Progressing"
    }
    return "Degraded"
}
```

## Evicting workloads from removed clusters

When a cluster is removed from the `PlacementDecision`, the controller
compares the previous cluster set (recorded in the `FederatedDeployment`
status) against the current set and deletes the `Deployment` from any
cluster that has been dropped — unless `evictOnRemoval` is set to `false`.

```go
func (r *Reconciler) evictRemoved(
    ctx context.Context,
    fd *fedv1alpha1.FederatedDeployment,
    currentClusters map[string]bool,
) {
    evict := fd.Spec.EvictOnRemoval == nil || *fd.Spec.EvictOnRemoval

    for _, prev := range fd.Status.Clusters {
        if currentClusters[prev.ClusterName] {
            continue
        }
        if !evict {
            continue
        }
        spokeClient, err := r.clientForClusterByName(ctx, prev.ClusterName)
        if err != nil {
            continue
        }
        _ = spokeClient.AppsV1().Deployments(fd.Namespace).Delete(
            ctx, fd.Name, metav1.DeleteOptions{},
        )
    }
}
```

Set `evictOnRemoval: false` for stateful workloads or when you want the
workload to drain naturally before removal.

## Reporting status back to the hub

After reconciling all clusters, patch the `FederatedDeployment` status
with the per-cluster rollout summary:

```go
func (r *Reconciler) updateStatus(
    ctx context.Context,
    fd *fedv1alpha1.FederatedDeployment,
    clusterStatuses []ClusterRolloutStatus,
) error {
    var totalReady int32
    for _, s := range clusterStatuses {
        totalReady += s.ReadyReplicas
    }

    fd.Status.Clusters = clusterStatuses
    fd.Status.TotalReady = totalReady

    return r.Status().Update(ctx, fd)
}
```

Operators can then check the fleet rollout status with a single command:

```bash
kubectl get federateddeployment my-app -o jsonpath='{.status.clusters[*]}'
```

Or with a table output configured in the CRD's `additionalPrinterColumns`:

```
NAME      TOTAL-READY   CLUSTERS
my-app    4             3
```

## Putting the reconcile loop together

```go
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    var fd fedv1alpha1.FederatedDeployment
    if err := r.Get(ctx, req.NamespacedName, &fd); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }

    // Collect all clusters from PlacementDecisions matching the placement key.
    decisions, err := r.collectDecisions(ctx, fd.Namespace, fd.Spec.PlacementKey)
    if err != nil {
        return ctrl.Result{}, err
    }

    currentClusters := make(map[string]bool, len(decisions))
    statuses := make([]ClusterRolloutStatus, 0, len(decisions))

    for _, d := range decisions {
        currentClusters[d.ClusterProfileRef.Name] = true
        status := r.reconcileCluster(ctx, d, fd.Spec.Template, fd.Namespace, fd.Name)
        statuses = append(statuses, status)
    }

    r.evictRemoved(ctx, &fd, currentClusters)

    if err := r.updateStatus(ctx, &fd, statuses); err != nil {
        return ctrl.Result{}, err
    }

    // Re-queue every 30 seconds to catch spoke-side changes.
    return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}
```

The 30-second requeue catches rollout progress on spoke clusters without
requiring the controller to set up watches on every spoke's API server.
For lower latency, add a watch on the spoke `Deployment` status using a
remote informer.

## Testing with a real workload

Deploy the federation controller to the hub cluster, then create a
`FederatedDeployment`:

```yaml
apiVersion: federation.example.com/v1alpha1
kind: FederatedDeployment
metadata:
  name: nginx
  namespace: default
spec:
  placementKey: my-placement
  template:
    replicas: 2
    selector:
      matchLabels:
        app: nginx
    template:
      metadata:
        labels:
          app: nginx
      spec:
        containers:
        - name: nginx
          image: registry.example.com/nginx:1.27
          resources:
            requests:
              cpu: 50m
              memory: 64Mi
            limits:
              cpu: 100m
              memory: 128Mi
```

Apply it and watch the controller reconcile across clusters:

```bash
kubectl apply -f nginx-federated.yaml
kubectl get federateddeployment nginx -w
```

```
NAME    TOTAL-READY   CLUSTERS
nginx   0             2
nginx   2             2
nginx   4             2
```

Verify on each spoke:

```bash
kubectl get deployment nginx -n default --context cluster-us-east-1
kubectl get deployment nginx -n default --context cluster-eu-west-1
```

## What comes next

This controller handles the core federation loop. Production systems
typically extend it with:

- **Progressive rollout** — apply the updated `Deployment` to one cluster
  at a time, wait for it to reach `Available`, then continue. Gate on
  per-cluster `ReadyReplicas` before moving forward.
- **Cross-cluster service discovery** — federate a `ServiceExport` alongside
  the `Deployment` so the workload is reachable across clusters via the
  [MCS API](https://github.com/kubernetes-sigs/mcs-api).
- **Metrics aggregation** — scrape per-cluster Prometheus metrics and
  federate them to a central store so you can alert on fleet-wide error
  rates rather than per-cluster thresholds.

For teams running Open Cluster Management or Karmada, the federation
pattern in this post maps directly to how their built-in work propagation
controllers operate — understanding the mechanics makes it easier to
extend or debug them when they diverge from expected behaviour.
