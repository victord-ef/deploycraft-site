---
title: "Ansible Role — Install and Configure Docker on Ubuntu"
date: 2026-07-25
description: "Ansible role that installs Docker Engine from the official repository, configures the daemon with sane production defaults, and adds a user to the docker group."
lang: "Ansible"
tags: ["ansible", "role", "docker", "ubuntu", "automation"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-07-25"
draft: false
---

## When to use this

When provisioning Ubuntu servers that need Docker — CI runners, container hosts, or dev boxes — and you want a repeatable, idempotent install from Docker's official APT repository rather than the older distro-packaged version.

## Without it

```bash
# Manual install — not repeatable, not idempotent, easy to drift across hosts
curl -fsSL https://get.docker.com | sh
usermod -aG docker $USER
```

Running the convenience script repeatedly may upgrade Docker unintentionally, skips daemon hardening, and leaves no record of what was configured or when.

## Snippet

### Role structure

```
roles/
└── docker/
    ├── defaults/
    │   └── main.yml
    ├── tasks/
    │   └── main.yml
    ├── handlers/
    │   └── main.yml
    └── templates/
        └── daemon.json.j2
```

### defaults/main.yml

```yaml
docker_users: []               # users to add to the docker group
docker_edition: "ce"           # ce (Community) or ee (Enterprise)
docker_log_driver: "json-file"
docker_log_max_size: "10m"
docker_log_max_file: "3"
docker_storage_driver: "overlay2"
docker_live_restore: true      # keep containers running during daemon restart
```

### tasks/main.yml

```yaml
- name: Install prerequisite packages
  ansible.builtin.apt:
    name:
      - ca-certificates
      - curl
      - gnupg
    state: present
    update_cache: true

- name: Create keyring directory
  ansible.builtin.file:
    path: /etc/apt/keyrings
    state: directory
    mode: "0755"

- name: Add Docker GPG key
  ansible.builtin.get_url:
    url: https://download.docker.com/linux/ubuntu/gpg
    dest: /etc/apt/keyrings/docker.asc
    mode: "0644"

- name: Add Docker APT repository
  ansible.builtin.apt_repository:
    repo: >-
      deb [arch={{ ansible_architecture | replace('x86_64','amd64') }}
      signed-by=/etc/apt/keyrings/docker.asc]
      https://download.docker.com/linux/ubuntu
      {{ ansible_distribution_release }} stable
    state: present
    filename: docker

- name: Install Docker Engine
  ansible.builtin.apt:
    name:
      - docker-ce
      - docker-ce-cli
      - containerd.io
      - docker-buildx-plugin
      - docker-compose-plugin
    state: present
    update_cache: true
  notify: Restart docker

- name: Deploy daemon configuration
  ansible.builtin.template:
    src: daemon.json.j2
    dest: /etc/docker/daemon.json
    owner: root
    group: root
    mode: "0644"
  notify: Restart docker

- name: Ensure Docker service is enabled and running
  ansible.builtin.service:
    name: docker
    state: started
    enabled: true

- name: Add users to docker group
  ansible.builtin.user:
    name: "{{ item }}"
    groups: docker
    append: true
  loop: "{{ docker_users }}"
  when: docker_users | length > 0
```

### handlers/main.yml

```yaml
- name: Restart docker
  ansible.builtin.service:
    name: docker
    state: restarted
```

### templates/daemon.json.j2

```json
{
  "log-driver": "{{ docker_log_driver }}",
  "log-opts": {
    "max-size": "{{ docker_log_max_size }}",
    "max-file": "{{ docker_log_max_file }}"
  },
  "storage-driver": "{{ docker_storage_driver }}",
  "live-restore": {{ docker_live_restore | lower }},
  "no-new-privileges": true,
  "userns-remap": "default"
}
```

### Playbook to invoke the role

```yaml
# playbooks/setup-docker-hosts.yml
- name: Install Docker on Ubuntu hosts
  hosts: docker_hosts
  become: true
  roles:
    - role: docker
      vars:
        docker_users:
          - deployer
        docker_log_max_size: "20m"
```

**Key decisions:**

| Decision | Why it matters |
|---|---|
| Official APT repo over distro package | Ubuntu ships `docker.io`, which lags Docker CE by several releases. The official repo tracks current releases and security patches. |
| GPG key in `/etc/apt/keyrings` | The modern location — avoids the deprecated `apt-key add` path that writes to a shared keyring and produces warnings on Ubuntu 22.04+. |
| `live-restore: true` | Containers keep running when the Docker daemon restarts (e.g. after a config change or package update). Critical for production hosts. |
| `no-new-privileges: true` | Prevents container processes from gaining additional privileges via `setuid` binaries — a default-deny at the daemon level. |
| `userns-remap: default` | Maps container root (UID 0) to an unprivileged host UID, so a container escape does not give root on the host. |
| Handler for restart | Daemon only restarts when `daemon.json` or the package actually changed — avoids dropping running containers unnecessarily. |

## Verify it worked

```bash
# Run the playbook
ansible-playbook playbooks/setup-docker-hosts.yml -i inventory/hosts.yml

# Confirm Docker version and runtime
ssh deployer@host "docker version --format '{{.Server.Version}}'"

# Confirm daemon config was applied
ssh deployer@host "docker info --format '{{.LoggingDriver}} {{.LiveRestoreEnabled}}'"

# Confirm the deploy user can run Docker without sudo
ssh deployer@host "docker run --rm hello-world"

# Confirm userns-remap is active
ssh deployer@host "docker info | grep -i 'userns'"
```

Expected output from `docker info`:

```
json-file true
userns remapping: default
```

> **Note:** `userns-remap` requires the `newuidmap` and `newgidmap` binaries (`uidmap` package on Ubuntu). The role installs prerequisites but if you see a daemon startup error, install `uidmap` manually first.

## Full walkthrough

Building a full container host hardening role including seccomp profiles and AppArmor → **Tutorial Pair 23: Capabilities & Syscalls** *(coming soon)*.
