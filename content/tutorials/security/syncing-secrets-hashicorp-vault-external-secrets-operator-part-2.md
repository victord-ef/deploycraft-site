---
title: "Syncing Secrets from HashiCorp Vault Using the External Secrets Operator — Part 2"
date: 2026-09-01
description: "Install the External Secrets Operator, connect it to HashiCorp Vault, and sync secrets into Kubernetes automatically — eliminating static secrets from your cluster configuration."
cluster: "Security & Hardening"
series: "Secrets"
part: 2
difficulty: "intermediate"
duration: "45 min"
tags: ["kubernetes", "secrets", "vault", "external-secrets", "security", "devsecops", "hashicorp"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

In [Part 1](/tutorials/security/encrypting-kubernetes-secrets-at-rest-part-1/) you encrypted Kubernetes Secrets at rest in etcd. In Part 2 you will eliminate static Kubernetes Secrets from your workload configuration entirely. Using the External Secrets Operator (ESO) and HashiCorp Vault, secrets are stored in Vault and synced into Kubernetes Secrets automatically — with automatic refresh when the Vault value changes.

## Prerequisites

- Completed [Part 1](/tutorials/security/encrypting-kubernetes-secrets-at-rest-part-1/) or encryption at rest enabled via another method
- A running HashiCorp Vault instance (self-hosted or HCP Vault), or ability to deploy Vault in-cluster for this tutorial
- `kubectl` with cluster-admin access
- `helm` v3 installed
- Vault CLI installed

---

## Why External Secrets Operator

Kubernetes Secrets are a delivery mechanism, not a secret store. The problem with managing secrets natively in Kubernetes:

- Secrets must be created before workloads that reference them — creating a chicken-and-egg bootstrapping problem in GitOps
- Rotating a secret requires updating the Kubernetes Secret manually or through a script
- Secret values end up in GitOps repositories (even as Sealed Secrets or SOPS), creating another store to secure

The External Secrets Operator decouples secret storage from secret delivery:

- Secrets live in Vault (or AWS Secrets Manager, GCP Secret Manager, Azure Key Vault, etc.)
- ESO watches `ExternalSecret` objects and syncs values into Kubernetes Secrets automatically
- When the Vault value changes, ESO refreshes the Kubernetes Secret on the next sync interval
- No secret values appear in Git

ESO introduces three objects:

| Object | Scope | Purpose |
|---|---|---|
| `SecretStore` | Namespace | Connection to a secret backend for a single namespace |
| `ClusterSecretStore` | Cluster-wide | Connection to a secret backend usable from any namespace |
| `ExternalSecret` | Namespace | Maps keys from the backend into a Kubernetes Secret |

---

## Step 1 — Deploy Vault in-cluster (optional)

If you do not have an existing Vault instance, deploy it in-cluster for this tutorial:

```bash
helm repo add hashicorp https://helm.releases.hashicorp.com
helm repo update

helm install vault hashicorp/vault \
  --namespace vault \
  --create-namespace \
  --set server.dev.enabled=true \
  --set server.dev.devRootToken=root
```

> `dev` mode starts Vault unsealed with a root token. **Do not use dev mode in production.**

Wait for Vault to be ready:

```bash
kubectl get pods -n vault -w
```

Set up the Vault CLI to talk to the in-cluster instance:

```bash
kubectl port-forward -n vault svc/vault 8200:8200 &
export VAULT_ADDR=http://127.0.0.1:8200
export VAULT_TOKEN=root
vault status
```

---

## Step 2 — Configure Vault secrets and Kubernetes auth

Enable the KV secrets engine and create some secrets:

```bash
# Enable KV v2 secrets engine
vault secrets enable -path=secret kv-v2

# Write secrets for a sample application
vault kv put secret/myapp/database \
  username=app_user \
  password=Str0ngPassw0rd! \
  host=postgres.default.svc.cluster.local

vault kv put secret/myapp/api \
  stripe_key=sk_live_abc123 \
  sendgrid_key=SG.xyz789
```

Enable the Kubernetes auth method so ESO can authenticate to Vault using its ServiceAccount token:

```bash
vault auth enable kubernetes

# Configure the Kubernetes auth method
vault write auth/kubernetes/config \
  kubernetes_host="https://kubernetes.default.svc.cluster.local:443"
```

Create a Vault policy that grants read access to the application secrets:

```bash
vault policy write myapp-readonly - <<EOF
path "secret/data/myapp/*" {
  capabilities = ["read"]
}
EOF
```

Create a Vault role that binds the policy to a Kubernetes ServiceAccount:

```bash
vault write auth/kubernetes/role/external-secrets \
  bound_service_account_names=external-secrets \
  bound_service_account_namespaces=external-secrets \
  policies=myapp-readonly \
  ttl=1h
```

---

## Step 3 — Install the External Secrets Operator

```bash
helm repo add external-secrets https://charts.external-secrets.io
helm repo update

helm install external-secrets external-secrets/external-secrets \
  --namespace external-secrets \
  --create-namespace \
  --version 0.10.0
```

Verify all pods are running:

```bash
kubectl get pods -n external-secrets
```

Expected output:

```
NAME                                            READY   STATUS
external-secrets-xxxxxxxxx-xxxxx               1/1     Running
external-secrets-cert-controller-xxx-xxxxx     1/1     Running
external-secrets-webhook-xxxxxxxxx-xxxxx       1/1     Running
```

Confirm the CRDs are installed:

```bash
kubectl get crd | grep external-secrets.io
```

---

## Step 4 — Create a ClusterSecretStore for Vault

A `ClusterSecretStore` defines how ESO connects to Vault. Using Kubernetes auth, ESO authenticates to Vault using its own ServiceAccount token:

```yaml
# vault-cluster-secret-store.yaml
apiVersion: external-secrets.io/v1beta1
kind: ClusterSecretStore
metadata:
  name: vault-backend
spec:
  provider:
    vault:
      server: "http://vault.vault.svc.cluster.local:8200"
      path: "secret"
      version: "v2"
      auth:
        kubernetes:
          mountPath: "kubernetes"
          role: "external-secrets"
          serviceAccountRef:
            name: external-secrets
            namespace: external-secrets
```

```bash
kubectl apply -f vault-cluster-secret-store.yaml
kubectl get clustersecretstore vault-backend
```

Check the status — it should show `Valid`:

```bash
kubectl get clustersecretstore vault-backend \
  -o jsonpath='{.status.conditions[0].message}'
```

---

## Step 5 — Create an ExternalSecret

An `ExternalSecret` maps keys from Vault into a Kubernetes Secret. Create one that syncs the database credentials:

```yaml
# database-external-secret.yaml
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: database-credentials
  namespace: default
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: vault-backend
    kind: ClusterSecretStore
  target:
    name: database-credentials
    creationPolicy: Owner
    deletionPolicy: Retain
  data:
    - secretKey: username
      remoteRef:
        key: myapp/database
        property: username
    - secretKey: password
      remoteRef:
        key: myapp/database
        property: password
    - secretKey: host
      remoteRef:
        key: myapp/database
        property: host
```

```bash
kubectl apply -f database-external-secret.yaml
kubectl get externalsecret database-credentials -n default
```

Watch the sync status:

```bash
kubectl get externalsecret database-credentials -n default \
  -o jsonpath='{.status.conditions}'  | jq .
```

A healthy `ExternalSecret` shows `SecretSynced: True`. Check the resulting Kubernetes Secret:

```bash
kubectl get secret database-credentials -n default
kubectl get secret database-credentials -n default \
  -o jsonpath='{.data.username}' | base64 -d
```

The Secret is owned by the `ExternalSecret` — if the `ExternalSecret` is deleted, the Secret is also deleted (controlled by `deletionPolicy: Retain` to keep the Secret if you want it preserved after ESO removal).

---

## Step 6 — Sync an entire Vault path with dataFrom

Instead of mapping individual keys, use `dataFrom` to sync all keys from a Vault path into a single Kubernetes Secret:

```yaml
# api-keys-external-secret.yaml
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: api-keys
  namespace: default
spec:
  refreshInterval: 30m
  secretStoreRef:
    name: vault-backend
    kind: ClusterSecretStore
  target:
    name: api-keys
    creationPolicy: Owner
  dataFrom:
    - extract:
        key: myapp/api
```

```bash
kubectl apply -f api-keys-external-secret.yaml
kubectl get secret api-keys -n default -o jsonpath='{.data}' | jq 'keys'
```

The resulting Secret contains `stripe_key` and `sendgrid_key` as keys — all properties from `secret/data/myapp/api` are synced automatically. When you add a new key to the Vault path, it appears in the Kubernetes Secret on the next refresh.

---

## Step 7 — Use synced secrets in a workload

Reference the ESO-managed Secret in a Deployment exactly as you would any Kubernetes Secret:

```yaml
# app-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
    spec:
      automountServiceAccountToken: false
      containers:
        - name: app
          image: myapp:latest
          env:
            - name: DB_USERNAME
              valueFrom:
                secretKeyRef:
                  name: database-credentials
                  key: username
            - name: DB_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: database-credentials
                  key: password
            - name: DB_HOST
              valueFrom:
                secretKeyRef:
                  name: database-credentials
                  key: host
```

The Deployment references the Kubernetes Secret by name — it has no knowledge of Vault. ESO is transparent to the workload.

---

## Step 8 — Rotate a secret in Vault and verify sync

Update the database password in Vault:

```bash
vault kv put secret/myapp/database \
  username=app_user \
  password=NewStr0ngPassw0rd! \
  host=postgres.default.svc.cluster.local
```

Force an immediate sync without waiting for `refreshInterval`:

```bash
# Annotate the ExternalSecret to trigger immediate refresh
kubectl annotate externalsecret database-credentials \
  force-sync=$(date +%s) \
  --overwrite \
  -n default
```

Verify the Secret has been updated:

```bash
kubectl get secret database-credentials -n default \
  -o jsonpath='{.data.password}' | base64 -d
# Output: NewStr0ngPassw0rd!
```

The Kubernetes Secret is updated without touching any Kubernetes configuration. The workload picks up the new value on the next Secret read (or pod restart, depending on how it mounts the Secret — refer to Part 1 of the Certificates series for the volume mount refresh behaviour).

---

## Step 9 — Namespace-scoped SecretStore

For cases where different namespaces connect to different Vault paths or use different auth roles, use a namespace-scoped `SecretStore` instead of a `ClusterSecretStore`:

```yaml
# namespace-secret-store.yaml
apiVersion: external-secrets.io/v1beta1
kind: SecretStore
metadata:
  name: vault-backend
  namespace: production
spec:
  provider:
    vault:
      server: "http://vault.vault.svc.cluster.local:8200"
      path: "secret"
      version: "v2"
      auth:
        kubernetes:
          mountPath: "kubernetes"
          role: "production-secrets"
          serviceAccountRef:
            name: production-sa
            namespace: production
```

A `SecretStore` can only be referenced by `ExternalSecret` objects in the same namespace. This enforces namespace-level access boundaries — a team in `production` cannot reference a `SecretStore` from `development`.

---

## Step 10 — Monitor ESO sync status

ESO exposes Prometheus metrics. Key metrics for secret sync health:

```promql
# Sync failures by ExternalSecret
externalsecret_sync_calls_total{status="error"}

# Time since last successful sync
time() - externalsecret_status_condition_last_transition_time{condition="SecretSynced", status="True"}

# Count of ExternalSecrets not synced
count(externalsecret_status_condition{condition="SecretSynced", status="False"})
```

Alert when an `ExternalSecret` has not synced successfully within twice its `refreshInterval`:

```yaml
- alert: ExternalSecretSyncFailed
  expr: |
    externalsecret_sync_calls_total{status="error"} > 0
  for: 15m
  labels:
    severity: warning
  annotations:
    summary: "ExternalSecret sync failing"
    description: >
      ExternalSecret {{ $labels.name }} in namespace {{ $labels.namespace }}
      has failed to sync from the secret backend.
```

---

## What you have built

- HashiCorp Vault configured with KV v2 and Kubernetes auth method
- External Secrets Operator installed with a `ClusterSecretStore` connecting to Vault
- `ExternalSecret` objects syncing individual keys and full Vault paths into Kubernetes Secrets
- Secret rotation in Vault propagated to Kubernetes without touching cluster config
- Namespace-scoped `SecretStore` for multi-tenant secret isolation
- Prometheus alerting for sync failures

Combined with encryption at rest from Part 1, secrets in your cluster are now both encrypted in etcd and sourced dynamically from a dedicated secret store — eliminating static secret management from your Kubernetes configuration entirely.
