---
title: "Terraform"
description: "Terraform workflow, state management, workspace, import, and debugging commands."
icon: "🏗️"
weight: 3
count: 40
tags: ["terraform", "iac", "infrastructure"]
---

## Core Workflow

```bash
terraform init
terraform init -upgrade                         # upgrade providers
terraform init -backend-config=backend.hcl
terraform validate
terraform fmt
terraform fmt -recursive
terraform plan
terraform plan -out=tfplan
terraform plan -var="region=us-east-1"
terraform plan -var-file=prod.tfvars
terraform plan -target=aws_instance.web
terraform apply
terraform apply tfplan
terraform apply -auto-approve
terraform apply -target=aws_instance.web
terraform destroy
terraform destroy -target=aws_instance.web
terraform destroy -auto-approve
```

## State

```bash
terraform state list
terraform state show <resource>
terraform state show 'aws_instance.web[0]'
terraform state pull                            # download and print state
terraform state push terraform.tfstate
terraform state mv <source> <destination>
terraform state rm <resource>
terraform state rm 'module.vpc'
terraform refresh                               # reconcile state with real infra
```

## Workspace

```bash
terraform workspace list
terraform workspace new <name>
terraform workspace select <name>
terraform workspace show
terraform workspace delete <name>
```

## Output & Show

```bash
terraform output
terraform output <name>
terraform output -json
terraform show
terraform show -json tfplan
terraform graph | dot -Tsvg > graph.svg
```

## Import

```bash
terraform import aws_instance.web i-1234567890abcdef0
terraform import 'aws_security_group.sg["name"]' sg-12345678
```

## Providers & Modules

```bash
terraform providers
terraform providers lock
terraform get                                   # download modules
terraform get -update
```

## Debugging

```bash
TF_LOG=DEBUG terraform apply
TF_LOG=TRACE terraform plan 2>&1 | tee tf.log
TF_VAR_region=us-east-1 terraform plan
terraform force-unlock <lock-id>
```
