---
title: "Scaling on Kafka Lag with KEDA ScaledObjects"
description: "Configure KEDA to scale a Kafka consumer Deployment on consumer group lag, set up SASL/TLS authentication, tune lag thresholds for production, and use ScaledJob for batch consumers."
weight: 20
toc: true
draft: true
---

The
[first part](/docs/kubernetes/deploying-keda-event-driven-autoscaling/)
covered installing KEDA and creating a basic ScaledObject. This post applies
KEDA to Kafka consumer lag — the most common production event-driven scaling
use case.

Consumer lag is the number of messages in a Kafka topic partition that a
consumer group has not yet processed. When lag grows, more consumer replicas
reduce it. When lag reaches zero, replicas can scale back down. KEDA makes
this loop automatic.

## Prerequisites

- KEDA 2.15+ installed on the cluster (see Part 1).
- A Kafka cluster reachable from the KEDA Metrics Server pod.
- A consumer group actively reading from a topic.
- Kafka broker address and, for secured clusters, SASL credentials or TLS certificates.

## How the Kafka scaler works

KEDA's Kafka scaler connects to the Kafka cluster as a metadata client — it
does not join the consumer group or consume any messages. On each polling
interval it:

1. Fetches the latest offset for each partition of the target topic.
2. Fetches the committed offset for the consumer group on each partition.
3. Computes lag per partition: `latest offset − committed offset`.
4. Sums lag across all partitions assigned to the consumer group.
5. Returns the total lag to the HPA as an external metric.

The HPA divides total lag by `lagThreshold` to determine the desired replica
count. If total lag is 500 and `lagThreshold` is 100, the HPA targets 5
replicas.

## Basic ScaledObject for a Kafka consumer

```yaml
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: kafka-consumer-scaler
  namespace: default
spec:
  scaleTargetRef:
    name: kafka-consumer
  minReplicaCount: 1
  maxReplicaCount: 30
  pollingInterval: 15
  cooldownPeriod: 120
  triggers:
  - type: kafka
    metadata:
      bootstrapServers: kafka-broker-0.kafka.svc.cluster.local:9092
      consumerGroup: my-consumer-group
      topic: my-topic
      lagThreshold: "100"
      offsetResetPolicy: latest
```

**Key fields:**

| Field | Description |
|-------|-------------|
| `bootstrapServers` | Comma-separated list of Kafka broker addresses. |
| `consumerGroup` | The consumer group whose lag is measured. Must match the group ID in your consumer application. |
| `topic` | Topic to monitor. Omit to monitor all topics the consumer group is subscribed to. |
| `lagThreshold` | Target lag per replica. Lower values scale up sooner and more aggressively. |
| `offsetResetPolicy` | `latest` (ignore existing lag on startup) or `earliest` (include existing lag). Use `latest` for streaming, `earliest` for batch reprocessing. |

Apply and verify:

```bash
kubectl apply -f kafka-consumer-scaler.yaml
kubectl get scaledobject kafka-consumer-scaler -n default -w
```

```
NAME                    SCALETARGETKIND   SCALETARGETNAME   MIN   MAX   TRIGGERS   READY   ACTIVE
kafka-consumer-scaler   apps/Deployment   kafka-consumer    1     30    kafka      True    True
```

Check the live lag metric:

```bash
kubectl get --raw "/apis/external.metrics.k8s.io/v1beta1/namespaces/default/s0-kafka-my-topic" | jq .
```

```json
{
  "items": [
    {
      "metricName": "s0-kafka-my-topic",
      "value": "342"
    }
  ]
}
```

## Authenticating against a secured Kafka cluster

Production Kafka clusters require SASL authentication, TLS, or both. Store
credentials in a Secret and reference them via `TriggerAuthentication`.

### SASL/PLAIN authentication

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: kafka-sasl-auth
  namespace: default
stringData:
  sasl: "plaintext"
  username: "my-service-account"
  password: "my-sasl-password"
