---
title: "Correlating Traces, Metrics, and Logs for Cluster-Wide Observability — Part 2"
date: 2026-09-01
description: "Extend the OTel Collector to receive metrics and logs alongside traces. Correlate all three signals using trace context, exemplars, and Grafana datasource linking."
cluster: "Kubernetes"
series: "Observability"
part: 2
difficulty: "advanced"
duration: "55 min"
tags: ["kubernetes", "opentelemetry", "observability", "prometheus", "loki", "grafana", "tracing", "metrics", "logs"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

In [Part 1](/tutorials/kubernetes/distributed-tracing-kubernetes-opentelemetry-part-1/) you deployed the OTel Collector and sent traces to Jaeger. In Part 2 you will extend the Collector to handle all three observability signals — traces, metrics, and logs — and connect them in Grafana so a single trace ID navigates between a distributed trace, the metrics at that moment, and the logs from that request.

## Prerequisites

- Completed [Part 1](/tutorials/kubernetes/distributed-tracing-kubernetes-opentelemetry-part-1/) — OTel Operator, Collector, and Jaeger deployed
- Prometheus and Grafana deployed in the `monitoring` namespace (via `kube-prometheus-stack` or equivalent)
- Loki deployed in the `monitoring` namespace

---

## The three-signal observability model

Traces, metrics, and logs are complementary — each answers a different question:

| Signal | Question answered |
|---|---|
| **Metrics** | Is something wrong? (error rate spike, latency p99 increase) |
| **Traces** | Where is it wrong? (which service, which operation, which downstream call) |
| **Logs** | Why is it wrong? (the error message, stack trace, context at the moment of failure) |

The workflow: an alert fires on a metric. You open the trace for a failed request. From the trace you jump to the logs for that exact request. Without correlation, this navigation is manual and slow. With trace context propagation, a single trace ID links all three.

---

## Step 1 — Extend the Collector for metrics

Update the `OpenTelemetryCollector` to add a Prometheus receiver and a metrics pipeline. The Prometheus receiver scrapes metrics from pods that expose a `/metrics` endpoint:

```yaml
# otel-collector-full.yaml
apiVersion: opentelemetry.io/v1alpha1
kind: OpenTelemetryCollector
metadata:
  name: otel-collector
  namespace: observability
spec:
  mode: deployment
  serviceAccount: otel-collector-sa
  config: |
    receivers:
      otlp:
        protocols:
          grpc:
            endpoint: 0.0.0.0:4317
          http:
            endpoint: 0.0.0.0:4318
      prometheus:
        config:
          scrape_configs:
            - job_name: 'otel-collector'
              static_configs:
                - targets: ['0.0.0.0:8888']
            - job_name: 'kubernetes-pods'
              kubernetes_sd_configs:
                - role: pod
              relabel_configs:
                - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
                  action: keep
                  regex: "true"
                - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_path]
                  action: replace
                  target_label: __metrics_path__
                  regex: (.+)
                - source_labels: [__address__, __meta_kubernetes_pod_annotation_prometheus_io_port]
                  action: replace
                  target_label: __address__
                  regex: ([^:]+)(?::\d+)?;(\d+)
                  replacement: $1:$2

    processors:
      batch:
        timeout: 1s
        send_batch_size: 1024
      memory_limiter:
        check_interval: 1s
        limit_mib: 512
        spike_limit_mib: 128
      resource:
        attributes:
          - key: cluster.name
            value: my-cluster
            action: insert
      transform/exemplars:
        metric_statements:
          - context: datapoint
            statements:
              - set(exemplar.filtered_attributes["trace_id"], trace_id.string)
                where exemplar.filtered_attributes["trace_id"] != ""

    exporters:
      otlp/jaeger:
        endpoint: jaeger.observability.svc.cluster.local:4317
        tls:
          insecure: true
      prometheusremotewrite:
        endpoint: http://prometheus-operated.monitoring.svc.cluster.local:9090/api/v1/write
        tls:
          insecure: true
      debug:
        verbosity: basic

    service:
      pipelines:
        traces:
          receivers: [otlp]
          processors: [memory_limiter, resource, batch]
          exporters: [otlp/jaeger, debug]
        metrics:
          receivers: [otlp, prometheus]
          processors: [memory_limiter, resource, transform/exemplars, batch]
          exporters: [prometheusremotewrite]
```

```bash
kubectl apply -f otel-collector-full.yaml
```

The `transform/exemplars` processor extracts the trace ID from OTel metric datapoints and attaches it as a Prometheus exemplar — enabling the Grafana trace-to-metrics link.

---

## Step 2 — Add a logs pipeline with Loki export

Extend the Collector with a log receiver and Loki exporter. The `filelog` receiver tails container log files on the node — deploy the Collector as a `DaemonSet` for log collection:

```yaml
# otel-daemonset-logs.yaml
apiVersion: opentelemetry.io/v1alpha1
kind: OpenTelemetryCollector
metadata:
  name: otel-logs
  namespace: observability
spec:
  mode: daemonset
  tolerations:
    - operator: Exists
  config: |
    receivers:
      filelog:
        include:
          - /var/log/pods/*/*/*.log
        exclude:
          - /var/log/pods/observability_*/*/*.log
        start_at: beginning
        include_file_path: true
        include_file_name: false
        operators:
          - type: router
            id: get-format
            routes:
              - output: parser-docker
                expr: 'body matches "^\\{"'
              - output: parser-crio
                expr: 'body matches "^[^ Z]+ "'
          - type: json_parser
            id: parser-docker
            output: extract-metadata
          - type: regex_parser
            id: parser-crio
            regex: '^(?P<time>[^ Z]+) (?P<stream>stdout|stderr) (?P<logtag>[^ ]*) ?(?P<log>.*)$'
            output: extract-metadata
          - type: metadata
            id: extract-metadata
            resource:
              k8s.pod.name: 'EXPR(attributes["k8s.pod.name"])'
              k8s.namespace.name: 'EXPR(attributes["k8s.namespace.name"])'
              k8s.container.name: 'EXPR(attributes["k8s.container.name"])'

    processors:
      batch:
        timeout: 5s
      k8sattributes:
        auth_type: serviceAccount
        passthrough: false
        extract:
          metadata:
            - k8s.pod.name
            - k8s.pod.uid
            - k8s.deployment.name
            - k8s.namespace.name
            - k8s.node.name
            - k8s.pod.start_time
          labels:
            - tag_name: app
              key: app
              from: pod
        pod_association:
          - sources:
              - from: resource_attribute
                name: k8s.pod.ip
          - sources:
              - from: resource_attribute
                name: k8s.pod.uid

    exporters:
      loki:
        endpoint: http://loki.monitoring.svc.cluster.local:3100/loki/api/v1/push
        default_labels_enabled:
          exporter: false
          job: true
          instance: true
          level: true

    service:
      pipelines:
        logs:
          receivers: [filelog]
          processors: [k8sattributes, batch]
          exporters: [loki]
```

```bash
kubectl apply -f otel-daemonset-logs.yaml
kubectl get pods -n observability -l app.kubernetes.io/component=opentelemetry-collector
```

The `k8sattributes` processor enriches each log line with Kubernetes metadata (pod name, namespace, deployment name, node) by querying the Kubernetes API. This metadata becomes Loki labels, enabling filtering by namespace, pod, and deployment in Grafana.

---

## Step 3 — Inject trace context into logs

For traces and logs to be correlated, the trace ID must appear in log lines. The OTel SDK injects the trace context into log records automatically when you use the OTel logging bridge.

### Go — using zap with OTel bridge

```go
import (
    "go.opentelemetry.io/contrib/bridges/otelzap"
    "go.uber.org/zap"
    "go.opentelemetry.io/otel/trace"
)

// Create a zap logger with OTel bridge
core := otelzap.NewCore("my-service", otelzap.WithLoggerProvider(loggerProvider))
logger := zap.New(core)

func handleRequest(ctx context.Context) {
    span := trace.SpanFromContext(ctx)
    // Log will automatically include trace_id and span_id
    logger.InfoContext(ctx, "processing request",
        zap.String("user_id", "123"),
    )
}
```

### Python — using logging with OTel bridge

```python
from opentelemetry.sdk._logs import LoggerProvider
from opentelemetry.sdk._logs.export import BatchLogRecordProcessor
from opentelemetry.exporter.otlp.proto.http._log_exporter import OTLPLogExporter
import logging

# Configure OTel logging bridge
logger_provider = LoggerProvider()
logger_provider.add_log_record_processor(
    BatchLogRecordProcessor(OTLPLogExporter())
)

# Standard Python logging now includes trace context
logging.getLogger(__name__).info("Processing request", extra={"user_id": "123"})
```

When a log record is emitted inside a traced context, the OTel bridge adds `trace_id` and `span_id` fields automatically. These appear in Loki as structured log attributes.

---

## Step 4 — Configure Grafana datasource linking

With all three signals flowing, configure Grafana to enable navigation between them.

### Jaeger datasource (traces)

In Grafana → Configuration → Data Sources → Add Jaeger:

```
URL: http://jaeger.observability.svc.cluster.local:16686
```

### Prometheus datasource with exemplars

Edit your existing Prometheus datasource in Grafana and enable exemplars:

```
Exemplars: enabled
Internal link: Jaeger (select the Jaeger datasource)
Label name: trace_id
```

### Loki datasource with trace linking

Edit your Loki datasource and add a derived field:

```
Derived fields:
  Name: TraceID
  Regex: "trace_id":"([a-f0-9]+)"
  Internal link: Jaeger
  Query: ${__value.raw}
```

With these links in place, Grafana enables:
- **Metric → Trace:** click an exemplar point on a Prometheus graph → jumps to the Jaeger trace
- **Log → Trace:** click the TraceID in a Loki log line → jumps to the Jaeger trace
- **Trace → Log:** from a Jaeger trace, use the Grafana Explore link → queries Loki for logs with that trace ID

---

## Step 5 — Build a RED metrics dashboard

The Rate/Error/Duration (RED) method is the standard for service health dashboards. These metrics come from OTel SDK instrumentation and are scraped by the Collector's Prometheus receiver.

Key Prometheus queries for a RED dashboard:

```promql
# Request rate (requests per second)
rate(http_server_request_duration_seconds_count{service_name="sample-app"}[5m])

# Error rate (percentage of 5xx responses)
rate(http_server_request_duration_seconds_count{
  service_name="sample-app",
  http_response_status_code=~"5.."
}[5m])
/
rate(http_server_request_duration_seconds_count{
  service_name="sample-app"
}[5m]) * 100

# p50, p95, p99 latency
histogram_quantile(0.99,
  rate(http_server_request_duration_seconds_bucket{
    service_name="sample-app"
  }[5m])
)
```

Add these as Grafana panels. Enable exemplar display on the latency panel — each high-latency data point will show an exemplar dot that links directly to the trace for that request.

---

## Step 6 — Correlate an incident end-to-end

With all three signals connected, the incident investigation workflow becomes:

**1. Alert fires** — Prometheus alerts on `p99 latency > 2s` for `sample-app`

**2. Open the Grafana dashboard** — the latency panel shows the spike and exemplar dots at the peak

**3. Click an exemplar** — Grafana opens the Jaeger trace for a slow request at that timestamp

**4. Inspect the trace** — the waterfall shows a 1.8s span on the `db-query` operation. The downstream database call is the bottleneck.

**5. Click the trace ID in Grafana Explore** — Loki shows log lines for that exact request, including the database query, its parameters, and the error that occurred

**6. Root cause identified** — the log line shows `ERROR: deadlock detected on table orders`. The trace pinpointed the slow operation; the log provided the exact error message.

This flow — alert → metric → trace → log — resolves incidents in minutes rather than hours. Without correlation, each signal is a separate investigation.

---

## Step 7 — OTel Collector health monitoring

The Collector itself emits metrics on port 8888. Add it to your Prometheus scrape targets:

```yaml
# prometheus-scrape-otel.yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: otel-collector
  namespace: observability
spec:
  selector:
    matchLabels:
      app.kubernetes.io/component: opentelemetry-collector
  endpoints:
    - port: metrics
      interval: 30s
      path: /metrics
```

Key Collector health metrics:

```promql
# Spans received per second
rate(otelcol_receiver_accepted_spans_total[5m])

# Spans dropped (pipeline backpressure)
rate(otelcol_processor_dropped_spans_total[5m])

# Exporter failures
rate(otelcol_exporter_send_failed_spans_total[5m])

# Queue size (approaching limit = backpressure risk)
otelcol_exporter_queue_size
```

Alert when `otelcol_processor_dropped_spans_total` is non-zero — dropped spans mean incomplete traces and gaps in observability.

---

## Step 8 — Production Collector sizing

The single-deployment Collector from Part 1 is a starting point. In production:

| Deployment mode | Use case |
|---|---|
| `deployment` | Gateway — receives from all cluster agents, processes, exports |
| `daemonset` | Agent — runs on every node, collects node-level metrics and logs |
| `sidecar` | Per-pod collector — for high-volume services needing isolation |

A common production topology: a DaemonSet agent on every node (receives local spans, scrapes node metrics, tails logs) sending to a central Deployment gateway (batches, filters, routes to multiple backends).

Configure resource limits for the gateway Collector based on your span volume. A starting point for a medium-sized cluster (50 nodes, 500 RPS):

```yaml
resources:
  requests:
    cpu: "500m"
    memory: "512Mi"
  limits:
    cpu: "2000m"
    memory: "2Gi"
```

Scale the gateway Collector horizontally for higher volumes — the OTLP load balancer exporter distributes spans across multiple Collector replicas consistently by trace ID, keeping all spans for a single trace on the same Collector instance.

---

## What you have built

- OTel Collector extended with Prometheus scraping and metrics pipeline
- DaemonSet log Collector with `filelog` receiver, `k8sattributes` enrichment, and Loki export
- Trace context injected into logs via OTel logging bridge
- Grafana datasource links connecting metrics exemplars and log trace IDs to Jaeger traces
- RED dashboard (rate, error rate, p50/p95/p99 latency) with exemplar trace links
- End-to-end incident correlation: alert → metric → trace → log
- Collector health monitoring with drop and failure alerts
- Production topology guidance for DaemonSet + Deployment Collector architecture

Your cluster now has unified observability: a single trace ID navigates between all three signals, and every alert has a direct path to root cause without manual log searching or timestamp guessing.
