---
title: "Defining Custom Resource Definitions (CRDs) in Kubernetes — Part 1"
date: 2026-09-01
description: "Extend the Kubernetes API with your own resource types. Define a CRD with a schema, create custom resources, add printer columns, and understand how CRDs integrate with kubectl and the control plane."
cluster: "Kubernetes"
series: "CRD Fundamentals"
part: 1
difficulty: "intermediate"
duration: "45 min"
tags: ["kubernetes", "crd", "custom-resources", "api-extension", "platform-engineering", "go"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

By the end of this tutorial you will have a working Custom Resource Definition (CRD) deployed in your cluster: a `Database` resource that application teams use to request managed database instances. You will define its schema with structural validation, create custom resources with `kubectl`, add custom printer columns, and understand how CRDs extend the Kubernetes API. Part 2 adds CEL-based validation expressions that enforce business rules directly in the API server.

## Prerequisites

- A Kubernetes cluster (v1.25+) with cluster-admin access
- `kubectl` configured
- Basic familiarity with Kubernetes Deployments and Services

---

## What CRDs are and why they matter

Kubernetes ships with built-in resources: Pods, Deployments, Services, ConfigMaps. CRDs let you add your own resource types to the Kubernetes API — resources that look and behave exactly like built-in ones but represent your own domain concepts.

Once you define a `Database` CRD, users can:

```bash
kubectl apply -f database.yaml
kubectl get databases
kubectl describe database prod-postgres
kubectl delete database staging-mysql
```

All standard `kubectl` operations work, RBAC applies, and the API server stores, validates, and serves your custom resources using the same machinery as built-in types. CRDs are the foundation of every Kubernetes operator and platform API.

---

## Step 1 — Define your first CRD

A CRD is a Kubernetes object that registers a new API resource. Define a `Database` CRD in the `platform.example.com` API group:

```yaml
# database-crd.yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: databases.platform.example.com    # must be <plural>.<group>
spec:
  group: platform.example.com
  scope: Namespaced                        # or Cluster for cluster-scoped resources
  names:
    plural: databases
    singular: database
    kind: Database
    shortNames:
      - db
  versions:
    - name: v1alpha1
      served: true       # this version is served by the API server
      storage: true      # this is the version stored in etcd
      schema:
        openAPIV3Schema:
          type: object
          required: [spec]
          properties:
            spec:
              type: object
              required: [engine, version, storageGB]
              properties:
                engine:
                  type: string
                  enum: [postgres, mysql, mariadb]
                  description: "Database engine to use"
                version:
                  type: string
                  description: "Engine version (e.g. 15.3)"
                storageGB:
                  type: integer
                  minimum: 1
                  maximum: 16384
                  description: "Storage in gigabytes"
                replicas:
                  type: integer
                  minimum: 1
                  maximum: 5
                  default: 1
                  description: "Number of read replicas"
                backupEnabled:
                  type: boolean
                  default: true
            status:
              type: object
              properties:
                phase:
                  type: string
                  enum: [Pending, Provisioning, Ready, Failed, Terminating]
                endpoint:
                  type: string
                message:
                  type: string
                observedGeneration:
                  type: integer
                  format: int64
```

```bash
kubectl apply -f database-crd.yaml
```

Verify the CRD is registered:

```bash
kubectl get crd databases.platform.example.com
# NAME                              CREATED AT
# databases.platform.example.com   2026-09-01T10:00:00Z
```

The API server now serves the `databases` resource at `/apis/platform.example.com/v1alpha1/databases`.

---

## Step 2 — Create a custom resource

With the CRD installed, create a `Database` instance:

```yaml
# my-database.yaml
apiVersion: platform.example.com/v1alpha1
kind: Database
metadata:
  name: prod-postgres
  namespace: default
  labels:
    team: backend
    env: production
spec:
  engine: postgres
  version: "15.3"
  storageGB: 100
  replicas: 2
  backupEnabled: true
```

```bash
kubectl apply -f my-database.yaml
kubectl get database prod-postgres
# NAME            AGE
# prod-postgres   5s
```

Use the short name:

```bash
kubectl get db
# NAME            AGE
# prod-postgres   12s
```

Try creating an invalid resource to see schema validation in action:

```bash
kubectl apply -f - <<EOF
apiVersion: platform.example.com/v1alpha1
kind: Database
metadata:
  name: invalid-db
  namespace: default
spec:
  engine: oracle       # not in enum
  version: "19c"
  storageGB: 50
EOF
# Error: The Database "invalid-db" is invalid:
#   spec.engine: Unsupported value: "oracle": supported values: "postgres", "mysql", "mariadb"
```

The API server enforces the schema before storing the object — no controller required for basic validation.

---

## Step 3 — Add printer columns

By default, `kubectl get databases` only shows `NAME` and `AGE`. Additional printer columns let you display spec fields directly in the list output.

Add `additionalPrinterColumns` to the CRD version:

```yaml
# database-crd-with-columns.yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: databases.platform.example.com
spec:
  group: platform.example.com
  scope: Namespaced
  names:
    plural: databases
    singular: database
    kind: Database
    shortNames:
      - db
  versions:
    - name: v1alpha1
      served: true
      storage: true
      additionalPrinterColumns:
        - name: Engine
          type: string
          jsonPath: .spec.engine
        - name: Version
          type: string
          jsonPath: .spec.version
        - name: Storage
          type: integer
          jsonPath: .spec.storageGB
          format: int32
        - name: Replicas
          type: integer
          jsonPath: .spec.replicas
        - name: Phase
          type: string
          jsonPath: .status.phase
        - name: Age
          type: date
          jsonPath: .metadata.creationTimestamp
      schema:
        # ... same schema as above
```

```bash
kubectl apply -f database-crd-with-columns.yaml
kubectl get databases
# NAME            ENGINE     VERSION   STORAGE   REPLICAS   PHASE   AGE
# prod-postgres   postgres   15.3      100       2          <none>  2m
```

The `Phase` column shows `<none>` because no controller has set `status.phase` yet — that is what operators do (see Pair 12).

---

## Step 4 — Understand the schema structure

The `openAPIV3Schema` in a CRD is a structural schema — it must describe every field that can appear in the resource. The rules:

**Required fields at the top level:**

```yaml
openAPIV3Schema:
  type: object
  required: [spec]        # metadata is implicit; status is optional
  properties:
    spec: ...
    status: ...
```

**Nested objects:**

```yaml
spec:
  type: object
  required: [engine, version, storageGB]
  properties:
    connectionPool:
      type: object
      properties:
        maxConnections:
          type: integer
          minimum: 1
          maximum: 1000
          default: 100
        mode:
          type: string
          enum: [transaction, session, statement]
          default: transaction
```

**Arrays:**

```yaml
spec:
  type: object
  properties:
    allowedCIDRs:
      type: array
      items:
        type: string
        pattern: '^([0-9]{1,3}\.){3}[0-9]{1,3}\/[0-9]{1,2}$'
      maxItems: 20
```

**Maps (additionalProperties):**

```yaml
spec:
  type: object
  properties:
    parameters:
      type: object
      additionalProperties:
        type: string    # free-form string map
```

**Defaulting** — the API server applies defaults at admission time, before storing the object. A user who omits `replicas` gets `1` stored in etcd without a controller needing to patch it.

---

## Step 5 — Subresources: status and scale

Enable the `status` subresource so controllers can update status independently from spec (important for RBAC and optimistic concurrency):

```yaml
versions:
  - name: v1alpha1
    served: true
    storage: true
    subresources:
      status: {}          # enables /status subresource
    schema:
      # ...
```

With the status subresource enabled:

- `kubectl apply` only updates `spec` (not `status`)
- Controllers use `UpdateStatus` to write `status` (separate API call, separate RBAC)
- The API server merges them independently, preventing controllers from accidentally overwriting user-applied spec changes

Enable the `scale` subresource to make your resource compatible with `kubectl scale` and HPA:

```yaml
subresources:
  status: {}
  scale:
    specReplicasPath: .spec.replicas
    statusReplicasPath: .status.readyReplicas
```

```bash
kubectl scale database prod-postgres --replicas=3
# database.platform.example.com/prod-postgres scaled
```

---

## Step 6 — Multiple versions and conversion

CRDs support multiple API versions served simultaneously. This lets you evolve the schema while maintaining backward compatibility:

```yaml
versions:
  - name: v1alpha1
    served: true
    storage: false     # not storage version anymore
    schema:
      # ... v1alpha1 schema

  - name: v1beta1
    served: true
    storage: true      # new storage version
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              engine:
                type: string
              version:
                type: string
              storage:
                type: object          # v1beta1 restructured storageGB into an object
                properties:
                  sizeGB:
                    type: integer
                  storageClass:
                    type: string
```

When multiple versions are served, you need a conversion webhook to translate between them. For schema-compatible changes (adding optional fields), Kubernetes handles `None` conversion automatically — no webhook needed.

```yaml
conversion:
  strategy: None    # for compatible changes; use Webhook for breaking changes
```

---

## Step 7 — RBAC for custom resources

Custom resources use the same RBAC system as built-in resources. Grant access using the CRD's group and resource name:

```yaml
# developer-db-role.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: database-developer
  namespace: default
rules:
  - apiGroups: [platform.example.com]
    resources: [databases]
    verbs: [get, list, watch, create, update, patch]
  - apiGroups: [platform.example.com]
    resources: [databases/status]
    verbs: [get]      # developers can read status but not write it
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: database-operator
rules:
  - apiGroups: [platform.example.com]
    resources: [databases]
    verbs: [get, list, watch, create, update, patch, delete]
  - apiGroups: [platform.example.com]
    resources: [databases/status]
    verbs: [get, update, patch]    # operators can write status
  - apiGroups: [platform.example.com]
    resources: [databases/finalizers]
    verbs: [update]                # operators manage finalizers
```

```bash
kubectl apply -f developer-db-role.yaml
kubectl auth can-i create databases --as=system:serviceaccount:default:developer -n default
```

---

## Step 8 — Inspect the CRD API

Explore how your CRD integrates with the Kubernetes API:

```bash
# Show all API resources including your CRD
kubectl api-resources | grep platform.example.com
# databases   db   platform.example.com/v1alpha1   true   Database

# Describe the full CRD schema
kubectl explain database.spec
# KIND:       Database
# VERSION:    platform.example.com/v1alpha1
# FIELD:      spec <Object>
# FIELDS:
#   engine            <string> -required-
#   version           <string> -required-
#   storageGB         <integer> -required-
#   replicas          <integer>
#   backupEnabled     <boolean>

kubectl explain database.spec.engine
# FIELD:  engine <string>
#   Database engine to use

# Dump the full OpenAPI schema for your CRD
kubectl get --raw /openapi/v2 | jq '.definitions["platform.example.com.v1alpha1.Database"]'
```

The OpenAPI schema from your CRD is what powers `kubectl explain`, client code generation, and IDE autocomplete for your custom resources.

---

## What you have built

- A `Database` CRD in the `platform.example.com` API group with structural schema validation
- Enum constraints, integer bounds, and defaults enforced at admission time by the API server
- Custom printer columns showing engine, version, storage, replicas, and phase in `kubectl get`
- Status and scale subresources for controller-safe status writes and HPA compatibility
- Multi-version scaffolding with `served`/`storage` version flags
- RBAC roles separating developer access from operator access on spec vs status

In [Part 2](/tutorials/kubernetes/validating-custom-resources-cel-expressions-part-2/) you will extend this CRD with CEL validation expressions — rules like "replica count must be odd for quorum" and "version must be compatible with engine" — enforced directly by the API server without a webhook.
