---
title: "Gobuster"
description: "Gobuster commands for directory and file brute forcing, DNS subdomain enumeration, virtual host discovery, and S3 bucket fuzzing."
icon: "🔎"
weight: 17
count: 45
tags: ["gobuster", "fuzzing", "recon", "pentesting", "security"]
---

{{< callout type="warning" >}}
Gobuster is for **authorised penetration testing, CTF competitions, and security research only**. Use only on systems you have explicit written permission to test.
{{< /callout >}}

## Dir Mode — Directory & File Brute Force

```bash
# Basic directory scan
gobuster dir -u http://target.com -w /usr/share/wordlists/dirb/common.txt

# With file extensions
gobuster dir -u http://target.com -w /usr/share/wordlists/dirb/common.txt -x php,html,txt,bak

# Follow redirects
gobuster dir -u http://target.com -w /usr/share/wordlists/dirb/common.txt -r

# Show only specific status codes
gobuster dir -u http://target.com -w wordlist.txt -s 200,204,301,302,307,401,403

# Exclude status codes
gobuster dir -u http://target.com -w wordlist.txt --exclude-length 0

# Add cookie (authenticated scan)
gobuster dir -u http://target.com -w wordlist.txt -c "session=abc123"

# Add custom header
gobuster dir -u http://target.com -w wordlist.txt -H "Authorization: Bearer <token>"

# Custom user agent
gobuster dir -u http://target.com -w wordlist.txt -a "Mozilla/5.0"

# Proxy through Burp
gobuster dir -u http://target.com -w wordlist.txt --proxy http://127.0.0.1:8080

# Increase threads (default 10)
gobuster dir -u http://target.com -w wordlist.txt -t 50

# Timeout per request
gobuster dir -u http://target.com -w wordlist.txt --timeout 10s

# Append slash to each word
gobuster dir -u http://target.com -w wordlist.txt --add-slash

# Case insensitive (lowercase all words)
gobuster dir -u http://target.com -w wordlist.txt --lowercase

# Output to file
gobuster dir -u http://target.com -w wordlist.txt -o results.txt

# No TLS verification
gobuster dir -u https://target.com -w wordlist.txt -k

# Verbose — show all attempts
gobuster dir -u http://target.com -w wordlist.txt -v

# Expanded output — show full URL
gobuster dir -u http://target.com -w wordlist.txt -e
```

## DNS Mode — Subdomain Enumeration

```bash
# Basic subdomain brute force
gobuster dns -d target.com -w /usr/share/wordlists/subdomains.txt

# Show IP addresses in output
gobuster dns -d target.com -w wordlist.txt --show-ips

# Use custom DNS resolver
gobuster dns -d target.com -w wordlist.txt -r 8.8.8.8
gobuster dns -d target.com -w wordlist.txt -r 8.8.8.8:53

# Wildcard detection (auto — warns if wildcard DNS is configured)
gobuster dns -d target.com -w wordlist.txt

# Output to file
gobuster dns -d target.com -w wordlist.txt -o dns_results.txt

# Increase threads
gobuster dns -d target.com -w wordlist.txt -t 50

# Timeout
gobuster dns -d target.com -w wordlist.txt --timeout 5s
```

## VHost Mode — Virtual Host Discovery

```bash
# Basic vhost scan
gobuster vhost -u http://target.com -w /usr/share/wordlists/subdomains.txt

# HTTPS
gobuster vhost -u https://target.com -w wordlist.txt -k

# Append domain to each word (e.g. dev → dev.target.com)
gobuster vhost -u http://target.com -w wordlist.txt --append-domain

# Exclude response length (filter false positives)
gobuster vhost -u http://target.com -w wordlist.txt --exclude-length 250

# Custom header
gobuster vhost -u http://target.com -w wordlist.txt -H "X-Custom-Header: value"

# Output to file
gobuster vhost -u http://target.com -w wordlist.txt -o vhost_results.txt

# Increase threads
gobuster vhost -u http://target.com -w wordlist.txt -t 50
```

## Fuzz Mode — URL Fuzzing

```bash
# Fuzz a URL position marked with FUZZ
gobuster fuzz -u "http://target.com/FUZZ" -w wordlist.txt

# Fuzz a parameter value
gobuster fuzz -u "http://target.com/page?id=FUZZ" -w /usr/share/wordlists/numbers.txt

# Exclude specific status codes
gobuster fuzz -u "http://target.com/FUZZ" -w wordlist.txt --exclude-length 0

# POST body fuzzing
gobuster fuzz -u "http://target.com/login" -w wordlist.txt \
  --method POST --body "username=FUZZ&password=test"

# With headers
gobuster fuzz -u "http://target.com/FUZZ" -w wordlist.txt \
  -H "Content-Type: application/json"
```

## S3 Mode — S3 Bucket Enumeration

```bash
# Enumerate S3 buckets
gobuster s3 -w wordlist.txt

# With custom wordlist of company names / prefixes
gobuster s3 -w company-names.txt

# Increase threads
gobuster s3 -w wordlist.txt -t 50

# Output to file
gobuster s3 -w wordlist.txt -o s3_results.txt
```

## TFTP Mode

```bash
# Enumerate files on a TFTP server
gobuster tftp -s target.com -w wordlist.txt
```

## Recommended Wordlists

```bash
# SecLists (install: apt install seclists)
/usr/share/seclists/Discovery/Web-Content/common.txt
/usr/share/seclists/Discovery/Web-Content/directory-list-2.3-medium.txt
/usr/share/seclists/Discovery/Web-Content/directory-list-2.3-big.txt
/usr/share/seclists/Discovery/Web-Content/raft-large-files.txt
/usr/share/seclists/Discovery/Web-Content/raft-large-directories.txt
/usr/share/seclists/Discovery/Web-Content/api/api-endpoints.txt
/usr/share/seclists/Discovery/DNS/subdomains-top1million-5000.txt
/usr/share/seclists/Discovery/DNS/subdomains-top1million-20000.txt

# Dirb (built-in)
/usr/share/wordlists/dirb/common.txt
/usr/share/wordlists/dirb/big.txt

# Dirbuster
/usr/share/wordlists/dirbuster/directory-list-2.3-medium.txt
```

## Useful Combinations

```bash
# Full web content discovery — common extensions, follow redirects, output file
gobuster dir -u http://target.com \
  -w /usr/share/seclists/Discovery/Web-Content/directory-list-2.3-medium.txt \
  -x php,html,txt,bak,zip,sql,conf,log \
  -r -t 50 -k -o gobuster_dir.txt

# Authenticated scan with session cookie
gobuster dir -u http://target.com/admin \
  -w /usr/share/seclists/Discovery/Web-Content/common.txt \
  -c "PHPSESSID=abc123; role=admin" \
  -x php,html -t 30 -o admin_scan.txt

# Subdomain enumeration with IP resolution
gobuster dns -d target.com \
  -w /usr/share/seclists/Discovery/DNS/subdomains-top1million-20000.txt \
  --show-ips -t 50 -o subdomains.txt

# API endpoint discovery
gobuster dir -u http://target.com/api \
  -w /usr/share/seclists/Discovery/Web-Content/api/api-endpoints.txt \
  -x json -t 40 -o api_scan.txt

# VHost discovery filtering noise by response length
gobuster vhost -u http://target.com \
  -w /usr/share/seclists/Discovery/DNS/subdomains-top1million-5000.txt \
  --append-domain --exclude-length 301 -t 50 -o vhosts.txt
```
