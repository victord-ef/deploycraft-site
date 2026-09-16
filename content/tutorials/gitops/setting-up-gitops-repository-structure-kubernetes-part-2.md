---
title: "Setting Up a GitOps Repository Structure for Kubernetes — Part 2"
date: 2026-09-02
description: "Design and implement a production GitOps repository structure: environment directories, Kustomize base/overlays, Helm releases, application tenancy separation, and the conventions that keep a multi-team repository maintainable as it grows."
cluster: "GitOps"
series: "GitOps Foundations"
part: 2
difficulty: "intermediate"
duration: "40 min"
tags: ["gitops", "kubernetes", "flux", "kustomize", "helm", "devops", "delivery"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

By the end of this tutorial you will have a production-ready GitOps repository structure: environment directories with Kustomize overlays, Flux bootstrap configuration, Helm releases managed as code, multi-tenant application separation, and the directory and naming conventions that keep a GitOps repository maintainable as teams and clusters multiply. [Part 1](/tutorials/gitops/introduction-gitops-principles-workflow-kubernetes-part-1/) established why GitOps works — this part is about building the structure that makes it practical.

## Prerequisites

- Completed [Part 1](/tutorials/gitops/introduction-gitops-principles-workflow-kubernetes-part-1/) — understanding of the four GitOps principles and the reconciliation loop
- `kubectl` configured against a Kubernetes cluster
- `flux` CLI installed (`brew install fluxcd/tap/flux` or from [fluxcd.io](https://fluxcd.io/flux/installation/))
- `kustomize` CLI installed (`brew install kustomize`)

---

## Step 1 — Choose a repository layout strategy

There are two dominant layout strategies. The right choice depends on team size and cluster count:

### Monorepo (recommended for most teams)

All environments and all applications live in one Git repository. One pull request can update application code references, infrastructure configuration, and environment-specific values together.

```
gitops-repo/
├── clusters/          # Flux bootstrap — per cluster
├── infrastructure/    # Shared infrastructure (ingress, cert-manager, monitoring)
├── apps/              # Application workloads — per environment overlay
└── tenants/           # Tenant RBAC and namespace configuration
```

**Choose monorepo when:** you have one primary cluster set (staging + production), one platform team, and fewer than ~50 distinct application services.

### Polyrepo

Infrastructure and application configurations live in separate repositories. The platform team owns the infrastructure repo; application teams own their own repos.

**Choose polyrepo when:** you need strict access separation — application teams should not be able to edit ingress controller or cert-manager configuration.

This tutorial implements the monorepo layout, which covers the majority of production use cases.

---

## Step 2 — Bootstrap Flux into the repository

Flux bootstrap creates its own configuration as Kubernetes manifests committed to your Git repository — Flux manages itself via GitOps.

```bash
# Prerequisites: kubectl context pointing at your cluster
export GITHUB_TOKEN=<your-pat>
export GITHUB_USER=<your-github-username>

# Bootstrap Flux into a GitHub repository
flux bootstrap github \
  --owner=${GITHUB_USER} \
  --repository=gitops-repo \
  --branch=main \
  --path=clusters/production \
  --personal

# For an organisation repository:
flux bootstrap github \
  --owner=my-org \
  --repository=gitops-repo \
  --branch=main \
  --path=clusters/production \
  --token-auth
```

After bootstrap, Flux creates the `clusters/production/flux-system/` directory in your repository with:

```
clusters/production/flux-system/
├── gotk-components.yaml    # Flux CRDs and controllers
├── gotk-sync.yaml          # GitRepository + Kustomization pointing at this path
└── kustomization.yaml      # Kustomize entrypoint
```

From this point, any manifest added under `clusters/production/` is automatically applied by Flux.

---

## Step 3 — Structure the clusters directory

The `clusters/` directory is the Flux entrypoint — one subdirectory per cluster. Each cluster directory references the shared infrastructure and application layers:

```
clusters/
├── staging/
│   ├── flux-system/
│   │   ├── gotk-components.yaml
│   │   ├── gotk-sync.yaml
│   │   └── kustomization.yaml
│   ├── infrastructure.yaml    # Kustomization → infrastructure/staging
│   └── apps.yaml              # Kustomization → apps/staging
└── production/
    ├── flux-system/
    │   ├── gotk-components.yaml
    │   ├── gotk-sync.yaml
    │   └── kustomization.yaml
    ├── infrastructure.yaml
    └── apps.yaml
```

The `infrastructure.yaml` and `apps.yaml` files in each cluster directory are Flux `Kustomization` resources that tell Flux where to find configuration for that environment:

```yaml
# clusters/production/infrastructure.yaml
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

```yaml
# clusters/production/apps.yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: apps
  namespace: flux-system
spec:
  interval: 5m
  dependsOn:
    - name: infrastructure    # apps deploy only after infrastructure is ready
  sourceRef:
    kind: GitRepository
    name: flux-system
  path: ./apps/production
  prune: true
```

The `dependsOn` field ensures infrastructure (ingress controller, cert-manager, secret operator) is fully reconciled before application workloads are applied.

---

## Step 4 — Structure the infrastructure directory

Infrastructure components are shared across environments but may have environment-specific values. Use Kustomize base/overlay to manage this:

```
infrastructure/
├── base/
│   ├── cert-manager/
│   │   ├── namespace.yaml
│   │   ├── helmrelease.yaml
│   │   └── kustomization.yaml
│   ├── ingress-nginx/
│   │   ├── namespace.yaml
│   │   ├── helmrelease.yaml
│   │   └── kustomization.yaml
│   └── monitoring/
│       ├── namespace.yaml
│       ├── helmrelease.yaml
│       └── kustomization.yaml
├── staging/
│   └── kustomization.yaml    # patches for staging
└── production/
    └── kustomization.yaml    # patches for production
```

### Example: cert-manager HelmRelease

```yaml
# infrastructure/base/cert-manager/helmrelease.yaml
apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: cert-manager
  namespace: cert-manager
spec:
  interval: 30m
  chart:
    spec:
      chart: cert-manager
      version: ">=1.14.0 <2.0.0"
      sourceRef:
        kind: HelmRepository
        name: jetstack
        namespace: flux-system
      interval: 12h
  values:
    installCRDs: true
    replicaCount: 1
```

```yaml
# infrastructure/base/cert-manager/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - namespace.yaml
  - helmrelease.yaml
```

### Kustomize overlays for environment differences

```yaml
# infrastructure/production/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - ../base/cert-manager
  - ../base/ingress-nginx
  - ../base/monitoring
patches:
  - patch: |
      - op: replace
        path: /spec/values/replicaCount
        value: 2
    target:
      kind: HelmRelease
      name: cert-manager
  - patch: |
      - op: replace
        path: /spec/values/controller/replicaCount
        value: 3
    target:
      kind: HelmRelease
      name: ingress-nginx
```

Staging uses the base values without patching (or with reduced replica counts). Production patches resources upward.

---

## Step 5 — Structure the apps directory

Applications follow the same base/overlay pattern, but the overlay is more significant — replica counts, resource limits, environment variables, and image tags all differ between environments:

```
apps/
├── base/
│   ├── my-app/
│   │   ├── namespace.yaml
│   │   ├── deployment.yaml
│   │   ├── service.yaml
│   │   ├── ingress.yaml
│   │   └── kustomization.yaml
│   └── payment-service/
│       ├── namespace.yaml
│       ├── deployment.yaml
│       ├── service.yaml
│       └── kustomization.yaml
├── staging/
│   ├── kustomization.yaml
│   ├── my-app-patch.yaml
│   └── payment-service-patch.yaml
└── production/
    ├── kustomization.yaml
    ├── my-app-patch.yaml
    └── payment-service-patch.yaml
```

### Base deployment

```yaml
# apps/base/my-app/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: my-app
spec:
  replicas: 1
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      containers:
        - name: my-app
          image: ghcr.io/my-org/my-app:sha-abc1234 # {"$imagepolicy": "flux-system:my-app"}
          ports:
            - containerPort: 8080
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 500m
              memory: 256Mi
```

### Production overlay

```yaml
# apps/production/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - ../base/my-app
  - ../base/payment-service
patches:
  - path: my-app-patch.yaml
  - path: payment-service-patch.yaml
```

```yaml
# apps/production/my-app-patch.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: my-app
spec:
  replicas: 3
  template:
    spec:
      containers:
        - name: my-app
          resources:
            requests:
              cpu: 250m
              memory: 256Mi
            limits:
              cpu: 1000m
              memory: 512Mi
          env:
            - name: LOG_LEVEL
              value: "warn"
            - name: ENVIRONMENT
              value: "production"
```

---

## Step 6 — Manage Helm repositories centrally

All Helm chart repositories used across the cluster are declared once in the `infrastructure/base/` directory and referenced by HelmRelease resources:

```yaml
# infrastructure/base/helm-repositories.yaml
apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRepository
metadata:
  name: jetstack
  namespace: flux-system
spec:
  interval: 12h
  url: https://charts.jetstack.io
---
apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRepository
metadata:
  name: ingress-nginx
  namespace: flux-system
spec:
  interval: 12h
  url: https://kubernetes.github.io/ingress-nginx
---
apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRepository
metadata:
  name: prometheus-community
  namespace: flux-system
spec:
  interval: 12h
  url: https://prometheus-community.github.io/helm-charts
```

Centralising HelmRepository resources prevents duplication and ensures all controllers poll the same upstream cache.

---

## Step 7 — Multi-tenancy: namespace and RBAC separation

For teams deploying their own applications, the `tenants/` directory manages namespace creation, service accounts, and RBAC constraints that limit what each tenant can deploy:

```
tenants/
├── base/
│   ├── team-platform/
│   │   ├── namespace.yaml
│   │   ├── rbac.yaml
│   │   └── kustomization.yaml
│   └── team-payments/
│       ├── namespace.yaml
│       ├── rbac.yaml
│       └── kustomization.yaml
├── staging/
│   └── kustomization.yaml
└── production/
    └── kustomization.yaml
```

```yaml
# tenants/base/team-payments/rbac.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: team-payments-reconciler
  namespace: payments
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin    # scoped to namespace by RoleBinding
subjects:
  - kind: ServiceAccount
    name: flux-reconciler
    namespace: flux-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: team-payments-developers
  namespace: payments
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: view
subjects:
  - kind: Group
    name: team-payments
    apiGroup: rbac.authorization.k8s.io
```

Each tenant team has a dedicated namespace. Flux reconciles into that namespace using a service account scoped to it. Developers have view access — they cannot `kubectl apply` directly.

---

## Step 8 — Secrets strategy

Secrets in plaintext cannot be committed to Git. Three approaches fit a GitOps workflow:

### Option A — Sealed Secrets (simplest)

Bitnami Sealed Secrets encrypts Kubernetes secrets with a cluster-held private key. The encrypted `SealedSecret` CRD is safe to commit to Git:

```bash
# Install the Sealed Secrets controller
flux create source helm sealed-secrets \
  --url=https://bitnami-labs.github.io/sealed-secrets \
  --namespace=flux-system

flux create helmrelease sealed-secrets \
  --chart=sealed-secrets \
  --source=HelmRepository/sealed-secrets \
  --chart-version=">=2.0.0" \
  --namespace=kube-system

# Seal a secret
kubectl create secret generic db-credentials \
  --from-literal=password=supersecret \
  --dry-run=client -o yaml \
  | kubeseal --format yaml > apps/base/my-app/db-credentials-sealed.yaml
```

The `SealedSecret` is committed to Git. The controller decrypts it in-cluster and creates the actual `Secret`. Rotating the cluster key requires re-sealing all secrets.

### Option B — External Secrets Operator (recommended for production)

ESO fetches secrets from an external secrets manager (AWS Secrets Manager, GCP Secret Manager, HashiCorp Vault, Azure Key Vault) and creates Kubernetes `Secret` objects:

```yaml
# apps/base/my-app/external-secret.yaml
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: db-credentials
  namespace: my-app
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: aws-secretsmanager
    kind: ClusterSecretStore
  target:
    name: db-credentials
    creationPolicy: Owner
  data:
    - secretKey: password
      remoteRef:
        key: production/my-app/db-credentials
        property: password
```

The `ExternalSecret` manifest (containing only the secret name, not the value) is safe to commit to Git. The actual secret value lives in AWS Secrets Manager and is injected at runtime.

### Option C — SOPS (for teams preferring Git-native encryption)

Mozilla SOPS encrypts secret values in YAML files using age keys or AWS KMS. Flux has native SOPS decryption support:

```bash
# Generate an age key
age-keygen -o age.agekey

# Create a Kubernetes secret with the private key (for Flux to use)
cat age.agekey | kubectl create secret generic sops-age \
  --namespace=flux-system \
  --from-file=age.agekey=/dev/stdin

# Encrypt a file
sops --age=$(cat age.agekey | grep "public key" | awk '{print $NF}') \
  --encrypt --in-place apps/base/my-app/secret.yaml
```

Configure Flux to decrypt SOPS-encrypted files:

```yaml
# clusters/production/flux-system/gotk-sync.yaml (add decryption section)
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: flux-system
  namespace: flux-system
spec:
  decryption:
    provider: sops
    secretRef:
      name: sops-age
```

---

## Step 9 — Repository conventions at scale

Conventions that prevent confusion as the repository grows:

**Naming:**
```
apps/base/<service-name>/          # kebab-case, matches Deployment name
infrastructure/base/<component>/   # matches the Helm chart name
clusters/<env-name>/               # matches kubectl context name convention
```

**Kustomization labels:** Every base includes consistent labels that propagate to all resources:

```yaml
# apps/base/my-app/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
commonLabels:
  app.kubernetes.io/name: my-app
  app.kubernetes.io/managed-by: flux
resources:
  - namespace.yaml
  - deployment.yaml
  - service.yaml
  - ingress.yaml
```

**Image policy markers in base only:** Put the `# {"$imagepolicy": ...}` comment in the base deployment, not in overlays. Overlays patch resources, replica counts, and limits — not the image field, which image automation manages.

**One directory per concern:** Resist combining multiple applications in one Kustomization path. Each application gets its own directory — Flux reconciles them independently, making errors and rollbacks easier to isolate.

---

## Step 10 — Validate the repository locally

Before committing, validate that Kustomize renders the expected output:

```bash
# Render the production apps layer — inspect all resources
kustomize build apps/production

# Render infrastructure
kustomize build infrastructure/production

# Validate rendered output against a live cluster (dry-run)
kustomize build apps/production | kubectl apply --dry-run=server -f -

# Flux can also validate locally (requires flux CLI)
flux build kustomization apps \
  --path=./apps/production \
  --kustomization-file=clusters/production/apps.yaml \
  --dry-run
```

Add this validation as a CI step so every pull request against the GitOps repository is validated before merge:

```yaml
# .github/workflows/validate.yml
name: Validate manifests

on:
  pull_request:
    branches: [main]

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Kustomize
        uses: imranismail/setup-kustomize@v2

      - name: Validate staging
        run: kustomize build apps/staging | kubectl apply --dry-run=client -f -

      - name: Validate production
        run: kustomize build apps/production | kubectl apply --dry-run=client -f -
```

---

## What you have built

- A production monorepo GitOps layout: `clusters/`, `infrastructure/`, `apps/`, `tenants/`
- Flux bootstrap into `clusters/<env>/flux-system/` — Flux managing itself via GitOps
- Flux `Kustomization` resources in each cluster directory wiring infrastructure and app layers together with `dependsOn` ordering
- Kustomize base/overlay structure: shared base definitions, environment-specific patches for replicas, resources, and environment variables
- Centralised `HelmRepository` declarations and `HelmRelease` resources for infrastructure components
- Multi-tenant RBAC separation via `tenants/`: namespace-scoped service accounts, developer view access
- Three secrets strategies — Sealed Secrets, External Secrets Operator, SOPS — with GitOps-compatible storage for each
- Repository naming conventions and kustomization common labels for consistent resource metadata
- Local validation with `kustomize build` and `kubectl --dry-run`, and a GitHub Actions CI workflow for PR validation

With this structure in place, environment promotion is a pull request that moves a patch value or image tag from one overlay to another. Full cluster recovery is `flux bootstrap` — the cluster reads its entire desired state from Git and self-heals.
