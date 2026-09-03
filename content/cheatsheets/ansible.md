---
title: "Ansible"
description: "Ansible ad-hoc commands, playbook execution, inventory, vault, and galaxy for automation and configuration management."
icon: "⚙️"
weight: 4
count: 45
tags: ["ansible", "automation", "iac"]
---

## Ad-hoc Commands

```bash
ansible all -m ping
ansible all -m ping -i inventory.ini
ansible webservers -m ping
ansible all -m shell -a "uptime"
ansible all -m shell -a "df -h"
ansible all -m command -a "systemctl status nginx"
ansible all -m copy -a "src=./file dest=/tmp/file"
ansible all -m file -a "path=/tmp/test state=directory mode=0755"
ansible all -m apt -a "name=nginx state=present" --become
ansible all -m service -a "name=nginx state=restarted" --become
ansible all -m user -a "name=deploy state=present" --become
ansible all -m gather_facts
```

## Playbook Execution

```bash
ansible-playbook site.yml
ansible-playbook site.yml -i inventory.ini
ansible-playbook site.yml --check                       # dry run
ansible-playbook site.yml --diff                        # show diffs
ansible-playbook site.yml --check --diff
ansible-playbook site.yml -v                            # verbose
ansible-playbook site.yml -vvv                          # very verbose
ansible-playbook site.yml --tags deploy
ansible-playbook site.yml --skip-tags debug
ansible-playbook site.yml --limit webservers
ansible-playbook site.yml --limit 192.168.1.10
ansible-playbook site.yml --start-at-task "Install nginx"
ansible-playbook site.yml -e "env=prod"
ansible-playbook site.yml -e @vars/prod.yml
ansible-playbook site.yml --list-tasks
ansible-playbook site.yml --list-hosts
ansible-playbook site.yml --syntax-check
ansible-playbook site.yml --become --become-user=root
ansible-playbook site.yml -u deploy --private-key=~/.ssh/id_rsa
```

## Inventory

```bash
ansible-inventory -i inventory.ini --list
ansible-inventory -i inventory.ini --graph
ansible-inventory -i inventory.ini --host <hostname>
ansible all --list-hosts
ansible webservers --list-hosts
```

## Vault

```bash
ansible-vault encrypt vars/secrets.yml
ansible-vault decrypt vars/secrets.yml
ansible-vault view vars/secrets.yml
ansible-vault edit vars/secrets.yml
ansible-vault rekey vars/secrets.yml
ansible-vault encrypt_string 'my_secret' --name 'db_password'
ansible-playbook site.yml --ask-vault-pass
ansible-playbook site.yml --vault-password-file=.vault_pass
```

## Galaxy

```bash
ansible-galaxy role init <role_name>
ansible-galaxy install <namespace>.<role>
ansible-galaxy install -r requirements.yml
ansible-galaxy list
ansible-galaxy collection install <namespace>.<collection>
ansible-galaxy collection install -r requirements.yml
```

## Config & Facts

```bash
ansible-config list
ansible-config dump
ansible all -m setup                                    # gather all facts
ansible all -m setup -a "filter=ansible_os_family"
ansible all -m setup -a "filter=ansible_interfaces"
ansible all -m debug -a "var=hostvars[inventory_hostname]"
```
