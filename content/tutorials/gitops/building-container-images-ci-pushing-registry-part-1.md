---
title: "Building Container Images in CI and Pushing to a Registry — Part 1"
date: 2026-09-02
description: "Build a production-grade CI pipeline that builds container images on every commit, tags them correctly, scans for vulnerabilities, and pushes to a container registry — using GitHub Actions with Docker Buildx and GHCR."
cluster: "GitOps"
series: "CI Pipeline Integration"
part: 1
difficulty: "intermediate"
duration: "45 min"
tags: ["ci-cd", "github-actions", "docker", "containers", "registry", "gitops", "buildx", "devsecops"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

By the end of this tutorial you will have a GitHub Actions CI pipeline that builds a multi-platform container image on every push, tags it with the commit SHA and semantic version, scans it for vulnerabilities with Trivy, and pushes it to GitHub Container Registry (GHCR). Part 2 extends this by connecting the pipeline to a GitOps repository so a new image automatically triggers a Flux or ArgoCD reconciliation.

## Prerequisites

- A GitHub repository with application source code
- A `Dockerfile` at the repository root
- Basic familiarity with GitHub Actions syntax

---

## Step 1 — Write a production Dockerfile

A good Dockerfile is the foundation of a fast, small, and secure image build. Use a multi-stage build to separate build tooling from the final runtime image:

```dockerfile
# Dockerfile — Go application example
FROM golang:1.22-alpine AS builder
WORKDIR /app

# Cache dependencies separately from source code
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /app/server .

# ── Final runtime image ──────────────────────────────────────────────
FROM gcr.io/distroless/static:nonroot
COPY --from=builder /app/server /server
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/server"]
```

Key practices applied:
- `distroless/static` — no shell, no package manager, minimal attack surface
- `nonroot` user — no root privileges at runtime
- `-ldflags="-s -w"` — strips debug symbols, reduces binary size
- Dependency layer cached before source copy — faster rebuilds

For Node.js:

```dockerfile
FROM node:20-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production

FROM node:20-alpine
RUN addgroup -S app && adduser -S app -G app
WORKDIR /app
COPY --from=builder /app/node_modules ./node_modules
COPY --chown=app:app . .
USER app
EXPOSE 3000
CMD ["node", "server.js"]
```

---

## Step 2 — Configure GHCR authentication

GitHub Container Registry (GHCR) is free for public repositories and included in GitHub Actions for private repositories. No external registry account needed.

Grant the Actions workflow permission to push to GHCR by adding this to your repository settings:

**Settings → Actions → General → Workflow permissions → Read and write permissions**

Or scope permissions directly in the workflow file (preferred):

```yaml
permissions:
  contents: read
  packages: write    # required to push to GHCR
```

GHCR image names follow the pattern: `ghcr.io/<owner>/<repo>:<tag>`

---

## Step 3 — Image tagging strategy

Tag images to support rollback, traceability, and GitOps automation:

| Tag | Source | Use |
|---|---|---|
| `sha-abc1234` | Git commit SHA (short) | Immutable — points to exact code version |
| `main` | Branch name | Mutable — latest build from main |
| `v1.2.3` | Git tag | Semver release |
| `pr-42` | Pull request number | Ephemeral — for review environments |
| `latest` | Convention | Avoid in production — ambiguous |

The `docker/metadata-action` generates tags automatically from GitHub event context:

```yaml
- name: Extract metadata
  id: meta
  uses: docker/metadata-action@v5
  with:
    images: ghcr.io/${{ github.repository }}
    tags: |
      # Branch builds: main → ghcr.io/org/app:main
      type=ref,event=branch
      # PR builds: pr-42 → ghcr.io/org/app:pr-42
      type=ref,event=pr
      # Semver tags: v1.2.3 → ghcr.io/org/app:1.2.3, :1.2, :1
      type=semver,pattern={{version}}
      type=semver,pattern={{major}}.{{minor}}
      # Commit SHA: ghcr.io/org/app:sha-abc1234
      type=sha,prefix=sha-,format=short
```

---

## Step 4 — The complete CI workflow

```yaml
# .github/workflows/ci.yml
name: CI — Build and Push

on:
  push:
    branches: [main, develop]
    tags: ["v*"]
  pull_request:
    branches: [main]

env:
  REGISTRY: ghcr.io
  IMAGE_NAME: ${{ github.repository }}

permissions:
  contents: read
  packages: write
  security-events: write    # for Trivy SARIF upload

jobs:
  build-and-push:
    runs-on: ubuntu-latest

    steps:
      # ── Checkout ──────────────────────────────────────────────────
      - name: Checkout
        uses: actions/checkout@v4

      # ── Set up Docker Buildx (multi-platform builds) ───────────────
      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3
        with:
          driver-opts: |
            network=host

      # ── Authenticate to GHCR ───────────────────────────────────────
      - name: Log in to GHCR
        if: github.event_name != 'pull_request'
        uses: docker/login-action@v3
        with:
          registry: ${{ env.REGISTRY }}
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      # ── Generate image tags and labels ────────────────────────────
      - name: Extract metadata
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
          tags: |
            type=ref,event=branch
            type=ref,event=pr
            type=semver,pattern={{version}}
            type=semver,pattern={{major}}.{{minor}}
            type=sha,prefix=sha-,format=short

      # ── Build (and push on non-PR events) ─────────────────────────
      - name: Build and push
        id: build
        uses: docker/build-push-action@v6
        with:
          context: .
          platforms: linux/amd64,linux/arm64
          push: ${{ github.event_name != 'pull_request' }}
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
          provenance: true      # SLSA provenance attestation
          sbom: true            # generate SBOM attestation

      # ── Vulnerability scan ────────────────────────────────────────
      - name: Scan image with Trivy
        uses: aquasecurity/trivy-action@master
        with:
          image-ref: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:sha-${{ github.sha }}
          format: sarif
          output: trivy-results.sarif
          severity: CRITICAL,HIGH
          exit-code: "1"    # fail the build on CRITICAL/HIGH findings
        env:
          TRIVY_USERNAME: ${{ github.actor }}
          TRIVY_PASSWORD: ${{ secrets.GITHUB_TOKEN }}

      # ── Upload Trivy results to GitHub Security tab ───────────────
      - name: Upload Trivy SARIF
        if: always()
        uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: trivy-results.sarif

      # ── Output the image digest for downstream jobs ───────────────
      - name: Output image digest
        run: echo "Image digest ${{ steps.build.outputs.digest }}"
```

---

## Step 5 — Layer caching for fast builds

The `cache-from: type=gha` and `cache-to: type=gha,mode=max` directives use GitHub Actions cache to store Docker layer cache between runs. This reduces build time from minutes to seconds for unchanged layers.

For high-frequency builds, use a registry-based cache instead (more durable, shared across branches):

```yaml
- name: Build and push
  uses: docker/build-push-action@v6
  with:
    cache-from: type=registry,ref=ghcr.io/${{ github.repository }}:cache
    cache-to: type=registry,ref=ghcr.io/${{ github.repository }}:cache,mode=max
```

Registry cache survives runner restarts and is shared across all branches — the most effective caching strategy for monorepos or large images.

---

## Step 6 — Multi-platform builds

The `platforms: linux/amd64,linux/arm64` directive builds for both x86-64 and ARM64 in a single step using QEMU emulation via Buildx. The resulting image manifest is a multi-arch manifest — Docker and Kubernetes pull the correct variant automatically based on the node architecture.

For production, prefer native builds per platform to avoid QEMU emulation overhead:

```yaml
jobs:
  build:
    strategy:
      matrix:
        platform: [linux/amd64, linux/arm64]
        include:
          - platform: linux/amd64
            runner: ubuntu-latest
          - platform: linux/arm64
            runner: ubuntu-24.04-arm    # GitHub-hosted ARM runner

    runs-on: ${{ matrix.runner }}

    steps:
      - uses: actions/checkout@v4
      - uses: docker/setup-buildx-action@v3
      - uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Build and push by digest
        id: build
        uses: docker/build-push-action@v6
        with:
          platforms: ${{ matrix.platform }}
          outputs: type=image,name=ghcr.io/${{ github.repository }},push-by-digest=true,name-canonical=true,push=true

      - name: Export digest
        run: |
          mkdir -p /tmp/digests
          digest="${{ steps.build.outputs.digest }}"
          touch "/tmp/digests/${digest#sha256:}"

      - name: Upload digest
        uses: actions/upload-artifact@v4
        with:
          name: digests-${{ matrix.platform == 'linux/amd64' && 'amd64' || 'arm64' }}
          path: /tmp/digests/*

  merge:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - name: Download digests
        uses: actions/download-artifact@v4
        with:
          pattern: digests-*
          path: /tmp/digests
          merge-multiple: true

      - name: Create multi-arch manifest
        uses: docker/build-push-action@v6
        with:
          platforms: linux/amd64,linux/arm64
          push: true
          tags: ghcr.io/${{ github.repository }}:${{ github.sha }}
```

---

## Step 7 — Protect the main branch

Require CI to pass before any merge to main:

**Settings → Branches → Branch protection rules → main:**
- Require status checks to pass: `build-and-push`
- Require branches to be up to date before merging
- Require signed commits (optional, but recommended)

This ensures no unscanned, unbuilt code reaches the branch that feeds your GitOps deployment.

---

## Step 8 — Verify the pipeline

After pushing a commit:

```bash
# Pull the image locally to verify it runs
docker pull ghcr.io/<your-org>/<your-repo>:sha-<short-sha>
docker run --rm -p 8080:8080 ghcr.io/<your-org>/<your-repo>:sha-<short-sha>

# Inspect the image labels (contain commit SHA, branch, build URL)
docker inspect ghcr.io/<your-org>/<your-repo>:sha-<short-sha> \
  | jq '.[0].Config.Labels'

# Check SBOM and provenance attestations
docker buildx imagetools inspect \
  ghcr.io/<your-org>/<your-repo>:sha-<short-sha> \
  --format '{{ json .Provenance }}'

# Verify multi-platform manifest
docker buildx imagetools inspect \
  ghcr.io/<your-org>/<your-repo>:sha-<short-sha>
# linux/amd64: sha256:...
# linux/arm64: sha256:...
```

---

## What you have built

- A multi-stage Dockerfile producing a minimal, non-root, distroless runtime image
- GHCR authentication scoped via `GITHUB_TOKEN` — no external secrets needed
- A tagging strategy covering commit SHA, branch, PR, and semver — with `docker/metadata-action`
- A complete GitHub Actions workflow: build → scan → push, with Trivy blocking on CRITICAL/HIGH CVEs
- GHA layer caching and registry-based cache for fast incremental builds
- Multi-platform `linux/amd64` + `linux/arm64` manifest using Buildx
- SLSA provenance and SBOM attestations attached to every pushed image
- Branch protection enforcing CI pass before merge to main

In [Part 2](/tutorials/gitops/triggering-gitops-reconciliation-image-update-automation-part-2/) you will connect this pipeline to your GitOps repository: use Flux Image Automation or ArgoCD Image Updater to detect the new image tag and automatically open a pull request — or directly commit — the updated image reference into your deployment manifests.
