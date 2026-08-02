---
title: "Deploying KEDA for Event-Driven Autoscaling"
description: "Install and configure KEDA on a Kubernetes cluster, understand its architecture, and set up your first ScaledObject to autoscale a workload from an external event source."
weight: 10
toc: true
draft: true
---

The Horizontal Pod Autoscaler scales workloads based on CPU and memory. For most event-driven systems — Kafka consumers, queue processors, scheduled batch jobs — CPU is the wrong signal. A consumer can sit idle at 0% CPU while a Kafka topic accumulates millions of unprocessed messages.

KEDA (Kubernetes Event-Driven Autoscaling) solves this by letting you scale any Deployment or Job directly on external metrics: queue depth, Kafka consumer lag, Prometheus query results, or any of its 60+ built-in scalers. It integrates with the Kubernetes HPA rather than replacing it — KEDA feeds external metrics into the HPA, so you keep standard Kubernetes scaling behaviour with event-source awareness.

The [second part](/docs/kubernetes/scaling-kafka-lag-keda-scaledobjects/) covers a production Kafka lag scaler with authentication, lag thresholds, and ScaledJob for batch consumers.

## How KEDA works

KEDA installs three components into your cluster:

- **KEDA Operator** — watches `ScaledObject` and `ScaledJob` custom resources, creates an HPA for each, and manages the scaling lifecycle.
- **Metrics Server** — implements the `external.metrics.k8s.io` API extension so the HPA can query external event sources.
- **Admission Webhooks** — validate `ScaledObject` and `ScaledJob` resources at creation time.

The scaling loop:

1. You create a `ScaledObject` pointing at a Deployment and an event source (e.g., a Kafka topic).
2. KEDA Operator creates an HPA targeting that Deployment.
3. On each HPA evaluation interval, the HPA queries the KEDA Metrics Server.
4. KEDA Metrics Server polls the event source (e.g., reads consumer lag from Kafka).
5. The HPA scales the Deployment replica count based on the metric value and your configured threshold.

When the event source returns zero (empty queue, zero lag), KEDA can scale the Deployment to zero — something the standard HPA cannot do.

## Prerequisites

- Kubernetes 1.27+ cluster with `kubectl` access.
- Helm 3.x installed locally.
- Cluster-admin permissions (KEDA installs CRDs and cluster-scoped RBAC).

## Installing KEDA with Helm

Add the KEDA Helm repository:

```bash
helm repo add kedacore https://kedacore.github.io/charts
helm repo update
```

Install KEDA into its own namespace:

```bash
helm install keda kedacore/keda \
  --namespace keda \
  --create-namespace \
  --version 2.15.0
```

Verify all three components are running:

```bash
kubectl get pods -n keda
```

```
NAME                                               READY   STATUS    RESTARTS
keda-operator-6d9f9d6b7f-xk2p4                    1/1     Running   0
keda-operator-metrics-apiserver-7f8d4c9b5d-m9q7r   1/1     Running   0
keda-webhooks-5c8b9f7d6c-r4t8n                     1/1     Running   0
```

Confirm the KEDA Metrics Server is registered as an API extension:

```bash
kubectl get apiservice v1beta1.external.metrics.k8s.io
```

```
NAME                            SERVICE                             AVAILABLE
v1beta1.external.metrics.k8s.io   keda/keda-operator-metrics-apiserver   True
```

## KEDA CRDs

KEDA installs four CRDs:

| CRD | Purpose |
|-----|---------|
| `ScaledObject` | Scales a `Deployment`, `StatefulSet`, or custom workload based on external triggers. |
| `ScaledJob` | Creates Kubernetes `Jobs` in response to events — one Job per event batch. |
| `TriggerAuthentication` | Stores credentials for a scaler (API keys, connection strings) at namespace scope. |
| `ClusterTriggerAuthentication` | Same as above but cluster-scoped — shared across namespaces. |

## Your first ScaledObject

A `ScaledObject` connects a workload to one or more triggers. This example scales a Deployment based on the length of a Redis list — a common pattern for task queues:

```yaml
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: redis-queue-scaler
  namespace: default
spec:
  scaleTargetRef:
    name: queue-worker
  minReplicaCount: 0
  maxReplicaCount: 20
  pollingInterval: 15
  cooldownPeriod: 60
  triggers:
  - type: redis
    metadata:
      address: redis-service.default.svc.cluster.local:6379
      listName: task-queue
      listLength: "5"
```

