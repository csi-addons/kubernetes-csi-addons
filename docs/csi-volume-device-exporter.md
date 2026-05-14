# CSI Volume Device Exporter

The CSI Volume Device Exporter is a Prometheus exporter that maps CSI
PersistentVolumes to their underlying block devices on each node. This mapping
enables cross-referencing storage path health metrics (e.g., DM-multipath path
state, NVMe-oF subsystem health) with Kubernetes PersistentVolumes.

## Usage

The exporter is deployed as a **DaemonSet** that runs on every node:

```bash
kubectl apply -f deploy/exporter/daemonset.yaml
kubectl apply -f deploy/exporter/podmonitor.yaml
```

### Building

```bash
# Build the binary
go build -o bin/csi-volume-device-exporter ./cmd/csi-volume-device-exporter

# Build the container image
make docker-build-exporter
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--listen-address` | `:9091` | Address to listen on for metrics and healthz |
| `--poll-interval` | `30s` | Interval between discovery cycles |
| `--log-level` | `info` | Log level (`debug`, `info`, `warn`, `error`) |
| `--host-proc` | `/host/proc` | Path to host `/proc` mount inside container |
| `--host-sys` | `/host/sys` | Path to host `/sys` mount inside container |
| `--host-kubelet` | `/host/kubelet` | Path to host kubelet root mount inside container |
| `--kubelet-root` | `/var/lib/kubelet` | Actual kubelet root path on the host (for mountinfo matching) |
| `--host-trident-tracking` | `/host/trident/tracking` | Path to host Trident tracking dir inside container |

The `NODE_NAME` environment variable is **required** and should be set via the
Kubernetes downward API (`spec.nodeName`).

## Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `csi_volume_node_device_info` | Gauge | Maps CSI volumes to node block devices (value always 1) |
| `csi_volume_device_exporter_discovery_duration_seconds` | Histogram | Duration of volume discovery by discoverer |
| `csi_volume_device_exporter_discovery_errors_total` | Counter | Total discovery errors by discoverer |
| `csi_volume_device_exporter_volumes_discovered` | Gauge | Number of volumes discovered per driver |
| `csi_volume_device_exporter_last_successful_discovery_timestamp_seconds` | Gauge | Unix timestamp of last successful discovery cycle |

## Alert Rules

The exporter ships with 5 Prometheus alert rules:

| Alert | Severity | Description |
|-------|----------|-------------|
| `CSIVolumeMultipathDegraded` | warning | At least one multipath path is down |
| `CSIVolumeMultipathLost` | critical | All multipath paths are down |
| `CSIVolumeNVMeSubsystemDegraded` | warning | At least one NVMe-oF controller is not live |
| `CSIVolumeNVMeSubsystemLost` | critical | All NVMe-oF controllers are dead |
| `CSIVolumeDeviceExporterDown` | warning | Exporter is not being scraped by Prometheus |

Runbooks for each alert are in
[docs/csi-volume-device-exporter/runbooks/](csi-volume-device-exporter/runbooks/).

## Discovery Engines

The exporter uses multiple discovery engines that run in parallel:

1. **Kubelet Discoverer** — Parses `vol_data.json` from kubelet's CSI plugin
   directory and correlates with `/proc/1/mountinfo` to resolve block devices
   via `/sys/dev/block/`.

2. **Trident Discoverer** — Reads NetApp Trident tracking JSON files for direct
   device path mapping.

3. **HPE Discoverer** — Reads HPE CSI driver `deviceInfo.json` files.

All discoverers resolve LUKS-over-multipath stacks, returning the underlying
multipath device rather than the LUKS dm device.

## OpenShift Deployment

On OpenShift, deploy the exporter with the custom SCC:

```bash
NAMESPACE=openshift-cnv
kubectl apply -n $NAMESPACE -f deploy/exporter/scc.yaml
kubectl apply -n $NAMESPACE -f deploy/exporter/daemonset.yaml
kubectl apply -n $NAMESPACE -f deploy/exporter/podmonitor.yaml
```

The `openshift-cnv` namespace carries `openshift.io/cluster-monitoring=true` so
the platform Prometheus scrapes the exporter alongside node-exporter, enabling
the PromQL join with `node_dmmultipath_path_state`.
