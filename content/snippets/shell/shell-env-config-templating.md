---
title: "Config File Templating from Environment Variables"
date: 2026-11-12
description: "Inject secrets and runtime values into config files at deploy time using envsubst, sed, and a pure-Bash fallback — without baking credentials into images or repositories."
lang: "Bash"
tags: ["bash", "envsubst", "secrets", "config", "templating", "deploy", "containers", "devsecops"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-11-12"
draft: false
---

## When to use this

When a service expects a config file (not environment variables) but secrets must not be stored in the repository or the container image. A template file with `${PLACEHOLDER}` markers lives in the repo; the actual values are injected from environment variables at container startup or deploy time.

## Without it

```bash
# Hard-coded credentials in a committed config file
db_password = "SuperSecret123"    # in git history forever
api_key = "sk-prod-abc123"
```

## Snippet

### Method 1 — `envsubst` (recommended, requires gettext)

```bash
#!/usr/bin/env bash
# render-config.sh — render a config template using envsubst

set -euo pipefail

TEMPLATE_FILE="${1:-/etc/myapp/config.template}"
OUTPUT_FILE="${2:-/etc/myapp/config.conf}"

# Fail fast if required variables are not set
: "${DB_HOST:?DB_HOST is required}"
: "${DB_PORT:?DB_PORT is required}"
: "${DB_PASSWORD:?DB_PASSWORD is required}"
: "${API_KEY:?API_KEY is required}"

# Only substitute the variables we expect — prevents accidental substitution
# of shell variables that happen to match $SOMETHING in the template
envsubst '${DB_HOST} ${DB_PORT} ${DB_PASSWORD} ${API_KEY}' \
  < "$TEMPLATE_FILE" \
  > "$OUTPUT_FILE"

chmod 640 "$OUTPUT_FILE"
echo "Config rendered: ${OUTPUT_FILE}"
```

### Template file (`config.template`, safe to commit)

```ini
# /etc/myapp/config.template
[database]
host     = ${DB_HOST}
port     = ${DB_PORT}
password = ${DB_PASSWORD}

[api]
key = ${API_KEY}
```

### Method 2 — `sed` (no external dependencies)

```bash
#!/usr/bin/env bash
# Works on any system without gettext installed

set -euo pipefail

TEMPLATE="/etc/myapp/config.template"
OUTPUT="/etc/myapp/config.conf"

: "${DB_HOST:?}" "${DB_PORT:?}" "${DB_PASSWORD:?}" "${API_KEY:?}"

sed \
  -e "s|\${DB_HOST}|${DB_HOST}|g" \
  -e "s|\${DB_PORT}|${DB_PORT}|g" \
  -e "s|\${DB_PASSWORD}|${DB_PASSWORD}|g" \
  -e "s|\${API_KEY}|${API_KEY}|g" \
  "$TEMPLATE" > "$OUTPUT"

chmod 640 "$OUTPUT"
```

### Method 3 — Pure Bash (no external tools at all)

```bash
#!/usr/bin/env bash
# Useful in minimal containers where even sed may not be present

set -euo pipefail

: "${DB_HOST:?}" "${DB_PORT:?}" "${DB_PASSWORD:?}" "${API_KEY:?}"

TEMPLATE=$(cat /etc/myapp/config.template)

RESULT="${TEMPLATE//\$\{DB_HOST\}/$DB_HOST}"
RESULT="${RESULT//\$\{DB_PORT\}/$DB_PORT}"
RESULT="${RESULT//\$\{DB_PASSWORD\}/$DB_PASSWORD}"
RESULT="${RESULT//\$\{API_KEY\}/$API_KEY}"

printf '%s' "$RESULT" > /etc/myapp/config.conf
chmod 640 /etc/myapp/config.conf
```

### Docker entrypoint pattern

```bash
#!/usr/bin/env bash
# docker-entrypoint.sh — render config then hand off to the main process

set -euo pipefail

/usr/local/bin/render-config.sh \
  /etc/myapp/config.template \
  /etc/myapp/config.conf

# exec replaces the shell process — PID 1 becomes the application
exec "$@"
```

```dockerfile
# Dockerfile
COPY config.template /etc/myapp/config.template
COPY render-config.sh /usr/local/bin/render-config.sh
COPY docker-entrypoint.sh /docker-entrypoint.sh

RUN chmod +x /usr/local/bin/render-config.sh /docker-entrypoint.sh

ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["myapp", "--config", "/etc/myapp/config.conf"]
```

**Key decisions:**

| Decision | Why it matters |
|---|---|
| `: "${VAR:?message}"` for required variable checks | Exits immediately with a clear error if a variable is unset or empty. Without this, the template renders with blank values and the service starts with broken config — which may not be detected until a request hits that code path. |
| Explicit variable list in `envsubst '${A} ${B}'` | Without the explicit list, `envsubst` substitutes every `$VARIABLE` in the template, including shell internals like `$PATH` or `$HOME` if they appear as literal strings. The explicit list is a safety boundary. |
| `\|` as `sed` delimiter | Config values can contain `/` (paths, URLs). Using `|` as the delimiter avoids escaping every `/` in the substitution value. |
| `chmod 640` on the output file | The rendered file contains plaintext secrets. World-readable permissions mean any process running as any user on the system can read the credentials. |
| `exec "$@"` in the entrypoint | Replaces the shell with the application process, making the application PID 1. Without `exec`, the shell is PID 1 and signals (SIGTERM from `docker stop`) go to the shell, not the application — causing unclean shutdowns and lost in-flight requests. |
| Template file committed, rendered file gitignored | The template documents the config structure without exposing values. Add the rendered output path to `.gitignore` to prevent accidental credential commits. |

## Verify it worked

```bash
# Export test values and render
export DB_HOST=db.internal DB_PORT=5432 DB_PASSWORD=test123 API_KEY=key-abc

./render-config.sh /etc/myapp/config.template /tmp/test.conf

cat /tmp/test.conf
# Expected: placeholders replaced with the exported values

# Confirm no placeholders remain
grep '\${' /tmp/test.conf && echo "UNREPLACED PLACEHOLDERS FOUND" || echo "All placeholders replaced"
```