---
apiVersion: keda.sh/v1alpha1
kind: TriggerAuthentication
metadata:
  name: kafka-trigger-auth
  namespace: default
spec:
  secretTargetRef:
  - parameter: sasl
    name: kafka-sasl-auth
    key: sasl
  - parameter: username
    name: kafka-sasl-auth
    key: username
  - parameter: password
    name: kafka-sasl-auth
    key: password
```

Reference it in the ScaledObject trigger:

```yaml
triggers:
- type: kafka
  authenticationRef:
    name: kafka-trigger-auth
  metadata:
    bootstrapServers: kafka-broker-0.kafka.svc.cluster.local:9092
    consumerGroup: my-consumer-group
    topic: my-topic
    lagThreshold: "100"
    sasl: plaintext
    tls: disable
```

### SASL/SCRAM-SHA-512 with TLS

For clusters using SCRAM authentication over TLS, store the CA certificate
and credentials separately:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: kafka-scram-tls-auth
  namespace: default
stringData:
  sasl: "scram_sha512"
  username: "my-service-account"
  password: "my-scram-password"
  ca: |
    -----BEGIN CERTIFICATE-----
    <base64-encoded CA certificate>
    -----END CERTIFICATE-----
---
apiVersion: keda.sh/v1alpha1
kind: TriggerAuthentication
metadata:
  name: kafka-scram-trigger-auth
  namespace: default
spec:
  secretTargetRef:
  - parameter: sasl
    name: kafka-scram-tls-auth
    key: sasl
  - parameter: username
    name: kafka-scram-tls-auth
    key: username
  - parameter: password
    name: kafka-scram-tls-auth
    key: password
  - parameter: ca
    name: kafka-scram-tls-auth
    key: ca
```

Update the trigger metadata:

```yaml
triggers:
- type: kafka
  authenticationRef:
    name: kafka-scram-trigger-auth
  metadata:
    bootstrapServers: kafka-broker-0.kafka.svc.cluster.local:9093
    consumerGroup: my-consumer-group
    topic: my-topic
    lagThreshold: "100"
    sasl: scram_sha512
    tls: enable
```

## Tuning lag thresholds for production

`lagThreshold` is the single most important tuning parameter. Setting it too
high leaves lag to grow unchecked; too low causes excessive scaling churn.

A starting point for threshold calculation:

```
lagThreshold = (target_processing_time_seconds × messages_per_second_per_replica)
```

For a consumer that processes 200 messages per second per replica and a
target of processing accumulated lag within 30 seconds:

```
lagThreshold = 30 × 200 = 6000
```

This means: for every 6000 messages of lag, add one replica.

Combine with conservative scale-down behaviour to avoid oscillation:

```yaml
spec:
  cooldownPeriod: 300
  advanced:
    horizontalPodAutoscalerConfig:
      behavior:
        scaleDown:
          stabilizationWindowSeconds: 180
          policies:
          - type: Percent
            value: 20
            periodSeconds: 60
        scaleUp:
          stabilizationWindowSeconds: 0
          policies:
          - type: Pods
            value: 5
            periodSeconds: 30
```

Scale up immediately and in large steps; scale down slowly and in small steps.
The 5-minute `cooldownPeriod` prevents scale-to-zero until lag has been at
zero for 5 minutes — useful for topics with bursty but regular traffic.

## Multi-topic scaling

A single ScaledObject can monitor multiple topics or consumer groups by
adding additional triggers. KEDA treats multiple triggers with an `OR`
relationship — the highest metric value across all triggers drives the
replica count:

```yaml
triggers:
- type: kafka
  authenticationRef:
    name: kafka-trigger-auth
  metadata:
    bootstrapServers: kafka-broker-0.kafka.svc.cluster.local:9092
    consumerGroup: my-consumer-group
    topic: orders
    lagThreshold: "500"
- type: kafka
  authenticationRef:
    name: kafka-trigger-auth
  metadata:
    bootstrapServers: kafka-broker-0.kafka.svc.cluster.local:9092
    consumerGroup: my-consumer-group
    topic: payments
    lagThreshold: "200"
```

