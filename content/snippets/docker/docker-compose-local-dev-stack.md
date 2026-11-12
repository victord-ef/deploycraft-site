---
title: "Docker Compose — Local Development Stack with Postgres and Redis"
date: 2026-11-12
description: "A complete Docker Compose setup for local development: application container with live reload, Postgres with a persistent volume, Redis, health checks, dependency ordering, and a production override pattern."
lang: "Docker"
tags: ["docker", "docker-compose", "postgres", "redis", "development", "local", "health-checks"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-11-12"
draft: false
---

## When to use this

When you need a reproducible local environment that mirrors the production dependency graph — same Postgres version, same Redis config, same network — without installing anything on the host beyond Docker. The override pattern keeps dev conveniences (live reload, debug ports) out of the production Compose file.

## Snippet

### Base file (`compose.yaml`)

```yaml
# compose.yaml — shared base, safe to commit
name: myapp

services:
  app:
    build:
      context: .
      target: runtime           # references the runtime stage in a multi-stage Dockerfile
    env_file: .env
    environment:
      DATABASE_URL: postgres://app:${POSTGRES_PASSWORD}@db:5432/appdb
      REDIS_URL: redis://cache:6379
    ports:
      - "3000:3000"
    depends_on:
      db:
        condition: service_healthy
      cache:
        condition: service_healthy
    restart: unless-stopped
    networks:
      - backend

  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: appdb
      POSTGRES_USER: app
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./db/init:/docker-entrypoint-initdb.d:ro  # runs *.sql files on first start
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U app -d appdb"]
      interval: 5s
      timeout: 5s
      retries: 10
      start_period: 10s
    networks:
      - backend

  cache:
    image: redis:7-alpine
    command: redis-server --maxmemory 128mb --maxmemory-policy allkeys-lru
    volumes:
      - redis_data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 5
    networks:
      - backend

volumes:
  postgres_data:
  redis_data:

networks:
  backend:
    driver: bridge
```

### Development override (`compose.override.yaml`)

```yaml
# compose.override.yaml — dev conveniences, NOT committed (add to .gitignore)
# Docker Compose merges this with compose.yaml automatically in development

services:
  app:
    build:
      target: builder           # use the builder stage for live reload tools
    command: npm run dev        # override CMD to start with hot reload
    volumes:
      - .:/app                  # mount source for live reload
      - /app/node_modules       # anonymous volume prevents host node_modules overwriting container's
    environment:
      NODE_ENV: development
      DEBUG: "app:*"

  db:
    ports:
      - "5432:5432"             # expose Postgres to host for local GUI tools

  cache:
    ports:
      - "6379:6379"             # expose Redis to host for local inspection
```

### Environment file (`.env`, add to `.gitignore`)

```bash
# .env — never committed
POSTGRES_PASSWORD=devpassword123
```

### `.gitignore` additions

```
.env
compose.override.yaml
```

### Usage

```bash
# Start the full stack (dev — merges compose.yaml + compose.override.yaml)
docker compose up -d

# Start without the override (simulates production Compose behaviour)
docker compose -f compose.yaml up -d

# Tail logs from all services
docker compose logs -f

# Open a psql shell against the running Postgres container
docker compose exec db psql -U app -d appdb

# Run a one-off command against the app container
docker compose run --rm app npm run migrate

# Destroy everything including volumes (full reset)
docker compose down -v
```

**Key decisions:**

| Decision | Why it matters |
|---|---|
| `condition: service_healthy` on `depends_on` | Waits for Postgres and Redis to pass their health checks before starting the app. Without this, the app starts while Postgres is still initialising and fails to connect — causing a crash-restart loop. |
| `compose.override.yaml` auto-merging | Docker Compose automatically merges `compose.override.yaml` on top of `compose.yaml` when both are present. Dev-only config (live reload, exposed ports) never touches the base file, which is what CI and production use. |
| Anonymous volume for `node_modules` | Mounting the host source (`- .:/app`) overwrites the container's `node_modules`. The anonymous volume (`- /app/node_modules`) pins the container's `node_modules` in place, preventing the host (which may have no `node_modules`) from shadowing it. |
| `/docker-entrypoint-initdb.d` for DB init | SQL or shell scripts in this directory run automatically on first container start (when the volume is empty). Useful for creating schemas, seed data, or extensions without a migration tool dependency. |
| `maxmemory-policy allkeys-lru` on Redis | Without an eviction policy, Redis fills to `maxmemory` and then rejects all writes. LRU eviction keeps the most recently used keys and silently drops the oldest — appropriate for a cache, not a persistent store. |
| Named volumes not bind mounts for data | Named volumes survive `docker compose down` and are managed by Docker. Bind mounts (`./data:/var/lib/postgresql/data`) expose database files to the host filesystem with Docker daemon permissions, which causes permission issues on Linux. |

## Verify it worked

```bash
# Confirm all services are healthy
docker compose ps

# Expected STATUS column: healthy for db and cache, running for app

# Confirm app can reach Postgres
docker compose exec app \
  node -e "const {Client}=require('pg'); const c=new Client(process.env.DATABASE_URL); c.connect().then(()=>{console.log('connected');c.end()})"

# Confirm app can reach Redis
docker compose exec app \
  node -e "const r=require('redis').createClient(process.env.REDIS_URL); r.connect().then(()=>{console.log('connected');r.quit()})"
```
