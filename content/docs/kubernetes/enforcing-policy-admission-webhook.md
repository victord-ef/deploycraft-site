---
title: "Enforcing Cluster Policy with Your Admission Webhook"
description: "Extend a validating admission webhook into a practical policy enforcement system covering resource limits, image registry restrictions, and namespace conventions."
weight: 20
toc: true
draft: true
---

This post is the second part of a two-part series. The
[first part](/blog/2026/custom-admission-webhook-kubernetes/) walked
through building a validating admission webhook in Go, handling TLS,
and registering it with a Kubernetes cluster. This post extends that
webhook into a practical policy enforcement system covering resource
limits, image registry restrictions, and namespace conventions.

By the end of this post, the webhook rejects non-compliant workloads
at admission time — before they ever reach a Node — and supports a
gradual rollout strategy that avoids disrupting existing workloads.

## What policy enforcement means at admission time

Admission webhooks enforce policy at the earliest possible point in a
resource's lifecycle: the moment it enters the API server. This is
fundamentally different from runtime enforcement tools that detect
violations after a Pod is already running.

Enforcing at admission time means:

- Non-compliant resources never reach a Node.
- Policy violations surface immediately with a clear error message.
- There is no drift between what is declared and what runs.

The trade-off is that a misconfigured or unavailable webhook with
`failurePolicy: Fail` blocks all matching requests. The rollout
strategy at the end of this post addresses that risk directly.

## Designing a multi-rule policy engine

Rather than adding separate handlers for each rule, a cleaner approach
is a small policy engine — a list of rule functions that each accept
the raw admission request and return a decision. The webhook handler
runs every rule in order and rejects the request on the first failure.

Add a `policy` package to the project from part one:

```bash
mkdir policy
```

Define the interface in `policy/policy.go`:

```go
package policy

import (
    "fmt"

    admissionv1 "k8s.io/api/admission/v1"
)

// Rule is a single admission policy check.
type Rule func(req *admissionv1.AdmissionRequest) (allowed bool, message string)

// Evaluate runs all rules in order and returns the first failure.
func Evaluate(req *admissionv1.AdmissionRequest, rules []Rule) (bool, string) {
    for _, rule := range rules {
        if allowed, msg := rule(req); !allowed {
            return false, fmt.Sprintf("policy violation: %s", msg)
        }
    }
    return true, ""
}
```

Each rule is just a function. Adding a new policy means adding a new
function — no changes to the handler or the registration configuration.

## Enforcing resource limits

Pods without resource limits can consume unbounded CPU and memory,
starving other workloads on the same Node. The rule below rejects any
Pod that omits a `resources.limits` block on any container:

```go
package policy

import (
    "encoding/json"
    "fmt"

    admissionv1 "k8s.io/api/admission/v1"
    corev1 "k8s.io/api/core/v1"
)

// RequireResourceLimits rejects Pods that omit CPU or memory limits
// on any container.
func RequireResourceLimits(req *admissionv1.AdmissionRequest) (bool, string) {
    var pod corev1.Pod
    if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
        return false, fmt.Sprintf("could not parse Pod: %v", err)
    }

    for _, c := range pod.Spec.Containers {
        if c.Resources.Limits.Cpu().IsZero() {
            return false, fmt.Sprintf(
                "container %q has no CPU limit", c.Name)
        }
        if c.Resources.Limits.Memory().IsZero() {
            return false, fmt.Sprintf(
                "container %q has no memory limit", c.Name)
        }
    }
    return true, ""
}
```

## Enforcing image registry restrictions

Pulling images from arbitrary public registries introduces supply chain
risk. The rule below restricts container images to an approved registry
prefix:

