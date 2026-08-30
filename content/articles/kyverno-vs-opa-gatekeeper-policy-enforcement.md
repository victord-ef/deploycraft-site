---
title: "Kyverno vs OPA/Gatekeeper — Policy Enforcement Compared"
date: 2026-08-06
author: "Victor D"
description: "Kyverno and OPA/Gatekeeper both enforce policy at the Kubernetes admission layer. The difference is in language, philosophy, and what each tool can do beyond validation."
tags: ["kyverno", "opa", "gatekeeper", "kubernetes", "security", "policy", "devsecops", "comparisons"]
categories: ["article"]
draft: false
toc: true
---

Both Kyverno and OPA/Gatekeeper sit at the same point in the Kubernetes request lifecycle — the admission webhook — and intercept every resource before it reaches etcd. Both can validate resources and reject non-compliant ones. Both audit existing cluster state. But they make fundamentally different choices about how policy should be expressed and what "policy enforcement" means beyond simple validation.

Understanding those choices is what makes this comparison useful. The feature lists overlap enough to confuse; the philosophy diverges enough to matter.

---

## The admission webhook foundation

Before comparing tools, the mechanism is worth understanding because it shapes what both tools can and cannot do.

When you run `kubectl apply`, the Kubernetes API server passes the request through a chain of admission controllers. **Validating admission webhooks** can inspect the request and reject it. **Mutating admission webhooks** can modify the request before it is stored. Both Kyverno and Gatekeeper register themselves as admission webhooks and intercept matching resource operations.

```
kubectl apply → API Server → Mutating Webhooks → Validating Webhooks → etcd
```

The key constraint: admission webhooks only intercept new requests. Resources that already exist in the cluster were admitted before your policy was installed. Both tools solve this with background audit — scanning existing resources and reporting violations — but background scan cannot retroactively block a resource that already exists.

---

## What each tool is

**OPA (Open Policy Agent)** is a general-purpose, domain-agnostic policy engine. It is not Kubernetes-specific — the same OPA instance can enforce policy over Terraform plans, HTTP API calls, Kafka topics, and Kubernetes resources, all using the same language: **Rego**.

**Gatekeeper** is the Kubernetes integration layer for OPA. It registers as an admission webhook, syncs Kubernetes resources into OPA's data cache, and translates Kubernetes admission requests into the format OPA expects. Gatekeeper is how you use OPA in a Kubernetes cluster.

**Kyverno** is a policy engine built specifically for Kubernetes. Policies are Kubernetes resources — YAML with `apiVersion: kyverno.io/v1`. There is no separate policy language to learn. Kyverno understands the Kubernetes object model natively and provides capabilities that go beyond validation: mutation, resource generation, and image signature verification.

---

## The policy language: Rego vs YAML

This is where most teams make their decision.

### Gatekeeper: Rego

OPA's policy language, Rego, is a declarative query language in the Datalog family. It is powerful, precise, and unfamiliar. Rego is not imperative — you do not write "if this then deny." You write logical rules that define when a violation exists.

A policy requiring that every Deployment has a `team` label:

```rego
# ConstraintTemplate — defines the policy logic in Rego
package k8srequiredlabels

violation[{"msg": msg}] {
    provided := {label | input.review.object.metadata.labels[label]}
    required := {label | label := input.parameters.labels[_]}
    missing := required - provided
    count(missing) > 0
    msg := sprintf("missing required labels: %v", [missing])
}
```

```yaml
# ConstraintTemplate — wraps the Rego in a Kubernetes CRD
apiVersion: templates.gatekeeper.sh/v1
kind: ConstraintTemplate
metadata:
  name: k8srequiredlabels
spec:
  crd:
    spec:
      names:
        kind: K8sRequiredLabels
      validation:
        openAPIV3Schema:
          properties:
            labels:
              type: array
              items:
                type: string
  targets:
  - target: admission.k8s.gatekeeper.sh
    rego: |
      package k8srequiredlabels
      violation[{"msg": msg}] {
        provided := {label | input.review.object.metadata.labels[label]}
        required := {label | label := input.parameters.labels[_]}
        missing := required - provided
        count(missing) > 0
        msg := sprintf("missing required labels: %v", [missing])
      }
```

```yaml
# Constraint — an instance of the template, applied to specific resources
apiVersion: constraints.gatekeeper.sh/v1beta1
kind: K8sRequiredLabels
metadata:
  name: require-team-label
spec:
  enforcementAction: deny
  match:
    kinds:
    - apiGroups: ["apps"]
      kinds: ["Deployment"]
  parameters:
    labels: ["team"]
```

