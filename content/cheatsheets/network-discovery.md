---
title: "Network Discovery"
description: "Nmap, masscan, netdiscover, and DNS enumeration commands for host discovery, port scanning, and service fingerprinting."
icon: "🌍"
weight: 9
count: 45
tags: ["nmap", "network", "recon", "security"]
---

## Nmap — Host Discovery

```bash
nmap -sn 192.168.1.0/24                     # ping sweep, no port scan
nmap -sn 192.168.1.0/24 --exclude 192.168.1.1
nmap -PE -sn 192.168.1.0/24                 # ICMP echo only
nmap -PS22,80,443 -sn 192.168.1.0/24       # TCP SYN discovery
nmap -PA80,443 -sn 192.168.1.0/24          # TCP ACK discovery
nmap -sn -iL targets.txt                    # from file
```

## Nmap — Port Scanning

```bash
nmap <target>                               # top 1000 TCP ports
nmap -p 22,80,443 <target>
nmap -p 1-65535 <target>                   # full port range
nmap -p- <target>                          # same as above
nmap --top-ports 100 <target>
nmap -F <target>                           # fast, top 100 ports

# Scan types
nmap -sS <target>                          # SYN scan (stealth, default)
nmap -sT <target>                          # TCP connect scan
nmap -sU <target>                          # UDP scan
nmap -sA <target>                          # ACK scan (firewall mapping)
nmap -sW <target>                          # Window scan
nmap -sN <target>                          # TCP Null scan
nmap -sF <target>                          # FIN scan
nmap -sX <target>                          # Xmas scan
```

## Nmap — Version & OS Detection

```bash
nmap -sV <target>                          # service/version detection
nmap -sV --version-intensity 9 <target>   # aggressive version detection
nmap -O <target>                           # OS detection
nmap -O --osscan-guess <target>
nmap -A <target>                           # OS + version + scripts + traceroute
```

## Nmap — Scripts (NSE)

```bash
nmap --script=default <target>
nmap --script=vuln <target>
nmap --script=safe <target>
nmap --script=http-title <target>
nmap --script=http-headers <target>
nmap --script=smb-vuln-ms17-010 <target>  # EternalBlue check
nmap --script=smb-enum-shares <target>
nmap --script=ftp-anon <target>
nmap --script=ssh-hostkey <target>
nmap --script=ssl-cert <target>
nmap --script=banner <target>
nmap --script-help=<script>
nmap --script-updatedb
```

## Nmap — Output & Timing

```bash
nmap -oN output.txt <target>               # normal
nmap -oX output.xml <target>               # XML
nmap -oG output.gnmap <target>             # grepable
nmap -oA output <target>                   # all formats

nmap -T0 <target>                          # paranoid (IDS evasion)
nmap -T1 <target>                          # sneaky
nmap -T2 <target>                          # polite
nmap -T3 <target>                          # normal (default)
nmap -T4 <target>                          # aggressive
nmap -T5 <target>                          # insane

nmap -v <target>                           # verbose
nmap -vv <target>                          # very verbose
nmap -d <target>                           # debug
nmap --reason <target>                     # show reason for state
nmap --open <target>                       # only open ports
```

## Masscan

```bash
masscan -p80,443,8080 192.168.1.0/24
masscan -p1-65535 192.168.1.0/24 --rate=10000
masscan -p80 0.0.0.0/0 --rate=100000 --exclude 255.255.255.255
masscan -iL targets.txt -p22,80,443 --rate=5000
masscan -p22 192.168.1.0/24 -oX output.xml
```

## Netdiscover

```bash
netdiscover -r 192.168.1.0/24
netdiscover -i eth0
netdiscover -p                             # passive mode
netdiscover -r 192.168.1.0/24 -oN out.txt
```

## DNS Enumeration

```bash
# Basic lookups
dig <domain>
dig <domain> ANY
dig <domain> MX
dig <domain> NS
dig <domain> TXT
dig @8.8.8.8 <domain>                     # specific resolver
dig +short <domain>

# Reverse lookup
dig -x 192.168.1.1

# Zone transfer
dig axfr <domain> @<nameserver>
host -t axfr <domain> <nameserver>

# Subdomain enumeration
gobuster dns -d <domain> -w /usr/share/wordlists/subdomains.txt
amass enum -d <domain>
subfinder -d <domain>
dnsrecon -d <domain>
dnsrecon -d <domain> -D wordlist.txt -t brt   # brute force

# nslookup
nslookup <domain>
nslookup <domain> 8.8.8.8
nslookup -type=MX <domain>
```

## Network Utilities

```bash
# Interface info
ip addr
ip link
ip route
ip neigh                                   # ARP table

# Connection state
ss -tuln                                   # listening ports
ss -tunp                                   # with process names
netstat -tuln
netstat -anp | grep <port>

# Packet capture
tcpdump -i eth0
tcpdump -i eth0 port 80
tcpdump -i eth0 host 192.168.1.1
tcpdump -i eth0 -w capture.pcap
tcpdump -r capture.pcap

# Connectivity
traceroute <host>
tracepath <host>
mtr <host>                                 # real-time traceroute
```
