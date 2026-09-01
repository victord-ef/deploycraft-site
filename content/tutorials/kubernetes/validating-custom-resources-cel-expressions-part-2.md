---
title: "Validating Custom Resources with CEL Expressions in CRDs — Part 2"
date: 2026-09-01
description: "Enforce complex business rules in your CRDs using Common Expression Language (CEL) validation rules. Write cross-field constraints, immutability guards, and transition rules that the API server evaluates without a webhook."
cluster: "Kubernetes"
series: "CRD Fundamentals"
part: 2
difficulty: "advanced"
duration: "40 min"
tags: ["kubernetes", "crd", "cel", "validation", "custom-resources", "api-extension", "platform-engineering"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

In [Part 1](/tutorials/kubernetes/defining-custom-resource-definitions-kubernetes-part-1/) you defined a `Database` CRD with structural schema validation — type checking, enums, and integer bounds. In Part 2 you will add CEL (Common Expression Language) validation rules that enforce business logic directly in the API server: cross-field constraints, immutability rules, transition guards, and custom error messages. No admission webhook required.

## Prerequisites

- Completed [Part 1](/tutorials/kubernetes/defining-custom-resource-definitions-kubernetes-part-1/) — `Database` CRD deployed
- Kubernetes v1.25+ (CEL validation is GA in v1.25)

---

## Why CEL validation

OpenAPI v3 schema validation covers type checking, enums, and numeric bounds. It cannot express:

- Cross-field rules ("MySQL does not support more than 3 replicas")
- Immutability ("engine cannot be changed after creation")
- Transition rules ("replicas can only increase, never decrease")
- Conditional requirements ("backupEnabled must be true when replicas > 1")

Before Kubernetes 1.25, enforcing these required an admission webhook — a separate server the API server calls synchronously on every create/update. Webhooks add operational complexity: they must be highly available, they can fail and block admission, and they require TLS certificates.

CEL validation rules are embedded directly in the CRD schema. The API server evaluates them inline during admission — no extra service, no TLS, no availability concern.

---

## Step 1 — Add your first CEL rule

CEL rules go inside `x-kubernetes-validations` in the schema. Add a cross-field rule: MySQL supports a maximum of 3 replicas.

```yaml
# database-crd-cel.yaml
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
    shortNames: [db]
  versions:
    - name: v1alpha1
      served: true
      storage: true
      subresources:
        status: {}
      schema:
        openAPIV3Schema:
          type: object
          required: [spec]
          properties:
            spec:
              type: object
              required: [engine, version, storageGB]
              x-kubernetes-validations:
                - rule: >
                    !(self.engine == 'mysql' && self.replicas > 3)
                  message: "MySQL supports a maximum of 3 replicas"
                - rule: >
                    self.engine != 'mariadb' || self.version.startsWith('10.')
                  message: "MariaDB version must start with '10.' (e.g. 10.11)"
              properties:
                engine:
                  type: string
                  enum: [postgres, mysql, mariadb]
                version:
                  type: string
                storageGB:
                  type: integer
                  minimum: 1
                  maximum: 16384
                replicas:
                  type: integer
                  minimum: 1
                  maximum: 5
                  default: 1
                backupEnabled:
                  type: boolean
                  default: true
```

```bash
kubectl apply -f database-crd-cel.yaml
```

Test the rule:

```bash
kubectl apply -f - <<EOF
apiVersion: platform.example.com/v1alpha1
kind: Database
metadata:
  name: test-mysql
  namespace: default
spec:
  engine: mysql
  version: "8.0"
  storageGB: 50
  replicas: 5
EOF
# Error: The Database "test-mysql" is invalid:
#   spec: Invalid value: "object": MySQL supports a maximum of 3 replicas
```

---

## Step 2 — CEL expression syntax

CEL expressions operate on `self` (the current object or field) and `oldSelf` (for transition rules). The available types mirror the schema types.

**Boolean logic:**

```cel
self.replicas > 1 && self.backupEnabled == true
self.engine == 'postgres' || self.engine == 'mysql'
!(self.engine == 'mysql' && self.replicas > 3)
```

**String operations:**

```cel
self.version.startsWith('15.')
self.version.matches('^[0-9]+\\.[0-9]+$')
self.name.contains('prod')
size(self.version) > 0
```

**Numeric comparisons:**

```cel
self.storageGB >= 10
self.replicas * self.storageGB <= 500     # combined limit
```

**Conditional (ternary):**

```cel
self.engine == 'postgres' ? self.replicas <= 5 : self.replicas <= 3
```

**List operations:**

```cel
self.allowedCIDRs.all(cidr, cidr.matches('^([0-9]{1,3}\\.){3}[0-9]{1,3}\\/[0-9]{1,2}$'))
self.allowedCIDRs.exists(cidr, cidr == '0.0.0.0/0')
size(self.allowedCIDRs) <= 10
```

**Optional fields** — use `has()` to check if a field is present before accessing it:

```cel
!has(self.connectionPool) || self.connectionPool.maxConnections >= 10
```

---

## Step 3 — Immutability rules with oldSelf

`oldSelf` contains the previous state of the object. Transition rules using `oldSelf` only run on updates (create has no old state). Use this for immutability:

```yaml
x-kubernetes-validations:
  # engine cannot change after creation
  - rule: self.engine == oldSelf.engine
    message: "engine is immutable after creation"

  # version can only increase (no downgrade)
  - rule: >
      self.version >= oldSelf.version
    message: "database version cannot be downgraded"

  # replicas can only increase
  - rule: >
      self.replicas >= oldSelf.replicas
    message: "replicas cannot be decreased — scale down is not supported"
```

Apply these at the `spec` level of the schema. Test immutability:

```bash
# First create the resource
kubectl apply -f my-database.yaml

# Try to change the engine
kubectl patch database prod-postgres --type=merge \
  -p '{"spec":{"engine":"mysql"}}'
# Error: The Database "prod-postgres" is invalid:
#   spec: Invalid value: "object": engine is immutable after creation
```

**Important:** rules containing `oldSelf` are only evaluated on update. On create, `oldSelf` is undefined — the rule is skipped. This is the correct behavior for immutability guards.

---

## Step 4 — Conditional requirements

Express "field X is required when field Y has value Z" — something OpenAPI schema cannot do:

```yaml
x-kubernetes-validations:
  # backup must be enabled for production replicas
  - rule: >
      self.replicas <= 1 || self.backupEnabled == true
    message: "backupEnabled must be true when replicas > 1 (data protection requirement)"

  # maintenanceWindow required when backupEnabled
  - rule: >
      !self.backupEnabled || has(self.maintenanceWindow)
    message: "maintenanceWindow must be set when backupEnabled is true"

  # TLS required for production engines
  - rule: >
      self.engine != 'postgres' || self.storageGB < 500 || has(self.tlsConfig)
    message: "tlsConfig required for Postgres instances with more than 500GB"
```

---

## Step 5 — Field-level rules

CEL rules can be placed at any level of the schema, not just the top level. A rule placed on a field receives `self` as that field's value:

```yaml
properties:
  version:
    type: string
    x-kubernetes-validations:
      - rule: self.matches('^[0-9]+\\.[0-9]+(\\.[0-9]+)?$')
        message: "version must be in semver format (e.g. 15.3 or 15.3.1)"

  storageGB:
    type: integer
    x-kubernetes-validations:
      - rule: self % 10 == 0
        message: "storageGB must be a multiple of 10"

  connectionPool:
    type: object
    x-kubernetes-validations:
      - rule: >
          self.maxConnections >= self.minConnections
        message: "maxConnections must be >= minConnections"
    properties:
      maxConnections:
        type: integer
        minimum: 1
      minConnections:
        type: integer
        minimum: 0
        default: 5
```

Field-level rules keep validation logic close to the fields they constrain, making the schema self-documenting.

---

## Step 6 — Validation with messageExpression

For rules that need to include field values in the error message, use `messageExpression` (a CEL expression that returns a string):

```yaml
x-kubernetes-validations:
  - rule: >
      !(self.engine == 'mysql' && self.replicas > 3)
    messageExpression: >
      'MySQL supports a maximum of 3 replicas, but ' + string(self.replicas) + ' were requested'

  - rule: >
      self.storageGB >= 10
    messageExpression: >
      'storageGB must be at least 10, got ' + string(self.storageGB)
```

The error now shows:

```
spec: Invalid value: "object":
  MySQL supports a maximum of 3 replicas, but 5 were requested
```

`messageExpression` takes precedence over `message` when both are set. Only use it when you need dynamic content — it has a higher cost than a static string.

---

## Step 7 — Authorisation-aware validation (optionalOldSelf)

Kubernetes 1.30+ supports `optionalOldSelf` — a CEL variable that is `null` on create and contains the previous object on update. This enables rules that differ between create and update without the `oldSelf` pitfall:

```yaml
x-kubernetes-validations:
  # on create: engine required; on update: engine must not change
  - rule: >
      optionalOldSelf == null
        ? has(self.engine)
        : self.engine == optionalOldSelf.engine
    message: "engine is required on creation and immutable thereafter"
    optionalOldSelf: true    # opt in to the new variable
```

For clusters below v1.30, the pattern using `oldSelf` with separate create/update logic is sufficient.

---

## Step 8 — CEL cost limits and optimisation

The API server imposes a cost budget on CEL evaluation per object. Complex rules (especially regex matching on large strings or `all()`/`exists()` over large arrays) can exceed the budget and be rejected at CRD install time.

View the estimated cost for each rule:

```bash
kubectl get crd databases.platform.example.com -o json \
  | jq '.spec.versions[0].schema.openAPIV3Schema.properties.spec."x-kubernetes-validations"'
```

Rules that exceed the per-expression cost limit cause a CRD validation error on `kubectl apply`. To optimise:

| Pattern | Lower-cost alternative |
|---|---|
| `self.field.matches('long.*regex')` | Pre-validate with enum, use regex only for edge cases |
| `self.list.all(...)` on large arrays | Add `maxItems` to bound the list, reducing worst-case cost |
| Multiple independent `all()` expressions | Combine into a single expression with `&&` |
| Regex with unbounded quantifiers `.*` | Use anchored patterns `^...$` |

Check the total rule cost at CRD apply time — if the CRD is rejected, the error message includes which rule exceeded the limit.

---

## Step 9 — Testing your validation rules

Write a test script that exercises each rule:

```bash
#!/bin/bash
# test-database-validation.sh

PASS=0
FAIL=0

assert_rejected() {
    local name="$1"
    local manifest="$2"
    if echo "$manifest" | kubectl apply -f - 2>&1 | grep -q "invalid"; then
        echo "PASS: $name"
        ((PASS++))
    else
        echo "FAIL: $name (should have been rejected)"
        kubectl delete -f - <<< "$manifest" 2>/dev/null
        ((FAIL++))
    fi
}

assert_accepted() {
    local name="$1"
    local manifest="$2"
    if echo "$manifest" | kubectl apply -f - 2>&1 | grep -q "configured\|created"; then
        echo "PASS: $name"
        ((PASS++))
    else
        echo "FAIL: $name (should have been accepted)"
        ((FAIL++))
    fi
}

# Rule: MySQL max 3 replicas
assert_rejected "mysql-5-replicas" "$(cat <<EOF
apiVersion: platform.example.com/v1alpha1
kind: Database
metadata: {name: test-mysql-5, namespace: default}
spec: {engine: mysql, version: "8.0", storageGB: 10, replicas: 5}
EOF
)"

# Rule: version semver format
assert_rejected "bad-version-format" "$(cat <<EOF
apiVersion: platform.example.com/v1alpha1
kind: Database
metadata: {name: test-badver, namespace: default}
spec: {engine: postgres, version: "latest", storageGB: 10}
EOF
)"

# Rule: backup required for replicas > 1
assert_rejected "no-backup-replicas" "$(cat <<EOF
apiVersion: platform.example.com/v1alpha1
kind: Database
metadata: {name: test-nobackup, namespace: default}
spec: {engine: postgres, version: "15.3", storageGB: 10, replicas: 2, backupEnabled: false}
EOF
)"

# Valid resource: should be accepted
assert_accepted "valid-postgres" "$(cat <<EOF
apiVersion: platform.example.com/v1alpha1
kind: Database
metadata: {name: test-valid, namespace: default}
spec: {engine: postgres, version: "15.3", storageGB: 100, replicas: 3}
EOF
)"

echo ""
echo "Results: $PASS passed, $FAIL failed"

# Cleanup
kubectl delete database test-valid 2>/dev/null
```

```bash
chmod +x test-database-validation.sh && ./test-database-validation.sh
# PASS: mysql-5-replicas
# PASS: bad-version-format
# PASS: no-backup-replicas
# PASS: valid-postgres
# Results: 4 passed, 0 failed
```

---

## Complete CRD with all CEL rules

The full CRD combining Part 1 schema and Part 2 CEL validation:

```yaml
# database-crd-final.yaml
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
    shortNames: [db]
  versions:
    - name: v1alpha1
      served: true
      storage: true
      subresources:
        status: {}
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
        openAPIV3Schema:
          type: object
          required: [spec]
          properties:
            spec:
              type: object
              required: [engine, version, storageGB]
              x-kubernetes-validations:
                - rule: "!(self.engine == 'mysql' && self.replicas > 3)"
                  messageExpression: "'MySQL supports a maximum of 3 replicas, got ' + string(self.replicas)"
                - rule: "self.engine != 'mariadb' || self.version.startsWith('10.')"
                  message: "MariaDB version must start with '10.'"
                - rule: "self.replicas <= 1 || self.backupEnabled == true"
                  message: "backupEnabled must be true when replicas > 1"
                - rule: "self.engine == oldSelf.engine"
                  message: "engine is immutable after creation"
                - rule: "self.replicas >= oldSelf.replicas"
                  message: "replicas cannot be decreased"
              properties:
                engine:
                  type: string
                  enum: [postgres, mysql, mariadb]
                version:
                  type: string
                  x-kubernetes-validations:
                    - rule: "self.matches('^[0-9]+\\\\.[0-9]+(\\\\.[0-9]+)?$')"
                      message: "version must be in semver format (e.g. 15.3)"
                storageGB:
                  type: integer
                  minimum: 1
                  maximum: 16384
                  x-kubernetes-validations:
                    - rule: "self % 10 == 0"
                      message: "storageGB must be a multiple of 10"
                replicas:
                  type: integer
                  minimum: 1
                  maximum: 5
                  default: 1
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
kubectl apply -f database-crd-final.yaml
```

---

## What you have built

- CEL cross-field rules enforcing engine-specific replica limits and version format constraints
- Immutability guards with `oldSelf` preventing engine changes and version downgrades after creation
- Conditional requirements linking `backupEnabled` to replica count
- Field-level CEL rules on individual fields (version semver, storageGB multiples)
- Dynamic error messages with `messageExpression` including field values
- CEL cost optimisation techniques to stay within API server budget
- A validation test script to exercise every rule against the live API server

Your `Database` CRD now enforces business rules that would previously require an admission webhook — entirely through declarative schema configuration, evaluated inline by the API server.