```go
package policy

import (
    "encoding/json"
    "fmt"
    "strings"

    admissionv1 "k8s.io/api/admission/v1"
    corev1 "k8s.io/api/core/v1"
)

// AllowedRegistry is the registry prefix all images must use.
const AllowedRegistry = "registry.example.com/"

// RequireApprovedRegistry rejects Pods whose containers reference
// images outside the approved registry.
func RequireApprovedRegistry(req *admissionv1.AdmissionRequest) (bool, string) {
    var pod corev1.Pod
    if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
        return false, fmt.Sprintf("could not parse Pod: %v", err)
    }

    for _, c := range pod.Spec.Containers {
        if !strings.HasPrefix(c.Image, AllowedRegistry) {
            return false, fmt.Sprintf(
                "container %q uses image %q which is not from the approved registry %q",
                c.Name, c.Image, AllowedRegistry)
        }
    }
    return true, ""
}
```

Replace `registry.example.com/` with your organisation's internal
registry address. If multiple registries are approved, change
`AllowedRegistry` to a slice and check each prefix in a loop.

## Enforcing namespace conventions

Pods scheduled into namespaces that lack an `owner` label are hard to
track down when something goes wrong. The rule below rejects Pods in
namespaces that do not carry that label.

This rule needs to read the Namespace object, so it accepts a Kubernetes
client rather than just the raw request:

```go
package policy

import (
    "context"
    "fmt"

    admissionv1 "k8s.io/api/admission/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
)

// RequireNamespaceOwner rejects Pods in namespaces that lack an
// "owner" label.
func RequireNamespaceOwner(client kubernetes.Interface) Rule {
    return func(req *admissionv1.AdmissionRequest) (bool, string) {
        ns, err := client.CoreV1().Namespaces().Get(
            context.Background(), req.Namespace, metav1.GetOptions{})
        if err != nil {
            return false, fmt.Sprintf(
                "could not fetch namespace %q: %v", req.Namespace, err)
        }

        if _, ok := ns.Labels["owner"]; !ok {
            return false, fmt.Sprintf(
                "namespace %q is missing the required label \"owner\"",
                req.Namespace)
        }
        return true, ""
    }
}
```

The function returns a `Rule` — a closure that captures the client. This
keeps the rule signature consistent with the policy engine while
allowing rules that need cluster state.

## Wiring the rules into the handler

Update `main.go` to initialise a Kubernetes client and pass all three
rules to the policy engine:

```go
package main

import (
    "encoding/json"
    "io"
    "log"
    "net/http"

    admissionv1 "k8s.io/api/admission/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"

    "example.com/my-webhook/policy"
)

func main() {
    cfg, err := rest.InClusterConfig()
    if err != nil {
        log.Fatalf("could not load in-cluster config: %v", err)
    }
    client, err := kubernetes.NewForConfig(cfg)
    if err != nil {
        log.Fatalf("could not create Kubernetes client: %v", err)
    }

    rules := []policy.Rule{
        policy.RequireResourceLimits,
        policy.RequireApprovedRegistry,
        policy.RequireNamespaceOwner(client),
    }

    http.HandleFunc("/validate", func(w http.ResponseWriter, r *http.Request) {
        body, _ := io.ReadAll(r.Body)
        var review admissionv1.AdmissionReview
        json.Unmarshal(body, &review)

        allowed, message := policy.Evaluate(review.Request, rules)
        review.Response = &admissionv1.AdmissionResponse{
            UID:     review.Request.UID,
            Allowed: allowed,
        }
        if !allowed {
            review.Response.Result = &metav1.Status{Message: message}
        }

        resp, _ := json.Marshal(review)
        w.Header().Set("Content-Type", "application/json")
        w.Write(resp)
    })

    http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    })

    log.Println("Listening on :8443")
    log.Fatal(http.ListenAndServeTLS(":8443", "/tls/tls.crt", "/tls/tls.key", nil))
}
```

