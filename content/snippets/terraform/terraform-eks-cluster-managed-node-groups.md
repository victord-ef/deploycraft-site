---
title: "Terraform — EKS Cluster with Managed Node Groups"
date: 2026-11-12
description: "Provision an EKS cluster with a managed node group, OIDC provider for IRSA, cluster add-ons, and the minimum IAM permissions required — ready to accept workloads."
lang: "Terraform"
tags: ["terraform", "aws", "eks", "kubernetes", "managed-node-groups", "oidc", "iam"]
categories: ["snippet"]
maturity: "battle-tested"
verified: "2026-11-12"
draft: false
---

## When to use this

When provisioning an EKS cluster from scratch. This configuration creates a production-ready cluster: a managed node group with launch template control, an OIDC provider for IRSA (so workloads can assume IAM roles without static credentials), and the core add-ons that every cluster needs.

## Prerequisites

- A VPC with private subnets. The cluster API endpoint and nodes are placed in private subnets — use the VPC snippet or `data` sources to reference an existing VPC.
- AWS provider `~> 5.0` and Terraform `>= 1.5`.

## Snippet

### eks.tf

```hcl
locals {
  cluster_name = "${var.project}-${var.environment}"
}

# ── Cluster IAM role ──────────────────────────────────────────────────────────
data "aws_iam_policy_document" "cluster_assume" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["eks.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "cluster" {
  name               = "${local.cluster_name}-cluster"
  assume_role_policy = data.aws_iam_policy_document.cluster_assume.json
  tags               = local.tags
}

resource "aws_iam_role_policy_attachment" "cluster_policy" {
  role       = aws_iam_role.cluster.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"
}

# ── EKS Cluster ───────────────────────────────────────────────────────────────
resource "aws_eks_cluster" "this" {
  name     = local.cluster_name
  version  = var.kubernetes_version
  role_arn = aws_iam_role.cluster.arn

  vpc_config {
    subnet_ids              = var.private_subnet_ids
    endpoint_private_access = true
    endpoint_public_access  = var.public_endpoint   # false for fully private clusters
    public_access_cidrs     = var.public_endpoint ? var.allowed_cidrs : []
  }

  enabled_cluster_log_types = ["api", "audit", "authenticator", "controllerManager", "scheduler"]

  tags = local.tags

  depends_on = [aws_iam_role_policy_attachment.cluster_policy]
}

# ── OIDC Provider (required for IRSA) ────────────────────────────────────────
data "tls_certificate" "eks" {
  url = aws_eks_cluster.this.identity[0].oidc[0].issuer
}

resource "aws_iam_openid_connect_provider" "this" {
  client_id_list  = ["sts.amazonaws.com"]
  thumbprint_list = [data.tls_certificate.eks.certificates[0].sha1_fingerprint]
  url             = aws_eks_cluster.this.identity[0].oidc[0].issuer
  tags            = local.tags
}

# ── Node group IAM role ───────────────────────────────────────────────────────
data "aws_iam_policy_document" "node_assume" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["ec2.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "node" {
  name               = "${local.cluster_name}-node"
  assume_role_policy = data.aws_iam_policy_document.node_assume.json
  tags               = local.tags
}

resource "aws_iam_role_policy_attachment" "node_policies" {
  for_each = toset([
    "arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy",
    "arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy",
    "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly",
  ])

  role       = aws_iam_role.node.name
  policy_arn = each.value
}

# ── Managed Node Group ────────────────────────────────────────────────────────
resource "aws_eks_node_group" "this" {
  cluster_name    = aws_eks_cluster.this.name
  node_group_name = "general"
  node_role_arn   = aws_iam_role.node.arn
  subnet_ids      = var.private_subnet_ids

  instance_types = var.node_instance_types
  capacity_type  = "ON_DEMAND"

  scaling_config {
    desired_size = var.node_desired
    min_size     = var.node_min
    max_size     = var.node_max
  }

  update_config {
    max_unavailable_percentage = 25
  }

  launch_template {
    id      = aws_launch_template.node.id
    version = aws_launch_template.node.latest_version
  }

  tags = local.tags

  depends_on = [aws_iam_role_policy_attachment.node_policies]

  lifecycle {
    ignore_changes = [scaling_config[0].desired_size]
  }
}

# ── Launch template — custom user data, IMDSv2 enforcement ───────────────────
resource "aws_launch_template" "node" {
  name_prefix = "${local.cluster_name}-node-"

  metadata_options {
    http_endpoint               = "enabled"
    http_tokens                 = "required"   # enforces IMDSv2
    http_put_response_hop_limit = 1            # prevents pods reaching IMDS
  }

  tag_specifications {
    resource_type = "instance"
    tags          = merge(local.tags, { Name = "${local.cluster_name}-node" })
  }
}

# ── Core add-ons ──────────────────────────────────────────────────────────────
resource "aws_eks_addon" "addons" {
  for_each = {
    vpc-cni            = "v1.18.1-eksbuild.1"
    coredns            = "v1.11.1-eksbuild.9"
    kube-proxy         = "v1.31.0-eksbuild.5"
    aws-ebs-csi-driver = "v1.35.0-eksbuild.1"
  }

  cluster_name             = aws_eks_cluster.this.name
  addon_name               = each.key
  addon_version            = each.value
  resolve_conflicts_on_update = "OVERWRITE"

  tags = local.tags
}

locals {
  tags = {
    Project     = var.project
    Environment = var.environment
    ManagedBy   = "terraform"
  }
}
```

