---
title: "Burp Suite"
description: "Burp Suite keyboard shortcuts, Proxy, Repeater, Intruder, Scanner, and key techniques for web application penetration testing."
icon: "🕷️"
weight: 15
count: 50
tags: ["burp-suite", "web-app", "pentesting", "security"]
---

{{< callout type="warning" >}}
Burp Suite is for **authorised web application penetration testing, CTF competitions, and security research only**. Use only on applications you have explicit written permission to test.
{{< /callout >}}

## Keyboard Shortcuts

```
Ctrl+R          Send to Repeater
Ctrl+I          Send to Intruder
Ctrl+S          Search
Ctrl+Z          Undo (Repeater/editor)
Ctrl+A          Select all
Ctrl+U          URL encode selection
Ctrl+Shift+U    URL decode selection
Ctrl+B          Base64 encode selection
Ctrl+Shift+B    Base64 decode selection
Ctrl+H          HTML encode selection
Ctrl+F          Forward request (Proxy intercept)
Ctrl+T          New tab
Ctrl+W          Close tab
Ctrl+Space      Autocomplete (Intruder payload positions)
F5              Refresh / resend (Repeater)
```

## Proxy Setup

```
Browser proxy:   127.0.0.1:8080
Burp CA cert:    http://burpsuite (download and install in browser)

# Firefox via FoxyProxy extension (recommended)
Pattern: *        Proxy: 127.0.0.1:8080

# CLI — proxy curl through Burp
curl -x http://127.0.0.1:8080 --proxy-insecure https://target.com

# CLI — proxy with CA cert
curl --cacert ~/burp-ca.crt -x http://127.0.0.1:8080 https://target.com
```

## Proxy — Intercept & Filters

```
Intercept on/off:   Proxy → Intercept → Intercept is on/off
Forward:            Ctrl+F or Forward button
Drop:               Drop button
Action → Send to Repeater / Intruder / Scanner

# Proxy history filters (HTTP history)
Filter by:
  - Status code (200, 301, 404, 500)
  - MIME type (HTML, JSON, script, image)
  - Search term in request/response
  - File extension (exclude: .js, .css, .png, .gif, .woff)

# Scope — limit Proxy to target only
Target → Scope → Add target
Proxy → Options → Intercept → "And URL Is in target scope"
```

## Repeater

```
Send request:      Ctrl+Enter or Send button
Previous request:  < button
Next request:      > button
Toggle pretty print: \n button (JSON/XML/HTML)
Search response:   Ctrl+F in response panel

# Useful Repeater techniques
- Manually test authentication bypass
- Replay and modify tokens, cookies, headers
- Test IDOR by changing IDs in path/body
- Test SQLi/XSS by modifying parameters
- Compare responses between authenticated and unauthenticated requests
```

## Intruder — Attack Types

```
Sniper          One payload set, one position at a time
Battering ram   One payload set, all positions simultaneously
Pitchfork       Multiple payload sets, one per position (parallel)
Cluster bomb    Multiple payload sets, all combinations (Cartesian product)
```

## Intruder — Common Attacks

```
# Brute force login
Position: username=§user§&password=§pass§
Attack type: Cluster bomb
Payload 1: usernames list
Payload 2: passwords list
Grep match: "Invalid" / "Welcome"

# Fuzzing parameters
Position: id=§1§
Attack type: Sniper
Payload: Numbers 1–1000

# Password spray
Position: password=§pass§
Attack type: Sniper
Payload: common passwords list

# Header fuzzing
Add payload position in custom header value
Attack type: Sniper
Payload: SecLists header wordlist
```

## Intruder — Payload Types

```
Simple list         Static wordlist
Runtime file        Load wordlist at runtime
Numbers             Sequential or random numbers
Dates               Date range with format
Brute forcer        Character set + min/max length
Null payloads       Repeat request N times (DoS testing, race conditions)
Username generator  From a name
Bit flipper         Flip bits in base value
```

## Intruder — Grep & Extraction

```
# Grep — Match
Options → Grep — Match → add strings to flag matching responses
e.g.: "Welcome", "dashboard", "admin", "invalid credentials"

# Grep — Extract
Options → Grep — Extract → extract value from response
e.g.: extract CSRF token from response for use in next request

# Analyse results
Sort by: Status code, Length, Response time
Filter by: Grep match columns
```

