---
title: "Automate a Custom Metrics Exporter Deployment with GitHub Actions"
description: "Build, push to GitHub Container Registry, and deploy a custom Prometheus metrics exporter to Kubernetes automatically on every push using GitHub Actions."
weight: 10
toc: true
---

This guide automates the manual steps from
[Create a custom metrics exporter](/docs/kubernetes/custom-metrics-exporter/)
into a single GitHub Actions pipeline. Every push to `main` builds the Go
exporter, pushes the image to GitHub Container Registry (GHCR), and applies
the updated Kubernetes manifests — no manual `docker build` or `kubectl apply`
needed.

## Prerequisites

- The Go exporter source, `Dockerfile`, and Kubernetes manifests committed in
  one GitHub repository.
- A running Kubernetes cluster with Prometheus installed.
- `kubectl` access to the cluster from your local machine (to export the
  kubeconfig secret).

## Repository structure

Organize your repository so the workflow can find everything it needs:

```
my-exporter/
├── .github/
│   └── workflows/
│       └── deploy.yml        # pipeline defined in this guide
├── k8s/
│   ├── deployment.yaml
│   └── service.yaml
├── main.go
├── go.mod
├── go.sum
└── Dockerfile
```

## Step 1 — Store secrets in GitHub

The pipeline needs two secrets.

### KUBECONFIG

Export your kubeconfig as a base64 string and save it as a GitHub secret:

```bash
cat ~/.kube/config | base64 | tr -d '\n'
```

Copy the output, then go to:

**GitHub → your repo → Settings → Secrets and variables → Actions → New repository secret**

- Name: `KUBECONFIG_B64`
- Value: the base64 string you copied

### GITHUB_TOKEN

GHCR authentication uses the built-in `GITHUB_TOKEN` — no extra secret needed.
Make sure your repository has **Read and write** permissions enabled:

**Settings → Actions → General → Workflow permissions → Read and write permissions**

## Step 2 — Update the Deployment manifest

The pipeline injects the image tag at deploy time, so use a placeholder in
`k8s/deployment.yaml` that the workflow replaces with `sed`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-exporter
  namespace: monitoring
  labels:
    app: my-exporter
spec:
  replicas: 1
  selector:
    matchLabels:
      app: my-exporter
  template:
    metadata:
      labels:
        app: my-exporter
    spec:
      containers:
      - name: exporter
        image: IMAGE_PLACEHOLDER
        ports:
        - name: metrics
          containerPort: 8080
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
        resources:
          requests:
            cpu: 50m
            memory: 32Mi
          limits:
            cpu: 100m
            memory: 64Mi
```

The workflow replaces `IMAGE_PLACEHOLDER` with the full GHCR image URI before
applying the manifest.

## Step 3 — Write the workflow

Create `.github/workflows/deploy.yml`:

```yaml
name: Build and Deploy

on:
  push:
    branches:
      - main

env:
  REGISTRY: ghcr.io
  IMAGE_NAME: ${{ github.repository_owner }}/my-exporter

jobs:
  build-and-push:
    name: Build and push image
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write

    outputs:
      image: ${{ steps.meta.outputs.tags }}

    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Log in to GHCR
        uses: docker/login-action@v3
        with:
          registry: ${{ env.REGISTRY }}
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Extract image metadata
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
          tags: |
            type=sha,format=short

      - name: Build and push
        uses: docker/build-push-action@v5
        with:
          context: .
          push: true
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}

  deploy:
    name: Deploy to Kubernetes
    runs-on: ubuntu-latest
    needs: build-and-push
    permissions:
      contents: read
      packages: read

    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up kubectl
        uses: azure/setup-kubectl@v4

      - name: Configure kubeconfig
        run: |
          mkdir -p $HOME/.kube
          echo "${{ secrets.KUBECONFIG_B64 }}" | base64 -d > $HOME/.kube/config
          chmod 600 $HOME/.kube/config

      - name: Substitute image tag
        run: |
          IMAGE="${{ needs.build-and-push.outputs.image }}"
          sed -i "s|IMAGE_PLACEHOLDER|${IMAGE}|g" k8s/deployment.yaml

      - name: Apply manifests
        run: |
          kubectl apply -f k8s/deployment.yaml
          kubectl apply -f k8s/service.yaml

      - name: Verify rollout
        run: |
          kubectl rollout status deployment/my-exporter -n monitoring --timeout=120s
```

### What each job does

| Job | Steps |
|-----|-------|
| `build-and-push` | Checks out code, logs in to GHCR, builds the image tagged with the short Git SHA, pushes it |
| `deploy` | Restores kubeconfig from the secret, substitutes the image tag into the manifest, runs `kubectl apply`, waits for the rollout to complete |

## Step 4 — Trigger the pipeline

Commit and push the workflow file:

```bash
git add .github/workflows/deploy.yml k8s/
git commit -m "Add CI/CD pipeline for metrics exporter"
git push origin main
```

Go to **GitHub → your repo → Actions** to watch the run. A green checkmark on
both jobs means the exporter is live on your cluster.

## Step 5 — Verify the deployment

Confirm the correct image is running:

```bash
kubectl get deployment my-exporter -n monitoring \
  -o jsonpath='{.spec.template.spec.containers[0].image}'
```

Check Prometheus has discovered the target:

```bash
kubectl port-forward svc/prometheus-operated 9090 -n monitoring
```

Open `http://localhost:9090/targets` and confirm `my-exporter` shows state **UP**.

## Troubleshooting

**`KUBECONFIG_B64` decodes to an empty file**

Verify the secret value has no trailing newline. Re-encode with:

```bash
cat ~/.kube/config | base64 | tr -d '\n'
```

**Image pull error on the cluster**

The Kubernetes node must be able to pull from `ghcr.io`. For private
repositories, create an `imagePullSecret` using a personal access token with
`read:packages` scope and reference it in the Deployment's
`spec.template.spec.imagePullSecrets`.

**`kubectl rollout status` times out**

Check the Pod events for the real error:

```bash
kubectl describe pod -n monitoring -l app=my-exporter
kubectl logs -n monitoring deployment/my-exporter
```

## What's next

- Add a `test` job before `build-and-push` that runs `go test ./...` so broken
  code never reaches the cluster.
- Pin image tags with a digest instead of a short SHA for reproducible
  deployments.
- Replace `kubectl apply` with a Helm chart or ArgoCD Application for
  declarative GitOps-style management.
