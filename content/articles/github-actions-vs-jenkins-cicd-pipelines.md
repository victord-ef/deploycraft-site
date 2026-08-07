---
title: "GitHub Actions vs Jenkins for CI/CD Pipelines"
date: 2026-08-06
author: "Victor D"
description: "Jenkins gives you complete control over your CI/CD infrastructure. GitHub Actions removes most of that infrastructure entirely. The choice is about what your team is actually equipped and willing to operate."
tags: ["github-actions", "jenkins", "cicd", "devops", "pipelines", "kubernetes", "security", "comparisons"]
categories: ["article"]
draft: false
toc: true
---

A caveat before anything else: if your code is not on GitHub, this comparison is largely academic. GitHub Actions is built into GitHub and works nowhere else. If you are on GitLab, Bitbucket, or Gitea, your path leads to GitLab CI, Bitbucket Pipelines, or Jenkins — not here. This article is specifically about teams whose source of truth is GitHub, deciding whether to stay in the GitHub ecosystem for CI/CD or run their own Jenkins.

With that framing, the real question is not "which tool is more powerful" — Jenkins is a 15-year-old project with 1,800 plugins and can do almost anything. The question is: how much CI/CD infrastructure are you willing to own, and what does that ownership actually cost?

---

## Configuration: YAML vs Groovy

The day-to-day experience of writing and maintaining pipelines looks very different between the two tools.

**GitHub Actions** pipelines are YAML files stored in `.github/workflows/`. They are event-driven — a push, a pull request, a schedule, a manual dispatch, or an event in another repository triggers a workflow:

```yaml
name: Build and Deploy
on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  build:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write
    steps:
    - uses: actions/checkout@v4

    - name: Build Docker image
      run: |
        docker build \
          --tag ghcr.io/${{ github.repository }}:${{ github.sha }} \
          --label "org.opencontainers.image.revision=${{ github.sha }}" \
          .

    - name: Push to GitHub Container Registry
      run: docker push ghcr.io/${{ github.repository }}:${{ github.sha }}

  deploy:
    needs: build
    runs-on: ubuntu-latest
    environment: production        # requires environment protection rules to pass
    steps:
    - uses: actions/checkout@v4

    - name: Deploy to Kubernetes
      uses: azure/k8s-deploy@v4
      with:
        manifests: k8s/production/
        images: ghcr.io/${{ github.repository }}:${{ github.sha }}
```

The pipeline, its triggers, its permissions, and its environment gates are all visible in one file, in the repository, reviewable in a pull request. Adding a new pipeline is a file commit.

**Jenkins** pipelines are `Jenkinsfile` using either Declarative or Scripted Pipeline syntax — both Groovy-based. Declarative is more constrained and readable; Scripted gives you a full programming language at the cost of discipline:

```groovy
pipeline {
  agent {
    kubernetes {
      yaml """
        apiVersion: v1
        kind: Pod
        spec:
          containers:
          - name: kaniko
            image: gcr.io/kaniko-project/executor:latest
            command: [sleep]
            args: [infinity]
          - name: kubectl
            image: bitnami/kubectl:latest
            command: [sleep]
            args: [infinity]
      """
    }
  }

  environment {
    IMAGE = "your-registry.example.com/api-server:${env.GIT_COMMIT}"
  }

  stages {
    stage('Build') {
      steps {
        container('kaniko') {
          sh """
            /kaniko/executor \
              --context=${env.WORKSPACE} \
              --destination=${IMAGE}
          """
        }
      }
    }

    stage('Deploy') {
      when {
        branch 'main'
      }
      steps {
        container('kubectl') {
          withCredentials([file(credentialsId: 'kubeconfig-prod', variable: 'KUBECONFIG')]) {
            sh "kubectl set image deployment/api-server api-server=${IMAGE} -n production"
          }
        }
      }
    }
  }
}
```

Groovy is expressive — you can write functions, use loops, build dynamic stage lists, and call Java libraries. This flexibility is genuinely useful for complex pipelines. It is also a maintenance burden: Groovy shared libraries become complex over time, Scripted Pipelines without discipline become unreadable, and debugging Groovy stack traces at 2am is not pleasant.

---

## Runners and agents: ephemeral vs persistent

**GitHub Actions runners** are ephemeral by default. Each job gets a fresh virtual machine — no state from previous builds, no dependency accumulation, no "it worked on my runner" drift. GitHub's hosted runners (Ubuntu, Windows, macOS) are provisioned, used for one job, and destroyed.

You can also run **self-hosted runners** in your own infrastructure, including as Kubernetes pods via the Actions Runner Controller. Self-hosted runners are useful for jobs that need access to private networks, specific hardware, or environments that cannot be reproduced in a hosted container.

**Jenkins** uses a controller-agent architecture. The Jenkins controller (formerly called master) manages the web UI, stores build history, schedules jobs, and coordinates agents. Agents execute the builds. Agents can be persistent VMs or dynamic pods via the Kubernetes plugin:

