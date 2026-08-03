---
title: "Kafka Concepts — Topics, Partitions, Producers, Consumers, and Consumer Groups"
date: 2026-07-25
description: "Understand the core Kafka primitives before deploying anything on Kubernetes."
cluster: "Message Queue — Kafka"
series: "Kafka Fundamentals"
part: 1
difficulty: "intermediate"
duration: "30 min"
tags: ["kafka", "kubernetes", "message-queue"]
categories: ["tutorial"]
draft: false
toc: true
---

## What is Kafka?

Apache Kafka is a distributed event streaming platform built around an append-only log. Unlike a traditional message queue where messages are consumed and deleted, Kafka retains records for a configurable period — every consumer reads the log independently at its own pace.

## Topics

A **topic** is a named, ordered, append-only log. Think of it as a category or feed to which records are published. Topics are the primary organisational unit in Kafka.

```
Topic: "order-events"
─────────────────────────────────────────────────────
Partition 0: [msg-0] [msg-1] [msg-4] [msg-7] ...
Partition 1: [msg-2] [msg-5] [msg-8] ...
Partition 2: [msg-3] [msg-6] [msg-9] ...
```

Topics are created with a configurable number of **partitions** and a **replication factor**.

```bash
kafka-topics.sh --create \
  --topic order-events \
  --partitions 3 \
  --replication-factor 2 \
  --bootstrap-server kafka:9092
```

## Partitions

Each topic is split into one or more partitions. Partitions are the unit of parallelism and fault tolerance in Kafka:

- **Ordering** is guaranteed within a partition, not across partitions.
- **Distribution** — partitions are spread across brokers so no single broker holds all the data.
- **Replication** — each partition has one leader and N-1 followers. Consumers and producers always talk to the leader.

The number of partitions determines the maximum parallelism for consumers in a group — you cannot have more active consumers than partitions.

## Producers

A **producer** publishes records to a topic. By default, Kafka assigns records to partitions using a round-robin strategy. To control placement, producers can specify a **partition key** — records with the same key always land in the same partition, preserving order for that key.

```python
producer.send(
    topic="order-events",
    key=b"customer-42",    # same key → same partition
    value=b'{"order_id": 9001, "status": "placed"}'
)
```

## Consumers

A **consumer** reads records from one or more partitions. Each record has an **offset** — a sequential integer identifying its position in the partition. Consumers track their own offset, giving you:

- **Replay** — rewind the offset to reprocess records.
- **Exactly-once** or **at-least-once** semantics depending on when you commit.

## Consumer Groups

A **consumer group** is a set of consumers that cooperate to consume a topic. Kafka assigns each partition to exactly one consumer in the group at a time:

```
Topic: "order-events" (3 partitions)

Consumer Group: "order-processor"
├── consumer-1  →  Partition 0
├── consumer-2  →  Partition 1
└── consumer-3  →  Partition 2
```

If a consumer dies, Kafka **rebalances** — reassigning its partitions to surviving members. Adding consumers beyond the number of partitions leaves some idle.

Different consumer groups each maintain their own offsets and read the topic independently. This is how Kafka fans out the same stream to multiple downstream services without interference.

## What's Next

In Part 2 we put this into practice — deploying a production-ready Kafka cluster on Kubernetes using the Strimzi Operator, with persistent storage and inter-broker TLS.
