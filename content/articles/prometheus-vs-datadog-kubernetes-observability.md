---
title: "Prometheus vs Datadog for Kubernetes Observability"
date: 2026-08-06
author: "Victor D"
description: "Prometheus and Datadog both give you Kubernetes metrics, dashboards, and alerts. The difference is what you pay with: money or engineering time. Neither is free."
tags: ["prometheus", "datadog", "observability", "kubernetes", "monitoring", "grafana", "devops", "comparisons"]
categories: ["article"]
draft: false
toc: true
---

The comparison between Prometheus and Datadog is often framed as open source vs commercial, but that framing misses the real question. Prometheus is not free — you pay with engineering time, infrastructure cost, and operational burden. Datadog is not just expensive — the cost buys you something real. The decision is about which currency you would rather spend and what your team is actually equipped to operate.

Both tools solve the same core problem. Both will tell you when your pods are crash-looping, your node memory is exhausted, or your API latency has spiked. The differences emerge in what surrounds the metrics — logs, traces, cost at scale, and the amount of glue your team has to write and maintain.

---

## The Prometheus stack — it is more than Prometheus

When people say "Prometheus," they usually mean the full observability stack that has grown around it:

- **Prometheus** — metrics collection and storage, PromQL query engine
- **Alertmanager** — alert routing, deduplication, silencing, notification channels
- **Grafana** — dashboards and visualisation
- **kube-state-metrics** — exposes Kubernetes object state (Deployment replicas, Pod phases, Node conditions) as Prometheus metrics
- **node-exporter** — exposes host-level metrics (CPU, memory, disk, network) per node
- **Loki** — log aggregation (separate project, also from Grafana Labs)
- **Tempo** — distributed tracing (separate project)
- **Thanos or Grafana Mimir** — long-term metric storage and multi-cluster query federation

The `kube-prometheus-stack` Helm chart installs Prometheus, Alertmanager, Grafana, kube-state-metrics, and node-exporter with sensible defaults and pre-built dashboards for Kubernetes. It is a reasonable starting point, but "starting point" is the right frame — running it in production at scale requires significant additional work.

Prometheus is managed in Kubernetes using the **Prometheus Operator**, which introduces CRDs for configuration:

```yaml
# ServiceMonitor — tells Prometheus to scrape a service
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: api-server
  namespace: production
spec:
  selector:
    matchLabels:
      app: api-server
  endpoints:
  - port: metrics
    interval: 30s
    path: /metrics
```

```yaml
# PrometheusRule — defines alerting and recording rules
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: api-server-alerts
  namespace: production
spec:
  groups:
  - name: api-server
    rules:
    - alert: HighErrorRate
      expr: |
        rate(http_requests_total{status=~"5.."}[5m])
        / rate(http_requests_total[5m]) > 0.05
      for: 5m
      labels:
        severity: warning
      annotations:
        summary: "Error rate above 5% on {{ $labels.service }}"
```

PromQL — Prometheus's query language — is powerful and precise for time-series analysis:

```promql
# 95th percentile API latency over the last 5 minutes, by service
histogram_quantile(0.95,
  sum by (le, service) (
    rate(http_request_duration_seconds_bucket[5m])
  )
)

# CPU throttling ratio per pod
rate(container_cpu_throttled_seconds_total[5m])
/ rate(container_cpu_usage_seconds_total[5m])
```

PromQL rewards investment. It is expressive once you understand its model, but the learning curve is real.

---

## Datadog — one agent, one platform

Datadog is a SaaS observability platform. You deploy the Datadog Agent as a DaemonSet in your cluster; it collects metrics, logs, and traces and ships them to Datadog's infrastructure. Everything else — storage, retention, dashboards, alerting, querying — happens in Datadog's cloud.

```yaml
# Datadog Agent via the Datadog Operator
apiVersion: datadoghq.com/v2alpha1
kind: DatadogAgent
metadata:
  name: datadog
  namespace: datadog
spec:
  global:
    credentials:
      apiSecret:
        secretName: datadog-secret
        keyName: api-key
  features:
    apm:
      enabled: true
    logCollection:
      enabled: true
      containerCollectAll: true
    liveProcessCollection:
      enabled: true
    orchestratorExplorer:
      enabled: true   # Kubernetes resource explorer
    npm:
      enabled: true   # Network Performance Monitoring
```