## Scanner (Pro)

```
# Active scan
Right-click request → Do active scan
Target → Site map → right-click host → Actively scan this host

# Passive scan (always running)
Automatically analyses all proxied traffic

# Scan config presets
Audit checks — light
Audit checks — medium active
Audit checks — all checks
Crawl and audit

# Issue activity
Target → Issue activity → filter by severity (High/Medium/Low/Info)
Dashboard → Issue activity
```

## Target & Site Map

```
# Build site map
Proxy through application → Target → Site map populates automatically
Spider: Target → right-click → Spider this host (legacy)
Crawl: Dashboard → New scan → Crawl

# Scope
Target → Scope → Use advanced scope control
Include: https://target.com/*
Exclude: https://target.com/logout

# Compare site maps
Engagement tools → Compare site maps (Pro)
```

## Decoder

```
Decode:  URL, HTML, Base64, Hex, Octal, Binary, Gzip
Encode:  URL, HTML, Base64, Hex, Octal, Binary
Hash:    MD5, SHA-1, SHA-256, SHA-512

# Smart decode
Paste encoded value → Smart decode → auto-detects encoding

# Chaining
Apply multiple encode/decode operations in sequence
```

## Comparer

```
# Compare two requests or responses
Send items to Comparer from Proxy history, Repeater, etc.
Comparer → select two items → Words / Bytes
Highlights differences between responses

# Use cases
- Compare authenticated vs unauthenticated response
- Compare admin vs standard user response
- Detect subtle differences in blind injection responses
```

## Sequencer (Token Analysis)

```
# Analyse randomness of tokens/session IDs
Proxy history → right-click response with Set-Cookie → Send to Sequencer
Sequencer → Live capture → Start live capture
Collect 10,000+ tokens → Analyse

# Metrics reviewed
Character-level analysis
Bit-level analysis
FIPS tests
```

## Extensions (BApp Store)

```
Logger++            Enhanced request/response logging with filters
Autorize            Automated authorisation testing (IDOR/priv-esc)
Active Scan++       Extended active scan checks
Turbo Intruder      High-speed Intruder replacement (Python scripted)
JWT Editor          Decode, modify, and attack JWTs
CSRF Scanner        Passive CSRF detection
Retire.js           Detect vulnerable JavaScript libraries
JSON Web Tokens     JWT decode/modify/sign
403 Bypasser        Automated 403 bypass attempts
Param Miner         Discover hidden parameters
Hunt                Tag requests by vulnerability class
```

## Common Testing Techniques

```
# CSRF token bypass
- Remove token entirely
- Send blank token
- Use another user's valid token
- Change request method (POST → GET)

# Authentication testing
- Try default credentials
- Check for account enumeration via different error messages
- Test password reset flow for token reuse
- Check remember-me token entropy (Sequencer)

# IDOR
- Change numeric IDs in path, body, headers
- Try GUIDs from other accounts
- Test indirect references (filenames, hashes)
- Check all HTTP methods (GET/POST/PUT/DELETE)

# JWT attacks (with JWT Editor)
- Alg:none attack (remove signature)
- Change RS256 → HS256, sign with public key
- Modify claims (role, sub, exp)
- Crack weak HS256 secret (hashcat -m 16500)

# SSRF
- Inject internal IPs in URL parameters
- Use Burp Collaborator as callback server
- Test: http://169.254.169.254/latest/meta-data/ (AWS)

# Burp Collaborator (Out-of-band)
Project options → Misc → Burp Collaborator client
Use generated payload in blind injection points
Poll for DNS/HTTP interactions
```

## Burp CLI & Headless Scanning

```bash
# Start Burp in headless mode (Pro)
java -jar burpsuite_pro.jar --unpause-spider-and-scanner

# REST API (Pro — v2021.9+)
curl http://localhost:1337/v0.1/scan -d '{"urls":["https://target.com"]}'
curl http://localhost:1337/v0.1/scan/<task-id>

# bambdas (Burp 2023.10+) — custom filter Java lambdas
# Proxy history → filter → Edit filter → Add Bambda
requestResponse.request().url().contains("api")
```
