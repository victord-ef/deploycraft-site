---
title: "Docker — Multi-stage Dockerfile for a Node.js Application"
date: 2026-11-12
description: "Multi-stage Dockerfile that installs dependencies in a full Node image, prunes dev dependencies, and copies only production files into a minimal runtime — small, secure, production-ready."
lang: "Docker"
tags: ["docker", "dockerfile", "multi-stage", "nodejs", "npm", "security"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-11-12"
draft: false
---

## When to use this

When containerising a Node.js service for production. A single-stage build ships `devDependencies`, build tools, test frameworks, and source TypeScript into every deployed container. Multi-stage prunes them out, cutting image size by 60–80% and removing unnecessary attack surface.

## Without it

```dockerfile
# Single-stage — ships devDependencies and source TypeScript to production
FROM node:22

WORKDIR /app
COPY package*.json ./
RUN npm install          # installs ALL deps including devDependencies
COPY . .
RUN npm run build

CMD ["node", "dist/server.js"]
```

The resulting image is ~1.2 GB, contains TypeScript, test runners, linters, and source maps with original file paths embedded.

## Snippet

```dockerfile
# ── Stage 1: Dependencies ─────────────────────────────────────────────────────
FROM node:22-alpine AS deps

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci --ignore-scripts

# ── Stage 2: Build ────────────────────────────────────────────────────────────
FROM node:22-alpine AS builder

WORKDIR /app

COPY --from=deps /app/node_modules ./node_modules
COPY . .

RUN npm run build

# Prune devDependencies — leaves only what production needs
RUN npm prune --omit=dev

# ── Stage 3: Runtime ──────────────────────────────────────────────────────────
FROM node:22-alpine AS runtime

ENV NODE_ENV=production
ENV PORT=3000

WORKDIR /app

# Create a non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Copy only the built output and production node_modules
COPY --from=builder --chown=appuser:appgroup /app/dist ./dist
COPY --from=builder --chown=appuser:appgroup /app/node_modules ./node_modules
COPY --from=builder --chown=appuser:appgroup /app/package.json ./package.json

USER appuser

EXPOSE 3000

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD node -e "require('http').get('http://localhost:3000/healthz', r => process.exit(r.statusCode === 200 ? 0 : 1))"

ENTRYPOINT ["node", "dist/server.js"]
```

### Build and run

```bash
# Build
docker build -t my-node-app:1.0.0 .

# Run
docker run --rm -p 3000:3000 my-node-app:1.0.0

# Check image size vs single-stage
docker images my-node-app:1.0.0
```

**Key decisions:**

| Decision | Why it matters |
|---|---|
| Separate `deps` stage | Isolates `npm ci` so the layer is cached until `package.json` or `package-lock.json` changes — rebuilds skip dependency installation entirely on source-only changes. |
| `npm ci` not `npm install` | `npm ci` installs exact versions from `package-lock.json`, fails if the lockfile is out of sync, and never modifies it. `npm install` can silently update versions during a build. |
| `--ignore-scripts` on install | Prevents `postinstall` scripts from running during the dependency stage. Malicious or broken lifecycle scripts cannot execute before the build runs. |
| `npm prune --omit=dev` | Removes `devDependencies` from `node_modules` after building. Only the pruned `node_modules` is copied into the runtime stage — TypeScript, Jest, ESLint, and similar tools are excluded. |
| `node:22-alpine` not `node:22` | Alpine-based Node images are ~50 MB vs ~300 MB for Debian. The tradeoff is musl libc — binaries with native addons that require glibc will fail. Test before adopting. |
| `addgroup -S / adduser -S` | `-S` creates a system user with no password, no home directory creation, and a UID in the system range. The container process runs with minimal OS privileges. |
| `HEALTHCHECK` with `node -e` | Avoids adding `curl` or `wget` to the image just for health checks. Node is already present — using it for the health check adds no new surface. |

## Verify it worked

```bash
# Confirm image size
docker images my-node-app:1.0.0 --format "{{.Size}}"
# Expected: 150–200 MB (vs 1.2 GB single-stage)

# Confirm process runs as non-root
docker run --rm my-node-app:1.0.0 id
# Expected: uid=100(appuser) ...

# Confirm devDependencies are absent
docker run --rm my-node-app:1.0.0 \
  node -e "require('typescript')"
# Expected: Cannot find module 'typescript'

# Scan for vulnerabilities
trivy image my-node-app:1.0.0
```

## Full walkthrough

Hardening container images with Seccomp, AppArmor, and image signing in a CI pipeline → **Tutorial Pair 59: Container Image Signing** *(coming soon)*.
