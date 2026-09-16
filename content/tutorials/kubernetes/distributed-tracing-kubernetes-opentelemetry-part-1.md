---
title: "Setting Up Distributed Tracing in Kubernetes with OpenTelemetry — Part 1"
date: 2026-09-01
description: "Deploy the OpenTelemetry Collector, instrument a workload with the OTel SDK, and send traces to Jaeger. Build the tracing foundation for cluster-wide observability."
cluster: "Kubernetes"
series: "Observability"
part: 1
difficulty: "intermediate"
duration: "50 min"
tags: ["kubernetes", "opentelemetry", "tracing", "observability", "jaeger", "otel", "devsecops"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

By the end of this tutorial you will have the OpenTelemetry Collector deployed in your cluster, a sample application instrumented with the OTel SDK sending traces, and Jaeger receiving and displaying those traces. Part 2 extends this by correlating traces with Prometheus metrics and Loki logs to build a unified observability stack.

## Prerequisites

- A running Kubernetes cluster (v1.25+)
- `kubectl` with cluster-admin access
- `helm` v3 installed
- Basic familiarity with HTTP services and Kubernetes Deployments

---

## Why OpenTelemetry

Before OpenTelemetry, every observability vendor (Jaeger, Zipkin, Datadog, Honeycomb) required a different SDK and agent. Switching backends meant re-instrumenting your code. OpenTelemetry (OTel) solves this with a vendor-neutral standard:

- **SDK** — libraries for instrumenting your application code (Go, Python, Java, Node.js, .NET, and more)
- **Collector** — a standalone agent/gateway that receives, processes, and exports telemetry in any format to any backend
- **Protocol** — OTLP (OpenTelemetry Protocol), the wire format used between SDKs, Collectors, and backends

The result: instrument once with the OTel SDK, route to any backend via the Collector. Changing from Jaeger to Honeycomb is a Collector configuration change, not a code change.

The three signals OTel covers:

| Signal | What it captures |
|---|---|
| **Traces** | The path of a request through distributed services, with timing per operation |
| **Metrics** | Numeric measurements over time (latency histograms, error rates, counts) |
| **Logs** | Structured event records, linked to traces via trace context |

---

## Step 1 — Deploy Jaeger as the trace backend

Jaeger is the standard open-source trace backend. Deploy it in-cluster using the all-in-one image for this tutorial:

```bash
kubectl create namespace observability

kubectl apply -n observability -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: jaeger
  namespace: observability
spec:
  replicas: 1
  selector:
    matchLabels:
      app: jaeger
  template:
    metadata:
      labels:
        app: jaeger
    spec:
      containers:
        - name: jaeger
          image: jaegertracing/all-in-one:1.58
          ports:
            - containerPort: 16686   # UI
            - containerPort: 4317    # OTLP gRPC
            - containerPort: 4318    # OTLP HTTP
          env:
            - name: COLLECTOR_OTLP_ENABLED
              value: "true"
---
apiVersion: v1
kind: Service
metadata:
  name: jaeger
  namespace: observability
spec:
  selector:
    app: jaeger
  ports:
    - name: ui
      port: 16686
      targetPort: 16686
    - name: otlp-grpc
      port: 4317
      targetPort: 4317
    - name: otlp-http
      port: 4318
      targetPort: 4318
EOF
```

Verify Jaeger is running:

```bash
kubectl get pods -n observability -l app=jaeger
```

Access the Jaeger UI:

```bash
kubectl port-forward -n observability svc/jaeger 16686:16686
# Open http://localhost:16686
```

---

## Step 2 — Install the OpenTelemetry Operator

The OTel Operator manages `OpenTelemetryCollector` and `Instrumentation` custom resources — it handles deploying and configuring the Collector and auto-instrumenting workloads without code changes.

```bash
helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
helm repo update

helm install opentelemetry-operator open-telemetry/opentelemetry-operator \
  --namespace opentelemetry-operator-system \
  --create-namespace \
  --set admissionWebhooks.certManager.enabled=false \
  --set admissionWebhooks.autoGenerateCert.enabled=true
```

Wait for the operator to be ready:

```bash
kubectl get pods -n opentelemetry-operator-system
```

---

## Step 3 — Deploy the OpenTelemetry Collector

Create an `OpenTelemetryCollector` resource. The Operator deploys and manages the Collector pod based on this spec:

```yaml
# otel-collector.yaml
apiVersion: opentelemetry.io/v1alpha1
kind: OpenTelemetryCollector
metadata:
  name: otel-collector
  namespace: observability
spec:
  mode: deployment
  config: |
    receivers:
      otlp:
        protocols:
          grpc:
            endpoint: 0.0.0.0:4317
          http:
            endpoint: 0.0.0.0:4318

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

    exporters:
      otlp/jaeger:
        endpoint: jaeger.observability.svc.cluster.local:4317
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
```

```bash
kubectl apply -f otel-collector.yaml
kubectl get pods -n observability -l app.kubernetes.io/component=opentelemetry-collector
```

The Collector pipeline:
1. **Receivers** — accept OTLP over gRPC (port 4317) and HTTP (port 4318)
2. **Processors** — batch spans for efficiency, enforce memory limits, add cluster metadata
3. **Exporters** — forward to Jaeger and log to stdout for debugging

---

## Step 4 — Deploy a sample instrumented application

Deploy a simple Go HTTP service that emits traces using the OTel SDK. This represents any application in your cluster:

```yaml
# sample-app.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: sample-app
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: sample-app
  template:
    metadata:
      labels:
        app: sample-app
    spec:
      containers:
        - name: app
          image: ghcr.io/open-telemetry/demo/frontendproxy:1.10.0
          ports:
            - containerPort: 8080
          env:
            - name: OTEL_EXPORTER_OTLP_ENDPOINT
              value: "http://otel-collector-collector.observability.svc.cluster.local:4318"
            - name: OTEL_SERVICE_NAME
              value: "sample-app"
            - name: OTEL_RESOURCE_ATTRIBUTES
              value: "service.version=1.0.0,deployment.environment=development"
---
apiVersion: v1
kind: Service
metadata:
  name: sample-app
  namespace: default
spec:
  selector:
    app: sample-app
  ports:
    - port: 8080
      targetPort: 8080
```

```bash
kubectl apply -f sample-app.yaml
kubectl get pods -l app=sample-app
```

The two critical environment variables:

| Variable | Purpose |
|---|---|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Where to send telemetry — points to the Collector |
| `OTEL_SERVICE_NAME` | The service name that appears in Jaeger traces |

---

## Step 5 — Instrument with auto-instrumentation (no code changes)

For workloads written in Java, Python, Node.js, or .NET, the OTel Operator supports auto-instrumentation — it injects an SDK init container at pod startup without modifying application code.

Create an `Instrumentation` resource:

```yaml
# auto-instrumentation.yaml
apiVersion: opentelemetry.io/v1alpha1
kind: Instrumentation
metadata:
  name: otel-instrumentation
  namespace: default
spec:
  exporter:
    endpoint: http://otel-collector-collector.observability.svc.cluster.local:4318
  propagators:
    - tracecontext
    - baggage
  sampler:
    type: parentbased_traceidratio
    argument: "1.0"
  python:
    env:
      - name: OTEL_LOG_LEVEL
        value: debug
  nodejs:
    env:
      - name: OTEL_LOG_LEVEL
        value: info
  java:
    image: ghcr.io/open-telemetry/opentelemetry-operator/autoinstrumentation-java:latest
```

```bash
kubectl apply -f auto-instrumentation.yaml
```

Annotate any Deployment to enable auto-instrumentation — no code changes required:

```bash
# For a Python application
kubectl annotate deployment my-python-app \
  instrumentation.opentelemetry.io/inject-python="otel-instrumentation" \
  -n default

# For a Node.js application
kubectl annotate deployment my-node-app \
  instrumentation.opentelemetry.io/inject-nodejs="otel-instrumentation" \
  -n default

# For a Java application
kubectl annotate deployment my-java-app \
  instrumentation.opentelemetry.io/inject-java="otel-instrumentation" \
  -n default
```

The Operator injects an init container that configures the OTel SDK before the application starts. Traces flow to the Collector automatically.

---

## Step 6 — Generate traffic and view traces

Generate traffic against the sample app to produce traces:

```bash
kubectl port-forward svc/sample-app 8080:8080 &

# Generate requests
for i in $(seq 1 20); do
  curl -s http://localhost:8080/ > /dev/null
  sleep 0.5
done
```

Open the Jaeger UI (`http://localhost:16686`) and:

1. Select `sample-app` from the **Service** dropdown
2. Click **Find Traces**
3. Click a trace to see the span waterfall

Each trace shows:
- The root span (the incoming HTTP request)
- Child spans for downstream calls (database queries, external API calls, internal functions)
- Timing for each span
- Attributes (HTTP method, status code, URL, custom tags)

---

## Step 7 — Add custom spans and attributes

Auto-instrumentation captures HTTP boundaries automatically. For business logic inside a function, add custom spans manually using the OTel SDK:

### Go example

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
)

