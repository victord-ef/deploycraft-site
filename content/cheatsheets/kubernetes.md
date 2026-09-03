---
title: "Kubernetes"
description: "Essential kubectl commands for pod management, deployments, debugging, RBAC, and cluster operations."
icon: "☸️"
weight: 1
count: 60
tags: ["kubernetes", "kubectl", "cloud-native"]
---

## Cluster Info

```bash
kubectl cluster-info
kubectl get nodes -o wide
kubectl get namespaces
kubectl config get-contexts
kubectl config use-context <context>
kubectl config current-context
kubectl api-resources
kubectl api-versions
```

## Pods

```bash
kubectl get pods -A                                 # all namespaces
kubectl get pods -n <ns> -o wide
kubectl describe pod <pod> -n <ns>
kubectl logs <pod> -n <ns>
kubectl logs <pod> -n <ns> -c <container>           # multi-container pod
kubectl logs <pod> -n <ns> --previous               # crashed container logs
kubectl logs -f <pod> -n <ns>                       # follow
kubectl exec -it <pod> -n <ns> -- /bin/bash
kubectl exec -it <pod> -n <ns> -c <container> -- sh
kubectl delete pod <pod> -n <ns> --grace-period=0 --force
kubectl top pod -n <ns>
kubectl top pod -n <ns> --sort-by=cpu
```

## Deployments

```bash
kubectl get deployments -n <ns>
kubectl describe deployment <name> -n <ns>
kubectl rollout status deployment/<name> -n <ns>
kubectl rollout history deployment/<name> -n <ns>
kubectl rollout undo deployment/<name> -n <ns>
kubectl rollout undo deployment/<name> --to-revision=2 -n <ns>
kubectl scale deployment <name> --replicas=3 -n <ns>
kubectl set image deployment/<name> <container>=<image>:<tag> -n <ns>
kubectl patch deployment <name> -p '{"spec":{"replicas":0}}' -n <ns>
```

## Services & Networking

```bash
kubectl get svc -n <ns>
kubectl describe svc <name> -n <ns>
kubectl expose deployment <name> --port=80 --type=ClusterIP -n <ns>
kubectl port-forward svc/<name> 8080:80 -n <ns>
kubectl port-forward pod/<pod> 8080:8080 -n <ns>
kubectl get endpoints -n <ns>
kubectl get ingress -n <ns>
kubectl describe ingress <name> -n <ns>
```

## ConfigMaps & Secrets

```bash
kubectl get configmaps -n <ns>
kubectl describe configmap <name> -n <ns>
kubectl get secret <name> -n <ns> -o jsonpath='{.data}' | base64 -d
kubectl get secret <name> -n <ns> -o yaml
kubectl create secret generic <name> --from-literal=key=value -n <ns>
kubectl create secret generic <name> --from-file=./config.json -n <ns>
kubectl create configmap <name> --from-file=./config.yaml -n <ns>
```

## RBAC

```bash
kubectl get roles -n <ns>
kubectl get rolebindings -n <ns>
kubectl get clusterroles
kubectl get clusterrolebindings
kubectl auth can-i get pods -n <ns> --as=<user>
kubectl auth can-i '*' '*' --as=system:serviceaccount:<ns>:<sa>
kubectl describe clusterrolebinding cluster-admin
```

## Nodes

```bash
kubectl get nodes
kubectl describe node <name>
kubectl top nodes
kubectl cordon <node>
kubectl drain <node> --ignore-daemonsets --delete-emptydir-data
kubectl uncordon <node>
kubectl taint nodes <node> key=value:NoSchedule
kubectl label node <node> <key>=<value>
```

## Debugging

```bash
kubectl get events -n <ns> --sort-by='.lastTimestamp'
kubectl run debug --image=busybox --restart=Never -it --rm -- sh
kubectl run debug --image=nicolaka/netshoot --restart=Never -it --rm -- bash
kubectl debug <pod> -it --image=busybox --copy-to=debug-pod -n <ns>
kubectl get pod <pod> -n <ns> -o yaml
kubectl diff -f manifest.yaml
```

## Resource Management

```bash
kubectl apply -f manifest.yaml
kubectl apply -f ./dir/
kubectl delete -f manifest.yaml
kubectl delete pod,svc --all -n <ns>
kubectl get all -n <ns>
kubectl get all -A
kubectl explain deployment.spec.template
kubectl kustomize ./overlay/ | kubectl apply -f -
```

## Output Formatting

```bash
kubectl get pods -o json
kubectl get pods -o yaml
kubectl get pods -o jsonpath='{.items[*].metadata.name}'
kubectl get pods -o custom-columns=NAME:.metadata.name,STATUS:.status.phase
kubectl get pods --no-headers | awk '{print $1}'
```
