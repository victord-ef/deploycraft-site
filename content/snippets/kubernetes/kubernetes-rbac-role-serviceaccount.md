---
title: "Kubernetes RBAC — Role, ClusterRole, and ServiceAccount Bindings"
date: 2026-11-12
description: "Namespace-scoped Role and cluster-scoped ClusterRole manifests with RoleBindings, bound to a dedicated ServiceAccount — covering read-only access, read-write deployment access, and cross-namespace binding patterns."
lang: "Kubernetes"
tags: ["kubernetes", "rbac", "role", "clusterrole", "serviceaccount", "security"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-11-12"
draft: false
---

## When to use this

Any time a workload, CI pipeline, or human user needs access to the Kubernetes API. The default ServiceAccount in every namespace has no permissions — but `default` is shared by all pods that do not specify otherwise, so any permissions added to it are inherited by every pod in the namespace. Always create a dedicated ServiceAccount per workload.

## Without it

```yaml
# Common anti-pattern — adding permissions to the default ServiceAccount
# Every pod in the namespace now has these permissions
subjects:
  - kind: ServiceAccount
    name: default
    namespace: my-app
```

## Snippet

### Read-only access — view pods, logs, and events in one namespace

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: app-reader
  namespace: my-app

---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: pod-reader
  namespace: my-app
rules:
  - apiGroups: [""]
    resources: ["pods", "pods/log", "events", "configmaps"]
    verbs: ["get", "list", "watch"]

---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: app-reader-binding
  namespace: my-app
subjects:
  - kind: ServiceAccount
    name: app-reader
    namespace: my-app
roleRef:
  kind: Role
  name: pod-reader
  apiGroup: rbac.authorization.k8s.io
```

### Read-write deployment access — CI pipeline ServiceAccount

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ci-deployer
  namespace: my-app

---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: deployer
  namespace: my-app
rules:
  - apiGroups: ["apps"]
    resources: ["deployments", "replicasets"]
    verbs: ["get", "list", "watch", "create", "update", "patch"]
  - apiGroups: [""]
    resources: ["services", "configmaps", "serviceaccounts"]
    verbs: ["get", "list", "watch", "create", "update", "patch"]
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["get", "list"]          # list only — never grant create/update on secrets to CI
  - apiGroups: ["networking.k8s.io"]
    resources: ["ingresses"]
    verbs: ["get", "list", "watch", "create", "update", "patch"]

---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: ci-deployer-binding
  namespace: my-app
subjects:
  - kind: ServiceAccount
    name: ci-deployer
    namespace: my-app
roleRef:
  kind: Role
  name: deployer
  apiGroup: rbac.authorization.k8s.io
```

### ClusterRole — read-only access to cluster-wide resources

```yaml
# Use ClusterRole + ClusterRoleBinding for cluster-scoped resources
# (Nodes, PersistentVolumes, Namespaces) or cross-namespace access
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: node-reader
rules:
  - apiGroups: [""]
    resources: ["nodes", "persistentvolumes", "namespaces"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["storage.k8s.io"]
    resources: ["storageclasses"]
    verbs: ["get", "list", "watch"]

---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: monitoring-node-reader
subjects:
  - kind: ServiceAccount
    name: prometheus
    namespace: monitoring
roleRef:
  kind: ClusterRole
  name: node-reader
  apiGroup: rbac.authorization.k8s.io
```

### ClusterRole bound to a namespace (cross-namespace read access)

```yaml
# ClusterRole + RoleBinding (not ClusterRoleBinding) scopes the ClusterRole
# to a single namespace — useful for reusing a role definition across namespaces
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: pod-reader-from-clusterrole
  namespace: team-b          # scoped to this namespace only
subjects:
  - kind: ServiceAccount
    name: dashboard
    namespace: monitoring
roleRef:
  kind: ClusterRole           # references a ClusterRole but scoped by RoleBinding
  name: view                  # built-in read-only ClusterRole
  apiGroup: rbac.authorization.k8s.io
```

### Pod spec — reference the dedicated ServiceAccount

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: my-app
spec:
  template:
    spec:
      serviceAccountName: app-reader   # explicit — never rely on 'default'
      automountServiceAccountToken: false   # disable if the app doesn't call the API
      containers:
        - name: app
          image: my-app:1.0.0
```

**Key decisions:**

| Decision | Why it matters |
|---|---|
| Dedicated ServiceAccount per workload | A unique ServiceAccount gives each workload its own identity in audit logs and RBAC policies. `default` is shared — any permission added to it applies to every pod that hasn't explicitly opted out. |
| `automountServiceAccountToken: false` | By default, Kubernetes mounts a service account token into every pod at `/var/run/secrets/kubernetes.io/serviceaccount/token`. If the application never calls the API, disable automounting to remove a credential that could be exfiltrated. |
| `verbs: ["get", "list"]` on secrets, never `create` | A CI ServiceAccount that can create Secrets can write arbitrary credentials into the namespace — effectively privilege escalation. Read-only access to existing secrets is the most that CI typically needs. |
| `Role` + `RoleBinding` over `ClusterRole` + `ClusterRoleBinding` | Namespace-scoped roles limit blast radius. A ClusterRoleBinding grants access to that resource type across every namespace and future namespaces — including ones that don't exist yet. |
| `ClusterRole` + `RoleBinding` for reuse | A ClusterRole can be bound in a specific namespace via a RoleBinding. This lets you define a role once and reuse it across namespaces without duplicating the YAML — the scope is set by which binding type you use. |

## Verify it worked

```bash
# Confirm what permissions the ServiceAccount has
kubectl auth can-i list pods \
  --as=system:serviceaccount:my-app:app-reader \
  -n my-app
# Expected: yes

kubectl auth can-i delete pods \
  --as=system:serviceaccount:my-app:app-reader \
  -n my-app
# Expected: no

# Full permission audit for a ServiceAccount
kubectl auth can-i --list \
  --as=system:serviceaccount:my-app:ci-deployer \
  -n my-app
```
