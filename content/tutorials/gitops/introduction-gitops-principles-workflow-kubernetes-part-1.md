---
title: "Introduction to GitOps Principles and Workflow in Kubernetes — Part 1"
date: 2026-09-02
description: "Understand the four core GitOps principles, how the reconciliation loop works, the difference between push-based and pull-based delivery, and why GitOps is the operational model that makes Kubernetes clusters auditable, reproducible, and recoverable."
cluster: "GitOps"
series: "GitOps Foundations"
part: 1
difficulty: "intermediate"
duration: "35 min"
tags: ["gitops", "kubernetes", "flux", "argocd", "devops", "delivery", "devsecops"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

By the end of this tutorial you will have a clear mental model of how GitOps works and why it matters — the four principles, the reconciliation loop, the difference between push-based and pull-based delivery, and what GitOps gives you that a standard CI/CD pipeline does not. Part 2 builds on this foundation by structuring a real GitOps repository for a multi-environment Kubernetes platform.

## Prerequisites

- Basic familiarity with Kubernetes — Deployments, Services, namespaces
- Understanding of Git branching and pull requests

---

## What GitOps is

GitOps is an operational model for managing infrastructure and applications where **Git is the single source of truth** for the desired state of your system. Every change — application deployment, configuration update, infrastructure modification — is made by committing to a Git repository. The cluster continuously reconciles its actual state toward what Git declares.

The term was coined by Weaveworks in 2017. The underlying insight was simple but powerful: Git already has everything you need for production operations — version history, audit trail, rollback, peer review via pull requests, access control, and branching for environment promotion. Instead of running `kubectl apply` from a pipeline, you commit to Git and let an in-cluster operator do the applying.

---

## Step 1 — The four GitOps principles

The OpenGitOps project (CNCF) defines GitOps through four principles:

### 1. Declarative

The desired state of the system is expressed declaratively — describing *what* you want, not *how* to achieve it. Kubernetes manifests (YAML) are inherently declarative. You declare "I want 3 replicas of this Deployment" and Kubernetes works out how to get there.

The alternative is imperative: "scale the Deployment to 3, then update the image, then restart the pods." Imperative commands produce unknown state — the result depends on the current state and whether every step succeeded.

### 2. Versioned and immutable

The desired state is stored in a way that enforces immutability and retains a complete version history. Git satisfies both: every commit is content-addressed (SHA), history is append-only, and you can reconstruct the exact state of any point in time.

This makes the answer to "what changed, when, and who approved it" a simple `git log`.

### 3. Pulled automatically

The desired state is applied to the system automatically. A software agent running *inside* the cluster continuously polls Git and applies changes when the desired state diverges from actual state. The cluster pulls its configuration — it is never pushed to from outside.

This is the most important operational distinction from traditional CI/CD. Nothing outside the cluster needs credentials to modify it. The attack surface for supply chain compromise is dramatically reduced.

### 4. Continuously reconciled

The software agent continuously compares desired state (Git) with actual state (cluster) and corrects any drift. If someone runs `kubectl edit` directly against a live cluster, the agent detects the discrepancy and reverts it within minutes. Accidental or malicious configuration drift is automatically corrected.

---

## Step 2 — The reconciliation loop

The reconciliation loop is the operational heart of GitOps:

```
┌─────────────────────────────────────────────────────────┐
│                    Git Repository                        │
│                 (desired state)                          │
└─────────────────────┬───────────────────────────────────┘
                      │ poll every 1m (or webhook trigger)
                      ▼
┌─────────────────────────────────────────────────────────┐
│              GitOps Operator (Flux / ArgoCD)            │
│              running inside the cluster                  │
│                                                          │
│  1. Fetch latest commit from Git                        │
│  2. Render manifests (Kustomize / Helm)                  │
│  3. Compare with current cluster state                   │
│  4. Apply diff to reconcile                              │
└─────────────────────┬───────────────────────────────────┘
                      │ kubectl apply (in-cluster)
                      ▼
┌─────────────────────────────────────────────────────────┐
│                 Kubernetes Cluster                        │
│                  (actual state)                          │
└─────────────────────────────────────────────────────────┘
```

The operator runs this loop continuously — typically every 1–10 minutes, or triggered immediately by a Git webhook. If actual state matches desired state, nothing happens. If they diverge (a deployment was scaled manually, a ConfigMap was edited in-cluster, a node restarted and a pod failed to reschedule), the operator applies the diff to bring the cluster back to the declared state.

---

## Step 3 — Push-based vs pull-based delivery

Understanding this distinction is essential for evaluating GitOps against traditional CI/CD.

### Push-based (traditional CI/CD)

```
Developer → Git push → CI pipeline → kubectl apply → Cluster
                                      (from outside)
```

A CI job (GitHub Actions, Jenkins, GitLab CI) runs `kubectl apply` against the cluster after a successful build. The cluster is a passive target — changes are pushed to it.

**Problems:**
- The CI system needs long-lived cluster credentials stored as CI secrets
- If the CI system is compromised, an attacker can deploy anything to the cluster
- No continuous reconciliation — drift from manual `kubectl` commands is not detected or corrected
- No audit trail for what is actually running — the cluster state and the pipeline history may diverge
- Recovery after a cluster rebuild requires re-running pipelines, with no guarantee of reproducibility

### Pull-based (GitOps)

```
Developer → Git push → GitOps operator (inside cluster) → Cluster
                        (polls Git, applies internally)
```

An operator running inside the cluster polls Git and applies changes. The cluster reaches out to Git — nothing reaches into the cluster.

**Advantages:**
- No external credentials needed to modify the cluster — only the operator has cluster access
- Reconciliation is continuous — drift is detected and corrected automatically
- Git history is the audit trail — every change is a commit with author, timestamp, and review
- Full cluster recovery is a `git clone` + `flux bootstrap` — the entire desired state is in Git
- Environment promotion is a Git operation (merge, PR) not a pipeline configuration change

---

## Step 4 — What changes in your workflow

GitOps changes how engineers interact with the cluster day-to-day:

| Traditional workflow | GitOps workflow |
|---|---|
| `kubectl edit deployment my-app` | Edit the manifest in Git, commit, push |
| Scale: `kubectl scale deployment my-app --replicas=5` | Update `replicas: 5` in Git |
| Rollback: re-run old pipeline | `git revert <commit>` — operator applies automatically |
| Audit: "who changed this?" | `git log apps/my-app/deployment.yaml` |
| Disaster recovery: re-run pipelines | `git clone` + bootstrap operator — cluster self-heals |
| Environment promotion: copy pipeline vars | Merge or cherry-pick commits between env branches/directories |

The mental shift: **the cluster is read-only from the outside**. Everything goes through Git.

---

## Step 5 — Flux vs ArgoCD

The two dominant GitOps operators have different design philosophies:

| Dimension | Flux | ArgoCD |
|---|---|---|
| Interface | CLI + Kubernetes CRDs | Web UI + CLI + CRDs |
| Architecture | Separate controllers per concern | Monolithic application server |
| Configuration | Managed in Git (bootstrap) | UI or declarative Application CRDs |
| Multi-tenancy | Namespace-scoped CRDs per tenant | Projects and RBAC in ArgoCD |
| Helm support | HelmRelease CRD | Native Helm application |
| Kustomize support | Kustomization CRD | Native Kustomize application |
| Notification | Flux Notification Controller | ArgoCD Notifications |
| Image automation | Built-in ImageRepository/Policy | ArgoCD Image Updater (separate install) |
| Learning curve | Steeper (CRD-first) | Gentler (UI-first) |

**Choose Flux when:**
- You prefer a fully declarative, Git-native approach with no UI dependency
- You want the operator itself managed as code (Flux bootstraps from a Git repo)
- Your team is comfortable with Kubernetes CRDs and the CLI

**Choose ArgoCD when:**
- You want a visual dashboard showing sync status across applications and clusters
- Your team is new to GitOps and benefits from UI-driven onboarding
- You have a large number of applications requiring centralised visibility

Both are CNCF graduated projects and production-ready. Many organisations use both: ArgoCD for application delivery visibility, Flux for infrastructure and operator management.

---

## Step 6 — The GitOps security model

GitOps improves the security posture of cluster operations in several ways:

**No external credentials in CI:**
The cluster operator has a read-only Git token. No CI system has `kubectl` access to production. Compromise of your CI system does not give an attacker cluster access.

**Peer review for every change:**
All cluster changes go through a pull request. A second engineer reviews the diff before it is applied. This catches misconfigurations that would previously have been `kubectl apply`-ed directly.

**Automatic drift correction:**
Manual changes to the cluster — whether accidental or malicious — are detected and reverted within minutes. An attacker who gains temporary `kubectl` access cannot make persistent changes.

**Complete audit trail:**
Every change to every resource is a Git commit with author identity, timestamp, and linked PR. "Who changed the replica count on that deployment on Tuesday?" is a `git log` query, not a support ticket.

**Immutable desired state:**
Git's content-addressing means the history cannot be silently rewritten. Every commit is a cryptographic hash of its content and parent — tampering is detectable.

---

## Step 7 — When GitOps is not the right fit

GitOps is not universally appropriate for every type of change:

**Stateful configuration with secrets:**
Secrets should not be stored in Git in plaintext. GitOps requires a secrets management strategy — sealed-secrets, External Secrets Operator, or SOPS encryption — before it can manage secret-containing workloads.

**One-off operational tasks:**
Running a database migration, triggering a manual job, or executing a diagnostic command are not GitOps operations. These are imperative, one-shot tasks that do not represent desired state.

**Rapidly mutating state:**
If a resource changes thousands of times per second (e.g., a custom resource managed by a high-frequency controller), modelling that in Git is impractical. GitOps manages configuration, not runtime application state.

**Initial cluster bootstrapping:**
The first cluster creation — provisioning the cloud resources, installing the OS, creating the Kubernetes control plane — typically requires imperative tooling (Terraform, Pulumi, kubeadm). GitOps takes over once the cluster exists.

---

## What you have built

- The four GitOps principles — declarative, versioned, pulled, continuously reconciled
- The reconciliation loop — how the operator compares Git and cluster state and applies the diff
- Push-based vs pull-based delivery — the security and operational differences
- How daily engineering workflows change under GitOps
- Flux vs ArgoCD — design philosophy, feature comparison, and selection criteria
- The GitOps security model — no external credentials, peer review, drift correction, audit trail
- Where GitOps does not fit — secrets, one-off tasks, runtime state, initial bootstrapping

In [Part 2](/tutorials/gitops/setting-up-gitops-repository-structure-kubernetes-part-2/) you will design and implement a production GitOps repository structure: environment directories, Kustomize overlays, Helm releases, application tenancy separation, and the conventions that keep a multi-team GitOps repository maintainable as it grows.