Three separate objects for one policy. The `ConstraintTemplate` is compiled into a CRD when applied — Gatekeeper creates a new CRD type (`K8sRequiredLabels`) that you then instantiate as a `Constraint`. This indirection allows the template to be reused with different parameters, but it adds cognitive overhead.

### Kyverno: YAML

The same policy in Kyverno:

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: require-team-label
spec:
  validationFailureAction: Enforce
  rules:
  - name: check-team-label
    match:
      any:
      - resources:
          kinds:
          - Deployment
    validate:
      message: "The label 'team' is required on all Deployments."
      pattern:
        metadata:
          labels:
            team: "?*"
```

One object. No new language. The policy is readable by anyone who knows YAML and Kubernetes — which is most of your team.

The trade-off is expressiveness. Rego can express arbitrarily complex logic: cross-object comparisons, list comprehensions, external data lookups, mathematical operations. Kyverno's pattern-matching and JMESPath expressions cover the vast majority of real-world policy use cases, but when you need to express something unusual, Kyverno's YAML syntax reaches its limits before Rego does.

---

## Mutation: modifying resources at admission

Both tools can mutate resources — add labels, inject sidecars, set default values — but Kyverno's mutation support is more mature and easier to use.

### Kyverno mutation

Add a `managed-by` label to any Deployment that does not already have one:

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: add-managed-by-label
spec:
  rules:
  - name: add-label
    match:
      any:
      - resources:
          kinds:
          - Deployment
    mutate:
      patchStrategicMerge:
        metadata:
          labels:
            +(managed-by): kyverno   # the + prefix means "add if not present"
```

Set a default `securityContext` on all containers that do not specify one:

```yaml
  mutate:
    patchStrategicMerge:
      spec:
        template:
          spec:
            containers:
            - (name): "*"
              securityContext:
                +(allowPrivilegeEscalation): false
                +(readOnlyRootFilesystem): true
                +(runAsNonRoot): true
```

### Gatekeeper mutation

Gatekeeper's mutation API uses separate resource types (`Assign`, `AssignMetadata`, `ModifySet`) and was added after the validation API — it is less polished:

```yaml
apiVersion: mutations.gatekeeper.sh/v1
kind: AssignMetadata
metadata:
  name: add-managed-by-label
spec:
  match:
    scope: Namespaced
    kinds:
    - apiGroups: ["apps"]
      kinds: ["Deployment"]
  location: "metadata.labels.managed-by"
  parameters:
    assign:
      value: "gatekeeper"
```

For mutation use cases, Kyverno is the more ergonomic choice. Most teams using Gatekeeper for validation reach for Kyverno (or just plain admission webhooks) if they need mutation.

---

## Generate: a capability Gatekeeper does not have

Kyverno's `generate` rule type has no equivalent in Gatekeeper. It automatically creates Kubernetes resources in response to a trigger resource being created or modified.

The most common use case: ensure every new namespace gets a default-deny NetworkPolicy, a LimitRange, and a ResourceQuota without any manual step:

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: namespace-baseline
spec:
  rules:
  - name: default-deny-network-policy
    match:
      any:
      - resources:
          kinds:
          - Namespace
    generate:
      apiVersion: networking.k8s.io/v1
      kind: NetworkPolicy
      name: default-deny-all
      namespace: "{{request.object.metadata.name}}"
      synchronize: true   # keep the generated resource in sync
      data:
        spec:
          podSelector: {}
          policyTypes:
          - Ingress
          - Egress

  - name: default-resource-quota
    match:
      any:
      - resources:
          kinds:
          - Namespace
    generate:
      apiVersion: v1
      kind: ResourceQuota
      name: default-quota
      namespace: "{{request.object.metadata.name}}"
      synchronize: true
      data:
        spec:
          hard:
            requests.cpu: "4"
            requests.memory: 4Gi
            limits.cpu: "8"
            limits.memory: 8Gi
```

With `synchronize: true`, Kyverno actively reconciles the generated resources — if someone deletes the NetworkPolicy, Kyverno recreates it. This turns "generate on creation" into "enforce as invariant."

This is one of the most operationally useful features in either tool. A team using Gatekeeper has to implement the namespace bootstrapping workflow separately — usually in a CI/CD pipeline or a custom controller.

---

## Image verification

Kyverno supports container image signature verification natively, using Cosign and Notary signatures. This allows you to enforce that only cryptographically signed images from trusted registries run in the cluster:

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: verify-image-signatures
spec:
  validationFailureAction: Enforce
  rules:
  - name: check-signature
    match:
      any:
      - resources:
          kinds:
          - Pod
    verifyImages:
    - imageReferences:
      - "your-registry.example.com/*"
      attestors:
      - count: 1
        entries:
        - keys:
            publicKeys: |-
              -----BEGIN PUBLIC KEY-----
              MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE...
              -----END PUBLIC KEY-----
```