```groovy
// Kubernetes plugin: spin up a pod agent per build
agent {
  kubernetes {
    cloud 'kubernetes'
    yaml """
      spec:
        containers:
        - name: jnlp
          image: jenkins/inbound-agent:latest
    """
  }
}
```

The Kubernetes plugin is widely used and works well, but it adds a layer of complexity: the Jenkins controller needs a service account with permission to create pods, pod templates need maintenance, and JNLP connectivity between the pod agent and the controller needs to be reliable.

The Jenkins controller itself is stateful and needs to be treated carefully: regular backups, upgrade planning, and high availability if uptime matters. A crashed Jenkins controller takes all build history and in-progress jobs with it.

---

## The plugin ecosystem vs the Actions marketplace

Jenkins has over 1,800 plugins covering nearly every integration imaginable. The depth is real. So is the maintenance burden.

Plugin compatibility is Jenkins's most persistent operational headache. Upgrading Jenkins core can break plugins. Plugins have their own release cycles, their own issue trackers, and some are maintained by a single volunteer. Before any Jenkins upgrade, the responsible path is checking the plugin compatibility matrix, testing in a staging Jenkins instance, and having a rollback plan. Teams running Jenkins at scale often pin plugin versions for this reason — which then requires active effort to stay current on security patches.

```groovy
// Shared library — Jenkins's answer to reusable pipeline logic
// vars/buildAndPush.groovy in a separate repo, referenced in Jenkinsfile:
@Library('pipeline-library@v2.3.1') _
buildAndPush(
  image: 'your-registry.example.com/api-server',
  dockerfile: 'Dockerfile'
)
```

Shared libraries in Jenkins are powerful for standardising pipelines across many repositories, but they require a separate repository, version management, and a team that maintains them.

**GitHub Actions** uses the Marketplace — thousands of pre-built Actions contributed by the community and verified publishers. Adding an Action is one line:

```yaml
- uses: actions/setup-node@v4
  with:
    node-version: '20'
    cache: 'npm'
```

The security model for third-party Actions is important to understand: an Action runs with full access to your runner's environment, including any secrets in scope. The safe practice is pinning Actions to a specific commit SHA rather than a floating tag, so you are not silently pulling updated code on every run:

```yaml
# Pinned to commit SHA — safe
- uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683  # v4.2.2

# Pinned to tag — trust the publisher's tag security
- uses: actions/checkout@v4

# Floating — never do this in production
- uses: actions/checkout@main
```

GitHub has a **verified creator** badge for trusted publishers (Microsoft, AWS, Google, HashiCorp, etc.) whose Actions have been reviewed. For Actions from unknown publishers, reading the source code before using them is not optional in a security-conscious team.

---

## Security: OIDC and the keyless credentials story

This is where GitHub Actions has a meaningful advantage that is often underappreciated.

**OpenID Connect (OIDC)** allows GitHub Actions to authenticate to AWS, Azure, and GCP without storing any long-lived credentials. Instead, the workflow requests a short-lived JWT from GitHub's OIDC provider and exchanges it for cloud credentials at runtime:

```yaml
permissions:
  id-token: write   # required for OIDC
  contents: read

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
    - name: Configure AWS credentials via OIDC
      uses: aws-actions/configure-aws-credentials@v4
      with:
        role-to-assume: arn:aws:iam::123456789012:role/GitHubActionsDeployRole
        aws-region: eu-west-1
        # No access keys. No secrets. Credentials are ephemeral per job.

    - name: Push to ECR
      run: |
        aws ecr get-login-password | docker login --username AWS --password-stdin \
          123456789012.dkr.ecr.eu-west-1.amazonaws.com
```

The AWS role is configured to trust GitHub's OIDC provider and scoped to specific repositories and branches:

```json
{
  "Condition": {
    "StringEquals": {
      "token.actions.githubusercontent.com:sub":
        "repo:your-org/your-repo:ref:refs/heads/main"
    }
  }
}
```

No AWS access key is stored anywhere — not in GitHub secrets, not in a `.env` file, not in a shared credential store. If a workflow is compromised, the attacker gets a short-lived token scoped to one role, not a persistent key.

Jenkins can be configured with OIDC through plugins and external systems, but it is not a first-class feature. The standard Jenkins pattern is storing credentials in the Jenkins credential store — long-lived API keys, service account tokens, and kubeconfig files that persist until someone rotates them. This is not inherently insecure, but it is a larger credential footprint with more rotation overhead.

GitHub Actions also provides granular `permissions` on the built-in `GITHUB_TOKEN`:

```yaml
permissions:
  contents: read       # read repository contents
  packages: write      # push to GitHub Container Registry
  pull-requests: write # post PR comments
  id-token: write      # OIDC
  # everything else defaults to none
```

Least-privilege per workflow, declared in the file itself.

