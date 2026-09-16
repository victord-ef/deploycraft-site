---
title: "ffuf"
description: "ffuf commands for web fuzzing — directory discovery, parameter fuzzing, subdomain enumeration, virtual hosts, POST body fuzzing, and filter techniques."
icon: "⚡"
weight: 18
count: 50
tags: ["ffuf", "fuzzing", "recon", "pentesting", "security"]
---

{{< callout type="warning" >}}
ffuf is for **authorised penetration testing, CTF competitions, and security research only**. Use only on systems you have explicit written permission to test.
{{< /callout >}}

## Basic Usage

```bash
# Directory fuzzing
ffuf -u http://target.com/FUZZ -w /usr/share/wordlists/dirb/common.txt

# File fuzzing with extensions
ffuf -u http://target.com/FUZZ -w wordlist.txt -e .php,.html,.txt,.bak,.zip

# Multiple wordlists (FUZZ and W2 positions)
ffuf -u http://target.com/FUZZ/W2 -w wordlist1.txt:FUZZ -w wordlist2.txt:W2

# POST body fuzzing
ffuf -u http://target.com/login -w wordlist.txt \
  -X POST -d "username=admin&password=FUZZ" \
  -H "Content-Type: application/x-www-form-urlencoded"

# JSON body fuzzing
ffuf -u http://target.com/api/login -w wordlist.txt \
  -X POST -d '{"username":"admin","password":"FUZZ"}' \
  -H "Content-Type: application/json"

# Fuzz a GET parameter value
ffuf -u "http://target.com/page?id=FUZZ" -w /usr/share/wordlists/numbers.txt

# Fuzz a header value
ffuf -u http://target.com/ -w wordlist.txt -H "X-Forwarded-For: FUZZ"

# Fuzz cookie value
ffuf -u http://target.com/ -w wordlist.txt -H "Cookie: session=FUZZ"
```

## Subdomain & VHost Enumeration

```bash
# Subdomain fuzzing (DNS)
ffuf -u http://FUZZ.target.com -w /usr/share/seclists/Discovery/DNS/subdomains-top1million-5000.txt

# Virtual host discovery (Host header fuzzing)
ffuf -u http://target.com -w /usr/share/seclists/Discovery/DNS/subdomains-top1million-5000.txt \
  -H "Host: FUZZ.target.com"

# Filter out default response size to remove false positives
ffuf -u http://target.com -w wordlist.txt \
  -H "Host: FUZZ.target.com" \
  -fs 1234
```

## Filtering & Matching

```bash
# Match by status code (show only these)
ffuf -u http://target.com/FUZZ -w wordlist.txt -mc 200,301,302,401,403

# Filter by status code (hide these)
ffuf -u http://target.com/FUZZ -w wordlist.txt -fc 404

# Filter by response size (bytes)
ffuf -u http://target.com/FUZZ -w wordlist.txt -fs 1234
ffuf -u http://target.com/FUZZ -w wordlist.txt -fs 1234,5678    # multiple sizes

# Match by response size
ffuf -u http://target.com/FUZZ -w wordlist.txt -ms 512

# Filter by word count
ffuf -u http://target.com/FUZZ -w wordlist.txt -fw 10

# Match by word count
ffuf -u http://target.com/FUZZ -w wordlist.txt -mw 25

# Filter by line count
ffuf -u http://target.com/FUZZ -w wordlist.txt -fl 10

# Match by line count
ffuf -u http://target.com/FUZZ -w wordlist.txt -ml 5

# Match by regex in response
ffuf -u http://target.com/FUZZ -w wordlist.txt -mr "admin|dashboard|welcome"

# Filter by regex
ffuf -u http://target.com/FUZZ -w wordlist.txt -fr "Not Found|Error"

# Match by response time (ms)
ffuf -u http://target.com/FUZZ -w wordlist.txt -mt 500

# Filter by response time (useful for time-based blind SQLi)
ffuf -u "http://target.com/?id=FUZZ" -w payloads.txt -ft 5000
```

## Authentication & Headers

```bash
# Cookie-based auth
ffuf -u http://target.com/FUZZ -w wordlist.txt \
  -H "Cookie: PHPSESSID=abc123"

# Bearer token
ffuf -u http://target.com/api/FUZZ -w wordlist.txt \
  -H "Authorization: Bearer eyJ..."

# Basic auth
ffuf -u http://target.com/FUZZ -w wordlist.txt \
  -H "Authorization: Basic $(echo -n 'admin:password' | base64)"

# Multiple custom headers
ffuf -u http://target.com/FUZZ -w wordlist.txt \
  -H "X-Forwarded-For: 127.0.0.1" \
  -H "User-Agent: Mozilla/5.0"
```

## Proxy & TLS

