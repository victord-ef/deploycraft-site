---
title: "Scaling on Kafka Lag with KEDA ScaledObjects — Part 2"
date: 2026-08-30
description: "Replace the CPU trigger from Part 1 with a Kafka scaler. Configure ScaledObject and TriggerAuthentication to drive replica count from consumer group lag in real time."
cluster: "Message Queue — Kafka"
series: "Event-Driven Autoscaling"
part: 2
difficulty: "intermediate"
duration: "45 min"
tags: ["keda", "kafka", "autoscaling", "kubernetes", "event-driven", "strimzi", "consumer-lag"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

In [Part 1](/tutorials/kafka/deploying-keda-event-driven-autoscaling-part-1/) you installed KEDA and validated the scaling pipeline using a CPU trigger. In Part 2 you will replace that with a Kafka scaler — the most common production use case for KEDA. By the end, your consumer deployment will scale up automatically when a Kafka topic accumulates lag and scale back to zero when the topic is empty.

## Prerequisites

- Completed [Part 1](/tutorials/kafka/deploying-keda-event-driven-autoscaling-part-1/) — KEDA installed, `event-consumer` deployment running
- A running Kafka cluster reachable from the Kubernetes cluster
  - Strimzi-managed Kafka on the same cluster is the simplest setup (see [Deploying Kafka on Kubernetes with the Strimzi Operator](/tutorials/kafka/deploying-kafka-kubernetes-strimzi/))
- A Kafka topic and consumer group created
- Kafka bootstrap server address and, if auth is enabled, SASL credentials

---

## How the Kafka scaler works

The KEDA Kafka scaler connects to your Kafka cluster and queries consumer group lag. Lag is the difference between the latest offset on a partition and the committed offset for a consumer group — the number of unprocessed messages waiting.

KEDA calculates a desired replica count from the lag:

```
desiredReplicas = ceil(totalLag / lagThreshold)
```

If `lagThreshold` is 100 and total lag across all partitions is 450, KEDA targets 5 replicas (`ceil(450/100) = 5`). When lag drops to zero, KEDA scales to zero if `minReplicaCount: 0` is set.

The scaler polls the Kafka cluster directly — it does not rely on the consumer application to report lag. This means scaling happens even if the consumer is completely stopped.

---

## Step 1 — Create the Kafka topic and consumer group

If you are using Strimzi, create a topic via the `KafkaTopic` CRD:

```yaml
# keda-demo-topic.yaml
apiVersion: kafka.strimzi.io/v1beta2
kind: KafkaTopic
metadata:
  name: keda-demo
  namespace: kafka
  labels:
    strimzi.io/cluster: my-cluster
spec:
  partitions: 6
  replicas: 1
  config:
    retention.ms: 3600000
    segment.bytes: 104857600
```

```bash
kubectl apply -f keda-demo-topic.yaml
kubectl get kafkatopic keda-demo -n kafka
```

The consumer group will be created automatically when the consumer first connects. Define the group name now — you will need it in the `ScaledObject`. Use `keda-consumer-group` for this tutorial.

---

## Step 2 — Update TriggerAuthentication with Kafka credentials

Update the `Secret` and `TriggerAuthentication` created in Part 1 with your Kafka bootstrap address and, if required, SASL credentials.

### Unauthenticated Kafka (plaintext)

```yaml
# keda-kafka-secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: keda-trigger-secret
  namespace: default
type: Opaque
stringData:
  bootstrapServers: "my-cluster-kafka-bootstrap.kafka.svc.cluster.local:9092"
```

### SASL/SCRAM-SHA-512 (Strimzi default for authenticated clusters)

```yaml
# keda-kafka-secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: keda-trigger-secret
  namespace: default
type: Opaque
stringData:
  bootstrapServers: "my-cluster-kafka-bootstrap.kafka.svc.cluster.local:9093"
  saslType: "scram_sha_512"
  username: "keda-user"
  password: "your-password-here"
```

```bash
kubectl apply -f keda-kafka-secret.yaml
```

Update the `TriggerAuthentication` to expose the Kafka-specific parameters:

```yaml
# trigger-auth.yaml
apiVersion: keda.sh/v1alpha1
kind: TriggerAuthentication
metadata:
  name: keda-trigger-auth
  namespace: default
spec:
  secretTargetRef:
    - parameter: bootstrapServers
      name: keda-trigger-secret
      key: bootstrapServers
    - parameter: saslType
      name: keda-trigger-secret
      key: saslType
    - parameter: username
      name: keda-trigger-secret
      key: username
    - parameter: password
      name: keda-trigger-secret
      key: password
```

For unauthenticated Kafka, include only the `bootstrapServers` entry and omit the SASL lines.

```bash
kubectl apply -f trigger-auth.yaml
```

---

## Step 3 — Replace the ScaledObject with a Kafka trigger

Delete the CPU-based `ScaledObject` from Part 1 and create a new one using the Kafka scaler:

```bash
kubectl delete scaledobject event-consumer-scaler
```

```yaml
# scaledobject-kafka.yaml
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: event-consumer-scaler
  namespace: default
spec:
  scaleTargetRef:
    name: event-consumer
  minReplicaCount: 0
  maxReplicaCount: 10
  cooldownPeriod: 60
  pollingInterval: 10
  triggers:
    - type: kafka
      metadata:
        bootstrapServers: my-cluster-kafka-bootstrap.kafka.svc.cluster.local:9092
        consumerGroup: keda-consumer-group
        topic: keda-demo
        lagThreshold: "100"
        offsetResetPolicy: latest
      authenticationRef:
        name: keda-trigger-auth
```

```bash
kubectl apply -f scaledobject-kafka.yaml
```

Key fields:

| Field | Value | Meaning |
|---|---|---|
| `consumerGroup` | `keda-consumer-group` | The consumer group whose lag KEDA monitors |
| `topic` | `keda-demo` | The topic to monitor for lag |
| `lagThreshold` | `"100"` | Target lag per replica — 100 messages per consumer |
| `offsetResetPolicy` | `latest` | If no committed offset exists, treat lag as zero (prevents cold-start spike) |
| `minReplicaCount` | 0 | Scale to zero when lag is zero |
| `cooldownPeriod` | 60 | Wait 60 seconds after lag clears before scaling down |

---

## Step 4 — Verify the Kafka scaler is active

Check the `ScaledObject` status:

```bash
kubectl describe scaledobject event-consumer-scaler
```

Look for the `Conditions` block:

```
Conditions:
  Type            Status  Reason
  ----            ------  ------
  AbleToScale     True    SucceededGetScale
  ScalingActive   False   ScalerNotActive
  ScalingLimited  False   ReadyForScale
  Ready           True    ScaledObjectReady
```

`ScalingActive: False` with reason `ScalerNotActive` means the topic is empty and the deployment is at zero — this is the correct idle state.

Check the HPA:

```bash
kubectl get hpa
```

```
NAME                               REFERENCE                   TARGETS              MINPODS   MAXPODS   REPLICAS
keda-hpa-event-consumer-scaler    Deployment/event-consumer   kafka: 0/100          0         10        0
```

The `kafka: 0/100` target shows current lag / threshold. Replicas at 0 confirms scale-to-zero is active.

---

## Step 5 — Produce messages and observe scaling

Produce a batch of messages to the `keda-demo` topic to trigger scaling. Using the Strimzi Kafka producer utility:

```bash
kubectl run kafka-producer -ti \
  --image=quay.io/strimzi/kafka:0.41.0-kafka-3.8.0 \
  --rm=true \
  --restart=Never \
  -n kafka \
  -- bin/kafka-console-producer.sh \
     --bootstrap-server my-cluster-kafka-bootstrap.kafka.svc.cluster.local:9092 \
     --topic keda-demo
```

Type 500 messages (or use a script to produce them in bulk):

```bash
# One-liner to produce 500 messages from outside the cluster
for i in $(seq 1 500); do
  echo "message-$i"
done | kubectl exec -i -n kafka kafka-producer -- \
  bin/kafka-console-producer.sh \
  --bootstrap-server my-cluster-kafka-bootstrap.kafka.svc.cluster.local:9092 \
  --topic keda-demo
```

Within one `pollingInterval` (10 seconds), KEDA detects the lag and begins scaling:

```bash
kubectl get deployment event-consumer -w
```

Expected progression with 500 messages and `lagThreshold: 100`:

```
NAME             READY   UP-TO-DATE   AVAILABLE
event-consumer   0/0     0            0
event-consumer   0/5     0            0      # KEDA detects lag, targets 5 replicas
event-consumer   1/5     5            1
event-consumer   5/5     5            5      # All replicas ready, processing begins
event-consumer   0/0     0            0      # Lag cleared, cooldown elapsed, back to zero
```

---

## Step 6 — Multi-topic and per-partition scaling

For workloads consuming from multiple topics, add multiple triggers to a single `ScaledObject`. KEDA takes the maximum desired replica count across all triggers:

```yaml
triggers:
  - type: kafka
    metadata:
      bootstrapServers: my-cluster-kafka-bootstrap.kafka.svc.cluster.local:9092
      consumerGroup: keda-consumer-group
      topic: keda-demo
      lagThreshold: "100"
      offsetResetPolicy: latest
    authenticationRef:
      name: keda-trigger-auth
  - type: kafka
    metadata:
      bootstrapServers: my-cluster-kafka-bootstrap.kafka.svc.cluster.local:9092
      consumerGroup: keda-consumer-group
      topic: keda-demo-priority
      lagThreshold: "20"
      offsetResetPolicy: latest
    authenticationRef:
      name: keda-trigger-auth
```

The priority topic uses a lower `lagThreshold` (20 vs 100) — it scales more aggressively. If the priority topic has 100 messages, KEDA targets 5 replicas. If the standard topic simultaneously has 500 messages targeting 5 replicas, KEDA uses the maximum — 5 replicas.

### Capping replicas by partition count

A consumer cannot usefully run more replicas than there are partitions on the topic — extra replicas sit idle. Set `maxReplicaCount` equal to your partition count:

```yaml
spec:
  maxReplicaCount: 6   # matches topic partition count
```

This prevents over-scaling. KEDA still drives the HPA but the ceiling prevents wasteful replica creation.

---

## Step 7 — Monitoring KEDA Kafka lag metrics

KEDA exposes Prometheus metrics on port 8080 of the metrics API server pod. Key metrics for Kafka scaling:

```bash
# Port-forward to the KEDA metrics server
kubectl port-forward -n keda \
  svc/keda-operator-metrics-apiserver 8080:8080
```

```bash
# Query lag metric
curl -s http://localhost:8080/metrics | grep keda_scaler_metrics_value
```

The `keda_scaler_metrics_value` gauge reports the current raw metric value (total lag) per `ScaledObject`. Feed this into your Prometheus stack and alert when lag consistently exceeds your `lagThreshold * maxReplicaCount` — this signals that the topic is producing faster than the maximum consumer capacity can drain.

Useful Grafana panel queries:

```promql
# Current consumer lag per ScaledObject
keda_scaler_metrics_value{scaledobject="event-consumer-scaler"}

# Replica count over time
kube_deployment_status_replicas{deployment="event-consumer"}

# Time-to-zero after lag clears (measure cooldown effectiveness)
kube_deployment_status_replicas{deployment="event-consumer"} > 0
```

---

## Step 8 — Production hardening

Three adjustments before running this in production:

**1. Use ClusterTriggerAuthentication for shared Kafka credentials**

If multiple workloads in different namespaces connect to the same Kafka cluster, use `ClusterTriggerAuthentication` to avoid duplicating secrets:

```yaml
apiVersion: keda.sh/v1alpha1
kind: ClusterTriggerAuthentication
metadata:
  name: kafka-cluster-auth
spec:
  secretTargetRef:
    - parameter: bootstrapServers
      name: keda-kafka-secret
      namespace: keda
      key: bootstrapServers
```

Reference it in any namespace with `authenticationRef.kind: ClusterTriggerAuthentication`.

**2. Set activationLagThreshold to prevent flapping**

`activationLagThreshold` sets a minimum lag before KEDA activates scaling from zero. This prevents scale-up on a single stray message:

```yaml
metadata:
  lagThreshold: "100"
  activationLagThreshold: "10"
```

With this setting: KEDA only scales from 0→1 when lag exceeds 10. Once active, it scales up/down based on `lagThreshold: 100`.

**3. Tune cooldownPeriod to match your drain time**

`cooldownPeriod` should be at least as long as the time it takes the consumer to drain the remaining messages after lag hits zero. Too short and replicas scale down before processing completes, causing the next poll to detect residual lag and scale back up — oscillation.

A safe starting point: `cooldownPeriod = average message processing time × lagThreshold`.

---

## What you have built

- A Kafka-triggered `ScaledObject` that drives replicas from consumer group lag
- `TriggerAuthentication` managing Kafka credentials separately from the scaling policy
- Scale-to-zero with `activationLagThreshold` to prevent single-message flapping
- Multi-topic trigger composition with per-topic thresholds
- Prometheus metrics for lag-based alerting

The event-consumer deployment now autoscales in direct proportion to unprocessed work — zero cost at idle, full capacity under load, with no manual intervention required.
