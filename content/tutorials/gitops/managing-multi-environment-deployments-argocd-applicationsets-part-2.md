---
title: "Managing Multi-Environment Deployments with ArgoCD ApplicationSets — Part 2"
date: 2026-09-02
description: "Use ArgoCD ApplicationSet generators to deploy one application across multiple environments and clusters from a single template — with List, Git directory, matrix, and cluster generators, diff previews, and progressive sync strategies."
cluster: "GitOps"
series: "ArgoCD"
part: 2
difficulty: "intermediate"
duration: "45 min"
tags: ["gitops", "argocd", "kubernetes", "applicationset", "devops", "delivery", "multi-cluster"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

In [Part 1](/tutorials/gitops/installing-argocd-connecting-git-repository-part-1/) you installed ArgoCD and deployed a single application. In Part 2 you will use `ApplicationSet` resources to manage multi-environment and multi-cluster deployments from a single parameterised template — eliminating the need to maintain one `Application` manifest per environment per application. You will implement List, Git directory, cluster, and matrix generators, add progressive sync strategies, and configure diff previews for pull request environments.

## Prerequisites

- Completed [Part 1](/tutorials/gitops/installing-argocd-connecting-git-repository-part-1/) — ArgoCD installed, CLI logged in, first Application deployed
- A GitOps repository with per-environment directories or overlays
- At least two target environments (staging + production namespaces, or two clusters)

---

## Step 1 — The problem ApplicationSet solves

Without `ApplicationSet`, each application × environment combination requires its own `Application` manifest:

```
# Without ApplicationSet — 6 files for 2 apps × 3 environments
applications/
├── my-app-dev.yaml
├── my-app-staging.yaml
├── my-app-production.yaml
├── payment-service-dev.yaml
├── payment-service-staging.yaml
└── payment-service-production.yaml
```

Adding a new environment means creating two more files. Adding a new application means creating three more. At 20 applications × 4 environments, you maintain 80 `Application` manifests with mostly duplicated configuration.

`ApplicationSet` replaces this with a single template that generates `Application` resources dynamically from a list, a directory structure, a cluster inventory, or a combination:

```
# With ApplicationSet — 1 file covers all combinations
applicationsets/
└── apps.yaml    # generates Applications for all apps × all environments
```

---

## Step 2 — List generator

The `List` generator creates one `Application` per entry in a hardcoded list. It is the simplest generator — useful when environments are well-known and stable:

```yaml
# applicationsets/environments.yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: my-app-environments
  namespace: argocd
spec:
  generators:
    - list:
        elements:
          - env: dev
            namespace: my-app-dev
            values:
              replicaCount: "1"
              domain: dev.example.com
          - env: staging
            namespace: my-app-staging
            values:
              replicaCount: "2"
              domain: staging.example.com
          - env: production
            namespace: my-app-production
            values:
              replicaCount: "5"
              domain: example.com
  template:
    metadata:
      name: "my-app-{{env}}"        # generates my-app-dev, my-app-staging, my-app-production
      namespace: argocd
    spec:
      project: default
      source:
        repoURL: https://github.com/my-org/my-gitops-repo
        targetRevision: main
        path: "apps/my-app/overlays/{{env}}"
      destination:
        server: https://kubernetes.default.svc
        namespace: "{{namespace}}"
      syncPolicy:
        automated:
          prune: true
          selfHeal: true
        syncOptions:
          - CreateNamespace=true
```

Applying this single `ApplicationSet` generates three `Application` resources. Adding a new environment is a single list entry — not a new file.

---

## Step 3 — Git directory generator

The `Git` directory generator discovers environments by scanning the repository for directories matching a pattern. The directory structure itself becomes the source of truth for which environments exist:

```
apps/
├── my-app/
│   ├── dev/
│   ├── staging/
│   └── production/
└── payment-service/
    ├── dev/
    ├── staging/
    └── production/
```

```yaml
# applicationsets/git-dirs.yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: apps-from-directories
  namespace: argocd
spec:
  generators:
    - git:
        repoURL: https://github.com/my-org/my-gitops-repo
        revision: main
        directories:
          - path: "apps/*/*"    # matches apps/<app-name>/<env>
  template:
    metadata:
      name: "{{path.basenameNormalized}}-{{path[1]}}"
      # e.g. apps/my-app/staging → my-app-staging
      namespace: argocd
    spec:
      project: default
      source:
        repoURL: https://github.com/my-org/my-gitops-repo
        targetRevision: main
        path: "{{path}}"       # the matched directory path
      destination:
        server: https://kubernetes.default.svc
        namespace: "{{path[0]}}-{{path[1]}}"    # my-app-staging
      syncPolicy:
        automated:
          prune: true
          selfHeal: true
        syncOptions:
          - CreateNamespace=true
```

**Key variable references:**
- `{{path}}` — the full matched path (`apps/my-app/staging`)
- `{{path.basename}}` — last segment (`staging`)
- `{{path[0]}}` — first segment after the glob start (`my-app`)
- `{{path[1]}}` — second segment (`staging`)

Creating a new environment is as simple as adding a directory to the repository — ArgoCD discovers it automatically and generates the `Application`.

---

## Step 4 — Git file generator

The `Git` file generator reads JSON or YAML configuration files from the repository to parameterise the template. This gives more control than the directory generator — each environment's parameters are explicitly declared in a config file:

```yaml
# apps/my-app/environments/dev.json
{
  "env": "dev",
  "namespace": "my-app-dev",
  "replicaCount": 1,
  "domain": "dev.example.com",
  "cluster": "https://kubernetes.default.svc"
}
```

```yaml
# applicationsets/git-files.yaml
spec:
  generators:
    - git:
        repoURL: https://github.com/my-org/my-gitops-repo
        revision: main
        files:
          - path: "apps/*/environments/*.json"    # matches any env config file
  template:
    metadata:
      name: "my-app-{{env}}"
      namespace: argocd
    spec:
      source:
        path: "apps/my-app/overlays/{{env}}"
      destination:
        server: "{{cluster}}"
        namespace: "{{namespace}}"
```

Parameters from the JSON file are available directly as `{{fieldName}}` in the template. This pattern separates environment configuration (the JSON files) from deployment logic (the template) clearly.

---

## Step 5 — Cluster generator

The `Cluster` generator creates one `Application` per registered cluster in ArgoCD. This is the standard pattern for deploying platform infrastructure (ingress, cert-manager, monitoring) uniformly across all clusters:

```bash
# Register clusters in ArgoCD
argocd cluster add staging-cluster --name staging
argocd cluster add production-cluster --name production

# Label clusters for selective targeting
kubectl label secret -n argocd \
  $(argocd cluster list -o name | grep staging) \
  env=staging tier=non-prod
```

```yaml
# applicationsets/cluster-addons.yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: cluster-addons
  namespace: argocd
spec:
  generators:
    - clusters:
        selector:
          matchLabels:
            env: production    # only deploy to production-labelled clusters
  template:
    metadata:
      name: "cert-manager-{{name}}"    # {{name}} = ArgoCD cluster name
      namespace: argocd
    spec:
      project: platform
      source:
        repoURL: https://github.com/my-org/my-gitops-repo
        targetRevision: main
        path: infrastructure/base/cert-manager
      destination:
        server: "{{server}}"    # {{server}} = cluster API server URL
        namespace: cert-manager
      syncPolicy:
        automated:
          prune: true
          selfHeal: true
        syncOptions:
          - CreateNamespace=true
```

Adding a new production cluster to ArgoCD and labelling it `env=production` automatically triggers a new `Application` for `cert-manager` — no manual manifest creation needed.

---

## Step 6 — Matrix generator

The `Matrix` generator combines two generators — every combination of generator A × generator B produces one `Application`. This is the most powerful pattern for covering all applications across all environments:

```yaml
# applicationsets/all-apps-all-envs.yaml
spec:
  generators:
    - matrix:
        generators:
          # Generator 1: list of applications
          - list:
              elements:
                - app: my-app
                  chart_path: apps/my-app
                - app: payment-service
                  chart_path: apps/payment-service
                - app: notification-service
                  chart_path: apps/notification-service
          # Generator 2: list of environments
          - list:
              elements:
                - env: staging
                  server: https://staging.k8s.example.com
                  namespace_suffix: "-staging"
                - env: production
                  server: https://production.k8s.example.com
                  namespace_suffix: "-production"
  template:
    metadata:
      name: "{{app}}-{{env}}"
      namespace: argocd
    spec:
      project: default
      source:
        repoURL: https://github.com/my-org/my-gitops-repo
        targetRevision: main
        path: "{{chart_path}}/overlays/{{env}}"
      destination:
        server: "{{server}}"
        namespace: "{{app}}{{namespace_suffix}}"
      syncPolicy:
        automated:
          prune: true
          selfHeal: true
```

This single `ApplicationSet` generates 6 `Application` resources (3 apps × 2 environments). Adding a fourth application or a third environment is a one-line list entry.

---

## Step 7 — Progressive sync strategies

For production environments, you want staged rollout rather than all applications syncing simultaneously. The `RollingSync` strategy syncs applications in batches:

```yaml
spec:
  strategy:
    type: RollingSync
    rollingSync:
      steps:
        # Step 1: sync dev first
        - matchExpressions:
            - key: env
              operator: In
              values: [dev]
        # Step 2: sync staging after dev is healthy
        - matchExpressions:
            - key: env
              operator: In
              values: [staging]
          maxUpdate: 50%    # sync up to 50% of matching apps at once
        # Step 3: sync production last
        - matchExpressions:
            - key: env
              operator: In
              values: [production]
          maxUpdate: 25%    # roll out 25% at a time
  template:
    metadata:
      labels:
        env: "{{env}}"    # label used by matchExpressions above
```

The `RollingSync` strategy pauses between steps and waits for all applications in the current step to report `Healthy` before advancing. If a staging deployment fails, production is not touched.

---

## Step 8 — Pull request environments with the PullRequest generator

The `PullRequest` generator creates a temporary `Application` for each open pull request — useful for review environments where each PR gets its own deployed preview:

```yaml
# applicationsets/pr-environments.yaml
spec:
  generators:
    - pullRequest:
        github:
          owner: my-org
          repo: my-app-source
          tokenRef:
            secretName: github-token
            key: token
          labels:
            - preview         # only PRs with this label get an environment
        requeueAfterSeconds: 120    # re-check open PRs every 2 minutes
  template:
    metadata:
      name: "pr-{{number}}-my-app"
      namespace: argocd
    spec:
      project: preview
      source:
        repoURL: https://github.com/my-org/my-gitops-repo
        targetRevision: "{{head_sha}}"    # deploy the PR's commit
        path: apps/my-app/overlays/dev
        helm:
          parameters:
            - name: image.tag
              value: "sha-{{head_short_sha}}"
            - name: ingress.host
              value: "pr-{{number}}.preview.example.com"
      destination:
        server: https://kubernetes.default.svc
        namespace: "pr-{{number}}"
      syncPolicy:
        automated:
          prune: true
          selfHeal: true
        syncOptions:
          - CreateNamespace=true
  syncPolicy:
    preserveResourcesOnDeletion: false    # delete namespace when PR is closed
```

When a PR is opened with the `preview` label, ArgoCD creates a namespace `pr-42` and deploys the PR's image into it. When the PR is merged or closed, ArgoCD deletes the `Application` and the namespace.

---

## Step 9 — Diff preview and sync windows

### Diff preview

Before syncing, view exactly what ArgoCD would apply:

```bash
# Show the diff between live cluster state and Git desired state
argocd app diff my-app-production

# Show diff for all applications in a project
argocd app diff --project platform
```

The web UI shows the same diff visually — green for additions, red for removals — on the application detail page.

### Sync windows

Prevent ArgoCD from syncing during business hours or maintenance-forbidden windows:

```yaml
# AppProject sync windows
spec:
  syncWindows:
    - kind: deny
      schedule: "0 9 * * 1-5"    # cron: 9am Monday–Friday
      duration: 8h                # deny for 8 hours = 9am–5pm weekdays
      applications:
        - "*"
      namespaces:
        - "production-*"
    - kind: allow
      schedule: "0 22 * * 0,6"   # allow Sunday and Saturday 10pm
      duration: 6h
      applications:
        - "*"
      namespaces:
        - "production-*"
      manualSync: true            # allow manual sync even when window is closed
```

ArgoCD blocks automated syncs during deny windows. Manual syncs are blocked unless `manualSync: true` is set on an `allow` window. Sync windows are surfaced in the UI and CLI sync output.

---

## Step 10 — Notifications and observability

ArgoCD Notifications sends alerts when application state changes. Configure a Slack notification for sync failures and successful production deployments:

```yaml
# argocd-notifications-cm ConfigMap
data:
  trigger.on-sync-failed: |
    - when: app.status.sync.status == 'Unknown' || app.status.operationState.phase in ['Error', 'Failed']
      send: [app-sync-failed]

  trigger.on-deployed: |
    - when: app.status.operationState.phase in ['Succeeded'] && app.status.health.status == 'Healthy'
      send: [app-deployed]

  template.app-sync-failed: |
    message: |
      Application {{.app.metadata.name}} sync failed.
      Revision: {{.app.status.sync.revision}}
      Error: {{.app.status.conditions[0].message}}

  template.app-deployed: |
    message: |
      ✅ {{.app.metadata.name}} deployed successfully to {{.app.spec.destination.namespace}}
      Revision: {{.app.status.sync.revision}}

  service.slack: |
    token: $slack-token
    signingSecret: $slack-signing-secret
```

Annotate `Application` resources (or all applications via a default subscription) to receive notifications:

```yaml
metadata:
  annotations:
    notifications.argoproj.io/subscribe.on-sync-failed.slack: deployments-alerts
    notifications.argoproj.io/subscribe.on-deployed.slack: deployments-prod
```

---

## What you have built

- `ApplicationSet` as the solution to `Application` manifest sprawl across environments and clusters
- **List generator** — explicit per-environment parameters in a hardcoded list
- **Git directory generator** — environment discovery from repository directory structure
- **Git file generator** — JSON/YAML config files per environment as parameter sources
- **Cluster generator** — deploy to all registered clusters matching a label selector
- **Matrix generator** — cartesian product of two generators covering all apps × all environments
- **RollingSync strategy** — staged rollout dev → staging → production with health gates between steps
- **PullRequest generator** — ephemeral per-PR preview environments with automatic cleanup on merge
- Diff preview and sync windows for deployment governance and change-freeze enforcement
- ArgoCD Notifications for Slack alerts on sync failure and successful production deployments

With `ApplicationSet` generators in place, the repository becomes the definitive inventory of what runs where — adding an environment is a directory or a list entry, adding a cluster is a `kubectl label`, and the ArgoCD UI gives a single-pane view of sync status and health across every application in every environment.
