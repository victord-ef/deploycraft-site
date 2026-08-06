---
title: "Ansible — Dynamic AWS EC2 Inventory with Tags"
date: 2026-07-25
description: "Replace static inventory files with a live AWS EC2 inventory plugin that groups hosts automatically by tag, region, and environment — no manual host management."
lang: "Ansible"
tags: ["ansible", "inventory", "aws", "ec2", "dynamic"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-07-25"
draft: false
---

## When to use this

When your infrastructure is on AWS and hosts are created or destroyed frequently — autoscaling groups, spot fleets, ephemeral CI nodes. Maintaining a static `hosts.yml` for fleets that change daily is error-prone and breaks playbooks the moment a host is replaced.

## Without it

```yaml
# inventory/hosts.yml — static, breaks the moment an instance is replaced
all:
  hosts:
    10.0.1.45:
    10.0.1.67:
    10.0.1.89:
```

A terminated instance stays in the file and causes failed SSH connections. A new instance is invisible until someone remembers to update the file. In an autoscaling group this is unmanageable.

## Snippet

### 1. Install the AWS collection (once)

```bash
ansible-galaxy collection install amazon.aws
pip install boto3 botocore
```

### 2. inventory/aws_ec2.yml

The filename **must** end in `aws_ec2.yml` or `aws_ec2.yaml` for the plugin to activate.

```yaml
plugin: amazon.aws.aws_ec2

# Regions to query — list all regions your fleet spans
regions:
  - us-east-1
  - eu-west-1

# Filter to only running instances (exclude stopped/terminated)
filters:
  instance-state-name: running

# Group hosts by tag values — creates groups like:
#   tag_Environment_production, tag_Role_app, tag_Role_db
keyed_groups:
  - key: tags.Environment
    prefix: env
    separator: "_"
  - key: tags.Role
    prefix: role
    separator: "_"
  - key: tags.Project
    prefix: project
    separator: "_"
  - key: placement.region
    prefix: region
    separator: "_"

# Use private IP for hosts inside a VPC (change to public_ip_address for public access)
hostnames:
  - private-ip-address

# Expose useful EC2 metadata as host variables
compose:
  ansible_host: private_ip_address
  ec2_instance_id: instance_id
  ec2_instance_type: instance_type
  ec2_ami_id: image_id

# Cache inventory for 5 minutes to avoid repeated API calls during a run
cache: true
cache_plugin: jsonfile
cache_timeout: 300
cache_connection: /tmp/ansible_aws_ec2_cache
```

### 3. ansible.cfg — point to the dynamic inventory

```ini
[defaults]
inventory = inventory/aws_ec2.yml
remote_user = ec2-user           ; or ubuntu, admin — depends on the AMI
private_key_file = ~/.ssh/id_rsa

[inventory]
enable_plugins = amazon.aws.aws_ec2
```

### 4. AWS credentials

```bash
# Option A — environment variables (preferred for CI)
export AWS_ACCESS_KEY_ID=AKIA...
export AWS_SECRET_ACCESS_KEY=...
export AWS_DEFAULT_REGION=us-east-1

# Option B — AWS profile in ~/.aws/credentials
# Add profile: my-profile to aws_ec2.yml and set:
export AWS_PROFILE=my-profile

# Option C — IAM instance role (best for EC2 control nodes — no credentials needed)
# Attach an IAM role with ec2:DescribeInstances to the Ansible control node
```

### 5. Playbook using dynamic groups

```yaml
# playbooks/patch-app-servers.yml
- name: Patch production application servers
  hosts: env_production:&role_app    # intersection — production AND role=app
  become: true
  gather_facts: true

  tasks:
    - name: Update all packages
      ansible.builtin.apt:
        upgrade: dist
        update_cache: true
      when: ansible_os_family == "Debian"

    - name: Show instance details
      ansible.builtin.debug:
        msg: "Patching {{ inventory_hostname }} ({{ ec2_instance_id }}) — {{ ec2_instance_type }}"
```

**Key decisions:**

| Decision | Why it matters |
|---|---|
| Filename must end in `aws_ec2.yml` | The plugin is auto-detected by filename convention. Any other name requires explicit `plugin:` activation in `ansible.cfg`. |
| `instance-state-name: running` filter | Without this, terminated and stopped instances appear in the inventory and cause failed connections at the start of every play. |
| `keyed_groups` by tag | Groups are created dynamically from tag values — adding a new tag to an instance makes it available as a group in Ansible without touching any config file. |
| `private-ip-address` as hostname | Correct for hosts inside a VPC reached over a VPN or bastion. Switch to `public-ip-address` or `dns-name` only for publicly accessible instances. |
| `cache: true` with 300s timeout | The EC2 API is called once per run, not once per task. Without cache, a 200-host playbook triggers hundreds of API calls and hits rate limits. |
| IAM instance role for the control node | The most secure credential option — no static keys to rotate or leak. Requires the control node itself to be an EC2 instance. |

## Verify it worked

```bash
# List all hosts the plugin discovers (dry run — no SSH)
ansible-inventory -i inventory/aws_ec2.yml --list

# Show hosts grouped by environment tag
ansible-inventory -i inventory/aws_ec2.yml --graph

# Ping only production app servers
ansible env_production:&role_app -i inventory/aws_ec2.yml -m ping

# Show the variables resolved for a specific host
ansible-inventory -i inventory/aws_ec2.yml --host 10.0.1.45
```

Expected output from `--graph`:

```
@all:
  |--@env_production:
  |  |--10.0.1.45
  |  |--10.0.1.67
  |--@env_staging:
  |  |--10.0.1.89
  |--@role_app:
  |  |--10.0.1.45
  |--@role_db:
  |  |--10.0.1.67
```

> **Note:** The IAM policy on the credentials or instance role must include at minimum `ec2:DescribeInstances` and `ec2:DescribeTags`. Without `DescribeTags`, the inventory returns hosts but `keyed_groups` produces no tag-based groups.

## Full walkthrough

Managing multi-region AWS fleets with Ansible, SSM Session Manager, and dynamic inventory → **Tutorial Pair 44: Tenant Onboarding** *(coming soon)*.