The in-cluster config picks up the ServiceAccount token mounted
automatically into the Pod. Add a ClusterRole so the webhook can read
Namespace objects:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: my-webhook-namespace-reader
rules:
- apiGroups: [""]
  resources: ["namespaces"]
  verbs: ["get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: my-webhook-namespace-reader
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: my-webhook-namespace-reader
subjects:
- kind: ServiceAccount
  name: default
  namespace: my-webhook
```

## Rolling out enforcement gradually

Switching a webhook from `failurePolicy: Ignore` to `failurePolicy: Fail`
across an entire cluster in one step risks breaking workloads that were
never designed with your policies in mind. A safer rollout uses namespace
selectors to expand coverage incrementally.

Update the `ValidatingWebhookConfiguration` to target only namespaces
carrying a specific label:

```yaml
webhooks:
- name: validate.my-webhook.example.com
  admissionReviewVersions: ["v1"]
  sideEffects: None
  failurePolicy: Fail
  namespaceSelector:
    matchLabels:
      policy.example.com/enforce: "true"
  clientConfig:
    service:
      name: my-webhook
      namespace: my-webhook
      path: /validate
    caBundle: <base64-encoded-CA>
  rules:
  - apiGroups: [""]
    apiVersions: ["v1"]
    operations: ["CREATE", "UPDATE"]
    resources: ["pods"]
```

Label namespaces one at a time as teams confirm their workloads comply:

```bash
kubectl label namespace <namespace> policy.example.com/enforce=true
```

This approach lets you enforce policy in new namespaces immediately
while giving existing teams time to adapt.

## Testing the policies

Rebuild and redeploy the image after adding the new rules, then test
each policy in isolation.

**Resource limits:**

```bash
kubectl run nginx --image=registry.example.com/nginx \
  -n enforced-namespace
```

```none
Error from server: admission webhook "validate.my-webhook.example.com"
denied the request: policy violation: container "nginx" has no CPU limit
```

**Image registry:**

```bash
kubectl run nginx --image=nginx:latest \
  -n enforced-namespace \
  --limits="cpu=100m,memory=128Mi"
```

```none
Error from server: admission webhook "validate.my-webhook.example.com"
denied the request: policy violation: container "nginx" uses image
"nginx:latest" which is not from the approved registry
"registry.example.com/"
```

**Namespace label:**

```bash
kubectl run nginx --image=registry.example.com/nginx \
  -n unlabelled-namespace \
  --limits="cpu=100m,memory=128Mi"
```

```none
Error from server: admission webhook "validate.my-webhook.example.com"
denied the request: policy violation: namespace "unlabelled-namespace"
is missing the required label "owner"
```

A Pod that satisfies all three rules is accepted without error.

## Monitoring webhook decisions

The webhook logs every decision. Stream the logs during testing to
follow each admission request in real time:

```bash
kubectl logs -n my-webhook -l app.kubernetes.io/name=my-webhook -f
```

For production, consider emitting structured JSON logs and shipping them
to your existing log aggregation stack. Each log line should include the
request UID, the resource name and namespace, the decision, and — for
rejections — the rule that triggered it. This makes it straightforward
to trace a rejection back to the policy that caused it.

## What comes next

The policies in this post are intentionally simple to keep the focus on
the webhook mechanics. Production policy systems typically need more:

- **Dry-run mode** — run the webhook in audit-only mode by setting
  `failurePolicy: Ignore` and logging all violations without rejecting
  requests, giving teams time to fix issues before enforcement begins.
- **Policy configuration** — externalise the allowed registry and
  required labels into a ConfigMap so policies can be updated without
  rebuilding the image.
- **Multiple webhooks** — separate high-severity rules (image registry)
  from advisory rules (namespace labels) into distinct
  ValidatingWebhookConfigurations with different `failurePolicy` settings.

If your policy needs grow beyond what a hand-written webhook can
reasonably maintain, consider
[OPA Gatekeeper](https://open-policy-agent.github.io/gatekeeper/),
which provides a policy engine, a library of reusable constraints, and
audit tooling built on top of the same admission webhook mechanism this
series covered.

For the full reference on webhook configuration options, see the
[Dynamic Admission Control](/docs/reference/access-authn-authz/extensible-admission-controllers/)
documentation.
