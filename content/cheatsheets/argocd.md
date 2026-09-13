---
title: "ArgoCD"
description: "ArgoCD CLI commands for application management, sync operations, rollbacks, RBAC, and cluster/repo registration."
icon: "🐙"
weight: 6
count: 40
tags: ["argocd", "gitops", "kubernetes"]
---

## Login & Context

```bash
argocd login <server> --username admin --password <pass>
argocd login <server> --sso
argocd login <server> --insecure                        # skip TLS verify
argocd context
argocd context <name>
argocd logout <server>
argocd account get-user-info
argocd account update-password
```

## Applications

```bash
argocd app list
argocd app get <app>
argocd app get <app> --show-params
argocd app get <app> --show-operation
argocd app diff <app>
argocd app diff <app> --local ./manifests/
argocd app history <app>
argocd app logs <app>
argocd app resources <app>
argocd app manifests <app>
```

## Sync & Refresh

```bash
argocd app sync <app>
argocd app sync <app> --prune
argocd app sync <app> --force
argocd app sync <app> --dry-run
argocd app sync <app> --resource apps:Deployment:my-deploy
argocd app wait <app>
argocd app wait <app> --sync
argocd app wait <app> --health
argocd app refresh <app>
argocd app refresh <app> --hard                         # bypass cache
```

## Rollback

```bash
argocd app rollback <app>
argocd app rollback <app> <revision>
argocd app history <app>                                # find revision IDs
```

## Create & Update

```bash
argocd app create <app> \
  --repo https://github.com/org/repo \
  --path manifests/overlays/prod \
  --dest-server https://kubernetes.default.svc \
  --dest-namespace default \
  --sync-policy automated \
  --auto-prune \
  --self-heal

argocd app set <app> --sync-policy automated
argocd app set <app> --self-heal
argocd app set <app> --auto-prune
argocd app set <app> -p image.tag=v1.2.3               # override Helm param
argocd app unset <app> -p image.tag
argocd app delete <app>
argocd app delete <app> --cascade
```

## Projects

```bash
argocd proj list
argocd proj get <project>
argocd proj create <project>
argocd proj allow-cluster-resource <project> '*' '*'
argocd proj add-destination <project> https://kubernetes.default.svc default
argocd proj add-source <project> https://github.com/org/repo
argocd proj delete <project>
```

## Clusters & Repos

```bash
argocd cluster list
argocd cluster get <server>
argocd cluster add <context>                            # from kubeconfig
argocd cluster rm <server>

argocd repo list
argocd repo add https://github.com/org/repo \
  --username git --password <token>
argocd repo add git@github.com:org/repo \
  --ssh-private-key-path ~/.ssh/id_rsa
argocd repo rm https://github.com/org/repo
```

## RBAC & Admin

```bash
argocd account list
argocd account get <account>
argocd account generate-token --account <account>
argocd cert list
argocd admin settings validate
argocd admin cluster stats
argocd admin app diff-reconcile-results <app>
```
