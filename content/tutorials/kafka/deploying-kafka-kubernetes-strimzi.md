---
title: "Deploying Kafka on Kubernetes with the Strimzi Operator"
date: 2026-07-25
description: "Run a production-ready Kafka cluster on Kubernetes using Strimzi custom resources and persistent storage."
cluster: "Message Queue — Kafka"
series: "Kafka Fundamentals"
part: 2
difficulty: "intermediate"
duration: "45 min"
tags: ["kafka", "kubernetes", "strimzi", "operator"]
categories: ["tutorial"]
draft: false
toc: true
---

## Prerequisites

- A running Kubernetes cluster (1.25+)
- `kubectl` configured and `helm` installed
- Familiarity with Kafka concepts — see [Part 1](/tutorials/kafka-concepts-topics-partitions/) if you haven't already

## What is Strimzi?

Strimzi is a CNCF project that makes running Kafka on Kubernetes production-ready. It provides a set of operators and custom resources — `Kafka`, `KafkaTopic`, `KafkaUser` — that manage the full lifecycle of a Kafka cluster declaratively.

## Install the Strimzi Operator

```bash
helm repo add strimzi https://strimzi.io/charts/
helm repo update

helm install strimzi-operator strimzi/strimzi-kafka-operator \
  --namespace kafka \
  --create-namespace \
  --version 0.40.0
```

Verify the operator is running:

```bash
kubectl get pods -n kafka
# NAME                                        READY   STATUS    RESTARTS
# strimzi-cluster-operator-6f5d4b5b8c-x9k2p  1/1     Running   0
```

## Define the Kafka Cluster

Create a `Kafka` custom resource. This tells Strimzi to provision brokers, ZooKeeper (or KRaft), and configure storage.

```yaml
# kafka-cluster.yaml
apiVersion: kafka.strimzi.io/v1beta2
kind: Kafka
metadata:
  name: my-cluster
  namespace: kafka
spec:
  kafka:
    version: 3.7.0
    replicas: 3
    listeners:
      - name: plain
        port: 9092
        type: internal
        tls: false
      - name: tls
        port: 9093
        type: internal
        tls: true
    config:
      offsets.topic.replication.factor: 3
      transaction.state.log.replication.factor: 3
      transaction.state.log.min.isr: 2
      default.replication.factor: 3
      min.insync.replicas: 2
    storage:
      type: persistent-claim
      size: 20Gi
      deleteClaim: false
  zookeeper:
    replicas: 3
    storage:
      type: persistent-claim
      size: 5Gi
      deleteClaim: false
  entityOperator:
    topicOperator: {}
    userOperator: {}
```

Apply it:

```bash
kubectl apply -f kafka-cluster.yaml
```

Watch the cluster come up — it takes 2–3 minutes for all pods to reach `Running`:

```bash
kubectl get pods -n kafka -w
```

## Create a Topic

With the cluster running, create a `KafkaTopic` resource:

```yaml
# order-events-topic.yaml
apiVersion: kafka.strimzi.io/v1beta2
kind: KafkaTopic
metadata:
  name: order-events
  namespace: kafka
  labels:
    strimzi.io/cluster: my-cluster
spec:
  partitions: 3
  replicas: 3
  config:
    retention.ms: 604800000   # 7 days
    segment.bytes: 1073741824 # 1 GiB
```

```bash
kubectl apply -f order-events-topic.yaml
```

## Verify the Cluster

Run a producer and consumer inside the cluster to confirm end-to-end messaging works:

```bash
# Producer
kubectl -n kafka run kafka-producer -ti \
  --image=quay.io/strimzi/kafka:0.40.0-kafka-3.7.0 \
  --rm=true --restart=Never -- \
  bin/kafka-console-producer.sh \
  --bootstrap-server my-cluster-kafka-bootstrap:9092 \
  --topic order-events

# Consumer (separate terminal)
kubectl -n kafka run kafka-consumer -ti \
  --image=quay.io/strimzi/kafka:0.40.0-kafka-3.7.0 \
  --rm=true --restart=Never -- \
  bin/kafka-console-consumer.sh \
  --bootstrap-server my-cluster-kafka-bootstrap:9092 \
  --topic order-events \
  --from-beginning
```

## What's Next

With a working Kafka cluster on Kubernetes, the natural next steps are:

- **Event-driven autoscaling** — scale consumers automatically on Kafka lag using KEDA (covered in the next series)
- **Security** — add mTLS between producers, consumers, and brokers
- **Observability** — expose broker and consumer lag metrics to Prometheus
