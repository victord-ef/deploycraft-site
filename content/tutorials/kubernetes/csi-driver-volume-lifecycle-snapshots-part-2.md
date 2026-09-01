---
title: "CSI Driver Volume Lifecycle and Snapshots — Part 2"
date: 2026-09-01
description: "Extend your CSI driver with volume snapshots, cloning, online expansion, and usage metrics. Implement CreateSnapshot/DeleteSnapshot, volume cloning via CreateVolume, ControllerExpandVolume, NodeExpandVolume, and NodeGetVolumeStats."
cluster: "Kubernetes"
series: "Storage / CSI"
part: 2
difficulty: "advanced"
duration: "55 min"
tags: ["kubernetes", "csi", "storage", "snapshots", "persistentvolume", "grpc", "go", "platform-engineering"]
categories: ["tutorial"]
draft: false
toc: true
---

## What you will build

In [Part 1](/tutorials/kubernetes/writing-custom-csi-driver-kubernetes-part-1/) you built the core CSI driver: Identity, Controller, and Node services with end-to-end PV provisioning. In Part 2 you will extend the driver with the advanced CSI capabilities that production storage systems require — volume snapshots, cloning a volume from a snapshot, online expansion without downtime, and per-volume usage metrics.

## Prerequisites

- Completed [Part 1](/tutorials/kubernetes/writing-custom-csi-driver-kubernetes-part-1/) — CSI driver deployed and PVC provisioning working
- The `external-snapshotter` sidecar and `VolumeSnapshot` CRDs installed (covered in Step 1)
- Go 1.21+ and `kubectl` with cluster-admin access

---

## Step 1 — Install the external-snapshotter

The `external-snapshotter` sidecar watches `VolumeSnapshot` objects and calls your driver's `CreateSnapshot`/`DeleteSnapshot` RPCs. Install the CRDs and the snapshot controller first:

```bash
# Install VolumeSnapshot CRDs
kubectl apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/v7.0.2/client/config/crd/snapshot.storage.k8s.io_volumesnapshotclasses.yaml
kubectl apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/v7.0.2/client/config/crd/snapshot.storage.k8s.io_volumesnapshotcontents.yaml
kubectl apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/v7.0.2/client/config/crd/snapshot.storage.k8s.io_volumesnapshots.yaml

# Install the snapshot controller (runs in kube-system)
kubectl apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/v7.0.2/deploy/kubernetes/snapshot-controller/rbac-snapshot-controller.yaml
kubectl apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/v7.0.2/deploy/kubernetes/snapshot-controller/setup-snapshot-controller.yaml
```

Verify the snapshot controller is running:

```bash
kubectl get pods -n kube-system -l app=snapshot-controller
```

---

## Step 2 — Implement CreateSnapshot and DeleteSnapshot

Add snapshot support to your `ControllerServer`. These RPCs are called by the `external-snapshotter` sidecar when a `VolumeSnapshot` object is created or deleted.

```go
// controller.go — add to ControllerServer

func (c *ControllerServer) CreateSnapshot(
    ctx context.Context,
    req *csi.CreateSnapshotRequest,
) (*csi.CreateSnapshotResponse, error) {
    if req.GetName() == "" {
        return nil, status.Error(codes.InvalidArgument, "snapshot name required")
    }
    if req.GetSourceVolumeId() == "" {
        return nil, status.Error(codes.InvalidArgument, "source volume ID required")
    }

    snapshotID := fmt.Sprintf("snapshot-%s", req.GetName())
    snapshotPath := filepath.Join(c.dataRoot, "snapshots", snapshotID)

    // Idempotency: if snapshot already exists, return it
    if info, err := os.Stat(snapshotPath); err == nil {
        return &csi.CreateSnapshotResponse{
            Snapshot: &csi.Snapshot{
                SnapshotId:     snapshotID,
                SourceVolumeId: req.GetSourceVolumeId(),
                SizeBytes:      info.Size(),
                CreationTime:   timestamppb.New(info.ModTime()),
                ReadyToUse:     true,
            },
        }, nil
    }

    // Copy the source volume directory to snapshot path
    sourcePath := filepath.Join(c.dataRoot, "volumes", req.GetSourceVolumeId())
    if err := copyDir(sourcePath, snapshotPath); err != nil {
        return nil, status.Errorf(codes.Internal,
            "failed to snapshot volume %s: %v", req.GetSourceVolumeId(), err)
    }

    info, _ := os.Stat(snapshotPath)
    return &csi.CreateSnapshotResponse{
        Snapshot: &csi.Snapshot{
            SnapshotId:     snapshotID,
            SourceVolumeId: req.GetSourceVolumeId(),
            SizeBytes:      info.Size(),
            CreationTime:   timestamppb.New(time.Now()),
            ReadyToUse:     true,
        },
    }, nil
}

func (c *ControllerServer) DeleteSnapshot(
    ctx context.Context,
    req *csi.DeleteSnapshotRequest,
) (*csi.DeleteSnapshotResponse, error) {
    if req.GetSnapshotId() == "" {
        return nil, status.Error(codes.InvalidArgument, "snapshot ID required")
    }

    snapshotPath := filepath.Join(c.dataRoot, "snapshots", req.GetSnapshotId())
    if err := os.RemoveAll(snapshotPath); err != nil {
        return nil, status.Errorf(codes.Internal,
            "failed to delete snapshot: %v", err)
    }
    return &csi.DeleteSnapshotResponse{}, nil
}

// copyDir recursively copies a directory tree
func copyDir(src, dst string) error {
    return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        rel, _ := filepath.Rel(src, path)
        target := filepath.Join(dst, rel)

        if info.IsDir() {
            return os.MkdirAll(target, info.Mode())
        }

        return copyFile(path, target, info.Mode())
    })
}

func copyFile(src, dst string, mode os.FileMode) error {
    in, err := os.Open(src)
    if err != nil {
        return err
    }
    defer in.Close()

    out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
    if err != nil {
        return err
    }
    defer out.Close()

    _, err = io.Copy(out, in)
    return err
}
```

