---
title: "Let's Encrypt — Automated Certificate Renewal with Certbot"
date: 2026-08-05
description: "Automate Let's Encrypt certificate renewal with Certbot — systemd timers, cron, pre/post hooks for nginx reload, and renewal validation."
lang: "Shell"
tags: ["tls", "certificates", "lets-encrypt", "certbot", "nginx", "automation", "security"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-08-05"
draft: false
---

## When to use this

Let's Encrypt certificates expire after 90 days. Certbot installs a renewal mechanism automatically, but you need to confirm it is working and add reload hooks so your web server picks up the new certificate without downtime.

---

## How Certbot renewal works

Certbot installs either a **systemd timer** (modern distros) or a **cron job** that runs `certbot renew` twice daily. It only renews certificates within 30 days of expiry, so the twice-daily schedule is safe and has no rate-limit impact.

```bash
# Check which mechanism is installed
systemctl list-timers | grep certbot      # systemd
cat /etc/cron.d/certbot                   # cron fallback
```

---

## Snippet

### 1. Test renewal without making changes

```bash
sudo certbot renew --dry-run
```

Expected output:

```
Simulating renewal of an existing certificate for example.com

Congratulations, all simulated renewals succeeded
```

If this fails, investigate before the cert actually expires.

---

### 2. Add a deploy hook to reload nginx after renewal

Certbot runs scripts in `/etc/letsencrypt/renewal-hooks/deploy/` after a successful renewal.

```bash
sudo tee /etc/letsencrypt/renewal-hooks/deploy/reload-nginx.sh > /dev/null <<'EOF'
#!/bin/bash
systemctl reload nginx
EOF

sudo chmod +x /etc/letsencrypt/renewal-hooks/deploy/reload-nginx.sh
```

For Apache:

```bash
sudo tee /etc/letsencrypt/renewal-hooks/deploy/reload-apache.sh > /dev/null <<'EOF'
#!/bin/bash
systemctl reload apache2   # Debian/Ubuntu
# systemctl reload httpd   # RHEL/Rocky
EOF

sudo chmod +x /etc/letsencrypt/renewal-hooks/deploy/reload-apache.sh
```

---

### 3. Verify the systemd timer is active

```bash
sudo systemctl status certbot.timer
sudo systemctl status certbot.service

# Force an immediate renewal attempt (respects 30-day threshold)
sudo systemctl start certbot.service

# View renewal logs
sudo journalctl -u certbot.service --since "7 days ago"
```

---

### 4. Manual renewal (force, ignoring 30-day threshold)

```bash
# Renew all certificates immediately regardless of expiry
sudo certbot renew --force-renewal

# Renew a specific certificate
sudo certbot renew --cert-name example.com --force-renewal
```

---

### 5. Cron-based renewal (if not using systemd)

```bash
# Add to /etc/cron.d/certbot or crontab -e
# Runs at 03:00 and 15:00 daily — offset to avoid peak times
0 3,15 * * * root certbot renew --quiet --deploy-hook "systemctl reload nginx"
```

---

### 6. Check certificate expiry dates

```bash
# All managed certificates
sudo certbot certificates

# Direct check via openssl
echo | openssl s_client -connect example.com:443 -servername example.com 2>/dev/null \
  | openssl x509 -noout -enddate

# Returns:
# notAfter=Sep  3 12:00:00 2026 GMT
```

Set a monitoring alert (e.g. Prometheus `ssl_certificate_expiry_seconds`, or UptimeRobot SSL check) so you get notified if a cert nears expiry unexpectedly.

---

### 7. Renewal config file

Each cert has a config at `/etc/letsencrypt/renewal/example.com.conf`. Inspect it if renewal fails:

```bash
cat /etc/letsencrypt/renewal/example.com.conf
```

Key fields:

```ini
[renewalparams]
authenticator = nginx        # or standalone, dns-cloudflare, etc.
installer = nginx
server = https://acme-v02.api.letsencrypt.org/directory
```

If the authenticator changed (e.g. you switched from standalone to nginx), update this file to match.

---

## Gotchas

| Issue | Fix |
|---|---|
| nginx not reloading after renewal | Add deploy hook — certbot does not reload nginx automatically |
| `certbot renew` fails silently | Check `/var/log/letsencrypt/letsencrypt.log` for the actual error |
| Renewal uses wrong challenge mode | Edit `/etc/letsencrypt/renewal/<domain>.conf` and change `authenticator` |
| Timer not running | `sudo systemctl enable --now certbot.timer` |
| Wildcard not renewing automatically | Manual DNS challenge cannot auto-renew — use a DNS plugin (Cloudflare, Route53) |
