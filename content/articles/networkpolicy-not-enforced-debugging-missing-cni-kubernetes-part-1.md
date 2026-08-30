---
title: "The NetworkPolicy That Did Nothing — Part 1: Debugging a Missing CNI in Kubernetes"
date: 2026-08-06
author: "Victor D"
description: "A NetworkPolicy was applied, the rules looked correct, but traffic kept flowing. The root cause had nothing to do with the policy itself — it was that no CNI plugin was running at all."
tags: ["kubernetes", "networkpolicy", "cni", "calico", "security", "rhel", "networking", "debugging"]
categories: ["article"]
draft: false
toc: true
---

The NetworkPolicy was there. The YAML was correct. The selector matched the right pods. And yet `adservice` could still reach `paymentservice:50051` without any trouble.

This is the story of a policy that did absolutely nothing — not because it was misconfigured, but because the enforcement engine was never running in the first place.

---

## The environment

The cluster was running on RHEL 8 nodes with `kubeadm`. The goal was straightforward: block all ingress to `paymentservice` except from explicitly allowed pods. A `NetworkPolicy` was applied to the `production` namespace, the syntax checked out, and yet connectivity tests kept passing.

```bash
# Test from adservice pod — should have been blocked
kubectl exec -it adservice-7d9f8b6c4-xk2p1 -n production -- \
  nc -zv paymentservice 50051
# Connection to paymentservice 50051 port [tcp/*] succeeded!
```

---

## Why the policy had no effect

The first thing to check when a NetworkPolicy appears to be ignored is whether a CNI plugin is actually running.

```bash
kubectl get pods -n kube-system
```

No Calico pods. No Cilium pods. No Weave pods. The CNI directory on the nodes had no plugin binary.

This is the critical thing to understand about Kubernetes NetworkPolicy: **the API object is just metadata**. The kubelet accepts it, stores it in etcd, and returns a success response. But nothing happens to your iptables rules until a CNI plugin reads that object and programs the dataplane.

Kubernetes itself is fail-open by default. With no CNI enforcing policy, every pod can reach every other pod regardless of what NetworkPolicy objects exist in the cluster. The policies are stored but completely inert.

```bash
# Confirm no CNI binary is present on the node
ls /etc/cni/net.d/
# (empty)

ls /opt/cni/bin/
# (only loopback — no calico, cilium, or flannel binary)
```

---

## Why Calico wasn't running

Calico had been installed — the manifest had been applied — but the `calico-node` DaemonSet pods were crash-looping. The reason emerged from the logs:

```bash
kubectl logs -n kube-system calico-node-xxxxx
# Error: IPAM configuration error: IPPool CIDR 192.168.0.0/16
# does not match the node's pod CIDR 10.244.0.0/16
```

The Calico `Installation` custom resource had shipped with its default IP pool: `192.168.0.0/16`. The cluster had been initialised with `--pod-network-cidr=10.244.0.0/16`. These two values have to match, and they didn't.

The Calico node agent detected the mismatch, refused to configure the dataplane, and exited. The DaemonSet kept restarting it. The pods kept crashing. No CNI enforcement was ever active.

---

## The fix

Edit the Calico `Installation` resource to use the actual cluster pod CIDR:

```bash
kubectl edit installation default -n calico-system
```

```yaml
# Change this:
spec:
  calicoNetwork:
    ipPools:
    - cidr: 192.168.0.0/16
      encapsulation: VXLANCrossSubnet

# To this:
spec:
  calicoNetwork:
    ipPools:
    - cidr: 10.244.0.0/16
      encapsulation: VXLANCrossSubnet
```

After saving, Calico's operator reconciled the change and the `calico-node` DaemonSet came up healthy:

```bash
kubectl get pods -n kube-system | grep calico
# calico-node-xxxxx   1/1   Running   0   45s
# calico-node-yyyyy   1/1   Running   0   45s
```

---

## Verifying enforcement

With `calico-node` running, Felix — Calico's policy enforcement agent — programmed the iptables rules. The same connectivity test now produced a different result:

