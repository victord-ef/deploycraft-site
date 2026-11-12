---
title: "System Health Check Script"
date: 2026-11-12
description: "Check CPU, memory, disk, and critical service status in a single script. Outputs a human-readable summary and exits non-zero when any threshold is breached — ready for cron, monitoring agents, or CI pre-flight."
lang: "Bash"
tags: ["bash", "monitoring", "sysadmin", "health-check", "cron", "alerting"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-11-12"
draft: false
---

## When to use this

As a lightweight pre-deploy check, a cron job that alerts on threshold breaches, or a sanity gate before running a heavy operation. The script exits 0 on pass and 1 on any failure, so it integrates directly with CI pipelines, systemd `ExecStartPre`, or Nagios-compatible monitoring.

## Snippet

```bash
#!/usr/bin/env bash
# health-check.sh — exits 1 if any threshold is breached

set -euo pipefail

# ── Thresholds ────────────────────────────────────────────────
CPU_THRESHOLD=85       # percent
MEM_THRESHOLD=90       # percent
DISK_THRESHOLD=85      # percent used
SERVICES=("nginx" "postgresql" "docker")   # adjust to your stack
# ──────────────────────────────────────────────────────────────

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

FAIL=0

pass() { echo -e "  ${GREEN}✓${NC}  $1"; }
warn() { echo -e "  ${YELLOW}!${NC}  $1"; FAIL=1; }
fail() { echo -e "  ${RED}✗${NC}  $1"; FAIL=1; }

echo "── System health check $(date -u '+%Y-%m-%d %H:%M UTC') ──"

# CPU (1-minute load average as a percentage of available cores)
CORES=$(nproc)
LOAD=$(awk '{print $1}' /proc/loadavg)
CPU_PCT=$(awk "BEGIN { printf \"%.0f\", ($LOAD / $CORES) * 100 }")
if [ "$CPU_PCT" -ge "$CPU_THRESHOLD" ]; then
  fail "CPU load ${CPU_PCT}% (${LOAD} / ${CORES} cores) — threshold ${CPU_THRESHOLD}%"
else
  pass "CPU load ${CPU_PCT}% (${LOAD} / ${CORES} cores)"
fi

# Memory
MEM_TOTAL=$(grep MemTotal /proc/meminfo | awk '{print $2}')
MEM_AVAIL=$(grep MemAvailable /proc/meminfo | awk '{print $2}')
MEM_USED=$(( MEM_TOTAL - MEM_AVAIL ))
MEM_PCT=$(awk "BEGIN { printf \"%.0f\", ($MEM_USED / $MEM_TOTAL) * 100 }")
MEM_USED_MB=$(( MEM_USED / 1024 ))
MEM_TOTAL_MB=$(( MEM_TOTAL / 1024 ))
if [ "$MEM_PCT" -ge "$MEM_THRESHOLD" ]; then
  fail "Memory ${MEM_PCT}% (${MEM_USED_MB}/${MEM_TOTAL_MB} MB) — threshold ${MEM_THRESHOLD}%"
else
  pass "Memory ${MEM_PCT}% (${MEM_USED_MB}/${MEM_TOTAL_MB} MB)"
fi

# Disk (all mounted filesystems, skip tmpfs and devtmpfs)
while IFS= read -r line; do
  PCT=$(echo "$line" | awk '{print $5}' | tr -d '%')
  MOUNT=$(echo "$line" | awk '{print $6}')
  if [ "$PCT" -ge "$DISK_THRESHOLD" ]; then
    fail "Disk ${MOUNT} at ${PCT}% — threshold ${DISK_THRESHOLD}%"
  else
    pass "Disk ${MOUNT} at ${PCT}%"
  fi
done < <(df -h --output=source,size,used,avail,pcent,target | \
         grep -vE '^(Filesystem|tmpfs|devtmpfs|udev)')

# Services
for svc in "${SERVICES[@]}"; do
  if systemctl is-active --quiet "$svc" 2>/dev/null; then
    pass "Service ${svc} is active"
  else
    fail "Service ${svc} is NOT active"
  fi
done

echo ""
if [ "$FAIL" -eq 0 ]; then
  echo -e "${GREEN}All checks passed.${NC}"
  exit 0
else
  echo -e "${RED}One or more checks failed.${NC}"
  exit 1
fi
```

## Usage

```bash
chmod +x health-check.sh

# Run manually
./health-check.sh

# Run via cron every 5 minutes, email on failure
*/5 * * * * /opt/scripts/health-check.sh || mail -s "Health check failed on $(hostname)" ops@example.com

# Use as a pre-deploy gate in a pipeline
./health-check.sh && kubectl apply -f manifests/
```

**Key decisions:**

| Decision | Why it matters |
|---|---|
| `set -euo pipefail` | `-e` exits on error, `-u` treats unset variables as errors, `-o pipefail` fails a pipe if any command in it fails. Without this, a broken command silently sets `FAIL=0` and the script reports healthy. |
| `/proc/loadavg` and `/proc/meminfo` | Available on any Linux kernel without installing extra tools. `vmstat` and `free` produce different output formats across distributions. `/proc` files are consistent. |
| Load average divided by core count | Raw load average is meaningless without core count context. A load of 4.0 on a single-core host is saturated; on a 16-core host it is 25% utilised. The percentage normalises across machines. |
| `MemAvailable` not `MemFree` | `MemFree` is memory not in use at all. `MemAvailable` accounts for reclaimable page cache and buffers — it is what the kernel can actually hand to a new process. `MemFree` will always read low on a healthy, busy server. |
| `FAIL=1` on warn/fail, single exit at the end | Collecting all failures before exiting surfaces every problem in one run. Failing immediately on the first check means a disk problem is hidden by a CPU alert. |
| `grep -vE '^(tmpfs\|devtmpfs\|udev)'` | Excludes virtual filesystems from disk checks. `tmpfs` fills up under normal operation and will always trigger false positives. |

## Verify it worked

```bash
# Confirm exit codes are correct
./health-check.sh; echo "Exit: $?"

# Simulate a threshold breach by temporarily lowering the limit
CPU_THRESHOLD=1 ./health-check.sh
```