### variables.tf

```hcl
variable "project"            { type = string }
variable "environment"        { type = string }
variable "kubernetes_version" { type = string; default = "1.31" }
variable "private_subnet_ids" { type = list(string) }
variable "public_endpoint"    { type = bool;   default = false }
variable "allowed_cidrs"      { type = list(string); default = [] }
variable "node_instance_types" { type = list(string); default = ["m6i.large"] }
variable "node_desired"       { type = number; default = 2 }
variable "node_min"           { type = number; default = 1 }
variable "node_max"           { type = number; default = 5 }
```

### outputs.tf

```hcl
output "cluster_name"          { value = aws_eks_cluster.this.name }
output "cluster_endpoint"      { value = aws_eks_cluster.this.endpoint }
output "cluster_ca_cert"       { value = aws_eks_cluster.this.certificate_authority[0].data }
output "oidc_provider_arn"     { value = aws_iam_openid_connect_provider.this.arn }
output "node_role_arn"         { value = aws_iam_role.node.arn }
```

**Key decisions:**

| Decision | Why it matters |
|---|---|
| `endpoint_private_access = true` | Nodes communicate with the API server over the private VPC network. Without this, node-to-API traffic leaves the VPC even when both are in the same VPC. |
| `http_tokens = required` on IMDS | Enforces IMDSv2 on every node. IMDSv1 allows any process (including a compromised pod) to query instance metadata without a session token — a common lateral movement path. |
| `http_put_response_hop_limit = 1` | IMDSv2 tokens are non-routable beyond one hop. With the default limit of 2, a pod inside a container can still reach IMDS. Setting 1 restricts IMDS access to the host network only. |
| `ignore_changes = [desired_size]` | Cluster Autoscaler or Karpenter modifies `desired_size` at runtime. Without this, the next `terraform apply` resets the node count to the Terraform-declared value, fighting the autoscaler. |
| `for_each` on node IAM policies | Cleaner than three separate `aws_iam_role_policy_attachment` resources. Adding or removing a policy is a one-line change in the `toset()` list. |
| `resolve_conflicts_on_update = OVERWRITE` | Prevents add-on updates from failing when the add-on configuration diverges from the managed default. Required for smooth version upgrades in CI. |
| All control plane log types enabled | Audit logs are the primary forensic source for a Kubernetes incident. Enabling them at cluster creation costs pennies and cannot be retroactively added after a breach. |

## Verify it worked

```bash
terraform init && terraform apply

# Update kubeconfig
aws eks update-kubeconfig --name <cluster-name> --region <region>

# Confirm nodes are Ready
kubectl get nodes

# Confirm add-ons are active
aws eks list-addons --cluster-name <cluster-name>

# Confirm IMDSv2 is enforced (from inside a node)
# Should return 401 — token required
curl -s http://169.254.169.254/latest/meta-data/instance-id
```

## Full walkthrough

Securing EKS workload identity end-to-end with IRSA, Pod Identity, and least-privilege IAM → see the [IRSA snippet](/snippets/terraform/terraform-iam-role-irsa-eks/) for the next step after cluster creation.
