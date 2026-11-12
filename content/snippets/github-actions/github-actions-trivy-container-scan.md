---
title: "Container Image Security Scan with Trivy"
date: 2026-11-12
description: "Scan a container image for OS and library vulnerabilities using Trivy, fail the build on HIGH or CRITICAL findings, and surface results as GitHub Security tab alerts."
lang: "YAML"
tags: ["github-actions", "trivy", "security", "container", "vulnerability", "devsecops", "sarif"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-11-12"
draft: false
---

## When to use this

As a gate on every pull request and push to `main` that modifies a `Dockerfile` or base image. Trivy scans the built image for known CVEs in OS packages and language dependencies, fails the pipeline on HIGH/CRITICAL findings, and uploads results to the GitHub Security tab so findings appear as code-scanning alerts on the PR.

## Snippet

```yaml
# .github/workflows/scan.yml
name: Container security scan

on:
  push:
    branches: [main]
    paths:
      - 'Dockerfile'
      - '.github/workflows/scan.yml'
  pull_request:
    branches: [main]
    paths:
      - 'Dockerfile'

jobs:
  scan:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      security-events: write    # required to upload SARIF results

    steps:
      - uses: actions/checkout@v4

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Build image
        uses: docker/build-push-action@v6
        with:
          context: .
          push: false
          tags: scan-target:${{ github.sha }}
          load: true            # load into local Docker daemon for scanning

      - name: Run Trivy vulnerability scan
        uses: aquasecurity/trivy-action@master
        with:
          image-ref: scan-target:${{ github.sha }}
          format: sarif
          output: trivy-results.sarif
          severity: HIGH,CRITICAL
          exit-code: "1"        # fail the job if findings exist
          ignore-unfixed: true  # skip vulnerabilities with no fix available
          vuln-type: os,library

      - name: Upload SARIF results to GitHub Security tab
        if: always()           # upload even when the scan step fails
        uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: trivy-results.sarif
          category: trivy-container
```

### Scan a filesystem (Dockerfile and source) instead of a built image

```yaml
      - name: Run Trivy filesystem scan
        uses: aquasecurity/trivy-action@master
        with:
          scan-type: fs
          scan-ref: .
          format: sarif
          output: trivy-results.sarif
          severity: HIGH,CRITICAL
          exit-code: "1"
```

### Scan on a schedule to catch newly disclosed CVEs

```yaml
on:
  schedule:
    - cron: '0 7 * * 1'    # every Monday at 07:00 UTC
  push:
    branches: [main]
    paths: ['Dockerfile']
```

**Key decisions:**

| Decision | Why it matters |
|---|---|
| `exit-code: "1"` | Makes the job fail when findings are found. Without this, the scan runs but never blocks a merge — it becomes a reporting step, not a gate. |
| `ignore-unfixed: true` | Suppresses CVEs that have no patch available. These create noise without an actionable fix path and cause alert fatigue. Remove this flag for a stricter posture. |
| `format: sarif` | SARIF is the standard format GitHub's Security tab accepts. Plain table or JSON output cannot be uploaded as code-scanning results. |
| `if: always()` on the SARIF upload | The upload step must run even when the scan step fails (exit code 1). Without `always()`, a failing scan exits the job before uploading results, and the PR gets no inline annotations. |
| `load: true` on build | Pulls the built image into the local Docker daemon so Trivy can scan it without pushing to a registry. Keeps unscanned images out of GHCR. |
| `paths:` filter on the trigger | Avoids running a full image build and scan on every commit. Only triggers when the `Dockerfile` or the workflow itself changes. |

## Verify it worked

```bash
# View the uploaded SARIF results for a repository
gh api repos/<org>/<repo>/code-scanning/alerts \
  --jq '.[] | {rule: .rule.id, severity: .rule.severity, state: .state}'
```

Findings will also appear as inline annotations on the PR diff and under the **Security → Code scanning** tab in the repository.

## Full walkthrough

Integrating Trivy into a full DevSecOps pipeline with Kubernetes admission control → **Tutorial Pair: CI/CD Pipeline Security** *(coming soon)*.
