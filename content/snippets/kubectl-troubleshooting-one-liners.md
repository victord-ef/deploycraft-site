---
title: "kubectl One-liners for Cluster Troubleshooting"
date: 2026-08-03
description: "Copy-paste kubectl commands for the most common debugging scenarios — crashing pods, pending scheduling, resource pressure, and network connectivity."
lang: "Bash"
tags: ["kubectl", "kubernetes", "debugging", "logs", "exec", "troubleshooting"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-08-03"
draft: false
---

## When to use this

When something is broken in a cluster and you need to move fast. These commands cover the first 10 minutes of any Kubernetes incident — from identifying the problem pod to inspecting its environment, logs, and network reachability.

## Pods

```bash
# All pods across all namespaces with node assignment and status
kubectl get pods -A -o wide

# Watch pod status in real time
kubectl get pods -n <namespace> -w

# Pods that are not Running or Completed (find broken ones fast)
kubectl get pods -A --field-selector=status.phase!=Running,status.phase!=Succeeded

# Full event log for a crashing pod — shows OOMKill, probe failures, image pull errors
kubectl describe pod <pod> -n <namespace>

# Previous container logs (the run before the current crash)
kubectl logs <pod> -n <namespace> --previous

# Stream logs from all pods matching a label
kubectl logs -l app=my-app -n <namespace> --follow --max-log-requests=10

# Tail the last 100 lines from a specific container in a multi-container pod
kubectl logs <pod> -n <namespace> -c <container> --tail=100
```

## Events

```bash
# Cluster-wide events sorted by time — fastest way to see what just happened
kubectl get events -A --sort-by='.lastTimestamp'

# Events for a specific namespace only
kubectl get events -n <namespace> --sort-by='.lastTimestamp'

# Watch events as they arrive
kubectl get events -n <namespace> -w
```

## Exec and Debug

```bash
# Open a shell in a running pod
kubectl exec -it <pod> -n <namespace> -- /bin/sh

# Run a one-off command without an interactive session
kubectl exec <pod> -n <namespace> -- env | grep AWS

# Attach an ephemeral debug container to a running pod (no restart required)
# Useful when the main container has no shell (e.g. distroless)
kubectl debug -it <pod> -n <namespace> \
  --image=busybox:latest \
  --target=<container> \
  -- /bin/sh

# Spawn a standalone debug pod in the cluster network
kubectl run debug --rm -it \
  --image=nicolaka/netshoot \
  --restart=Never \
  -n <namespace> \
  -- /bin/bash
```

## Resource Pressure

```bash
# Node resource usage (requires metrics-server)
kubectl top nodes

# Pod CPU and memory usage across a namespace
kubectl top pods -n <namespace> --sort-by=memory

# Describe a node to see capacity, allocatable, and all pod resource requests
kubectl describe node <node-name>

# Find pods with no resource requests set (scheduling risk)
kubectl get pods -A -o json | \
  jq -r '.items[] | select(.spec.containers[].resources.requests == null) |
  "\(.metadata.namespace)/\(.metadata.name)"'

# Check ResourceQuota consumption in a namespace
kubectl describe resourcequota -n <namespace>
```

## Scheduling and Pending Pods

```bash
# Why is a pod Pending? — look at Events section in the output
kubectl describe pod <pod> -n <namespace>

# List nodes with their labels (for nodeSelector / affinity debugging)
kubectl get nodes --show-labels

# Check taints on all nodes (a missing toleration is a common Pending cause)
kubectl get nodes -o custom-columns=\
NAME:.metadata.name,TAINTS:.spec.taints

# Simulate scheduling — does a pod fit on a node?
kubectl get pods <pod> -n <namespace> -o yaml | \
  kubectl apply --dry-run=server -f -
```

## Network and DNS

```bash
# Test DNS resolution from inside the cluster
kubectl run dns-test --rm -it \
  --image=busybox:latest \
  --restart=Never \
  -- nslookup kubernetes.default

# Test HTTP connectivity to a service from inside the cluster
kubectl run curl-test --rm -it \
  --image=curlimages/curl:latest \
  --restart=Never \
  -- curl -v http://<service>.<namespace>.svc.cluster.local:<port>/healthz

# List all services and their endpoints (check for empty endpoint slices)
kubectl get endpoints -n <namespace>

# Inspect a specific service's endpoint backing pods
kubectl describe endpoints <service> -n <namespace>
```

## ConfigMaps and Secrets

```bash
# View a ConfigMap's contents
kubectl get configmap <name> -n <namespace> -o yaml

# Decode all values in a Secret (base64)
kubectl get secret <name> -n <namespace> -o json | \
  jq -r '.data | to_entries[] | "\(.key): \(.value | @base64d)"'

# Check which environment variables a running pod sees
kubectl exec <pod> -n <namespace> -- env | sort
```

## Quick Status Summary

```bash
# Full cluster health snapshot — nodes, pods, services
kubectl get nodes,pods,svc -A -o wide

# All non-running pods with their restart counts
kubectl get pods -A --sort-by='.status.containerStatuses[0].restartCount' | \
  grep -v "Running\|Completed"

# Rollout status for all deployments in a namespace
kubectl rollout status deployment -n <namespace>
```

**Key decisions:**

| Command | Why this version |
|---|---|
| `--previous` on logs | The current container's logs are empty after a crash. `--previous` shows the logs from the container run that actually failed. |
| `kubectl debug` with `--target` | Attaches to the same PID/network namespace as the target container — you see exactly what that container sees, not a fresh isolated env. |
| `nicolaka/netshoot` image | Ships `curl`, `dig`, `nmap`, `tcpdump`, `iperf`, `ss`, and more. Purpose-built for Kubernetes network debugging. |
| `jq` for no-requests pods | `kubectl` has no built-in filter for missing resource fields — `jq` is the practical way to find these across a large cluster. |
| `@base64d` in jq | Decodes Secret values inline — faster than piping through `base64 -d` per field. |

> **Note:** `kubectl top` requires `metrics-server` to be installed in the cluster. On clusters without it, use `kubectl exec` into a pod and run `top` or `cat /proc/meminfo` directly.

## Full walkthrough

Systematic incident response for Kubernetes — from alert to root cause in under 30 minutes → **Tutorial Pair 9: Observability** *(coming soon)*.
