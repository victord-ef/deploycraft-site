---
title: "Kubernetes Blog Series"
description: "In-depth DevSecOps and CI/CD coverage for Kubernetes practitioners"

sections:

  - number: 1
    title: "Custom Metrics & Autoscaling"
    description: "Scale your workloads on the signals that actually drive load — not just CPU and memory."
    pairs:
      - pair: "Pair 1"
        part1: "Building a custom metrics exporter in Go"
        part1url: "/blog/custom-metrics-exporter-kubernetes"
        part2: "Connecting custom metrics to the HorizontalPodAutoscaler"
        part2url: "/blog/connecting-custom-metrics-hpa"
        status: published
      - pair: "Pair 4"
        part1: "Deploying KEDA for event-driven autoscaling"
        part2: "Scaling on Kafka lag with KEDA ScaledObjects"
        status: coming-soon

  - number: 2
    title: "Admission Webhooks & Policy"
    description: "Intercept, validate, and enforce policy on every resource before it hits your cluster."
    pairs:
      - pair: "Pair 2"
        part1: "Writing a custom admission webhook in Kubernetes"
        part1url: "/blog/custom-admission-webhook-kubernetes"
        part2: "Enforcing policy with an admission webhook"
        part2url: "/blog/enforcing-policy-admission-webhook"
        status: published

  - number: 3
    title: "Multi-cluster Scheduling"
    description: "Place and federate workloads across clusters with precision."
    pairs:
      - pair: "Pair 3"
        part1: "Adopting the PlacementDecision API for multi-cluster scheduling"
        part1url: "/blog/placement-decision-api-adoption-guide"
        part2: "Federating workloads across clusters using PlacementDecision"
        status: partial

  - number: 4
    title: "RBAC & Access Control"
    description: "Design, enforce, and audit access control across your Kubernetes environments."
    pairs:
      - pair: "Pair 5"
        part1: "Implementing RBAC in Kubernetes"
        part2: "Auditing and tightening RBAC permissions with kubectl-who-can"
        status: coming-soon

  - number: 5
    title: "Certificates & Secrets"
    description: "Automate TLS, rotate credentials, and keep secrets out of plain sight."
    pairs:
      - pair: "Pair 6"
        part1: "Issuing TLS certificates for workloads using cert-manager"
        part2: "Automating certificate rotation and renewal in Kubernetes"
        status: coming-soon
      - pair: "Pair 7"
        part1: "Encrypting Kubernetes Secrets at rest"
        part2: "Syncing secrets from HashiCorp Vault using the External Secrets Operator"
        status: coming-soon

  - number: 6
    title: "Networking"
    description: "Control traffic, enforce boundaries, and secure communication between workloads."
    pairs:
      - pair: "Pair 8"
        part1: "Implementing NetworkPolicy for pod-to-pod traffic control"
        part2: "Enforcing zero-trust networking with Kubernetes NetworkPolicy"
        status: coming-soon

  - number: 7
    title: "Observability"
    description: "Trace, measure, and correlate signals across your entire cluster."
    pairs:
      - pair: "Pair 9"
        part1: "Setting up distributed tracing in Kubernetes with OpenTelemetry"
        part2: "Correlating traces, metrics, and logs for cluster-wide observability"
        status: coming-soon

  - number: 8
    title: "Storage / CSI"
    description: "Build and operate custom storage integrations for stateful workloads."
    pairs:
      - pair: "Pair 10"
        part1: "Writing a custom CSI driver for Kubernetes"
        part2: "Handling volume lifecycle and snapshots with your CSI driver"
        status: coming-soon

  - number: 9
    title: "Operators & CRDs"
    description: "Extend Kubernetes with custom resources and automate Day-2 operations."
    pairs:
      - pair: "Pair 11"
        part1: "Defining a Custom Resource Definition (CRD) in Kubernetes"
        part2: "Validating custom resources with CEL expressions in CRDs"
        status: coming-soon
      - pair: "Pair 12"
        part1: "Writing your first Kubernetes controller with controller-runtime"
        part2: "Handling reconciliation loops and error recovery in your controller"
        status: coming-soon
      - pair: "Pair 13"
        part1: "Building a Kubernetes Operator with the Operator SDK"
        part2: "Managing Operator lifecycle with the Operator Lifecycle Manager (OLM)"
        status: coming-soon
      - pair: "Pair 14"
        part1: "Implementing leader election and high availability in your Operator"
        part2: "Upgrading CRDs safely across versions with conversion webhooks"
        status: coming-soon
      - pair: "Pair 15"
        part1: "Testing Kubernetes Operators with envtest"
        part2: "Publishing and distributing your Operator on OperatorHub"
        status: coming-soon

  - number: 10
    title: "GitOps & CI/CD"
    description: "Automate deployments, enforce delivery standards, and ship with confidence."
    pairs:
      - pair: "Pair 16"
        part1: "Introduction to GitOps principles and workflow in Kubernetes"
        part2: "Setting up a GitOps repository structure for Kubernetes manifests"
        status: coming-soon
      - pair: "Pair 17"
        part1: "Installing and bootstrapping Flux in a Kubernetes cluster"
        part2: "Automating deployments with Flux Kustomization and HelmRelease"
        status: coming-soon
      - pair: "Pair 18"
        part1: "Installing ArgoCD and connecting your first Git repository"
        part2: "Managing multi-environment deployments with ArgoCD ApplicationSets"
        status: coming-soon
      - pair: "Pair 19"
        part1: "Building container images in CI and pushing to a registry"
        part2: "Triggering GitOps reconciliation from a CI pipeline with image update automation"
        status: coming-soon
      - pair: "Pair 20"
        part1: "Implementing canary deployments with Flagger and Flux"
        part2: "Automating rollbacks on metric failures with Flagger analysis"
        status: coming-soon

  - number: 11
    title: "Pod Security"
    description: "Harden workloads, drop privileges, and enforce security standards cluster-wide."
    pairs:
      - pair: "Pair 21"
        part1: "Configuring security contexts for Pods and containers in Kubernetes"
        part2: "Hardening workloads with read-only filesystems and non-root users"
        status: coming-soon
      - pair: "Pair 22"
        part1: "Introduction to Pod Security Admission and the three built-in policy levels"
        part2: "Enforcing Pod Security Standards across namespaces with labels and exceptions"
        status: coming-soon
      - pair: "Pair 23"
        part1: "Dropping Linux capabilities from Kubernetes workloads"
        part2: "Restricting syscalls with Seccomp profiles in Kubernetes"
        status: coming-soon
      - pair: "Pair 24"
        part1: "Applying AppArmor profiles to Kubernetes Pods"
        part2: "Enforcing SELinux labels on Pods and volumes in Kubernetes"
        status: coming-soon
      - pair: "Pair 25"
        part1: "Auditing workloads for Pod Security Standard violations with kubectl"
        part2: "Migrating from PodSecurityPolicy to Pod Security Admission"
        status: coming-soon

  - number: 12
    title: "Service Mesh"
    description: "Encrypt, observe, and control service-to-service traffic with Istio and Linkerd."
    pairs:
      - pair: "Pair 26"
        part1: "Introduction to service mesh concepts and architecture in Kubernetes"
        part2: "Choosing between Istio and Linkerd for your Kubernetes cluster"
        status: coming-soon
      - pair: "Pair 27"
        part1: "Installing Istio on a Kubernetes cluster with the Istio Operator"
        part2: "Configuring traffic management with Istio VirtualServices and DestinationRules"
        status: coming-soon
      - pair: "Pair 28"
        part1: "Enabling mutual TLS (mTLS) between services with Istio PeerAuthentication"
        part2: "Managing mTLS certificates and SPIFFE identities with Istio"
        status: coming-soon
      - pair: "Pair 29"
        part1: "Installing and meshing workloads with Linkerd"
        part2: "Securing service-to-service communication with Linkerd mTLS and authorization policies"
        status: coming-soon
      - pair: "Pair 30"
        part1: "Visualizing service traffic and latency with Istio and Kiali"
        part2: "Implementing traffic shifting and fault injection for resilience testing"
        status: coming-soon

  - number: 13
    title: "Ingress & Gateway API"
    description: "Route, secure, and manage external traffic into your cluster."
    pairs:
      - pair: "Pair 31"
        part1: "Routing external traffic into Kubernetes with Ingress resources"
        part2: "Configuring TLS termination and host-based routing with Ingress"
        status: coming-soon
      - pair: "Pair 32"
        part1: "Installing and configuring the NGINX Ingress Controller in Kubernetes"
        part2: "Rate limiting and access control with NGINX Ingress annotations"
        status: coming-soon
      - pair: "Pair 33"
        part1: "Introduction to the Kubernetes Gateway API and how it replaces Ingress"
        part2: "Configuring HTTPRoutes and TLSRoutes with the Gateway API"
        status: coming-soon
      - pair: "Pair 34"
        part1: "Splitting traffic across backends with Gateway API weighted routing"
        part2: "Securing Gateway API routes with ReferenceGrants and policy attachments"
        status: coming-soon
      - pair: "Pair 35"
        part1: "Migrating from Ingress to the Kubernetes Gateway API"
        part2: "Running Gateway API in production with observability and rate limiting"
        status: coming-soon

  - number: 14
    title: "Logging"
    description: "Collect, enrich, and query logs from every corner of your cluster."
    pairs:
      - pair: "Pair 36"
        part1: "Understanding Kubernetes logging architecture and log sources"
        part2: "Collecting container logs with a DaemonSet-based log forwarder"
        status: coming-soon
      - pair: "Pair 37"
        part1: "Installing and configuring Fluentd as a log collector in Kubernetes"
        part2: "Parsing, filtering, and routing logs to multiple outputs with Fluentd"
        status: coming-soon
      - pair: "Pair 38"
        part1: "Setting up Grafana Loki for log aggregation in Kubernetes"
        part2: "Querying and alerting on logs with LogQL and Grafana dashboards"
        status: coming-soon
      - pair: "Pair 39"
        part1: "Emitting structured JSON logs from Kubernetes workloads"
        part2: "Enriching logs with Kubernetes metadata using Fluent Bit"
        status: coming-soon
      - pair: "Pair 40"
        part1: "Enabling and configuring Kubernetes API server audit logging"
        part2: "Forwarding and analysing audit logs with Fluentd and Loki"
        status: coming-soon

  - number: 15
    title: "Multitenancy"
    description: "Isolate teams, enforce resource boundaries, and onboard tenants at scale."
    pairs:
      - pair: "Pair 41"
        part1: "Designing namespace boundaries for multitenancy in Kubernetes"
        part2: "Enforcing namespace isolation with NetworkPolicy and RBAC"
        status: coming-soon
      - pair: "Pair 42"
        part1: "Setting resource quotas to limit consumption per namespace in Kubernetes"
        part2: "Monitoring and alerting on ResourceQuota usage across tenants"
        status: coming-soon
      - pair: "Pair 43"
        part1: "Defining default resource requests and limits with LimitRange"
        part2: "Combining LimitRange and ResourceQuota for predictable tenant workloads"
        status: coming-soon
      - pair: "Pair 44"
        part1: "Automating tenant namespace provisioning with a Kubernetes Operator"
        part2: "Bootstrapping tenant RBAC, quotas, and policies with GitOps"
        status: coming-soon
      - pair: "Pair 45"
        part1: "Managing namespace hierarchies with the Hierarchical Namespace Controller"
        part2: "Propagating policies and quotas across child namespaces with HNC"
        status: coming-soon

  - number: 16
    title: "Backup & Disaster Recovery"
    description: "Protect your cluster state, automate backups, and recover with confidence."
    pairs:
      - pair: "Pair 46"
        part1: "Understanding Kubernetes backup strategies and what needs to be backed up"
        part2: "Installing and configuring Velero for cluster backup in Kubernetes"
        status: coming-soon
      - pair: "Pair 47"
        part1: "Creating on-demand and scheduled backups of namespaces with Velero"
        part2: "Backing up persistent volumes with Velero and CSI volume snapshots"
        status: coming-soon
      - pair: "Pair 48"
        part1: "Restoring namespaces and resources from a Velero backup"
        part2: "Cross-cluster restore with Velero for disaster recovery"
        status: coming-soon
      - pair: "Pair 49"
        part1: "Backing up and restoring etcd in a self-managed Kubernetes cluster"
        part2: "Automating etcd snapshots and validating restore integrity"
        status: coming-soon
      - pair: "Pair 50"
        part1: "Testing disaster recovery with simulated cluster failures and Velero restores"
        part2: "Building a disaster recovery runbook for Kubernetes platform teams"
        status: coming-soon
      - pair: "Pair 51"
        part1: "Configuring Velero with AWS S3 as a backup storage location"
        part2: "Storing Velero backups in Azure Blob Storage and Google Cloud Storage"
        status: coming-soon
      - pair: "Pair 52"
        part1: "Setting backup schedules and retention policies with Velero"
        part2: "Managing backup lifecycle and cleaning up expired backups automatically"
        status: coming-soon
      - pair: "Pair 53"
        part1: "Centralising backups from multiple clusters into a single Velero storage location"
        part2: "Migrating workloads between clusters using Velero backup and restore"
        status: coming-soon
      - pair: "Pair 54"
        part1: "Encrypting Velero backups at rest with server-side encryption"
        part2: "Securing Velero credentials and storage access with IAM roles and IRSA"
        status: coming-soon
      - pair: "Pair 55"
        part1: "Monitoring Velero backup status with Prometheus metrics and alerts"
        part2: "Validating backup completeness and integrity with automated restore tests"
        status: coming-soon
---
