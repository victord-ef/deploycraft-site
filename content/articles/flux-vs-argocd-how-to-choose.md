---
title: "Flux vs ArgoCD — How to Choose for Your Team"
date: 2026-08-07
author: "Victor D"
description: "Both Flux and ArgoCD are CNCF-graduated GitOps tools that do the same fundamental job. The choice between them is not about capability — it is about how your team works and what you want to own."
tags: ["gitops", "flux", "argocd", "kubernetes", "devops", "platform-engineering", "comparisons"]
categories: ["article"]
draft: false
toc: true
---

Both Flux and ArgoCD are CNCF-graduated projects. Both implement GitOps correctly. Both have large production user bases and active maintainer communities. If you are waiting for one of them to be objectively better, you will wait a long time.

The honest framing is this: Flux and ArgoCD make different bets about what GitOps tooling should look like, and those bets suit different teams. This article lays out the real differences so you can match the tool to how your team actually works — not to a feature comparison matrix.

---

## What they both do

Before the differences, the common ground: both tools watch a Git repository (or OCI registry), compare the desired state stored there against the live state of your cluster, and reconcile any drift. Both support Kustomize and Helm natively. Both are pull-based — the agent runs inside the cluster and pulls from Git, so you never need to open inbound firewall rules or hand CI credentials to a deployment pipeline.

The divergence is in philosophy, architecture, and the operational model they impose.

---

## Architecture: modular vs integrated

**Flux** is built as a set of independent controllers, each responsible for a specific concern:

- `source-controller` — watches Git repositories, Helm repositories, OCI registries, and S3 buckets; fetches and caches source artifacts
- `kustomize-controller` — reconciles `Kustomization` objects; applies manifests to the cluster
- `helm-controller` — reconciles `HelmRelease` objects; manages Helm chart lifecycle
- `notification-controller` — handles inbound webhooks and outbound alerts
- `image-reflector-controller` / `image-automation-controller` — scans registries and automates image tag updates

Each controller is a separate Kubernetes deployment. You can install only what you need. If you do not use Helm, you do not run `helm-controller`. If you do not need image automation, you leave those two controllers out entirely.

**ArgoCD** is architecturally more integrated:

- `argocd-application-controller` — the reconciliation engine; watches `Application` objects and syncs state
- `argocd-repo-server` — clones repositories and renders manifests (Kustomize, Helm, plain YAML, Jsonnet)
- `argocd-server` — the API server and the web UI
- `argocd-redis` — a caching layer used by the above components

The UI is not an optional add-on — it is baked into the architecture. The `argocd-server` component serves both the REST API and the web interface. This is a meaningful design choice: ArgoCD treats visibility as a first-class feature, not an afterthought.

---

## The UI question

This is usually the first practical fork in the road.

ArgoCD's web UI is genuinely good. It shows a live resource tree for every Application — Deployments, ReplicaSets, Pods, Services, ConfigMaps all visualised with their sync status and health. You can see a diff between what Git says and what the cluster has, trigger a manual sync, view recent sync history, and roll back to a previous state — all without touching the terminal. For teams where developers (not just platform engineers) need visibility into deployment status, this matters.

```
ArgoCD UI — Application view
┌─────────────────────────────────────────────┐
│ api-server          ● Synced  ✓ Healthy      │
│  └─ Deployment/api-server  ✓ Healthy         │
│      └─ ReplicaSet/api-server-7d9f8b   ✓    │
│          ├─ Pod/api-server-7d9f8b-xk2p  ✓   │
│          └─ Pod/api-server-7d9f8b-m3n7  ✓   │
│  └─ Service/api-server     ✓ Healthy         │
│  └─ ConfigMap/api-config   ✓ Synced          │
└─────────────────────────────────────────────┘
```

Flux has no built-in UI. There is Weave GitOps (a separate open-source project from Weaveworks) which provides one, but it is not part of Flux itself. If your team is comfortable living in `flux` CLI and Kubernetes events, the absence of a UI is not a problem. If you regularly need to answer "what is deployed where and is it healthy?" without touching `kubectl`, the Flux answer requires more work.

---

## Defining deployments: Kustomization vs Application

The core unit of deployment looks different in each tool.