Datadog's Kubernetes integration gives you pre-built dashboards for cluster health, node performance, pod states, and namespace-level resource consumption — all available immediately after the agent is running, with no dashboard configuration required. The **Orchestrator Explorer** provides a live view of every Kubernetes resource in the cluster, with filtering, live container logs, and event history.

This out-of-the-box experience is where Datadog consistently wins in early evaluations. A team can go from zero to meaningful dashboards in an hour. The equivalent Prometheus journey involves installing the stack, tuning scrape intervals, importing or building Grafana dashboards, writing alerting rules, and configuring Alertmanager notification channels.

---

## The cost question

This is the conversation that comparison articles usually avoid. Let's have it directly.

**Prometheus stack costs:**

- **Infrastructure**: Prometheus storage grows with metric volume. At 100 services scraping every 15 seconds, you need meaningful disk (hundreds of gigabytes) and the compute to run Prometheus, Grafana, Alertmanager, Loki, and Thanos or Mimir. In a managed Kubernetes cluster, this adds to your node costs.
- **Engineering time**: Someone has to install, upgrade, tune, and operate this stack. Prometheus storage compaction, cardinality management, Alertmanager routing configuration, Grafana dashboard maintenance, Loki log retention — these are ongoing engineering tasks, not one-time setup.
- **Long-term retention**: Prometheus default retention is 15 days. Getting 13 months of metric history (a common compliance requirement) requires Thanos or Mimir, which adds architectural complexity and storage cost.

**Datadog costs:**

Datadog bills primarily per host (the node running the agent), with additional charges for:

- **Custom metrics** — metrics beyond Datadog's built-in integrations; charged per metric per month above a free tier. This is where Datadog bills surprise teams: an application emitting 50 custom metrics across 100 pods can consume custom metric quota quickly.
- **Log Management** — priced per GB ingested and indexed. Kubernetes clusters generate a lot of logs; at scale this line item grows fast.
- **APM** — priced per host. Adding distributed tracing to your observability stack at Datadog pricing is a separate budget conversation.

A rough threshold: for teams below ~30–50 nodes with modest custom metric and log volumes, Datadog's per-host pricing is often comparable to or cheaper than the infrastructure and engineering cost of running the Prometheus stack. Above that threshold, Datadog's costs tend to grow faster than the savings from managed infrastructure.

{{< callout type="warning" title="The custom metrics trap" >}}
Datadog's default custom metric quota is 100 per host. Kubernetes workloads are label-heavy, and each label combination on a metric creates a separate time series counted against your custom metric quota. Before committing to Datadog, instrument a representative workload and measure custom metric usage against your anticipated bill.
{{< /callout >}}

---

## Cardinality: where Prometheus hurts

High cardinality is the most common Prometheus production problem and deserves its own section.

In Prometheus, every unique combination of label values creates a separate time series. A metric like `http_requests_total` with labels `{service, endpoint, status_code, pod_name}` can explode: 10 services × 50 endpoints × 10 status codes × 100 pods = 500,000 time series from one metric.

This is a real operational risk. High cardinality leads to Prometheus OOM crashes, scrape timeouts, and degraded query performance. The fix — removing high-cardinality labels like `pod_name` from metrics — requires changes to your application instrumentation, which is not always in your control.

```promql
# Check which metrics have the most time series
topk(10,
  count by (__name__) ({__name__=~".+"})
)
```

Datadog handles cardinality differently. Its tag-based data model indexes tags separately rather than creating a time series per combination. You can filter and group by `pod_name` in a Datadog dashboard without a cardinality explosion. This is a genuine architectural advantage.

---

## Logs: the full picture

