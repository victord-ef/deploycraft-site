---
title: "Helm vs Kustomize — Part 1: Picking the Right Templating Approach"
date: 2026-08-06
author: "Victor D"
description: "Helm and Kustomize solve different problems despite often being compared as alternatives. Understanding what each tool actually is — not just what it does — makes the choice obvious for most use cases."
tags: ["helm", "kustomize", "kubernetes", "gitops", "devops", "platform-engineering", "comparisons"]
categories: ["article"]
draft: false
toc: true
---

The first thing to understand about Helm vs Kustomize is that you are not always choosing between them. They solve different problems, and many production environments use both — Kustomize layered on top of Helm charts, or Helm charts alongside Kustomize-managed manifests.

That said, the choice of which to reach for first — for your own application manifests, your GitOps overlays, your environment configuration — is a real decision with real consequences. This article explains what each tool actually is, where each one fits naturally, and where they start to fight you.

---

## What Helm actually is

Helm is a **package manager** for Kubernetes. The analogy to `apt`, `yum`, or `brew` is intentional and accurate. Its primary design goal is distributing pre-packaged Kubernetes applications so that others can install, configure, and upgrade them without understanding every manifest inside.

A Helm chart is a versioned, distributable package. A `helm install` creates a **release** — a named, tracked instance of a chart in a cluster. Helm stores release state as Secrets in the cluster, which is what enables `helm rollback`, `helm history`, and `helm status`.

The templating system — Go templates in `templates/` rendered against `values.yaml` — is the mechanism that makes charts configurable. It is not the point. The point is packaging.

```
mychart/
├── Chart.yaml          # chart metadata: name, version, dependencies
├── values.yaml         # default configuration values
├── templates/
│   ├── deployment.yaml
│   ├── service.yaml
│   └── _helpers.tpl    # reusable template snippets
└── charts/             # bundled dependency charts
```

A typical Helm template:

```yaml
# templates/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "mychart.fullname" . }}
  labels:
    {{- include "mychart.labels" . | nindent 4 }}
spec:
  replicas: {{ .Values.replicaCount }}
  selector:
    matchLabels:
      {{- include "mychart.selectorLabels" . | nindent 6 }}
  template:
    spec:
      containers:
      - name: {{ .Chart.Name }}
        image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
        resources:
          {{- toYaml .Values.resources | nindent 10 }}
```

And the values a user provides to configure it:

```yaml
# values.yaml (or a custom override file)
replicaCount: 3
image:
  repository: your-org/api-server
  tag: "1.4.2"
resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 512Mi
```

This is powerful for distribution. Someone can `helm install my-release oci://registry.example.com/charts/api-server --version 1.4.2` and get a working application without reading a single template.

---

## What Kustomize actually is

Kustomize is a **configuration transformer**. It takes valid Kubernetes YAML as input and produces modified Kubernetes YAML as output — no templating language involved. The output is always plain YAML that you could apply directly with `kubectl`.

The core concept is **base and overlays**:

```
k8s/
├── base/
│   ├── kustomization.yaml
│   ├── deployment.yaml
│   └── service.yaml
└── overlays/
    ├── staging/
    │   └── kustomization.yaml
    └── production/
        └── kustomization.yaml
```

The base contains your canonical manifests. Overlays apply environment-specific transformations — patches, replica counts, resource limits, image tags — without duplicating the base YAML.

```yaml
# base/deployment.yaml — plain, unmodified Kubernetes YAML
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api-server
spec:
  replicas: 1
  template:
    spec:
      containers:
      - name: api-server
        image: your-org/api-server:latest
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
```

```yaml
# overlays/production/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- ../../base
namePrefix: prod-
commonLabels:
  environment: production
patches:
- patch: |-
    - op: replace
      path: /spec/replicas
      value: 5
    - op: replace
      path: /spec/template/spec/containers/0/image
      value: your-org/api-server:1.4.2
  target:
    kind: Deployment
    name: api-server
```

Running `kubectl apply -k overlays/production/` renders the base with the production overlay applied and applies the result. No release tracking, no versioning, no registry — just transformed YAML.

Kustomize is built into `kubectl` since version 1.14. You do not need to install anything.

---

## Where Helm wins

**Distributing software to others.** If you are building a product that others install in their clusters, Helm is the correct tool. Versioned charts, Artifact Hub, OCI registries, `helm pull` — the entire distribution infrastructure exists because Helm was designed for this. Kustomize has no equivalent concept of a distributable package.

**Third-party software you consume.** Prometheus, cert-manager, Istio, Loki, ArgoCD, ExternalDNS — all of these publish official Helm charts. Installing them with Helm is the path of least resistance and keeps you on a supported upgrade track.

```bash
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm install kube-prometheus-stack prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --create-namespace \
  --version 61.0.0 \
  -f my-values.yaml
```

**Many configuration parameters.** When a workload genuinely needs dozens of configuration knobs — feature flags, database URLs, resource profiles, TLS options, replica counts — Helm's values system handles this cleanly. A `values.yaml` with sensible defaults and well-documented overrides is much easier to manage than a pile of JSON patches.

