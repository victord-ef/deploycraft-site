---
title: "Let's Encrypt — Issue TLS Certificates with Certbot"
date: 2026-08-05
description: "Issue free TLS certificates from Let's Encrypt using Certbot — standalone, webroot, nginx/Apache plugin, and wildcard DNS challenge modes."
lang: "Shell"
tags: ["tls", "certificates", "lets-encrypt", "certbot", "nginx", "security"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-08-05"
draft: false
---

## When to use this

When you need a free, trusted TLS certificate for a domain you control. Four modes covered:

- **Standalone** — no web server running yet
- **Nginx / Apache plugin** — web server keeps running, Certbot configures it automatically
- **Webroot** — web server running, you manage config yourself
- **Wildcard** — `*.example.com` via DNS challenge

## Prerequisites

```bash
# Debian/Ubuntu
sudo apt update && sudo apt install -y certbot python3-certbot-nginx python3-certbot-apache

# RHEL / Rocky / AlmaLinux
sudo dnf install -y certbot python3-certbot-nginx python3-certbot-apache

certbot --version
```

Port 80 must be reachable from the internet for HTTP-01 challenges. Port 443 for HTTPS after issuance.

---

## Option 1 — Standalone

Certbot spins up a temporary HTTP server on port 80. Stop your web server first.

```bash
sudo systemctl stop nginx   # or apache2

sudo certbot certonly \
  --standalone \
  --preferred-challenges http \
  --email admin@example.com \
  --agree-tos \
  --no-eff-email \
  -d example.com \
  -d www.example.com
```

Certificates written to:

```
/etc/letsencrypt/live/example.com/fullchain.pem   # cert + intermediates
/etc/letsencrypt/live/example.com/privkey.pem      # private key
```

---

## Option 2 — Nginx plugin

Certbot issues the cert and updates nginx config automatically.

```bash
sudo certbot --nginx \
  --email admin@example.com \
  --agree-tos \
  --no-eff-email \
  --redirect \
  -d example.com \
  -d www.example.com
```

---

## Option 3 — Apache plugin

```bash
sudo certbot --apache \
  --email admin@example.com \
  --agree-tos \
  --no-eff-email \
  --redirect \
  -d example.com \
  -d www.example.com
```

---

## Option 4 — Wildcard (DNS challenge)

Required for `*.example.com`. Certbot pauses and asks you to add a DNS TXT record.

```bash
sudo certbot certonly \
  --manual \
  --preferred-challenges dns \
  --email admin@example.com \
  --agree-tos \
  --no-eff-email \
  -d example.com \
  -d "*.example.com"
```

Certbot shows the TXT record value — add it at your DNS provider, wait for propagation, then press Enter.

### Automated wildcard with Cloudflare DNS plugin

```bash
pip install certbot-dns-cloudflare

cat > /etc/letsencrypt/cloudflare.ini <<EOF
dns_cloudflare_api_token = YOUR_CF_API_TOKEN
EOF
chmod 600 /etc/letsencrypt/cloudflare.ini

sudo certbot certonly \
  --dns-cloudflare \
  --dns-cloudflare-credentials /etc/letsencrypt/cloudflare.ini \
  --email admin@example.com \
  --agree-tos \
  --no-eff-email \
  -d example.com \
  -d "*.example.com"
```

---

## Manual nginx config (without plugin)

```nginx
# /etc/nginx/sites-available/example.com
server {
    listen 80;
    server_name example.com www.example.com;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl;
    server_name example.com www.example.com;

    ssl_certificate     /etc/letsencrypt/live/example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/example.com/privkey.pem;

    ssl_protocols       TLSv1.2 TLSv1.3;
    ssl_ciphers         ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384;
    ssl_prefer_server_ciphers off;
    ssl_session_cache   shared:SSL:10m;
    ssl_session_timeout 1d;

    add_header Strict-Transport-Security "max-age=63072000; includeSubDomains" always;

    location / {
        proxy_pass http://127.0.0.1:3000;
    }
}
```

---

## Verify

```bash
# List issued certificates
sudo certbot certificates

# Inspect expiry and issuer
echo | openssl s_client -connect example.com:443 -servername example.com 2>/dev/null \
  | openssl x509 -noout -dates -subject -issuer

# Expected
# notAfter=...  (~90 days from issuance)
# issuer=C=US, O=Let's Encrypt, CN=R11
```

---

## Gotchas

| Issue | Fix |
|---|---|
| Port 80 blocked | Open port 80 in your firewall before running standalone or webroot |
| Rate limit hit | Use `--staging` flag while testing — 5 duplicate certs/week limit in production |
| Browser shows untrusted cert | Use `fullchain.pem`, not `cert.pem` — chain includes intermediates |
| Wildcard not matching sub-subdomains | `*.example.com` covers `app.example.com`, not `a.b.example.com` |

### Staging flag (avoid rate limits during testing)

```bash
sudo certbot certonly --standalone --staging \
  -d example.com -d www.example.com \
  --email admin@example.com --agree-tos --no-eff-email
```

Remove `--staging` once confirmed working.
