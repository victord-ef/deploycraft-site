---
title: "Automating Deployments with Flux Kustomization and HelmRelease — Part 2"
date: 2026-09-02
description: "Use Flux Kustomization and HelmRelease resources to deploy applications and infrastructure: environment-specific overlays, Helm chart version pinning, health checks, automated upgrades, rollback on failure, and dependency ordering across the stack."
cluster: "GitOps"
series: "Flux"
part: 2
difficulty: "intermediate"
duration: "45 min"
tags: ["gitops", "flux", "kubernetes", "kustomize", "helm", "devops", "delivery"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

In [Part 1](/tutorials/gitops/installing-bootstrapping-flux-kubernetes-cluster-part-1/) you bootstrapped Flux and connected it to a Git repository. In Part 2 you will use Flux `Kustomization` and `HelmRelease` resources to automate real deployments: application workloads with Kustomize overlays, infrastructure components with Helm charts, dependency ordering between layers, health-gate checks before promotion, and automatic rollback on upgrade failure.

## Prerequisites

- Completed [Part 1](/tutorials/gitops/installing-bootstrapping-flux-kubernetes-cluster-part-1/) — Flux bootstrapped in a cluster with `flux check` passing
- The GitOps repository from Part 1 cloned locally
- Familiarity with Kustomize base/overlay layout (covered in [GitOps Foundations Part 2](/tutorials/gitops/setting-up-gitops-repository-structure-kubernetes-part-2/))

---

## Step 1 — The Flux source model

Before deploying anything, understand how Flux separates *source fetching* from *deployment*:

```
GitRepository / HelmRepository / OCIRepository
      │  (source-controller fetches and caches)
      ▼
Kustomization / HelmRelease
      │  (kustomize-controller / helm-controller deploys)
      ▼
Kubernetes cluster
```

A `GitRepository` or `HelmRepository` is declared once and shared by multiple `Kustomization` or `HelmRelease` resources. This separation means changing the polling interval or authentication for a source affects all consumers without touching deployment configuration.

The `flux-system` bootstrap already created a `GitRepository` named `flux-system` pointing at your repository. All subsequent `Kustomization` resources can reference this same source.

---

## Step 2 — Deploy an application with Kustomization

Add a `Kustomization` resource that deploys your application from a subdirectory in the GitOps repository:

```yaml
# clusters/my-cluster/apps.yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: apps
  namespace: flux-system
spec:
  interval: 5m
  sourceRef:
    kind: GitRepository
    name: flux-system          # reuse the bootstrap GitRepository
  path: ./apps/production      # path within the repo
  prune: true                  # delete resources removed from Git
  wait: true                   # wait for all resources to become ready
  timeout: 3m
  healthChecks:
    - apiVersion: apps/v1
      kind: Deployment
      name: my-app
      namespace: my-app
```

Commit this to `clusters/my-cluster/apps.yaml`. Flux picks it up within the next poll interval (default 1 minute) and starts reconciling `apps/production/`.

### Force an immediate reconciliation

```bash
flux reconcile kustomization apps --with-source
```

`--with-source` forces the `GitRepository` to re-fetch from Git before reconciling — useful after a commit when you don't want to wait for the poll interval.

---

## Step 3 — Kustomization dependencies with dependsOn

Declare that applications should only deploy after infrastructure is healthy:

```yaml
# clusters/my-cluster/apps.yaml (updated)
spec:
  dependsOn:
    - name: infrastructure    # wait for infrastructure Kustomization to be Ready
```

```yaml
# clusters/my-cluster/infrastructure.yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: infrastructure
  namespace: flux-system
spec:
  interval: 10m
  sourceRef:
    kind: GitRepository
    name: flux-system
  path: ./infrastructure/production
  prune: true
  wait: true
  timeout: 5m
```

The dependency chain ensures:

1. Flux reconciles `infrastructure` first
2. Only when all infrastructure resources report `Ready` does Flux begin reconciling `apps`

This prevents application pods from crashing because cert-manager or the ingress controller is not yet running.

---

## Step 4 — Kustomize patches via the Kustomization spec

Beyond the standard `kustomization.yaml` in the source directory, Flux `Kustomization` resources support inline patches and variable substitution directly in the spec:

### Inline patches

```yaml
# clusters/my-cluster/apps.yaml
spec:
  path: ./apps/base/my-app
  patches:
    - patch: |
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: my-app
          namespace: my-app
        spec:
          replicas: 3
    - patch: |
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: my-app
          namespace: my-app
        spec:
          template:
            spec:
              containers:
                - name: my-app
                  env:
                    - name: ENVIRONMENT
                      value: production
```

This avoids duplicating `kustomization.yaml` files in the repository — environment-specific patches live in the cluster directory instead of the shared application base.

### Variable substitution

Reference cluster-specific values stored in ConfigMaps or Secrets:

```yaml
# clusters/my-cluster/cluster-vars.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: cluster-vars
  namespace: flux-system
data:
  cluster_name: "production"
  cluster_region: "eu-west-1"
  ingress_class: "nginx"
```

```yaml
# clusters/my-cluster/apps.yaml
spec:
  postBuild:
    substituteFrom:
      - kind: ConfigMap
        name: cluster-vars
```

In your manifests, use `${cluster_name}` syntax — Flux substitutes values after Kustomize rendering but before applying to the cluster:

```yaml
# apps/base/my-app/ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app
  annotations:
    kubernetes.io/ingress.class: ${ingress_class}
spec:
  rules:
    - host: my-app.${cluster_region}.example.com
```

---

## Step 5 — Deploy infrastructure with HelmRelease

Flux `HelmRelease` manages the full lifecycle of a Helm chart — install, upgrade, rollback, and uninstall — declaratively from Git.

### Declare the Helm repository

```yaml
# infrastructure/base/helm-repos/ingress-nginx.yaml
apiVersion: source.toolkit.fluxcd.io/v1
kind: HelmRepository
metadata:
  name: ingress-nginx
  namespace: flux-system
spec:
  interval: 12h
  url: https://kubernetes.github.io/ingress-nginx
```

Commit this to your repository. The source-controller polls the Helm repository index every 12 hours and caches it in-cluster.

### Create a HelmRelease

```yaml
# infrastructure/base/ingress-nginx/helmrelease.yaml
apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: ingress-nginx
  namespace: ingress-nginx
spec:
  interval: 30m
  chart:
    spec:
      chart: ingress-nginx
      version: ">=4.10.0 <5.0.0"    # semver constraint — auto-upgrades within range
      sourceRef:
        kind: HelmRepository
        name: ingress-nginx
        namespace: flux-system
      interval: 12h                  # how often to check for new chart versions
  values:
    controller:
      replicaCount: 2
      service:
        type: LoadBalancer
      metrics:
        enabled: true
        serviceMonitor:
          enabled: true
  install:
    remediation:
      retries: 3
  upgrade:
    cleanupOnFail: true
    remediation:
      retries: 3
      strategy: rollback
```

Key fields:
- `version: ">=4.10.0 <5.0.0"` — Flux upgrades to any patch/minor release in this range automatically
- `remediation.strategy: rollback` — on upgrade failure, Flux rolls back to the last successful release
- `cleanupOnFail: true` — removes partially-applied resources on upgrade failure to leave a clean state

---

## Step 6 — Override Helm values per environment

The base `HelmRelease` defines shared values. Environment-specific overrides use Kustomize patches:

```yaml
# infrastructure/production/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - ../base/ingress-nginx
patches:
  - patch: |
      - op: replace
        path: /spec/values/controller/replicaCount
        value: 3
      - op: add
        path: /spec/values/controller/autoscaling
        value:
          enabled: true
          minReplicas: 3
          maxReplicas: 10
    target:
      kind: HelmRelease
      name: ingress-nginx
```

Staging uses the base values (2 replicas, no autoscaling). Production patches in 3 replicas and horizontal pod autoscaling.

### Values from Secrets

For values that must not be in Git (database passwords, API keys), reference a Kubernetes Secret:

```yaml
spec:
  valuesFrom:
    - kind: Secret
      name: my-app-helm-values
      valuesKey: values.yaml    # key within the Secret containing YAML values
      optional: false
```

Create the secret out-of-band (via External Secrets Operator or sealed-secrets) and reference it in the `HelmRelease`. Flux merges the values from the Secret with the inline `values:` block.

---

## Step 7 — Health checks and readiness gates

Flux can verify that deployed resources become healthy before marking a reconciliation as successful:

```yaml
spec:
  healthChecks:
    - apiVersion: apps/v1
      kind: Deployment
      name: ingress-nginx-controller
      namespace: ingress-nginx
    - apiVersion: apps/v1
      kind: Deployment
      name: cert-manager
      namespace: cert-manager
  timeout: 5m    # fail if resources are not healthy within this window
```

With `wait: true` on a `Kustomization`, Flux automatically health-checks all applied resources without explicit `healthChecks` entries — but explicit entries let you target only the critical resources.

Health check status is surfaced in the `Kustomization` or `HelmRelease` status:

```bash
kubectl get kustomization infrastructure -n flux-system
# NAME             AGE   READY   STATUS
# infrastructure   2d    True    Applied revision: main@sha1:abc1234

kubectl get helmrelease ingress-nginx -n ingress-nginx
# NAME            AGE    READY   STATUS
# ingress-nginx   2d     True    Helm upgrade succeeded for release ingress-nginx/ingress-nginx.v5
```

---

## Step 8 — Monitor reconciliation with the Flux CLI

```bash
# List all Kustomization resources and their status
flux get kustomizations

# List all HelmRelease resources
flux get helmreleases -A    # -A = all namespaces

# Watch a specific Kustomization reconcile in real time
flux get kustomization apps --watch

# Get events for a HelmRelease
flux events --for HelmRelease/ingress-nginx -n ingress-nginx

# Describe a Kustomization (shows conditions, errors, last applied revision)
kubectl describe kustomization apps -n flux-system

# View kustomize-controller logs (for reconciliation errors)
kubectl logs -n flux-system -l app=kustomize-controller --tail=50

# View helm-controller logs
kubectl logs -n flux-system -l app=helm-controller --tail=50
```

---

## Step 9 — Rollback a HelmRelease

When a Helm upgrade fails, Flux rolls back automatically if `remediation.strategy: rollback` is set. To roll back manually:

```bash
# Suspend the HelmRelease to prevent Flux re-applying the broken chart version
flux suspend helmrelease ingress-nginx -n ingress-nginx

# Roll back using Helm directly
helm rollback ingress-nginx -n ingress-nginx

# Inspect the Helm history
helm history ingress-nginx -n ingress-nginx

# Fix the values in Git, commit and push, then resume
flux resume helmrelease ingress-nginx -n ingress-nginx
```

For application `Kustomization` rollback, revert the commit in Git and push:

```bash
git revert HEAD
git push origin main
# Flux detects the new commit and applies the reverted manifest
```

The GitOps rollback is a Git operation — not a cluster operation. This is the operational advantage: rollback state is fully captured in Git history.

---

## Step 10 — Full end-to-end reconciliation trace

After committing a change to `apps/production/my-app/deployment.yaml`:

```bash
# 1. Force the GitRepository to re-fetch (skip poll interval)
flux reconcile source git flux-system

# 2. Force the infrastructure Kustomization to reconcile
flux reconcile kustomization infrastructure --with-source

# 3. Force the apps Kustomization to reconcile
flux reconcile kustomization apps --with-source

# 4. Watch all kustomizations converge
flux get kustomizations --watch

# 5. Verify the Deployment rolled out
kubectl rollout status deployment/my-app -n my-app

# 6. Confirm the running image matches what is in Git
kubectl get pods -n my-app -o jsonpath='{.items[0].spec.containers[0].image}'
```

Under normal operations, none of these manual steps are required — Flux polls Git every 1–5 minutes and applies changes automatically. The `reconcile` commands are for development iteration and incident response.

---

## What you have built

- A clear Flux source model: `GitRepository`/`HelmRepository` → `Kustomization`/`HelmRelease`
- `Kustomization` resources in the cluster directory wiring application and infrastructure paths with `dependsOn` ordering
- Inline patches and `postBuild.substituteFrom` for environment-specific overrides without duplicating Kustomize overlays
- `HelmRelease` resources with semver chart version ranges, automatic upgrade within range, and rollback remediation on failure
- Per-environment Helm value overrides via Kustomize patches, plus out-of-band secret values via `valuesFrom`
- Health checks and `wait: true` as readiness gates before marking reconciliation successful
- Flux CLI monitoring: `flux get`, `flux events`, controller logs, and `kubectl describe`
- Rollback patterns — automatic via `remediation.strategy`, manual via Helm CLI or Git revert

With Flux fully wired, any change merged to the main branch of your GitOps repository is automatically applied to the cluster within minutes. The cluster is self-healing, auditable, and recoverable from Git alone.
