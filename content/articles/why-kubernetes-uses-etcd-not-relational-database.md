---
title: "Why Kubernetes Uses etcd and Not a Relational Database"
date: 2026-08-06
author: "Victor D"
description: "Kubernetes stores all cluster state in etcd — a distributed key-value store — not PostgreSQL or MySQL. Here is why that decision makes sense and what it means for how Kubernetes actually works."
tags: ["kubernetes", "etcd", "architecture", "distributed-systems", "control-plane"]
categories: ["article"]
draft: false
toc: true
---

When people first encounter Kubernetes architecture, a common question comes up: why does the control plane store everything in **etcd** — a distributed key-value store — rather than a proper relational database like PostgreSQL or MySQL?

The answer reveals something fundamental about how Kubernetes is designed and why it behaves the way it does.

---

## What etcd actually is

etcd is a distributed, strongly consistent key-value store built on the **Raft consensus algorithm**. It was created by CoreOS specifically to solve the problem of distributed coordination — storing small amounts of critical configuration data in a way that every node in a cluster agrees on.

Every Kubernetes object you create — Pods, Deployments, ConfigMaps, Secrets, ServiceAccounts — is serialised and written to etcd as a key-value pair. The key is a path:

```
/registry/pods/production/api-server-7d9f8b6c4-xk2p1
/registry/deployments/production/api-server
/registry/secrets/production/db-credentials
```

The value is the object serialised as protobuf (or JSON for older objects).

That's it. No tables, no joins, no schemas, no migrations.

---

## What Kubernetes actually needs from a data store

To understand why etcd is the right choice, you need to understand what Kubernetes's control plane actually does with stored data.

### 1. Watches — the engine of the control plane

This is the most important reason. etcd supports a **watch API** that lets clients subscribe to changes on any key or key prefix and receive a stream of events in real time.

Every Kubernetes controller is built around this pattern:

```
watch /registry/pods → receive event → reconcile
watch /registry/deployments → receive event → reconcile
watch /registry/nodes → receive event → reconcile
```

The scheduler watches for unscheduled Pods. The Deployment controller watches for Deployments and the ReplicaSets they own. The kubelet watches for Pods assigned to its node. The entire control loop model — the reconciliation pattern Kubernetes is famous for — depends on the ability to watch for changes and react immediately.

A relational database can approximate this with polling or triggers, but it is not a first-class primitive. etcd's watch mechanism is purpose-built, efficient, and delivers ordered, versioned event streams with a **resource version** on every object so clients know exactly where they are in the history of changes.

### 2. Strong consistency over availability

Kubernetes is a **CP system** in CAP theorem terms — it prioritises consistency over availability. When you run `kubectl apply`, you need a guarantee that the write succeeded and that every subsequent read reflects that write. There is no room for eventual consistency in a system that is scheduling workloads and enforcing state.

etcd provides **linearisable reads and writes** — every read reflects the most recent write, even across a distributed cluster. Raft consensus ensures that no write is acknowledged until a majority of etcd members have committed it.

A relational database can be configured for strong consistency, but it is not the default and comes with significant operational complexity in distributed deployments.

### 3. Leader election via leases

Kubernetes runs multiple instances of the scheduler and controller-manager for high availability, but only one instance should be active at a time to avoid split-brain conflicts. This is solved using **etcd leases**.

```bash
# The active controller-manager holds a lease object
kubectl get lease -n kube-system kube-controller-manager
# NAME                      HOLDER                                    AGE
# kube-controller-manager   control-plane-node-1_abc123               12d
```

etcd leases expire automatically if not renewed. If the active instance dies, the lease expires and another instance acquires it. This is a distributed lock primitive that etcd provides natively — no application-level locking logic required.

### 4. The data model is genuinely simple

Kubernetes does not need complex queries. The control plane never asks:

- "Give me all Pods created in the last 7 days that have more than 3 restarts and are scheduled on nodes with less than 20% CPU"
- "Join Pods to their ReplicaSets to their Deployments and return the resource version of each"

