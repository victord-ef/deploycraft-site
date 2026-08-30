---
title: "The Double-Edged Sword — Part 1: Securing kubectl debug in Kubernetes"
date: 2026-08-06
author: "Victor D"
description: "kubectl debug is invaluable for troubleshooting distroless containers — and one of the easiest privilege escalation paths in a Kubernetes cluster. Here is how it works, what attackers do with it, and how to lock it down."
tags: ["kubernetes", "security", "rbac", "kubectl", "debugging", "devsecops", "ephemeral-containers"]
categories: ["article"]
draft: false
toc: true
---

"Kubernetes debugging tools are designed for convenience, not security." That line comes from a post-mortem after a cluster compromise that began with a routine `kubectl debug` session. The engineer was fixing a broken application. They had no malicious intent. But by the time the session ended, an attacker had a shell with cluster-admin credentials — and nobody noticed for days.

`kubectl debug` is a genuinely useful feature. It is also one of the most underappreciated privilege escalation paths in a Kubernetes cluster. Understanding why requires looking at exactly what it does.

---

## How kubectl debug actually works

When you run `kubectl debug`, Kubernetes injects an **ephemeral container** into a running pod. Unlike a sidecar, this container is temporary — it has no restart policy and disappears when you exit. But while it is running, it shares the original pod's namespaces:

```bash
kubectl debug -it frontend-7d9f8b6c4-xk2p1 \
  -n production \
  --image=busybox \
  --target=frontend
```

That `--target` flag makes the debug container share the process namespace of the `frontend` container. It can see the original container's processes, its mounted volumes, and its network stack. It sends and receives traffic from the pod's IP address. And — critically — it inherits the pod's mounted service account token.

This is exactly what makes it useful for troubleshooting distroless containers that have no shell. It is also exactly what makes it dangerous in the wrong hands.

---

## When debug becomes a backdoor

### The credential inheritance problem

Consider a scenario that has played out in real post-mortems: a senior engineer runs `kubectl debug` on a failing application pod. The pod's service account was provisioned with broad permissions — perhaps during a late-night incident months earlier, when `cluster-admin` was the fastest fix. The engineer's intent is innocent. But the ephemeral busybox container now has access to the pod's service account token at the standard path:

```bash
# Inside the debug container
cat /var/run/secrets/kubernetes.io/serviceaccount/token
```

That token can be used to make API calls with the pod's identity. If that identity has `cluster-admin` permissions, the attacker has full control of the cluster — scheduling pods on any node, reading every secret, modifying any workload.

The detail that makes this particularly hard to catch: ephemeral containers are easy to miss in routine inspection. They appear under an `Ephemeral Containers:` section in `kubectl describe pod`, which most engineers never scroll to because they don't expect it to have content. Combined with the temporary nature of the container, a short-lived debug session can come and go without triggering any alert.

### Bypassing distroless minimalism

Production workloads increasingly use distroless images — containers with no shell, no package manager, and no standard Unix utilities. This is a deliberate security choice: without `bash`, `curl`, or `nc`, an attacker who achieves code execution inside the container has almost nothing to work with.

`kubectl debug` dissolves that protection in one command. Inject a busybox or netshoot image and you have a full toolkit inside the pod's network namespace. Port scanning, DNS enumeration, internal service discovery — all now available. And because the traffic originates from the legitimate pod IP, it bypasses any network filter or anomaly detection that uses IP reputation as a signal.

### Data exfiltration via mounted volumes

The debug container shares the original pod's volume mounts. Secrets, ConfigMaps, and application data files that the original container has access to are all readable from the debug container. An attacker can exfiltrate them to an external endpoint using `curl` — traffic that appears to come from a trusted workload IP.

### Node-level compromise

{{< callout type="warning" title="The most dangerous kubectl debug variant" >}}
`kubectl debug node/mynode --image=busybox` creates a privileged pod on the node that mounts the node's root filesystem at `/host` and joins the host's network, IPC, and PID namespaces. An attacker with node debug permissions has effective root on the underlying host — not just the cluster. Treat this sub-resource as equivalent to `cluster-admin`.
{{< /callout >}}

### Bypassing admission controls

The `pods/ephemeralcontainers` sub-resource was added to Kubernetes later than the core pod sub-resources, and some admission webhooks (including older versions of Gatekeeper and Kyverno) did not intercept it. A user could create an ephemeral container with privileges that would have been blocked at pod admission time. Verify that your admission webhook version explicitly covers this sub-resource before relying on it as a control.

---

## Locking it down with RBAC

The primary control is simple: **treat `pods/ephemeralcontainers` as a privileged sub-resource and restrict `create` and `patch` on it**.

Most developer roles should not include this permission at all. The ability to run `kubectl debug` should be limited to a small set of senior SREs or security engineers who need it for specific, authorised tasks.

### Safe role — no debug access

This role allows a developer to view pod state and logs without being able to inject ephemeral containers:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: developer-read
  namespace: production
rules:
- apiGroups: [""]
  resources: ["pods", "pods/log"]
  verbs: ["get", "list", "watch"]
