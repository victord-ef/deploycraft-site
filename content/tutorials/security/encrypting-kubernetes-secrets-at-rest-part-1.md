---
title: "Encrypting Kubernetes Secrets at Rest — Part 1"
date: 2026-09-01
description: "Enable envelope encryption for Kubernetes Secrets using a KMS provider or a local encryption key. Verify that etcd stores ciphertext and rotate encryption keys safely."
cluster: "Security & Hardening"
series: "Secrets"
part: 1
difficulty: "intermediate"
duration: "40 min"
tags: ["kubernetes", "secrets", "encryption", "etcd", "kms", "security", "devsecops"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

By the end of this tutorial you will have encryption at rest enabled for Kubernetes Secrets, verified that etcd contains ciphertext rather than base64-encoded plaintext, and safely rotated the encryption key. Part 2 builds on this foundation by replacing static Kubernetes Secrets with secrets synced dynamically from HashiCorp Vault using the External Secrets Operator.

## Prerequisites

- A self-managed Kubernetes cluster where you have access to the API server configuration (kubeadm, k3s, RKE2)
- SSH access to the control plane nodes
- `kubectl` with cluster-admin access
- For the KMS section: a cloud KMS key (AWS KMS, GCP Cloud KMS, or Azure Key Vault) or a local KMS plugin

> **Managed clusters (EKS, GKE, AKE):** Encryption at rest is configured through the cloud provider console or CLI, not through API server flags. The envelope encryption concepts apply, but the procedure differs — refer to your provider's documentation for the specific steps.

---

## Why Secrets encryption at rest matters

By default, Kubernetes Secrets are stored in etcd as base64-encoded values — not encrypted. Base64 is encoding, not encryption. Anyone with read access to etcd can decode every Secret in the cluster in seconds:

```bash
# What an attacker with etcd access sees (default, no encryption)
etcdctl get /registry/secrets/default/my-secret | strings
# Output: plaintext key-value pairs from your Secret
```

Encryption at rest ensures that etcd stores ciphertext. Even with direct etcd access, an attacker without the encryption key sees only opaque bytes.

Kubernetes supports two encryption approaches:

| Approach | Key storage | Suitable for |
|---|---|---|
| Local key (`aescbc`, `aesgcm`) | `EncryptionConfiguration` file on control plane | Dev/test, air-gapped clusters |
| Envelope encryption (KMS) | External KMS (AWS, GCP, Azure, Vault) | Production — key never touches the node |

---

## Step 1 — Check current encryption status

Before enabling encryption, verify the current state. Read a Secret directly from etcd to confirm it is stored in plaintext:

```bash
# Install etcdctl if not present
# On the control plane node:
ETCD_VERSION=v3.5.12
curl -LO https://github.com/etcd-io/etcd/releases/download/${ETCD_VERSION}/etcd-${ETCD_VERSION}-linux-amd64.tar.gz
tar -xzf etcd-${ETCD_VERSION}-linux-amd64.tar.gz
cp etcd-${ETCD_VERSION}-linux-amd64/etcdctl /usr/local/bin/
```

```bash
# Create a test Secret
kubectl create secret generic test-secret \
  --from-literal=password=supersecret \
  -n default

# Read it directly from etcd
ETCDCTL_API=3 etcdctl \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  --cert=/etc/kubernetes/pki/etcd/server.crt \
  --key=/etc/kubernetes/pki/etcd/server.key \
  get /registry/secrets/default/test-secret | hexdump -C | head -20
```

If you see `supersecret` in the output, encryption at rest is not enabled. The value is base64-decoded and stored as plaintext in etcd.

---

## Step 2 — Generate a local encryption key

For clusters where a KMS provider is not available, use a local AES-GCM key. Generate a 32-byte random key:

```bash
# Generate a base64-encoded 32-byte key
head -c 32 /dev/urandom | base64
# Example output: 4vLNv3y0Sj1+tGfp2kQwL8Xa9mZRc7Eo0DqUb3hYiPk=
```

Save this key — you will need it in the `EncryptionConfiguration` file and for key rotation later.

---

## Step 3 — Create the EncryptionConfiguration file

On each control plane node, create the encryption configuration file. This file defines which resources to encrypt and which providers to use:

```yaml
# /etc/kubernetes/encryption/config.yaml
apiVersion: apiserver.config.k8s.io/v1
kind: EncryptionConfiguration
resources:
  - resources:
      - secrets
    providers:
      - aesgcm:
          keys:
            - name: key1
              secret: 4vLNv3y0Sj1+tGfp2kQwL8Xa9mZRc7Eo0DqUb3hYiPk=
      - identity: {}
```

Key points about this configuration:

- **Provider order matters.** The first provider is used for encryption. All providers in the list are tried for decryption, in order. The `identity: {}` provider at the end allows reading unencrypted Secrets that existed before encryption was enabled.
- **`aesgcm` vs `aescbc`.** Use `aesgcm` (AES-GCM) — it is authenticated encryption and the current recommendation. `aescbc` is older and lacks authentication.
- **All control plane nodes must have the same file.** In a multi-control-plane cluster, copy this file to every control plane node before enabling it.

```bash
# Create the directory and set restrictive permissions
mkdir -p /etc/kubernetes/encryption
chmod 700 /etc/kubernetes/encryption

# Write the config file
# (copy the YAML above to /etc/kubernetes/encryption/config.yaml)
chmod 600 /etc/kubernetes/encryption/config.yaml
```

---

## Step 4 — Configure the API server to use encryption

Add the encryption configuration flag to the API server. For kubeadm clusters, edit the static pod manifest:

```bash
# On the control plane node
vi /etc/kubernetes/manifests/kube-apiserver.yaml
```

Add two entries to the `spec.containers[0].command` section:

```yaml
- --encryption-provider-config=/etc/kubernetes/encryption/config.yaml
- --encryption-provider-config-automatic-reload=true
```

Add a volume mount and volume for the encryption config directory:

```yaml
volumeMounts:
  - name: encryption-config
    mountPath: /etc/kubernetes/encryption
    readOnly: true

volumes:
  - name: encryption-config
    hostPath:
      path: /etc/kubernetes/encryption
      type: DirectoryOrCreate
```

The kubelet detects the manifest change and restarts the API server pod automatically. Watch for the restart:

```bash
watch kubectl get pods -n kube-system -l component=kube-apiserver
```

Wait until the pod is `Running` again before proceeding. If the pod crashes, check the API server logs:

```bash
crictl logs $(crictl ps -a --name kube-apiserver -q | head -1)
```

---

## Step 5 — Verify encryption is active

Create a new Secret and read it from etcd to confirm it is now encrypted:

```bash
kubectl create secret generic encrypted-secret \
  --from-literal=apikey=my-api-key-value \
  -n default

ETCDCTL_API=3 etcdctl \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  --cert=/etc/kubernetes/pki/etcd/server.crt \
  --key=/etc/kubernetes/pki/etcd/server.key \
  get /registry/secrets/default/encrypted-secret | hexdump -C | head -20
```

You should see the prefix `k8s:enc:aesgcm:v1:key1:` followed by opaque bytes. The plaintext value `my-api-key-value` should not appear anywhere in the output.

The old `test-secret` (created before encryption was enabled) is still stored in plaintext because only new writes are encrypted. Existing Secrets must be re-encrypted — covered in Step 7.

---

## Step 6 — Configure envelope encryption with AWS KMS

For production clusters, use envelope encryption. The API server generates a data encryption key (DEK) per Secret, encrypts the DEK with your KMS key, and stores only ciphertext in etcd. The KMS key never leaves the KMS service.

Install the AWS KMS provider plugin on each control plane node:

```bash
# Download the AWS encryption provider
curl -LO https://github.com/kubernetes-sigs/aws-encryption-provider/releases/latest/download/aws-encryption-provider_linux_amd64.tar.gz
tar -xzf aws-encryption-provider_linux_amd64.tar.gz
mv aws-encryption-provider /usr/local/bin/
chmod +x /usr/local/bin/aws-encryption-provider
```

Create a systemd service to run the KMS plugin as a Unix socket:

```bash
# /etc/systemd/system/aws-encryption-provider.service
[Unit]
Description=AWS KMS Encryption Provider for Kubernetes
After=network.target

[Service]
ExecStart=/usr/local/bin/aws-encryption-provider \
  --key=arn:aws:kms:eu-west-1:123456789012:key/your-key-id \
  --region=eu-west-1 \
  --listen=/var/run/kmsplugin/socket.sock
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
mkdir -p /var/run/kmsplugin
systemctl daemon-reload
systemctl enable --now aws-encryption-provider
```

Update `EncryptionConfiguration` to use the KMS provider:

```yaml
# /etc/kubernetes/encryption/config.yaml
apiVersion: apiserver.config.k8s.io/v1
kind: EncryptionConfiguration
resources:
  - resources:
      - secrets
    providers:
      - kms:
          apiVersion: v2
          name: aws-kms
          endpoint: unix:///var/run/kmsplugin/socket.sock
          timeout: 3s
      - identity: {}
```

Mount the socket directory in the API server manifest (add alongside the encryption config volume):

```yaml
volumeMounts:
  - name: kmsplugin
    mountPath: /var/run/kmsplugin

volumes:
  - name: kmsplugin
    hostPath:
      path: /var/run/kmsplugin
      type: DirectoryOrCreate
```

Restart the API server and verify with the etcd check from Step 5 — the prefix changes to `k8s:enc:kms:v2:aws-kms:`.

---

## Step 7 — Re-encrypt all existing Secrets

New Secrets are encrypted immediately. Secrets written before encryption was enabled remain as plaintext in etcd until they are updated. Force a re-write of all Secrets:

```bash
# Re-encrypt all Secrets cluster-wide
kubectl get secrets --all-namespaces -o json | \
  kubectl replace -f -
```

This reads every Secret through the API server (which decrypts using the `identity` provider) and writes it back (which encrypts using the configured provider). After this completes, remove the `identity: {}` entry from `EncryptionConfiguration` to prevent plaintext fallback:

```yaml
providers:
  - aesgcm:
      keys:
        - name: key1
          secret: 4vLNv3y0Sj1+tGfp2kQwL8Xa9mZRc7Eo0DqUb3hYiPk=
  # identity: {} — remove this line after re-encryption is complete
```

---

## Step 8 — Rotate the encryption key

Key rotation involves adding a new key as the primary encryption key while keeping the old key available for decryption:

**Phase 1 — Add new key as primary:**

```bash
# Generate a new key
head -c 32 /dev/urandom | base64
# Output: newkey...
```

```yaml
providers:
  - aesgcm:
      keys:
        - name: key2
          secret: <new-key>        # new key is first = used for encryption
        - name: key1
          secret: <old-key>        # old key stays for decryption of existing data
```

Restart the API server on all control plane nodes, then re-encrypt all Secrets to migrate them to the new key:

```bash
kubectl get secrets --all-namespaces -o json | kubectl replace -f -
```

**Phase 2 — Remove the old key:**

After re-encryption, remove `key1` from the configuration. All Secrets are now encrypted with `key2`.

```yaml
providers:
  - aesgcm:
      keys:
        - name: key2
          secret: <new-key>
```

Restart the API server once more. Store the retired `key1` value in a secure location for forensic purposes — you may need it to decrypt etcd backups taken before the rotation.

---

## What you have built

- etcd encryption at rest enabled with AES-GCM for a self-managed cluster
- Verified ciphertext storage in etcd before and after enabling encryption
- Envelope encryption configured with AWS KMS for production key management
- All pre-existing Secrets re-encrypted after enabling the provider
- Safe two-phase key rotation procedure with no downtime

## Next steps

In [Part 2](/tutorials/security/syncing-secrets-hashicorp-vault-external-secrets-operator-part-2/) you will replace static Kubernetes Secrets with dynamically synced secrets from HashiCorp Vault using the External Secrets Operator — eliminating long-lived secrets from your cluster configuration entirely.
