---
title: "Kubernetes HorizontalPodAutoscaler — CPU, Memory, and Custom Metrics"
date: 2026-11-12
description: "HPA manifests for CPU and memory scaling, custom Prometheus metrics via the custom.metrics.k8s.io API, and multiple metrics with individual targets — plus stabilisation windows to prevent flapping."
lang: "Kubernetes"
tags: ["kubernetes", "hpa", "autoscaling", "prometheus", "custom-metrics", "keda"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-11-12"
draft: false
---

## When to use this

When workload demand is variable and you need replicas to scale up under load and back down when it drops. The HPA watches metrics and adjusts the replica count automatically — without it, you either over-provision (waste money) or under-provision (drop requests under load).

## Snippet

### CPU and memory scaling (most common starting point)

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: my-app
  namespace: my-app
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: my-app

  minReplicas: 2
  maxReplicas: 20

  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 60    # scale up when average CPU > 60% of request

    - type: Resource
      resource:
        name: memory
        target:
          type: Utilization
          averageUtilization: 70    # scale up when average memory > 70% of request

  behavior:
    scaleUp:
      stabilizationWindowSeconds: 60     # wait 60s before scaling up again
      policies:
        - type: Pods
          value: 4                       # add at most 4 pods per scaling event
          periodSeconds: 60
    scaleDown:
      stabilizationWindowSeconds: 300    # wait 5 min before scaling down
      policies:
        - type: Percent
          value: 25                      # remove at most 25% of pods per event
          periodSeconds: 60
```

### Custom Prometheus metric via Prometheus Adapter

```yaml
# Requires prometheus-adapter configured to expose custom.metrics.k8s.io
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: my-app-rps
  namespace: my-app
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: my-app

  minReplicas: 2
  maxReplicas: 50

  metrics:
    - type: Pods
      pods:
        metric:
          name: http_requests_per_second   # metric name as registered in prometheus-adapter
        target:
          type: AverageValue
          averageValue: "100"              # scale up when avg RPS per pod > 100

    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70           # CPU as a safety backstop
```

### External metric (queue depth — SQS, RabbitMQ)

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: worker
  namespace: workers
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: queue-worker

  minReplicas: 1
  maxReplicas: 30

  metrics:
    - type: External
      external:
        metric:
          name: sqs_approximate_number_of_messages_visible
          selector:
            matchLabels:
              queue: my-job-queue
        target:
          type: AverageValue
          averageValue: "50"               # one worker replica per 50 queued messages
```

### Prometheus Adapter ConfigMap — register a custom metric

```yaml
# prometheus-adapter/config.yaml — added to the adapter's ConfigMap
rules:
  - seriesQuery: 'http_requests_total{namespace!="",pod!=""}'
    resources:
      overrides:
        namespace: { resource: namespace }
        pod:       { resource: pod }
    name:
      matches: "^(.*)_total$"
      as: "${1}_per_second"
    metricsQuery: 'sum(rate(<<.Series>>{<<.LabelMatchers>>}[2m])) by (<<.GroupBy>>)'
```

**Key decisions:**

| Decision | Why it matters |
|---|---|
| `averageUtilization: 60` not 80+ | The HPA needs headroom to provision new pods before the existing ones are saturated. At 80%, a traffic spike may overwhelm current pods before the new ones pass their readiness probe and join the pool. 60% is a common production starting point. |
| `minReplicas: 2` | A single replica has no high-availability guarantee. A single bad node, rolling update, or eviction takes the workload to zero. `minReplicas: 2` with `podAntiAffinity` ensures at least two pods are always scheduled on separate nodes. |
| `stabilizationWindowSeconds: 300` on scale-down | Without a stabilisation window, a brief dip in traffic causes immediate scale-down followed by immediate scale-up when traffic recovers — the HPA flaps. 5 minutes of sustained low load before scaling down prevents this. |
| Scale-up by pods count, scale-down by percent | Adding a fixed number of pods (`type: Pods, value: 4`) bounds the rate of scale-up during traffic spikes. Removing a percentage on scale-down (`type: Percent, value: 25`) is gentler — it removes fewer pods from a large fleet than a fixed count would. |
| `type: Pods` not `type: Object` for per-pod metrics | `Pods` averages the metric across all pods matching the selector and divides by replica count — the right behaviour for per-pod metrics like RPS. `Object` reads a single metric from one object and does not scale with replica count. |
| `autoscaling/v2` not `v1` | `autoscaling/v1` only supports CPU. `v2` supports memory, custom metrics, external metrics, and `behavior` configuration. `v1` is effectively deprecated for production use. |

## Verify it worked

```bash
# Watch HPA status and current metric values
kubectl get hpa my-app -n my-app -w

# Detailed view including last scale event and current metrics
kubectl describe hpa my-app -n my-app

# Confirm custom metric is available
kubectl get --raw "/apis/custom.metrics.k8s.io/v1beta1" | jq '.resources[].name'

# Load test to trigger scale-up (requires hey or k6)
hey -z 2m -c 50 http://my-app.my-app.svc.cluster.local/

# Watch replicas change during the load test
kubectl get deployment my-app -n my-app -w
```
