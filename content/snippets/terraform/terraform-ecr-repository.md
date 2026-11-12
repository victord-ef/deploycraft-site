---
title: "Terraform — ECR Repository with Lifecycle Policy and Image Scanning"
date: 2026-11-12
description: "Create a private ECR repository with immutable image tags, enhanced vulnerability scanning on push, a lifecycle policy to control image retention, and a cross-account pull policy."
lang: "Terraform"
tags: ["terraform", "aws", "ecr", "docker", "containers", "security", "scanning", "lifecycle"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-11-12"
draft: false
---

## When to use this

When EKS workloads or CI pipelines push and pull container images. ECR by default has no image scanning, no tag immutability, and no lifecycle policy — untagged images accumulate indefinitely, tagged images can be overwritten silently, and vulnerabilities in shipped images are invisible until something breaks.

## Snippet

```hcl
# ecr.tf

resource "aws_ecr_repository" "this" {
  name                 = var.repository_name
  image_tag_mutability = "IMMUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }

  encryption_configuration {
    encryption_type = "AES256"
  }

  tags = {
    Service   = var.repository_name
    ManagedBy = "terraform"
  }
}

# Lifecycle policy — keep the last N tagged releases, expire untagged images fast
resource "aws_ecr_lifecycle_policy" "this" {
  repository = aws_ecr_repository.this.name

  policy = jsonencode({
    rules = [
      {
        rulePriority = 1
        description  = "Expire untagged images after 7 days"
        selection = {
          tagStatus   = "untagged"
          countType   = "sinceImagePushed"
          countUnit   = "days"
          countNumber = 7
        }
        action = { type = "expire" }
      },
      {
        rulePriority = 2
        description  = "Keep last ${var.tagged_image_count} tagged images"
        selection = {
          tagStatus     = "tagged"
          tagPrefixList = ["v", "sha-"]
          countType     = "imageCountMoreThan"
          countNumber   = var.tagged_image_count
        }
        action = { type = "expire" }
      }
    ]
  })
}

# Repository policy — allow cross-account pull from EKS node role
resource "aws_ecr_repository_policy" "this" {
  count      = length(var.pull_account_ids) > 0 ? 1 : 0
  repository = aws_ecr_repository.this.name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "AllowCrossAccountPull"
        Effect = "Allow"
        Principal = {
          AWS = [
            for account_id in var.pull_account_ids :
            "arn:aws:iam::${account_id}:root"
          ]
        }
        Action = [
          "ecr:GetDownloadUrlForLayer",
          "ecr:BatchGetImage",
          "ecr:BatchCheckLayerAvailability",
        ]
      }
    ]
  })
}
```

### variables.tf

```hcl
variable "repository_name" {
  type        = string
  description = "ECR repository name. Typically matches the application name."
}

variable "tagged_image_count" {
  type        = number
  description = "Number of tagged images to retain per prefix before expiry."
  default     = 20
}

variable "pull_account_ids" {
  type        = list(string)
  description = "AWS account IDs allowed to pull images (e.g. a separate EKS account)."
  default     = []
}
```

### outputs.tf

```hcl
output "repository_url" {
  value       = aws_ecr_repository.this.repository_url
  description = "Full ECR URL — use as the image registry in Kubernetes manifests and CI."
}

output "repository_arn" {
  value       = aws_ecr_repository.this.arn
  description = "Repository ARN — use in IAM policies granting push access."
}
```

### IAM policy for CI push access

```hcl
# Attach to the IAM role used by your CI runner
data "aws_iam_policy_document" "ecr_push" {
  statement {
    effect = "Allow"
    actions = [
      "ecr:GetAuthorizationToken",
    ]
    resources = ["*"]   # GetAuthorizationToken is not resource-scoped
  }

  statement {
    effect = "Allow"
    actions = [
      "ecr:BatchCheckLayerAvailability",
      "ecr:CompleteLayerUpload",
      "ecr:InitiateLayerUpload",
      "ecr:PutImage",
      "ecr:UploadLayerPart",
      "ecr:DescribeImages",
    ]
    resources = [aws_ecr_repository.this.arn]
  }
}

resource "aws_iam_policy" "ecr_push" {
  name   = "${var.repository_name}-ecr-push"
  policy = data.aws_iam_policy_document.ecr_push.json
}
```

### GitHub Actions — authenticate and push

```yaml
- name: Configure AWS credentials
  uses: aws-actions/configure-aws-credentials@v4
  with:
    role-to-assume: ${{ secrets.CI_ROLE_ARN }}
    aws-region: us-east-1

- name: Log in to ECR
  id: login-ecr
  uses: aws-actions/amazon-ecr-login@v2

- name: Build and push
  env:
    REGISTRY: ${{ steps.login-ecr.outputs.registry }}
    REPOSITORY: my-app
    IMAGE_TAG: sha-${{ github.sha }}
  run: |
    docker build -t $REGISTRY/$REPOSITORY:$IMAGE_TAG .
    docker push $REGISTRY/$REPOSITORY:$IMAGE_TAG
```

**Key decisions:**

| Decision | Why it matters |
|---|---|
| `IMMUTABLE` image tags | Prevents any push from silently overwriting an existing tag. A `sha-abc123` tag in a Kubernetes manifest always refers to the same image bytes — no silent supply-chain substitution. |
| `scan_on_push = true` | Triggers an Enhanced Scanning (Inspector) scan on every push. Findings appear in the ECR console and can be queried via the AWS Security Hub or EventBridge for automated gates. |
| Untagged expiry at 7 days (rule priority 1) | CI builds produce untagged layer cache images continuously. Without expiry they accumulate for years. Priority 1 ensures this rule evaluates before the tagged retention rule. |
| `tagPrefixList = ["v", "sha-"]` | Applies retention only to versioned and SHA-tagged images — the ones actually deployed. Avoids expiring `latest` or other utility tags unintentionally. |
| `GetAuthorizationToken` on `"*"` | This action retrieves a temporary Docker login token and is not resource-scoped in IAM — AWS requires `"*"` as the resource. All image-level actions are scoped to the specific repository ARN. |
| Cross-account pull via repository policy | In a multi-account setup (CI account pushes, production account pulls), the repository policy on the source account allows the production account's node role to pull without copying images across accounts. |

## Verify it worked

```bash
terraform init && terraform apply

# Confirm immutability
aws ecr describe-repositories \
  --repository-names <repo-name> \
  --query 'repositories[0].imageTagMutability'
# Expected: "IMMUTABLE"

# Confirm scanning is enabled
aws ecr describe-repositories \
  --repository-names <repo-name> \
  --query 'repositories[0].imageScanningConfiguration'

# Push a test image and check for scan findings
docker tag alpine:latest <account>.dkr.ecr.<region>.amazonaws.com/<repo>:test
docker push <account>.dkr.ecr.<region>.amazonaws.com/<repo>:test

aws ecr describe-image-scan-findings \
  --repository-name <repo-name> \
  --image-id imageTag=test \
  --query 'imageScanFindings.findingSeverityCounts'
```
