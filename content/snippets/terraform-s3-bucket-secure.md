---
title: "Terraform — S3 Bucket with Versioning, Encryption, and Block Public Access"
date: 2026-07-25
description: "Production-safe S3 bucket with versioning enabled, AES-256 server-side encryption, all public access blocked, and a lifecycle rule to expire old versions."
lang: "Terraform"
tags: ["terraform", "aws", "s3", "encryption", "security", "versioning"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-07-25"
draft: false
---

## When to use this

Any time you create an S3 bucket that holds application data, backups, Terraform state, or logs. The AWS default for every one of these settings is insecure or off — versioning disabled, no encryption enforced, public access potentially open.

## Without it

```hcl
# The Terraform default — one line, zero protection
resource "aws_s3_bucket" "this" {
  bucket = "my-app-data"
}
```

No encryption means data sits in plaintext. No versioning means a bad `aws s3 rm` or application bug permanently destroys data. No public access block means a misconfigured bucket policy can expose the bucket to the internet — the source of most S3 data breaches.

## Snippet

```hcl
# s3.tf

resource "aws_s3_bucket" "this" {
  bucket = var.bucket_name

  tags = {
    Environment = var.environment
    ManagedBy   = "terraform"
  }
}

# Block all public access — four settings, all must be true
resource "aws_s3_bucket_public_access_block" "this" {
  bucket = aws_s3_bucket.this.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# Server-side encryption with AES-256 (SSE-S3)
# Swap to aws:kms and provide a kms_master_key_id for CMK control
resource "aws_s3_bucket_server_side_encryption_configuration" "this" {
  bucket = aws_s3_bucket.this.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
    bucket_key_enabled = true   # reduces KMS API calls if you switch to aws:kms later
  }
}

# Versioning — keeps every object version on overwrite or delete
resource "aws_s3_bucket_versioning" "this" {
  bucket = aws_s3_bucket.this.id

  versioning_configuration {
    status = "Enabled"
  }
}

# Lifecycle — expire non-current versions after 30 days to control storage cost
resource "aws_s3_bucket_lifecycle_configuration" "this" {
  bucket = aws_s3_bucket.this.id

  depends_on = [aws_s3_bucket_versioning.this]

  rule {
    id     = "expire-old-versions"
    status = "Enabled"

    noncurrent_version_expiration {
      noncurrent_days = 30
    }

    abort_incomplete_multipart_upload {
      days_after_initiation = 7
    }
  }
}

# Enforce TLS-only access via bucket policy
resource "aws_s3_bucket_policy" "this" {
  bucket = aws_s3_bucket.this.id

  depends_on = [aws_s3_bucket_public_access_block.this]

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid       = "DenyNonTLS"
        Effect    = "Deny"
        Principal = "*"
        Action    = "s3:*"
        Resource = [
          aws_s3_bucket.this.arn,
          "${aws_s3_bucket.this.arn}/*"
        ]
        Condition = {
          Bool = {
            "aws:SecureTransport" = "false"
          }
        }
      }
    ]
  })
}
```

### variables.tf

```hcl
variable "bucket_name" {
  type        = string
  description = "Globally unique S3 bucket name."
}

variable "environment" {
  type        = string
  description = "Deployment environment (production, staging, etc.)."
  default     = "production"
}
```

### outputs.tf

```hcl
output "bucket_id" {
  value       = aws_s3_bucket.this.id
  description = "The bucket name, used as a reference in other modules."
}

output "bucket_arn" {
  value       = aws_s3_bucket.this.arn
  description = "The bucket ARN, used in IAM policies."
}
```

**Key decisions:**

| Decision | Why it matters |
|---|---|
| Four public access block settings | AWS requires all four set to `true` for full protection. Setting only `block_public_acls` still allows public bucket policies. |
| `bucket_key_enabled = true` | Reduces per-request KMS API calls by up to 99% if you migrate from AES256 to aws:kms later — zero cost to enable now. |
| `depends_on` on lifecycle config | The lifecycle rule for non-current versions requires versioning to be enabled first. Without this, Terraform may create both in the wrong order and error. |
| `abort_incomplete_multipart_upload` | Orphaned multipart uploads accumulate silently and generate ongoing storage charges. A 7-day abort rule eliminates this. |
| TLS-only bucket policy | Encryption at rest (SSE) does not protect data in transit. The `DenyNonTLS` statement rejects any request over HTTP. |
| `depends_on` on bucket policy | The public access block must be fully applied before attaching a policy — otherwise Terraform may fail with a public policy error even for a private policy. |

## Verify it worked

```bash
# Apply
terraform init && terraform apply

# Confirm public access block
aws s3api get-public-access-block --bucket <bucket-name>

# Confirm encryption is enforced
aws s3api get-bucket-encryption --bucket <bucket-name>

# Confirm versioning is active
aws s3api get-bucket-versioning --bucket <bucket-name>

# Test TLS enforcement — this should fail with 403
aws s3 cp test.txt s3://<bucket-name>/ --no-verify-ssl
```

Expected output from `get-public-access-block`:

```json
{
  "PublicAccessBlockConfiguration": {
    "BlockPublicAcls": true,
    "IgnorePublicAcls": true,
    "BlockPublicPolicy": true,
    "RestrictPublicBuckets": true
  }
}
```

> **Note:** If you use this bucket for Terraform remote state, also enable DynamoDB state locking and set `prevent_destroy = true` in a `lifecycle` block on the bucket resource to block accidental deletion.

## Full walkthrough

Terraform module patterns for secure AWS storage including KMS CMK rotation and cross-account replication → **Tutorial Pair 54: Security & Encryption** *(coming soon)*.