func processOrder(ctx context.Context, orderID string) error {
    tracer := otel.Tracer("order-service")
    ctx, span := tracer.Start(ctx, "processOrder")
    defer span.End()

    span.SetAttributes(
        attribute.String("order.id", orderID),
        attribute.String("order.status", "processing"),
    )

    // business logic here
    if err := validateOrder(ctx, orderID); err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        return err
    }

    span.SetAttributes(attribute.String("order.status", "validated"))
    return nil
}
```

### Python example

```python
from opentelemetry import trace

tracer = trace.get_tracer("order-service")

def process_order(order_id: str):
    with tracer.start_as_current_span("processOrder") as span:
        span.set_attribute("order.id", order_id)
        span.set_attribute("order.status", "processing")
        # business logic here
        span.set_attribute("order.status", "completed")
```

Custom attributes appear in the Jaeger trace detail and can be used to filter and search traces.

---

## Step 8 — Sampling strategy

In production, tracing every request is expensive. Configure sampling to control the volume of traces collected:

```yaml
# In the Instrumentation resource
spec:
  sampler:
    type: parentbased_traceidratio
    argument: "0.1"    # Sample 10% of traces
```

Sampling types:

| Type | Behaviour |
|---|---|
| `always_on` | Trace every request — development only |
| `always_off` | Trace nothing |
| `traceidratio` | Sample a percentage of requests at the SDK |
| `parentbased_traceidratio` | Respect parent sampling decision; if no parent, sample at ratio |

`parentbased_traceidratio` is the correct production default — it ensures that if an upstream service decided to sample a request, all downstream services also trace that request, preserving complete traces rather than partial ones.

---

## What you have built

- Jaeger deployed as the trace backend in the `observability` namespace
- OpenTelemetry Operator managing the Collector and Instrumentation resources
- OTel Collector with OTLP receivers, batching, memory limiting, and Jaeger export
- A sample application emitting traces via environment variable configuration
- Auto-instrumentation via Operator annotation for Java, Python, and Node.js
- Custom spans and attributes for business logic instrumentation
- Sampling strategy configured for production traffic volumes

## Next steps

In [Part 2](/tutorials/kubernetes/correlating-traces-metrics-logs-observability-part-2/) you will extend the Collector pipeline to also receive Prometheus metrics and Loki logs, correlate traces with metrics using exemplars, and link logs to traces using trace context injection — building a unified observability stack where a single trace ID connects all three signals.
