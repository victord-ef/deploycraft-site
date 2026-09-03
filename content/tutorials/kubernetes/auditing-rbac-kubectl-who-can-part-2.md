---
title: "Auditing and Tightening RBAC Permissions with kubectl-who-can — Part 2"
date: 2026-09-01
description: "Use kubectl-who-can to audit who can perform sensitive operations in your cluster, identify over-permissioned subjects, and apply targeted fixes to reach a least-privilege RBAC posture."
cluster: "Kubernetes"
series: "RBAC"
part: 2
difficulty: "intermediate"
duration: "40 min"
tags: ["kubernetes", "rbac", "security", "authorization", "kubectl-who-can", "devsecops", "auditing"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

In [Part 1](/tutorials/kubernetes/implementing-rbac-kubernetes-part-1/) you built a least-privilege RBAC model with developer, CI/CD, and SRE personas. In Part 2 you will audit that model using `kubectl-who-can` — a kubectl plugin that answers the question "who can perform this action?" across your entire cluster. You will identify over-permissioned subjects, common RBAC misconfigurations, and apply targeted fixes verified by re-audit.

## Prerequisites

- Completed [Part 1](/tutorials/kubernetes/implementing-rbac-kubernetes-part-1/) — developer, cicd, and sre ServiceAccounts configured with their Roles and Bindings
- `kubectl` with cluster-admin access
- `krew` (kubectl plugin manager) installed, or the ability to install binaries manually

---

## Why RBAC auditing is non-negotiable

RBAC permissions accumulate silently. A Role created during an incident response never gets cleaned up. A developer ServiceAccount gets `edit` cluster-wide "temporarily" and stays that way for 18 months. A Helm chart installs a ClusterRoleBinding with `cluster-admin` for its operator. None of these show up as errors — they are valid configurations that happen to violate least privilege.

`kubectl auth can-i` answers "can subject X do action Y?" but you have to know what to ask. `kubectl-who-can` inverts this: it answers "who can do action Y?" across every subject in the cluster — users, groups, and service accounts — without you having to enumerate them first.

---

## Step 1 — Install kubectl-who-can

### Via krew

```bash
kubectl krew install who-can
kubectl who-can --version
```

### Manual install (Linux/macOS)

```bash
# Get the latest release
VERSION=$(curl -s https://api.github.com/repos/aquasecurity/kubectl-who-can/releases/latest \
  | grep tag_name | cut -d '"' -f4)

curl -LO "https://github.com/aquasecurity/kubectl-who-can/releases/download/${VERSION}/kubectl-who-can_linux_x86_64.tar.gz"
tar -xzf kubectl-who-can_linux_x86_64.tar.gz
chmod +x kubectl-who-can
mv kubectl-who-can /usr/local/bin/
```

### Manual install (Windows)

Download the latest release from the [GitHub releases page](https://github.com/aquasecurity/kubectl-who-can/releases), extract the binary, and add it to your PATH.

Verify:

```bash
kubectl who-can get pods -n development
```

---

## Step 2 — Audit sensitive verbs cluster-wide

Start with the operations that matter most for security. These are the verbs that attackers target after gaining initial access to a cluster:

```bash
# Who can create pods? (pod creation = potential escape to node)
kubectl who-can create pods --all-namespaces

# Who can exec into pods? (exec = interactive shell on running container)
kubectl who-can create pods/exec --all-namespaces

# Who can delete pods? (delete = disruption, covering tracks)
kubectl who-can delete pods --all-namespaces

# Who can access secrets? (secrets = credentials, tokens, keys)
kubectl who-can get secrets --all-namespaces

# Who can update deployments? (deploy = code execution at scale)
kubectl who-can update deployments --all-namespaces

# Who can create ClusterRoleBindings? (RBAC escalation path)
kubectl who-can create clusterrolebindings --all-namespaces

# Who can impersonate other users or service accounts?
kubectl who-can impersonate users --all-namespaces
```

Review the output for each. Every subject in the list is a potential attack path for that operation. For each unexpected entry, ask: does this subject legitimately need this permission, or is this accumulation?

---

## Step 3 — Audit a specific namespace

For the namespaces built in Part 1:

```bash
# Full audit of sensitive operations in development namespace
for verb in get list create update delete patch exec; do
  echo "=== who can $verb pods in development ==="
  kubectl who-can $verb pods -n development
done
```

```bash
# Check who can read secrets specifically
kubectl who-can get secrets -n development
kubectl who-can list secrets -n development
```

Cross-reference the output against your expected access matrix:

| Subject | Expected | Flag if seen in |
|---|---|---|
| `developer-sa` | get/list pods, exec | get secrets, delete pods |
| `cicd-sa` | update deployments | exec pods, get secrets |
| `sre-sa` | get/list cluster-wide | any write verb |

---

## Step 4 — Detect common RBAC misconfigurations

### Wildcard permissions

The most dangerous RBAC pattern is wildcard verbs or resources. Search for them:

```bash
kubectl get roles,clusterroles --all-namespaces -o yaml | grep -E '"\*"'
```

Any Role containing `"*"` in verbs or resources grants unlimited access to that scope. This includes the built-in `cluster-admin` ClusterRole — which is intentional — but also any custom roles that inadvertently used wildcards.

### Subjects bound to cluster-admin

```bash
kubectl who-can "*" "*" --all-namespaces | grep -v "^CLUSTERROLEBINDING"
```

Or inspect directly:

```bash
kubectl get clusterrolebindings -o json | \
  jq '.items[] | select(.roleRef.name=="cluster-admin") | 
  {name: .metadata.name, subjects: .subjects}'
```

In a healthy cluster, `cluster-admin` bindings should cover only: the initial bootstrap user, the `kube-system` namespace system components, and explicitly named platform operators. Any service account outside `kube-system` bound to `cluster-admin` is a finding.

### Overly broad namespace access

Check if any RoleBinding references a ClusterRole with a ClusterRoleBinding when a namespaced Role would suffice:

```bash
kubectl get clusterrolebindings -o json | \
  jq '.items[] | select(.roleRef.name | test("admin|edit|view")) |
  {binding: .metadata.name, role: .roleRef.name, subjects: .subjects}'
```

`edit` or `admin` ClusterRoleBindings granted cluster-wide (rather than via a RoleBinding scoped to one namespace) are a common misconfiguration from Helm chart installs.

---

## Step 5 — Fix over-permissioned subjects

Based on the audit, apply targeted fixes. Three common patterns:

### Fix 1 — Remove wildcard verbs

```yaml
# Before (dangerous)
rules:
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["*"]

# After (explicit)
rules:
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list", "watch"]
```

```bash
kubectl edit role <role-name> -n <namespace>
```

### Fix 2 — Downscope a ClusterRoleBinding to a RoleBinding

If a service account only needs access in one namespace, replace its ClusterRoleBinding with a RoleBinding:

```bash
# Remove the overly broad ClusterRoleBinding
kubectl delete clusterrolebinding <binding-name>

# Create a namespaced RoleBinding instead
kubectl create rolebinding <binding-name> \
  --clusterrole=<role-name> \
  --serviceaccount=<namespace>:<sa-name> \
  --namespace=<target-namespace>
```

### Fix 3 — Remove exec from the developer Role

If your policy decision is that exec should require a separate approval process, remove it from the developer Role:

```bash
kubectl edit role developer -n development
# Remove the pods/exec rule entirely
```

Re-verify after each fix:

```bash
kubectl who-can create pods/exec -n development
kubectl auth can-i create pods/exec \
  --namespace development \
  --as system:serviceaccount:development:developer-sa
```

---

## Step 6 — Audit ServiceAccount token permissions

Every pod in Kubernetes automatically mounts a ServiceAccount token unless explicitly disabled. This token can be used to call the API server. Check what the default ServiceAccount can do:

```bash
kubectl who-can get secrets \
  --as system:serviceaccount:development:default \
  -n development

kubectl who-can create pods \
  --as system:serviceaccount:development:default \
  -n development
```

If the default ServiceAccount has meaningful permissions, either tighten its Role bindings or disable token auto-mounting for workloads that don't need API access:

```yaml
# In your Deployment spec
spec:
  template:
    spec:
      automountServiceAccountToken: false
```

Or at the ServiceAccount level to apply to all pods using it:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: default
  namespace: development
automountServiceAccountToken: false
```

---

## Step 7 — Build a continuous audit check

Run `kubectl-who-can` as part of your security posture checks. A simple script that flags unexpected access to sensitive verbs:

```bash
#!/bin/bash
# rbac-audit.sh — flag subjects with access to sensitive operations

SENSITIVE_CHECKS=(
  "create pods --all-namespaces"
  "create pods/exec --all-namespaces"
  "get secrets --all-namespaces"
  "create clusterrolebindings --all-namespaces"
  "impersonate users --all-namespaces"
)

KNOWN_PRIVILEGED=(
  "system:masters"
  "system:kube-controller-manager"
  "system:kube-scheduler"
  "kube-system"
)

echo "=== RBAC Sensitive Access Audit $(date -u +%Y-%m-%dT%H:%M:%SZ) ==="

for check in "${SENSITIVE_CHECKS[@]}"; do
  echo ""
  echo "--- who can $check ---"
  kubectl who-can $check 2>/dev/null
done
```

```bash
chmod +x rbac-audit.sh
./rbac-audit.sh | tee rbac-audit-$(date +%Y%m%d).txt
```

Store the output. Diff against the previous run to detect new bindings:

```bash
diff rbac-audit-20260828.txt rbac-audit-20260901.txt
```

New entries in the diff are new permissions granted since the last audit — review each one.

---

## Step 8 — Enforcing RBAC policy with Kyverno

To prevent future misconfigurations, enforce RBAC policy at admission time with Kyverno. Block any Role or ClusterRole that uses wildcard verbs:

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: restrict-wildcard-verbs
spec:
  validationFailureAction: Enforce
  rules:
    - name: no-wildcard-verbs
      match:
        any:
          - resources:
              kinds: ["Role", "ClusterRole"]
      validate:
        message: "Wildcard verbs are not permitted in Roles or ClusterRoles."
        deny:
          conditions:
            any:
              - key: "{{ request.object.rules[].verbs[] | contains(@, '*') }}"
                operator: Equals
                value: true
```

```bash
kubectl apply -f restrict-wildcard-verbs.yaml
```

Test the policy:

```bash
# Should be blocked
kubectl create role test-wildcard \
  --verb="*" \
  --resource=pods \
  --namespace=development
```

---

## What you have built

- A full audit of cluster RBAC using `kubectl-who-can` across all sensitive verbs
- Detection of wildcard permissions, over-scoped ClusterRoleBindings, and cluster-admin sprawl
- Targeted remediation patterns: wildcard removal, ClusterRoleBinding downscoping, exec restriction
- Default ServiceAccount token audit and auto-mount disable
- A repeatable audit script for ongoing posture monitoring
- A Kyverno policy blocking wildcard verb Roles at admission

The combination of `kubectl-who-can` for discovery and Kyverno for enforcement gives you both reactive audit capability and proactive prevention — the two layers a production RBAC posture requires.
