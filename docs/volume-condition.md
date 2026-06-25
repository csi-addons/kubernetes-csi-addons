# Volume Condition Reporter

The Volume Condition Reporter uses the [Container Storage Interface
Specification's `NodeGetVolumeStats` operation][nodegetvolumestats] to detect
if a PersistentVolume has an _abnormal_ condition. CSI drivers can return the
condition of a volume in the `NodeVolumeStatsResponse` message.

## Usage

The Volume Condition Reporter is disabled by default. Enabling the
`--enable-volume-condition` for the CSI-Addons sidecar starts the Volume
Condition Reporter.

## Abnormal Volume Condition reporting

Once enabled, the sidecar reports the healthy and abnormal volume conditions as follows:

- logs in the CSI-Addons sidecar
- Event for the PersistentVolumeClaim
- a health annotation on the PersistentVolumeClaim

The health annotation on the PersistentVolumeClaim is written per node:

```yaml
csiaddons.openshift.io/volumehealth.<node-uid>: '{"state":"healthy|unhealthy","lastChecked":"<RFC3339>","since":"<RFC3339>","node":"<node-name>"}'
```

The sidecar always writes both states explicitly (`healthy` and `unhealthy`) and
updates `lastChecked` on every tick.

- `state` is always `healthy` or `unhealthy`
- `lastChecked` is refreshed on every tick
- `since` is set when the sidecar first observes the current state, and is
  kept unchanged while that state does not change

Each sidecar instance only updates its own
`csiaddons.openshift.io/volumehealth.<node-uid>` key and never modifies keys
written by sidecars running on other nodes.

Users will see the Event in their Namespace, and also when they describe (with
`kubectl describe ...`) the PersistentVolumeClaim.

### Stale Annotation Cleanup

Stale health annotations are cleaned up by the addons-controller with periodic
cleanup. The cleanup removes complete per-node keys when their
`lastChecked` value is older than a configured stale threshold.

Cleanup behavior:

- cleanup is eventual, not immediate
- cleanup interval and stale threshold are configurable
- cleanup can be disabled with `--enable-volume-health-cleanup=false`
  (default is enabled)
- only stale `csiaddons.openshift.io/volumehealth.<node-uid>` keys are removed
- other annotation keys on the PVC are preserved

### Future Enhancements

Additional options for reporting include:

- include the volume condition in the metrics (similar to [KEP-4132][k8s_kep])
- generate an event for one or more of

  1. the PersistentVolume
  1. the Pod that uses the PersistentVolumeClaim
  1. the Node where the volume condition is abnormal

- annotate one or more of

  1. the PersistentVolume
  1. the Pod that uses the PersistentVolumeClaim
  1. the Node where the volume condition is abnormal
     > unlikely acceptable, needs permissions to the Node object

## Potential Consumers of Abnormal Volume Condition check results

More feedback on the reporting and recovery steps are needed, but there are
potential approaches that could use the reported volume condition:

- [Rook](https://rook.io) is a Kubernetes Operator that is able to [Network
  Fence][rook_fencing] a workernode where a Ceph volume is unhealthy.

- [Node Problem Detector][k8s_npd] provides a generic interface for reporting
  problems on a node. A project like [medik8s](https://medik8s.io/) can remedy
  node problems once they are reported.

## Dependencies

The `NodeGetVolumeStats` operation in the current CSI Specification (v1.8.0)
defines the `VolumeCondition` as an _alpha_ feature. Very few CSI-drivers seem
to implement the volume condition at the moment. Drivers that implement the
feature, are required to expose `VOLUME_CONDITION` as a
`NodeServiceCapability`, otherwise the Volume Condition Reporter will not be
able to check the condition of the volume.

## Required Permissions (RBAC)

When a Kubernetes cluster uses Role Based Access Control (RBAC) like OpenShift,
the CSI-Addons sidecar requires extra permissions to check and report the
volume condition.

```yaml
---
# permissions for csi-addons sidecar to create events.
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: csiaddons-events-editor-role
rules:
  - apiGroups:
      - ""
    resources:
      - events
    verbs:
      - create
      - delete
      - get
      - list
      - patch
      - update
      - watch
  - apiGroups:
      - ""
    resources:
      - persistentvolumeclaims
    verbs:
      - get
---
# permissions for csi-addons sidecar to patch PVC health annotation.
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: csiaddons-pvc-editor-role
rules:
  - apiGroups:
      - ""
    resources:
      - persistentvolumeclaims
    verbs:
      - get
      - patch
```

[nodegetvolumestats]: https://github.com/container-storage-interface/spec/blob/master/spec.md#nodegetvolumestats
[rook_fencing]: https://rook.github.io/docs/rook/v1.12/Storage-Configuration/Block-Storage-RBD/block-storage/#handling-node-loss
[k8s_npd]: https://github.com/kubernetes/node-problem-detector/
[k8s_kep]: https://github.com/kubernetes/enhancements/blob/master/keps/sig-storage/1432-volume-health-monitor/README.md
