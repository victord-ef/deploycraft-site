---
title: "AWS CLI"
description: "AWS CLI commands for IAM, EC2, S3, EKS, ECR, Secrets Manager, and cross-account operations."
icon: "☁️"
weight: 10
count: 50
tags: ["aws", "cloud", "devops"]
---

## Auth & Config

```bash
aws configure
aws configure --profile prod
aws configure list
aws configure list-profiles
aws sts get-caller-identity
aws sts get-caller-identity --profile prod
aws sts assume-role --role-arn arn:aws:iam::123456789:role/MyRole --role-session-name session1
```

## IAM

```bash
aws iam list-users
aws iam get-user --user-name <user>
aws iam list-groups
aws iam list-roles
aws iam get-role --role-name <role>
aws iam list-attached-user-policies --user-name <user>
aws iam list-attached-role-policies --role-name <role>
aws iam get-policy --policy-arn <arn>
aws iam simulate-principal-policy --policy-source-arn <arn> --action-names s3:GetObject
aws iam create-access-key --user-name <user>
aws iam delete-access-key --access-key-id <id> --user-name <user>
```

## EC2

```bash
aws ec2 describe-instances
aws ec2 describe-instances --filters "Name=instance-state-name,Values=running"
aws ec2 describe-instances --query 'Reservations[*].Instances[*].[InstanceId,PublicIpAddress,Tags[?Key==`Name`].Value]' --output table
aws ec2 start-instances --instance-ids <id>
aws ec2 stop-instances --instance-ids <id>
aws ec2 terminate-instances --instance-ids <id>
aws ec2 describe-security-groups
aws ec2 describe-vpcs
aws ec2 describe-subnets
aws ec2 describe-key-pairs
aws ec2 create-key-pair --key-name my-key --query 'KeyMaterial' --output text > my-key.pem
aws ec2 describe-images --owners amazon --filters "Name=name,Values=amzn2-ami-hvm-*"
```

## S3

```bash
aws s3 ls
aws s3 ls s3://<bucket>
aws s3 ls s3://<bucket>/<prefix>/ --recursive
aws s3 cp file.txt s3://<bucket>/
aws s3 cp s3://<bucket>/file.txt ./
aws s3 cp s3://<bucket>/ . --recursive
aws s3 sync ./local/ s3://<bucket>/remote/
aws s3 sync s3://<bucket>/ ./local/
aws s3 mv s3://<bucket>/old.txt s3://<bucket>/new.txt
aws s3 rm s3://<bucket>/file.txt
aws s3 rm s3://<bucket>/ --recursive
aws s3 mb s3://<bucket>
aws s3 rb s3://<bucket> --force
aws s3 presign s3://<bucket>/file.txt --expires-in 3600
aws s3api get-bucket-acl --bucket <bucket>
aws s3api get-bucket-policy --bucket <bucket>
aws s3api list-buckets --query 'Buckets[*].Name'
```

## EKS

```bash
aws eks list-clusters
aws eks describe-cluster --name <cluster>
aws eks update-kubeconfig --name <cluster> --region <region>
aws eks update-kubeconfig --name <cluster> --region <region> --profile prod
aws eks list-nodegroups --cluster-name <cluster>
aws eks describe-nodegroup --cluster-name <cluster> --nodegroup-name <ng>
aws eks list-fargate-profiles --cluster-name <cluster>
```

## ECR

```bash
aws ecr get-login-password --region <region> | docker login --username AWS --password-stdin <account>.dkr.ecr.<region>.amazonaws.com
aws ecr describe-repositories
aws ecr list-images --repository-name <repo>
aws ecr create-repository --repository-name <repo>
aws ecr delete-repository --repository-name <repo> --force
aws ecr describe-images --repository-name <repo>
aws ecr batch-delete-image --repository-name <repo> --image-ids imageTag=old-tag
```

## Secrets Manager & SSM

```bash
aws secretsmanager list-secrets
aws secretsmanager get-secret-value --secret-id <name>
aws secretsmanager get-secret-value --secret-id <name> --query SecretString --output text
aws secretsmanager create-secret --name <name> --secret-string '{"key":"value"}'
aws secretsmanager update-secret --secret-id <name> --secret-string '{"key":"new"}'

aws ssm get-parameter --name <name>
aws ssm get-parameter --name <name> --with-decryption
aws ssm get-parameters-by-path --path /app/prod/ --recursive --with-decryption
aws ssm put-parameter --name <name> --value <value> --type SecureString
```

## CloudWatch Logs

```bash
aws logs describe-log-groups
aws logs describe-log-streams --log-group-name <group>
aws logs get-log-events --log-group-name <group> --log-stream-name <stream>
aws logs filter-log-events --log-group-name <group> --filter-pattern "ERROR"
aws logs tail <group> --follow
```
