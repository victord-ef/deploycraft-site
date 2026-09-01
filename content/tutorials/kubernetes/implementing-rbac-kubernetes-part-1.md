---
title: "Implementing RBAC in Kubernetes — Part 1"
date: 2026-09-01
description: "Define Roles, ClusterRoles, RoleBindings, and ClusterRoleBindings to control who can do what in your Kubernetes cluster. Build a least-privilege RBAC model from scratch."
cluster: "Kubernetes"
series: "RBAC"
part: 1
difficulty: "intermediate"
duration: "40 min"
tags: ["kubernetes", "rbac", "security", "authorization", "devsecops"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

By the end of this tutorial you will have a working RBAC model in Kubernetes: namespaced Roles and ClusterRoles, RoleBindings that grant access to specific service accounts and users, and a verified least-privilege setup for two common personas — a developer and a CI/CD pipeline service account. Part 2 builds on this by auditing what you've built and tightening permissions using `kubectl-who-can`.

## Prerequisites

- A running Kubernetes cluster (v1.25+)
- `kubectl` configured with cluster-admin permissions
- Basic familiarity with Kubernetes namespaces and workloads

---

## How Kubernetes RBAC works

Kubernetes RBAC controls access to the API server. Every request — whether from a human user, a service account, or a controller — passes through the authorisation layer. RBAC answers one question: is this subject allowed to perform this verb on this resource?

The model has four objects:

| Object | Scope | Purpose |
|---|---|---|
| `Role` | Namespace | Grants permissions within a single namespace |
| `ClusterRole` | Cluster-wide | Grants permissions across all namespaces, or for cluster-scoped resources |
| `RoleBinding` | Namespace | Binds a Role or ClusterRole to a subject within a namespace |
| `ClusterRoleBinding` | Cluster-wide | Binds a ClusterRole to a subject across the entire cluster |

Subjects are: `User`, `Group`, or `ServiceAccount`.

The key rule: **permissions are additive**. There is no `deny`. A subject gets the union of all permissions granted by all bindings that reference them. This makes RBAC accumulation — where permissions grow over time without cleanup — one of the most common misconfigurations in production clusters.

---

## Step 1 — Create namespaces for the exercise

```bash
kubectl create namespace development
kubectl create namespace production
```

All developer access in this tutorial is scoped to `development`. The `production` namespace will demonstrate what happens when access is not granted.

---

## Step 2 — Create a developer Role

A `Role` defines a set of permissions within a namespace. Create a role that gives developers read access to most resources and the ability to exec into pods — a realistic developer access level.

```yaml
# developer-role.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: developer
  namespace: development
rules:
  - apiGroups: [""]
    resources: ["pods", "pods/log", "services", "configmaps", "endpoints"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["pods/exec"]
    verbs: ["create"]
  - apiGroups: ["apps"]
    resources: ["deployments", "replicasets", "statefulsets", "daemonsets"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["batch"]
    resources: ["jobs", "cronjobs"]
    verbs: ["get", "list", "watch"]
```

```bash
kubectl apply -f developer-role.yaml
kubectl get role developer -n development
```

A few deliberate decisions in this Role:

- **No `create`, `update`, `delete` on Deployments** — developers can observe workloads but cannot modify them in this model. Deployment changes go through GitOps.
- **`pods/exec` is explicitly listed** — exec requires its own sub-resource permission. Many RBAC policies grant pod access but forget this, blocking `kubectl exec`.
- **No Secrets access** — secrets are excluded entirely. If developers need to read a specific secret, that is a separate, explicit Role.

---

## Step 3 — Create a ServiceAccount and bind the developer Role

Create a service account to represent a developer, then bind the Role to it with a `RoleBinding`.

```bash
kubectl create serviceaccount developer-sa -n development
```

```yaml
# developer-rolebinding.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: developer-binding
  namespace: development
subjects:
  - kind: ServiceAccount
    name: developer-sa
    namespace: development
roleRef:
  kind: Role
  name: developer
  apiGroup: rbac.authorization.k8s.io
```

```bash
kubectl apply -f developer-rolebinding.yaml
kubectl get rolebinding developer-binding -n development
```

---

## Step 4 — Verify the developer ServiceAccount permissions

Use `kubectl auth can-i` to verify what the service account can and cannot do:

```bash
# Should be allowed
kubectl auth can-i get pods \
  --namespace development \
  --as system:serviceaccount:development:developer-sa

# Should be allowed
kubectl auth can-i create pods/exec \
  --namespace development \
  --as system:serviceaccount:development:developer-sa

# Should be denied — no write access to deployments
kubectl auth can-i delete deployments \
  --namespace development \
  --as system:serviceaccount:development:developer-sa

# Should be denied — no access to production namespace
kubectl auth can-i get pods \
  --namespace production \
  --as system:serviceaccount:development:developer-sa

# Should be denied — no secrets access
kubectl auth can-i get secrets \
  --namespace development \
  --as system:serviceaccount:development:developer-sa
```

All five checks should return the expected result. If any allow when they should deny, the Role rules need tightening.

---

## Step 5 — Create a CI/CD pipeline ServiceAccount

A CI/CD pipeline typically needs to deploy workloads — it needs write access to Deployments, but nothing else. Create a tightly scoped Role for this.

```yaml
# cicd-role.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: cicd-deployer
  namespace: development
rules:
  - apiGroups: ["apps"]
    resources: ["deployments"]
    verbs: ["get", "list", "watch", "create", "update", "patch"]
  - apiGroups: [""]
    resources: ["services", "configmaps"]
    verbs: ["get", "list", "watch", "create", "update", "patch"]
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list", "watch"]
```

```bash
kubectl create serviceaccount cicd-sa -n development
kubectl apply -f cicd-role.yaml
```

```yaml
# cicd-rolebinding.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: cicd-deployer-binding
  namespace: development
subjects:
  - kind: ServiceAccount
    name: cicd-sa
    namespace: development
roleRef:
  kind: Role
  name: cicd-deployer
  apiGroup: rbac.authorization.k8s.io
```

```bash
kubectl apply -f cicd-rolebinding.yaml
```

Verify:

```bash
# Should be allowed — pipeline needs to deploy
kubectl auth can-i update deployments \
  --namespace development \
  --as system:serviceaccount:development:cicd-sa

# Should be denied — pipeline has no exec access
kubectl auth can-i create pods/exec \
  --namespace development \
  --as system:serviceaccount:development:cicd-sa

# Should be denied — pipeline cannot delete deployments
kubectl auth can-i delete deployments \
  --namespace development \
  --as system:serviceaccount:development:cicd-sa
```

---

## Step 6 — ClusterRole for read-only cluster visibility

Some personas — SREs, on-call engineers — need read-only visibility across all namespaces but should not be able to modify anything. A `ClusterRole` with a `ClusterRoleBinding` provides this.

```yaml
# readonly-clusterrole.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: cluster-reader
rules:
  - apiGroups: [""]
    resources:
      - pods
      - pods/log
      - services
      - endpoints
      - configmaps
      - namespaces
      - nodes
      - persistentvolumes
      - persistentvolumeclaims
    verbs: ["get", "list", "watch"]
  - apiGroups: ["apps"]
    resources: ["deployments", "replicasets", "statefulsets", "daemonsets"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["batch"]
    resources: ["jobs", "cronjobs"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["networking.k8s.io"]
    resources: ["ingresses", "networkpolicies"]
    verbs: ["get", "list", "watch"]
```

```bash
kubectl apply -f readonly-clusterrole.yaml
```

Bind it to a service account:

```bash
kubectl create serviceaccount sre-sa -n development
```

```yaml
# sre-clusterrolebinding.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: cluster-reader-binding
subjects:
  - kind: ServiceAccount
    name: sre-sa
    namespace: development
roleRef:
  kind: ClusterRole
  name: cluster-reader
  apiGroup: rbac.authorization.k8s.io
```

```bash
kubectl apply -f sre-clusterrolebinding.yaml
```

Verify cross-namespace visibility:

```bash
# Should be allowed — ClusterRoleBinding grants cluster-wide access
kubectl auth can-i get pods \
  --namespace production \
  --as system:serviceaccount:development:sre-sa

# Should be denied — read-only
kubectl auth can-i delete pods \
  --namespace production \
  --as system:serviceaccount:development:sre-sa
```

---

## Step 7 — Using built-in ClusterRoles

Kubernetes ships with several built-in ClusterRoles worth knowing:

| ClusterRole | Purpose |
|---|---|
| `view` | Read-only access to most resources in a namespace |
| `edit` | Read/write access to most resources, no RBAC or secrets |
| `admin` | Full namespace access including RBAC, but not cluster-scoped resources |
| `cluster-admin` | Full cluster access — treat as root |

You can bind these with a `RoleBinding` to scope them to a single namespace, or with a `ClusterRoleBinding` for cluster-wide access. The `view` ClusterRole is a safe starting point for developer read access that you can then extend with a supplementary Role.

```bash
# Bind built-in view ClusterRole to a namespace only
kubectl create rolebinding view-binding \
  --clusterrole=view \
  --serviceaccount=development:readonly-sa \
  --namespace=development
```

---

## Step 8 — RBAC for custom resources

If your cluster runs CRDs (Kyverno policies, Flux objects, Cert-manager certificates), RBAC for those resources follows the same pattern using the CRD's API group:

```yaml
# Allow reading Flux Kustomizations
- apiGroups: ["kustomize.toolkit.fluxcd.io"]
  resources: ["kustomizations"]
  verbs: ["get", "list", "watch"]

# Allow reading cert-manager Certificates
- apiGroups: ["cert-manager.io"]
  resources: ["certificates", "certificaterequests"]
  verbs: ["get", "list", "watch"]

# Allow managing Kyverno ClusterPolicies
- apiGroups: ["kyverno.io"]
  resources: ["clusterpolicies", "policies"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
```

Find the API group for any CRD:

```bash
kubectl get crd <crd-name> -o jsonpath='{.spec.group}'
# Example
kubectl get crd kustomizations.kustomize.toolkit.fluxcd.io \
  -o jsonpath='{.spec.group}'
# Output: kustomize.toolkit.fluxcd.io
```

---

## What you have built

- A `developer` Role with read and exec access scoped to one namespace
- A `cicd-deployer` Role with deployment write access only
- A `cluster-reader` ClusterRole with read-only visibility across all namespaces
- All permissions verified with `kubectl auth can-i`
- CRD RBAC patterns for extending access to custom resources

## Next steps

In [Part 2](/tutorials/kubernetes/auditing-rbac-kubectl-who-can-part-2/) you will install `kubectl-who-can`, audit the RBAC model you've built for over-permissioned subjects, identify which subjects can perform sensitive operations like `exec`, `delete`, and `escalate`, and apply targeted fixes to reach a tighter least-privilege posture.
