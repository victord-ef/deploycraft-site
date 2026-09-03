---
title: "Hash Cracking"
description: "Hashcat and John the Ripper commands for hash identification, dictionary attacks, rule-based cracking, and mask attacks."
icon: "🔓"
weight: 8
count: 35
tags: ["hashcat", "john", "password-cracking", "security"]
---

## Hash Identification

```bash
# hashid
hashid '<hash>'
hashid -m '<hash>'                          # include hashcat mode
hashid -j '<hash>'                          # include john format

# haiti (more accurate)
haiti '<hash>'

# name-that-hash
nth --text '<hash>'
```

## Hashcat — Common Modes

```
-m 0      MD5
-m 100    SHA-1
-m 1400   SHA2-256
-m 1700   SHA2-512
-m 1000   NTLM
-m 3000   LM
-m 5600   NetNTLMv2
-m 5500   NetNTLMv1
-m 2100   DCC2 (Domain Cached Credentials)
-m 13100  Kerberos 5 TGS-REP (Kerberoast)
-m 18200  Kerberos 5 AS-REP (AS-REP Roast)
-m 500    md5crypt ($1$)
-m 1800   sha512crypt ($6$)
-m 3200   bcrypt ($2*$)
-m 400    phpass (WordPress, Joomla)
-m 1500   descrypt (DES)
-m 16500  JWT
```

## Hashcat — Attack Modes

```bash
# Dictionary attack (-a 0)
hashcat -m 1000 hashes.txt /usr/share/wordlists/rockyou.txt

# Dictionary + rules (-a 0 -r)
hashcat -m 1000 hashes.txt rockyou.txt -r /usr/share/hashcat/rules/best64.rule
hashcat -m 1000 hashes.txt rockyou.txt -r OneRuleToRuleThemAll.rule

# Combination attack (-a 1)
hashcat -m 0 hashes.txt wordlist1.txt wordlist2.txt

# Mask attack (-a 3)
hashcat -m 0 hashes.txt '?u?l?l?l?d?d?d?d'   # Upper+lower+4digits
hashcat -m 0 hashes.txt '?a?a?a?a?a?a?a?a'    # 8 any chars

# Hybrid wordlist + mask (-a 6)
hashcat -m 0 hashes.txt rockyou.txt '?d?d?d?d'

# Hybrid mask + wordlist (-a 7)
hashcat -m 0 hashes.txt '?d?d' rockyou.txt
```

## Hashcat — Mask Charsets

```
?l   lowercase (abcdefghijklmnopqrstuvwxyz)
?u   uppercase (ABCDEFGHIJKLMNOPQRSTUVWXYZ)
?d   digits (0123456789)
?s   special (!@#$%^&*...)
?a   all printable (?l?u?d?s)
?b   all bytes (0x00-0xff)
```

## Hashcat — Performance & Options

```bash
hashcat -m 1000 hashes.txt rockyou.txt --status          # show progress
hashcat -m 1000 hashes.txt rockyou.txt --status-timer 10
hashcat -m 1000 hashes.txt rockyou.txt -O                # optimised kernels
hashcat -m 1000 hashes.txt rockyou.txt -w 3              # workload profile
hashcat -m 1000 hashes.txt rockyou.txt --force           # ignore warnings
hashcat -m 1000 hashes.txt rockyou.txt --show            # show cracked
hashcat -m 1000 hashes.txt rockyou.txt --potfile-disable
hashcat --benchmark                                       # GPU benchmark
hashcat --example-hashes                                  # example per mode
```

## John the Ripper

```bash
# Basic usage
john hashes.txt
john hashes.txt --wordlist=/usr/share/wordlists/rockyou.txt
john hashes.txt --format=NT                       # NTLM
john hashes.txt --format=sha512crypt

# Rules
john hashes.txt --wordlist=rockyou.txt --rules=best64
john hashes.txt --wordlist=rockyou.txt --rules=jumbo

# Show cracked
john hashes.txt --show
john hashes.txt --show --format=NT

# Format helpers
john --list=formats
john --list=formats | grep -i ntlm
john --list=rules

# Crack /etc/shadow
unshadow /etc/passwd /etc/shadow > combined.txt
john combined.txt --wordlist=rockyou.txt
```
