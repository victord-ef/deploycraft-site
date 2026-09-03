---
title: "John the Ripper"
description: "John the Ripper commands for hash cracking, format detection, wordlist and rule-based attacks, incremental mode, and cracking common file types."
icon: "🔨"
weight: 20
count: 50
tags: ["john", "password-cracking", "pentesting", "security"]
---

{{< callout type="warning" >}}
John the Ripper is for **authorised penetration testing, CTF competitions, and security research only**. Use only on hashes and files you have explicit permission to test.
{{< /callout >}}

## Basic Usage

```bash
john hashes.txt                                        # auto-detect format, default attack
john hashes.txt --wordlist=/usr/share/wordlists/rockyou.txt
john hashes.txt --wordlist=rockyou.txt --format=NT
john hashes.txt --format=sha512crypt
john hashes.txt --show                                 # show cracked passwords
john hashes.txt --show --format=NT
john --list=formats                                    # list all supported formats
john --list=formats | grep -i ntlm
john --list=formats | grep -i sha
```

## Format Detection

```bash
# Auto-detect
john hashes.txt

# Common formats
john hashes.txt --format=NT                           # NTLM (Windows)
john hashes.txt --format=LM                           # LM (legacy Windows)
john hashes.txt --format=mscash2                      # Domain Cached Credentials (DCC2)
john hashes.txt --format=krb5tgs                      # Kerberoast TGS
john hashes.txt --format=krb5asrep                    # AS-REP Roast
john hashes.txt --format=md5crypt                     # md5crypt ($1$)
john hashes.txt --format=sha512crypt                  # sha512crypt ($6$)
john hashes.txt --format=sha256crypt                  # sha256crypt ($5$)
john hashes.txt --format=bcrypt                       # bcrypt ($2a$/$2b$)
john hashes.txt --format=descrypt                     # DES
john hashes.txt --format=md5                          # raw MD5
john hashes.txt --format=sha1                         # raw SHA-1
john hashes.txt --format=sha256                       # raw SHA-256
john hashes.txt --format=sha512                       # raw SHA-512
john hashes.txt --format=Raw-MD5
john hashes.txt --format=Raw-SHA1
john hashes.txt --format=phpass                       # WordPress / Joomla ($P$)
john hashes.txt --format=django                       # Django PBKDF2
john hashes.txt --format=netntlmv2                    # NetNTLMv2 (captured via Responder)
john hashes.txt --format=netntlm                      # NetNTLMv1
```

## Wordlist Attack

```bash
john hashes.txt --wordlist=rockyou.txt
john hashes.txt --wordlist=/usr/share/wordlists/rockyou.txt --format=NT
john hashes.txt --wordlist=rockyou.txt --fork=4        # use 4 CPU cores
```

## Rules

```bash
# Built-in rule sets
john hashes.txt --wordlist=rockyou.txt --rules
john hashes.txt --wordlist=rockyou.txt --rules=best64
john hashes.txt --wordlist=rockyou.txt --rules=jumbo
john hashes.txt --wordlist=rockyou.txt --rules=KoreLogic
john hashes.txt --wordlist=rockyou.txt --rules=OneRuleToRuleThemAll

# List available rule sets
john --list=rules

# Apply rules and print to stdout (for piping)
john hashes.txt --wordlist=rockyou.txt --rules=best64 --stdout | wc -l
john hashes.txt --wordlist=rockyou.txt --rules --stdout > mutated.txt
```

## Incremental (Brute Force) Mode

```bash
john hashes.txt --incremental                          # all printable chars
john hashes.txt --incremental=alpha                    # lowercase only
john hashes.txt --incremental=upper                    # uppercase only
john hashes.txt --incremental=digits                   # digits only
john hashes.txt --incremental=alnum                    # alphanumeric

# List incremental modes
john --list=inc-modes
```

## Single Crack Mode

```bash
# Uses username, GECOS fields, and login info as base words
john hashes.txt --single
john hashes.txt --single --format=sha512crypt
```

