---
title: "Ansible — Deploy a Kubernetes Manifest with the k8s Module"
date: 2026-07-25
description: "Use Ansible's kubernetes.core.k8s module to apply, update, and delete Kubernetes manifests from a playbook — no kubectl required on the control node."
lang: "Ansible"
tags: ["ansible", "kubernetes", "k8s-module", "manifest", "deploy"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-07-25"
draft: false
---

## When to use this

When Kubernetes deployments are one step in a larger Ansible workflow — for example, after provisioning infrastructure with Terraform, configuring nodes, and pushing secrets. Keeping everything in one playbook removes the need to shell out to `kubectl` and keeps idempotency consistent across the whole run.

## Without it

```yaml
# Common workaround — shelling out to kubectl
- name: Apply deployment
  ansible.builtin.command:
    cmd: kubectl apply -f manifests/deployment.yaml
```

This breaks idempotency (`changed` on every run regardless of actual state), requires `kubectl` installed and configured on the control node, and gives no structured output back to Ansible for conditional logic.

## Snippet

### 1. Install the collection (once)

```bash
ansible-galaxy collection install kubernetes.core
```

### 2. Playbook — apply a manifest from a file

```yaml
# playbooks/deploy-app.yml
- name: Deploy application to Kubernetes
  hosts: localhost          # k8s module talks to the API server, not the node
  connection: local
  gather_facts: false

  vars:
    kubeconfig_path: "~/.kube/config"
    app_namespace: "my-app"
    app_image: "my-registry/my-app:1.2.0"

  tasks:
    - name: Ensure namespace exists
      kubernetes.core.k8s:
        kubeconfig: "{{ kubeconfig_path }}"
        state: present
        definition:
          apiVersion: v1
          kind: Namespace
          metadata:
            name: "{{ app_namespace }}"

    - name: Apply Deployment manifest from file
      kubernetes.core.k8s:
        kubeconfig: "{{ kubeconfig_path }}"
        state: present
        src: "../manifests/deployment.yaml"

    - name: Apply Deployment inline with variable substitution
      kubernetes.core.k8s:
        kubeconfig: "{{ kubeconfig_path }}"
        state: present
        definition:
          apiVersion: apps/v1
          kind: Deployment
          metadata:
            name: my-app
            namespace: "{{ app_namespace }}"
          spec:
            replicas: 2
            selector:
              matchLabels:
                app: my-app
            template:
              metadata:
                labels:
                  app: my-app
              spec:
                containers:
                  - name: my-app
                    image: "{{ app_image }}"
                    ports:
                      - containerPort: 8080

    - name: Wait for rollout to complete
      kubernetes.core.k8s_rollout_status:
        kubeconfig: "{{ kubeconfig_path }}"
        name: my-app
        namespace: "{{ app_namespace }}"
        kind: Deployment
        timeout: 120
```

### 3. Delete a resource

```yaml
    - name: Remove a resource by state absent
      kubernetes.core.k8s:
        kubeconfig: "{{ kubeconfig_path }}"
        state: absent
        definition:
          apiVersion: apps/v1
          kind: Deployment
          metadata:
            name: my-app
            namespace: "{{ app_namespace }}"
```

**Key decisions:**

| Decision | Why it matters |
|---|---|
| `hosts: localhost` + `connection: local` | The module calls the Kubernetes API directly — it does not SSH into a cluster node. Running on localhost avoids unnecessary network hops. |
| `state: present` | Idempotent — applies on first run, patches on subsequent runs if the definition changed, no-ops if already in desired state. |
| `src:` vs `definition:` | Use `src` for static manifests you version in Git as-is. Use `definition` when you need Ansible variable substitution inside the manifest. |
| `k8s_rollout_status` | The `k8s` module returns after the API accepts the resource, not after pods are running. Adding a rollout wait catches image pull errors and CrashLoopBackOffs before the play reports success. |
| `kubeconfig` param | Explicit path prevents the module from silently picking up the wrong context when multiple clusters are configured. |

## Verify it worked

```bash
# Run the playbook
ansible-playbook playbooks/deploy-app.yml

# Confirm the deployment is live and ready
kubectl get deployment my-app -n my-app
kubectl rollout status deployment/my-app -n my-app

# Check the exact image version was applied
kubectl get deployment my-app -n my-app \
  -o jsonpath='{.spec.template.spec.containers[0].image}'
```

Expected output from the image check:

```
my-registry/my-app:1.2.0
```

> **Note:** The `kubernetes.core` collection requires the `kubernetes` Python package on the control node: `pip install kubernetes`.

## Full walkthrough

Combining Ansible with Kubernetes for full-stack provisioning and deployment pipelines → **Tutorial Pair 44: Tenant Onboarding** *(coming soon)*.
