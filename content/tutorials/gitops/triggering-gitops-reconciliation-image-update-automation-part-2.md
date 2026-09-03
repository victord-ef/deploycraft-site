---
title: "Triggering GitOps Reconciliation from a CI Pipeline with Image Update Automation — Part 2"
date: 2026-09-02
description: "Close the loop between CI and GitOps: use Flux Image Automation and ArgoCD Image Updater to detect new image tags, update deployment manifests automatically, and trigger cluster reconciliation — with policy controls over which tags are promoted to which environments."
cluster: "GitOps"
series: "CI Pipeline Integration"
part: 2
difficulty: "intermediate"
duration: "50 min"
tags: ["ci-cd", "gitops", "flux", "argocd", "image-automation", "github-actions", "kubernetes", "devsecops"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

In [Part 1](/tutorials/gitops/building-container-images-ci-pushing-registry-part-1/) you built a CI pipeline that builds, scans, and pushes a tagged container image to GHCR on every commit. In Part 2 you will close the GitOps loop: configure Flux Image Automation (or ArgoCD Image Updater) to detect the new image, update the deployment manifest in your GitOps repository, and trigger a cluster reconciliation — automatically, without manual manifest edits.

## Prerequisites

- Completed [Part 1](/tutorials/gitops/building-container-images-ci-pushing-registry-part-1/) — images being pushed to GHCR with commit SHA tags
- A Kubernetes cluster with Flux or ArgoCD installed
- A GitOps repository containing your deployment manifests

---

## The CI-to-GitOps gap

After Part 1, CI pushes `ghcr.io/org/app:sha-abc1234` to GHCR. But your Kubernetes cluster still runs the previous image — nothing has told it about the new tag. Bridging this gap is the critical last mile of a GitOps delivery pipeline.

There are two patterns:

**Pattern A — CI writes to Git (push-based):**
The CI pipeline itself opens a pull request or commits a manifest update into the GitOps repo after a successful build. The GitOps operator detects the Git change and reconciles.

**Pattern B — GitOps operator polls the registry (pull-based):**
The GitOps operator (Flux, ArgoCD) continuously polls the container registry for new tags. When a new tag matching a policy is found, the operator updates the manifest and commits to Git itself.

Pattern B is the recommended GitOps approach — the cluster drives reconciliation from registry state, keeping Git as the single source of truth.

---

## Part A — Flux Image Automation

### Step 1 — Install the Flux image-automation controllers

```bash
# Check if image-automation controllers are installed
flux get sources image

# If not installed, upgrade with image-automation components
flux install \
  --components-extra=image-reflector-controller,image-automation-controller

# Verify controllers are running
kubectl get pods -n flux-system | grep image
# image-automation-controller-xxx   1/1   Running
# image-reflector-controller-xxx    1/1   Running
```

### Step 2 — Create an ImageRepository

An `ImageRepository` tells Flux which registry image to poll and how often:

```yaml
# image-repository.yaml
apiVersion: image.toolkit.fluxcd.io/v1beta2
kind: ImageRepository
metadata:
  name: my-app
  namespace: flux-system
spec:
  image: ghcr.io/my-org/my-app
  interval: 1m0s
  secretRef:
    name: ghcr-credentials    # optional for public repos
```

If your GHCR image is private, create the pull secret:

```bash
kubectl create secret docker-registry ghcr-credentials \
  --namespace flux-system \
  --docker-server=ghcr.io \
  --docker-username=<github-username> \
  --docker-password=<github-pat>
```

```bash
kubectl apply -f image-repository.yaml

# Verify Flux can see the image tags
flux get image repository my-app
# NAME     LAST SCAN          TAGS
# my-app   2026-09-02T10:01Z  47
```

### Step 3 — Create an ImagePolicy

An `ImagePolicy` defines which tags Flux should track and which one is "latest":

```yaml
# image-policy.yaml
apiVersion: image.toolkit.fluxcd.io/v1beta2
kind: ImagePolicy
metadata:
  name: my-app
  namespace: flux-system
spec:
  imageRepositoryRef:
    name: my-app
  # Track semver tags — latest stable release
  policy:
    semver:
      range: ">=1.0.0"
```

For commit-SHA-based tags (the `sha-abc1234` pattern from Part 1):

```yaml
spec:
  imageRepositoryRef:
    name: my-app
  policy:
    alphabetical:
      order: asc    # sha- prefixed tags sort chronologically
  filterTags:
    pattern: "^sha-[a-f0-9]+"    # only match sha- prefixed tags
```

For environment promotion — track only `main` branch builds for staging, semver for production:

```yaml
# staging: track main branch SHA tags
filterTags:
  pattern: "^sha-[a-f0-9]+"
  extract: "$timestamp"    # if tags include timestamp for ordering

# production: track semver releases only
policy:
  semver:
    range: ">=1.0.0 <2.0.0"    # major version pinning
```

```bash
kubectl apply -f image-policy.yaml

# See which tag Flux selected as latest
flux get image policy my-app
# NAME    LATEST IMAGE
# my-app  ghcr.io/my-org/my-app:sha-abc1234
```

### Step 4 — Mark the manifest for automated updates

Add a comment marker to your deployment manifest telling Flux which field to update:

```yaml
# apps/my-app/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: default
spec:
  template:
    spec:
      containers:
        - name: my-app
          image: ghcr.io/my-org/my-app:sha-abc1234 # {"$imagepolicy": "flux-system:my-app"}
```

The comment `# {"$imagepolicy": "flux-system:my-app"}` is the marker Flux uses to locate and update the image tag. Everything else in the manifest is untouched.

### Step 5 — Create an ImageUpdateAutomation

The `ImageUpdateAutomation` resource tells Flux to commit manifest updates back to Git:

```yaml
# image-update-automation.yaml
apiVersion: image.toolkit.fluxcd.io/v1beta1
kind: ImageUpdateAutomation
metadata:
  name: my-app
  namespace: flux-system
spec:
  interval: 1m0s
  sourceRef:
    kind: GitRepository
    name: flux-system    # the GitRepository pointing to your GitOps repo
  git:
    checkout:
      ref:
        branch: main
    commit:
      author:
        email: flux@deploycraft.io
        name: Flux Image Automation
      messageTemplate: |
        chore(image): update my-app to {{range .Updated.Images}}{{.}}{{end}}
    push:
      branch: main    # commit directly to main
      # For PR-based promotion, use a different branch:
      # branch: flux/image-update-my-app
  update:
    path: ./apps/my-app    # path in the GitOps repo to scan for markers
    strategy: Setters
```

```bash
kubectl apply -f image-update-automation.yaml

# Watch automation status
flux get image update my-app
# NAME    LAST RUN              RESULT
# my-app  2026-09-02T10:02Z     committed 1 change(s)
```

The full automated flow:
1. CI pushes `ghcr.io/my-org/my-app:sha-def5678` to GHCR
2. `ImageRepository` polls GHCR — sees the new tag
3. `ImagePolicy` evaluates the tag against the policy — selects it as latest
4. `ImageUpdateAutomation` updates `apps/my-app/deployment.yaml` and commits to main
5. Flux `Kustomization` detects the Git change — reconciles the cluster
6. Kubernetes rolls out the new image

---

## Part B — ArgoCD Image Updater

### Step 6 — Install ArgoCD Image Updater

```bash
kubectl apply -n argocd \
  -f https://raw.githubusercontent.com/argoproj-labs/argocd-image-updater/stable/manifests/install.yaml

kubectl rollout status deployment argocd-image-updater -n argocd
```

### Step 7 — Configure registry credentials

```bash
# Create a secret for GHCR access
kubectl create secret generic ghcr-credentials \
  --namespace argocd \
  --from-literal=credentials=ghcr.io:my-github-username:ghp_mytoken

# Reference it in the image-updater ConfigMap
kubectl edit configmap argocd-image-updater-config -n argocd
```

```yaml
# argocd-image-updater-config ConfigMap
data:
  registries.conf: |
    registries:
      - name: GitHub Container Registry
        api_url: https://ghcr.io
        prefix: ghcr.io
        credentials: secret:argocd/ghcr-credentials#credentials
        default: false
```

### Step 8 — Annotate your ArgoCD Application

Image Updater is configured via annotations on the `Application` resource:

```yaml
# argocd-application.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app
  namespace: argocd
  annotations:
    # Which images to watch
    argocd-image-updater.argoproj.io/image-list: my-app=ghcr.io/my-org/my-app

    # Update strategy: git (commits to repo) or argocd (patches Application directly)
    argocd-image-updater.argoproj.io/write-back-method: git

    # Which Git branch to target
    argocd-image-updater.argoproj.io/git-branch: main

    # Tag update strategy — semver latest
    argocd-image-updater.argoproj.io/my-app.update-strategy: semver
    argocd-image-updater.argoproj.io/my-app.allow-tags: regexp:^v[0-9]+\.[0-9]+\.[0-9]+$

    # For SHA-based tags with alphabetical ordering:
    # argocd-image-updater.argoproj.io/my-app.update-strategy: latest
    # argocd-image-updater.argoproj.io/my-app.allow-tags: regexp:^sha-[a-f0-9]+$

spec:
  source:
    repoURL: https://github.com/my-org/my-gitops-repo
    targetRevision: main
    path: apps/my-app
  destination:
    server: https://kubernetes.default.svc
    namespace: default
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
```

```bash
kubectl apply -f argocd-application.yaml

# Monitor Image Updater logs
kubectl logs -n argocd -l app.kubernetes.io/name=argocd-image-updater -f
# time="..." level=info msg="Setting new image" image=ghcr.io/my-org/my-app:v1.2.3
```

---

## Step 9 — Environment promotion with PR-based approval

For production environments, requiring a human approval before an image is promoted is good practice. Use Flux with a separate branch to create a pull request for each update:

```yaml
# image-update-automation-prod.yaml
spec:
  git:
    push:
      branch: flux/promote-to-prod    # push to a feature branch, not main
```

Then add a GitHub Actions workflow that auto-creates a PR from that branch:

```yaml
# .github/workflows/promote-pr.yml
name: Create promotion PR

on:
  push:
    branches: ["flux/promote-to-prod"]

jobs:
  open-pr:
    runs-on: ubuntu-latest
    permissions:
      pull-requests: write
      contents: read
    steps:
      - uses: actions/checkout@v4

      - name: Create Pull Request
        uses: peter-evans/create-pull-request@v6
        with:
          token: ${{ secrets.GITHUB_TOKEN }}
          base: main
          head: flux/promote-to-prod
          title: "chore: promote image to production"
          body: |
            Automated image promotion from staging to production.
            Review the image tag change and approve to deploy.
          labels: "promotion,automated"
          reviewers: "platform-team"
```

The flow:
1. CI builds and pushes image — staging auto-deploys via Flux
2. Flux writes the updated tag to `flux/promote-to-prod` branch
3. GitHub Actions opens a PR targeting `main`
4. A platform engineer reviews the diff and approves
5. PR merges — Flux reconciles production

---

## Step 10 — End-to-end pipeline verification

```bash
# 1. Push a commit to trigger CI
git commit --allow-empty -m "test: trigger CI build"
git push origin main

# 2. Watch the GitHub Actions run complete and push the image
# Check GHCR: https://github.com/org/repo/pkgs/container/repo

# 3. Watch Flux detect the new tag (within interval — default 1m)
flux get image repository my-app --watch

# 4. Watch the policy select the new tag
flux get image policy my-app --watch

# 5. Watch the automation commit to Git
flux get image update my-app --watch

# 6. Watch the Kustomization reconcile
flux get kustomizations --watch

# 7. Verify the new image is running in the cluster
kubectl get pods -n default -l app=my-app -o jsonpath='{.items[0].spec.containers[0].image}'
# ghcr.io/my-org/my-app:sha-def5678
```

Full round-trip — from `git push` to running pod — typically completes in under 3 minutes with default Flux intervals.

---

## What you have built

- Flux `ImageRepository`, `ImagePolicy`, and `ImageUpdateAutomation` resources forming the complete image automation stack
- Semver and SHA-based tag policies for different environments
- Manifest marker comments (`$imagepolicy`) for Flux to locate and update the correct field
- ArgoCD Image Updater as an alternative — annotation-driven image tracking with git write-back
- Environment promotion via Flux push to a feature branch and a GitHub Actions PR creation workflow
- End-to-end pipeline verification from `git push` through to running pod