```bash
kubectl exec -it adservice-7d9f8b6c4-xk2p1 -n production -- \
  nc -zv paymentservice 50051 -w 5
# nc: connect to paymentservice port 50051 (tcp) timed out: Operation now in progress
```

The connection timed out instead of succeeding. The NetworkPolicy was finally being enforced.

You can also inspect the iptables rules Calico installed directly on the node:

```bash
# On the RHEL 8 node
iptables -L --line-numbers | grep cali
# cali-INPUT, cali-FORWARD chains visible — Felix is active
```

---

## What to verify after any CNI installation

This incident is easy to avoid if you build CNI verification into your cluster bootstrap checklist.

**1. Confirm the DaemonSet is healthy before applying any NetworkPolicy:**

```bash
kubectl rollout status daemonset/calico-node -n kube-system
```

**2. Verify the IP pool CIDR matches the cluster pod CIDR:**

```bash
# What CIDR did kubeadm use?
kubectl get cm kubeadm-config -n kube-system -o yaml | grep podSubnet

# What CIDR does Calico think it should use?
kubectl get installation default -n calico-system -o yaml | grep cidr
```

These two values must match.

**3. Test a default-deny policy in a non-production namespace first:**

```bash
# Apply deny-all, verify it blocks, then allow specific traffic
kubectl apply -f default-deny-all.yaml -n sandbox
kubectl exec -it test-pod -n sandbox -- nc -zv target-svc 8080 -w 3
# Should time out — if it connects, CNI is not enforcing
```

**4. Confirm iptables rules are being managed by the CNI agent:**

```bash
iptables -L FORWARD --line-numbers
# Should show calico-FORWARD or cali-FORWARD chain if Calico is active
```

---

## Recommendations

**Apply a default-deny policy in every production namespace.** The Kubernetes default is allow-all. An explicit default-deny posture means any misconfigured pod that escapes your NetworkPolicy rules hits a deny rather than an accidental permit.

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny-all
  namespace: production
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  - Egress
```

**Monitor CNI pod health.** If `calico-node` or `cilium` pods are not ready, your NetworkPolicies are silent. Add an alert on DaemonSet availability for kube-system components.

**Check the Kubernetes audit log.** Even when CNI is absent, the API server records every NetworkPolicy admission. If you see policies accepted but no CNI enforcement is happening, the audit log confirms the policies were received — making the CNI the obvious next thing to check.

---

{{< callout type="warning" title="Compliance note: STIG and HIPAA environments" >}}
RHEL 8 systems in DISA STIG or HIPAA-regulated environments must enforce network segmentation at the pod level. A NetworkPolicy object in etcd is not sufficient for compliance — auditors expect demonstrable CNI enforcement. If Calico or Cilium is not running, you have a policy gap regardless of what your YAML says. Include CNI DaemonSet health in your compliance evidence collection.
{{< /callout >}}

---

## What this incident teaches

Kubernetes is designed to accept configuration that has no immediate effect. This is by design — it allows control plane components to start up in any order and converge. But it means the absence of an error is not proof that your configuration is active.

NetworkPolicy is the clearest example. The API accepts the object unconditionally. Whether it does anything depends entirely on an external component that the Kubernetes control plane itself does not validate.

If a security control is critical, verify it at the dataplane — not at the API. A policy that exists in etcd but has no enforcement engine is not a security control. It is a document.

---

## Further reading

- [Kubernetes NetworkPolicy documentation](https://kubernetes.io/docs/concepts/services-networking/network-policies/)
- [Calico installation guide — custom pod CIDR](https://docs.tigera.io/calico/latest/getting-started/kubernetes/self-managed-onprem/onpremises)
- [Calico IP pool configuration](https://docs.tigera.io/calico/latest/networking/ipam/ip-pools)
- [Felix — Calico's policy enforcement agent](https://docs.tigera.io/calico/latest/reference/architecture/overview)
- [Kubernetes CNI specification](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/network-plugins/)
