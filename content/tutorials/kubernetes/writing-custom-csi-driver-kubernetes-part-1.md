---
title: "Writing a Custom CSI Driver for Kubernetes — Part 1"
date: 2026-09-01
description: "Implement a Container Storage Interface driver from scratch: build the gRPC Identity, Controller, and Node services, register the driver with Kubernetes, and provision a PersistentVolume."
cluster: "Kubernetes"
series: "Storage / CSI"
part: 1
difficulty: "advanced"
duration: "60 min"
tags: ["kubernetes", "csi", "storage", "persistentvolume", "grpc", "go", "platform-engineering"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

By the end of this tutorial you will have a working CSI driver deployed in Kubernetes: a gRPC server implementing the Identity, Controller, and Node services, a `CSIDriver` object registered with the cluster, and a PersistentVolume provisioned through your driver. Part 2 extends this with volume snapshots, cloning, expansion, and the full volume lifecycle.

## Prerequisites

- A Kubernetes cluster (v1.26+) with cluster-admin access
- Go 1.21+ installed
- `kubectl` configured
- Familiarity with gRPC and Protocol Buffers
- Basic understanding of Kubernetes PersistentVolumes and StorageClasses

---

## How CSI works in Kubernetes

The Container Storage Interface (CSI) is a standard API that decouples Kubernetes from storage vendors. Instead of building storage logic into Kubernetes itself, CSI defines a gRPC interface that any storage system can implement.

When a user creates a `PersistentVolumeClaim`, Kubernetes communicates with your CSI driver through three gRPC services:

| Service | Runs as | Responsibilities |
|---|---|---|
| **Identity** | Both Controller and Node | Driver name, capabilities, health |
| **Controller** | Deployment (one per cluster) | Create/delete volumes, attach/detach to nodes |
| **Node** | DaemonSet (one per node) | Mount/unmount volumes on the node filesystem |

Communication happens over a Unix socket. The kubelet and Kubernetes controllers discover your driver via a `CSIDriver` object and connect to the socket you expose.

The full provisioning flow:

```
User creates PVC
    │
    ▼
external-provisioner sidecar calls CreateVolume (Controller service)
    │
    ▼
Kubernetes creates PV and binds to PVC
    │
    ▼
Pod scheduled to a node
    │
    ▼
kubelet calls NodeStageVolume then NodePublishVolume (Node service)
    │
    ▼
Volume mounted at pod's requested path
```

---

## Step 1 — Set up the Go project

```bash
mkdir csi-demo-driver && cd csi-demo-driver
go mod init github.com/yourorg/csi-demo-driver

# Install CSI spec and gRPC dependencies
go get github.com/container-storage-interface/spec/lib/go/csi@latest
go get google.golang.org/grpc@latest
go get google.golang.org/grpc/credentials/insecure
go get k8s.io/klog/v2@latest
```

Project structure:

```
csi-demo-driver/
├── cmd/
│   └── main.go
├── pkg/
│   ├── driver/
│   │   ├── driver.go       # driver entrypoint, socket setup
│   │   ├── identity.go     # Identity gRPC service
│   │   ├── controller.go   # Controller gRPC service
│   │   └── node.go         # Node gRPC service
│   └── util/
│       └── util.go
├── deploy/
│   ├── csidriver.yaml
│   ├── storageclass.yaml
│   ├── controller.yaml
│   └── node.yaml
├── Dockerfile
└── go.mod
```

---

## Step 2 — Implement the Identity service

The Identity service tells Kubernetes what your driver is called and what capabilities it supports:

```go
// pkg/driver/identity.go
package driver

import (
    "context"
    "github.com/container-storage-interface/spec/lib/go/csi"
)

const (
    DriverName    = "demo.csi.example.com"
    DriverVersion = "0.1.0"
)

type identityServer struct{}

func (s *identityServer) GetPluginInfo(ctx context.Context, req *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
    return &csi.GetPluginInfoResponse{
        Name:          DriverName,
        VendorVersion: DriverVersion,
    }, nil
}

func (s *identityServer) GetPluginCapabilities(ctx context.Context, req *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
    return &csi.GetPluginCapabilitiesResponse{
        Capabilities: []*csi.PluginCapability{
            {
                Type: &csi.PluginCapability_Service_{
                    Service: &csi.PluginCapability_Service{
                        Type: csi.PluginCapability_Service_CONTROLLER_SERVICE,
                    },
                },
            },
            {
                Type: &csi.PluginCapability_VolumeExpansion_{
                    VolumeExpansion: &csi.PluginCapability_VolumeExpansion{
                        Type: csi.PluginCapability_VolumeExpansion_ONLINE,
                    },
                },
            },
        },
    }, nil
}

func (s *identityServer) Probe(ctx context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {
    return &csi.ProbeResponse{}, nil
}
```

---

## Step 3 — Implement the Controller service

The Controller service handles volume lifecycle at the cluster level — creating and deleting volumes, and publishing (attaching) them to specific nodes:

```go
// pkg/driver/controller.go
package driver

import (
    "context"
    "fmt"
    "os"
    "path/filepath"

    "github.com/container-storage-interface/spec/lib/go/csi"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
    "k8s.io/klog/v2"
)

type controllerServer struct {
    dataRoot string
}

func (cs *controllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
    if req.GetName() == "" {
        return nil, status.Error(codes.InvalidArgument, "volume name required")
    }
    if req.GetVolumeCapabilities() == nil {
        return nil, status.Error(codes.InvalidArgument, "volume capabilities required")
    }

    volumeID := req.GetName()
    volumePath := filepath.Join(cs.dataRoot, volumeID)

    // Create the directory that represents this volume
    if err := os.MkdirAll(volumePath, 0750); err != nil {
        return nil, status.Errorf(codes.Internal, "failed to create volume directory: %v", err)
    }

    capacity := req.GetCapacityRange().GetRequiredBytes()
    if capacity == 0 {
        capacity = 1 * 1024 * 1024 * 1024 // default 1GiB
    }

    klog.V(4).Infof("CreateVolume: created volume %s at %s", volumeID, volumePath)

    return &csi.CreateVolumeResponse{
        Volume: &csi.Volume{
            VolumeId:      volumeID,
            CapacityBytes: capacity,
            VolumeContext: req.GetParameters(),
        },
    }, nil
}

func (cs *controllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
    if req.GetVolumeId() == "" {
        return nil, status.Error(codes.InvalidArgument, "volume ID required")
    }

    volumePath := filepath.Join(cs.dataRoot, req.GetVolumeId())
    if err := os.RemoveAll(volumePath); err != nil {
        return nil, status.Errorf(codes.Internal, "failed to delete volume: %v", err)
    }

    klog.V(4).Infof("DeleteVolume: deleted volume %s", req.GetVolumeId())
    return &csi.DeleteVolumeResponse{}, nil
}

func (cs *controllerServer) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
    caps := []csi.ControllerServiceCapability_RPC_Type{
        csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
        csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT,
        csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
    }

    var csiCaps []*csi.ControllerServiceCapability
    for _, c := range caps {
        csiCaps = append(csiCaps, &csi.ControllerServiceCapability{
            Type: &csi.ControllerServiceCapability_Rpc{
                Rpc: &csi.ControllerServiceCapability_RPC{Type: c},
            },
        })
    }

    return &csi.ControllerGetCapabilitiesResponse{Capabilities: csiCaps}, nil
}

// ControllerPublishVolume, ValidateVolumeCapabilities, ListVolumes,
// GetCapacity, ControllerUnpublishVolume — return Unimplemented for
// drivers that do not support node-attach semantics (local storage)
func (cs *controllerServer) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
    return nil, status.Error(codes.Unimplemented, "")
}

func (cs *controllerServer) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
    return nil, status.Error(codes.Unimplemented, "")
}

func (cs *controllerServer) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
    return &csi.ValidateVolumeCapabilitiesResponse{
        Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
            VolumeCapabilities: req.GetVolumeCapabilities(),
        },
    }, nil
}

func (cs *controllerServer) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
    return nil, status.Error(codes.Unimplemented, "")
}

func (cs *controllerServer) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
    return nil, status.Error(codes.Unimplemented, "")
}

func (cs *controllerServer) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
    return &csi.ControllerExpandVolumeResponse{
        CapacityBytes:         req.GetCapacityRange().GetRequiredBytes(),
        NodeExpansionRequired: true,
    }, nil
}

func (cs *controllerServer) ControllerGetVolume(ctx context.Context, req *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
    return nil, status.Error(codes.Unimplemented, "")
}

func (cs *controllerServer) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
    return nil, status.Error(codes.Unimplemented, fmt.Sprintf("snapshots implemented in Part 2"))
}

func (cs *controllerServer) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
    return nil, status.Error(codes.Unimplemented, "")
}

func (cs *controllerServer) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
    return nil, status.Error(codes.Unimplemented, "")
}
```

---

## Step 4 — Implement the Node service

The Node service mounts and unmounts volumes on the node where a pod is scheduled:

```go
// pkg/driver/node.go
package driver

import (
    "context"
    "os"
    "path/filepath"

    "github.com/container-storage-interface/spec/lib/go/csi"
    "golang.org/x/sys/unix"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
    "k8s.io/klog/v2"
)

type nodeServer struct {
    nodeID   string
    dataRoot string
}

func (ns *nodeServer) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
    // NodeStageVolume is called once per volume per node (global mount)
    // For local hostpath volumes, staging is a no-op
    return &csi.NodeStageVolumeResponse{}, nil
}

func (ns *nodeServer) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
    return &csi.NodeUnstageVolumeResponse{}, nil
}

func (ns *nodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
    if req.GetVolumeId() == "" {
        return nil, status.Error(codes.InvalidArgument, "volume ID required")
    }
    if req.GetTargetPath() == "" {
        return nil, status.Error(codes.InvalidArgument, "target path required")
    }

    targetPath := req.GetTargetPath()
    sourcePath := filepath.Join(ns.dataRoot, req.GetVolumeId())

    // Ensure source volume directory exists
    if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
        return nil, status.Errorf(codes.NotFound, "volume %s not found at %s", req.GetVolumeId(), sourcePath)
    }

    // Create target mount point
    if err := os.MkdirAll(targetPath, 0750); err != nil {
        return nil, status.Errorf(codes.Internal, "failed to create target path: %v", err)
    }

    // Bind mount the volume directory to the target path
    if err := unix.Mount(sourcePath, targetPath, "", unix.MS_BIND|unix.MS_REC, ""); err != nil {
        return nil, status.Errorf(codes.Internal, "failed to bind mount %s to %s: %v", sourcePath, targetPath, err)
    }

    klog.V(4).Infof("NodePublishVolume: mounted %s at %s", sourcePath, targetPath)
    return &csi.NodePublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
    if req.GetTargetPath() == "" {
        return nil, status.Error(codes.InvalidArgument, "target path required")
    }

    if err := unix.Unmount(req.GetTargetPath(), unix.MNT_DETACH); err != nil && !os.IsNotExist(err) {
        return nil, status.Errorf(codes.Internal, "failed to unmount %s: %v", req.GetTargetPath(), err)
    }

    if err := os.RemoveAll(req.GetTargetPath()); err != nil {
        return nil, status.Errorf(codes.Internal, "failed to remove target path: %v", err)
    }

    klog.V(4).Infof("NodeUnpublishVolume: unmounted %s", req.GetTargetPath())
    return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (ns *nodeServer) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
    return &csi.NodeGetCapabilitiesResponse{
        Capabilities: []*csi.NodeServiceCapability{
            {
                Type: &csi.NodeServiceCapability_Rpc{
                    Rpc: &csi.NodeServiceCapability_RPC{
                        Type: csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
                    },
                },
            },
            {
                Type: &csi.NodeServiceCapability_Rpc{
                    Rpc: &csi.NodeServiceCapability_RPC{
                        Type: csi.NodeServiceCapability_RPC_EXPAND_VOLUME,
                    },
                },
            },
        },
    }, nil
}

func (ns *nodeServer) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
    return &csi.NodeGetInfoResponse{NodeId: ns.nodeID}, nil
}

func (ns *nodeServer) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
    return &csi.NodeExpandVolumeResponse{CapacityBytes: req.GetCapacityRange().GetRequiredBytes()}, nil
}

func (ns *nodeServer) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
    return nil, status.Error(codes.Unimplemented, "")
}
```

---

## Step 5 — Wire the driver and gRPC server

```go
// pkg/driver/driver.go
package driver

import (
    "net"
    "os"

    "github.com/container-storage-interface/spec/lib/go/csi"
    "google.golang.org/grpc"
    "k8s.io/klog/v2"
)

type Driver struct {
    name     string
    version  string
    nodeID   string
    endpoint string
    dataRoot string
}

func New(nodeID, endpoint, dataRoot string) *Driver {
    return &Driver{
        name:     DriverName,
        version:  DriverVersion,
        nodeID:   nodeID,
        endpoint: endpoint,
        dataRoot: dataRoot,
    }
}

func (d *Driver) Run() error {
    // Remove existing socket
    os.Remove(d.endpoint)

    listener, err := net.Listen("unix", d.endpoint)
    if err != nil {
        return err
    }

    server := grpc.NewServer(
        grpc.UnaryInterceptor(logGRPC),
    )

    csi.RegisterIdentityServer(server, &identityServer{})
    csi.RegisterControllerServer(server, &controllerServer{dataRoot: d.dataRoot})
    csi.RegisterNodeServer(server, &nodeServer{nodeID: d.nodeID, dataRoot: d.dataRoot})

    klog.Infof("CSI driver %s listening on %s", d.name, d.endpoint)
    return server.Serve(listener)
}

func logGRPC(ctx interface{}, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
    klog.V(4).Infof("gRPC call: %s", info.FullMethod)
    resp, err := handler(ctx.(interface{ Deadline() (interface{}, bool) }), req)
    if err != nil {
        klog.Errorf("gRPC error: %s: %v", info.FullMethod, err)
    }
    return resp, err
}
```

```go
// cmd/main.go
package main

import (
    "flag"
    "os"

    "github.com/yourorg/csi-demo-driver/pkg/driver"
    "k8s.io/klog/v2"
)

func main() {
    var (
        endpoint = flag.String("endpoint", "unix:///csi/csi.sock", "CSI endpoint")
        dataRoot = flag.String("data-root", "/csi-data", "Path where volumes are stored")
    )
    flag.Parse()

    nodeID := os.Getenv("NODE_ID")
    if nodeID == "" {
        klog.Fatal("NODE_ID environment variable required")
    }

    d := driver.New(nodeID, *endpoint, *dataRoot)
    if err := d.Run(); err != nil {
        klog.Fatalf("Driver failed: %v", err)
    }
}
```

---

## Step 6 — Build and containerise

```dockerfile
# Dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o csi-demo-driver ./cmd/main.go

FROM alpine:3.19
RUN apk add --no-cache util-linux
COPY --from=builder /app/csi-demo-driver /usr/local/bin/
ENTRYPOINT ["csi-demo-driver"]
```

```bash
docker build -t yourregistry/csi-demo-driver:0.1.0 .
docker push yourregistry/csi-demo-driver:0.1.0
```

---

## Step 7 — Deploy to Kubernetes

Register the driver with the cluster:

```yaml
# deploy/csidriver.yaml
apiVersion: storage.k8s.io/v1
kind: CSIDriver
metadata:
  name: demo.csi.example.com
spec:
  attachRequired: false
  podInfoOnMount: true
  volumeLifecycleModes:
    - Persistent
```

Deploy the Controller (runs the Controller service + sidecars):

```yaml
# deploy/controller.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: csi-demo-controller
  namespace: kube-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: csi-demo-controller
  template:
    metadata:
      labels:
        app: csi-demo-controller
    spec:
      serviceAccountName: csi-demo-controller-sa
      containers:
        - name: csi-driver
          image: yourregistry/csi-demo-driver:0.1.0
          args:
            - --endpoint=/csi/csi.sock
            - --data-root=/csi-data
          env:
            - name: NODE_ID
              valueFrom:
                fieldRef:
                  fieldPath: spec.nodeName
          volumeMounts:
            - name: socket-dir
              mountPath: /csi
            - name: data-dir
              mountPath: /csi-data
        - name: external-provisioner
          image: registry.k8s.io/sig-storage/csi-provisioner:v4.0.0
          args:
            - --csi-address=/csi/csi.sock
            - --v=5
          volumeMounts:
            - name: socket-dir
              mountPath: /csi
        - name: external-resizer
          image: registry.k8s.io/sig-storage/csi-resizer:v1.10.0
          args:
            - --csi-address=/csi/csi.sock
          volumeMounts:
            - name: socket-dir
              mountPath: /csi
      volumes:
        - name: socket-dir
          emptyDir: {}
        - name: data-dir
          hostPath:
            path: /csi-data
            type: DirectoryOrCreate
```

Deploy the Node DaemonSet (runs the Node service + node-driver-registrar):

```yaml
# deploy/node.yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: csi-demo-node
  namespace: kube-system
spec:
  selector:
    matchLabels:
      app: csi-demo-node
  template:
    metadata:
      labels:
        app: csi-demo-node
    spec:
      hostNetwork: true
      containers:
        - name: csi-driver
          image: yourregistry/csi-demo-driver:0.1.0
          args:
            - --endpoint=/csi/csi.sock
            - --data-root=/csi-data
          env:
            - name: NODE_ID
              valueFrom:
                fieldRef:
                  fieldPath: spec.nodeName
          securityContext:
            privileged: true
          volumeMounts:
            - name: socket-dir
              mountPath: /csi
            - name: data-dir
              mountPath: /csi-data
            - name: pods-mount-dir
              mountPath: /var/lib/kubelet/pods
              mountPropagation: Bidirectional
        - name: node-driver-registrar
          image: registry.k8s.io/sig-storage/csi-node-driver-registrar:v2.10.0
          args:
            - --csi-address=/csi/csi.sock
            - --kubelet-registration-path=/var/lib/kubelet/plugins/demo.csi.example.com/csi.sock
          volumeMounts:
            - name: socket-dir
              mountPath: /csi
            - name: registration-dir
              mountPath: /registration
      volumes:
        - name: socket-dir
          hostPath:
            path: /var/lib/kubelet/plugins/demo.csi.example.com
            type: DirectoryOrCreate
        - name: registration-dir
          hostPath:
            path: /var/lib/kubelet/plugins_registry
            type: Directory
        - name: data-dir
          hostPath:
            path: /csi-data
            type: DirectoryOrCreate
        - name: pods-mount-dir
          hostPath:
            path: /var/lib/kubelet/pods
            type: Directory
```

```bash
kubectl apply -f deploy/
kubectl get pods -n kube-system | grep csi-demo
```

---

## Step 8 — Provision a volume through your driver

Create a StorageClass, PVC, and test Pod:

```yaml
# deploy/storageclass.yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: csi-demo
provisioner: demo.csi.example.com
reclaimPolicy: Delete
volumeBindingMode: Immediate
allowVolumeExpansion: true
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: csi-demo-pvc
spec:
  accessModes:
    - ReadWriteOnce
  storageClassName: csi-demo
  resources:
    requests:
      storage: 1Gi
---
apiVersion: v1
kind: Pod
metadata:
  name: csi-demo-pod
spec:
  containers:
    - name: app
      image: busybox:1.36
      command: ["sh", "-c", "echo hello > /data/test.txt && sleep 3600"]
      volumeMounts:
        - name: data
          mountPath: /data
  volumes:
    - name: data
      persistentVolumeClaim:
        claimName: csi-demo-pvc
```

```bash
kubectl apply -f deploy/storageclass.yaml
kubectl get pvc csi-demo-pvc
kubectl get pod csi-demo-pod

# Verify data was written
kubectl exec csi-demo-pod -- cat /data/test.txt
```

The PVC should bind and the pod should mount the volume successfully — all routed through your CSI driver.

---

## What you have built

- A fully functional CSI driver in Go implementing Identity, Controller, and Node gRPC services
- Controller Deployment with `external-provisioner` and `external-resizer` sidecars
- Node DaemonSet with `node-driver-registrar` sidecar
- A registered `CSIDriver` object and working `StorageClass`
- A PersistentVolume provisioned end-to-end through your driver

## Next steps

In [Part 2](/tutorials/kubernetes/csi-driver-volume-lifecycle-snapshots-part-2/) you will implement `CreateSnapshot` and `DeleteSnapshot`, add volume cloning via `CreateVolume` from a snapshot source, handle online volume expansion, and implement `NodeGetVolumeStats` for volume usage metrics.