**Flux — Kustomization CR:**

```yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: api-server
  namespace: flux-system
spec:
  interval: 5m
  sourceRef:
    kind: GitRepository
    name: platform-repo
  path: ./clusters/production/api-server
  prune: true
  healthChecks:
  - apiVersion: apps/v1
    kind: Deployment
    name: api-server
    namespace: production
```

The `GitRepository` source object is defined separately and referenced here. This separation is intentional — multiple `Kustomization` objects can share one source, and sources can be updated independently.

**ArgoCD — Application CR:**

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: api-server
  namespace: argocd
spec:
  project: production
  source:
    repoURL: https://github.com/your-org/platform-repo
    targetRevision: HEAD
    path: clusters/production/api-server
  destination:
    server: https://kubernetes.default.svc
    namespace: production
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
```

The source is inline. The `Application` is more self-contained, which makes it easier to read in isolation but means that configuration is duplicated when many applications share the same repository.

---

## Helm support

Both tools support Helm charts as first-class citizens, but with different models.

**Flux** uses a `HelmRelease` CR that separates the chart source from the release configuration:

```yaml
apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: cert-manager
  namespace: flux-system
spec:
  interval: 1h
  chart:
    spec:
      chart: cert-manager
      version: "v1.15.*"
      sourceRef:
        kind: HelmRepository
        name: jetstack
  targetNamespace: cert-manager
  values:
    installCRDs: true
```

Flux renders Helm charts server-side using its own Helm library. Values can be sourced from ConfigMaps or Secrets in the cluster, not just inline YAML — useful for injecting environment-specific values without duplicating charts.

**ArgoCD** treats a Helm chart as a source type directly in the `Application`:

```yaml
spec:
  source:
    repoURL: https://charts.jetstack.io
    chart: cert-manager
    targetRevision: v1.15.0
    helm:
      values: |
        installCRDs: true
```

ArgoCD renders Helm templates and then applies the rendered YAML — it treats Helm as a templating engine, not as a package manager. This means ArgoCD does not use `helm upgrade` under the hood; it manages the lifecycle directly. This has implications: `helm list` will not show ArgoCD-managed releases, and Helm hooks behave differently.

---

## Multi-tenancy and scale

If you are managing a single cluster for a single team, both tools handle this easily. The differences emerge at scale — multiple teams, multiple clusters, or both.

**ArgoCD** solves multi-tenancy with `AppProject`, which defines which repositories, clusters, and namespaces a set of `Application` objects can target. An `ApplicationSet` generates Applications dynamically from a generator (a list of clusters, a Git directory structure, a pull request list):

```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: cluster-apps
  namespace: argocd
spec:
  generators:
  - clusters: {}   # generates one Application per registered cluster
  template:
    spec:
      project: default
      source:
        repoURL: https://github.com/your-org/platform-repo
        path: clusters/{{name}}
        targetRevision: HEAD
      destination:
        server: '{{server}}'
        namespace: production
```

This is powerful for hub-and-spoke models where a central ArgoCD instance manages many downstream clusters.

**Flux** handles multi-tenancy differently — through namespace isolation and `Tenant` objects (introduced in Flux v2). Each team's Flux resources live in their own namespace and their `GitRepository` sources scope their access:

```yaml
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: team-alpha-repo
  namespace: team-alpha     # scoped to this namespace
spec:
  interval: 1m
  url: https://github.com/your-org/team-alpha
  ref:
    branch: main
```

Flux's model is arguably more Kubernetes-native — isolation is enforced by RBAC and namespace boundaries rather than a custom project concept. But ArgoCD's `ApplicationSet` is more ergonomic when you need to manage 50 clusters and want to express "deploy this to all of them" in one object.

---

## Sync ordering and deployment hooks

ArgoCD has a first-class concept of **sync waves** and **resource hooks** — annotations that control the order in which resources are applied and allow you to run jobs before or after a sync:

```yaml
# Run database migration before the main deployment syncs
apiVersion: batch/v1
kind: Job
metadata:
  name: db-migrate
  annotations:
    argocd.argoproj.io/hook: PreSync
    argocd.argoproj.io/hook-delete-policy: HookSucceeded
