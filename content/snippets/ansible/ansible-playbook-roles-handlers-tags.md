---
title: "Ansible Playbook — Roles, Handlers, Tags, and Conditionals"
date: 2026-11-12
description: "A complete playbook showing the canonical structure: roles for reusable task grouping, handlers for service restarts triggered by change, tags for selective execution, and when conditionals for OS branching."
lang: "Ansible"
tags: ["ansible", "playbook", "roles", "handlers", "tags", "conditionals", "structure"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-11-12"
draft: false
---

## When to use this

As a reference layout when writing a playbook from scratch. The structure here scales from a single server to hundreds — roles keep tasks reusable, handlers prevent unnecessary service restarts, tags let you run a subset of tasks without editing the file, and `when` conditionals handle mixed OS environments.

## Snippet

### Directory layout

```
site.yml                  # top-level playbook
group_vars/
  all.yml                 # variables for all hosts
  webservers.yml          # variables for the webservers group
roles/
  nginx/
    tasks/
      main.yml
    handlers/
      main.yml
    templates/
      nginx.conf.j2
    defaults/
      main.yml            # low-precedence defaults, always overridable
    vars/
      main.yml            # high-precedence role vars, rarely overridden
  app/
    tasks/
      main.yml
    handlers/
      main.yml
```

### site.yml — top-level playbook

```yaml
---
- name: Configure web tier
  hosts: webservers
  become: true

  pre_tasks:
    - name: Update apt cache (Debian/Ubuntu only)
      ansible.builtin.apt:
        update_cache: true
        cache_valid_time: 3600
      when: ansible_facts['os_family'] == 'Debian'
      tags: [always]            # 'always' tag runs regardless of --tags filter

  roles:
    - role: nginx
      tags: [nginx, web]
    - role: app
      tags: [app]

  post_tasks:
    - name: Confirm nginx is responding
      ansible.builtin.uri:
        url: "http://{{ ansible_host }}/healthz"
        status_code: 200
      tags: [nginx, verify]
```

### roles/nginx/tasks/main.yml

```yaml
---
- name: Install nginx
  ansible.builtin.package:
    name: nginx
    state: present
  tags: [nginx, install]

- name: Deploy nginx configuration
  ansible.builtin.template:
    src: nginx.conf.j2
    dest: /etc/nginx/nginx.conf
    owner: root
    group: root
    mode: "0644"
    validate: "nginx -t -c %s"   # validate before writing
  notify: Reload nginx            # triggers handler only when this task changes
  tags: [nginx, config]

- name: Enable and start nginx
  ansible.builtin.service:
    name: nginx
    state: started
    enabled: true
  tags: [nginx]

- name: Open HTTP port (firewalld)
  ansible.posix.firewalld:
    service: http
    permanent: true
    state: enabled
    immediate: true
  when: ansible_facts['service_mgr'] == 'systemd' and
        'firewalld' in ansible_facts.get('packages', {})
  tags: [nginx, firewall]
```

### roles/nginx/handlers/main.yml

```yaml
---
# Handlers run once at the end of a play, even if notified multiple times.
# A config change and a cert change both notify this handler — nginx reloads once.
- name: Reload nginx
  ansible.builtin.service:
    name: nginx
    state: reloaded

- name: Restart nginx
  ansible.builtin.service:
    name: nginx
    state: restarted
```

### roles/nginx/defaults/main.yml

```yaml
---
nginx_worker_processes: auto
nginx_worker_connections: 1024
nginx_server_name: "{{ inventory_hostname }}"
nginx_root: /var/www/html
```

### group_vars/webservers.yml

```yaml
---
# Overrides role defaults for the webservers group
nginx_worker_connections: 2048
app_port: 8080
```

### Usage

```bash
# Full run
ansible-playbook site.yml -i inventory/

# Run only nginx tasks
ansible-playbook site.yml -i inventory/ --tags nginx

# Run config update only, skip install
ansible-playbook site.yml -i inventory/ --tags nginx,config --skip-tags install

# Dry run with diff to preview config changes
ansible-playbook site.yml -i inventory/ --check --diff --tags nginx,config

# Limit to one host
ansible-playbook site.yml -i inventory/ --limit web-prod-01
```

**Key decisions:**

| Decision | Why it matters |
|---|---|
| `tags: [always]` on apt cache update | Runs the cache update even when `--tags nginx` is passed. Without `always`, selective tag runs skip pre_tasks entirely and may fail on a stale cache. |
| `notify` on template task, not install | The handler triggers only when the config file changes — not on every run. If the template output is identical to the existing file, Ansible reports no change and the handler is never queued. |
| `validate:` on template module | Runs `nginx -t` against the rendered file before replacing the live config. A broken config never reaches disk. |
| `defaults/` over `vars/` for role variables | `defaults/` has the lowest variable precedence — `group_vars`, `host_vars`, and `-e` flags all override it. `vars/` has high precedence and cannot be overridden from inventory. Use `defaults/` for anything a caller might reasonably customise. |
| `ansible_facts['os_family']` not `ansible_os_family` | The `ansible_facts` dict form works consistently regardless of `inject_facts_as_vars` configuration. Both work in default setups, but the dict form is explicit and immune to namespace collisions. |
| Handlers run once per play | If ten tasks all `notify: Reload nginx`, the handler still runs once at the end. Reloading nginx ten times would cause unnecessary downtime on a busy server. |

## Verify it worked

```bash
# Syntax check before running
ansible-playbook site.yml --syntax-check

# List all tasks without running them
ansible-playbook site.yml --list-tasks

# List which hosts would be affected
ansible-playbook site.yml --list-hosts
```
