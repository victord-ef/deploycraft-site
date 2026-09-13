---
title: "Helm"
description: "Helm install, upgrade, rollback, repo management, templating, and chart authoring commands."
icon: "⛵"
weight: 7
count: 40
tags: ["helm", "kubernetes", "cloud-native"]
---

## Install & Upgrade

```bash
helm install <release> <chart>
helm install <release> <chart> -n <namespace>
helm install <release> <chart> --create-namespace
helm install <release> <chart> -f values.yaml
helm install <release> <chart> --set image.tag=v1.2.3
helm install <release> <chart> --set-string config.port=8080
helm install <release> <chart> --dry-run --debug
helm install <release> oci://registry/chart --version 1.2.3

helm upgrade <release> <chart>
helm upgrade <release> <chart> -f values.yaml
helm upgrade <release> <chart> --reuse-values
helm upgrade <release> <chart> --set image.tag=v2.0.0
helm upgrade --install <release> <chart>              # install if not exists
helm upgrade --install <release> <chart> --atomic     # rollback on failure
helm upgrade --install <release> <chart> --wait
helm upgrade --install <release> <chart> --timeout 5m
```

## Releases

```bash
helm list
helm list -A                                          # all namespaces
helm list -n <namespace>
helm list --deployed
helm list --failed
helm status <release> -n <namespace>
helm get all <release> -n <namespace>
helm get values <release> -n <namespace>
helm get values <release> -n <namespace> --all        # including defaults
helm get manifest <release> -n <namespace>
helm get notes <release> -n <namespace>
helm history <release> -n <namespace>
```

## Rollback & Uninstall

```bash
helm rollback <release> -n <namespace>
helm rollback <release> <revision> -n <namespace>
helm uninstall <release> -n <namespace>
helm uninstall <release> -n <namespace> --keep-history
```

## Repositories

```bash
helm repo add <name> <url>
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update
helm repo list
helm repo remove <name>
helm search repo <keyword>
helm search repo <chart> --versions
helm search hub <keyword>
```

## Templating & Lint

```bash
helm template <release> <chart>
helm template <release> <chart> -f values.yaml
helm template <release> <chart> --set image.tag=v1.0
helm template <release> <chart> | kubectl apply --dry-run=client -f -
helm lint <chart>
helm lint <chart> -f values.yaml
helm lint <chart> --strict
```

## Chart Development

```bash
helm create <chart-name>
helm package <chart-dir>
helm package <chart-dir> --version 1.2.3
helm show chart <chart>
helm show values <chart>
helm show readme <chart>
helm dependency update <chart-dir>
helm dependency build <chart-dir>
helm dependency list <chart-dir>
helm plugin install <url>
helm plugin list
```
