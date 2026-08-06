---
title: "Ansible Vault — Encrypting and Using Secrets"
date: 2026-07-25
description: "Encrypt sensitive variables with ansible-vault so secrets never sit in plaintext in your repository, and use them transparently in playbooks."
lang: "Ansible"
tags: ["ansible", "vault", "secrets", "encryption", "security"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-07-25"
draft: false
---

## When to use this

Any time a playbook needs a password, API key, or certificate — and that playbook lives in a Git repository. Plaintext secrets in YAML files get committed, leaked in diffs, and end up in log output.

## Without it

```yaml
# vars/secrets.yml — plaintext, committed to Git
db_password: "SuperSecret123"
api_key: "sk-prod-abc123xyz"
```

Anyone with read access to the repository — including CI runners, contractors, and future employees — has those credentials permanently in git history even after the file is updated.

## Snippet

### 1. Encrypt a secrets file

```bash
# Encrypt an existing file in place
ansible-vault encrypt vars/secrets.yml

# Create a new encrypted file from scratch
ansible-vault create vars/secrets.yml
```

The file becomes an opaque vault blob:

```yaml
$ANSIBLE_VAULT;1.1;AES256
61383232363636333937313937653165623865343564333733333165336665613438376364663033
3634623265663464316162306665306233363865636633320a3964333431...
```

Safe to commit. Unreadable without the vault password.

### 2. vars/secrets.yml (before encryption)

```yaml
db_password: "SuperSecret123"
db_user: "app_user"
api_key: "sk-prod-abc123xyz"
```

### 3. Reference vault variables in a playbook

```yaml
# playbooks/deploy.yml
- name: Deploy application
  hosts: app_servers
  vars_files:
    - ../vars/secrets.yml        # Ansible decrypts this transparently at runtime
  tasks:
    - name: Configure database connection
      ansible.builtin.template:
        src: templates/db.conf.j2
        dest: /etc/myapp/db.conf
        mode: "0640"

    - name: Set API key in environment
      ansible.builtin.lineinfile:
        path: /etc/myapp/app.env
        line: "API_KEY={{ api_key }}"
        create: true
        mode: "0640"
```

### 4. Store the vault password securely

```bash
# Option A — password file (add to .gitignore immediately)
echo "your-vault-password" > .vault_pass
echo ".vault_pass" >> .gitignore
ansible-playbook playbooks/deploy.yml --vault-password-file .vault_pass

# Option B — environment variable (preferred for CI)
export ANSIBLE_VAULT_PASSWORD_FILE=.vault_pass
ansible-playbook playbooks/deploy.yml

# Option C — prompt at runtime (preferred for local use)
ansible-playbook playbooks/deploy.yml --ask-vault-pass
```

### 5. Editing an encrypted file later

```bash
# Opens the decrypted content in $EDITOR, re-encrypts on save
ansible-vault edit vars/secrets.yml
```

**Key decisions:**

| Decision | Why it matters |
|---|---|
| Encrypt the whole file, not individual values | Simpler to manage. Inline encrypted values (`!vault`) work but make diffs unreadable and reviews painful. |
| `vars_files` over `include_vars` | Loaded at parse time — variables are available everywhere in the play without task ordering concerns. |
| Password file in `.gitignore` | The one thing that must never be committed. Verify `.gitignore` is in place before running `encrypt`. |
| `mode: "0640"` on written files | The secret leaves the vault and lands on disk — restrict it to the owning user and group immediately. |
| CI via environment variable | Most CI systems (GitHub Actions, GitLab CI) support injecting secrets as env vars, keeping the password out of the repo entirely. |

## Verify it worked

```bash
# Confirm the file is encrypted (should print the vault header, not plaintext)
head -1 vars/secrets.yml

# Decrypt and view without writing plaintext to disk
ansible-vault view vars/secrets.yml

# Dry-run the playbook to confirm vault variables resolve correctly
ansible-playbook playbooks/deploy.yml --ask-vault-pass --check --diff
```

Expected output from `head -1`:

```
$ANSIBLE_VAULT;1.1;AES256
```

If you see your variable names instead, the file was never encrypted.

## Full walkthrough

How to manage vault passwords in CI/CD pipelines and rotate secrets without downtime → **Tutorial Pair 57: Secrets Management** *(coming soon)*.
