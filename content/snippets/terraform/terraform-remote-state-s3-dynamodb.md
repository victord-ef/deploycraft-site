---
title: "Terraform Remote State with S3 and DynamoDB Locking"
date: 2026-11-12
description: "Store Terraform state in S3 with server-side encryption and versioning, use DynamoDB for state locking to prevent concurrent applies, and bootstrap the backend resources with a local state first-apply pattern."
lang: "Terraform"
tags: ["terraform", "aws", "s3", "dynamodb", "remote-state", "backend", "locking"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-11-12"
draft: false
---

## When to use this

As soon as more than one person or CI pipeline runs `terraform apply` against the same infrastructure. Local state files cause split-brain: two applies running simultaneously corrupt state and leave resources in an unknown configuration. Remote state with locking serialises applies and makes state available to every team member and pipeline without sharing files.

## Without it

```
# Two engineers apply simultaneously
Engineer A: terraform apply  → reads state, plans, starts applying
Engineer B: terraform apply  → reads same state, plans the same changes
                              → both apply, state is now corrupted
```

## Snippet

### Step 1 — Bootstrap the backend resources (local state)

Create the S3 bucket and DynamoDB table using local state first. Once they exist, migrate state to them.

```hcl
# bootstrap/main.tf
# Run this once with local state, then never touch it again.
# Do NOT use this file's resources as the backend for itself.

terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.region
}

locals {
  name = "${var.project}-${var.environment}-tfstate"
}

resource "aws_s3_bucket" "state" {
  bucket = local.name

  tags = {
    Purpose     = "terraform-state"
    Environment = var.environment
    ManagedBy   = "terraform"
  }
}

resource "aws_s3_bucket_versioning" "state" {
  bucket = aws_s3_bucket.state.id
  versioning_configuration { status = "Enabled" }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "state" {
  bucket = aws_s3_bucket.state.id
  rule {
    apply_server_side_encryption_by_default { sse_algorithm = "AES256" }
    bucket_key_enabled = true
  }
}

resource "aws_s3_bucket_public_access_block" "state" {
  bucket                  = aws_s3_bucket.state.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_lifecycle_configuration" "state" {
  bucket     = aws_s3_bucket.state.id
  depends_on = [aws_s3_bucket_versioning.state]

  rule {
    id     = "expire-old-state-versions"
    status = "Enabled"
    noncurrent_version_expiration { noncurrent_days = 90 }
  }
}

resource "aws_dynamodb_table" "lock" {
  name         = "${local.name}-lock"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "LockID"

  attribute {
    name = "LockID"
    type = "S"
  }

  tags = {
    Purpose     = "terraform-state-lock"
    Environment = var.environment
    ManagedBy   = "terraform"
  }
}

output "state_bucket" { value = aws_s3_bucket.state.bucket }
output "lock_table"   { value = aws_dynamodb_table.lock.name }
```

### Step 2 — Configure the backend in your main configuration

```hcl
# main/backend.tf
terraform {
  backend "s3" {
    bucket         = "my-project-production-tfstate"
    key            = "my-project/production/terraform.tfstate"
    region         = "us-east-1"
    encrypt        = true
    dynamodb_table = "my-project-production-tfstate-lock"
  }

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}
```

### Separate state per environment using the `key` path

```hcl
# staging — same bucket, different key
backend "s3" {
  bucket         = "my-project-production-tfstate"
  key            = "my-project/staging/terraform.tfstate"
  region         = "us-east-1"
  encrypt        = true
  dynamodb_table = "my-project-production-tfstate-lock"
}
```

### Read outputs from another state file

```hcl
# Reference outputs from a separate configuration (e.g. VPC outputs consumed by EKS)
data "terraform_remote_state" "vpc" {
  backend = "s3"
  config = {
    bucket = "my-project-production-tfstate"
    key    = "my-project/production/vpc/terraform.tfstate"
    region = "us-east-1"
  }
}

resource "aws_eks_cluster" "this" {
  name = "my-cluster"
  vpc_config {
    subnet_ids = data.terraform_remote_state.vpc.outputs.private_subnet_ids
  }
}
```

**Key decisions:**

| Decision | Why it matters |
|---|---|
| Bootstrap with local state | The state backend resources cannot store their own state — a chicken-and-egg problem. Create them with local state once, then `terraform init` in the main config migrates to S3 automatically. |
| `encrypt = true` in backend config | Enforces SSE at the Terraform level on every state write, independent of the bucket's default encryption setting. |
| `PAY_PER_REQUEST` on DynamoDB | The lock table is written only during active applies — often once per hour or less. Provisioned capacity wastes money on idle throughput. |
| `key` path includes project and environment | A single bucket holds state for multiple configurations, separated by key prefix. A flat `terraform.tfstate` key makes bucket reuse across projects impossible. |
| 90-day noncurrent version expiration | Every `terraform apply` adds a new state version. Without expiration, the bucket grows indefinitely. 90 days covers any reasonable rollback window. |
| `terraform_remote_state` for cross-stack references | Outputs from one configuration are consumed by another without hard-coding ARNs that drift when the upstream stack is rebuilt. |

## Verify it worked

```bash
# Bootstrap
cd bootstrap && terraform init && terraform apply

# Initialise main config — Terraform migrates local state to S3 automatically
cd ../main && terraform init

# Confirm state file is in S3
aws s3 ls s3://my-project-production-tfstate/my-project/production/

# Confirm lock table exists
aws dynamodb describe-table \
  --table-name my-project-production-tfstate-lock \
  --query 'Table.TableStatus'
# Expected: "ACTIVE"
```
