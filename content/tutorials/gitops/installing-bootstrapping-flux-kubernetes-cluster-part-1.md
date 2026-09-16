---
title: "Installing and Bootstrapping Flux in a Kubernetes Cluster — Part 1"
date: 2026-09-02
description: "Install the Flux CLI, preflight-check your cluster, bootstrap Flux from a GitHub repository, verify the control plane components, and understand how Flux manages itself via GitOps — with Kustomize and HelmRelease controllers ready for Part 2."
cluster: "GitOps"
series: "Flux"
part: 1
difficulty: "intermediate"
duration: "35 min"
tags: ["gitops", "flux", "kubernetes", "kustomize", "helm", "devops", "delivery"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

By the end of this tutorial you will have Flux fully installed and bootstrapped in a Kubernetes cluster — connected to a GitHub repository that Flux itself manages as code. You will understand every component Flux installs, how the bootstrap process works, how to verify a healthy Flux installation, and how to configure multi-cluster and multi-tenant setups. Part 2 builds on this foundation by deploying applications and infrastructure using Flux `Kustomization` and `HelmRelease` resources.

## Prerequisites

- A Kubernetes cluster with `kubectl` configured (kind, k3s, EKS, GKE, or AKS all work)
- A GitHub account and a personal access token (PAT) with `repo` scope
- `kubectl` 1.28+

---

## Step 1 — Install the Flux CLI

The Flux CLI is the primary tool for bootstrapping, inspecting, and troubleshooting Flux installations.

```bash
# macOS / Linux — Homebrew
brew install fluxcd/tap/flux

# Linux — official install script
curl -s https://fluxcd.io/install.sh | sudo bash

# Windows — Chocolatey
choco install flux

# Or download a specific release binary
# https://github.com/fluxcd/flux2/releases
```

Verify the installation:

```bash
flux version
# flux: v2.3.0
# distribution: flux-operator
```

---

## Step 2 — Preflight check

Before bootstrapping, Flux can verify that your cluster meets the minimum requirements:

```bash
flux check --pre
# ► checking prerequisites
# ✔ Kubernetes 1.29.2 >=1.26.0-0
# ✔ prerequisites checks passed
```

The preflight check verifies:
- Kubernetes version is supported (1.26+)
- `kubectl` can reach the API server
- The cluster has sufficient RBAC permissions to install Flux CRDs
- No conflicting Flux installations exist

If you are using a private or restricted cluster (EKS with restrictive IAM, GKE Autopilot, or OpenShift), check the Flux compatibility notes for your distribution before proceeding.

---

## Step 3 — Create a GitHub repository for GitOps configuration

If you do not already have a GitOps repository, create one:

```bash
# Create a new private GitHub repository via the CLI
gh repo create my-org/gitops-repo \
  --private \
  --description "GitOps configuration for Kubernetes clusters"
```

Or create it through the GitHub UI. The repository can be empty — Flux will populate it during bootstrap.

Export your credentials:

```bash
export GITHUB_TOKEN=<your-pat>
export GITHUB_USER=<your-github-username>     # personal account
# or
export GITHUB_ORG=<your-organisation>          # organisation account
```

The PAT needs `repo` scope (read/write to repository contents). For fine-grained PATs, grant **Contents: Read and write** and **Metadata: Read** on the target repository.

---

## Step 4 — Bootstrap Flux

Bootstrap installs Flux into your cluster and commits the Flux manifests back to your Git repository — so Flux manages itself:

```bash
# Personal repository
flux bootstrap github \
  --owner=${GITHUB_USER} \
  --repository=gitops-repo \
  --branch=main \
  --path=clusters/my-cluster \
  --personal

# Organisation repository
flux bootstrap github \
  --owner=${GITHUB_ORG} \
  --repository=gitops-repo \
  --branch=main \
  --path=clusters/my-cluster
```

What bootstrap does step by step:

1. **Installs Flux CRDs** — `GitRepository`, `Kustomization`, `HelmRepository`, `HelmRelease`, `ImageRepository`, `ImagePolicy`, `ImageUpdateAutomation`, and the notification resources
2. **Deploys Flux controllers** into the `flux-system` namespace
3. **Creates a deploy key** — a read-only SSH key registered on the Git repository, used by the source controller to pull from Git
4. **Commits Flux manifests** to `clusters/my-cluster/flux-system/` in your repository
5. **Applies the manifests** — Flux reconciles from Git immediately, entering the GitOps loop

After bootstrap completes:

```bash
flux check
# ► checking prerequisites
# ✔ Kubernetes 1.29.2 >=1.26.0-0
# ► checking controllers
# ✔ helm-controller: deployment ready
# ✔ kustomize-controller: deployment ready
# ✔ notification-controller: deployment ready
# ✔ source-controller: deployment ready
# ► checking crds
# ✔ alerts.notification.toolkit.fluxcd.io/v1beta3
# ✔ gitrepositories.source.toolkit.fluxcd.io/v1
# ✔ helmreleases.helm.toolkit.fluxcd.io/v2
# ✔ kustomizations.kustomize.toolkit.fluxcd.io/v1
# ✔ all checks passed
```

---

## Step 5 — Inspect what bootstrap committed to Git

Pull the repository to see what Flux created:

```bash
git clone https://github.com/${GITHUB_USER}/gitops-repo
cd gitops-repo
tree clusters/
```

```
clusters/
└── my-cluster/
    └── flux-system/
        ├── gotk-components.yaml    # all Flux CRDs and controller Deployments
        ├── gotk-sync.yaml          # the GitRepository + Kustomization that bootstraps Flux itself
        └── kustomization.yaml      # Kustomize entrypoint for flux-system
```

### gotk-sync.yaml — the self-referential bootstrap

```yaml
# clusters/my-cluster/flux-system/gotk-sync.yaml
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: flux-system
  namespace: flux-system
spec:
  interval: 1m0s
  ref:
    branch: main
  secretRef:
    name: flux-system    # the deploy key Flux created
  url: ssh://git@github.com/my-org/gitops-repo
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: flux-system
  namespace: flux-system
spec:
  interval: 10m0s
  path: ./clusters/my-cluster
  prune: true
  sourceRef:
    kind: GitRepository
    name: flux-system
```

This `Kustomization` tells Flux to reconcile everything under `clusters/my-cluster/` — including itself. Adding any manifest under that path causes Flux to automatically apply it.

---

## Step 6 — Understand the Flux controller architecture

Flux is composed of separate controllers, each responsible for a specific concern:

```
┌─────────────────────────────────────────────────────────────────┐
│                        flux-system namespace                     │
│                                                                  │
│  source-controller          kustomize-controller                │
│  ─────────────────          ─────────────────────               │
│  Fetches from Git,          Renders Kustomize overlays,         │
│  Helm repos, OCI,           applies resources to cluster        │
│  S3 buckets                                                      │
│                                                                  │
│  helm-controller            notification-controller             │
│  ─────────────              ───────────────────────             │
│  Manages Helm releases,     Routes alerts to Slack,             │
│  upgrades, rollbacks        GitHub, PagerDuty, etc.             │
│                                                                  │
│  image-reflector-controller     image-automation-controller     │
│  ──────────────────────────     ────────────────────────────    │
│  Polls container registries     Updates manifests in Git        │
│  for new image tags             when new tags appear            │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

| Controller | CRDs it manages | What it does |
|---|---|---|
| source-controller | `GitRepository`, `HelmRepository`, `OCIRepository`, `Bucket` | Fetches and caches external sources |
| kustomize-controller | `Kustomization` | Renders and applies Kustomize manifests |
| helm-controller | `HelmRelease` | Installs and upgrades Helm charts |
| notification-controller | `Alert`, `Provider`, `Receiver` | Sends alerts and receives webhook triggers |
| image-reflector-controller | `ImageRepository`, `ImagePolicy` | Scans registries for new tags |
| image-automation-controller | `ImageUpdateAutomation` | Updates manifest image tags and commits to Git |

The image controllers are not installed by default — enable them with:

```bash
flux bootstrap github \
  --components-extra=image-reflector-controller,image-automation-controller \
  ...
```

---

## Step 7 — Add a second cluster

To manage multiple clusters from one repository, bootstrap each cluster into its own subdirectory:

```bash
# Switch kubectl context to the staging cluster
kubectl config use-context staging-cluster

flux bootstrap github \
  --owner=${GITHUB_ORG} \
  --repository=gitops-repo \
  --branch=main \
  --path=clusters/staging \
  --token-auth
```

The repository now has:

```
clusters/
├── my-cluster/        # production
│   └── flux-system/
└── staging/
    └── flux-system/
```

Each cluster has its own `gotk-sync.yaml` pointing at its own path. Adding a manifest under `clusters/staging/` applies only to staging — adding it under `clusters/my-cluster/` applies only to production. Shared configuration lives under `infrastructure/` and `apps/` and is referenced by Flux `Kustomization` resources in each cluster directory.

---

## Step 8 — Configure Flux notifications

Flux can send alerts to Slack, Microsoft Teams, GitHub commit statuses, and PagerDuty when reconciliation succeeds or fails:

```yaml
# clusters/my-cluster/flux-system/slack-provider.yaml
apiVersion: notification.toolkit.fluxcd.io/v1beta3
kind: Provider
metadata:
  name: slack
  namespace: flux-system
spec:
  type: slack
  channel: "#deployments"
  secretRef:
    name: slack-url    # contains the Slack webhook URL
---
apiVersion: notification.toolkit.fluxcd.io/v1beta3
kind: Alert
metadata:
  name: on-call-alert
  namespace: flux-system
spec:
  summary: "Flux reconciliation failure"
  providerRef:
    name: slack
  eventSeverity: error
  eventSources:
    - kind: GitRepository
      name: "*"
    - kind: Kustomization
      name: "*"
    - kind: HelmRelease
      name: "*"
```

Create the secret for the Slack webhook:

```bash
kubectl create secret generic slack-url \
  --namespace flux-system \
  --from-literal=address=https://hooks.slack.com/services/T.../B.../...
```

Commit `slack-provider.yaml` to `clusters/my-cluster/flux-system/` and Flux applies it automatically — the notification controller starts sending alerts immediately.

---

## Step 9 — Suspend and resume reconciliation

During maintenance windows or incident response, you may need to pause Flux reconciliation:

```bash
# Suspend all reconciliation for a specific Kustomization
flux suspend kustomization apps

# Suspend a HelmRelease
flux suspend helmrelease cert-manager -n cert-manager

# Resume
flux resume kustomization apps
flux resume helmrelease cert-manager -n cert-manager

# Suspend the entire source-controller (stops all fetching)
kubectl patch gitrepository flux-system -n flux-system \
  --type merge -p '{"spec":{"suspend":true}}'
```

Suspension is recorded in the resource spec (`suspend: true`) so it survives controller restarts. Always resume explicitly — do not rely on cluster restarts to clear a suspension.

---

## Step 10 — Uninstall Flux

To cleanly remove Flux from a cluster (leaving your workloads running):

```bash
flux uninstall --namespace=flux-system
```

This removes:
- All Flux controllers and CRDs
- The `flux-system` namespace
- Flux-managed resources that have `prune: true` set **are kept** unless you pass `--silent` — Flux uninstalls itself without pruning reconciled resources by default.

To remove a specific cluster's bootstrap from the repository without uninstalling from the cluster:

```bash
git rm -r clusters/my-cluster/flux-system
git commit -m "chore: remove flux bootstrap for my-cluster"
git push
```

---

## What you have built

- Flux CLI installed and preflight-checked against the target cluster
- GitHub PAT configuration and repository creation for the GitOps store
- Flux bootstrapped via `flux bootstrap github` — self-managing Flux installation committed to Git
- The bootstrap directory layout: `clusters/<name>/flux-system/` with `gotk-components.yaml`, `gotk-sync.yaml`, `kustomization.yaml`
- Understanding of the six Flux controllers and the CRDs each manages
- Multi-cluster setup: each cluster bootstrapped to its own `clusters/<name>/` path in the same repository
- Slack notifications via `Provider` and `Alert` resources
- Reconciliation suspension and resume for maintenance windows

In [Part 2](/tutorials/gitops/automating-deployments-flux-kustomization-helmrelease-part-2/) you will use Flux `Kustomization` and `HelmRelease` resources to deploy applications and infrastructure components — including environment-specific overlays, Helm chart version pinning, automated upgrades, and deployment health verification.
