---
title: "Deploying KEDA for Event-Driven Autoscaling — Part 1"
date: 2026-08-30
description: "Install KEDA on a Kubernetes cluster and scale a workload based on external event sources using ScaledObject and TriggerAuthentication."
cluster: "Message Queue — Kafka"
series: "Event-Driven Autoscaling"
part: 1
difficulty: "intermediate"
duration: "45 min"
tags: ["keda", "autoscaling", "kubernetes", "event-driven", "kafka"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

By the end of this tutorial you will have KEDA installed on a Kubernetes cluster, a sample workload configured with a `ScaledObject`, and a working `TriggerAuthentication` that KEDA uses to authenticate against an external event source. Part 2 builds directly on this by wiring KEDA to a live Kafka topic and scaling on consumer lag.

## Prerequisites

- A running Kubernetes cluster (v1.27+)
- `kubectl` configured and pointing at the target cluster
- `helm` v3 installed
- Cluster admin permissions

---

## What is KEDA

The Kubernetes Event-Driven Autoscaler (KEDA) extends the native HorizontalPodAutoscaler (HPA) to scale workloads based on external event sources — Kafka consumer lag, queue depth, Redis list length, HTTP request rate, and over 60 other scalers. It does this without replacing the HPA; instead, KEDA acts as an external metrics provider that feeds custom metrics into the HPA pipeline.

The core objects KEDA introduces:

| Object | Purpose |
|---|---|
| `ScaledObject` | Binds a workload (Deployment, StatefulSet) to a scaler and defines min/max replicas and scaling thresholds |
| `ScaledJob` | Like `ScaledObject` but for batch Jobs — one Job per event |
| `TriggerAuthentication` | Stores credentials for a scaler (API keys, connection strings, certificates) |
| `ClusterTriggerAuthentication` | Cluster-scoped variant of `TriggerAuthentication` |

KEDA handles scale-to-zero natively. When no events are present and `minReplicaCount: 0`, KEDA scales the deployment to zero and holds it there until events appear. The HPA cannot do this — it has a minimum of one replica.

---

## Step 1 — Install KEDA with Helm

Add the KEDA Helm repository and install into its own namespace:

```bash
helm repo add kedacore https://kedacore.github.io/charts
helm repo update

helm install keda kedacore/keda \
  --namespace keda \
  --create-namespace \
  --version 2.15.0
```

Verify the installation:

```bash
kubectl get pods -n keda
```

Expected output:

```
NAME                                      READY   STATUS    RESTARTS   AGE
keda-operator-xxxxxxxxx-xxxxx             1/1     Running   0          60s
keda-operator-metrics-apiserver-xxx-xxx   1/1     Running   0          60s
keda-admission-webhooks-xxx-xxxxx         1/1     Running   0          60s
```

Three components are running:

- **keda-operator** — watches `ScaledObject` resources and drives reconciliation
- **keda-operator-metrics-apiserver** — exposes custom metrics to the Kubernetes metrics pipeline and HPA
- **keda-admission-webhooks** — validates `ScaledObject` and `TriggerAuthentication` resources at admission

Confirm the custom resource definitions are installed:

```bash
kubectl get crd | grep keda
```

You should see `scaledobjects.keda.sh`, `scaledjobs.keda.sh`, `triggerauthentications.keda.sh`, and `clustertriggerauthentications.keda.sh`.

---

## Step 2 — Deploy a sample workload

Deploy a simple consumer workload to use as the scaling target. This represents any application that processes events — a Kafka consumer, a queue worker, or an API handler.

```yaml
# consumer-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: event-consumer
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: event-consumer
  template:
    metadata:
      labels:
        app: event-consumer
    spec:
      containers:
        - name: consumer
          image: busybox:1.36
          command: ["sh", "-c", "while true; do echo processing; sleep 5; done"]
          resources:
            requests:
              cpu: "50m"
              memory: "64Mi"
            limits:
              cpu: "100m"
              memory: "128Mi"
```

```bash
kubectl apply -f consumer-deployment.yaml
kubectl get deployment event-consumer
```

---

## Step 3 — Create a TriggerAuthentication

`TriggerAuthentication` decouples credentials from the `ScaledObject`. This is the correct pattern — credentials live in a Kubernetes Secret, and the `TriggerAuthentication` references them. The `ScaledObject` references the `TriggerAuthentication` by name.

For this tutorial, create a placeholder Secret and a corresponding `TriggerAuthentication`. In Part 2 this will hold your Kafka bootstrap credentials.

```yaml
# trigger-auth-secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: keda-trigger-secret
  namespace: default
type: Opaque
stringData:
  connection: "placeholder-connection-string"
```

```yaml
# trigger-auth.yaml
apiVersion: keda.sh/v1alpha1
kind: TriggerAuthentication
metadata:
  name: keda-trigger-auth
  namespace: default
spec:
  secretTargetRef:
    - parameter: connection
      name: keda-trigger-secret
      key: connection
```

```bash
kubectl apply -f trigger-auth-secret.yaml
kubectl apply -f trigger-auth.yaml
kubectl get triggerauthentication keda-trigger-auth
```

---

## Step 4 — Create a ScaledObject with the CPU scaler

To validate the full KEDA pipeline without needing a Kafka cluster, use the `cpu` scaler — it scales based on CPU utilisation, which you can trigger manually. This confirms KEDA is correctly managing the HPA before you wire it to an external event source.

```yaml
# scaledobject-cpu.yaml
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: event-consumer-scaler
  namespace: default
spec:
  scaleTargetRef:
    name: event-consumer
  minReplicaCount: 1
  maxReplicaCount: 10
  cooldownPeriod: 30
  pollingInterval: 15
  triggers:
    - type: cpu
      metricType: Utilization
      metadata:
        value: "50"
```

```bash
kubectl apply -f scaledobject-cpu.yaml
```

Key fields to understand:

| Field | Value | Meaning |
|---|---|---|
| `minReplicaCount` | 1 | Never scale below 1 replica (use 0 for scale-to-zero) |
| `maxReplicaCount` | 10 | Hard ceiling on replicas |
| `cooldownPeriod` | 30 | Seconds to wait after last scale event before scaling down |
| `pollingInterval` | 15 | Seconds between scaler checks |
| `value` | "50" | Target CPU utilisation percentage |

---

## Step 5 — Verify KEDA is managing the HPA

KEDA creates and manages an HPA on your behalf. Inspect it:

```bash
kubectl get hpa
```

Expected output:

```
NAME                          REFERENCE                   TARGETS        MINPODS   MAXPODS   REPLICAS
keda-hpa-event-consumer-scaler   Deployment/event-consumer   cpu: 2%/50%    1         10        1
```

KEDA owns this HPA — do not edit it directly. All configuration changes go through the `ScaledObject`.

Check the `ScaledObject` status:

```bash
kubectl get scaledobject event-consumer-scaler -o yaml | grep -A 10 "status:"
```

A healthy `ScaledObject` will show `Active: true` and `Ready: true` conditions.

---

## Step 6 — Test scaling behaviour

Trigger CPU load on the consumer pod to observe KEDA scaling up:

```bash
# Get the pod name
POD=$(kubectl get pod -l app=event-consumer -o jsonpath='{.items[0].metadata.name}')

# Run a CPU stress loop inside the pod
kubectl exec -it $POD -- sh -c "while true; do :; done" &

# Watch replicas scale up
kubectl get deployment event-consumer -w
```

After the cooldown period (30 seconds in this config), kill the stress loop and watch replicas scale back down:

```bash
# Kill the background stress job
kill %1

# Watch scale-down
kubectl get deployment event-consumer -w
```

---

## Step 7 — Enable scale-to-zero

Change `minReplicaCount` to 0 to enable scale-to-zero. With the CPU scaler, the deployment will scale to zero when there is no CPU pressure. This is most useful with queue-based scalers in production.

```bash
kubectl patch scaledobject event-consumer-scaler \
  --type merge \
  -p '{"spec":{"minReplicaCount":0}}'
```

Wait for the cooldown period. The deployment will scale to zero:

```bash
kubectl get deployment event-consumer -w
```

When a new event arrives (in Part 2, a Kafka message), KEDA detects it via the scaler, scales the deployment from zero to one, and the consumer begins processing.

---

## Step 8 — Inspect KEDA logs

When troubleshooting scaling behaviour, the KEDA operator logs are the primary diagnostic:

```bash
kubectl logs -n keda -l app=keda-operator --tail=50 -f
```

Look for lines referencing your `ScaledObject` name. Common messages:

| Log message | Meaning |
|---|---|
| `Successfully set ScaledObject conditions` | ScaledObject reconciled cleanly |
| `Scaler is inactive` | No events detected, will scale to zero if minReplicaCount allows |
| `Successfully updated HPA` | KEDA adjusted the HPA target replica count |
| `Error getting scaler metrics` | Scaler cannot reach the event source — check TriggerAuthentication |

---

## What you have built

- KEDA installed via Helm with all three components healthy
- A `TriggerAuthentication` pattern established for secure credential management
- A `ScaledObject` managing an HPA for an event-consumer workload
- Scale-up, scale-down, and scale-to-zero verified

## Next steps

In [Part 2](/tutorials/kafka/scaling-kafka-lag-keda-scaledobjects-part-2/) you will replace the CPU trigger with a Kafka scaler, connect `TriggerAuthentication` to real Kafka credentials, and configure scaling thresholds based on consumer group lag — the standard production pattern for event-driven Kubernetes workloads.
