---
title: "Implementing Canary Deployments with Flagger and Flux — Part 1"
date: 2026-09-02
description: "Install Flagger alongside Flux, configure a Canary resource to progressively shift traffic from a stable release to a new version, understand the analysis loop, and implement canary deployments with Istio, Nginx ingress, or Contour as the traffic provider."
cluster: "GitOps"
series: "Progressive Delivery"
part: 1
difficulty: "intermediate"
duration: "45 min"
tags: ["gitops", "flux", "flagger", "canary", "progressive-delivery", "kubernetes", "istio", "devops"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

By the end of this tutorial you will have Flagger installed alongside Flux, running canary deployments for an application: traffic shifts progressively from the stable version to the canary as Flagger validates each step against configurable metrics. A single failed metric check halts the rollout and triggers an automatic rollback. Part 2 extends this with custom metric analysis, A/B testing, blue-green deployments, and webhook integrations that gate promotion on external signals.

## Prerequisites

- A Kubernetes cluster with Flux bootstrapped (covered in [Flux Part 1](/tutorials/gitops/installing-bootstrapping-flux-kubernetes-cluster-part-1/))
- A service mesh or ingress controller — this tutorial covers Nginx ingress (simplest) and Istio (full traffic splitting). Install one before proceeding.
- `kubectl` and `flux` CLIs installed
- Prometheus running in the cluster (required for metric-based analysis)

---

## Step 1 — What progressive delivery adds to GitOps

Standard GitOps replaces the old version immediately when the new image is detected. Kubernetes performs a rolling update — but provides no gate based on actual application behaviour. If the new version has elevated error rates or latency, the rolling update completes before the problem is detected.

Progressive delivery interposes a controlled traffic-shifting layer between the GitOps commit and the full rollout:

```
Git commit (new image tag)
        │
        ▼
Flagger detects change → creates canary Deployment
        │
        ▼
Traffic: 95% primary (old) / 5% canary (new)
        │  [wait analysis interval]
        ▼
Metrics healthy? → 90% / 10% → 80% / 20% → ... → 0% / 100%
        │
        ▼
Flagger promotes canary to primary, deletes canary Deployment
```

If metrics fail at any step, Flagger rolls back to 100% primary traffic and marks the release as failed. No manual intervention required.

---

## Step 2 — Install Flagger with Flux

Flagger is a Kubernetes operator installed via Helm. Add it to your GitOps repository so Flux manages it:

```yaml
# infrastructure/base/flagger/helmrepository.yaml
apiVersion: source.toolkit.fluxcd.io/v1
kind: HelmRepository
metadata:
  name: flagger
  namespace: flux-system
spec:
  interval: 12h
  url: https://flagger.app
```

```yaml
# infrastructure/base/flagger/helmrelease.yaml
apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: flagger
  namespace: flagger-system
spec:
  interval: 30m
  chart:
    spec:
      chart: flagger
      version: ">=1.37.0 <2.0.0"
      sourceRef:
        kind: HelmRepository
        name: flagger
        namespace: flux-system
      interval: 12h
  values:
    meshProvider: nginx          # set to: nginx, istio, contour, linkerd, or appmesh
    metricsServer: http://prometheus-operated.monitoring:9090
    slack:
      url: ""                    # configure in a Secret, not inline
      channel: deployments
      user: flagger
```

```yaml
# infrastructure/base/flagger/namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: flagger-system
```

```yaml
# infrastructure/base/flagger/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - namespace.yaml
  - helmrepository.yaml
  - helmrelease.yaml
```

Reference from `infrastructure/production/kustomization.yaml`:

```yaml
resources:
  - ../base/flagger
  - ../base/cert-manager
  - ../base/ingress-nginx
```

Flux reconciles this and installs Flagger. Verify:

```bash
kubectl get pods -n flagger-system
# flagger-xxx   1/1   Running

kubectl get crds | grep flagger
# canaries.flagger.app
# metrictemplates.flagger.app
# alertproviders.flagger.app
```

---

## Step 3 — How Flagger manages Deployments

When Flagger takes ownership of a Deployment, it creates two additional Deployments automatically:

```
Original Deployment: my-app         → Flagger renames to: my-app-primary
                                       Flagger creates:   my-app-canary (new version)
                                       Flagger creates:   my-app (target for traffic routing)
```

You should **not** scale or modify `my-app-primary` directly — Flagger manages it. Your GitOps manifests continue to reference `my-app` by name. Flagger intercepts the image change on `my-app`, creates the canary Deployment, and runs the analysis.

The original Service is also duplicated:

| Service | Routes to | Purpose |
|---|---|---|
| `my-app` | Primary pods | Stable traffic (all traffic before canary) |
| `my-app-canary` | Canary pods | Canary traffic during rollout |
| `my-app-primary` | Primary pods | Internal reference, not exposed |

---

## Step 4 — Configure a Canary resource (Nginx ingress)

A `Canary` resource tells Flagger how to analyse and promote a deployment:

```yaml
# apps/base/my-app/canary.yaml
apiVersion: flagger.app/v1beta1
kind: Canary
metadata:
  name: my-app
  namespace: my-app
spec:
  # Target Deployment
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: my-app

  # Traffic provider
  provider: nginx

  # Ingress reference (Nginx traffic splitting)
  ingressRef:
    apiVersion: networking.k8s.io/v1
    kind: Ingress
    name: my-app

  # Service configuration
  service:
    port: 80
    targetPort: 8080

  # Canary analysis configuration
  analysis:
    # How long to wait between traffic weight increments
    interval: 1m

    # How many analysis intervals before promotion
    threshold: 5

    # Maximum traffic weight to shift to canary
    maxWeight: 50

    # Traffic increment per step
    stepWeight: 10

    # Metrics that must pass at each step
    metrics:
      - name: request-success-rate
        # Minimum success rate percentage (non-5xx responses)
        thresholdRange:
          min: 99
        interval: 1m
      - name: request-duration
        # Maximum average request duration in milliseconds
        thresholdRange:
          max: 500
        interval: 1m
```

With this configuration:
- Traffic shifts 10% at a time: 10% → 20% → 30% → 40% → 50%
- Each step waits 1 minute and checks metrics
- After 5 successful steps, Flagger promotes the canary to primary (100%)
- If success rate drops below 99% or p99 latency exceeds 500ms at any step, Flagger rolls back

---

## Step 5 — Configure a Canary resource (Istio)

With Istio, Flagger uses `VirtualService` and `DestinationRule` for precise traffic splitting with header-based routing:

```yaml
# apps/base/my-app/canary-istio.yaml
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
    trafficPolicy:
      tls:
        mode: DISABLE

  analysis:
    interval: 1m
    threshold: 10
    maxWeight: 50
    stepWeight: 5
    metrics:
      - name: request-success-rate
        thresholdRange:
          min: 99
        interval: 1m
      - name: request-duration
        thresholdRange:
          max: 500
        interval: 1m
    # Route specific headers to canary (for internal testing)
    match:
      - headers:
          x-canary:
            exact: "true"
```

The `match` block routes requests with the `x-canary: true` header directly to the canary — useful for QA teams to test the new version before it receives production traffic percentage.

---

## Step 6 — Trigger a canary deployment

Flagger watches the Deployment's image field. To trigger a canary:

1. Update the image tag in your GitOps repository
2. Commit and push to main
3. Flux detects the change and updates the Deployment
4. Flagger detects the image change and initialises the canary

```bash
# Watch Flagger events during a canary rollout
kubectl get canary my-app -n my-app --watch

# NAME     STATUS        WEIGHT   FAILEDCHECKS
# my-app   Initializing  0        0
# my-app   Progressing   10       0
# my-app   Progressing   20       0
# my-app   Progressing   30       0
# my-app   Progressing   40       0
# my-app   Progressing   50       0
# my-app   Promoting     0        0
# my-app   Finalizing    0        0
# my-app   Succeeded     0        0
```

Inspect canary events:

```bash
kubectl describe canary my-app -n my-app
# Events:
#   Normal   Synced  Starting canary analysis for my-app.my-app
#   Normal   Synced  Advance my-app.my-app canary weight 10
#   Normal   Synced  Advance my-app.my-app canary weight 20
#   Normal   Synced  Advance my-app.my-app canary weight 30
#   Normal   Synced  Routing all traffic to primary — promoting my-app.my-app
#   Normal   Synced  Promotion completed! Scaling down my-app.my-app.canary
```

---

## Step 7 — Observe traffic shifting in real time

With Istio, inspect the `VirtualService` that Flagger manages:

```bash
kubectl get virtualservice my-app -n my-app -o yaml
# spec:
#   http:
#     - match:
#         - headers:
#             x-canary:
#               exact: "true"
#       route:
#         - destination:
#             host: my-app-canary
#             port:
#               number: 80
#     - route:
#         - destination:
#             host: my-app-primary
#             port:
#               number: 80
#           weight: 80     ← currently 80% primary
#         - destination:
#             host: my-app-canary
#             port:
#               number: 80
#           weight: 20     ← currently 20% canary
```

Flagger updates the `weight` values automatically at each analysis interval. You never edit the `VirtualService` directly.

For Nginx, Flagger sets the `nginx.ingress.kubernetes.io/canary-weight` annotation on the canary ingress:

```bash
kubectl get ingress -n my-app
# NAME            CLASS   HOSTS              ADDRESS
# my-app          nginx   my-app.example.com  <lb-ip>
# my-app-canary   nginx   my-app.example.com  <lb-ip>  ← weight: 20%
```

---

## Step 8 — Manual gating with webhooks

Add a pre-promotion webhook that must return 200 before Flagger promotes to 100%:

```yaml
spec:
  analysis:
    webhooks:
      - name: load-test
        type: rollout
        url: http://flagger-loadtester.flagger-system/
        timeout: 5s
        metadata:
          cmd: "hey -z 1m -q 10 -c 2 http://my-app-canary.my-app/"

      - name: smoke-test
        type: pre-rollout
        url: http://flagger-loadtester.flagger-system/
        timeout: 15s
        metadata:
          type: bash
          cmd: "curl -sd 'test' http://my-app-canary.my-app/health | grep -q 'ok'"

      - name: acceptance-test
        type: confirm-rollout
        url: https://my-ci-system/api/canary-gate
        timeout: 1h    # wait up to 1 hour for a human to approve
```

Webhook types:
- `pre-rollout` — runs before the first traffic shift. Fails abort immediately.
- `rollout` — runs at each analysis interval (used for load generation).
- `confirm-rollout` — blocks promotion until the external URL returns 200. Used for human approval gates.
- `confirm-promotion` — runs before the final 100% promotion.
- `post-rollout` — runs after promotion completes (for notifications or cleanup).

---

## Step 9 — Install the Flagger load tester

The Flagger load tester is an optional helper that runs load generation and bash commands in response to webhook calls:

```yaml
# infrastructure/base/flagger/loadtester.yaml
apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: flagger-loadtester
  namespace: flagger-system
spec:
  interval: 30m
  chart:
    spec:
      chart: loadtester
      sourceRef:
        kind: HelmRepository
        name: flagger
        namespace: flux-system
```

The load tester is useful during analysis: without synthetic load, a canary serving 10% of real traffic may have too few requests for metrics to be statistically meaningful — especially on low-traffic staging clusters.

---

## Step 10 — Verify Flagger is protecting against bad releases

Simulate a bad release by deploying an image that returns 5xx errors:

```bash
# Deploy a deliberately broken image via your GitOps repo
# (update the image tag in Git to a version that returns 500s)

# Watch the canary fail
kubectl get canary my-app -n my-app --watch
# NAME     STATUS      WEIGHT   FAILEDCHECKS
# my-app   Progressing  10      0
# my-app   Progressing  10      1   ← metric failure
# my-app   Progressing  10      2
# my-app   Progressing  10      3
# my-app   Failed        0      3   ← rolled back after threshold exceeded
```

Inspect the failure reason:

```bash
kubectl describe canary my-app -n my-app | grep -A5 "Warning"
# Warning  Synced  Halt advancement my-app.my-app success rate 94.2% < 99%
# Warning  Synced  Halt advancement my-app.my-app success rate 93.8% < 99%
# Warning  Synced  Halt advancement my-app.my-app success rate 91.1% < 99%
# Warning  Synced  Rolling back my-app.my-app failed checks threshold reached 3
```

After rollback, 100% of traffic returns to the primary (old stable version). The broken image never received more than 10% of traffic.

---

## What you have built

- The progressive delivery mental model: traffic shift → metric gate → promote or rollback
- Flagger installed via Flux `HelmRelease` with Prometheus metric server configured
- How Flagger transforms a single Deployment into primary + canary + routing Deployments
- `Canary` resource configuration with Nginx ingress provider: step weight, max weight, interval, threshold
- `Canary` resource configuration with Istio: `VirtualService` traffic splitting and header-based canary routing
- Triggering a canary by updating an image tag in Git — the complete GitOps-to-progressive-delivery pipeline
- Real-time traffic weight observation via `VirtualService` YAML and ingress annotations
- Pre-rollout, rollout, and confirm-rollout webhook types for load generation and human approval gates
- Flagger load tester for synthetic traffic during analysis on low-traffic clusters
- Verified rollback behaviour: metric failure halts advancement and restores 100% primary traffic

In [Part 2](/tutorials/gitops/automating-rollbacks-metric-failures-flagger-analysis-part-2/) you will build custom metric templates using PromQL, configure A/B testing with header-based routing, implement blue-green deployments for zero-traffic switching, and add alert providers so Flagger reports canary status to Slack and GitHub.
