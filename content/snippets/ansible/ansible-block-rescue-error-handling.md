---
title: "Ansible — Error Handling with block, rescue, and always"
date: 2026-11-12
description: "Use block/rescue/always to run cleanup tasks on failure, send failure notifications, roll back a partial deploy, and distinguish between expected and unexpected errors."
lang: "Ansible"
tags: ["ansible", "error-handling", "block", "rescue", "always", "rollback", "notifications"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-11-12"
draft: false
---

## When to use this

When a task sequence must be treated as a unit — if any step fails, run cleanup or rollback tasks before the play exits. Without `block`/`rescue`, a failed task immediately stops the play and leaves the system in a partial state: service stopped, new binary deployed but not started, load balancer entry removed but never re-added.

## Without it

```yaml
- name: Stop service
  ansible.builtin.service:
    name: myapp
    state: stopped

- name: Deploy binary         # if this fails...
  ansible.builtin.copy: ...

- name: Start service         # ...this never runs
  ansible.builtin.service:
    name: myapp
    state: started
# Service is now stopped on the host. No rollback, no notification.
```

## Snippet

### Basic block / rescue / always

```yaml
- name: Deploy application with rollback
  block:
    - name: Stop application service
      ansible.builtin.service:
        name: myapp
        state: stopped

    - name: Back up current binary
      ansible.builtin.copy:
        src: /opt/myapp/myapp
        dest: /opt/myapp/myapp.backup
        remote_src: true

    - name: Deploy new binary
      ansible.builtin.copy:
        src: "{{ deploy_artifact }}"
        dest: /opt/myapp/myapp
        owner: appuser
        group: appuser
        mode: "0750"

    - name: Run database migrations
      ansible.builtin.command:
        cmd: /opt/myapp/myapp migrate --config /etc/myapp/config.yaml
      register: migration_result

    - name: Start application service
      ansible.builtin.service:
        name: myapp
        state: started

    - name: Verify application health
      ansible.builtin.uri:
        url: "http://localhost:8080/healthz"
        status_code: 200
      retries: 5
      delay: 3
      until: health_check.status == 200
      register: health_check

  rescue:
    # Runs only when any task in the block fails
    - name: Log failure reason
      ansible.builtin.debug:
        msg: "Deploy failed on {{ inventory_hostname }}: {{ ansible_failed_task.name }}"

    - name: Restore previous binary from backup
      ansible.builtin.copy:
        src: /opt/myapp/myapp.backup
        dest: /opt/myapp/myapp
        remote_src: true
        owner: appuser
        group: appuser
        mode: "0750"

    - name: Restart application with previous binary
      ansible.builtin.service:
        name: myapp
        state: restarted

    # Re-raise the error so the play still fails after rescue completes
    - name: Fail the play after rollback
      ansible.builtin.fail:
        msg: "Deploy failed and was rolled back. Check {{ ansible_failed_task.name }}."

  always:
    # Runs regardless of success or failure — use for cleanup and notifications
    - name: Remove deploy artifact from remote
      ansible.builtin.file:
        path: "/tmp/{{ deploy_artifact | basename }}"
        state: absent

    - name: Send Slack notification
      community.general.slack:
        token: "{{ slack_token }}"
        channel: "#deployments"
        msg: >-
          {{ 'FAILED' if ansible_failed_task is defined else 'SUCCESS' }}:
          Deploy {{ app_version }} on {{ inventory_hostname }}
      delegate_to: localhost
      when: slack_token is defined
```

### Ignoring expected errors with failed_when

```yaml
# Use failed_when instead of block/rescue when a task may legitimately fail
- name: Check if application is already running
  ansible.builtin.command:
    cmd: systemctl is-active myapp
  register: service_status
  failed_when: false          # never fail — capture the result instead
  changed_when: false

- name: Drain connections before stop (only if running)
  ansible.builtin.command:
    cmd: /opt/myapp/myapp drain --timeout 30s
  when: service_status.stdout == "active"
```

### Catching specific error conditions

```yaml
- name: Deploy with specific error handling
  block:
    - name: Apply database migration
      ansible.builtin.command:
        cmd: /opt/myapp/myapp migrate
      register: migrate_result

  rescue:
    - name: Handle migration conflict (exit code 2 — already applied)
      ansible.builtin.debug:
        msg: "Migration already applied — skipping"
      when: migrate_result.rc == 2

    - name: Re-raise on unexpected errors
      ansible.builtin.fail:
        msg: "Migration failed with unexpected exit code {{ migrate_result.rc }}"
      when: migrate_result.rc not in [0, 2]
```

### Nested blocks for fine-grained control

```yaml
- name: Outer block — entire deploy unit
  block:

    - name: Inner block — optional pre-flight check
      block:
        - name: Ping upstream database
          ansible.builtin.wait_for:
            host: "{{ db_host }}"
            port: 5432
            timeout: 10
      rescue:
        - name: Warn but continue if DB ping fails (may be firewalled)
          ansible.builtin.debug:
            msg: "Could not reach DB at pre-flight — continuing anyway"

    - name: Core deploy tasks
      ansible.builtin.include_tasks: deploy.yml

  rescue:
    - name: Outer rescue — rollback entire deploy
      ansible.builtin.include_tasks: rollback.yml
```

**Key decisions:**

| Decision | Why it matters |
|---|---|
| `ansible_failed_task.name` in rescue | Built-in variable set when a task in the block fails. Logs the exact task name and module that caused the failure — far more useful than a generic "deploy failed" message. |
| `ansible.builtin.fail` at the end of rescue | `rescue` suppresses the original failure — the play continues and reports success unless you explicitly fail it. Always re-raise after rollback so callers (CI pipelines, monitoring) know the deploy failed. |
| `always` for artifact cleanup | Whether the deploy succeeds or fails, temporary files should be removed and notifications sent. `always` guarantees this runs in both paths without duplicating tasks. |
| `delegate_to: localhost` on Slack notification | The `community.general.slack` module makes an HTTPS call. Delegating to localhost means the control node makes the call — not each application host, which may not have internet access. |
| `failed_when: false` over `ignore_errors: true` | `ignore_errors: true` suppresses the failure but still marks the task as failed in the play output. `failed_when: false` treats any exit code as success — cleaner for status checks where non-zero is expected. |
| `changed_when: false` on status check | The `command` module always reports `changed`. For read-only status queries, `changed_when: false` prevents spurious change counts in the play summary. |

## Verify it worked

```bash
# Trigger the rescue path intentionally by pointing to a non-existent artifact
ansible-playbook deploy.yml -i inventory/ \
  -e "app_version=1.2.3 deploy_artifact=/nonexistent/myapp"
# Expected: rescue tasks run, play fails after rollback, service still running

# Confirm service is still running after a failed deploy
ansible appservers -i inventory/ -m command -a "systemctl is-active myapp"
# Expected: active
```
