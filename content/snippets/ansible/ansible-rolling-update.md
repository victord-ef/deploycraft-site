---
title: "Ansible — Zero-Downtime Rolling Update with serial and Health Checks"
date: 2026-11-12
description: "Deploy a new application version to a fleet one host at a time using serial, drain each host from a load balancer before updating, run a post-deploy health check, and abort the entire play on first failure."
lang: "Ansible"
tags: ["ansible", "rolling-update", "serial", "zero-downtime", "deployment", "health-check"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-11-12"
draft: false
---

## When to use this

When deploying to a group of application servers behind a load balancer and downtime is not acceptable. `serial` controls how many hosts Ansible touches simultaneously — the rest keep serving traffic until their turn. A health check gate aborts the entire play before the second batch if the first batch fails.

## Without it

```yaml
# Default Ansible behaviour — all hosts updated simultaneously
- hosts: appservers
  tasks:
    - name: Deploy app
      ...
# All 10 servers go down together. If the deploy is broken, all 10 stay broken.
```

## Snippet

```yaml
---
- name: Rolling application deploy
  hosts: appservers
  serial: 1               # update one host at a time; use "30%" for large fleets
  max_fail_percentage: 0  # abort the play if any host fails (default is to continue)
  become: true

  vars:
    app_version: "{{ version | mandatory }}"   # must be passed with -e version=1.2.3
    app_port: 8080
    health_check_retries: 5
    health_check_delay: 5

  pre_tasks:
    # ── 1. Drain host from load balancer ─────────────────────────────────────
    - name: Remove host from load balancer
      community.general.haproxy:
        state: disabled
        host: "{{ inventory_hostname }}"
        backend: app_backend
        socket: /run/haproxy/admin.sock
        wait: true          # block until active connections drain
        wait_interval: 2
        wait_retries: 30
      delegate_to: "{{ groups['loadbalancers'][0] }}"
      tags: [deploy, lb]

    # ── 2. Wait for in-flight requests to complete ───────────────────────────
    - name: Wait for active connections to drain
      ansible.builtin.wait_for:
        host: "{{ ansible_host }}"
        port: "{{ app_port }}"
        state: drained
        timeout: 60
      tags: [deploy, drain]

  tasks:
    # ── 3. Stop the application ───────────────────────────────────────────────
    - name: Stop application service
      ansible.builtin.service:
        name: myapp
        state: stopped
      tags: [deploy]

    # ── 4. Deploy new version ─────────────────────────────────────────────────
    - name: Download application archive
      ansible.builtin.get_url:
        url: "https://releases.example.com/myapp/{{ app_version }}/myapp-linux-amd64.tar.gz"
        dest: /tmp/myapp-{{ app_version }}.tar.gz
        checksum: "sha256:{{ app_checksum }}"
      tags: [deploy]

    - name: Extract application
      ansible.builtin.unarchive:
        src: /tmp/myapp-{{ app_version }}.tar.gz
        dest: /opt/myapp
        remote_src: true
        owner: appuser
        group: appuser
      tags: [deploy]

    - name: Start application service
      ansible.builtin.service:
        name: myapp
        state: started
      tags: [deploy]

    # ── 5. Health check gate — abort play if this host is unhealthy ───────────
    - name: Wait for application to pass health check
      ansible.builtin.uri:
        url: "http://{{ ansible_host }}:{{ app_port }}/healthz"
        method: GET
        status_code: 200
        return_content: true
      register: health_result
      until: health_result.status == 200
      retries: "{{ health_check_retries }}"
      delay: "{{ health_check_delay }}"
      tags: [deploy, verify]

  post_tasks:
    # ── 6. Re-add host to load balancer only after health check passes ────────
    - name: Add host back to load balancer
      community.general.haproxy:
        state: enabled
        host: "{{ inventory_hostname }}"
        backend: app_backend
        socket: /run/haproxy/admin.sock
      delegate_to: "{{ groups['loadbalancers'][0] }}"
      tags: [deploy, lb]

    - name: Log successful deploy
      ansible.builtin.debug:
        msg: "{{ inventory_hostname }} is now running {{ app_version }} and is back in rotation."
      tags: [deploy]
```

### Inventory structure

```ini
# inventory/hosts.ini
[appservers]
app-01.internal
app-02.internal
app-03.internal

[loadbalancers]
lb-01.internal
```

### Usage

```bash
# Deploy version 1.2.3 — rolls out one host at a time
ansible-playbook deploy.yml -i inventory/ -e version=1.2.3

# Deploy to 30% of the fleet simultaneously
# Change: serial: "30%"
ansible-playbook deploy.yml -i inventory/ -e version=1.2.3

# Dry run — shows which hosts would be updated and in what order
ansible-playbook deploy.yml -i inventory/ -e version=1.2.3 --check

# Deploy only to a specific host
ansible-playbook deploy.yml -i inventory/ -e version=1.2.3 --limit app-01.internal
```

**Key decisions:**

| Decision | Why it matters |
|---|---|
| `serial: 1` | Updates one host at a time. The other hosts continue serving traffic throughout. Use `serial: "30%"` for large fleets where updating one at a time is too slow. |
| `max_fail_percentage: 0` | The default behaviour is to continue the play if a host fails, skipping only that host. `0` aborts the entire play on the first failure — a broken deploy on host 1 never reaches hosts 2–10. |
| `community.general.haproxy` with `wait: true` | Blocks until all active connections to the backend have finished, not just until the backend is marked down. Without `wait: true`, requests mid-flight are dropped when the app is stopped. |
| `delegate_to: lb` for LB operations | The HAProxy commands run on the load balancer host, not the application host — but they execute in the context of each application host's loop iteration. `delegate_to` redirects the task to the right machine. |
| `until` with `retries` on health check | The app needs a few seconds to start. A single immediate check would fail. `until` polls until the condition is met or retries are exhausted, failing the play cleanly if the app never becomes healthy. |
| `app_version: "{{ version | mandatory }}"` | The `mandatory` filter makes Ansible fail immediately with a clear error if `-e version=` is not passed. Without it, the variable is undefined and the error surfaces mid-play in a confusing way. |

## Verify it worked

```bash
# Watch HAProxy stats during the deploy to confirm drain and re-add
watch -n1 "echo 'show stat' | socat stdio /run/haproxy/admin.sock | cut -d, -f1,2,18"

# Check application version on each host after deploy
ansible appservers -i inventory/ -m command -a "/opt/myapp/myapp --version"
```
