---
title: "Building a Custom Admission Webhook in Go"
description: "Write a validating admission webhook in Go from scratch, package it as a container, and register it with a Kubernetes cluster."
weight: 10
toc: true
draft: true
---

Kubernetes gives you several extension points for influencing how
resources are created and modified. One of the most powerful is the
_admission webhook_ — an HTTP server that the API server calls every
time a resource is created, updated, or deleted, before the change is
persisted to etcd.

This post walks through writing a validating admission webhook in Go
from scratch, packaging it as a container, and registering it with a
cluster. The
[second part](/blog/2026/enforcing-policy-admission-webhook/) builds on
this foundation to enforce real cluster policy with the webhook.

## What admission webhooks do

When you submit a resource to the Kubernetes API server, the request
passes through two admission phases before it is persisted:

- _Mutating_ admission webhooks run first. They can modify the incoming
  object — injecting sidecar containers, adding default labels, or
  setting missing fields.

- _Validating_ admission webhooks run second, after all mutations are
  complete. They cannot modify the object, but they can reject it with
  a descriptive error message.

A single webhook server can implement both types, or just one. This post
focuses on a validating webhook, which is the simpler starting point and
the right tool for enforcement logic.

## How the API server calls a webhook

When a resource matches a webhook's rules, the API server sends an
`AdmissionReview` object to the webhook's HTTPS endpoint. The webhook
inspects the request and returns an `AdmissionReview` response with an
`allowed` field set to `true` or `false`.

The full exchange looks like this:

1. A user runs `kubectl apply -f pod.yaml`.
2. The API server matches the resource against registered webhooks.
3. The API server sends a JSON `AdmissionReview` request to the webhook.
4. The webhook inspects the object and returns an `AdmissionReview`
   response.
5. If `allowed` is `false`, the API server rejects the request and
   returns the webhook's message to the user.

One important constraint: webhooks must be served over HTTPS. The API
server will not call a plain HTTP endpoint. The TLS setup is covered in
its own section below.

## Setting up the project

Create a Go module for the webhook server:

```bash
mkdir my-webhook && cd my-webhook
go mod init example.com/my-webhook
go get k8s.io/api/admission/v1
go get k8s.io/apimachinery/pkg/apis/meta/v1
```

The `k8s.io/api/admission/v1` package provides the `AdmissionReview`
struct that matches the JSON the API server sends and expects back.

## Writing the webhook handler

Create `main.go`. The webhook server has two responsibilities: decode
the incoming `AdmissionReview`, inspect the object, and return a
decision.

The example below validates that every Pod carries a
`app.kubernetes.io/name` label. Pods without that label are rejected:

```go
package main

import (
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"

    admissionv1 "k8s.io/api/admission/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func validate(w http.ResponseWriter, r *http.Request) {
    body, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "could not read body", http.StatusBadRequest)
        return
    }

    var review admissionv1.AdmissionReview
    if err := json.Unmarshal(body, &review); err != nil {
        http.Error(w, "could not decode AdmissionReview", http.StatusBadRequest)
        return
    }

    allowed, message := validatePod(review.Request)

    review.Response = &admissionv1.AdmissionResponse{
        UID:     review.Request.UID,
        Allowed: allowed,
    }
    if !allowed {
        review.Response.Result = &metav1.Status{
            Message: message,
        }
    }

    resp, err := json.Marshal(review)
    if err != nil {
        http.Error(w, "could not encode response", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.Write(resp)
}

func validatePod(req *admissionv1.AdmissionRequest) (bool, string) {
    var meta struct {
        Labels map[string]string `json:"labels"`
    }
    var obj struct {
        Metadata interface{} `json:"metadata"`
    }

    // Unmarshal just the labels from the raw object.
    var raw map[string]json.RawMessage
    if err := json.Unmarshal(req.Object.Raw, &raw); err != nil {
        return false, fmt.Sprintf("could not parse object: %v", err)
    }
    if err := json.Unmarshal(raw["metadata"], &meta); err != nil {
        return false, fmt.Sprintf("could not parse metadata: %v", err)
    }
    _ = obj

    if _, ok := meta.Labels["app.kubernetes.io/name"]; !ok {
        return false, "Pod must have the label app.kubernetes.io/name"
    }
    return true, ""
}

func main() {
    http.HandleFunc("/validate", validate)
    http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    })

    log.Println("Listening on :8443")
    if err := http.ListenAndServeTLS(":8443", "/tls/tls.crt", "/tls/tls.key", nil); err != nil {
        log.Fatalf("server error: %v", err)
    }
}
```

The `/healthz` path gives Kubernetes a liveness probe target. The
`/validate` path is the admission endpoint the API server calls.

## Handling TLS