---

## Cost: what you pay and to whom

**GitHub Actions hosted runners** are billed per minute of execution:

| Runner | Cost per minute |
|---|---|
| ubuntu-latest (2 CPU, 7 GB) | $0.008 |
| ubuntu-latest 4-core | $0.016 |
| ubuntu-latest 8-core | $0.032 |
| windows-latest | $0.016 |
| macos-latest | $0.08 |

Private repositories include 2,000 free minutes per month on the free plan. Public repositories get unlimited free minutes.

**Self-hosted runners** are free — you pay only for the infrastructure running them. This is the path for teams with high build volumes where per-minute billing becomes significant, or for teams running GPU workloads, large instance types, or builds that need access to private networks.

**Jenkins** costs nothing in software licensing. The real cost is:

- **Infrastructure** — controller node (recommend dedicated VM or deployment, minimum 4 CPU / 8 GB RAM for anything serious), agent capacity, storage for artifacts and build history
- **Engineering time** — upgrades, plugin management, backup configuration, HA setup, shared library maintenance. For a team that takes Jenkins seriously, this is a meaningful fraction of a platform engineer's time.
- **Incident cost** — a Jenkins controller outage blocks all pipelines. Recovery from a corrupted Jenkins home directory without recent backups is a bad day.

The crossover point varies, but teams running more than ~50,000 build minutes per month often find self-hosted runners (GitHub Actions) or dedicated Jenkins agents cost-comparable once engineering time is factored in.

---

## Multi-pipeline management at scale

Both tools face the problem of managing dozens or hundreds of pipelines across many repositories.

**GitHub Actions** addresses this with **reusable workflows** — workflows that can be called from other workflows, like a shared pipeline template:

```yaml
# .github/workflows/deploy.yml — reusable workflow in your platform repo
on:
  workflow_call:
    inputs:
      environment:
        required: true
        type: string
    secrets:
      deploy-token:
        required: true

jobs:
  deploy:
    runs-on: ubuntu-latest
    environment: ${{ inputs.environment }}
    steps:
    - name: Deploy
      run: ./deploy.sh ${{ inputs.environment }}
      env:
        TOKEN: ${{ secrets.deploy-token }}
```

```yaml
# Called from any other repository's workflow
jobs:
  deploy-prod:
    uses: your-org/platform-repo/.github/workflows/deploy.yml@main
    with:
      environment: production
    secrets:
      deploy-token: ${{ secrets.DEPLOY_TOKEN }}
```

**Jenkins** uses **shared libraries** — Groovy code in a separate repository, imported into Jenkinsfiles. This works well but requires more discipline: the shared library is a dependency that every consuming Jenkinsfile must version-pin, and breaking changes in the library can affect many pipelines simultaneously.

---

## How to choose

**Choose GitHub Actions if:**

- Your code is on GitHub — the native integration eliminates webhook setup, authentication, and SCM polling entirely
- You want OIDC-based keyless cloud authentication — the security model is materially better than long-lived credentials in a credential store
- Your team should be focused on product, not CI/CD infrastructure — hosted runners mean zero infrastructure to maintain for most workloads
- Build volumes are moderate — hosted runner per-minute costs are reasonable below significant scale
- You want pipeline-as-code that is reviewable in pull requests like any other change

**Choose Jenkins if:**

- Your source control is not GitHub — Jenkins integrates with any SCM via webhooks or polling
- Compliance or air-gap requirements prevent shipping code or build logs to a third-party SaaS
- You have very high build volumes where per-minute hosted runner billing becomes significant and self-hosted runners on your own infrastructure are more cost-effective
- You have a dedicated platform team that can maintain Jenkins and derives value from its flexibility and plugin depth
- You have existing investment in Groovy shared libraries that would take significant effort to migrate

{{< callout type="note" title="GitHub Actions on self-hosted runners" >}}
The two choices are not mutually exclusive. GitHub Actions with self-hosted runners (deployed via Actions Runner Controller in Kubernetes) gives you GitHub Actions' YAML syntax, OIDC authentication, and Marketplace ecosystem while running builds on your own infrastructure. This is a common path for teams that want Actions' developer experience without per-minute billing or data residency concerns.
{{< /callout >}}

---

## Related reading

- [GitHub Actions documentation](https://docs.github.com/en/actions)
- [Actions Runner Controller — self-hosted runners in Kubernetes](https://github.com/actions/actions-runner-controller)
- [Configuring OIDC with AWS](https://docs.github.com/en/actions/security-for-github-actions/security-hardening-your-deployments/configuring-openid-connect-in-amazon-web-services)
- [Jenkins documentation](https://www.jenkins.io/doc/)
- [Jenkins Kubernetes plugin](https://plugins.jenkins.io/kubernetes/)
- How we caught a supply chain attack in our CI/CD pipeline → **Articles**
- Flux vs ArgoCD — how to choose for your team → **Articles**
