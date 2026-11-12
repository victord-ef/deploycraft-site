---
title: "Log Error Extraction and Summary"
date: 2026-11-12
description: "Extract errors, warnings, and stack traces from application logs using grep and awk. Build a frequency table of unique error messages and isolate the log window around each occurrence."
lang: "Bash"
tags: ["bash", "logs", "grep", "awk", "debugging", "sysadmin", "monitoring"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-11-12"
draft: false
---

## When to use this

When an alert fires and you need to understand what went wrong before opening the full log. These patterns extract the signal — unique errors with counts, stack traces, and the lines surrounding each failure — without scrolling through thousands of INFO lines.

## Snippet

### Count unique error messages (frequency table)

```bash
# Count unique ERROR lines, sorted by frequency (most common first)
grep -i "error\|exception\|fatal\|panic" app.log \
  | awk '{
      # Strip timestamps, PIDs, and request IDs (common prefixes)
      sub(/^[0-9TZ:.\-]+ /, "")   # ISO timestamp
      sub(/^\[[0-9]+\] /, "")     # [PID]
      sub(/^[a-f0-9-]{36} /, "")  # UUID
      print
    }' \
  | sort \
  | uniq -c \
  | sort -rn \
  | head -20
```

### Show N lines of context around each error

```bash
# 5 lines before and after every ERROR — useful for seeing what triggered it
grep -n -i "error\|exception\|fatal" app.log \
  | awk -F: '{print $1}' \
  | while read -r lineno; do
      echo "──── Line ${lineno} ────"
      sed -n "$((lineno - 5)),$((lineno + 5))p" app.log
      echo ""
    done
```

### Extract errors within a time window

```bash
# Errors between 14:00 and 15:00 UTC (adjust timestamp format to match your logs)
awk '/2026-11-12T14:00/,/2026-11-12T15:00/' app.log \
  | grep -i "error\|exception\|fatal"
```

### Extract and deduplicate Java / Python stack traces

```bash
# Capture multi-line stack traces: starts with ERROR/FATAL, includes indented lines
awk '
  /^[0-9TZ:.\-]+ (ERROR|FATAL)/ { capture=1; block=$0; next }
  capture && /^[[:space:]]/ { block=block "\n" $0; next }
  capture { print block; print ""; capture=0 }
  END { if (capture) { print block } }
' app.log | sort -u
```

### Live tail — show only errors as they arrive

```bash
tail -f app.log | grep --line-buffered -i "error\|exception\|fatal\|panic"
```

### Build a per-hour error rate from a log file

```bash
# Count errors per hour to find the spike window
grep -i "error\|exception\|fatal" app.log \
  | grep -oE '[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}' \
  | sort \
  | uniq -c \
  | awk '{printf "%s:00  %s errors\n", $2, $1}'
```

### Output

```
2026-11-12T09:00  3 errors
2026-11-12T10:00  7 errors
2026-11-12T11:00  142 errors    ← spike
2026-11-12T12:00  5 errors
```

**Key decisions:**

| Decision | Why it matters |
|---|---|
| `grep -i` (case-insensitive) | Log libraries differ: some emit `ERROR`, others `Error`, `error`, or `[error]`. Case-insensitive matching catches all variants without an alternation regex. |
| Strip timestamps and IDs before `uniq -c` | Two identical errors at different times differ only in their timestamp or request ID. Without stripping these prefixes, `uniq -c` counts each occurrence as a unique message and the frequency table is useless. |
| `--line-buffered` on `tail -f \| grep` | Without this, `grep` buffers output and nothing appears until the buffer fills. `--line-buffered` flushes after every line — essential for real-time tailing. |
| `awk` range pattern `/start/,/end/` | Selects all lines between two patterns inclusive. Much faster than piping through two `grep` calls for large log files, and handles multi-line ranges correctly. |
| `sort -u` on stack traces | Identical stack traces are extremely common during error storms. Deduplicating them reduces pages of identical output to a single representative trace. |
| `grep -oE` to extract the hour | Extracts only the matched portion (`-o`) using an extended regex (`-E`). Avoids `awk` or `cut` for a simple timestamp truncation. |

## Verify it worked

```bash
# Generate a test log with known errors
printf '%s\n' \
  '2026-11-12T10:00:01Z INFO started' \
  '2026-11-12T10:00:02Z ERROR database connection refused' \
  '2026-11-12T10:00:03Z INFO retrying' \
  '2026-11-12T10:00:04Z ERROR database connection refused' \
  '2026-11-12T10:00:05Z FATAL out of memory' \
  > /tmp/test.log

grep -i "error\|fatal" /tmp/test.log \
  | awk '{sub(/^[0-9TZ:.\-]+ /, ""); print}' \
  | sort | uniq -c | sort -rn
```

Expected output:
```
   2 ERROR database connection refused
   1 FATAL out of memory
```