```

### Dangerous role — implicit debug access

This role grants `create` and `patch` on `pods/ephemeralcontainers`. Any user assigned it can run `kubectl debug` on any pod in the namespace:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: developer-debug   # overly permissive
  namespace: production
rules:
- apiGroups: [""]
  resources: ["pods", "pods/log", "pods/ephemeralcontainers"]
  verbs: ["get", "list", "watch", "create", "patch"]
```

### Correct SRE role — scoped debug access

Grant debug access explicitly to a trusted role, separate from the general developer role:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: sre-debug
  namespace: production
rules:
- apiGroups: [""]
  resources: ["pods", "pods/log"]
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources: ["pods/ephemeralcontainers"]
  verbs: ["create", "patch"]
```

### Auditing who already has this permission

Before you change anything, find out who currently has `create` on `pods/ephemeralcontainers` in your cluster:

```bash
# Find all roles that include ephemeralcontainers
kubectl get roles,clusterroles -A -o json | \
  jq -r '.items[] | 
    select(.rules[]? | 
      select(.resources[]? | contains("ephemeralcontainers")) | 
      select(.verbs[]? | test("create|patch|\\*"))
    ) | "\(.metadata.namespace // "cluster")/\(.metadata.name)"'

# Then find who is bound to those roles
kubectl get rolebindings,clusterrolebindings -A -o wide | grep sre-debug
```

Run this before making any RBAC changes — the result will tell you whether the restriction is already in place or whether you have a remediation task ahead of you.

---

## Beyond RBAC: layered defences

RBAC is the primary control but not the only one.

**Fix service account token mounting at the source.** The advice to "pass `--automount-service-account-token=false` to `kubectl debug`" is incorrect — that flag applies to pod specs, not ephemeral containers. The token is already mounted in the pod's filesystem before the debug container is injected. The correct mitigation is setting `automountServiceAccountToken: false` on the ServiceAccount or the Pod itself, so that sensitive workloads never have a token mounted in the first place:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: frontend
  namespace: production
automountServiceAccountToken: false
```

**Pod Security Standards.** Apply the `Restricted` or `Baseline` Pod Security Standard to your production namespaces. This prevents debug containers from running in privileged mode or mounting sensitive host paths — even if the RBAC check passes.

**Admission webhooks.** Gatekeeper and Kyverno can enforce custom policies on ephemeral container creation. Verify your webhook version covers the `pods/ephemeralcontainers` sub-resource explicitly:

```bash
# Check Kyverno policy applies to ephemeralcontainers
kubectl get clusterpolicy -o yaml | grep ephemeralcontainers
```

**Just-in-time access.** Rather than granting permanent debug permissions, tools like Teleport or HashiCorp Vault with Kubernetes auth can issue time-limited credentials for a specific debug session. The access expires automatically and every session is logged with a human-readable reason.

**Audit logging.** Enable API server audit logging and alert on `create` events on the `pods/ephemeralcontainers` sub-resource. This gives you a record of every debug session with the requesting user, timestamp, and target pod:

```yaml
# Audit policy excerpt
- level: Request
  verbs: ["create", "patch"]
  resources:
  - group: ""
    resources: ["pods/ephemeralcontainers"]
```

---

## Best practices

1. **Restrict `pods/ephemeralcontainers`** — `create` and `patch` on this sub-resource belong in a named, elevated role only. Never include it in a general developer role.
2. **Never use wildcard verbs** — `verbs: ["*"]` on pods or ephemeralcontainers grants implicit debug access. Enumerate verbs explicitly.
3. **Disable token mounts on sensitive service accounts** — set `automountServiceAccountToken: false` at the ServiceAccount level so tokens are never present to inherit.
4. **Avoid `--share-processes` in production sessions** — sharing the process namespace exposes all processes in the pod to the debug container, including any in-memory secrets.
5. **Alert on ephemeral container creation** — treat every `kubectl debug` event as a privileged action that should be reviewed.
6. **Audit RoleBindings regularly** — run the audit query above on a schedule. RBAC creep is real; permissions granted during incidents often stay long after they should be revoked.
7. **Use Network Policies alongside RBAC** — even if an attacker gains debug access, a default-deny NetworkPolicy limits what they can reach from inside the pod.

---

## The bottom line

`kubectl debug` is a tool designed for a specific job: troubleshooting workloads that cannot be inspected with a shell. It does that job well. But the same properties that make it useful — namespace sharing, toolkit injection, credential inheritance — make it a significant attack surface if access is not controlled.

The fix is not to disable the feature. It is to treat it like what it is: a privileged operation that injects trusted-looking traffic into your network, reads your secrets, and bypasses your minimal image security. Grant it deliberately, audit it consistently, and revoke it when it is no longer needed.

A NetworkPolicy and a distroless image are strong controls. An ungated `kubectl debug` undoes both of them in a single command.

---

## Related reading

- [Kubernetes ephemeral containers documentation](https://kubernetes.io/docs/concepts/workloads/pods/ephemeral-containers/)
- [Kubernetes RBAC documentation](https://kubernetes.io/docs/reference/access-authn-authz/rbac/)
- [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/)
- Debugging distroless containers with ephemeral containers → **Article: Security through Minimalism**
- The NetworkPolicy that did nothing: debugging a missing CNI → **Article: NetworkPolicy Not Enforced**