Declare the snapshot capability in your `IdentityServer`:

```go
// identity.go — add to GetPluginCapabilities
{
    Type: &csi.PluginCapability_Service_{
        Service: &csi.PluginCapability_Service{
            Type: csi.PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS,
        },
    },
},
```

And in `ControllerGetCapabilities`:

```go
csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT,
csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS,
```

---

## Step 3 — Deploy the external-snapshotter sidecar

Add the `external-snapshotter` container to your Controller Deployment:

```yaml
# controller-with-snapshotter.yaml — update the controller deployment
containers:
  - name: my-csi-driver
    image: my-registry/my-csi-driver:v0.2.0
    # ... same as Part 1

  - name: external-snapshotter
    image: registry.k8s.io/sig-storage/csi-snapshotter:v7.0.2
    args:
      - "--csi-address=/var/lib/csi/sockets/pluginproxy/csi.sock"
      - "--leader-election"
      - "--leader-election-namespace=kube-system"
    volumeMounts:
      - name: socket-dir
        mountPath: /var/lib/csi/sockets/pluginproxy/

  - name: external-provisioner
    # ... same as Part 1

  - name: external-resizer
    # ... same as Part 1
```

Grant the snapshotter RBAC permissions:

```yaml
# snapshotter-rbac.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: my-csi-snapshotter
rules:
  - apiGroups: [""]
    resources: [events]
    verbs: [list, watch, create, update, patch]
  - apiGroups: [snapshot.storage.k8s.io]
    resources: [volumesnapshotclasses]
    verbs: [get, list, watch]
  - apiGroups: [snapshot.storage.k8s.io]
    resources: [volumesnapshotcontents]
    verbs: [create, get, list, watch, update, delete, patch]
  - apiGroups: [snapshot.storage.k8s.io]
    resources: [volumesnapshotcontents/status]
    verbs: [patch]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: my-csi-snapshotter
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: my-csi-snapshotter
subjects:
  - kind: ServiceAccount
    name: my-csi-controller-sa
    namespace: kube-system
```

```bash
kubectl apply -f snapshotter-rbac.yaml
kubectl apply -f controller-with-snapshotter.yaml
```

---

## Step 4 — Create a VolumeSnapshotClass and take a snapshot

Register your driver as a snapshot provider:

```yaml
# volume-snapshot-class.yaml
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshotClass
metadata:
  name: my-csi-snapclass
  annotations:
    snapshot.storage.kubernetes.io/is-default-class: "true"
driver: my-csi-driver
deletionPolicy: Delete
```

```bash
kubectl apply -f volume-snapshot-class.yaml
```

Snapshot an existing PVC:

```yaml
# snapshot.yaml
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: my-pvc-snapshot
  namespace: default
spec:
  volumeSnapshotClassName: my-csi-snapclass
  source:
    persistentVolumeClaimName: my-pvc    # the PVC from Part 1
```

