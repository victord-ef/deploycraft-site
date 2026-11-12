---
title: "Automated Backup Script with Retention Rotation"
date: 2026-11-12
description: "Create date-stamped tar.gz backups of a directory, rotate old backups beyond a configurable retention window, and log every run — ready to drop into a cron job."
lang: "Bash"
tags: ["bash", "backup", "cron", "sysadmin", "retention", "tar"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-11-12"
draft: false
---

## When to use this

For any directory that needs periodic snapshots without a full backup agent — application data, configuration directories, database dumps already written to disk, or static files. The script is self-contained, appends to a log file, and removes backups older than a retention window so the backup destination does not fill indefinitely.

## Snippet

```bash
#!/usr/bin/env bash
# backup.sh — create a date-stamped archive and rotate old backups

set -euo pipefail

# ── Configuration ─────────────────────────────────────────────
SOURCE_DIR="/var/lib/myapp/data"
BACKUP_DIR="/mnt/backups/myapp"
RETENTION_DAYS=30
LOG_FILE="/var/log/backup-myapp.log"
# ──────────────────────────────────────────────────────────────

TIMESTAMP=$(date -u '+%Y%m%dT%H%M%SZ')
ARCHIVE="${BACKUP_DIR}/${TIMESTAMP}.tar.gz"
HOSTNAME=$(hostname -s)

log() {
  echo "[$(date -u '+%Y-%m-%dT%H:%M:%SZ')] [$HOSTNAME] $*" | tee -a "$LOG_FILE"
}

# Ensure backup directory exists
mkdir -p "$BACKUP_DIR"

log "Starting backup of ${SOURCE_DIR}"

# Create archive
tar \
  --create \
  --gzip \
  --file="$ARCHIVE" \
  --directory="$(dirname "$SOURCE_DIR")" \
  "$(basename "$SOURCE_DIR")"

SIZE=$(du -sh "$ARCHIVE" | cut -f1)
log "Archive created: $(basename "$ARCHIVE") (${SIZE})"

# Rotate: remove archives older than RETENTION_DAYS
PRUNED=0
while IFS= read -r old_archive; do
  rm -f "$old_archive"
  log "Pruned: $(basename "$old_archive")"
  (( PRUNED++ )) || true
done < <(find "$BACKUP_DIR" -maxdepth 1 -name "*.tar.gz" \
           -mtime +"$RETENTION_DAYS" -type f)

log "Rotation complete. Pruned ${PRUNED} archive(s)."

# Summary
TOTAL=$(find "$BACKUP_DIR" -maxdepth 1 -name "*.tar.gz" -type f | wc -l)
OLDEST=$(find "$BACKUP_DIR" -maxdepth 1 -name "*.tar.gz" -type f \
           -printf '%T+ %p\n' 2>/dev/null | sort | head -1 | awk '{print $2}' \
           | xargs -I{} basename {} 2>/dev/null || echo "none")
log "Backup store: ${TOTAL} archive(s). Oldest: ${OLDEST}"
```

## Usage

```bash
chmod +x backup.sh

# Run manually
./backup.sh

# Schedule via cron — run daily at 02:00
0 2 * * * /opt/scripts/backup.sh

# Alert on failure — cron mails MAILTO on non-zero exit
MAILTO=ops@example.com
0 2 * * * /opt/scripts/backup.sh
```

### Verify the latest backup is restorable

```bash
# List contents without extracting
tar -tzf /mnt/backups/myapp/20261112T020000Z.tar.gz | head -20

# Test-extract to a temp directory
mkdir /tmp/restore-test
tar -xzf /mnt/backups/myapp/20261112T020000Z.tar.gz -C /tmp/restore-test
ls /tmp/restore-test/
rm -rf /tmp/restore-test
```

### Restore from a specific backup

```bash
ARCHIVE="/mnt/backups/myapp/20261112T020000Z.tar.gz"
RESTORE_DIR="/var/lib/myapp/data"

# Stop the service before restoring
systemctl stop myapp

# Extract to the parent of the target directory
tar -xzf "$ARCHIVE" -C "$(dirname "$RESTORE_DIR")"

systemctl start myapp
```

**Key decisions:**

| Decision | Why it matters |
|---|---|
| ISO 8601 UTC timestamp in filename | Date-stamped names sort lexicographically — `ls` and `find` ordering matches chronological ordering with no extra tooling. UTC avoids DST discontinuities in filenames. |
| `--directory` with `basename` on tar | Archives the directory by its leaf name, not by its full absolute path. Restoring with `tar -xzf` to any location recreates `data/`, not `/var/lib/myapp/data/`. |
| `find -mtime +N` for rotation | `mtime` is updated when the archive is written. Files older than `RETENTION_DAYS` days are reliably selected regardless of filename format. `-maxdepth 1` avoids descending into subdirectories. |
| `(( PRUNED++ )) \|\| true` | `(( expr ))` exits with status 1 when the result is 0 (i.e. the first increment). The `\|\| true` prevents `set -e` from treating the first increment as a failure. |
| `tee -a "$LOG_FILE"` | Writes to the log file and to stdout simultaneously. Stdout appears in cron mail; the log file persists for later inspection. |
| No compression pipeline through ssh | Pushing to remote storage is out of scope here. Use `rsync -az`, `rclone`, or `aws s3 cp` as a post-backup step rather than embedding network transfer in the backup script itself. |

## Verify it worked

```bash
# Check the log for the most recent run
tail -20 /var/log/backup-myapp.log

# Count archives and confirm oldest is within retention window
find /mnt/backups/myapp -name "*.tar.gz" -type f \
  -printf '%T+ %p\n' | sort
```