**Versioned releases with rollback.** Helm tracks every release in the cluster. If a deployment breaks, `helm rollback my-release 3` returns to revision 3 in one command. Kustomize has no equivalent — rollback is a Git revert followed by a re-apply.

```bash
helm history api-server -n production
# REVISION  STATUS     CHART              DESCRIPTION
# 1         superseded api-server-1.2.0   Install complete
# 2         superseded api-server-1.3.1   Upgrade complete
# 3         deployed   api-server-1.4.2   Upgrade complete

helm rollback api-server 2 -n production
```

---

## Where Kustomize wins

**Your own application manifests in a GitOps workflow.** When you own the manifests and need to maintain multiple environment variants, Kustomize's base-and-overlay model is a natural fit. The Git repository becomes the source of truth without any rendering layer between what you read and what gets applied.

**No templating language to learn or debug.** Go templates are powerful but unpleasant. Debugging `{{- if and .Values.ingress.enabled (not .Values.ingress.className) -}}` at 2am during an incident is not pleasant. Kustomize's patches are plain YAML operations — strategic merge patches and JSON 6902 patches — that are easier to read and reason about.

**ConfigMap and Secret generation.** Kustomize has first-class support for generating ConfigMaps and Secrets from files or literals, with automatic hash suffixing that triggers rolling updates when the content changes:

```yaml
# kustomization.yaml
configMapGenerator:
- name: app-config
  literals:
  - LOG_LEVEL=info
  - MAX_CONNECTIONS=100
  files:
  - configs/nginx.conf

secretGenerator:
- name: db-credentials
  envs:
  - secrets/.env.production
```

The generated ConfigMap name gets a hash suffix (`app-config-8f92kd`). Every change to the content produces a new hash, which forces a Deployment rollout — no manual restarts needed.

**Built into kubectl.** `kubectl apply -k ./overlays/production` works anywhere `kubectl` works. No Helm installation, no version pinning, no helm binary to manage. In environments where you cannot install additional tooling — locked-down CI runners, restricted nodes — this matters.

---

## Where each one fights you

**Helm fights you when:**

The Go template syntax becomes the complexity you manage instead of the Kubernetes configuration. Large, heavily-templated charts become difficult to read and reason about. `helm template` helps — it renders the chart without installing it — but debugging template logic is still painful.

```bash
# Render chart locally to inspect output before applying
helm template my-release ./mychart -f my-values.yaml | less
```

Helm also fights you when you need to make small tweaks to a third-party chart that the chart author did not expose as a value. You cannot add a label or patch a field that the template does not support — you have to fork the chart or use a post-renderer.

**Kustomize fights you when:**

Overlays get deep. Three or four levels of inheritance — base, shared-dev, team-dev, feature-branch — become hard to reason about. When something is wrong in the rendered output, tracing which overlay introduced it requires checking every layer.

Kustomize also has no native way to express "give me version 1.4.2 of this application." Version is just an image tag in a patch, managed by whatever process updates that value. If you want a formal version history or rollback mechanism, you have to build it yourself — usually via Git tags and CI automation.

---

## Using them together

The most common production pattern is not a choice between them — it is both:

**Kustomize on top of Helm (post-rendering).** You install a third-party Helm chart but need to add annotations, change a label, or patch a field the chart does not expose. Kustomize can transform the rendered Helm output before it reaches the cluster.

Flux supports this natively in a `HelmRelease`:

```yaml
apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: cert-manager
  namespace: flux-system
spec:
  chart:
    spec:
      chart: cert-manager
      version: "v1.15.*"
      sourceRef:
        kind: HelmRepository
        name: jetstack
  values:
    installCRDs: true
  postRenderers:
  - kustomize:
      patches:
      - patch: |-
          - op: add
            path: /metadata/annotations/custom.example.com~1team
            value: platform
        target:
          kind: Deployment
```

ArgoCD also supports Helm-then-Kustomize in its `Application` source configuration.

**Helm for third-party, Kustomize for yours.** Install Prometheus, cert-manager, and ExternalDNS with Helm. Manage your own application manifests with Kustomize overlays. The two live alongside each other in the same cluster without conflict.

---

## How to choose

**Use Helm when you are distributing or consuming packaged software.** Third-party charts, internal shared platforms, software that other teams install with configuration — Helm is the right tool. Its versioning, release tracking, and values system are designed for this.

**Use Kustomize when you are managing your own manifests across environments.** Base-and-overlay for dev/staging/production is exactly what Kustomize was built for. Plain YAML, no templating language, built into kubectl.

**Use both when you need to customise what Helm gives you.** Post-rendering with Kustomize lets you patch third-party charts without forking them.

The question to ask is not "which tool is better" but "what am I doing with this YAML?" If you are packaging it for others or consuming a package, reach for Helm. If you are transforming your own configuration for different environments, reach for Kustomize.

---

## Related reading

- [Helm documentation](https://helm.sh/docs/)
- [Kustomize documentation](https://kubectl.docs.kubernetes.io/references/kustomize/)
- [Flux HelmRelease with Kustomize post-renderer](https://fluxcd.io/flux/components/helm/helmreleases/#post-renderers)
- Flux vs ArgoCD — how to choose for your team → **Articles**
- Helm is not Infrastructure as Code — and that matters → **Articles**
