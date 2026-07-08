# Summary

This document proposes adding a **CSI Volume Device Exporter** as a new component in the kubernetes-csi-addons project. The exporter is a per-node DaemonSet that maps CSI volumes to their underlying block devices, enabling correlation of storage path health metrics with Kubernetes workloads.

Unlike existing csi-addons features (which extend CSI via gRPC RPCs), this component operates independently of CSI drivers — it reads kubelet-internal metadata and host sysfs to produce Prometheus metrics that bridge the gap between host-level storage health and Kubernetes volume identity.

## Terminologies

- **CSI** — Container Storage Interface
- **DM-multipath** — Device Mapper multipath (Linux redundant storage paths)
- **NVMe-oF** — NVMe over Fabrics (high-performance remote block storage)
- **PV/PVC** — PersistentVolume / PersistentVolumeClaim
- **CMO** — Cluster Monitoring Operator (OpenShift-specific, manages node_exporter)
- **sysfs** — Linux kernel pseudo-filesystem exposing device information

## Motivation

Prometheus `node_exporter` now includes collectors for DM-multipath path state ([node_exporter#3581](https://github.com/prometheus/node_exporter/pull/3581)) and NVMe-oF subsystem health ([node_exporter#3579](https://github.com/prometheus/node_exporter/pull/3579)). These expose storage path health at the **node level** — which device has degraded paths, on which node.

However, no metric exists to answer the critical operational question: **which workloads are affected?**

When a Fibre Channel link drops or an NVMe-oF controller dies, operators need to know immediately which PersistentVolumes, PersistentVolumeClaims, and Pods/VMs are impacted. Today, this requires manual investigation: SSH into each node, run `multipath -ll`, cross-reference with `kubectl get pv`, and trace device relationships through LUKS and DM layers.

The CSI Volume Device Exporter solves this by providing the **join key** between Kubernetes volume identity and host block device identity as a Prometheus metric.

### Why csi-addons?

1. **Driver-agnostic CSI infrastructure** — The exporter works for all CSI drivers without any driver-specific code. This aligns with csi-addons' role as a vendor-neutral CSI ecosystem project.
2. **Complements existing csi-addons operations** — ReclaimSpace, NetworkFence, and VolumeReplication all operate on CSI volumes. The exporter provides observability into the physical layer beneath those volumes.
3. **Deployment lifecycle** — The csi-addons operator already manages per-node components (the sidecar). Adding a DaemonSet for volume observability is a natural extension of its lifecycle management.
4. **Cross-distribution** — Housing the exporter here makes it available to any Kubernetes distribution, not tied to a specific platform operator.

## Goal

- Provide a per-node agent that discovers CSI volume-to-block-device mappings and exposes them as Prometheus metrics.
- Ship Prometheus alert rules that correlate storage path degradation with impacted workloads.
- Support both filesystem volumes (VolumeMode=Filesystem) and block volumes (VolumeMode=Block).
- Work with any CSI driver — no driver-specific gRPC RPCs or sidecars required.

## Non-Goals

- Replacing or duplicating node_exporter's multipath/NVMe collectors (those remain upstream in node_exporter).
- Adding new gRPC RPCs to the CSI-Addons spec.
- Automated remediation (e.g., live-migrating VMs) — the exporter provides observability only.

## Design

### Architecture

```
┌─────────────────────────────────────────────────────────┐
│  Prometheus                                             │
│                                                         │
│  csi_volume_node_device_info{volume_handle="pvc-abc",   │
│    driver="csi.trident.netapp.io", device="dm-0"}       │
│         ┊                                               │
│         ┊ PromQL JOIN on (node, device)                 │
│         ┊                                               │
│  node_dmmultipath_path_state{node="worker-1",           │
│    sysfs_name="dm-0", path_state="faulty"}              │
│                                                         │
│  → Alert: CSIVolumeMultipathDegraded                    │
│    "PVC my-app/data on worker-1 has degraded paths"     │
└─────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────┐
│  Per-Node DaemonSet: csi-volume-device-exporter         │
│                                                         │
│  1. Walk kubelet pod directories                        │
│  2. Read vol_data.json (CSI volume identity)            │
│  3. stat() mount point → device major:minor             │
│  4. Resolve via sysfs → kernel device name              │
│  5. Walk DM slaves if LUKS → find multipath device      │
│  6. Emit metric: volume_handle + driver + device        │
└─────────────────────────────────────────────────────────┘
```

### Component Placement

| Existing Components | New Component |
|---|---|
| `cmd/manager` — controller (Deployment) | |
| `cmd/csi-addons` — CLI tool | |
| | `cmd/csi-volume-device-exporter` — per-node metrics agent (DaemonSet) |

The exporter is a **standalone binary** — it does not communicate with CSI drivers via gRPC, does not use CRDs, and does not depend on the csi-addons controller. It reads host files and emits Prometheus metrics.

The csi-addons operator manages its full lifecycle: DaemonSet, PrometheusRule, and PodMonitor are deployed and upgraded atomically by the operator controller. This ensures alert rule PromQL stays in sync with the metric names and labels the exporter emits.

### Discovery Strategy

#### Filesystem Volumes (VolumeMode=Filesystem)

```
/var/lib/kubelet/pods/<uid>/volumes/kubernetes.io~csi/<pv-name>/
├── vol_data.json    ← CSI volume identity (volumeHandle, driverName)
└── mount/           ← stat() this → st_dev gives device major:minor
```

1. Walk `pods/*/volumes/kubernetes.io~csi/*/`
2. Read `vol_data.json` for volume identity
3. `stat()` the `mount/` subdirectory → `st_dev` gives major:minor
4. Verify mount propagation (st_dev must differ from parent directory)
5. Resolve via `/sys/dev/block/{major}:{minor}` → kernel device name
6. For DM devices, check if multipath (one hop through LUKS if present)

#### Block Volumes (VolumeMode=Block)

```
/var/lib/kubelet/pods/<uid>/volumeDevices/kubernetes.io~csi/<pv-name>/
└── <pv-name>        ← device file; stat() → st_rdev gives major:minor

/var/lib/kubelet/plugins/kubernetes.io~csi/volumeDevices/<pv-name>/data/
└── vol_data.json    ← CSI volume identity
```

1. Walk `pods/*/volumeDevices/kubernetes.io~csi/*/`
2. `stat()` the device file → `st_rdev` gives major:minor
3. Read `vol_data.json` from the plugin staging path for volume identity
4. Same sysfs resolution as filesystem volumes

#### Network Filesystem Filtering

NFS, CephFS, and other network filesystems use pseudo-device numbers (major=0) that don't exist in `/sys/dev/block/`. Sysfs resolution fails naturally — no fragile filesystem-type list needed.

### Metrics

| Metric | Type | Labels | Description |
|---|---|---|---|
| `csi_volume_node_device_info` | Gauge (1) | `node`, `volume_handle`, `driver`, `device` | Maps a CSI volume to its block device |
| `csi_volume_device_exporter_discovery_errors_total` | Counter | `node` | Discovery cycle errors |

### Alert Rules

| Alert | Severity | Condition |
|---|---|---|
| `CSIVolumeMultipathDegraded` | warning | PV-backed multipath device has non-active paths |
| `CSIVolumeMultipathLost` | critical | All multipath paths failed |
| `CSIVolumeNVMeSubsystemDegraded` | warning | NVMe-oF subsystem has non-live controllers |
| `CSIVolumeNVMeSubsystemLost` | critical | All NVMe-oF controllers dead |
| `CSIVolumeDeviceExporterDown` | warning | Exporter not scraped for >10 minutes |

Alerts use PromQL joins between `csi_volume_node_device_info` and `node_dmmultipath_*` / `node_nvmesubsystem_*` metrics from node_exporter.

### Security Model

- Non-privileged container (no capabilities, read-only rootfs)
- `runAsUser: 0` (needed for sysfs readdir, not for privilege escalation)
- Read-only hostPath mounts: `/var/lib/kubelet` (with `mountPropagation: HostToContainer`), `/sys`
- No Kubernetes API access
- No network access beyond serving metrics on a local port

### Deployment

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: csi-volume-device-exporter
spec:
  template:
    spec:
      containers:
        - name: exporter
          image: quay.io/csiaddons/csi-volume-device-exporter:latest
          args:
            - --listen-address=:9710
            - --poll-interval=30s
          ports:
            - name: metrics
              containerPort: 9710
          securityContext:
            privileged: false
            readOnlyRootFilesystem: true
            runAsUser: 0
            allowPrivilegeEscalation: false
            capabilities:
              drop: [ALL]
          volumeMounts:
            - name: kubelet
              mountPath: /host/kubelet
              readOnly: true
              mountPropagation: HostToContainer
            - name: host-sys
              mountPath: /host/sys
              readOnly: true
      volumes:
        - name: kubelet
          hostPath:
            path: /var/lib/kubelet
        - name: host-sys
          hostPath:
            path: /sys
```

## Relationship to Existing Work

- **node_exporter#3581** (merged) — `dmmultipath` collector exposing per-path state
- **node_exporter#3579** (merged) — `nvmesubsystem` collector exposing NVMe-oF health
- **openshift-virtualization/csi-volume-device-exporter#1** — reference implementation (approved)
- **csi-addons/kubernetes-csi-addons#1039** (closed) — previous attempt; closed because framing as a gRPC extension was an architectural mismatch. This proposal takes a different approach: the exporter is a standalone component managed by the csi-addons operator, not a gRPC RPC.

## Deployment Lifecycle

The csi-addons operator deploys the exporter **unconditionally** when the operator is installed. The exporter is benign on clusters without multipath or NVMe-oF storage — it emits volume-to-device mappings but the correlation alerts simply don't fire (the PromQL join produces no matches when `node_dmmultipath_*` / `node_nvmesubsystem_*` metrics are absent).

The operator manages three resources as a unit:
- **DaemonSet** — the exporter pods
- **PrometheusRule** — alert rules (upgraded atomically with the exporter to keep PromQL in sync with metric names)
- **PodMonitor** — scrape configuration

This approach:
- Eliminates configuration burden (no opt-in flag to discover and enable)
- Guarantees alert rules always match the running exporter version
- Works on both ODF clusters (where csi-addons is bundled) and non-ODF clusters (standalone install from OperatorHub)

## Open Questions

1. Should the exporter support optional driver-specific discovery plugins (e.g., Trident JSON, HPE JSON) or keep only the universal kubelet-based approach for upstream?
