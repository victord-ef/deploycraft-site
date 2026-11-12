---
title: "Dockerfile Security Hardening Patterns"
date: 2026-11-12
description: "Non-root user, read-only root filesystem, no-new-privileges, minimal base image selection, and layer hygiene — applied to a single Dockerfile with explanations for each control."
lang: "Docker"
tags: ["docker", "dockerfile", "security", "hardening", "devsecops", "non-root", "distroless"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-11-12"
draft: false
---

## When to use this

As a checklist when writing or reviewing any production Dockerfile. Each pattern below is an independent control — apply the ones that fit your runtime. Together they address the most common container escape and lateral movement vectors without requiring a runtime security tool.

## Snippet

```dockerfile
# Hardened Dockerfile — annotated reference

# ── 1. Pin the base image to an exact digest ──────────────────────────────────
# Tags are mutable. A tag re-push can silently change the base image under you.
# A digest is immutable — the same bytes, every build.
FROM ubuntu:24.04@sha256:a6d2b38300ce017add71440577d5b0a90460d0602a4f38e4e430f287e43ac59c AS base

# ── 2. Avoid running as root — create a dedicated service account ─────────────
RUN groupadd --gid 10001 appgroup \
 && useradd \
      --uid 10001 \
      --gid appgroup \
      --no-create-home \
      --shell /sbin/nologin \
      appuser

# ── 3. Install only what the application needs, then clean up in the same layer
RUN apt-get update -qq \
 && apt-get install -y --no-install-recommends \
      ca-certificates \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# ── 4. Copy application files with correct ownership before switching user ─────
COPY --chown=appuser:appgroup dist/server /app/server
COPY --chown=appuser:appgroup config/     /app/config/

# ── 5. Make the binary executable but not writable ───────────────────────────
RUN chmod 550 /app/server

# ── 6. Drop to non-root ───────────────────────────────────────────────────────
USER appuser

# ── 7. Declare a non-privileged port (> 1024) ─────────────────────────────────
# Ports ≤ 1024 require CAP_NET_BIND_SERVICE or root. Use 8080/3000 and let the
# orchestrator map it to 80/443 externally.
EXPOSE 8080

# ── 8. Health check without curl ──────────────────────────────────────────────
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
  CMD ["/app/server", "--health-check"]

# ── 9. Use ENTRYPOINT with exec form — no shell wrapper ───────────────────────
# exec form: PID 1 is the application. Signals (SIGTERM) go directly to it.
# shell form (CMD ["sh", "-c", "..."]) adds a shell as PID 1 — signals are lost.
ENTRYPOINT ["/app/server"]
```

### Runtime flags that complement Dockerfile hardening

Apply these in `docker run` or in the container spec in Kubernetes (`securityContext`):

```bash
docker run \
  --read-only \                          # read-only root filesystem
  --tmpfs /tmp:rw,noexec,nosuid,size=64m \ # writable scratch space with noexec
  --security-opt no-new-privileges \     # prevents setuid/setgid privilege escalation
  --security-opt seccomp=seccomp.json \  # restrict allowed syscalls
  --cap-drop ALL \                       # drop all Linux capabilities
  --cap-add NET_BIND_SERVICE \           # add back only what is needed
  --memory 256m \                        # prevent memory exhaustion
  --pids-limit 50 \                      # prevent fork bombs
  my-app:1.0.0
```

### Equivalent Kubernetes `securityContext`

```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 10001
  runAsGroup: 10001
  readOnlyRootFilesystem: true
  allowPrivilegeEscalation: false
  capabilities:
    drop: ["ALL"]
  seccompProfile:
    type: RuntimeDefault
```

### Minimal base image selection guide

| Base image | Use when | Shell | Package manager | Size |
|---|---|---|---|---|
| `distroless/static` | Static binary (Go, Rust) | No | No | ~2 MB |
| `distroless/base` | Binary with glibc | No | No | ~20 MB |
| `alpine` | Need a package manager | Yes (`ash`) | `apk` | ~7 MB |
| `ubuntu:minimal` | Need `apt`, broad compat | Yes | `apt` | ~30 MB |
| `scratch` | Fully static binary only | No | No | 0 MB |

**Key decisions:**

| Decision | Why it matters |
|---|---|
| Digest pinning | A `latest` or named tag can be re-pushed by anyone with write access to the registry. If your base image is compromised upstream, a digest pin prevents the change reaching your builds silently. |
| `--no-install-recommends` | APT installs recommended packages by default — often many MBs of unrelated tooling. `--no-install-recommends` installs the named package and its hard dependencies only. |
| `rm -rf /var/lib/apt/lists/*` in the same `RUN` | Each `RUN` creates a new layer. Cleaning the APT cache in a separate `RUN` layer does not remove it from the image — it is already committed in the previous layer. Same-layer cleanup is required. |
| `--chown` on `COPY` | Setting ownership at copy time is a single layer operation. `COPY` followed by `RUN chown` creates two layers and temporarily doubles the file size in the image. |
| `chmod 550` not `755` | The binary needs execute permission for owner and group. World-execute (`755`) allows any process in the container to execute the binary — unnecessary on a service binary. |
| `no-new-privileges` at runtime | Prevents the process from gaining additional privileges via `setuid`/`setgid` binaries after startup, even if such a binary is somehow present in the container. |

## Verify it worked

```bash
# Confirm process is not root
docker run --rm my-app:1.0.0 id
# Expected: uid=10001(appuser) gid=10001(appgroup)

# Confirm read-only root filesystem blocks writes
docker run --rm --read-only my-app:1.0.0 \
  touch /tmp/test 2>&1
# Expected: touch: /tmp/test: Read-only file system

# Confirm no shell is present (distroless) or shell is non-functional (nologin)
docker run --rm --entrypoint sh my-app:1.0.0 2>&1
# Expected: exec /bin/sh: no such file or directory (distroless)
#        or: This account is currently not available. (nologin)

# Scan for misconfigurations
trivy image --scanners misconfig my-app:1.0.0
docker scout recommendations my-app:1.0.0
```