**Field reference:**

| Field | Description |
|-------|-------------|
| `scaleTargetRef.name` | Name of the Deployment to scale. |
| `minReplicaCount` | Minimum replicas. Set to `0` to allow scale-to-zero. |
| `maxReplicaCount` | Upper bound on replicas. |
| `pollingInterval` | How often (seconds) KEDA polls the event source. Default: 30. |
| `cooldownPeriod` | Seconds to wait after the last event before scaling to `minReplicaCount`. Default: 300. |
| `triggers[].type` | The scaler type. See the [KEDA scaler catalogue](https://keda.sh/docs/scalers/). |
| `triggers[].metadata.listLength` | Target queue length per replica — the HPA `targetAverageValue`. |

Apply it:

```bash
kubectl apply -f redis-queue-scaler.yaml
```

Inspect the ScaledObject status:

```bash
kubectl get scaledobject redis-queue-scaler -n default
```

```
NAME                 SCALETARGETKIND   SCALETARGETNAME   MIN   MAX   TRIGGERS   READY   ACTIVE
redis-queue-scaler   apps/Deployment   queue-worker      0     20    redis      True    False
```

`ACTIVE: False` means the trigger is at zero — the Deployment has been scaled to zero replicas. As soon as messages arrive in the queue, `ACTIVE` becomes `True` and KEDA scales up.

Inspect the HPA KEDA created automatically:

```bash
kubectl get hpa -n default
```

```
NAME                          REFERENCE                 TARGETS       MINPODS   MAXPODS   REPLICAS
keda-hpa-redis-queue-scaler   Deployment/queue-worker   0/5 (avg)     1         20        0
```

## Storing credentials with TriggerAuthentication

Most production event sources require credentials. Store them in a Secret and reference them from a `TriggerAuthentication`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: redis-auth
  namespace: default
stringData:
  password: "your-redis-password"
---
apiVersion: keda.sh/v1alpha1
kind: TriggerAuthentication
metadata:
  name: redis-trigger-auth
  namespace: default
spec:
  secretTargetRef:
  - parameter: password
    name: redis-auth
    key: password
```

Reference it from the ScaledObject trigger:

```yaml
triggers:
- type: redis
  authenticationRef:
    name: redis-trigger-auth
  metadata:
    address: redis-service.default.svc.cluster.local:6379
    listName: task-queue
    listLength: "5"
```

`TriggerAuthentication` keeps credentials out of the `ScaledObject` spec and allows the same auth object to be reused across multiple ScaledObjects in the same namespace. Use `ClusterTriggerAuthentication` when the same credentials are shared across namespaces.

## Scaling behaviour tuning

KEDA exposes the HPA `behavior` block directly in the `ScaledObject` spec:

```yaml
spec:
  advanced:
    horizontalPodAutoscalerConfig:
      behavior:
        scaleDown:
          stabilizationWindowSeconds: 120
          policies:
          - type: Percent
            value: 25
            periodSeconds: 60
        scaleUp:
          stabilizationWindowSeconds: 0
          policies:
          - type: Pods
            value: 4
            periodSeconds: 15
```

This configuration scales up aggressively (4 pods every 15 seconds, no stabilisation window) but scales down conservatively (maximum 25% of replicas per minute, 2-minute stabilisation window). The asymmetry matches most queue-processing workloads where slow scale-up causes lag accumulation but fast scale-down risks dropping in-flight work.

## Removing KEDA

To uninstall KEDA without affecting running workloads, delete the ScaledObjects first (which removes the associated HPAs), then uninstall the Helm release:

```bash
kubectl delete scaledobjects --all -A
helm uninstall keda -n keda
kubectl delete namespace keda
```

Deleting ScaledObjects before uninstalling prevents orphaned HPAs from continuing to scale workloads after KEDA is gone.

## What comes next

The [second part](/docs/kubernetes/scaling-kafka-lag-keda-scaledobjects/) applies KEDA to a Kafka consumer — the most common production use case. It covers configuring consumer lag as the scaling metric, SASL and TLS authentication against a secured Kafka cluster, lag threshold tuning, and using `ScaledJob` for batch consumers that process a fixed number of messages and exit.