```bash
kubectl apply -f snapshot.yaml
kubectl get volumesnapshot my-pvc-snapshot
```

Wait for `READYTOUSE` to be `true`:

```bash
kubectl get volumesnapshot my-pvc-snapshot -w
# NAME               READYTOUSE   SOURCEPVC   RESTORESIZE   AGE
# my-pvc-snapshot    true         my-pvc      1Gi           12s
```

---

## Step 5 — Implement volume cloning from a snapshot

`CreateVolume` receives a `VolumeContentSource` when a PVC is created from a snapshot. Handle the `Snapshot` case:

```go
// controller.go — update CreateVolume

func (c *ControllerServer) CreateVolume(
    ctx context.Context,
    req *csi.CreateVolumeRequest,
) (*csi.CreateVolumeResponse, error) {
    // ... validation from Part 1 ...

    volumePath := filepath.Join(c.dataRoot, "volumes", req.GetName())

    // Check if this is a clone from snapshot
    if source := req.GetVolumeContentSource(); source != nil {
        if snap := source.GetSnapshot(); snap != nil {
            snapshotPath := filepath.Join(c.dataRoot, "snapshots", snap.GetSnapshotId())
            if _, err := os.Stat(snapshotPath); os.IsNotExist(err) {
                return nil, status.Errorf(codes.NotFound,
                    "snapshot %s not found", snap.GetSnapshotId())
            }
            if err := copyDir(snapshotPath, volumePath); err != nil {
                return nil, status.Errorf(codes.Internal,
                    "failed to clone from snapshot: %v", err)
            }
            return &csi.CreateVolumeResponse{
                Volume: &csi.Volume{
                    VolumeId:      req.GetName(),
                    CapacityBytes: req.GetCapacityRange().GetRequiredBytes(),
                    VolumeContext: req.GetParameters(),
                    ContentSource: req.GetVolumeContentSource(),
                },
            }, nil
        }

        // Volume-to-volume cloning
        if vol := source.GetVolume(); vol != nil {
            sourcePath := filepath.Join(c.dataRoot, "volumes", vol.GetVolumeId())
            if err := copyDir(sourcePath, volumePath); err != nil {
                return nil, status.Errorf(codes.Internal,
                    "failed to clone volume: %v", err)
            }
            return &csi.CreateVolumeResponse{
                Volume: &csi.Volume{
                    VolumeId:      req.GetName(),
                    CapacityBytes: req.GetCapacityRange().GetRequiredBytes(),
                    VolumeContext: req.GetParameters(),
                    ContentSource: req.GetVolumeContentSource(),
                },
            }, nil
        }
    }

    // Standard provisioning path (from Part 1)
    // ...
}
```

Add the clone capability:

```go
// In ControllerGetCapabilities
csi.ControllerServiceCapability_RPC_CLONE_VOLUME,
```

Restore a PVC from a snapshot:

```yaml
# pvc-from-snapshot.yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: restored-pvc
  namespace: default
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 1Gi
  storageClassName: my-csi-storageclass
  dataSource:
    name: my-pvc-snapshot
    kind: VolumeSnapshot
    apiGroup: snapshot.storage.k8s.io
```

```bash
kubectl apply -f pvc-from-snapshot.yaml
kubectl get pvc restored-pvc
# STATUS should become Bound
```

---

## Step 6 — Implement online volume expansion

Volume expansion lets users resize a PVC without detaching it. The CSI spec requires two RPCs: `ControllerExpandVolume` (expand the backing storage) and `NodeExpandVolume` (resize the filesystem in-place).

```go
// controller.go — add ControllerExpandVolume

func (c *ControllerServer) ControllerExpandVolume(
    ctx context.Context,
    req *csi.ControllerExpandVolumeRequest,
) (*csi.ControllerExpandVolumeResponse, error) {
    if req.GetVolumeId() == "" {
        return nil, status.Error(codes.InvalidArgument, "volume ID required")
    }

    capRange := req.GetCapacityRange()
    newSize := capRange.GetRequiredBytes()

    volumePath := filepath.Join(c.dataRoot, "volumes", req.GetVolumeId())
    if _, err := os.Stat(volumePath); os.IsNotExist(err) {
        return nil, status.Errorf(codes.NotFound,
            "volume %s not found", req.GetVolumeId())
    }

    // For a real block device driver, resize the device here.
    // For our file-backed driver, update the size metadata.
    if err := os.WriteFile(
        filepath.Join(volumePath, ".size"),
        []byte(fmt.Sprintf("%d", newSize)),
        0600,
    ); err != nil {
        return nil, status.Errorf(codes.Internal,
            "failed to update volume size: %v", err)
    }

    return &csi.ControllerExpandVolumeResponse{
        CapacityBytes:         newSize,
        NodeExpansionRequired: true,    // tells Kubernetes to call NodeExpandVolume
    }, nil
}
```