OPA/Gatekeeper can verify images but requires an external data provider to fetch attestation data — a more complex setup that is not built into Gatekeeper itself.

For supply chain security requirements, Kyverno's native Cosign integration is a meaningful advantage.

---

## Audit mode and policy reports

Both tools produce `PolicyReport` and `ClusterPolicyReport` resources — a standard defined by the Kubernetes Policy Working Group. These reports show which existing resources violate policy, which is essential for understanding your current compliance posture before moving from audit to enforcement.

**Gatekeeper:**

```bash
# Set a constraint to audit mode rather than deny
spec:
  enforcementAction: warn   # or "dryrun" to log without warning

# Check audit results
kubectl get k8srequiredlabels.constraints.gatekeeper.sh require-team-label \
  -o jsonpath='{.status.violations}' | jq .
```

**Kyverno:**

```bash
# Set a policy to audit mode
spec:
  validationFailureAction: Audit   # instead of Enforce

# Check policy reports
kubectl get policyreport -A
kubectl get clusterpolicyreport

# Detailed violations
kubectl get policyreport -n production -o yaml | \
  yq '.results[] | select(.result == "fail")'
```

Kyverno's integration with PolicyReport is tighter — background scanning runs continuously and reports are kept up to date as resources change. Gatekeeper's audit controller runs on a configurable schedule (default every 60 seconds).

---

## Shift-left: testing policies before they reach the cluster

Both ecosystems support testing policies in CI pipelines, before code reaches the cluster.

**Gatekeeper** uses **conftest** — a general-purpose tool for running Rego policies against structured data (YAML, JSON, HCL). The same Rego you write for Gatekeeper can be tested against local manifests:

```bash
# Test a Kubernetes manifest against Rego policies in CI
conftest test deployment.yaml --policy ./policies/
```

Because Rego is general-purpose, the same policy library can validate Terraform plans, Helm rendered output, and Kubernetes manifests in one pipeline.

**Kyverno** has a dedicated CLI:

```bash
# Install the Kyverno CLI
kubectl kyverno version

# Test policies against local manifests
kyverno test ./tests/

# Apply a policy to a local manifest and see the result
kyverno apply ./policies/require-team-label.yaml \
  --resource ./manifests/deployment.yaml
```

The Kyverno CLI is simpler to use and does not require learning a separate testing framework. The trade-off: it only tests Kubernetes resources, not Terraform or other resource types.

---

## How to choose

**Choose Kyverno if:**

- Your team writes and reviews policy in YAML — no Rego expertise required or desired
- You need resource generation (automatic NetworkPolicies, ResourceQuotas, LimitRanges on namespace creation)
- You want native image signature verification with Cosign
- Mutation is a first-class requirement
- You want a single tool for validate, mutate, generate, and image verify in one policy object
- Your scope is Kubernetes only

**Choose OPA/Gatekeeper if:**

- You already use OPA outside Kubernetes — for API authorization, Terraform policy, or infrastructure validation — and want to share Rego policy across systems
- Your policies require complex logic that exhausts Kyverno's pattern-matching and JMESPath: cross-object lookups, complex list comprehensions, external data integration
- Your security team already has Rego expertise and prefers the expressiveness of a proper query language
- You are on Azure Kubernetes Service — Azure Policy for AKS is built on Gatekeeper, so you inherit it

**The honest middle ground:** For most platform engineering teams starting from scratch, Kyverno is the faster path to working policy enforcement. The YAML syntax removes a learning curve, the generate feature handles namespace bootstrapping that Gatekeeper cannot, and the Cosign integration handles supply chain security in one policy object. Teams reach for Gatekeeper when they need Rego's expressiveness or have an existing OPA investment to leverage.

Using both is also valid — some teams use Kyverno for mutation and generation, and Gatekeeper for complex validation logic that requires Rego. The two can coexist in the same cluster.

---

## Related reading

- [Kyverno documentation](https://kyverno.io/docs/)
- [OPA Gatekeeper documentation](https://open-policy-agent.github.io/gatekeeper/)
- [Gatekeeper policy library](https://open-policy-agent.github.io/gatekeeper-library/)
- [Kyverno policy library](https://kyverno.io/policies/)
- [conftest — policy testing for CI](https://www.conftest.dev/)
- Why least privilege is harder than it sounds in Kubernetes → **Articles**
- How container image layers work and why it matters for security → **Articles**
