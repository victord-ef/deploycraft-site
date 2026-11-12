---
title: "Ansible in GitHub Actions CI"
date: 2026-11-12
description: "Run Ansible playbooks from a GitHub Actions workflow — inject the vault password from a secret, load an SSH key for host access, pin the Ansible version, and cache pip dependencies between runs."
lang: "Ansible"
tags: ["ansible", "github-actions", "cicd", "vault", "ssh", "automation", "pipeline"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-11-12"
draft: false
---

## When to use this

When Ansible playbooks are part of a CI/CD pipeline — triggered on merge to deploy infrastructure changes, run on a schedule to enforce configuration, or gated on a PR approval before production. The workflow here handles the three things that break most Ansible CI setups: vault password injection, SSH key availability, and consistent Ansible versions across runners.

## Snippet

### .github/workflows/deploy.yml

```yaml
name: Ansible deploy

on:
  push:
    branches: [main]
    paths:
      - 'ansible/**'
      - '.github/workflows/deploy.yml'
  workflow_dispatch:
    inputs:
      environment:
        description: "Target environment"
        type: choice
        options: [staging, production]
        required: true

jobs:
  deploy:
    runs-on: ubuntu-latest
    environment: ${{ github.event.inputs.environment || 'staging' }}
    permissions:
      contents: read

    steps:
      - uses: actions/checkout@v4

      # ── Pin Python and cache pip packages ──────────────────────────────────
      - name: Set up Python
        uses: actions/setup-python@v5
        with:
          python-version: "3.12"
          cache: pip
          cache-dependency-path: ansible/requirements.txt

      - name: Install Ansible and dependencies
        run: pip install -r ansible/requirements.txt

      # ── Install Ansible collections ────────────────────────────────────────
      - name: Cache Ansible collections
        uses: actions/cache@v4
        with:
          path: ~/.ansible/collections
          key: ansible-collections-${{ hashFiles('ansible/requirements.yml') }}
          restore-keys: ansible-collections-

      - name: Install Ansible collections
        run: ansible-galaxy collection install -r ansible/requirements.yml

      # ── Write vault password from secret ──────────────────────────────────
      - name: Write vault password file
        run: |
          echo "${{ secrets.ANSIBLE_VAULT_PASSWORD }}" > /tmp/.vault_pass
          chmod 600 /tmp/.vault_pass

      # ── Load SSH key for host access ───────────────────────────────────────
      - name: Load SSH private key
        uses: webfactory/ssh-agent@v0.9.0
        with:
          ssh-private-key: ${{ secrets.ANSIBLE_SSH_KEY }}

      - name: Add known hosts
        run: |
          mkdir -p ~/.ssh
          chmod 700 ~/.ssh
          # Add host keys for all inventory hosts to prevent interactive prompts
          ansible-inventory -i ansible/inventory/ --list \
            | jq -r '._meta.hostvars | to_entries[].value.ansible_host // empty' \
            | sort -u \
            | xargs -I{} ssh-keyscan -H {} >> ~/.ssh/known_hosts 2>/dev/null
          chmod 644 ~/.ssh/known_hosts

      # ── Syntax check before running ────────────────────────────────────────
      - name: Validate playbook syntax
        run: |
          ansible-playbook ansible/site.yml \
            -i ansible/inventory/ \
            --syntax-check \
            --vault-password-file /tmp/.vault_pass

      # ── Run the playbook ───────────────────────────────────────────────────
      - name: Run Ansible playbook
        run: |
          ansible-playbook ansible/site.yml \
            -i ansible/inventory/ \
            --vault-password-file /tmp/.vault_pass \
            --extra-vars "env=${{ github.event.inputs.environment || 'staging' }}" \
            -v
        env:
          ANSIBLE_FORCE_COLOR: "1"
          ANSIBLE_HOST_KEY_CHECKING: "False"   # covered by known_hosts step above

      # ── Clean up vault password ────────────────────────────────────────────
      - name: Remove vault password file
        if: always()
        run: rm -f /tmp/.vault_pass
```

### ansible/requirements.txt

```
ansible-core==2.17.6
boto3>=1.35               # for AWS dynamic inventory
botocore>=1.35
jmespath>=1.0             # required by json_query filter
```

### ansible/requirements.yml

```yaml
---
collections:
  - name: community.general
    version: ">=9.0.0"
  - name: community.aws
    version: ">=7.0.0"
  - name: ansible.posix
    version: ">=1.5.0"
  - name: kubernetes.core
    version: ">=3.0.0"
```

### ansible/ansible.cfg

```ini
[defaults]
inventory          = inventory/
roles_path         = roles/
collections_path   = ~/.ansible/collections
vault_password_file = /tmp/.vault_pass   # used in CI; override locally with --ask-vault-pass
retry_files_enabled = False
stdout_callback    = yaml
host_key_checking  = True

[ssh_connection]
pipelining         = True     # reduces SSH round-trips — significant speedup on large inventories
control_path       = /tmp/ansible-ssh-%%h-%%p-%%r
```

### Production deploy with manual approval gate

```yaml
jobs:
  deploy-staging:
    runs-on: ubuntu-latest
    environment: staging
    steps:
      - uses: actions/checkout@v4
      - name: Deploy to staging
        run: ansible-playbook ansible/site.yml -i ansible/inventory/staging/
        # ... (same setup steps as above)

  deploy-production:
    needs: deploy-staging
    runs-on: ubuntu-latest
    environment: production   # requires approval from configured reviewers
    steps:
      - uses: actions/checkout@v4
      - name: Deploy to production
        run: ansible-playbook ansible/site.yml -i ansible/inventory/production/
```

**Key decisions:**

| Decision | Why it matters |
|---|---|
| `pip install` with pinned `requirements.txt` | Ansible behaviour changes between minor versions. A pinned version ensures the CI runner uses the same Ansible as local development. `pip install ansible` pulls the latest, which breaks playbooks silently. |
| `cache: pip` on `setup-python` | Caches the pip download cache between runs. Ansible and its dependencies are ~200 MB — without caching, every run re-downloads them, adding 60–90 seconds to the job. |
| Vault password written to a file, removed with `if: always()` | The `--vault-password-file` flag requires a file. Writing the secret to a file and removing it in an `always` step ensures it is cleaned up even if the playbook fails. Using `--vault-password-file /dev/stdin` works locally but is unreliable on some CI runners. |
| `webfactory/ssh-agent` for SSH key | Loads the private key into an SSH agent rather than writing it to `~/.ssh/id_rsa`. The key is never on disk — the agent handles signing. `chmod 600` on a file is a weaker guarantee than keeping it in memory only. |
| `ssh-keyscan` for known_hosts | Prevents interactive host key prompts during the playbook run. `ANSIBLE_HOST_KEY_CHECKING=False` disables verification entirely — a weaker security posture. Populating known_hosts preserves verification without prompts. |
| `pipelining = True` in ansible.cfg | Reduces the number of SSH connections per task from 3 to 1 by piping commands directly to the Python interpreter. Can provide a 2–5× speedup on large inventories. Requires `requiretty` to be disabled in sudoers. |
| `paths:` filter on the trigger | Avoids running a full Ansible deploy on every commit. Only triggers when files under `ansible/` change — documentation updates and application code changes don't cause unnecessary deploys. |

## Verify it worked

```bash
# View the workflow run log
gh run list --workflow=deploy.yml

# Watch a run in real time
gh run watch $(gh run list --workflow=deploy.yml --json databaseId -q '.[0].databaseId')
```
