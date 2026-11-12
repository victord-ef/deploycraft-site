---
title: "Ansible — Jinja2 Config File Templating"
date: 2026-11-12
description: "Use the template module with Jinja2 to render nginx, application, and systemd unit config files from variables — with loops, conditionals, and filters that handle the cases static files cannot."
lang: "Ansible"
tags: ["ansible", "jinja2", "template", "config", "nginx", "systemd"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-11-12"
draft: false
---

## When to use this

When a config file differs between environments (staging vs production), between host groups (different worker counts, ports, upstream addresses), or when it must embed values only known at runtime (IP addresses, hostnames, inventory variables). Static files copied with `copy:` cannot handle any of these — `template:` with Jinja2 can.

## Snippet

### Task — render and deploy a template

```yaml
- name: Deploy nginx configuration
  ansible.builtin.template:
    src: templates/nginx.conf.j2       # relative to the role or playbook directory
    dest: /etc/nginx/nginx.conf
    owner: root
    group: root
    mode: "0644"
    validate: "nginx -t -c %s"         # validate the rendered file before writing
    backup: true                       # keep a timestamped backup of the previous version
  notify: Reload nginx
```

### nginx.conf.j2 — conditionals, loops, and filters

```jinja2
# Managed by Ansible — do not edit manually
# Rendered for {{ inventory_hostname }} on {{ ansible_date_time.iso8601 }}

user  {{ nginx_user | default('www-data') }};
worker_processes  {{ nginx_worker_processes | default('auto') }};

error_log  /var/log/nginx/error.log {{ nginx_log_level | default('warn') }};
pid        /run/nginx.pid;

events {
    worker_connections  {{ nginx_worker_connections | default(1024) }};
}

http {
    include       mime.types;
    default_type  application/octet-stream;

    sendfile    on;
    tcp_nopush  on;
    keepalive_timeout  65;

    # Gzip — enabled only in production to avoid masking compression bugs in staging
    {% if env == 'production' %}
    gzip on;
    gzip_comp_level 5;
    gzip_types text/plain text/css application/json application/javascript;
    {% endif %}

    # Upstream block — generated from the app_servers list variable
    upstream app {
        {% for host in groups['appservers'] %}
        server {{ hostvars[host]['ansible_host'] }}:{{ app_port }};
        {% endfor %}
    }

    server {
        listen 80;
        server_name {{ nginx_server_name | default(inventory_hostname) }};

        # Redirect HTTP to HTTPS in production only
        {% if env == 'production' %}
        return 301 https://$host$request_uri;
        {% else %}
        location / {
            proxy_pass http://app;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
        }
        {% endif %}
    }

    {% if env == 'production' %}
    server {
        listen 443 ssl;
        server_name {{ nginx_server_name }};

        ssl_certificate     /etc/ssl/certs/{{ nginx_server_name }}.crt;
        ssl_certificate_key /etc/ssl/private/{{ nginx_server_name }}.key;
        ssl_protocols       TLSv1.2 TLSv1.3;
        ssl_ciphers         ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256;

        location / {
            proxy_pass http://app;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-Proto $scheme;
        }
    }
    {% endif %}
}
```

### systemd unit template — app.service.j2

```jinja2
[Unit]
Description={{ app_description | default(app_name) }}
After=network.target
{% if 'postgresql' in ansible_facts.get('packages', {}) %}
After=postgresql.service
Requires=postgresql.service
{% endif %}

[Service]
Type=simple
User={{ app_user }}
Group={{ app_group }}
WorkingDirectory={{ app_install_dir }}
ExecStart={{ app_install_dir }}/{{ app_binary }} \
    --port {{ app_port }} \
    --config {{ app_config_dir }}/config.yaml
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure
RestartSec=5

# Environment variables — rendered from the app_env dict variable
{% for key, value in app_env.items() | sort %}
Environment="{{ key }}={{ value }}"
{% endfor %}

# Resource limits
LimitNOFILE={{ app_fd_limit | default(65536) }}
{% if app_memory_limit is defined %}
MemoryMax={{ app_memory_limit }}
{% endif %}

[Install]
WantedBy=multi-user.target
```

### Variables driving the templates

```yaml
# group_vars/webservers.yml
env: production
nginx_worker_connections: 2048
nginx_server_name: "app.example.com"
app_port: 8080

# group_vars/staging.yml
env: staging
nginx_worker_connections: 512
nginx_server_name: "app.staging.example.com"
app_port: 8080

# host_vars/app-01.internal.yml
app_env:
  DATABASE_URL: "postgres://app:{{ db_password }}@db.internal:5432/appdb"
  LOG_LEVEL: "info"
  FEATURE_FLAGS: "new_dashboard=true"
app_memory_limit: "512M"
```

### Useful Jinja2 filters for config files

```jinja2
{# Default value if variable is undefined or empty #}
{{ nginx_user | default('www-data') }}

{# Fail the play with a message if variable is not defined #}
{{ db_password | mandatory("db_password must be set in vault") }}

{# Convert a list to a comma-separated string #}
{{ allowed_ips | join(', ') }}

{# Quote a value safely for shell use #}
{{ app_secret | quote }}

{# Base64 encode a value (e.g. for a Kubernetes secret) #}
{{ db_password | b64encode }}

{# Convert dict to JSON inline #}
{{ feature_flags | to_json }}

{# Trim whitespace from a variable before inserting #}
{{ app_name | trim }}

{# Conditional with inline default — avoids undefined errors #}
{{ (app_debug | default(false)) | ternary('debug', 'info') }}
```

**Key decisions:**

| Decision | Why it matters |
|---|---|
| `validate:` on template task | Passes the rendered file to the validation command before overwriting the live config. A bad nginx config never lands on disk — the task fails and the handler is never triggered. |
| `backup: true` | Keeps the previous config as `/etc/nginx/nginx.conf.2026-11-12@14:32:01~`. Essential for rollback when the new config causes unexpected behaviour after deploy. |
| `hostvars[host]['ansible_host']` in upstream loop | `hostvars` gives access to variables from other hosts in the inventory. This builds a complete upstream block from the actual IP addresses of the app servers without hard-coding them in the template. |
| `| sort` on `app_env.items()` | Dicts in Python 3.7+ maintain insertion order, but the order can vary between plays if the dict is built from multiple sources. Sorting produces a stable file on every run, keeping diffs clean. |
| `| mandatory` filter | Explicitly fails the play with a human-readable message if a required variable is missing. Without it, `{{ db_password }}` renders as an empty string and the config silently connects with no password. |
| Environment variable block from dict | Maintaining `app_env` as a dict means adding an env var is a one-line change in `host_vars` or `group_vars` — no Jinja2 template changes needed. |

## Verify it worked

```bash
# Render the template locally without connecting to any host
ansible all -i inventory/ -m template \
  -a "src=templates/nginx.conf.j2 dest=/tmp/nginx-preview.conf" \
  --check --diff

# Check the rendered file on the target host
ansible webservers -i inventory/ -m command \
  -a "cat /etc/nginx/nginx.conf"

# Confirm nginx accepted the config
ansible webservers -i inventory/ -m command \
  -a "nginx -t"
```
