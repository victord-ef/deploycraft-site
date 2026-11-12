---
title: "Idempotent Bootstrap / Dependency Installer"
date: 2026-11-12
description: "Install system dependencies, create users, write config files, and enable services in a script that is safe to run multiple times without side effects — the foundation for any provisioning or CI setup script."
lang: "Bash"
tags: ["bash", "provisioning", "idempotent", "bootstrap", "ci", "sysadmin", "automation"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-11-12"
draft: false
---

## When to use this

Any time a script runs on a machine that may already be partially configured — a CI runner, a freshly cloned dev environment, an Ansible alternative for simple setups, or a Dockerfile `RUN` layer that gets rebuilt. Idempotent scripts check before acting, so re-running them never breaks a working system or produces spurious errors.

## Without it

```bash
# Non-idempotent — fails on second run
useradd appuser          # error: user already exists
mkdir /opt/myapp         # error: file exists
systemctl enable nginx   # harmless, but noisy output
```

## Snippet

```bash
#!/usr/bin/env bash
# bootstrap.sh — idempotent system setup

set -euo pipefail

LOG_PREFIX="[bootstrap]"

log()  { echo "${LOG_PREFIX} $*"; }
ok()   { echo "${LOG_PREFIX} ✓ $*"; }
skip() { echo "${LOG_PREFIX} — $*  (already done)"; }

require_root() {
  if [[ $EUID -ne 0 ]]; then
    echo "${LOG_PREFIX} This script must be run as root." >&2
    exit 1
  fi
}

# ── 1. Package installation ───────────────────────────────────
install_packages() {
  local packages=("curl" "git" "jq" "unzip" "ca-certificates")

  log "Checking packages..."
  local to_install=()
  for pkg in "${packages[@]}"; do
    if dpkg-query -W -f='${Status}' "$pkg" 2>/dev/null | grep -q "install ok installed"; then
      skip "Package ${pkg}"
    else
      to_install+=("$pkg")
    fi
  done

  if [[ ${#to_install[@]} -gt 0 ]]; then
    log "Installing: ${to_install[*]}"
    apt-get update -qq
    apt-get install -y -qq "${to_install[@]}"
    ok "Installed: ${to_install[*]}"
  fi
}

# ── 2. User creation ──────────────────────────────────────────
create_user() {
  local username="appuser"
  local home_dir="/opt/myapp"

  if id "$username" &>/dev/null; then
    skip "User ${username}"
  else
    useradd \
      --system \
      --shell /bin/false \
      --home-dir "$home_dir" \
      --no-create-home \
      "$username"
    ok "Created user ${username}"
  fi
}

# ── 3. Directory setup ────────────────────────────────────────
create_dirs() {
  local dirs=(
    "/opt/myapp"
    "/opt/myapp/data"
    "/opt/myapp/logs"
    "/etc/myapp"
  )

  for dir in "${dirs[@]}"; do
    if [[ -d "$dir" ]]; then
      skip "Directory ${dir}"
    else
      mkdir -p "$dir"
      ok "Created directory ${dir}"
    fi
  done

  # Set ownership idempotently — always safe to re-apply
  chown -R appuser:appuser /opt/myapp
}

# ── 4. Config file from template ─────────────────────────────
write_config() {
  local config_file="/etc/myapp/config.env"

  if [[ -f "$config_file" ]]; then
    skip "Config ${config_file}"
    return
  fi

  cat > "$config_file" <<'EOF'
APP_ENV=production
LOG_LEVEL=info
DATA_DIR=/opt/myapp/data
EOF
  chmod 640 "$config_file"
  chown root:appuser "$config_file"
  ok "Wrote ${config_file}"
}

# ── 5. Binary installation (version-pinned) ──────────────────
install_binary() {
  local name="yq"
  local version="4.44.3"
  local binary="/usr/local/bin/${name}"
  local url="https://github.com/mikefarah/yq/releases/download/v${version}/yq_linux_amd64"

  if [[ -x "$binary" ]]; then
    local installed_version
    installed_version=$("$binary" --version 2>&1 | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1)
    if [[ "$installed_version" == "$version" ]]; then
      skip "${name} ${version}"
      return
    fi
    log "${name} version mismatch (installed: ${installed_version}, want: ${version}). Upgrading."
  fi

  curl -sSL "$url" -o "$binary"
  chmod +x "$binary"
  ok "Installed ${name} ${version}"
}

# ── 6. Service enablement ─────────────────────────────────────
enable_service() {
  local service="myapp"

  if systemctl is-enabled --quiet "$service" 2>/dev/null; then
    skip "Service ${service} already enabled"
  else
    systemctl enable "$service"
    ok "Enabled service ${service}"
  fi

  if systemctl is-active --quiet "$service" 2>/dev/null; then
    skip "Service ${service} already running"
  else
    systemctl start "$service"
    ok "Started service ${service}"
  fi
}

# ── Main ──────────────────────────────────────────────────────
require_root
install_packages
create_user
create_dirs
write_config
install_binary
# enable_service    # uncomment when the service unit file is in place

log "Bootstrap complete."
```

**Key decisions:**

| Decision | Why it matters |
|---|---|
| Check before act, not catch-on-error | Wrapping non-idempotent commands in `2>/dev/null \|\| true` silences real errors too. Explicit checks (`id`, `-d`, `-f`, `dpkg-query`) distinguish "already done" from "actually failed". |
| `dpkg-query` over `which` or `command -v` for packages | `which` and `command -v` find executables on `$PATH`, not installed packages. A package can be installed with no binary in `$PATH`, or a binary can exist without its package (manual install). `dpkg-query` checks the package database directly. |
| Version comparison on binaries | Installing a binary without checking the version means upgrades never apply. The version check catches mismatches and re-installs only when necessary. |
| `--system --shell /bin/false` on `useradd` | Creates a service account that cannot log in interactively. The `--system` flag assigns a UID in the system range (< 1000) and flags it as non-human in `lastlog` and similar tools. |
| `chmod 640 / chown root:appuser` on config | Config contains env vars. Owner root prevents the app user from modifying it; group `appuser` grants read access. World-readable config risks leaking values to other processes. |
| `set -euo pipefail` | Ensures any unexpected failure exits the script immediately rather than silently continuing through a broken state. |

## Verify it worked

```bash
# First run — installs everything
sudo ./bootstrap.sh

# Second run — should print only "already done" lines, no errors
sudo ./bootstrap.sh

# Confirm script is truly idempotent: exit code must be 0 on both runs
sudo ./bootstrap.sh; echo "Exit: $?"
```
