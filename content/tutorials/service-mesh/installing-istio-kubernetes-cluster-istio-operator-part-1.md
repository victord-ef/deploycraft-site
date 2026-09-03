---
title: "Installing Istio on a Kubernetes Cluster with the Istio Operator — Part 1"
date: 2026-09-03
description: "Install Istio using istioctl and the IstioOperator API, configure ingress and egress gateways, enable namespace injection, verify mTLS is active, and harden the installation for production with external CA integration and control plane high availability."
cluster: "Service Mesh"
series: "Installing & Configuring Istio"
part: 1
difficulty: "intermediate"
duration: "45 min"
tags: ["service-mesh", "istio", "kubernetes", "mtls", "ingress", "devops", "devsecops", "networking"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

By the end of this tutorial you will have a production-grade Istio installation on a Kubernetes cluster: control plane deployed via the `IstioOperator` API, ingress and egress gateways configured, namespace-level sidecar injection enabled, mTLS verified with `istioctl`, and the installation hardened with an external CA and high-availability control plane. Part 2 builds on this by configuring traffic management with `VirtualService` and `DestinationRule` resources.

## Prerequisites

- A Kubernetes cluster (1.28+) with `kubectl` configured — EKS, GKE, AKS, or k3s all work
- At least 4 vCPUs and 8 GB memory available across the cluster
- `helm` installed (used for some extension components)
- Familiarity with [Service Mesh Foundations Part 1](/tutorials/service-mesh/introduction-service-mesh-concepts-architecture-kubernetes-part-1/) — sidecar model, mTLS, control/data plane

---

## Step 1 — Install the istioctl CLI

`istioctl` is the primary tool for installing and troubleshooting Istio. It is separate from the in-cluster operator.

```bash
# Download the latest stable release
curl -L https://istio.io/downloadIstio | ISTIO_VERSION=1.23.0 sh -
cd istio-1.23.0
export PATH=$PWD/bin:$PATH

# Verify
istioctl version --remote=false
# client version: 1.23.0

# Run preflight checks against the cluster
istioctl x precheck
# ✔ No issues found when checking the cluster. Istio is safe to install or upgrade!
```

Add `istioctl` to your PATH permanently:

```bash
# Linux / macOS
sudo cp bin/istioctl /usr/local/bin/

# Verify cluster compatibility
istioctl x precheck
```

---

## Step 2 — Understand IstioOperator profiles

Istio ships with several built-in profiles that control which components are installed:

```bash
# List available profiles
istioctl profile list
# Istio configuration profiles:
#   ambient
#   default
#   demo
#   empty
#   minimal
#   openshift
#   preview
#   remote
```

| Profile | Components installed | Use for |
|---|---|---|
| `minimal` | Istiod only | Clusters with an existing ingress controller |
| `default` | Istiod + ingressgateway | Standard production installs |
| `demo` | All components, permissive mTLS, debug logging | Learning and demos — not for production |
| `ambient` | Istiod + ztunnel (no sidecars) | Ambient mode adoption |
| `empty` | Nothing | Full custom installs via `IstioOperator` |
| `remote` | Remote cluster components only | Multi-cluster secondary clusters |

Inspect what a profile installs before applying:

```bash
istioctl profile dump default
```

---

## Step 3 — Install Istio with a custom IstioOperator

Rather than using a profile directly, define an `IstioOperator` manifest so the installation is version-controlled and reproducible:

```yaml
# istio-operator.yaml
apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
metadata:
  name: control-plane
  namespace: istio-system
spec:
  profile: default
  meshConfig:
    # Enable access logging to stdout (JSON format)
    accessLogFile: /dev/stdout
    accessLogEncoding: JSON
    # Default proxy settings
    defaultConfig:
      # Concurrency: number of worker threads per proxy (0 = auto = num CPUs)
      concurrency: 2
      # Tracing
      tracing:
        sampling: 100.0    # 100% in staging, reduce in production
        zipkin:
          address: jaeger-collector.observability:9411
    # Outbound traffic policy: REGISTRY_ONLY blocks calls to unregistered services
    outboundTrafficPolicy:
      mode: REGISTRY_ONLY
    # Enable locality load balancing
    localityLbSetting:
      enabled: true

  components:
    pilot:
      k8s:
        replicaCount: 2
        resources:
          requests:
            cpu: 500m
            memory: 2Gi
          limits:
            cpu: 1000m
            memory: 4Gi
        hpaSpec:
          minReplicas: 2
          maxReplicas: 5
          metrics:
            - type: Resource
              resource:
                name: cpu
                target:
                  type: Utilization
                  averageUtilization: 80
        affinity:
          podAntiAffinity:
            preferredDuringSchedulingIgnoredDuringExecution:
              - weight: 100
                podAffinityTerm:
                  labelSelector:
                    matchLabels:
                      app: istiod
                  topologyKey: kubernetes.io/hostname

    ingressGateways:
      - name: istio-ingressgateway
        enabled: true
        k8s:
          replicaCount: 2
          service:
            type: LoadBalancer
            ports:
              - name: http2
                port: 80
                targetPort: 8080
              - name: https
                port: 443
                targetPort: 8443
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 1000m
              memory: 1Gi
          hpaSpec:
            minReplicas: 2
            maxReplicas: 10

    egressGateways:
      - name: istio-egressgateway
        enabled: true
        k8s:
          replicaCount: 1
          resources:
            requests:
              cpu: 100m
              memory: 128Mi

  values:
    global:
      # Proxy resource requests — set per your workload density
      proxy:
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 256Mi
      # Log level for sidecars (warning in production, info for debugging)
      logging:
        level: "default:warning"
```

Apply the installation:

```bash
kubectl create namespace istio-system

istioctl install -f istio-operator.yaml -y

# Watch installation progress
kubectl get pods -n istio-system -w
# istiod-xxx              1/1   Running
# istio-ingressgateway-xxx 1/1   Running
# istio-egressgateway-xxx  1/1   Running
```

Verify with `istioctl`:

```bash
istioctl verify-install -f istio-operator.yaml
# ✔ Istio is installed and verified successfully
```

---

## Step 4 — Enable sidecar injection

Label namespaces to enable automatic sidecar injection:

```bash
# Enable injection for application namespaces
kubectl label namespace my-app istio-injection=enabled
kubectl label namespace payments istio-injection=enabled
kubectl label namespace checkout istio-injection=enabled

# Explicitly disable injection for system namespaces
kubectl label namespace kube-system istio-injection=disabled
kubectl label namespace kube-public istio-injection=disabled
kubectl label namespace kube-node-lease istio-injection=disabled

# Restart existing pods to inject sidecars
kubectl rollout restart deployment -n my-app
kubectl rollout restart deployment -n payments
kubectl rollout restart deployment -n checkout
```

Verify injection:

```bash
# Each pod should show 2/2 containers (app + istio-proxy)
kubectl get pods -n my-app
# NAME               READY   STATUS    RESTARTS
# my-app-xxx         2/2     Running   0

# Describe a pod to see the injected sidecar
kubectl describe pod my-app-xxx -n my-app | grep "istio-proxy"
# istio-proxy:
#   Image: docker.io/istio/proxyv2:1.23.0
```

Check injection status across all namespaces:

```bash
kubectl get namespaces --show-labels | grep istio-injection
# my-app      Active   istio-injection=enabled
# payments    Active   istio-injection=enabled
```

---

## Step 5 — Verify mTLS is active

After injection, verify that mTLS is working between services:

```bash
# Check mTLS status for a specific pod
istioctl x describe pod my-app-xxx.my-app
# Pod: my-app-xxx
# Pod Ports: 8080 (my-app)
# ----
# Service: my-app
# MatchedPolicy: /default
# mTLS: yes

# Check mTLS status across a namespace
istioctl x describe svc my-app -n my-app
# Service: my-app/my-app
# MatchedPolicies:
# PeerAuthentication: default/istio-system
# mTLS: yes (strict)
```

Apply strict mTLS cluster-wide:

```yaml
# strict-mtls.yaml
apiVersion: security.istio.io/v1
kind: PeerAuthentication
metadata:
  name: default
  namespace: istio-system    # mesh-wide policy
spec:
  mtls:
    mode: STRICT
```

```bash
kubectl apply -f strict-mtls.yaml

# Test: a pod without a sidecar can no longer call a mesh-injected service
kubectl run test --image=curlimages/curl --rm -it --restart=Never \
  -- curl http://my-app.my-app:8080/health
# curl: (56) Recv failure: Connection reset by peer
# ← expected: STRICT mTLS rejects plaintext connections
```

---

## Step 6 — Configure the Ingress Gateway

The Istio Ingress Gateway is the entry point for traffic from outside the cluster into the mesh. Configure it with a `Gateway` and `VirtualService`:

```yaml
# gateway.yaml
apiVersion: networking.istio.io/v1
kind: Gateway
metadata:
  name: my-app-gateway
  namespace: my-app
spec:
  selector:
    istio: ingressgateway    # targets the default ingress gateway
  servers:
    - port:
        number: 443
        name: https
        protocol: HTTPS
      tls:
        mode: SIMPLE
        credentialName: my-app-tls    # Kubernetes Secret with TLS cert/key
      hosts:
        - "my-app.example.com"
    - port:
        number: 80
        name: http
        protocol: HTTP
      hosts:
        - "my-app.example.com"
      tls:
        httpsRedirect: true    # redirect HTTP to HTTPS
```

```yaml
# virtualservice-ingress.yaml
apiVersion: networking.istio.io/v1
kind: VirtualService
metadata:
  name: my-app
  namespace: my-app
spec:
  hosts:
    - "my-app.example.com"
  gateways:
    - my-app-gateway
  http:
    - match:
        - uri:
            prefix: /api
      route:
        - destination:
            host: my-app
            port:
              number: 8080
```

Create the TLS secret (using cert-manager or a manually provisioned certificate):

```bash
kubectl create secret tls my-app-tls \
  --namespace my-app \
  --cert=path/to/tls.crt \
  --key=path/to/tls.key
```

Get the ingress gateway external IP:

```bash
kubectl get svc istio-ingressgateway -n istio-system
# NAME                   TYPE           CLUSTER-IP    EXTERNAL-IP     PORT(S)
# istio-ingressgateway   LoadBalancer   10.96.1.100   203.0.113.42    80:31380/TCP,443:31390/TCP

# Test the gateway
curl -k https://my-app.example.com/api/health
```

---

## Step 7 — Configure the Egress Gateway

The egress gateway controls and monitors outbound traffic from the mesh to external services. Combined with `REGISTRY_ONLY` outbound policy, it enforces that pods can only reach external hosts that are explicitly declared:

```yaml
# serviceentry-external-api.yaml
apiVersion: networking.istio.io/v1
kind: ServiceEntry
metadata:
  name: stripe-api
  namespace: payments
spec:
  hosts:
    - api.stripe.com
  ports:
    - number: 443
      name: https
      protocol: HTTPS
  resolution: DNS
  location: MESH_EXTERNAL
```

```yaml
# egress-gateway-stripe.yaml
apiVersion: networking.istio.io/v1
kind: Gateway
metadata:
  name: egress-stripe
  namespace: istio-system
spec:
  selector:
    istio: egressgateway
  servers:
    - port:
        number: 443
        name: https
        protocol: HTTPS
      hosts:
        - api.stripe.com
      tls:
        mode: PASSTHROUGH
---
apiVersion: networking.istio.io/v1
kind: VirtualService
metadata:
  name: stripe-via-egress
  namespace: payments
spec:
  hosts:
    - api.stripe.com
  gateways:
    - mesh                    # applies to sidecar → sidecar traffic
    - istio-system/egress-stripe
  tls:
    - match:
        - gateways:
            - mesh
          port: 443
          sniHosts:
            - api.stripe.com
      route:
        - destination:
            host: istio-egressgateway.istio-system.svc.cluster.local
            port:
              number: 443
    - match:
        - gateways:
            - istio-system/egress-stripe
          port: 443
          sniHosts:
            - api.stripe.com
      route:
        - destination:
            host: api.stripe.com
            port:
              number: 443
```

With this configuration, all outbound calls from pods in the `payments` namespace to `api.stripe.com` are routed through the egress gateway — creating a single auditable egress point visible in the gateway's access logs.

---

## Step 8 — Plug in an external CA (production hardening)

By default, Istiod acts as its own root CA. For production, replace this with a certificate signed by your organisation's PKI:

```bash
# Generate an intermediate CA signed by your root CA
# (using cfssl, openssl, or your PKI system)

# Create the cacerts secret that Istiod reads on startup
kubectl create secret generic cacerts \
  --namespace istio-system \
  --from-file=ca-cert.pem \
  --from-file=ca-key.pem \
  --from-file=root-cert.pem \
  --from-file=cert-chain.pem

# Restart Istiod to pick up the new CA
kubectl rollout restart deployment istiod -n istio-system

# Verify the new CA is in use
istioctl x check-inject -n my-app
# ✔ Namespace my-app is configured for injection

# Inspect a workload certificate to confirm it chains to your root CA
istioctl x workload-certs get my-app-xxx.my-app
# Certificate chain:
#   Subject: spiffe://cluster.local/ns/my-app/sa/my-app
#   Issuer:  CN=Istio CA
#   Valid until: 2026-09-04 (24h)
```

**Certificate lifetime tuning:**

```yaml
# IstioOperator patch — reduce cert lifetime for high-security environments
spec:
  meshConfig:
    defaultConfig:
      proxyMetadata:
        SECRET_TTL: "3600s"    # 1-hour workload cert lifetime
```

---

## Step 9 — Observability: Kiali, Prometheus, and Jaeger

Install the Istio observability addons (for evaluation — use production-grade installations in live clusters):

```bash
# Install Prometheus, Grafana, Kiali, and Jaeger from the Istio samples
kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.23/samples/addons/prometheus.yaml
kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.23/samples/addons/grafana.yaml
kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.23/samples/addons/kiali.yaml
kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.23/samples/addons/jaeger.yaml

# Open the Kiali service graph dashboard
istioctl dashboard kiali

# Open Grafana (Istio workload and service dashboards)
istioctl dashboard grafana

# Open Jaeger (distributed traces)
istioctl dashboard jaeger
```

Kiali provides a live service topology graph showing request rates, error rates, and mTLS status on every edge — without any application instrumentation.

---

## Step 10 — Upgrade and uninstall

### Upgrade Istio

Istio supports in-place upgrades and canary upgrades (running two control plane versions simultaneously):

```bash
# Canary upgrade — install a new control plane revision alongside the existing one
istioctl install -f istio-operator.yaml \
  --set revision=1-23-1    # new revision tag

# Migrate namespaces to the new revision incrementally
kubectl label namespace my-app \
  istio-injection-  \       # remove the old label
  istio.io/rev=1-23-1       # add the revision label

kubectl rollout restart deployment -n my-app

# After all namespaces are migrated, remove the old control plane
istioctl uninstall --revision=1-23-0
```

### Uninstall

```bash
# Remove Istio (leaves CRDs in place)
istioctl uninstall --purge -y

# Remove the namespace
kubectl delete namespace istio-system

# Remove CRDs
kubectl get crd | grep istio.io | awk '{print $1}' | xargs kubectl delete crd
```

---

## What you have built

- `istioctl` installed with preflight checks, and a tour of the built-in installation profiles
- A version-controlled `IstioOperator` manifest with: 2-replica HA Istiod, HPA, pod anti-affinity, configurable proxy resources, access logging, tracing, and `REGISTRY_ONLY` outbound policy
- Namespace injection labelling with explicit exclusions for system namespaces
- mTLS verification via `istioctl x describe` and enforcement via cluster-wide `PeerAuthentication STRICT`
- Ingress Gateway configured with `Gateway` + `VirtualService`: TLS termination, HTTP→HTTPS redirect, path-based routing
- Egress Gateway with `ServiceEntry` + `VirtualService`: all traffic to external hosts routed through a single auditable egress point
- External CA integration: replacing Istiod's self-signed root with an organisation PKI intermediate
- Certificate lifetime tuning via `proxyMetadata.SECRET_TTL`
- Kiali, Prometheus, Grafana, and Jaeger observability addon installation
- Canary upgrade procedure (two control plane revisions, incremental namespace migration) and clean uninstall

In [Part 2](/tutorials/service-mesh/configuring-traffic-management-istio-virtualservices-destinationrules-part-2/) you will configure fine-grained traffic management: weighted routing between service versions, header-based routing for A/B testing, fault injection for chaos testing, circuit breaking with outlier detection, and locality-aware load balancing.
