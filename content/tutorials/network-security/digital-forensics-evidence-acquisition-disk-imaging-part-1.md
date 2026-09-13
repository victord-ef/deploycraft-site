---
title: "Digital Forensics Fundamentals — Evidence Acquisition, Chain of Custody, and Disk Imaging — Part 1"
date: 2026-09-01
description: "Learn the foundational principles of digital forensics: maintaining chain of custody, acquiring forensic disk images without contaminating evidence, verifying image integrity, and preserving volatile data before system shutdown."
cluster: "Network Security"
series: "Forensics"
part: 1
difficulty: "intermediate"
duration: "45 min"
tags: ["forensics", "incident-response", "evidence-acquisition", "disk-imaging", "network-security", "devsecops", "security-engineering"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

By the end of this tutorial you will understand the principles and process of forensic evidence acquisition: why order of volatility matters, how to take a forensic disk image without modifying the source, how to verify image integrity, and how to document chain of custody. Part 2 covers Linux memory forensics, log analysis, timeline reconstruction, and artefact recovery.

## Prerequisites

- Familiarity with Linux file systems (ext4, partitions, block devices)
- Basic Linux command line (dd, sha256sum, mount)
- Understanding of incident response concepts

---

## Why digital forensics matters

Digital forensics is the application of scientific methodology to the collection, preservation, examination, and presentation of digital evidence. It is used for:

- **Incident response:** Determining what happened, how, and when during a security breach
- **Legal proceedings:** Providing evidence admissible in court (chain of custody is mandatory)
- **Compliance investigations:** Demonstrating due diligence after a data breach
- **Internal investigations:** Employee misconduct, insider threat, IP theft

Forensic investigation differs from standard sysadmin troubleshooting in one critical way: **every action you take on a live system modifies potential evidence.** The forensic investigator's first obligation is to preserve evidence — then analyse it.

---

## Step 1 — Order of volatility

Evidence disappears at different rates. Collect the most volatile evidence first.

| Volatility | Data type | Why it disappears |
|---|---|---|
| Highest | CPU registers, cache | Lost immediately on shutdown |
| | Running processes (RAM) | Lost on shutdown or process termination |
| | Network connections (netstat) | Connections close naturally |
| | Kernel state (loaded modules, routing tables) | Lost on shutdown |
| | Temp files (`/tmp`, swap) | May be cleared on reboot |
| | System logs | May be overwritten by rotation |
| | Disk artefacts (deleted files, slack space) | Overwritten as disk fills |
| Lowest | Backup media, optical disks | Preserved until physically destroyed |

**Capture memory before shutting down the system.** A powered-off system cannot provide RAM contents. If the system is encrypted, the decryption keys exist only in RAM while the system is running.

---

## Step 2 — Chain of custody

Chain of custody is the documented history of who collected, handled, transferred, and stored evidence. Without it, evidence may be inadmissible in legal proceedings and its integrity questioned.

A chain of custody record includes:

```
EVIDENCE COLLECTION FORM
─────────────────────────────────────────────────────────────
Case number:       INC-2026-001
Date/time:         2026-09-01 14:32 UTC
Collected by:      J. Smith (Senior Security Analyst)
Witnessed by:      A. Jones (IT Manager)

Evidence item:     Forensic image of server srv-prod-01
Original media:    2TB Samsung SSD, S/N: SN12345678
Image file:        srv-prod-01-disk.dd
Image size:        2,000,398,934,016 bytes
SHA-256 (source):  a3f4b2c1...
SHA-256 (image):   a3f4b2c1...   ← must match source
Write blocker:     Tableau TX1, S/N: TX123456
Collection method: dd with write-blocker, verified with sha256sum

Storage location:  Encrypted USB drive, locked in Evidence Room B-12
─────────────────────────────────────────────────────────────
Transfer history:
  2026-09-01 16:00  J. Smith → Evidence Room B-12  (secured)
  2026-09-02 09:15  J. Smith retrieved for analysis
  2026-09-02 17:30  J. Smith returned to Evidence Room B-12
```

Every access to evidence must be logged. Evidence must be stored in a tamper-evident container. The hash values prove the evidence has not been modified since collection.

---

## Step 3 — Write blocking

A write blocker prevents any write operations to the evidence disk. Without a write blocker, mounting a disk (even read-only) updates access timestamps, potentially modifying evidence.

### Hardware write blockers

Dedicated forensic write blockers (Tableau, WiebeTech) sit between the evidence disk and the analysis workstation at the hardware level. Any write command sent by the OS is intercepted and discarded by the hardware. The only truly safe option for legal proceedings.

```
Evidence disk → Write blocker → USB/SATA → Analysis workstation
                [hardware]      [connection]
```

### Software write blocking on Linux

If a hardware write blocker is unavailable, use Linux block device read-only mounting:

```bash
# Block all writes to the evidence device at the kernel level
blockdev --setro /dev/sdb

# Verify the device is read-only
blockdev --getro /dev/sdb
# 1 (read-only)

# Mount with explicit read-only flag and noatime (do not update access times)
mount -o ro,noatime,noexec /dev/sdb1 /mnt/evidence

# Verify mount options
mount | grep evidence
# /dev/sdb1 on /mnt/evidence type ext4 (ro,noatime,noexec)
```

**Warning:** Software write blocking is less reliable than hardware. The OS kernel may still write (e.g., journal recovery on mount). For legal proceedings, use hardware write blockers.

---

## Step 4 — Acquire a forensic disk image

A forensic image is a bit-for-bit copy of every sector on the evidence disk, including deleted files, unallocated space, and file system slack. This is different from a simple file copy, which only copies existing files.

### Using dc3dd (enhanced forensic dd)

```bash
# Install dc3dd
apt-get install -y dc3dd

# Acquire image with real-time hashing and progress reporting
dc3dd if=/dev/sdb \
      of=/evidence/srv-prod-01-disk.dd \
      hash=sha256 \
      log=/evidence/srv-prod-01-acquisition.log \
      hlog=/evidence/srv-prod-01-hash.log \
      bsize=65536 \
      verb=on

# Output includes:
# Total sectors hashed: 3,907,029,168
# SHA-256 of input: a3f4b2c1d5e6f7a8...
# SHA-256 of output: a3f4b2c1d5e6f7a8...  ← identical confirms integrity
```

### Using dd with manual hashing

```bash
# Calculate hash of source before imaging (establishes baseline)
sha256sum /dev/sdb > /evidence/source-hash.txt

# Acquire image
dd if=/dev/sdb \
   of=/evidence/srv-prod-01-disk.dd \
   bs=65536 \
   conv=noerror,sync \
   status=progress

# Verify image matches source
sha256sum /evidence/srv-prod-01-disk.dd

# Compare: both hashes must match
diff /evidence/source-hash.txt <(sha256sum /evidence/srv-prod-01-disk.dd | awk '{print $1}')
```

The `conv=noerror,sync` option continues imaging past read errors (filling unreadable sectors with zeros) — critical for damaged disks. Without it, dd exits on the first read error.

### Compressed image with ewfacquire (Expert Witness Format)

EWF (.E01) format stores the image with built-in hash verification, case metadata, and compression:

```bash
apt-get install -y ewf-tools

ewfacquire /dev/sdb \
  -t /evidence/srv-prod-01 \
  -c best \
  -S 2GiB \
  -m removable \
  -u Examiner:J.Smith \
  -D "Investigation INC-2026-001" \
  -e "J.Smith@example.com"

# Creates: srv-prod-01.E01, srv-prod-01.E02, etc. (2GB segments)
# Embedded hash: srv-prod-01.E01 contains SHA-1 and MD5 of every segment
```

---

## Step 5 — Verify image integrity

After acquisition, verify the image hash matches the source hash documented in the chain of custody:

```bash
# Compute hash of the completed image
sha256sum /evidence/srv-prod-01-disk.dd | tee /evidence/image-hash.txt

# Verify EWF image integrity (checks internal hashes)
ewfverify /evidence/srv-prod-01.E01
# Verification completed.
# MD5 hash stored in file:    a3f4b2c1...
# MD5 hash calculated over data: a3f4b2c1...
# ewfverify: SUCCESS

# Document the verified hash in the chain of custody form
echo "Image integrity verified at $(date -u)" >> /evidence/chain-of-custody.txt
```

---

## Step 6 — Capture volatile data (live system)

If the system is still running and you must preserve volatile evidence before shutdown:

```bash
# Document the current timestamp (correlate all logs to this)
date -u | tee /evidence/collection-timestamp.txt

# Capture running processes (full process tree with open files)
ps auxf              | tee /evidence/processes.txt
lsof -n              | tee /evidence/open-files.txt
pstree -p            | tee /evidence/process-tree.txt

# Network state
netstat -anp         | tee /evidence/netstat.txt
ss -tulpna           | tee /evidence/socket-state.txt
ip route show        | tee /evidence/routing-table.txt
arp -a               | tee /evidence/arp-cache.txt

# Logged-in users and recent activity
who                  | tee /evidence/logged-in-users.txt
w                    | tee /evidence/user-activity.txt
last -50             | tee /evidence/login-history.txt

# Loaded kernel modules (possible rootkit)
lsmod                | tee /evidence/kernel-modules.txt

# Environment variables of running processes
cat /proc/*/environ 2>/dev/null | tr '\0' '\n' | tee /evidence/env-vars.txt

# Scheduled tasks
crontab -l 2>/dev/null    | tee /evidence/crontab.txt
ls -la /etc/cron* /var/spool/cron | tee /evidence/cron-dirs.txt

# Installed packages and recent modifications
dpkg -l              | tee /evidence/installed-packages.txt
rpm -qa 2>/dev/null  | tee /evidence/installed-rpm.txt
```

### Capture RAM

```bash
# Install LiME (Linux Memory Extractor) kernel module
apt-get install -y build-essential linux-headers-$(uname -r)
git clone https://github.com/504ensicsLabs/LiME.git
cd LiME/src
make

# Acquire memory to a remote server (avoids writing to the evidence system)
insmod lime-$(uname -r).ko "path=tcp:4444 format=lime"

# On collection server
nc -l 4444 > /evidence/memory.lime

# Alternatively: write directly to an external drive
insmod lime-$(uname -r).ko "path=/mnt/external/memory.lime format=lime"

# Record memory size for documentation
wc -c /evidence/memory.lime
sha256sum /evidence/memory.lime | tee /evidence/memory-hash.txt
```

---

## Step 7 — Mount and examine the image

Examine the image without modifying the original evidence disk:

```bash
# Mount dd image as a loop device (read-only)
losetup -r -f --show /evidence/srv-prod-01-disk.dd
# /dev/loop0

# If the image contains a partition table, show partitions
fdisk -l /dev/loop0
# Device        Start       End   Sectors  Size Type
# /dev/loop0p1   2048  1048575   1046528  511M EFI System
# /dev/loop0p2  1048576 ...               ...  Linux filesystem

# Mount with partition offset
OFFSET=$(fdisk -l /dev/loop0 | grep loop0p2 | awk '{print $2 * 512}')
mount -o ro,noatime,offset=$OFFSET /dev/loop0 /mnt/evidence

# For EWF images — mount via ewf-fuse
apt-get install -y ewf-tools
mkdir /mnt/ewf /mnt/evidence
ewfmount /evidence/srv-prod-01.E01 /mnt/ewf
mount -o ro,noatime /mnt/ewf/ewf1 /mnt/evidence

# Verify you see the file system
ls /mnt/evidence
df -h /mnt/evidence
```

---

## Step 8 — Document everything

During acquisition, maintain real-time notes:

```bash
# Use script to log all commands to a terminal transcript
script /evidence/collection-transcript.txt

# All subsequent commands are recorded automatically
# When done:
exit    # stops script recording

# Hash the transcript itself
sha256sum /evidence/collection-transcript.txt >> /evidence/chain-of-custody.txt
```

**Minimum documentation for each evidence item:**
- System description: hostname, IP, OS, hardware
- Date, time (UTC), and who collected
- Hash of source media before imaging
- Hash of image after acquisition
- Tool versions used (dc3dd version, OS version)
- Any anomalies encountered during collection
- Storage location and access log

---

## What you have built

- Order of volatility — which data to collect first and why
- Chain of custody documentation requirements for admissible evidence
- Write blocking — hardware and software approaches, and their limitations
- Forensic disk imaging with dc3dd, dd, and EWF format using ewfacquire
- Image integrity verification matching acquisition hash to documented baseline
- Volatile evidence capture — running processes, network state, open files, RAM acquisition with LiME
- Read-only image mounting for analysis without contaminating evidence
- Real-time documentation with script and hash verification

In [Part 2](/tutorials/network-security/linux-memory-log-forensics-timeline-analysis-part-2/) you will analyse the acquired evidence: extract artefacts from a Linux memory dump with Volatility, reconstruct a timeline from file system timestamps, analyse log files for attacker activity, and recover deleted files.
