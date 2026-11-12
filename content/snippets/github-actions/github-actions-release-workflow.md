---
title: "Tag-triggered Release with GitHub Release and Changelog"
date: 2026-11-12
description: "Trigger a release pipeline from a semver tag, build binaries for multiple platforms, generate a changelog from conventional commits, and publish a GitHub Release with attached assets."
lang: "YAML"
tags: ["github-actions", "release", "semver", "changelog", "goreleaser", "cicd"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-11-12"
draft: false
---

## When to use this

When you version software with semver tags and want the full release process — cross-platform builds, checksums, changelog, and a GitHub Release — to run automatically on `git push --tags`. No manual draft, no copy-pasting changelogs.

## Snippet

### Release workflow (using GoReleaser for Go binaries)

```yaml
# .github/workflows/release.yml
name: Release

on:
  push:
    tags:
      - 'v[0-9]+.[0-9]+.[0-9]+'    # match v1.2.3, not v1.2.3-rc1

jobs:
  release:
    runs-on: ubuntu-latest
    permissions:
      contents: write      # create GitHub Release and upload assets
      packages: write      # push container image if goreleaser builds one
      id-token: write      # OIDC for signing

    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0    # full history required for changelog generation

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true

      - name: Install Cosign
        uses: sigstore/cosign-installer@v3

      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v6
        with:
          distribution: goreleaser
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

### `.goreleaser.yml` (project root)

```yaml
# .goreleaser.yml
version: 2

before:
  hooks:
    - go mod tidy

builds:
  - env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w
      - -X main.version={{ .Version }}
      - -X main.commit={{ .Commit }}
      - -X main.date={{ .Date }}

archives:
  - format: tar.gz
    format_overrides:
      - goos: windows
        format: zip
    name_template: "{{ .ProjectName }}_{{ .Os }}_{{ .Arch }}"

checksum:
  name_template: "checksums.txt"

changelog:
  sort: asc
  use: github
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^chore:"
  groups:
    - title: "New features"
      regexp: "^feat"
      order: 0
    - title: "Bug fixes"
      regexp: "^fix"
      order: 1
    - title: "Security"
      regexp: "^sec"
      order: 2
    - title: Other
      order: 999

signs:
  - artifacts: checksum
    cmd: cosign
    args:
      - sign-blob
      - --output-certificate=${certificate}
      - --output-signature=${signature}
      - ${artifact}
      - --yes
    output: true
```

### Lightweight release workflow (without GoReleaser)

```yaml
# For projects that don't need cross-platform binaries
name: Release

on:
  push:
    tags:
      - 'v[0-9]+.[0-9]+.[0-9]+'

jobs:
  release:
    runs-on: ubuntu-latest
    permissions:
      contents: write

    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Generate changelog
        id: changelog
        uses: mikepenz/release-changelog-builder-action@v4
        with:
          configuration: ".github/changelog-config.json"
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

      - name: Create GitHub Release
        uses: softprops/action-gh-release@v2
        with:
          body: ${{ steps.changelog.outputs.changelog }}
          draft: false
          prerelease: ${{ contains(github.ref_name, '-rc') }}
          generate_release_notes: false
```

**Key decisions:**

| Decision | Why it matters |
|---|---|
| `fetch-depth: 0` on checkout | Changelog generators and GoReleaser read the full git history to build the commit list since the previous tag. Shallow clones (the default) only fetch recent commits and produce an empty or truncated changelog. |
| Tag pattern `v[0-9]+.[0-9]+.[0-9]+` | Restricts releases to production semver tags. Pre-release tags (`v1.0.0-rc1`) and date-based tags don't match and don't trigger a release. Adjust the pattern if pre-release automation is needed. |
| `CGO_ENABLED=0` | Produces a fully static binary with no glibc dependency. The binary runs in `scratch` or `distroless` containers and on any Linux distribution without library mismatch errors. |
| Cosign signing on checksums | Signs the checksum file rather than each binary individually — one signature covers all artifacts. Users verify the checksum file, then compare binary hashes against it. |
| `prerelease: ${{ contains(github.ref_name, '-rc') }}` | Automatically marks releases containing `-rc` in the tag as pre-releases in GitHub without a separate workflow branch. |
| `--clean` flag on GoReleaser | Removes the `dist/` directory before building, preventing stale artifacts from a previous failed run from being included in the release. |

## Verify it worked

```bash
# List releases and their assets
gh release list

# View the release notes for the latest tag
gh release view $(git describe --tags --abbrev=0)

# Download and verify the checksum
gh release download --pattern "*.tar.gz" --pattern "checksums.txt"
sha256sum --check checksums.txt
```
