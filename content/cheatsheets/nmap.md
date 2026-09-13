---
title: "Nmap"
description: "Nmap host discovery, port scanning, service detection, NSE scripts, output formats, and evasion techniques."
icon: "🔍"
weight: 9
count: 55
tags: ["nmap", "network", "recon", "security"]
---

## Host Discovery

```bash
nmap -sn 192.168.1.0/24                      # ping sweep, no port scan
nmap -sn 192.168.1.0/24 --exclude 192.168.1.1
nmap -sn -iL targets.txt                     # from file
nmap -PE -sn 192.168.1.0/24                  # ICMP echo only
nmap -PP -sn 192.168.1.0/24                  # ICMP timestamp
nmap -PM -sn 192.168.1.0/24                  # ICMP address mask
nmap -PS22,80,443 -sn 192.168.1.0/24        # TCP SYN discovery
nmap -PA80,443 -sn 192.168.1.0/24           # TCP ACK discovery
nmap -PU53 -sn 192.168.1.0/24               # UDP discovery
nmap -Pn <target>                             # skip host discovery (treat as up)
```

## Port Scanning

```bash
nmap <target>                                 # top 1000 TCP ports
nmap -p 22,80,443 <target>                   # specific ports
nmap -p 1-65535 <target>                     # full range
nmap -p- <target>                            # full range (shorthand)
nmap -p U:53,T:80,443 <target>              # UDP + TCP
nmap --top-ports 100 <target>
nmap --top-ports 1000 <target>
nmap -F <target>                             # fast — top 100 ports
```

## Scan Types

```bash
nmap -sS <target>                            # SYN scan (stealth, default root)
nmap -sT <target>                            # TCP connect (no root needed)
nmap -sU <target>                            # UDP scan
nmap -sU -sS <target>                        # UDP + SYN combined
nmap -sA <target>                            # ACK scan (firewall mapping)
nmap -sW <target>                            # Window scan
nmap -sM <target>                            # Maimon scan
nmap -sN <target>                            # TCP Null scan
nmap -sF <target>                            # FIN scan
nmap -sX <target>                            # Xmas scan
nmap -sI <zombie> <target>                   # Idle/zombie scan
nmap -sY <target>                            # SCTP INIT scan
```

## Version & OS Detection

```bash
nmap -sV <target>                            # service/version detection
nmap -sV --version-intensity 0 <target>     # lightest probe
nmap -sV --version-intensity 9 <target>     # most aggressive
nmap -sV --version-all <target>             # try every probe
nmap -O <target>                             # OS detection
nmap -O --osscan-guess <target>             # guess if not certain
nmap -O --osscan-limit <target>             # only likely targets
nmap -A <target>                             # OS + version + scripts + traceroute
```

## NSE Scripts

```bash
nmap --script=default <target>
nmap --script=safe <target>
nmap --script=vuln <target>
nmap --script=auth <target>
nmap --script=exploit <target>
nmap --script=discovery <target>
nmap --script=intrusive <target>

# HTTP
nmap --script=http-title <target>
nmap --script=http-headers <target>
nmap --script=http-methods <target>
nmap --script=http-auth <target>
nmap --script=http-enum <target>            # web path enumeration
nmap --script=http-robots.txt <target>

# SMB
nmap --script=smb-vuln-ms17-010 <target>   # EternalBlue
nmap --script=smb-vuln-ms08-067 <target>   # MS08-067
nmap --script=smb-enum-shares <target>
nmap --script=smb-enum-users <target>
nmap --script=smb-os-discovery <target>
nmap -p 445 --script=smb-security-mode <target>

# FTP
nmap --script=ftp-anon <target>
nmap --script=ftp-brute <target>

# SSH
nmap --script=ssh-hostkey <target>
nmap --script=ssh-auth-methods <target>
nmap --script=ssh2-enum-algos <target>

# SSL/TLS
nmap --script=ssl-cert <target>
nmap --script=ssl-enum-ciphers <target>
nmap --script=ssl-heartbleed <target>
nmap --script=ssl-poodle <target>

# DNS
nmap --script=dns-zone-transfer <target>
nmap --script=dns-brute <target>
nmap -p 53 --script=dns-recursion <target>

# Database
nmap --script=mysql-info <target>
nmap --script=mysql-empty-password <target>
nmap --script=ms-sql-info <target>

# Script args
nmap --script=http-brute --script-args userdb=users.txt,passdb=pass.txt <target>
nmap --script-help=smb-vuln-ms17-010
nmap --script-updatedb
```

## Timing & Performance

```bash
nmap -T0 <target>                            # paranoid  — 5 min between probes
nmap -T1 <target>                            # sneaky    — 15 sec between probes
nmap -T2 <target>                            # polite    — 0.4 sec between probes
nmap -T3 <target>                            # normal    (default)
nmap -T4 <target>                            # aggressive
nmap -T5 <target>                            # insane

nmap --min-rate 1000 <target>               # min packets per second
nmap --max-rate 500 <target>
nmap --min-parallelism 10 <target>
nmap --max-retries 2 <target>
nmap --host-timeout 30s <target>
```

## Evasion & Spoofing

```bash
nmap -f <target>                             # fragment packets
nmap -f -f <target>                          # 16-byte fragments
nmap --mtu 24 <target>                       # custom MTU (must be multiple of 8)
nmap -D RND:10 <target>                      # decoy scan (10 random decoys)
nmap -D 192.168.1.1,192.168.1.2 <target>   # specific decoys
nmap -S <spoof-ip> <target>                 # spoof source IP
nmap -e eth0 <target>                        # specify interface
nmap -g 53 <target>                          # spoof source port
nmap --source-port 53 <target>
nmap --data-length 25 <target>              # append random data
nmap --randomize-hosts -iL targets.txt      # randomise scan order
nmap --scan-delay 1s <target>              # delay between probes
nmap --badsum <target>                       # send bad checksums (detect firewalls)
```

## Output

```bash
nmap -oN output.txt <target>                 # normal
nmap -oX output.xml <target>                 # XML
nmap -oG output.gnmap <target>              # grepable
nmap -oA output <target>                     # all three formats
nmap -oS output.txt <target>                 # script kiddie (leet speak)
nmap -v <target>                             # verbose
nmap -vv <target>                            # very verbose
nmap -d <target>                             # debug
nmap --reason <target>                       # show why port is in state
nmap --open <target>                         # only show open ports
nmap --packet-trace <target>                # show packets sent/received
nmap -n <target>                             # no DNS resolution
nmap -R <target>                             # always resolve DNS
```

## Useful Combinations

```bash
# Full TCP + service + OS detection, save all formats
nmap -sS -sV -O -p- -T4 -A -oA fullscan <target>

# Quick web service check
nmap -sV -p 80,443,8080,8443 --script=http-title,http-headers <target>

# SMB vulnerability check
nmap -p 445 --script=smb-vuln-* <target>

# UDP top ports + version
nmap -sU -sV --top-ports 20 <target>

# Stealth scan with decoys, fragmented packets
nmap -sS -f -D RND:5 -T2 <target>

# Parse grepable output for open ports
grep "Ports:" output.gnmap | grep -oP '\d+/open' | cut -d/ -f1
```
