---
title: "Installing ArgoCD and Connecting Your First Git Repository — Part 1"
date: 2026-09-02
description: "Install ArgoCD in a Kubernetes cluster, configure the CLI and web UI, connect a Git repository, deploy your first Application, understand sync policies and health checks, and set up RBAC and SSO for team access."
cluster: "GitOps"
series: "ArgoCD"
part: 1
difficulty: "intermediate"
duration: "40 min"
tags: ["gitops", "argocd", "kubernetes", "devops", "delivery", "devsecops"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

By the end of this tutorial you will have ArgoCD installed in a Kubernetes cluster, connected to a Git repository, and managing your first application. You will understand ArgoCD's architecture, how the web UI and CLI work, how sync policies control when and how changes are applied, and how to configure RBAC so teams can access their own applications without touching others. Part 2 extends this to multi-environment deployments using `ApplicationSet` resources.

## Prerequisites

- A Kubernetes cluster with `kubectl` configured (kind, k3s, EKS, GKE, or AKS)
- A Git repository containing Kubernetes manifests or a Helm chart
- `kubectl` 1.28+

---

## Step 1 — Install ArgoCD

ArgoCD is installed from its official manifests into a dedicated namespace:

```bash
# Create the argocd namespace
kubectl create namespace argocd

# Install the latest stable release
kubectl apply -n argocd \
  -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml

# Wait for all ArgoCD components to become ready
kubectl rollout status deployment argocd-server -n argocd
kubectl rollout status deployment argocd-repo-server -n argocd
kubectl rollout status deployment argocd-application-controller -n argocd
```

Verify all pods are running:

```bash
kubectl get pods -n argocd
# NAME                                                READY   STATUS    RESTARTS
# argocd-application-controller-0                    1/1     Running   0
# argocd-applicationset-controller-xxx               1/1     Running   0
# argocd-dex-server-xxx                              1/1     Running   0
# argocd-notifications-controller-xxx                1/1     Running   0
# argocd-redis-xxx                                   1/1     Running   0
# argocd-repo-server-xxx                             1/1     Running   0
# argocd-server-xxx                                  1/1     Running   0
```

### High-availability installation

For production clusters, use the HA manifest which runs multiple replicas of each component:

```bash
kubectl apply -n argocd \
  -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/ha/install.yaml
```

---

## Step 2 — Install the ArgoCD CLI

```bash
# macOS
brew install argocd

# Linux
curl -sSL -o argocd \
  https://github.com/argoproj/argo-cd/releases/latest/download/argocd-linux-amd64
chmod +x argocd
sudo mv argocd /usr/local/bin/

# Windows
choco install argocd-cli
```

---

## Step 3 — Access the ArgoCD API server

The ArgoCD API server is not exposed externally by default. Access it via port-forward during initial setup:

```bash
kubectl port-forward svc/argocd-server -n argocd 8080:443
```

Retrieve the initial admin password:

```bash
argocd admin initial-password -n argocd
# <generated-password>
```

Log in:

```bash
argocd login localhost:8080 \
  --username admin \
  --password <generated-password> \
  --insecure    # skip TLS verification for port-forward access
```

Change the admin password immediately:

```bash
argocd account update-password \
  --current-password <generated-password> \
  --new-password <your-new-password>
```

The web UI is available at `https://localhost:8080` — the same credentials apply.

### Expose ArgoCD via an ingress (production)

For persistent access, create an ingress with TLS:

```yaml
# argocd-ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: argocd-server
  namespace: argocd
  annotations:
    nginx.ingress.kubernetes.io/ssl-passthrough: "true"
    nginx.ingress.kubernetes.io/backend-protocol: "HTTPS"
spec:
  ingressClassName: nginx
  rules:
    - host: argocd.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: argocd-server
                port:
                  name: https
```

`ssl-passthrough` lets ArgoCD handle TLS termination with its own certificate. Alternatively, disable ArgoCD's TLS and let the ingress controller terminate:

```bash
kubectl patch configmap argocd-cmd-params-cm -n argocd \
  --type merge \
  -p '{"data":{"server.insecure":"true"}}'
kubectl rollout restart deployment argocd-server -n argocd
```

---

## Step 4 — ArgoCD architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         argocd namespace                         │
│                                                                  │
│  argocd-server              argocd-repo-server                  │
│  ─────────────              ───────────────────                  │
│  API server + Web UI        Clones Git repos, renders           │
│  CLI target                 Helm/Kustomize manifests            │
│                                                                  │
│  argocd-application-controller    argocd-applicationset-        │
│  ──────────────────────────────   controller                    │
│  Watches cluster state,           Generates Application         │
│  detects drift, syncs             resources from templates      │
│                                                                  │
│  argocd-dex-server          argocd-notifications-controller     │
│  ──────────────             ────────────────────────────────    │
│  SSO / OIDC federation      Slack, email, webhook alerts        │
│                                                                  │
│  argocd-redis                                                    │
│  ─────────────                                                   │
│  Shared cache for           argocd-notifications-controller     │
│  repo-server + app-         Sends alerts on sync events         │
│  controller                                                      │
└─────────────────────────────────────────────────────────────────┘
```

| Component | Responsibility |
|---|---|
| `argocd-server` | REST API, gRPC API, web UI — the interface layer |
| `argocd-repo-server` | Clones repositories, renders Helm/Kustomize/plain YAML, caches rendered manifests |
| `argocd-application-controller` | Compares desired state (Git) with live state (cluster), triggers syncs, manages health |
| `argocd-applicationset-controller` | Generates `Application` resources from `ApplicationSet` templates |
| `argocd-dex-server` | OIDC identity provider bridge for SSO (GitHub, Google, LDAP, Okta) |
| `argocd-notifications-controller` | Sends alerts to Slack, email, GitHub, PagerDuty on sync/health events |

---

## Step 5 — Connect a Git repository

ArgoCD needs credentials to fetch from private repositories. Public repositories require no credentials.

```bash
# Add a private HTTPS repository (username + PAT)
argocd repo add https://github.com/my-org/my-gitops-repo \
  --username git \
  --password <github-pat>

# Add a private SSH repository
argocd repo add git@github.com:my-org/my-gitops-repo.git \
  --ssh-private-key-path ~/.ssh/id_rsa

# Verify the connection
argocd repo list
# TYPE  NAME  REPO                                          INSECURE  STATUS      MESSAGE
# git         https://github.com/my-org/my-gitops-repo     false     Successful
```

Credentials are stored as Kubernetes Secrets in the `argocd` namespace. The repository is now available to all `Application` resources — you do not re-enter credentials per application.

---

## Step 6 — Create your first Application

An ArgoCD `Application` links a Git source path to a cluster destination:

```yaml
# my-app.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/my-org/my-gitops-repo
    targetRevision: main
    path: apps/my-app                # directory containing Kubernetes manifests
  destination:
    server: https://kubernetes.default.svc    # in-cluster
    namespace: my-app
  syncPolicy:
    syncOptions:
      - CreateNamespace=true         # create the namespace if it doesn't exist
```

Apply it:

```bash
kubectl apply -f my-app.yaml

# Or use the CLI
argocd app create my-app \
  --repo https://github.com/my-org/my-gitops-repo \
  --path apps/my-app \
  --dest-server https://kubernetes.default.svc \
  --dest-namespace my-app \
  --sync-option CreateNamespace=true
```

Check the application status:

```bash
argocd app get my-app
# Name:               argocd/my-app
# Project:            default
# Server:             https://kubernetes.default.svc
# Namespace:          my-app
# URL:                https://argocd.example.com/applications/my-app
# Repo:               https://github.com/my-org/my-gitops-repo
# Target:             main
# Path:               apps/my-app
# SyncWindow:         Sync Allowed
# Sync Policy:        <none>
# Sync Status:        OutOfSync from main (abc1234)
# Health Status:      Missing
```

The application is `OutOfSync` — it exists in Git but has not been applied to the cluster yet.

---

## Step 7 — Sync policies: manual vs automated

By default, ArgoCD detects drift but does not apply changes automatically — a human must trigger a sync. This is **manual sync**.

### Trigger a manual sync

```bash
argocd app sync my-app

# Sync with pruning (delete resources removed from Git)
argocd app sync my-app --prune

# Dry run — show what would be applied without applying it
argocd app sync my-app --dry-run
```

### Enable automated sync

Add `automated` to the sync policy for fully automatic reconciliation:

```yaml
spec:
  syncPolicy:
    automated:
      prune: true        # delete resources removed from Git
      selfHeal: true     # revert manual changes to the cluster
    syncOptions:
      - CreateNamespace=true
      - PrunePropagationPolicy=foreground
      - ApplyOutOfSyncOnly=true    # only apply changed resources, not all
```

With `automated` and `selfHeal: true`, ArgoCD behaves like Flux — any drift from the Git state is corrected automatically within the sync interval (default: 3 minutes).

`prune: true` enables deletion of resources that were removed from Git. Without it, ArgoCD will detect the resources as extra but leave them in place.

---

## Step 8 — Sync waves and hooks

For applications with ordering requirements (run a database migration before deploying the application), use sync waves and resource hooks:

### Sync waves

Assign a wave number to resources — ArgoCD applies wave 0 first, waits for all wave-0 resources to become healthy, then applies wave 1, and so on:

```yaml
# database-migration-job.yaml
metadata:
  annotations:
    argocd.argoproj.io/sync-wave: "0"    # run first
---
# deployment.yaml
metadata:
  annotations:
    argocd.argoproj.io/sync-wave: "1"    # run after migration succeeds
```

### Sync hooks

Hooks run at specific points in the sync lifecycle:

```yaml
# pre-sync-job.yaml
metadata:
  annotations:
    argocd.argoproj.io/hook: PreSync          # runs before sync begins
    argocd.argoproj.io/hook-delete-policy: HookSucceeded    # delete after success
spec:
  # ... Job spec for database migration
```

Available hook phases: `PreSync`, `Sync`, `PostSync`, `SyncFail`. `PostSync` is useful for smoke tests — if the PostSync Job fails, ArgoCD marks the sync as failed and can trigger a rollback.

---

## Step 9 — Health checks and resource customisation

ArgoCD has built-in health assessments for standard Kubernetes resources (Deployment, StatefulSet, DaemonSet, Job, Ingress). For custom resources, define a Lua health check:

```yaml
# argocd-cm ConfigMap patch
data:
  resource.customizations.health.my-operator_MyResource: |
    hs = {}
    if obj.status ~= nil then
      if obj.status.phase == "Running" then
        hs.status = "Healthy"
        hs.message = "MyResource is running"
        return hs
      end
      if obj.status.phase == "Failed" then
        hs.status = "Degraded"
        hs.message = obj.status.message
        return hs
      end
    end
    hs.status = "Progressing"
    hs.message = "Waiting for MyResource to start"
    return hs
```

ArgoCD uses this Lua script to evaluate the health of `MyResource` objects. The `status` field is surfaced in the UI and CLI.

---

## Step 10 — RBAC and Projects

ArgoCD `Projects` scope what a team can deploy and where. Without Projects, the `default` project allows deploying to any cluster and any namespace.

### Create a Project for the payments team

```yaml
# payments-project.yaml
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: payments
  namespace: argocd
spec:
  description: "Payments team applications"
  sourceRepos:
    - https://github.com/my-org/payments-repo    # only this repo
  destinations:
    - namespace: "payments-*"                     # only namespaces matching this pattern
      server: https://kubernetes.default.svc
  clusterResourceWhitelist:
    - group: ""
      kind: Namespace                             # allow creating Namespaces
  namespaceResourceBlacklist:
    - group: ""
      kind: ResourceQuota                        # cannot modify ResourceQuotas
```

### Configure RBAC

ArgoCD RBAC is defined in the `argocd-rbac-cm` ConfigMap using Casbin policy syntax:

```yaml
# argocd-rbac-cm ConfigMap
data:
  policy.csv: |
    # Payments team members: full access to payments project
    p, role:payments-dev, applications, *, payments/*, allow
    p, role:payments-dev, repositories, get, *, allow

    # Platform team: admin access everywhere
    p, role:platform-admin, *, *, *, allow

    # Bind GitHub team to ArgoCD role
    g, my-org:payments-team, role:payments-dev
    g, my-org:platform-team, role:platform-admin

  policy.default: role:readonly    # unauthenticated users get read-only view
```

With this configuration, payments team members can only deploy from their repository into `payments-*` namespaces. They cannot touch other teams' applications or cluster-wide resources.

---

## Step 11 — SSO with GitHub

Connect ArgoCD to GitHub OAuth for team-based login:

```bash
# Register an OAuth App in GitHub:
# Settings → Developer settings → OAuth Apps → New OAuth App
# Homepage URL: https://argocd.example.com
# Authorization callback URL: https://argocd.example.com/api/dex/callback
```

```yaml
# argocd-cm ConfigMap
data:
  url: https://argocd.example.com
  dex.config: |
    connectors:
      - type: github
        id: github
        name: GitHub
        config:
          clientID: <github-oauth-client-id>
          clientSecret: $dex.github.clientSecret    # reference a Secret key
          orgs:
            - name: my-org
```

```bash
# Store the client secret
kubectl create secret generic argocd-secret \
  --namespace argocd \
  --from-literal=dex.github.clientSecret=<oauth-client-secret> \
  --dry-run=client -o yaml | kubectl apply -f -
```

After restarting `argocd-dex-server` and `argocd-server`, the login page presents a "Log in with GitHub" button. Users authenticate via GitHub and ArgoCD maps their organisation membership to RBAC roles via the `g, my-org:team-name, role:...` bindings.

---

## What you have built

- ArgoCD installed from stable manifests with HA option for production
- CLI and web UI access via port-forward, with ingress configuration for persistent access
- The ArgoCD component architecture: server, repo-server, application-controller, applicationset-controller, dex, notifications
- Git repository connection with HTTPS PAT and SSH key options
- An `Application` resource connecting a Git source path to a cluster destination
- Manual sync and automated sync with `prune` and `selfHeal`
- Sync waves and resource hooks for ordered deployment (pre-sync migrations, post-sync smoke tests)
- Lua-based custom health checks for CRDs
- `AppProject` scoping source repositories and destination namespaces per team
- Casbin RBAC policy binding GitHub organisation teams to ArgoCD roles
- SSO via GitHub OAuth and Dex for team-based authentication

In [Part 2](/tutorials/gitops/managing-multi-environment-deployments-argocd-applicationsets-part-2/) you will use ArgoCD `ApplicationSet` resources to manage multi-environment and multi-cluster deployments from a single template — eliminating the need to maintain one `Application` manifest per environment per application.
