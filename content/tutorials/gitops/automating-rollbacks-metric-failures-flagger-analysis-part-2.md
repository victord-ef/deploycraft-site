---
title: "Automating Rollbacks on Metric Failures with Flagger Analysis — Part 2"
date: 2026-09-02
description: "Build custom PromQL metric templates, configure A/B testing with header-based routing, implement blue-green deployments, add Slack and GitHub alert providers, and tune Flagger analysis thresholds to match real production SLOs."
cluster: "GitOps"
series: "Progressive Delivery"
part: 2
difficulty: "intermediate"
duration: "50 min"
tags: ["gitops", "flux", "flagger", "canary", "progressive-delivery", "kubernetes", "prometheus", "slo", "devops"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

In [Part 1](/tutorials/gitops/implementing-canary-deployments-flagger-flux-part-1/) you installed Flagger and ran a basic canary with built-in Prometheus metrics. In Part 2 you will write custom `MetricTemplate` resources backed by arbitrary PromQL queries, configure A/B testing for experiment-driven releases, implement blue-green deployments for databases and stateful services, connect Flagger to Slack and GitHub for alerting, and tune analysis parameters against real production SLOs.

## Prerequisites

- Completed [Part 1](/tutorials/gitops/implementing-canary-deployments-flagger-flux-part-1/) — Flagger running, at least one `Canary` resource successfully promoted
- Prometheus with application metrics available (request rate, error rate, latency histograms)
- Istio or Nginx ingress in the cluster

---

## Step 1 — Custom MetricTemplate resources

Flagger's built-in `request-success-rate` and `request-duration` metrics cover most cases. For business metrics — conversion rate, checkout completion, queue depth — use `MetricTemplate` resources with arbitrary PromQL:

```yaml
# apps/base/my-app/metric-templates.yaml
apiVersion: flagger.app/v1beta1
kind: MetricTemplate
metadata:
  name: error-rate
  namespace: my-app
spec:
  provider:
    type: prometheus
    address: http://prometheus-operated.monitoring:9090
  query: |
    100 - (
      sum(
        rate(
          http_requests_total{
            namespace="{{ namespace }}",
            pod=~"{{ target }}-[0-9a-zA-Z]+(-[0-9a-zA-Z]+)",
            status!~"5.."
          }[{{ interval }}]
        )
      )
      /
      sum(
        rate(
          http_requests_total{
            namespace="{{ namespace }}",
            pod=~"{{ target }}-[0-9a-zA-Z]+(-[0-9a-zA-Z]+)"
          }[{{ interval }}]
        )
      )
    ) * 100
```

Flagger substitutes `{{ namespace }}`, `{{ target }}` (the canary Deployment name), and `{{ interval }}` at query time. The query returns error rate as a percentage.

Reference the template in the `Canary`:

```yaml
spec:
  analysis:
    metrics:
      - name: error-rate
        templateRef:
          name: error-rate
          namespace: my-app
        thresholdRange:
          max: 1          # fail if error rate exceeds 1%
        interval: 1m

      - name: p99-latency
        templateRef:
          name: p99-latency
          namespace: my-app
        thresholdRange:
          max: 250        # fail if p99 latency exceeds 250ms
        interval: 1m
```

---

## Step 2 — P99 latency MetricTemplate

```yaml
# apps/base/my-app/metric-template-latency.yaml
apiVersion: flagger.app/v1beta1
kind: MetricTemplate
metadata:
  name: p99-latency
  namespace: my-app
spec:
  provider:
    type: prometheus
    address: http://prometheus-operated.monitoring:9090
  query: |
    histogram_quantile(
      0.99,
      sum(
        rate(
          http_request_duration_seconds_bucket{
            namespace="{{ namespace }}",
            pod=~"{{ target }}-[0-9a-zA-Z]+(-[0-9a-zA-Z]+)"
          }[{{ interval }}]
        )
      ) by (le)
    ) * 1000
```

This returns p99 request duration in milliseconds. The `histogram_quantile` function requires your application to expose Prometheus histogram metrics — available from standard Go, Java, and Node.js Prometheus client libraries.

---

## Step 3 — Business metric analysis

Canary analysis is not limited to infrastructure metrics. If a new version has a lower conversion rate — even with healthy HTTP metrics — you want to catch it. Define a business metric template:

```yaml
# apps/base/checkout/metric-template-conversion.yaml
apiVersion: flagger.app/v1beta1
kind: MetricTemplate
metadata:
  name: checkout-conversion
  namespace: checkout
spec:
  provider:
    type: prometheus
    address: http://prometheus-operated.monitoring:9090
  query: |
    sum(
      rate(
        checkout_completed_total{
          namespace="{{ namespace }}",
          pod=~"{{ target }}-[0-9a-zA-Z]+(-[0-9a-zA-Z]+)"
        }[{{ interval }}]
      )
    )
    /
    sum(
      rate(
        checkout_initiated_total{
          namespace="{{ namespace }}",
          pod=~"{{ target }}-[0-9a-zA-Z]+(-[0-9a-zA-Z]+)"
        }[{{ interval }}]
      )
    ) * 100
```

```yaml
spec:
  analysis:
    metrics:
      - name: checkout-conversion
        templateRef:
          name: checkout-conversion
          namespace: checkout
        thresholdRange:
          min: 85     # conversion rate must stay above 85%
        interval: 5m  # longer interval for business metrics — more data needed
```

A canary with a regression in checkout conversion fails the analysis even if HTTP success rates look healthy. This is a significant advantage over any purely infrastructure-based deployment gate.

---

## Step 4 — A/B testing with header-based routing

A/B testing differs from canary in its intent: instead of gradually shifting all traffic, you route a specific cohort (users with a feature flag, internal staff, beta users) to the new version while everyone else sees the stable version.

Configure Flagger for A/B testing by setting `iterations` instead of `stepWeight`:

```yaml
# apps/base/my-app/canary-ab.yaml
apiVersion: flagger.app/v1beta1
kind: Canary
metadata:
  name: my-app
  namespace: my-app
spec:
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: my-app
  provider: istio
  service:
    port: 80
    targetPort: 8080
    gateways:
      - public-gateway.istio-system.svc.cluster.local
    hosts:
      - my-app.example.com
  analysis:
    # A/B: run for N iterations, no traffic weight progression
    interval: 1m
    iterations: 10         # run 10 analysis intervals
    threshold: 2           # fail after 2 consecutive metric failures

    # Route matching — only users with this header see the canary
    match:
      - headers:
          x-feature-flag:
            exact: "new-ui"
      - headers:
          cookie:
            regex: "^(.*; )?beta-user=true(;.*)?$"

    metrics:
      - name: request-success-rate
        thresholdRange:
          min: 99
        interval: 1m
      - name: checkout-conversion
        templateRef:
          name: checkout-conversion
          namespace: my-app
        thresholdRange:
          min: 85
        interval: 5m
```

With this configuration:
- Users with the `x-feature-flag: new-ui` header or the `beta-user=true` cookie are routed to the canary
- All other users continue to see the primary version
- Flagger runs 10 analysis intervals and promotes if both metric checks pass throughout

This allows you to run statistical experiments — does the new UI increase conversion for opted-in users? — before exposing the change to everyone.

---

## Step 5 — Blue-green deployments

For services where gradual traffic shifting is unsafe — databases, message consumers, stateful processors — use blue-green deployment: switch 100% of traffic atomically after a validation period, with instant rollback available.

```yaml
# apps/base/my-worker/canary-bluegreen.yaml
apiVersion: flagger.app/v1beta1
kind: Canary
metadata:
  name: my-worker
  namespace: my-worker
spec:
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: my-worker
  provider: kubernetes    # no ingress/mesh required — uses Service selector switching

  service:
    port: 8080

  analysis:
    # Blue-green: no traffic stepping — validate, then switch
    interval: 30s
    iterations: 10         # validate for 10 × 30s = 5 minutes before switching
    threshold: 1

    metrics:
      - name: request-success-rate
        thresholdRange:
          min: 99
        interval: 30s

    webhooks:
      - name: integration-test
        type: pre-rollout
        url: http://flagger-loadtester.flagger-system/
        timeout: 60s
        metadata:
          type: bash
          cmd: "curl -sf http://my-worker-canary.my-worker/healthz"
```

With `provider: kubernetes`, Flagger switches the Service selector from primary pods to canary pods atomically — no gradual weight shift. Validation runs against the canary (which receives no production traffic) via the `my-worker-canary` Service. On promotion, the Service selector switches instantly.

---

## Step 6 — Alert providers

Configure Flagger to send notifications to Slack when a canary starts, succeeds, or fails:

```yaml
# apps/base/my-app/alert-provider.yaml
apiVersion: flagger.app/v1beta1
kind: AlertProvider
metadata:
  name: slack
  namespace: my-app
spec:
  type: slack
  channel: "#deployments"
  username: "Flagger"
  secretRef:
    name: slack-webhook-url    # Secret with key: address = https://hooks.slack.com/...
```

Reference the alert provider in the `Canary`:

```yaml
spec:
  analysis:
    alerts:
      - name: "deployment-alert"
        severity: info        # info | warn | error
        providerRef:
          name: slack
          namespace: my-app
```

Severity levels:
- `info` — notify on every canary event (start, weight advance, promotion)
- `warn` — notify on metric failures and rollbacks
- `error` — notify only on rollback (canary failed to promote)

### GitHub commit status

Update the GitHub commit status to reflect canary progress — useful when canary status needs to be visible in pull request checks:

```yaml
apiVersion: flagger.app/v1beta1
kind: AlertProvider
metadata:
  name: github
  namespace: my-app
spec:
  type: github
  secretRef:
    name: github-token    # Secret with key: token = ghp_...
```

```yaml
spec:
  analysis:
    alerts:
      - name: "github-status"
        severity: info
        providerRef:
          name: github
          namespace: my-app
```

Flagger sets the commit status on the SHA being deployed: `pending` during analysis, `success` on promotion, `failure` on rollback. The canary status becomes a visible GitHub check.

---

## Step 7 — Tuning analysis parameters for production SLOs

The right analysis parameters depend on your traffic volume and SLO targets. Misconfigured thresholds cause false positives (rollbacks on healthy releases) or false negatives (promoting broken releases).

### Parameter reference

| Parameter | Effect | Guidance |
|---|---|---|
| `interval` | How often metrics are queried | Use 1m for high-traffic services, 5m for low-traffic to accumulate meaningful data |
| `threshold` | Failed checks before rollback | Set to 5–10 to allow transient metric spikes without false rollbacks |
| `stepWeight` | Traffic increment per step | Smaller = safer, slower. Use 5–10% for critical services, 20% for internal tools |
| `maxWeight` | Maximum canary traffic % | 50% limits blast radius. Rarely need to go higher — at 50%, you have enough data |
| `iterations` | Steps before promotion (A/B or blue-green) | Set to cover at least 2–3× your application's request rate to ensure statistical significance |

### Low-traffic services

For services processing fewer than 100 requests per minute:

```yaml
analysis:
  interval: 5m           # longer interval — wait for enough requests
  threshold: 3           # more tolerance for metric noise
  stepWeight: 20         # fewer steps — each step needs enough data
  maxWeight: 60
  metrics:
    - name: request-success-rate
      thresholdRange:
        min: 95          # lower threshold — small N makes 99% hard to sustain
      interval: 5m
```

### High-traffic services

For services processing thousands of requests per minute:

```yaml
analysis:
  interval: 30s          # shorter — data accumulates quickly
  threshold: 10          # stricter — transient spikes should not trigger rollback
  stepWeight: 5
  maxWeight: 50
  metrics:
    - name: request-success-rate
      thresholdRange:
        min: 99.5        # tighter threshold — high volume makes this achievable
      interval: 30s
    - name: p99-latency
      thresholdRange:
        max: 150
      interval: 30s
```

---

## Step 8 — Canary status in the GitOps workflow

Flagger integrates with the GitOps workflow rather than replacing it:

```
Developer commits new image tag to Git
          │
          ▼
Flux reconciles — updates Deployment image
          │
          ▼
Flagger detects image change — initialises Canary
          │
          ▼
Analysis runs for N intervals (5–30 minutes typically)
          │
     ┌────┴────┐
     │         │
  Metrics    Metrics
  pass       fail
     │         │
     ▼         ▼
Flagger      Flagger
promotes     rolls back
     │         │
     ▼         ▼
Git is now   Git still has the
up to date   broken image tag
             ↑ requires a fix commit
```

After a rollback, Flagger sets the `Canary` status to `Failed`. The cluster runs the old stable image. The bad image tag remains in Git — which means if Flux reconciles again, Flagger immediately starts a new canary and will roll back again.

To clear this state, commit a corrected image tag to Git:

```bash
# Revert the bad image commit in your GitOps repository
git revert HEAD
git push origin main
# Flux applies the revert — Flagger sees no image change (reverted to previous) → no new canary
```

Or deploy a fixed version with a new tag:

```bash
# Update to the fixed image
# Edit apps/production/my-app/patch.yaml: image: ghcr.io/org/app:sha-fixed123
git commit -m "fix: deploy corrected image sha-fixed123"
git push origin main
# Flux applies — Flagger starts a new canary on the fixed image
```

---

## Step 9 — Multi-cluster progressive delivery

For multi-cluster setups, deploy the same `Canary` resource to each cluster via your GitOps repository. Each cluster runs an independent Flagger instance and independent canary analysis:

```
Staging cluster:   canary runs → metrics checked against staging Prometheus
                   → promoted after 5 steps
                   
Production cluster: canary runs → metrics checked against production Prometheus
                    → promoted independently
```

For ordered promotion — where staging must succeed before production begins — use ArgoCD `ApplicationSet` with `RollingSync` (covered in [ArgoCD Part 2](/tutorials/gitops/managing-multi-environment-deployments-argocd-applicationsets-part-2/)) or Flux `dependsOn` between cluster-scoped `Kustomization` resources.

There is no built-in cross-cluster promotion gate in Flagger. Cross-cluster ordering requires an external orchestration layer — typically a CI/CD webhook that monitors the staging canary status via the Flagger API and triggers the production deployment only after staging reports `Succeeded`.

---

## Step 10 — Full observability of a canary rollout

During a canary, query Prometheus directly to see what Flagger is seeing:

```bash
# Port-forward Prometheus
kubectl port-forward svc/prometheus-operated -n monitoring 9090

# In a browser or via curl:
# Request success rate for canary pods:
# sum(rate(http_requests_total{pod=~"my-app-[a-z0-9]+",status!~"5.."}[1m]))
# / sum(rate(http_requests_total{pod=~"my-app-[a-z0-9]+"}[1m])) * 100

# Check canary events in real time
kubectl get events -n my-app --field-selector reason=Synced --watch

# Check Flagger controller logs
kubectl logs -n flagger-system -l app.kubernetes.io/name=flagger -f

# Summarise all canary states across namespaces
kubectl get canaries -A
# NAMESPACE   NAME          STATUS      WEIGHT   FAILEDCHECKS   AGE
# my-app      my-app        Progressing 30       0              5m
# checkout    checkout-svc  Succeeded   0        0              2h
# payments    payment-api   Failed      0        3              1h
```

---

## What you have built

- Custom `MetricTemplate` resources with PromQL — error rate, p99 latency, and business conversion metrics
- A/B testing with `iterations` and `match` header/cookie routing — controlled cohort-based experiments
- Blue-green deployment with `provider: kubernetes` — atomic Service selector switching after validation
- `AlertProvider` resources for Slack notifications and GitHub commit statuses tied to canary lifecycle events
- Analysis parameter tuning guidance for low-traffic and high-traffic services against real SLO targets
- The GitOps-to-progressive-delivery pipeline: Git commit → Flux → Flagger canary → promote or rollback
- Rollback recovery: reverting Git or committing a fixed image tag to clear the `Failed` canary state
- Multi-cluster progressive delivery patterns and cross-cluster ordering via ArgoCD RollingSync
- Full observability: Prometheus queries, Kubernetes events, Flagger controller logs, and `kubectl get canaries`

With Flagger analysis, every new deployment is automatically protected against regressions in error rate, latency, and business metrics — with zero-touch rollback when something goes wrong. The combination of GitOps (Git as source of truth) and progressive delivery (traffic as a deployment gate) gives you both auditability and safety for production releases.