```go
// node.go — add NodeExpandVolume

func (n *NodeServer) NodeExpandVolume(
    ctx context.Context,
    req *csi.NodeExpandVolumeRequest,
) (*csi.NodeExpandVolumeResponse, error) {
    if req.GetVolumeId() == "" {
        return nil, status.Error(codes.InvalidArgument, "volume ID required")
    }
    if req.GetVolumePath() == "" {
        return nil, status.Error(codes.InvalidArgument, "volume path required")
    }

    // Resize the filesystem on the mounted volume.
    // For ext4/xfs mounts, the kernel does this via resize2fs / xfs_growfs.
    // The node-driver-registrar + Kubernetes coordinate calling this after
    // the block device has been expanded by ControllerExpandVolume.
    output, err := exec.CommandContext(ctx,
        "resize2fs", req.GetVolumePath(),
    ).CombinedOutput()
    if err != nil {
        return nil, status.Errorf(codes.Internal,
            "resize2fs failed: %v: %s", err, output)
    }

    return &csi.NodeExpandVolumeResponse{
        CapacityBytes: req.GetCapacityRange().GetRequiredBytes(),
    }, nil
}
```

Declare the expansion capabilities:

```go
// In ControllerGetCapabilities
csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,

// In NodeGetCapabilities
csi.NodeServiceCapability_RPC_EXPAND_VOLUME,
```

Update the StorageClass to allow expansion:

```yaml
# storageclass-expandable.yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: my-csi-storageclass
provisioner: my-csi-driver
allowVolumeExpansion: true    # required to enable PVC resize
reclaimPolicy: Delete
volumeBindingMode: Immediate
```

Test online expansion:

```bash
# Patch the PVC to request more storage
kubectl patch pvc my-pvc -p '{"spec":{"resources":{"requests":{"storage":"5Gi"}}}}'

# Watch the resize events
kubectl describe pvc my-pvc | grep -A5 Conditions
# Condition: FileSystemResizePending -> True -> FileSystemResizeSuccessful
```

The sequence: Kubernetes detects the PVC request increased → calls `ControllerExpandVolume` → sets `FileSystemResizePending` → on next pod mount (or immediately if online expansion is supported) calls `NodeExpandVolume` → `FileSystemResizeSuccessful`.

---

## Step 7 — Implement NodeGetVolumeStats

`NodeGetVolumeStats` lets Kubernetes report volume usage metrics (used bytes, available bytes, inode counts). The kubelet calls this RPC periodically for each mounted volume; the results appear in `kubectl top` and Prometheus metrics.

```go
// node.go — add NodeGetVolumeStats

func (n *NodeServer) NodeGetVolumeStats(
    ctx context.Context,
    req *csi.NodeGetVolumeStatsRequest,
) (*csi.NodeGetVolumeStatsResponse, error) {
    if req.GetVolumeId() == "" {
        return nil, status.Error(codes.InvalidArgument, "volume ID required")
    }
    if req.GetVolumePath() == "" {
        return nil, status.Error(codes.InvalidArgument, "volume path required")
    }

    var stat syscall.Statfs_t
    if err := syscall.Statfs(req.GetVolumePath(), &stat); err != nil {
        return nil, status.Errorf(codes.Internal,
            "statfs on %s failed: %v", req.GetVolumePath(), err)
    }

    totalBytes := int64(stat.Blocks) * int64(stat.Bsize)
    availBytes := int64(stat.Bavail) * int64(stat.Bsize)
    usedBytes := totalBytes - int64(stat.Bfree)*int64(stat.Bsize)

    totalInodes := int64(stat.Files)
    freeInodes := int64(stat.Ffree)
    usedInodes := totalInodes - freeInodes

    return &csi.NodeGetVolumeStatsResponse{
        Usage: []*csi.VolumeUsage{
            {
                Unit:      csi.VolumeUsage_BYTES,
                Total:     totalBytes,
                Available: availBytes,
                Used:      usedBytes,
            },
            {
                Unit:      csi.VolumeUsage_INODES,
                Total:     totalInodes,
                Available: freeInodes,
                Used:      usedInodes,
            },
        },
    }, nil
}
```

Declare the capability:

```go
// In NodeGetCapabilities
csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
```

