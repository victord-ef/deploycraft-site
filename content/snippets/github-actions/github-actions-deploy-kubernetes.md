---
title: "Deploy to Kubernetes with Environment Protection"
date: 2026-11-12
description: "Roll out a new image tag to Kubernetes using kubectl, gate production deployments behind a GitHub environment with required reviewers, and verify rollout success."
lang: "YAML"
tags: ["github-actions", "kubernetes", "kubectl", "deploy", "environments", "cicd"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-11-12"
draft: false
---

## When to use this

When you need a push-to-deploy pipeline for Kubernetes with a human approval gate before production. GitHub environments provide the approval gate; `kubectl set image` and `rollout status` provide the deployment and health check.

## Prerequisites

- A `kubeconfig` for the target cluster stored as a repository secret (`KUBECONFIG_STAGING`, `KUBECONFIG_PROD`).
- GitHub environments named `staging` and `production` configured in repository settings with required reviewers on `production`.

## Snippet

```yaml
# .github/workflows/deploy.yml
name: Deploy

on:
  push:
    branches: [main]

env:
  REGISTRY: ghcr.io
  IMAGE_NAME: ${{ github.repository }}
  IMAGE_TAG: sha-${{ github.sha }}

jobs:
  deploy-staging:
    runs-on: ubuntu-latest
    environment: staging
    permissions:
      contents: read

    steps:
      - uses: actions/checkout@v4

      - name: Set up kubectl
        uses: azure/setup-kubectl@v4
        with:
          version: "v1.31.0"

      - name: Configure kubeconfig
        run: |
          mkdir -p $HOME/.kube
          echo "${{ secrets.KUBECONFIG_STAGING }}" | base64 -d > $HOME/.kube/config
          chmod 600 $HOME/.kube/config

      - name: Deploy to staging
        run: |
          kubectl set image deployment/my-app \
            app=${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:${{ env.IMAGE_TAG }} \
            -n staging

      - name: Wait for rollout
        run: |
          kubectl rollout status deployment/my-app \
            -n staging \
            --timeout=5m

      - name: Verify pods are healthy
        run: |
          kubectl get pods -n staging -l app=my-app
          kubectl rollout history deployment/my-app -n staging

  deploy-production:
    needs: deploy-staging
    runs-on: ubuntu-latest
    environment: production    # requires approval from configured reviewers
    permissions:
      contents: read

    steps:
      - uses: actions/checkout@v4

      - name: Set up kubectl
        uses: azure/setup-kubectl@v4
        with:
          version: "v1.31.0"

      - name: Configure kubeconfig
        run: |
          mkdir -p $HOME/.kube
          echo "${{ secrets.KUBECONFIG_PROD }}" | base64 -d > $HOME/.kube/config
          chmod 600 $HOME/.kube/config

      - name: Deploy to production
        run: |
          kubectl set image deployment/my-app \
            app=${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:${{ env.IMAGE_TAG }} \
            -n production

      - name: Wait for rollout
        run: |
          kubectl rollout status deployment/my-app \
            -n production \
            --timeout=10m

      - name: Verify pods are healthy
        run: |
          kubectl get pods -n production -l app=my-app

      - name: Record deployment
        if: success()
        run: |
          echo "Deployed ${{ env.IMAGE_TAG }} to production at $(date -u)" \
            >> $GITHUB_STEP_SUMMARY
```

### Automatic rollback on failure

```yaml
      - name: Wait for rollout
        id: rollout
        run: |
          kubectl rollout status deployment/my-app \
            -n production \
            --timeout=10m

      - name: Rollback on failure
        if: failure() && steps.rollout.outcome == 'failure'
        run: |
          kubectl rollout undo deployment/my-app -n production
          kubectl rollout status deployment/my-app -n production --timeout=5m
```

**Key decisions:**

| Decision | Why it matters |
|---|---|
| Base64-encoded kubeconfig secret | A kubeconfig is multi-line YAML with embedded certificates. Base64 encoding stores it cleanly as a single-line secret value and avoids newline escaping bugs. |
| `environment: production` with required reviewers | The job pauses and sends an approval request before any production command runs. The approval is logged in the deployment history with the approver's name. |
| `needs: deploy-staging` | Enforces staging-first. If staging rollout fails, the production job never starts and no approval request is sent. |
| `--timeout=10m` on `rollout status` | Blocks the job until the rollout completes or the deadline passes. Without a timeout, a stalled rollout (e.g. image pull backoff) hangs the runner indefinitely. |
| `chmod 600 $HOME/.kube/config` | kubectl refuses to use a kubeconfig that is world-readable. The `base64 -d` output inherits runner umask — explicitly restrict it. |
| `kubectl set image` over applying full manifests | Applies a surgical image tag update without touching other manifest fields. Combine with a GitOps tool (Flux, ArgoCD) if you need full manifest reconciliation. |

## Verify it worked

```bash
# View the deployment history from the pipeline run summary
gh run view <run-id>

# Check the rollout history on the cluster directly
kubectl rollout history deployment/my-app -n production

# Confirm the running image matches the expected SHA tag
kubectl get deployment my-app -n production \
  -o jsonpath='{.spec.template.spec.containers[0].image}'
```

## Full walkthrough

Triggering GitOps reconciliation from a CI pipeline instead of `kubectl set image` → **Tutorial Pair: GitOps & CI/CD** *(coming soon)*.
