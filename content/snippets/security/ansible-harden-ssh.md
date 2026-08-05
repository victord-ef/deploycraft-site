---
title: "Ansible Playbook — Harden SSH Configuration on Linux Hosts"
date: 2026-07-25
description: "Playbook that locks down sshd_config to disable root login, enforce key-based auth, restrict ciphers, and restart SSH safely — idempotent on every run."
lang: "Ansible"
tags: ["ansible", "playbook", "ssh", "hardening", "linux", "security"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-07-25"
draft: false
---

## When to use this

When provisioning new Linux hosts or auditing existing ones to meet a security baseline — CIS Benchmark, SOC 2, or an internal hardening standard. Run it once at build time and on a schedule to catch configuration drift.

## Without it

```ini
# Default /etc/ssh/sshd_config on most distros
PermitRootLogin yes
PasswordAuthentication yes
X11Forwarding yes
# No cipher or MAC restrictions
```

A host with these defaults accepts password brute-force attempts on root, allows X11 forwarding as a lateral movement vector, and negotiates weak legacy ciphers that are vulnerable to downgrade attacks.

## Snippet

### 1. Playbook

```yaml
# playbooks/harden-ssh.yml
- name: Harden SSH configuration
  hosts: all
  become: true
  gather_facts: true

  vars:
    ssh_port: 22
    ssh_allowed_users: []          # leave empty to allow all non-root users
    ssh_banner_path: /etc/ssh/banner

  handlers:
    - name: Restart sshd
      ansible.builtin.service:
        name: "{{ 'ssh' if ansible_os_family == 'Debian' else 'sshd' }}"
        state: restarted

  tasks:
    - name: Set hardened sshd_config options
      ansible.builtin.lineinfile:
        path: /etc/ssh/sshd_config
        regexp: "^#?{{ item.key }}"
        line: "{{ item.key }} {{ item.value }}"
        state: present
        validate: /usr/sbin/sshd -t -f %s
      loop:
        - { key: "PermitRootLogin",          value: "no" }
        - { key: "PasswordAuthentication",   value: "no" }
        - { key: "ChallengeResponseAuthentication", value: "no" }
        - { key: "X11Forwarding",            value: "no" }
        - { key: "AllowAgentForwarding",     value: "no" }
        - { key: "AllowTcpForwarding",       value: "no" }
        - { key: "MaxAuthTries",             value: "3" }
        - { key: "LoginGraceTime",           value: "30" }
        - { key: "ClientAliveInterval",      value: "300" }
        - { key: "ClientAliveCountMax",      value: "2" }
        - { key: "Protocol",                 value: "2" }
        - { key: "Port",                     value: "{{ ssh_port }}" }
        - { key: "Ciphers",                  value: "aes256-gcm@openssh.com,aes128-gcm@openssh.com,aes256-ctr,aes192-ctr,aes128-ctr" }
        - { key: "MACs",                     value: "hmac-sha2-512-etm@openssh.com,hmac-sha2-256-etm@openssh.com,hmac-sha2-512,hmac-sha2-256" }
        - { key: "KexAlgorithms",            value: "curve25519-sha256,curve25519-sha256@libssh.org,diffie-hellman-group14-sha256" }
      notify: Restart sshd

    - name: Restrict SSH access to allowed users (if list is defined)
      ansible.builtin.lineinfile:
        path: /etc/ssh/sshd_config
        regexp: "^#?AllowUsers"
        line: "AllowUsers {{ ssh_allowed_users | join(' ') }}"
        state: present
        validate: /usr/sbin/sshd -t -f %s
      when: ssh_allowed_users | length > 0
      notify: Restart sshd

    - name: Deploy SSH login banner
      ansible.builtin.copy:
        dest: "{{ ssh_banner_path }}"
        content: |
          ********************************************************************
          Authorised access only. All activity is monitored and logged.
          Disconnect immediately if you are not an authorised user.
          ********************************************************************
        mode: "0644"

    - name: Point sshd_config to banner
      ansible.builtin.lineinfile:
        path: /etc/ssh/sshd_config
        regexp: "^#?Banner"
        line: "Banner {{ ssh_banner_path }}"
        validate: /usr/sbin/sshd -t -f %s
      notify: Restart sshd

    - name: Set strict permissions on sshd_config
      ansible.builtin.file:
        path: /etc/ssh/sshd_config
        owner: root
        group: root
        mode: "0600"
```

**Key decisions:**

| Decision | Why it matters |
|---|---|
| `validate: /usr/sbin/sshd -t -f %s` | Runs `sshd -t` (config test) before writing. A bad config that locks you out of SSH is the most dangerous failure mode here. |
| Handler for restart | SSH only restarts if a task actually changed something — avoids unnecessary disconnects on repeat runs. |
| OS-family service name | The daemon is `ssh` on Debian/Ubuntu and `sshd` on RHEL/Rocky/AlmaLinux. The conditional keeps one playbook working across both families. |
| `loop` over `lineinfile` | Each setting is managed independently. If the key already exists it is updated in place; it won't be duplicated on re-runs. |
| Cipher/MAC/Kex restriction | Removes CBC-mode ciphers, MD5/SHA1 MACs, and weak DH groups targeted by BEAST and Lucky13 attacks. |
| `AllowUsers` conditional | Optional — only applied when you explicitly set `ssh_allowed_users`. Avoids accidentally locking out all users if the variable is left empty. |

## Verify it worked

```bash
# Run the playbook
ansible-playbook playbooks/harden-ssh.yml -i inventory/hosts.yml --check --diff
ansible-playbook playbooks/harden-ssh.yml -i inventory/hosts.yml

# Confirm key settings on the remote host
ssh user@host "sudo sshd -T | grep -E 'permitrootlogin|passwordauthentication|x11forwarding|ciphers|macs'"

# Test that root login is rejected
ssh root@host   # should return: Permission denied (publickey)

# Scan with ssh-audit for a full cipher report (optional)
ssh-audit host
```

Expected output from `sshd -T`:

```
permitrootlogin no
passwordauthentication no
x11forwarding no
ciphers aes256-gcm@openssh.com,...
macs hmac-sha2-512-etm@openssh.com,...
```

> **Warning:** Always ensure at least one non-root user has a valid SSH key authorised on the host before running this playbook. Setting `PasswordAuthentication no` with no key in place will lock you out.

## Full walkthrough

Building a full Linux hardening role covering SSH, kernel parameters, auditd, and PAM → **Tutorial Pair 56: Pipeline Security Foundations** *(coming soon)*.
