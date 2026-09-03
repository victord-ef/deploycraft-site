---
title: "GitHub Actions"
description: "GitHub Actions workflow syntax, triggers, secrets, caching, matrix builds, and the gh CLI for pipeline automation."
icon: "🔄"
weight: 5
count: 35
tags: ["github-actions", "cicd", "automation"]
---

## Workflow Triggers

```yaml
on:
  push:
    branches: [main, dev]
    paths: ['src/**', '**.go']
  pull_request:
    branches: [main]
  schedule:
    - cron: '0 6 * * 1'             # every Monday at 06:00 UTC
  workflow_dispatch:                 # manual trigger
    inputs:
      environment:
        type: choice
        options: [dev, staging, prod]
  workflow_call:                     # reusable workflow trigger
```

## Job Structure

```yaml
jobs:
  build:
    runs-on: ubuntu-latest
    environment: production
    timeout-minutes: 30
    permissions:
      contents: read
      packages: write
      id-token: write               # for OIDC

    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Run tests
        run: go test ./...
```

## Secrets & Variables

```yaml
# Access secrets
${{ secrets.MY_SECRET }}
${{ vars.MY_VAR }}
${{ github.token }}

# Set as env var
env:
  DATABASE_URL: ${{ secrets.DATABASE_URL }}
  APP_ENV: ${{ vars.ENVIRONMENT }}

# Mask a value at runtime
- run: echo "::add-mask::$MY_SECRET"
```

## Caching

```yaml
- uses: actions/cache@v4
  with:
    path: ~/.cache/go/pkg/mod
    key: ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}
    restore-keys: |
      ${{ runner.os }}-go-

# Cache Docker layers
- uses: docker/setup-buildx-action@v3
- uses: actions/cache@v4
  with:
    path: /tmp/.buildx-cache
    key: ${{ runner.os }}-buildx-${{ github.sha }}
```

## Matrix Builds

```yaml
strategy:
  fail-fast: false
  matrix:
    os: [ubuntu-latest, windows-latest]
    go: ['1.21', '1.22']
    exclude:
      - os: windows-latest
        go: '1.21'

runs-on: ${{ matrix.os }}
```

## Artifacts

```yaml
- uses: actions/upload-artifact@v4
  with:
    name: build-output
    path: dist/
    retention-days: 7

- uses: actions/download-artifact@v4
  with:
    name: build-output
    path: ./dist
```

## Conditionals

```yaml
if: github.ref == 'refs/heads/main'
if: github.event_name == 'pull_request'
if: contains(github.event.head_commit.message, '[skip ci]') == false
if: failure()
if: success() && github.ref == 'refs/heads/main'
if: always()
```

## Outputs Between Jobs

```yaml
jobs:
  setup:
    outputs:
      version: ${{ steps.version.outputs.value }}
    steps:
      - id: version
        run: echo "value=$(cat VERSION)" >> $GITHUB_OUTPUT

  build:
    needs: setup
    steps:
      - run: echo "Building ${{ needs.setup.outputs.version }}"
```

## gh CLI

```bash
gh workflow list
gh workflow run <workflow.yml>
gh workflow run <workflow.yml> --ref dev
gh run list --workflow=build.yml
gh run view <run-id>
gh run watch <run-id>
gh run download <run-id>
gh secret set MY_SECRET
gh secret list
gh variable set MY_VAR --body "value"
gh pr checks
```