The replica count will be the maximum of what each trigger independently
demands.

## ScaledJob for batch consumers

A long-running `Deployment` consumer is right for streaming workloads. For
batch consumers — processes that read a fixed number of messages and exit —
use `ScaledJob` instead. KEDA creates one Kubernetes `Job` per trigger
activation and cleans up completed Jobs automatically.

```yaml
apiVersion: keda.sh/v1alpha1
kind: ScaledJob
metadata:
  name: kafka-batch-job
  namespace: default
spec:
  jobTargetRef:
    template:
      spec:
        containers:
        - name: batch-consumer
          image: registry.example.com/batch-consumer:v1.0.0
          env:
          - name: KAFKA_BOOTSTRAP
            value: kafka-broker-0.kafka.svc.cluster.local:9092
          - name: KAFKA_GROUP
            value: batch-consumer-group
          - name: KAFKA_TOPIC
            value: batch-jobs
          - name: BATCH_SIZE
            value: "100"
        restartPolicy: Never
  minReplicaCount: 0
  maxReplicaCount: 10
  pollingInterval: 30
  triggers:
  - type: kafka
    authenticationRef:
      name: kafka-trigger-auth
    metadata:
      bootstrapServers: kafka-broker-0.kafka.svc.cluster.local:9092
      consumerGroup: batch-consumer-group
      topic: batch-jobs
      lagThreshold: "100"
```

Each Job processes up to `BATCH_SIZE` messages and exits. KEDA creates a
new Job whenever lag exceeds the threshold, up to `maxReplicaCount`
concurrent Jobs.

Control completed Job retention with:

```yaml
spec:
  successfulJobsHistoryLimit: 5
  failedJobsHistoryLimit: 3
```

## Testing the scaler

Produce a burst of messages to trigger scaling:

```bash
kubectl run kafka-producer --restart=Never --image=confluentinc/cp-kafka:7.6.0 -- \
  bash -c "seq 1 10000 | kafka-console-producer \
    --broker-list kafka-broker-0.kafka.svc.cluster.local:9092 \
    --topic my-topic"
```

Watch replicas scale up:

```bash
kubectl get deployment kafka-consumer -w
```

```
NAME             READY   UP-TO-DATE   AVAILABLE
kafka-consumer   1/1     1            1
kafka-consumer   3/3     3            3
kafka-consumer   8/8     8            8
kafka-consumer   10/10   10           10
```

Once the topic is drained, watch the cooldown and scale-down:

```bash
kubectl get scaledobject kafka-consumer-scaler -w
```

```
NAME                    ACTIVE   READY   REPLICAS
kafka-consumer-scaler   True     True    10
kafka-consumer-scaler   True     True    8
kafka-consumer-scaler   True     True    4
kafka-consumer-scaler   False    True    1
```

`ACTIVE: False` indicates lag is zero and the cooldown period has elapsed.

## Observability

KEDA exposes Prometheus metrics from the operator and metrics server pods.
Scrape them with:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: keda-operator
  namespace: keda
spec:
  selector:
    matchLabels:
      app: keda-operator
  endpoints:
  - port: metrics
    interval: 30s
```

Key metrics to alert on:

| Metric | Alert condition |
|--------|----------------|
| `keda_scaler_errors_total` | > 0 for more than 2 minutes — scaler is failing to poll the event source. |
| `keda_scaled_object_paused` | == 1 — ScaledObject has been manually paused. |
| `keda_resource_totals` | Track total ScaledObjects/ScaledJobs for capacity planning. |

For Kafka-specific lag visibility, the consumer group lag metric exposed
through `external.metrics.k8s.io` can be queried directly from Prometheus
if you have the Prometheus adapter configured alongside KEDA.
