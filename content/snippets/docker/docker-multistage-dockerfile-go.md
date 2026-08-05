---
title: "Docker — Multi-stage Dockerfile for a Go Application"
date: 2026-08-03
description: "Multi-stage Dockerfile that compiles a Go binary in a full SDK image and copies only the binary into a minimal distroless runtime — small, secure, production-ready."
lang: "Docker"
tags: ["docker", "dockerfile", "multi-stage", "go", "distroless", "security"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-08-03"
draft: false
---

## When to use this

When containerising a Go service for production. A single-stage build copies the entire Go toolchain, module cache, and source code into the final image — multi-stage keeps only the compiled binary, cutting image size by 95%+ and eliminating the compiler as an attack surface.

## Without it

```dockerfile
# Single-stage — ships the entire Go SDK to production
FROM golang:1.22

WORKDIR /app
COPY . .
RUN go build -o server .

CMD ["/app/server"]
```

The resulting image is ~900 MB, contains the Go compiler, build tools, and all source code, and runs as root. Any vulnerability in those tools is present in every deployed container.

## Snippet

```dockerfile
# ── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /build

# Download dependencies first — cached unless go.mod/go.sum change
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build a statically linked binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
    -trimpath \
    -ldflags="-s -w" \
    -o server \
    ./cmd/server

# ── Stage 2: Runtime ──────────────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot

# Copy only the compiled binary from the builder stage
COPY --from=builder /build/server /server

# Distroless nonroot image already runs as UID 65532 (nonroot)
# No USER instruction needed — it is enforced by the base image

EXPOSE 8080

ENTRYPOINT ["/server"]
```

### Build and run

```bash
# Build
docker build -t my-app:1.0.0 .

# Run
docker run --rm -p 8080:8080 my-app:1.0.0

# Inspect final image size
docker images my-app:1.0.0
```

**Key decisions:**

| Decision | Why it matters |
|---|---|
| `golang:1.22-alpine` as builder | Alpine keeps the build stage lean and avoids pulling a ~1 GB Debian SDK image just to compile. |
| `COPY go.mod go.sum` before source | Docker layer cache — dependency download is skipped on rebuilds unless the module files change. A full `COPY . .` first would invalidate this cache on every source change. |
| `CGO_ENABLED=0` | Produces a fully static binary with no libc dependency. Required to run in distroless, which has no C runtime. |
| `-trimpath` | Strips local build paths from the binary. Without this, the full filesystem path of your build machine is embedded in stack traces — an information leak. |
| `-ldflags="-s -w"` | Removes the symbol table (`-s`) and DWARF debug info (`-w`), reducing binary size by ~25% with no runtime impact. |
| `distroless/static-debian12:nonroot` | No shell, no package manager, no libc, no OS utilities — nothing for an attacker to use if the binary is compromised. The `:nonroot` tag enforces a non-root UID at the image level. |
| `ENTRYPOINT` not `CMD` | `ENTRYPOINT` makes the binary the process (PID 1) directly. `CMD` via a shell (`sh -c`) adds a shell layer, which distroless does not have and would fail. |

## Verify it worked

```bash
# Confirm the final image has no shell (distroless property)
docker run --rm my-app:1.0.0 sh
# Expected: exec /bin/sh: no such file or directory

# Confirm the process runs as non-root
docker run --rm my-app:1.0.0 id
# Expected: uid=65532(nonroot) gid=65532(nonroot)

# Confirm image size is small
docker images my-app:1.0.0 --format "{{.Size}}"
# Expected: < 20MB for a typical Go service

# Scan for vulnerabilities (distroless has near-zero CVE surface)
docker scout cves my-app:1.0.0
# or
trivy image my-app:1.0.0
```

Expected image layers (inspect confirms only the binary is present):

```bash
docker history my-app:1.0.0
# IMAGE         CREATED     CREATED BY                 SIZE
# <hash>        ...         ENTRYPOINT ["/server"]     0B
# <hash>        ...         COPY /build/server /server ~8MB
# <hash>        ...         # distroless base           ~2MB
```

> **Note:** If your application reads files at runtime (templates, config, TLS certs), copy them from the builder stage alongside the binary. Distroless has no mechanism to pull files in after the image is built.

## Full walkthrough

Hardening container images with Seccomp, AppArmor, and image signing in a CI pipeline → **Tutorial Pair 59: Container Image Signing** *(coming soon)*.
