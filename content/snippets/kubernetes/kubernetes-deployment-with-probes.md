---
title: "Kubernetes Deployment with Resource Limits and Probes"
date: 2026-07-25
description: "Production-ready Deployment manifest with resource requests/limits, liveness and readiness probes, and a non-root security context."
lang: "Kubernetes"
tags: ["kubernetes", "deployment", "probes", "security-context", "resources"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-07-25"
draft: false
---

## When to use this

Any time you write a Deployment destined for a shared cluster or production — Kubernetes applies no limits and no health checks by default, which is never acceptable in a multi-tenant or customer-facing environment.

## Without it

```yaml
# A pod with no limits or probes — the Kubernetes default
containers:
  - name: my-app
    image: my-registry/my-app:1.0.0
    ports:
      - containerPort: 8080
```

A memory leak or runaway loop will consume the entire node's resources and evict neighbouring pods. A crashed-but-running container will keep receiving traffic because Kubernetes has no signal that anything is wrong.

## Snippet

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: default
  labels:
    app: my-app
spec:
  replicas: 2
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        fsGroup: 2000
      containers:
        - name: my-app
          image: my-registry/my-app:1.0.0
          ports:
            - containerPort: 8080
          resources:
            requests:
              cpu: "100m"
              memory: "128Mi"
            limits:
              cpu: "500m"
              memory: "256Mi"
          readinessProbe:
            httpGet:
              path: /healthz/ready
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 10
            failureThreshold: 3
          livenessProbe:
            httpGet:
              path: /healthz/live
              port: 8080
            initialDelaySeconds: 15
            periodSeconds: 20
            failureThreshold: 3
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities:
              drop: ["ALL"]
```

**Key decisions:**

| Field | Why it matters |
|---|---|
| `requests` vs `limits` | Requests drive scheduling; limits cap burst. Setting both prevents OOMKill surprises and noisy-neighbour evictions. |
| `readinessProbe` | Keeps traffic away from pods not yet ready — critical during rollouts so users never hit a starting container. |
| `livenessProbe` | Restarts pods stuck in a broken state that readiness alone won't catch (e.g. deadlocked goroutine). |
| `runAsNonRoot` | Prevents privilege escalation if the container process is compromised. |
| `readOnlyRootFilesystem` | Blocks an attacker from writing malicious files or binaries to the container filesystem. |
| `capabilities: drop: ["ALL"]` | Removes all Linux capabilities — grant back only what the app explicitly needs via `add`. |

## Verify it worked

Apply the manifest, then confirm each concern is covered:

```bash
# Watch the rollout complete cleanly
kubectl rollout status deployment/my-app

# Confirm resource limits are set on the running pod
kubectl get pod -l app=my-app -o jsonpath='{.items[0].spec.containers[0].resources}' | jq .

# Confirm both probes are configured
kubectl describe pod -l app=my-app | grep -A 6 "Liveness\|Readiness"

# Confirm the pod is running as a non-root user
kubectl get pod -l app=my-app -o jsonpath='{.items[0].spec.securityContext}'
```

Expected output for the resources check:

```json
{
  "limits":   { "cpu": "500m", "memory": "256Mi" },
  "requests": { "cpu": "100m", "memory": "128Mi" }
}
```

## Full walkthrough

Deep-dive into every field in this snippet, including how to tune probe timings and choose the right UID → **Tutorial Pair 21: Security Contexts** *(coming soon)*.