**Prometheus has no log management.** Metrics only. If you want logs alongside metrics in the same platform, you need Loki (Grafana Labs' log aggregation system) or a separate solution like Elasticsearch.

Loki is intentionally simple — it indexes only log stream labels (like `namespace`, `pod`, `container`), not log content. This keeps storage cost low but means full-text search on log content requires querying across all streams. LogQL, Loki's query language, is modeled on PromQL and is learnable but is another syntax to maintain:

```logql
# Error logs from the api-server in the last hour
{namespace="production", app="api-server"}
  |= "ERROR"
  | json
  | line_format "{{.time}} {{.message}}"
```

Datadog Log Management indexes log content by default, enabling full-text search without configuration. Log pipelines can parse, enrich, and filter logs before storage. The trade-off is cost: indexed logs in Datadog are priced per GB and can become the largest line item in a busy cluster.

---

## Distributed tracing

**The Prometheus ecosystem** uses **Tempo** (Grafana Labs) or **Jaeger** for distributed tracing, with OpenTelemetry as the collection standard. Correlating traces with metrics and logs requires configuring exemplars in Prometheus and trace-to-log correlation in Grafana — possible, but requires deliberate setup.

**Datadog APM** is integrated into the same platform as metrics and logs. Flame charts, service maps, error tracking, and latency breakdowns are available with one click from a metric spike or a log error. The correlation between signals is automatic:

- Click a metric anomaly → see correlated traces from the same time window
- Click a log error → jump to the trace that generated it
- Click a slow span → see the host metrics for that pod at that moment

This unified workflow is Datadog's strongest practical advantage. In a production incident, navigating between three separate tools (Grafana for metrics, Loki for logs, Tempo for traces) while trying to diagnose a problem adds friction. Datadog's single pane of glass reduces that friction.

---

## Multi-cluster observability

**Prometheus** scales horizontally to multiple clusters through federation or Thanos/Mimir. In a federated model, a central Prometheus scrapes aggregated metrics from per-cluster Prometheus instances. Thanos adds a query layer that transparently queries across multiple Prometheus instances and provides long-term storage via object storage (S3, GCS):

```yaml
# Thanos Querier — queries across multiple Prometheus instances
apiVersion: monitoring.banzaicloud.io/v1alpha1
kind: ThanosQuery
spec:
  stores:
  - dnsdiscovery+http://prometheus-cluster-east:10901
  - dnsdiscovery+http://prometheus-cluster-west:10901
```

**Datadog** multi-cluster observability is simpler operationally: deploy the agent in each cluster, all data flows to the same Datadog organisation. Filtering by cluster is a tag filter. No additional infrastructure is required.

---

## Operational burden: the honest account

If your team has a dedicated platform engineer who understands Prometheus, PromQL, Alertmanager routing, and Grafana well — the Prometheus stack is a reasonable long-term choice. It is flexible, extensible, and the operational cost is predictable.

If your team is small, observability expertise is thin, or the platform team is already stretched managing Kubernetes itself — the operational cost of running the Prometheus stack is non-trivial. Prometheus upgrades, storage tuning, Alertmanager configuration drift, Grafana dashboard maintenance, and Loki retention management are ongoing work, not background tasks.

Datadog's operational burden is close to zero. Agent upgrades are rolling DaemonSet updates. Storage, retention, and reliability are Datadog's problem. The trade-off is vendor lock-in and a bill that grows with your cluster.

---

## How to choose

**Choose the Prometheus stack if:**

- You have engineering bandwidth to operate and tune it
- Cost at scale is a hard constraint — the infrastructure cost is predictable and scales better than Datadog per-host pricing above ~50 nodes
- You prefer open standards (OpenTelemetry, PromQL, Grafana) and avoiding vendor lock-in
- Your compliance requirements prevent shipping data to a third-party SaaS
- You have or plan to have a dedicated platform/SRE team

**Choose Datadog if:**

- Your team needs observability that works immediately without configuration investment
- You need correlated metrics, logs, and traces in one workflow — especially for incident response
- Your cluster is below ~50 nodes and the per-host cost is within budget
- You need security features alongside observability (Datadog CSPM and Workload Security share the same agent and console)
- The engineering time saved justifies the cost at your scale

**The hybrid approach:** Using the Prometheus stack for metrics while routing logs to Datadog (or vice versa) is more common than either/or comparisons suggest. Some teams run kube-prometheus-stack for Kubernetes infrastructure metrics and Datadog for APM and log management, keeping the flexibility of PromQL for infrastructure alerting while using Datadog's integrated trace-to-log workflow for application debugging.

---

## Related reading

- [Prometheus documentation](https://prometheus.io/docs/)
- [kube-prometheus-stack Helm chart](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack)
- [Grafana Loki documentation](https://grafana.com/docs/loki/latest/)
- [Thanos — long-term Prometheus storage](https://thanos.io/tip/thanos/getting-started.md/)
- [Datadog Kubernetes integration](https://docs.datadoghq.com/containers/kubernetes/)
- How we reduced Kubernetes cluster costs by 40% with VPA and node autoscaling → **Articles**
