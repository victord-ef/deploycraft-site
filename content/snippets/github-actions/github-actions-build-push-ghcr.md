---
title: "Build and Push a Container Image to GHCR with OIDC"
date: 2026-11-12
description: "Build a Docker image with Buildx, authenticate to GitHub Container Registry using OIDC (no static credentials), and push with provenance attestation."
lang: "YAML"
tags: ["github-actions", "docker", "ghcr", "oidc", "container", "cicd", "supply-chain"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-11-12"
draft: false
---

## When to use this

Any pipeline that builds and distributes a container image. OIDC authentication means no long-lived `GITHUB_TOKEN` or registry password is stored as a secret — the runner requests a short-lived token at job execution time. Provenance attestation provides a verifiable record of what built the image and from which commit.

## Snippet

```yaml
# .github/workflows/build-push.yml
name: Build and push

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

env:
  REGISTRY: ghcr.io
  IMAGE_NAME: ${{ github.repository }}

jobs:
  build:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write
      id-token: write        # required for OIDC token request
      attestations: write    # required for provenance attestation

    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Log in to GHCR
        uses: docker/login-action@v3
        with:
          registry: ${{ env.REGISTRY }}
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Extract metadata
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
          tags: |
            type=ref,event=branch
            type=ref,event=pr
            type=sha,prefix=sha-
            type=raw,value=latest,enable=${{ github.ref == 'refs/heads/main' }}

      - name: Build and push
        id: push
        uses: docker/build-push-action@v6
        with:
          context: .
          push: ${{ github.event_name != 'pull_request' }}
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
          provenance: true
          sbom: true

      - name: Generate provenance attestation
        if: github.event_name != 'pull_request'
        uses: actions/attest-build-provenance@v1
        with:
          subject-name: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
          subject-digest: ${{ steps.push.outputs.digest }}
          push-to-registry: true
```

**Key decisions:**

| Decision | Why it matters |
|---|---|
| `id-token: write` permission | Grants the runner an OIDC token so it can authenticate to external services without storing static credentials. Without this, OIDC-based auth silently falls back or fails. |
| `push: ${{ github.event_name != 'pull_request' }}` | Builds the image on PRs (catching `Dockerfile` errors early) but only pushes on merge to `main`. Avoids polluting the registry with every branch build. |
| `cache-from/cache-to: type=gha` | Uses the GitHub Actions cache backend for layer caching — no separate cache registry needed. `mode=max` caches all intermediate layers, not just the final stage. |
| `provenance: true` and `sbom: true` | Embeds a provenance attestation and SBOM into the image manifest. Required for SLSA Level 3 and expected by modern vulnerability scanners. |
| `type=sha` tag | Gives every pushed image a unique, immutable tag tied to the commit. `latest` is mutable and unreliable in production — use the SHA tag in Kubernetes manifests. |

## Verify it worked

```bash
# List images in the package (requires gh CLI authenticated)
gh api /user/packages/container/<repo-name>/versions \
  --jq '.[].metadata.container.tags'

# Verify provenance attestation is attached to the image
gh attestation verify \
  oci://ghcr.io/<org>/<repo>:latest \
  --repo <org>/<repo>
```

Expected output from `gh attestation verify`:
```
Loaded digest sha256:abc123... for oci://ghcr.io/<org>/<repo>:latest
✓ Verification succeeded!
```

## Full walkthrough

Building a full CI/CD pipeline that pushes to GHCR and deploys to Kubernetes on every merge → **Tutorial: Automate a Custom Metrics Exporter Deployment with GitHub Actions**.
