---
title: "Reusable Workflow — Caller and Callee Pattern"
date: 2026-11-12
description: "Define a reusable workflow with typed inputs and secrets inheritance, then call it from multiple pipelines without duplicating job definitions."
lang: "YAML"
tags: ["github-actions", "reusable-workflow", "workflow-call", "cicd", "dry"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-11-12"
draft: false
---

## When to use this

When the same job sequence (lint, test, build, scan) appears in multiple repositories or multiple workflows in the same repository. Reusable workflows let you define that sequence once and call it by reference — changes to the shared workflow propagate to all callers on the next run.

## Without it

Three repositories each maintain their own copy of the same 80-line test workflow. One repository gets a security fix. The other two don't. The fix ships to one service and misses two others.

## Snippet

### Reusable workflow (the callee)

```yaml
# .github/workflows/ci-shared.yml
# This file lives in a central repo (e.g. org/.github) or the same repo
name: Shared CI

on:
  workflow_call:
    inputs:
      go-version:
        description: "Go version to use"
        type: string
        required: false
        default: "1.23"
      run-lint:
        description: "Run golangci-lint"
        type: boolean
        required: false
        default: true
    secrets:
      SONAR_TOKEN:
        required: false

jobs:
  test:
    runs-on: ubuntu-latest
    permissions:
      contents: read

    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ inputs.go-version }}
          cache: true

      - name: Run tests
        run: go test -race -coverprofile=coverage.out ./...

      - name: Upload coverage
        uses: actions/upload-artifact@v4
        with:
          name: coverage
          path: coverage.out
          retention-days: 7

  lint:
    if: ${{ inputs.run-lint }}
    runs-on: ubuntu-latest
    permissions:
      contents: read

    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ inputs.go-version }}
          cache: true

      - name: Lint
        uses: golangci/golangci-lint-action@v6
        with:
          version: latest
```

### Caller workflow (application repository)

```yaml
# .github/workflows/ci.yml  — lives in the application repo
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  ci:
    uses: my-org/.github/.github/workflows/ci-shared.yml@main
    with:
      go-version: "1.23"
      run-lint: true
    secrets:
      SONAR_TOKEN: ${{ secrets.SONAR_TOKEN }}

  # Additional jobs specific to this repository run after the shared jobs
  deploy:
    needs: ci
    if: github.ref == 'refs/heads/main'
    runs-on: ubuntu-latest
    steps:
      - run: echo "Deploy step — only runs after shared CI passes"
```

### Calling a workflow in the same repository

```yaml
jobs:
  ci:
    # Reference the file by its path within the same repo
    uses: ./.github/workflows/ci-shared.yml
    with:
      go-version: "1.23"
```

**Key decisions:**

| Decision | Why it matters |
|---|---|
| `workflow_call` trigger | Marks the workflow as a callee. Without this, `uses:` references to the file are rejected. |
| Typed inputs (`type: string`, `type: boolean`) | Provides validation at call time — the runner rejects a mismatched type before any job starts. Prevents silent string coercion bugs. |
| `secrets: inherit` vs explicit secrets | `secrets: inherit` passes the caller's entire secret context. Explicit declaration (`secrets.SONAR_TOKEN`) is more auditable — the callee only receives what it names. Prefer explicit for shared workflows used across repositories. |
| Pin caller reference to `@main` or a tag | `@main` picks up the latest shared workflow automatically. Pin to a tag (e.g. `@v2`) for stability when the shared workflow may receive breaking changes. |
| `needs: ci` on the deploy job | Ensures the shared jobs must pass before repository-specific jobs run. The caller can gate on the entire reusable job group with a single `needs` entry. |

## Verify it worked

```bash
# List runs of the calling workflow
gh run list --workflow=ci.yml

# Inspect a specific run to confirm the shared jobs appeared
gh run view <run-id> --log
```

The run graph will show the shared workflow's jobs (`test`, `lint`) nested under the caller job name (`ci`).