Once deployed, the kubelet exposes these metrics on its `/metrics` endpoint:

```promql
# Bytes available on a CSI volume
kubelet_volume_stats_available_bytes{
  namespace="default",
  persistentvolumeclaim="my-pvc"
}

# Bytes used
kubelet_volume_stats_used_bytes{
  namespace="default",
  persistentvolumeclaim="my-pvc"
}

# Inodes used
kubelet_volume_stats_inodes_used{
  namespace="default",
  persistentvolumeclaim="my-pvc"
}
```

Add a Grafana alert: fire when `available_bytes / capacity_bytes < 0.10` — volume is more than 90% full.

---

## Step 8 — Full capability matrix

Update the Dockerfile tag and rebuild:

```dockerfile
# Dockerfile — same as Part 1, bump version
FROM golang:1.21 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o my-csi-driver .

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /app/my-csi-driver /
USER nonroot:nonroot
ENTRYPOINT ["/my-csi-driver"]
```

```bash
docker build -t my-registry/my-csi-driver:v0.2.0 .
docker push my-registry/my-csi-driver:v0.2.0
```

Update the controller and node DaemonSet image tags to `v0.2.0` and apply:

```bash
kubectl set image deployment/my-csi-controller \
  my-csi-driver=my-registry/my-csi-driver:v0.2.0 \
  -n kube-system

kubectl set image daemonset/my-csi-node \
  my-csi-driver=my-registry/my-csi-driver:v0.2.0 \
  -n kube-system
```

Your driver now supports the full CSI capability matrix:

| Capability | RPC | Implemented |
|---|---|---|
| Provision / delete | `CreateVolume`, `DeleteVolume` | Part 1 |
| Attach / detach | `ControllerPublishVolume`, `ControllerUnpublishVolume` | Part 1 |
| Mount / unmount | `NodeStageVolume`, `NodePublishVolume`, `NodeUnpublishVolume` | Part 1 |
| Snapshots | `CreateSnapshot`, `DeleteSnapshot` | Part 2 |
| Cloning | `CreateVolume` from `VolumeContentSource` | Part 2 |
| Online expansion | `ControllerExpandVolume`, `NodeExpandVolume` | Part 2 |
| Volume stats | `NodeGetVolumeStats` | Part 2 |

---

## Step 9 — End-to-end snapshot and restore test

Run a complete snapshot lifecycle test:

```bash
# 1. Write data to the original PVC
kubectl exec -it test-pod -- sh -c 'echo "original data" > /data/test.txt'

# 2. Take a snapshot
kubectl apply -f snapshot.yaml
kubectl wait --for=condition=ReadyToUse volumesnapshot/my-pvc-snapshot --timeout=60s

# 3. Restore to a new PVC
kubectl apply -f pvc-from-snapshot.yaml
kubectl wait --for=condition=Bound pvc/restored-pvc --timeout=60s

# 4. Mount the restored PVC and verify data
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: restore-test-pod
spec:
  containers:
    - name: verify
      image: busybox
      command: ["sh", "-c", "cat /data/test.txt && echo OK"]
      volumeMounts:
        - mountPath: /data
          name: restored
  volumes:
    - name: restored
      persistentVolumeClaim:
        claimName: restored-pvc
  restartPolicy: Never
EOF

kubectl logs restore-test-pod
# Expected: "original data"
#           "OK"

# 5. Test online expansion
kubectl patch pvc my-pvc -p '{"spec":{"resources":{"requests":{"storage":"5Gi"}}}}'
kubectl get pvc my-pvc -w
# STATUS should remain Bound, capacity increases

# 6. Verify stats are available
kubectl top pod test-pod
```

---

## What you have built

- `CreateSnapshot` / `DeleteSnapshot` — snapshot a PVC into a persistent point-in-time copy
- `external-snapshotter` sidecar deployed and RBAC configured
- `VolumeSnapshotClass` registered with your driver as the provider
- `CreateVolume` from `VolumeContentSource` — restore a snapshot or clone a volume
- `ControllerExpandVolume` + `NodeExpandVolume` — online volume expansion without pod restart
- `NodeGetVolumeStats` — per-volume usage metrics surfaced to kubelet and Prometheus
- Full capability matrix across Identity, Controller, and Node services

Your CSI driver now implements the complete production feature set. The pattern you have followed — gRPC service interfaces, idempotent operations, sidecar-managed Kubernetes lifecycle — applies directly to drivers backed by any real storage system: NFS, iSCSI, cloud block storage, or distributed filesystems.