spec:
  template:
    spec:
      containers:
      - name: migrate
        image: your-org/migrator:latest
```

```yaml
# Control ordering within a sync with waves
metadata:
  annotations:
    argocd.argoproj.io/sync-wave: "2"   # applied after wave 1
```

Flux does not have an equivalent native primitive. You can approximate sync ordering by chaining `Kustomization` objects with `dependsOn`, but it is more verbose and less expressive for complex multi-step deployments.

If your deployment process involves database migrations, schema changes, or any ordered sequence of steps that must complete before the next begins, ArgoCD's hook model is significantly easier to work with.

---

## Progressive delivery

ArgoCD integrates natively with **Argo Rollouts** — a separate project that adds canary deployments, blue-green deployments, and automated analysis (checking metrics before promoting a rollout). If you are already using or planning to use Argo Rollouts, ArgoCD is the natural companion: it understands `Rollout` resources natively and shows their state in the UI.

Flux does not have a built-in equivalent, but integrates well with **Flagger** — a CNCF project that works with any service mesh or ingress controller to automate canary analysis. Flagger is more infrastructure-agnostic than Argo Rollouts (it supports Istio, Linkerd, Nginx, Traefik, and others), but it is a separate tool to install and configure.

---

## Image update automation

Both tools can automate container image tag updates — watching a registry for new tags and committing the updated image reference back to Git.

Flux builds this in via `image-reflector-controller` and `image-automation-controller`. You define an `ImagePolicy` (e.g., semver range `>=1.0.0 <2.0.0`) and an `ImageUpdateAutomation` object that commits the updated tag to your repository on your behalf.

ArgoCD delegates this to a separate project — **ArgoCD Image Updater** — which you install and configure independently. It works well but adds an operational dependency that Flux handles natively.

---

## How to choose

**Choose ArgoCD if:**

- You need a UI. If developers, product managers, or stakeholders need to see deployment status without `kubectl`, ArgoCD's UI is genuinely better than anything Flux offers out of the box.
- You use sync waves and hooks. Ordered, multi-step deployments with pre- and post-sync jobs are much easier to express in ArgoCD.
- You are already in the Argo ecosystem. If you run Argo Workflows or plan to use Argo Rollouts, ArgoCD is the obvious fit — they share CRDs, CLI tooling, and operational patterns.
- You manage many clusters from a central control plane. `ApplicationSet` with the cluster generator is purpose-built for hub-and-spoke GitOps.

**Choose Flux if:**

- Your team is platform-engineering focused and CLI-native. Flux's modular controller model feels natural to teams that think in terms of Kubernetes controllers and CRDs.
- You want to minimise operational footprint. No UI means fewer components to run, upgrade, and secure. In a small team where everyone has `kubectl`, the absence of a web server is a simplification.
- You need image automation built in. Flux's image update automation is native; ArgoCD's is a separate installation.
- You want fine-grained modularity. The ability to run only the controllers you need makes Flux easier to audit and harden — there is less surface area.

**Either works well for:**

- Single-cluster deployments for a single team
- Helm chart management
- Multi-tenancy (the models differ but both are capable)
- OCI artifact sources
- Notifications and alerting

---

## The real question

The feature lists are close enough that the deciding factor is almost always one of three things:

1. **Do your developers need a UI?** → ArgoCD
2. **Are you already running Argo Rollouts or Argo Workflows?** → ArgoCD
3. **Does your team prefer Kubernetes-native controllers and CLI over a web interface?** → Flux

Neither choice will hold you back. Both communities are active, both tools are production-proven at scale, and both are maintained under the CNCF umbrella with long-term sustainability guarantees. Pick the one that matches how your team actually works today — not the one with the longer feature list.

---

## Related reading

- [Flux documentation](https://fluxcd.io/flux/)
- [ArgoCD documentation](https://argo-cd.readthedocs.io/en/stable/)
- [Argo Rollouts — progressive delivery](https://argoproj.github.io/rollouts/)
- [Flagger — progressive delivery for Flux](https://flagger.app/)
- Why GitOps is an operating model, not a deployment tool → **Articles**
- ArgoCD vs Flux — it's not about features, it's about your team → **Articles** (opinion piece)
