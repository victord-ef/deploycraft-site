---
title: "BuildKit Secrets at Build Time"
date: 2026-11-12
description: "Pass SSH keys, API tokens, and registry credentials to docker build without baking them into image layers, using BuildKit's --mount=type=secret and --mount=type=ssh."
lang: "Docker"
tags: ["docker", "buildkit", "secrets", "ssh", "npm", "pip", "security", "devsecops"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-11-12"
draft: false
---

## When to use this

When a build step needs a credential — an SSH key to clone a private repo, an npm auth token for a private registry, a pip index token for a private PyPI server — but that credential must not appear in any image layer, the build cache, or `docker history` output.

## Without it

```dockerfile
# Leaks the token into a committed image layer
ARG NPM_TOKEN
RUN echo "//registry.npmjs.org/:_authToken=${NPM_TOKEN}" > ~/.npmrc \
 && npm ci
# ~/.npmrc with the token is now baked into this layer forever
# even if deleted in a later RUN step
```

The token is recoverable from the image with `docker history --no-trunc` or by extracting the layer tarball.

## Snippet

### Secret file (npm auth token)

```dockerfile
# syntax=docker/dockerfile:1
FROM node:22-alpine AS deps

WORKDIR /app

COPY package.json package-lock.json ./

# --mount=type=secret makes the file available only during this RUN step
# It is never committed to any layer
RUN --mount=type=secret,id=npmrc,target=/root/.npmrc \
    npm ci --ignore-scripts
```

```bash
# Build — pass the secret from a file on the host
echo "//registry.npmjs.org/:_authToken=${NPM_TOKEN}" > .npmrc
docker build \
  --secret id=npmrc,src=.npmrc \
  -t my-app:1.0.0 .
rm .npmrc
```

### Secret from an environment variable (no intermediate file)

```bash
# Pass the secret directly from an env var — no file on disk
docker build \
  --secret id=npmrc,env=NPM_TOKEN \
  -t my-app:1.0.0 .
```

```dockerfile
# In the Dockerfile, write the .npmrc inline from the secret value
RUN --mount=type=secret,id=npmrc \
    NPM_TOKEN=$(cat /run/secrets/npmrc) \
    npm config set "//registry.npmjs.org/:_authToken" "${NPM_TOKEN}" \
 && npm ci --ignore-scripts \
 && npm config delete "//registry.npmjs.org/:_authToken"
```

### SSH agent forwarding (private Git repos)

```dockerfile
# syntax=docker/dockerfile:1
FROM golang:1.23-alpine AS builder

WORKDIR /build

# Configure git to use SSH instead of HTTPS for private modules
RUN git config --global url."git@github.com:".insteadOf "https://github.com/"

COPY go.mod go.sum ./

# Forward the host SSH agent — no key is copied into the image
RUN --mount=type=ssh \
    go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o server ./cmd/server
```

```bash
# Start the SSH agent and add your key
eval $(ssh-agent)
ssh-add ~/.ssh/id_ed25519

# Build with SSH agent forwarding
docker build \
  --ssh default \
  -t my-go-app:1.0.0 .
```

### Private pip index (Python)

```dockerfile
# syntax=docker/dockerfile:1
FROM python:3.12-slim AS builder

WORKDIR /app

COPY requirements.txt .

RUN --mount=type=secret,id=pip_conf,target=/root/.pip/pip.conf \
    pip install --no-cache-dir -r requirements.txt
```

```bash
# pip.conf points at the private index
cat > pip.conf <<EOF
[global]
index-url = https://__token__:${PYPI_TOKEN}@private.pypi.example.com/simple/
EOF

docker build --secret id=pip_conf,src=pip.conf -t my-python-app:1.0.0 .
rm pip.conf
```

### GitHub Actions integration

```yaml
- name: Build image
  uses: docker/build-push-action@v6
  with:
    context: .
    secrets: |
      npmrc=//registry.npmjs.org/:_authToken=${{ secrets.NPM_TOKEN }}
    tags: my-app:${{ github.sha }}
```

**Key decisions:**

| Decision | Why it matters |
|---|---|
| `# syntax=docker/dockerfile:1` directive | Opts the build into BuildKit syntax. Without it, `--mount=type=secret` is a parse error on older Docker versions or when BuildKit is not the default builder. |
| Secret at `/run/secrets/<id>` | BuildKit mounts secrets as files at `/run/secrets/<id>` by default. The `target=` override changes the mount path — useful when a tool expects a credential at a fixed location (e.g. `~/.npmrc`). |
| `--mount=type=ssh` over copying keys | Copying an SSH private key (`COPY id_ed25519 /root/.ssh/`) bakes it into a layer permanently. SSH agent forwarding delegates signing to the host agent — the key never enters the container. |
| `eval $(ssh-agent) && ssh-add` before build | BuildKit connects to the host SSH agent socket. The agent must be running and have the key loaded before `docker build` is invoked, or the mount is empty and `go mod download` fails to authenticate. |
| `--no-cache-dir` on pip | Prevents pip from writing the downloaded wheel cache inside the layer. Without it, the cache is committed to the layer, inflating the image even though it is never used again. |
| `secrets:` in `build-push-action` | The action maps GitHub Actions secrets to BuildKit `--secret` flags. The secret value never appears in runner logs, build context, or image layers. |

## Verify it worked

```bash
# Confirm the secret is NOT in any image layer
docker history --no-trunc my-app:1.0.0 | grep -i "token\|npmrc\|password"
# Expected: no output

# Confirm the secret file does not exist in the running container
docker run --rm my-app:1.0.0 cat /root/.npmrc 2>&1
# Expected: cat: /root/.npmrc: No such file or directory

# Scan for secrets accidentally embedded in the image
trivy image --scanners secret my-app:1.0.0
```
