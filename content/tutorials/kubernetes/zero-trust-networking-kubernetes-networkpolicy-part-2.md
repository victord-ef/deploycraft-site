---
title: "Enforcing Zero-Trust Networking with Kubernetes NetworkPolicy — Part 2"
date: 2026-09-01
description: "Apply zero-trust networking cluster-wide: default-deny all traffic in every namespace, build a systematic allow-list model, handle system traffic, and use Cilium for policy visibility."
cluster: "Kubernetes"
series: "Networking"
part: 2
difficulty: "advanced"
duration: "50 min"
tags: ["kubernetes", "networkpolicy", "zero-trust", "networking", "security", "cilium", "calico", "devsecops"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

In [Part 1](/tutorials/kubernetes/implementing-networkpolicy-pod-traffic-control-part-1/) you built targeted NetworkPolicies for specific namespaces and services. In Part 2 you will apply zero-trust networking cluster-wide — every namespace gets a default-deny for both ingress and egress, and only explicitly declared traffic is permitted. You will handle the edge cases (health checks, DNS, metrics, system components) that make cluster-wide zero-trust operationally viable, and use Cilium's observability tooling to verify policy decisions.

## Prerequisites

- Completed [Part 1](/tutorials/kubernetes/implementing-networkpolicy-pod-traffic-control-part-1/)
- A CNI plugin with NetworkPolicy enforcement (Calico or Cilium recommended for this tutorial)
- For the Cilium observability section: Cilium installed as the CNI

---

## Zero-trust networking in Kubernetes

Zero-trust networking means: no traffic is permitted by default. Every connection — ingress and egress — must be explicitly declared. Applied to Kubernetes, this means:

1. Every namespace gets a default-deny-all policy (both ingress and egress)
2. Every service gets an explicit allow policy for its known callers
3. System traffic (DNS, health checks, metrics scraping) is explicitly permitted
4. No implicit trust between namespaces or pods

The operational benefit: any unexpected connection is blocked. Lateral movement after a pod compromise is constrained to only the services that pod is explicitly permitted to reach. An attacker with RCE in an application pod cannot pivot to the database, the Vault sidecar, or other services — the network enforces the boundary.

---

## Step 1 — Default deny all traffic cluster-wide

Apply a default-deny-all policy to every namespace. This policy selects all pods and defines both `Ingress` and `Egress` in `policyTypes` with no rules — blocking all traffic in both directions:

```yaml
# default-deny-all.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny-all
  namespace: NAMESPACE
spec:
  podSelector: {}
  policyTypes:
    - Ingress
    - Egress
```

Apply to all application namespaces at once:

```bash
for ns in app db frontend production staging; do
  kubectl apply -f - <<EOF
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny-all
  namespace: $ns
spec:
  podSelector: {}
  policyTypes:
    - Ingress
    - Egress
EOF
done
```

After this, all pods in those namespaces have no network connectivity — including DNS. Verify:

```bash
kubectl exec -n app client -- curl -s --max-time 3 server.db.svc.cluster.local
# Expected: timeout (000)

kubectl exec -n app client -- nslookup kubernetes.default
# Expected: timeout (DNS is also blocked)
```

---

## Step 2 — Restore DNS for all namespaces

DNS must be explicitly permitted before any other egress policy is useful. CoreDNS runs in `kube-system` — allow egress to it from every namespace:

```yaml
# allow-dns-egress.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-dns-egress
  namespace: NAMESPACE
spec:
  podSelector: {}
  policyTypes:
    - Egress
  egress:
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: kube-system
      ports:
        - protocol: UDP
          port: 53
        - protocol: TCP
          port: 53
```

Apply to all namespaces:

```bash
for ns in app db frontend production staging; do
  kubectl apply -f - <<EOF
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-dns-egress
  namespace: $ns
spec:
  podSelector: {}
  policyTypes:
    - Egress
  egress:
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: kube-system
      ports:
        - protocol: UDP
          port: 53
        - protocol: TCP
          port: 53
EOF
done
```

Verify DNS works again:

```bash
kubectl exec -n app client -- nslookup kubernetes.default
# Expected: resolves successfully
```

---

## Step 3 — Allow Kubernetes API server egress

Many workloads and controllers need to reach the Kubernetes API server. The API server runs on the control plane nodes, not as a pod — its IP is accessible via the `kubernetes` Service in `default`:

```bash
# Get the API server ClusterIP
kubectl get svc kubernetes -n default
```

Allow egress to the API server for pods that need it (scope this to specific pods rather than all pods):

```yaml
# allow-apiserver-egress.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-apiserver-egress
  namespace: app
spec:
  podSelector:
    matchLabels:
      needs-api-access: "true"
  policyTypes:
    - Egress
  egress:
    - to:
        - ipBlock:
            cidr: 10.96.0.1/32    # kubernetes Service ClusterIP — adjust to your cluster
      ports:
        - protocol: TCP
          port: 443
```

Label pods that need API access rather than opening it cluster-wide:

```bash
kubectl label pod <pod-name> -n app needs-api-access=true
```

---

## Step 4 — Allow health check and liveness probe ingress

The kubelet runs on the node — not as a pod — and probes pod liveness and readiness endpoints directly via the node IP. NetworkPolicies cannot select the kubelet by label, so you must allow ingress from the node CIDR.

Find your node CIDR:

```bash
kubectl get nodes -o jsonpath='{.items[*].status.addresses[?(@.type=="InternalIP")].address}'
# Example: 10.0.1.10 10.0.1.11 10.0.1.12
```

Allow kubelet probe ingress:

```yaml
# allow-kubelet-probes.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-kubelet-probes
  namespace: app
spec:
  podSelector: {}
  policyTypes:
    - Ingress
  ingress:
    - from:
        - ipBlock:
            cidr: 10.0.1.0/24    # node CIDR — adjust to your cluster
      ports:
        - protocol: TCP
          port: 8080    # liveness probe port — adjust per workload
        - protocol: TCP
          port: 8081    # readiness probe port — adjust per workload
```

If your workloads use multiple probe ports, list them all. Failing to allow probe ingress causes the kubelet to mark pods as unhealthy and restart them — a subtle and confusing failure mode when rolling out zero-trust policies.

---

## Step 5 — Allow Prometheus metrics scraping

Prometheus scrapes metrics endpoints from outside the pod's namespace. Allow ingress from the `monitoring` namespace to metrics ports:

```yaml
# allow-prometheus-scrape.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-prometheus-scrape
  namespace: app
spec:
  podSelector:
    matchLabels:
      prometheus.io/scrape: "true"
  policyTypes:
    - Ingress
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: monitoring
          podSelector:
            matchLabels:
              app.kubernetes.io/name: prometheus
      ports:
        - protocol: TCP
          port: 9090    # adjust to your metrics port
```

Label pods that expose metrics:

```bash
kubectl label pod <pod-name> -n app prometheus.io/scrape=true
```

---

## Step 6 — Build the application allow-list

With system traffic handled, add the application-level allow policies. For a three-tier application (frontend → app → db):

```yaml
# frontend-to-app.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-frontend-ingress
  namespace: app
spec:
  podSelector:
    matchLabels:
      role: api
  policyTypes:
    - Ingress
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: frontend
          podSelector:
            matchLabels:
              role: frontend
      ports:
        - protocol: TCP
          port: 8080
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-api-egress-to-db
  namespace: app
spec:
  podSelector:
    matchLabels:
      role: api
  policyTypes:
    - Egress
  egress:
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: db
          podSelector:
            matchLabels:
              role: postgres
      ports:
        - protocol: TCP
          port: 5432
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-api-ingress-to-db
  namespace: db
spec:
  podSelector:
    matchLabels:
      role: postgres
  policyTypes:
    - Ingress
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: app
          podSelector:
            matchLabels:
              role: api
      ports:
        - protocol: TCP
          port: 5432
```

```bash
kubectl apply -f frontend-to-app.yaml
```

Verify the full path:

```bash
# frontend → app: should succeed
kubectl exec -n frontend frontend -- curl -s --max-time 3 api.app.svc.cluster.local:8080

# app → db: should succeed
kubectl exec -n app client -- nc -zv postgres.db.svc.cluster.local 5432

# frontend → db direct: should be blocked
kubectl exec -n frontend frontend -- nc -zv postgres.db.svc.cluster.local 5432
```

---

## Step 7 — Policy visibility with Cilium Hubble

If your cluster runs Cilium, Hubble provides real-time visibility into which connections are allowed or dropped by NetworkPolicy. Enable Hubble:

```bash
helm upgrade cilium cilium/cilium \
  --namespace kube-system \
  --reuse-values \
  --set hubble.enabled=true \
  --set hubble.relay.enabled=true \
  --set hubble.ui.enabled=true
```

Install the Hubble CLI:

```bash
HUBBLE_VERSION=$(curl -s https://raw.githubusercontent.com/cilium/hubble/master/stable.txt)
curl -LO "https://github.com/cilium/hubble/releases/download/${HUBBLE_VERSION}/hubble-linux-amd64.tar.gz"
tar -xzf hubble-linux-amd64.tar.gz
mv hubble /usr/local/bin/
```

Port-forward the Hubble relay and observe traffic:

```bash
kubectl port-forward -n kube-system svc/hubble-relay 4245:80 &

# Watch all dropped connections in real time
hubble observe --verdict DROPPED --follow

# Watch traffic for a specific namespace
hubble observe --namespace app --follow

# Show only policy-dropped flows
hubble observe --verdict DROPPED --type policy-denied --follow
```

A dropped connection appears as:

```
Sep  1 12:34:56.789 DROPPED  app/client:45678 -> db/server:80
  Policy: default-deny-all (namespace: db)
```

This tells you exactly which NetworkPolicy dropped the connection and from which pod. Use this during rollout to identify missing allow rules before they cause production issues.

### Calico equivalent — policy visualisation

If using Calico, use `calicoctl` to inspect policy hit counts:

```bash
calicoctl get networkpolicy --all-namespaces -o wide
```

---

## Step 8 — NetworkPolicy for system namespaces

System namespaces (`kube-system`, `cert-manager`, `monitoring`) require careful handling. Applying default-deny to these namespaces can break cluster functionality. The safest approach:

- **Do not apply default-deny to `kube-system`** — the risk of disrupting DNS, the API server, and other critical components outweighs the benefit
- **Apply selective policies to `cert-manager` and `monitoring`** — restrict egress to known targets but allow inbound from all namespaces for metrics scraping

```yaml
# monitoring-ingress-policy.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-all-scrape-ingress
  namespace: monitoring
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: prometheus
  policyTypes:
    - Egress
  egress:
    - {}    # Allow all egress — Prometheus must reach all namespaces
```

The empty `egress: [{}]` rule allows all egress from the selected pods — use this only for Prometheus and similar cluster-wide scrapers that legitimately need unrestricted outbound.

---

## Step 9 — Audit your zero-trust posture

Regularly verify that no unexpected traffic paths exist. Use a simple audit loop:

```bash
#!/bin/bash
# zero-trust-audit.sh

NAMESPACES=$(kubectl get ns -o jsonpath='{.items[*].metadata.name}')

echo "=== Namespaces without default-deny-all policy ==="
for ns in $NAMESPACES; do
  has_deny=$(kubectl get networkpolicy -n $ns \
    -o jsonpath='{.items[?(@.spec.podSelector=={})].metadata.name}' 2>/dev/null)
  if [[ -z "$has_deny" ]]; then
    echo "  MISSING: $ns"
  fi
done

echo ""
echo "=== NetworkPolicy count per namespace ==="
for ns in $NAMESPACES; do
  count=$(kubectl get networkpolicy -n $ns --no-headers 2>/dev/null | wc -l)
  echo "  $ns: $count policies"
done
```

```bash
chmod +x zero-trust-audit.sh && ./zero-trust-audit.sh
```

Any namespace listed under "MISSING" has no default-deny policy and is operating in default-allow mode — a gap in your zero-trust posture.

---

## What you have built

- Cluster-wide default-deny-all for both ingress and egress
- Explicit DNS egress restored to all application namespaces
- Kubelet probe ingress permitted via node CIDR
- Prometheus metrics scraping allowed from the monitoring namespace
- Three-tier application allow-list (frontend → app → db)
- Real-time policy visibility with Cilium Hubble or Calico policy stats
- System namespace handling strategy
- An automated audit script to detect namespaces missing default-deny policies

Your cluster now enforces zero-trust networking: all traffic is denied unless explicitly permitted. Lateral movement after a pod compromise is constrained to only the paths declared in your NetworkPolicy allow-list.