Kubernetes requires webhooks to be served over HTTPS. The simplest
approach for a cluster that already has
[cert-manager](https://cert-manager.io/) installed is to issue a
certificate automatically.

Create a `Certificate` resource that issues a TLS certificate for the
webhook's Service:

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: my-webhook-tls
  namespace: my-webhook
spec:
  secretName: my-webhook-tls
  dnsNames:
  - my-webhook.my-webhook.svc
  - my-webhook.my-webhook.svc.cluster.local
  issuerRef:
    name: cluster-issuer
    kind: ClusterIssuer
```

Replace `cluster-issuer` with the name of the ClusterIssuer already
configured in your cluster. cert-manager stores the certificate and
private key in the Secret named `my-webhook-tls`, which the Deployment
below mounts at `/tls`.

If cert-manager is not available, you can generate a self-signed
certificate manually using `openssl` and store it in a Secret:

```bash
openssl req -x509 -newkey rsa:4096 -keyout tls.key -out tls.crt \
  -days 365 -nodes \
  -subj "/CN=my-webhook.my-webhook.svc"

kubectl create secret tls my-webhook-tls \
  --cert=tls.crt --key=tls.key \
  -n my-webhook
```

Note the CA bundle from `tls.crt` — you will need it when registering
the webhook in a later step.

## Building the container image

A multi-stage build keeps the final image small. The example below uses
Docker, but the same pattern works with any OCI-compatible build tool
such as Buildah or Podman:

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /webhook .

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /webhook /webhook
EXPOSE 8443
ENTRYPOINT ["/webhook"]
```

Build and push the image, replacing `<registry>` with your own registry
address:

```bash
docker build -t <registry>/my-webhook:v1.0.0 .
docker push <registry>/my-webhook:v1.0.0
```

## Deploying to the cluster

Three manifests are enough to run the webhook: a Namespace, a
Deployment, and a Service.

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: my-webhook
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-webhook
  namespace: my-webhook
  labels:
    app.kubernetes.io/name: my-webhook
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: my-webhook
  template:
    metadata:
      labels:
        app.kubernetes.io/name: my-webhook
    spec:
      containers:
      - name: webhook
        image: <registry>/my-webhook:v1.0.0
        ports:
        - name: webhook
          containerPort: 8443
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8443
            scheme: HTTPS
          initialDelaySeconds: 5
          periodSeconds: 10
        volumeMounts:
        - name: tls
          mountPath: /tls
          readOnly: true
        resources:
          requests:
            cpu: 50m
            memory: 32Mi
          limits:
            cpu: 100m
            memory: 64Mi
      volumes:
      - name: tls
        secret:
          secretName: my-webhook-tls
---
apiVersion: v1
kind: Service
metadata:
  name: my-webhook
  namespace: my-webhook
  labels:
    app.kubernetes.io/name: my-webhook
spec:
  selector:
    app.kubernetes.io/name: my-webhook
  ports:
  - name: webhook
    port: 443
    targetPort: webhook
```

Apply the manifests:

```bash
kubectl apply -f manifests.yaml
```

## Registering the webhook

With the server running, register it with the API server by creating a
`ValidatingWebhookConfiguration`. The `caBundle` field must contain the
base64-encoded CA certificate that signed the webhook's TLS certificate.
If you used cert-manager, retrieve it from the Secret:

```bash
kubectl get secret my-webhook-tls -n my-webhook \
  -o jsonpath='{.data.ca\.crt}'
```

Paste the output into the `caBundle` field:

```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: my-webhook
  annotations:
    cert-manager.io/inject-ca-from: my-webhook/my-webhook-tls
webhooks:
- name: validate.my-webhook.example.com
  admissionReviewVersions: ["v1"]
  sideEffects: None
  failurePolicy: Fail
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

The `failurePolicy: Fail` setting means that if the webhook is
unreachable, the API server rejects the request. This is the safe
default for enforcement webhooks — a webhook that silently passes
requests when it is down provides no guarantees. Change it to `Ignore`
only during initial development.

Apply the configuration:

```bash
kubectl apply -f webhook-config.yaml
```

## Testing the webhook

Create a Pod without the required label to confirm the webhook rejects
it:

```bash
kubectl run nginx --image=nginx -n default
```

The API server returns an error from the webhook:

```none
Error from server: admission webhook "validate.my-webhook.example.com"
denied the request: Pod must have the label app.kubernetes.io/name
```

Now create a Pod with the label to confirm it is accepted:

```bash
kubectl run nginx --image=nginx -n default \
  --labels="app.kubernetes.io/name=nginx"
```

The Pod is created successfully. The webhook is working correctly.

To inspect the webhook's decision log:

```bash
kubectl logs -n my-webhook -l app.kubernetes.io/name=my-webhook
```

## What comes next

A working webhook is the foundation for cluster-wide policy enforcement.
The
[second part](/blog/2026/enforcing-policy-admission-webhook/) of this
series builds on this webhook to enforce a broader set of policies —
covering resource limits, image registry restrictions, and namespace
conventions — and introduces strategies for rolling out enforcement
gradually without disrupting existing workloads.

For the full reference on admission webhooks, including mutating webhooks
and webhook configuration options, see the
[Dynamic Admission Control](/docs/reference/access-authn-authz/extensible-admission-controllers/)
documentation.