## Session Management

```bash
# Name a session (resume later)
john hashes.txt --wordlist=rockyou.txt --session=mysession

# Restore interrupted session
john --restore=mysession
john --restore                                         # restore most recent session

# List active/saved sessions
ls ~/.john/
```

## Performance

```bash
# Use multiple CPU cores
john hashes.txt --wordlist=rockyou.txt --fork=4

# OpenCL (GPU acceleration — jumbo build)
john hashes.txt --wordlist=rockyou.txt --format=NT-opencl
john hashes.txt --wordlist=rockyou.txt --format=sha512crypt-opencl

# Set time limit
john hashes.txt --wordlist=rockyou.txt --max-run-time=3600   # seconds
```

## Cracking System Files

```bash
# Linux /etc/shadow
john /etc/shadow
john /etc/shadow --wordlist=rockyou.txt

# Combine /etc/passwd and /etc/shadow
unshadow /etc/passwd /etc/shadow > combined.txt
john combined.txt --wordlist=rockyou.txt

# Show cracked
john combined.txt --show
```

## Cracking File Types

```bash
# ZIP password
zip2john archive.zip > zip_hash.txt
john zip_hash.txt --wordlist=rockyou.txt

# RAR password
rar2john archive.rar > rar_hash.txt
john rar_hash.txt --wordlist=rockyou.txt

# 7-Zip password
7z2john archive.7z > 7z_hash.txt
john 7z_hash.txt --wordlist=rockyou.txt

# PDF password
pdf2john file.pdf > pdf_hash.txt
john pdf_hash.txt --wordlist=rockyou.txt

# Office documents (Word, Excel)
office2john document.docx > office_hash.txt
john office_hash.txt --wordlist=rockyou.txt

# SSH private key passphrase
ssh2john id_rsa > ssh_hash.txt
john ssh_hash.txt --wordlist=rockyou.txt

# KeePass database
keepass2john database.kdbx > keepass_hash.txt
john keepass_hash.txt --wordlist=rockyou.txt

# Bitcoin wallet
bitcoin2john wallet.dat > wallet_hash.txt
john wallet_hash.txt --wordlist=rockyou.txt

# PGP/GPG key
gpg2john key.gpg > gpg_hash.txt
john gpg_hash.txt --wordlist=rockyou.txt
```

## Kerberos & Active Directory

```bash
# Kerberoast — TGS tickets (from Impacket GetUserSPNs)
john spn_hashes.txt --format=krb5tgs --wordlist=rockyou.txt
john spn_hashes.txt --format=krb5tgs --wordlist=rockyou.txt --rules=best64

# AS-REP Roast (from Impacket GetNPUsers)
john asrep_hashes.txt --format=krb5asrep --wordlist=rockyou.txt

# NetNTLMv2 (captured via Responder)
john netntlmv2_hashes.txt --format=netntlmv2 --wordlist=rockyou.txt

# NTLM hashes (from secretsdump)
john ntlm_hashes.txt --format=NT --wordlist=rockyou.txt
john ntlm_hashes.txt --format=NT --wordlist=rockyou.txt --rules=best64

# Domain Cached Credentials (DCC2 / mscash2)
john dcc2_hashes.txt --format=mscash2 --wordlist=rockyou.txt
```

## Useful Combinations

```bash
# Full attack chain — wordlist + rules + incremental fallback
john hashes.txt --wordlist=rockyou.txt --rules=best64 --format=NT
john hashes.txt --incremental=alnum --format=NT       # if wordlist fails

# Crack SSH key then show result
ssh2john id_rsa > ssh.hash && john ssh.hash --wordlist=rockyou.txt
john ssh.hash --show

# Crack ZIP, show password
zip2john secret.zip > zip.hash && john zip.hash --wordlist=rockyou.txt && john zip.hash --show

# Multi-core Kerberoast with rules
john spn_hashes.txt --format=krb5tgs --wordlist=rockyou.txt --rules=jumbo --fork=4
```