Those kinds of queries happen in monitoring tools (Prometheus), not in the control plane itself.

The actual queries Kubernetes makes are:

- Get all Pods in namespace `production` → list all keys under `/registry/pods/production/`
- Get a specific Deployment → fetch `/registry/deployments/production/api-server`
- Watch for changes to any Pod → watch `/registry/pods/`

These are list and get operations on hierarchical keys. A relational schema would add complexity — schema design, migrations, index tuning — without providing any benefit for these access patterns.

---

## What would go wrong with a relational database

Imagine Kubernetes used PostgreSQL instead. You would immediately face several problems.

**Schema migrations become cluster upgrades.** Every time a new Kubernetes version adds a field to the Pod spec, you need to run a database migration. In etcd, objects are stored as serialised structs — adding a new field to the Go struct just works. There is no ALTER TABLE.

**Distributed deployment is hard.** Running a highly available PostgreSQL cluster with synchronous replication, automatic failover, and strong consistency is operationally complex. etcd is purpose-built for this — it handles its own distributed consensus through Raft and you manage it as a single logical unit.

**The watch API needs to be bolted on.** PostgreSQL LISTEN/NOTIFY exists, but it does not carry the full change history or resource versions that Kubernetes controllers depend on. You would need to build a separate event bus or use logical replication, adding significant infrastructure.

**Performance profile mismatch.** Kubernetes makes a very large number of small, frequent writes — every controller heartbeat, every status update, every lease renewal. Relational databases are optimised for complex query workloads, not high-frequency small writes with watch semantics.

---

## What etcd is not good at

etcd is not a general-purpose database. It has explicit constraints:

- **Size limit** — the default value size limit is 1.5 MiB per object and the recommended total database size is 8 GiB. This is why Kubernetes stores large data (logs, metrics, application data) elsewhere.
- **No complex queries** — if you need aggregation, filtering, or joins, you need a separate system.
- **Not designed for application data** — do not use the Kubernetes etcd cluster to store your application's state. Run a separate etcd cluster if you need one for your own use.

This is also why Kubernetes offloads metrics to Prometheus, logs to Loki or Elasticsearch, and events are stored ephemerally — etcd is only for control plane state.

---

## etcd in a managed Kubernetes cluster

If you run EKS, GKE, or AKS, you never interact with etcd directly. The cloud provider manages it for you. But on self-managed clusters, etcd health is your responsibility:

```bash
# Check etcd cluster health
etcdctl endpoint health \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  --cert=/etc/kubernetes/pki/etcd/healthcheck-client.crt \
  --key=/etc/kubernetes/pki/etcd/healthcheck-client.key

# Check etcd database size
etcdctl endpoint status \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  --cert=/etc/kubernetes/pki/etcd/healthcheck-client.crt \
  --key=/etc/kubernetes/pki/etcd/healthcheck-client.key \
  --write-out=table
```

If etcd goes down, the Kubernetes API server stops accepting writes. Running Pods continue running — the kubelet is independent — but no new Pods can be scheduled and no changes can be made until etcd recovers.

---

## The deeper lesson

The choice of etcd reflects a broader principle in distributed systems design: **use the right tool for the access pattern, not the most familiar one**.

Relational databases are excellent tools. They are the right choice for application data with complex query requirements, foreign key constraints, and transactional integrity across multiple entities.

etcd is the right choice when you need:
- Distributed consensus with strong consistency guarantees
- First-class watch semantics for reactive control loops
- Distributed locking via leases
- A simple hierarchical key space that matches your data model

Kubernetes's designers understood their access patterns precisely and chose a data store that was purpose-built for them. The result is a control plane that is fast, consistent, and architecturally coherent — even if etcd seems like an unusual choice at first glance.

---

## Further reading

- [etcd documentation — why etcd](https://etcd.io/docs/v3.5/learning/why/)
- [Kubernetes components — etcd](https://kubernetes.io/docs/concepts/overview/components/#etcd)
- [The Raft consensus algorithm](https://raft.github.io/)
- Backup and restore etcd on self-managed clusters → **Docs: etcd Backup**
