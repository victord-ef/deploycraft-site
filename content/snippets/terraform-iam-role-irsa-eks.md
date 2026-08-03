---
title: "Terraform — IAM Role with IRSA for EKS Workloads"
date: 2026-07-25
description: "Create an IAM role that EKS pods can assume via IRSA (IAM Roles for Service Accounts), giving workloads scoped AWS permissions without static credentials."
lang: "Terraform"
tags: ["terraform", "aws", "iam", "irsa", "eks", "security"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-07-25"
draft: false
---

## When to use this

Any time a pod running on EKS needs to call an AWS API — reading from S3, writing to SQS, querying DynamoDB. IRSA binds an IAM role to a Kubernetes ServiceAccount so individual pods get scoped credentials that rotate automatically, with no secrets to manage.

## Without it

```yaml
# Common anti-pattern — static credentials mounted as a Secret
apiVersion: v1
kind: Secret
metadata:
  name: aws-credentials
stringData:
  AWS_ACCESS_KEY_ID: "AKIA..."
  AWS_SECRET_ACCESS_KEY: "..."
```

Static keys do not rotate, are visible in etcd, appear in `kubectl describe`, and grant the same permissions to every pod that mounts them regardless of which workload it is. A single compromised pod exposes credentials for the entire cluster.

## Snippet

### irsa.tf

```hcl
# Fetch the OIDC provider details from the EKS cluster
data "aws_eks_cluster" "this" {
  name = var.cluster_name
}

data "aws_iam_openid_connect_provider" "this" {
  url = data.aws_eks_cluster.this.identity[0].oidc[0].issuer
}

locals {
  oidc_provider_arn = data.aws_iam_openid_connect_provider.this.arn
  # Strip the https:// prefix — required for the trust policy condition
  oidc_provider_id  = replace(
    data.aws_eks_cluster.this.identity[0].oidc[0].issuer,
    "https://",
    ""
  )
}

# Trust policy — only the specified ServiceAccount in the specified namespace
# can assume this role
data "aws_iam_policy_document" "irsa_trust" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = [local.oidc_provider_arn]
    }

    condition {
      test     = "StringEquals"
      variable = "${local.oidc_provider_id}:sub"
      values   = ["system:serviceaccount:${var.namespace}:${var.service_account_name}"]
    }

    condition {
      test     = "StringEquals"
      variable = "${local.oidc_provider_id}:aud"
      values   = ["sts.amazonaws.com"]
    }
  }
}

# IAM role with the IRSA trust policy
resource "aws_iam_role" "this" {
  name               = "${var.cluster_name}-${var.service_account_name}-irsa"
  assume_role_policy = data.aws_iam_policy_document.irsa_trust.json

  tags = {
    Cluster        = var.cluster_name
    Namespace      = var.namespace
    ServiceAccount = var.service_account_name
    ManagedBy      = "terraform"
  }
}

# Attach the permissions policy — scope this to exactly what the workload needs
resource "aws_iam_role_policy" "this" {
  name = "workload-permissions"
  role = aws_iam_role.this.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "S3ReadWrite"
        Effect = "Allow"
        Action = [
          "s3:GetObject",
          "s3:PutObject",
          "s3:DeleteObject",
          "s3:ListBucket"
        ]
        Resource = [
          var.s3_bucket_arn,
          "${var.s3_bucket_arn}/*"
        ]
      }
    ]
  })
}

# Kubernetes ServiceAccount — annotated with the IAM role ARN
resource "kubernetes_service_account" "this" {
  metadata {
    name      = var.service_account_name
    namespace = var.namespace
    annotations = {
      "eks.amazonaws.com/role-arn" = aws_iam_role.this.arn
    }
  }
}
```

### variables.tf

```hcl
variable "cluster_name" {
  type        = string
  description = "EKS cluster name."
}

variable "namespace" {
  type        = string
  description = "Kubernetes namespace the ServiceAccount lives in."
}

variable "service_account_name" {
  type        = string
  description = "Kubernetes ServiceAccount name to bind to the IAM role."
}

variable "s3_bucket_arn" {
  type        = string
  description = "ARN of the S3 bucket the workload needs access to."
}
```

### outputs.tf

```hcl
output "role_arn" {
  value       = aws_iam_role.this.arn
  description = "IAM role ARN — reference this in the pod spec if not using the ServiceAccount annotation."
}

output "service_account_name" {
  value       = kubernetes_service_account.this.metadata[0].name
  description = "ServiceAccount name to set in the pod spec."
}
```

### Pod spec — reference the ServiceAccount

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: my-app
  namespace: my-app
spec:
  serviceAccountName: my-app-sa    # must match var.service_account_name
  containers:
    - name: my-app
      image: my-registry/my-app:1.0.0
      env:
        - name: AWS_REGION
          value: us-east-1
        # AWS_ROLE_ARN and AWS_WEB_IDENTITY_TOKEN_FILE are injected automatically
        # by the EKS pod identity webhook — no manual configuration needed
```

**Key decisions:**

| Decision | Why it matters |
|---|---|
| Two `condition` blocks in trust policy | The `sub` condition pins the role to one specific ServiceAccount. The `aud` condition prevents token reuse across AWS services. Both are required — omitting either opens the role to any pod in the cluster. |
| `StringEquals` not `StringLike` | Using `StringLike` with a wildcard (`system:serviceaccount:*:my-sa`) allows any namespace to assume the role. Always use exact match. |
| `replace()` to strip `https://` | The OIDC issuer URL includes the scheme; the trust policy condition variable must not. Forgetting this causes a silent `AssumeRoleWithWebIdentity` failure. |
| Inline policy over managed policy | The permissions are specific to this workload. An inline policy makes the scope explicit and avoids accidentally attaching a shared policy to the wrong role. |
| `kubernetes_service_account` in Terraform | Creates the ServiceAccount and the IAM role in one apply — no manual `kubectl annotate` step that could be missed or drift over time. |

## Verify it worked

```bash
# Apply
terraform init && terraform apply

# Confirm the trust policy is correct
aws iam get-role --role-name <role-name> \
  --query 'Role.AssumeRolePolicyDocument' --output json

# Confirm the ServiceAccount has the annotation
kubectl get serviceaccount <sa-name> -n <namespace> -o yaml | grep role-arn

# Test credential injection from inside the pod
kubectl run irsa-test --rm -it \
  --image=amazon/aws-cli \
  --serviceaccount=<sa-name> \
  --namespace=<namespace> \
  -- sts get-caller-identity
```

Expected output from `sts get-caller-identity`:

```json
{
  "UserId": "AROA...:botocore-session-...",
  "Account": "123456789012",
  "Arn": "arn:aws:sts::123456789012:assumed-role/<role-name>/botocore-session-..."
}
```

The `Arn` field must reference your IRSA role — not the node instance profile. If it shows the node role, the ServiceAccount annotation is missing or the pod is not using the ServiceAccount.

> **Note:** The EKS cluster must have an OIDC provider associated. If `data.aws_iam_openid_connect_provider.this` errors, create the provider first: `eksctl utils associate-iam-oidc-provider --cluster <name> --approve`.

## Full walkthrough

Securing EKS workload identity end-to-end with IRSA, Pod Identity, and least-privilege IAM → **Tutorial Pair 54: Security & Encryption** *(coming soon)*.