```bash
# Route through Burp Suite
ffuf -u http://target.com/FUZZ -w wordlist.txt -x http://127.0.0.1:8080

# Skip TLS verification
ffuf -u https://target.com/FUZZ -w wordlist.txt -k

# Follow redirects
ffuf -u http://target.com/FUZZ -w wordlist.txt -r
```

## Performance & Rate Limiting

```bash
# Set threads (default 40)
ffuf -u http://target.com/FUZZ -w wordlist.txt -t 100

# Delay between requests (ms)
ffuf -u http://target.com/FUZZ -w wordlist.txt -p 100

# Randomised delay range (ms)
ffuf -u http://target.com/FUZZ -w wordlist.txt -p 50-200

# Rate limit (requests per second)
ffuf -u http://target.com/FUZZ -w wordlist.txt -rate 10

# Timeout per request (seconds, default 10)
ffuf -u http://target.com/FUZZ -w wordlist.txt -timeout 5

# Max number of results to return
ffuf -u http://target.com/FUZZ -w wordlist.txt -maxtime 60   # stop after 60s
```

## Output

```bash
# Save output — JSON
ffuf -u http://target.com/FUZZ -w wordlist.txt -o results.json -of json

# Save output — CSV
ffuf -u http://target.com/FUZZ -w wordlist.txt -o results.csv -of csv

# Save output — Markdown
ffuf -u http://target.com/FUZZ -w wordlist.txt -o results.md -of md

# Save output — HTML
ffuf -u http://target.com/FUZZ -w wordlist.txt -o results.html -of html

# Save all formats
ffuf -u http://target.com/FUZZ -w wordlist.txt -o results -of all

# Silent mode (only results, no banner)
ffuf -u http://target.com/FUZZ -w wordlist.txt -s

# Verbose (show all requests)
ffuf -u http://target.com/FUZZ -w wordlist.txt -v

# No colour
ffuf -u http://target.com/FUZZ -w wordlist.txt -noninteractive
```

## Calibration (Auto-filter False Positives)

```bash
# Auto-calibrate filters based on random non-existent paths
ffuf -u http://target.com/FUZZ -w wordlist.txt -ac

# Calibrate with custom strategy
ffuf -u http://target.com/FUZZ -w wordlist.txt -ac -acs advanced

# Manual calibration — run once without wordlist to check baseline response size
ffuf -u http://target.com/FUZZ -w /dev/null -X GET
```

## Recommended Wordlists

```bash
# Web content discovery
/usr/share/seclists/Discovery/Web-Content/common.txt
/usr/share/seclists/Discovery/Web-Content/directory-list-2.3-medium.txt
/usr/share/seclists/Discovery/Web-Content/raft-large-files.txt
/usr/share/seclists/Discovery/Web-Content/raft-large-directories.txt
/usr/share/seclists/Discovery/Web-Content/api/api-endpoints.txt

# Parameters
/usr/share/seclists/Discovery/Web-Content/burp-parameter-names.txt

# Subdomains
/usr/share/seclists/Discovery/DNS/subdomains-top1million-5000.txt
/usr/share/seclists/Discovery/DNS/subdomains-top1million-20000.txt

# Passwords
/usr/share/wordlists/rockyou.txt
/usr/share/seclists/Passwords/Common-Credentials/10k-most-common.txt
```

## Useful Combinations

```bash
# Full directory + file scan with auto-calibration and output
ffuf -u http://target.com/FUZZ \
  -w /usr/share/seclists/Discovery/Web-Content/directory-list-2.3-medium.txt \
  -e .php,.html,.txt,.bak,.zip,.sql \
  -ac -r -t 50 -o ffuf_dir.json -of json

# Authenticated parameter fuzzing through Burp
ffuf -u "http://target.com/api/user?id=FUZZ" \
  -w /usr/share/seclists/Fuzzing/4-digits-0000-9999.txt \
  -H "Cookie: session=abc123" \
  -mc 200 -x http://127.0.0.1:8080

# VHost discovery with noise filter
ffuf -u http://target.com \
  -H "Host: FUZZ.target.com" \
  -w /usr/share/seclists/Discovery/DNS/subdomains-top1million-5000.txt \
  -ac -fs 301 -t 50 -o vhosts.json

# Credential brute force with rate limit
ffuf -u http://target.com/login \
  -X POST -d "username=admin&password=FUZZ" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -w /usr/share/seclists/Passwords/Common-Credentials/10k-most-common.txt \
  -fc 200 -fr "Invalid password" \
  -rate 5 -t 1

# Cluster bomb — two positions (username + password)
ffuf -u http://target.com/login \
  -X POST -d "username=USER&password=PASS" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -w usernames.txt:USER -w passwords.txt:PASS \
  -fc 401 -t 20 -rate 10
```
