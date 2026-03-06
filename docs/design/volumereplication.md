# Summary

This document discusses the csi-addons API that allows replication of volumes to another peer cluster.

## Terminologies

For the sake of simplicity, we will be using some abbreviations in the document which are as follows:

- VR - VolumeReplication
- VRC - VolumeReplicationClass
- K8s - Kubernetes
- DR - Disaster Recovery
- PVC - Persistent Volume Claim
- PV - Persistent Volume
- Primary - Cluster where the applications are currently running
- Secondary - Cluster where the application data is being replicated

## Motivation

Most of the storage vendors provide a mechanism to replicate data to other nodes, clusters, peers in order to achieve crash consistency for the data stored on the volumes. But, there are no K8s APIs that provide crash consistency at real time. We have K8s VolumeGroupSnapshot APIs provided but that again involves adding another application to take backup and restore it on the secondary cluster.
Therefore, there was a need to have K8s APIs that provide replication of volumes and a group of volumes at real time to provide support for Disaster Recovery (DR).

## Goal

- Provide APIs to support replication of a group of volumes.

## Design for VolumeReplication

This section discusses the API definition, implementation nuances of VolumeReplication (VR) and VolumeReplicationClass (VRC).

VolumeReplication is a namespaced scope resource that contains reference to a storage object (a PVC) and VolumeReplicationClass corresponding to the driver providing replication. VR contains the details that explains which PVC is being replicated and what’s the current status of replication along with errors, if any.

VolumeReplicationClass (VRC) is a cluster scoped resource that contains driver related configuration parameters required for replication. It is similar to the definition of a StorageClass (SC) in K8s but crafted for VolumeReplication.

### Implementation/Workflow Details

![VolumeReplication Workflow Diagram](volumereplication_arch.svg)

To start with, users should create the VRC which contains the provisioner (csi driver) that supports mirroring capability and the volumes (PVCs) should be created using the same provisioner. Also, add the necessary parameters required by the storage vendor to mirror the volume to a peer cluster like the secret name, namespace.

To start replication of the volume, the user needs to create a VR that contains the PVC as the dataSource which would be mirrored/replicated to a peer cluster, along with other required fields like the volumeReplicationClass (shared below) and the desired replicationState of the VR.
The VR supports PVC and VolumeGroupReplication as the `dataSource` and the storage vendor is responsible for adding necessary checks to validate that if the dataSource is a single volume or a volumegroup and proceed accordingly. `replicationState` is used to determine if the VR is replicating to a secondary cluster i.e. `primary` or is on replicating end of the mirroring i.e. `secondary`.

If [RamenDR](https://github.com/RamenDR/ramen) is the extension/manager being used to manage DR for the workloads then, the storage vendor needs to add the below set of `Status` and `status.Conditions` to the VR for ramen to read the VR status and perform failover/relocate on the workload properly.

### VolumeReplication Status Examples

The following sections provide detailed examples of VolumeReplication status for different replication states. Each status example includes the necessary conditions and state information that RamenDR uses to manage disaster recovery operations.
The condition's `Reason`, `Status`, and `Type` and the `status.State` must match the patterns shown below for RamenDR to correctly identify and manage the replication state.

#### Primary Status

**Description:** The Primary status indicates that the volume is actively serving I/O operations and is replicating data to a secondary cluster. This is the active state where the volume is being written to by applications, and changes are being mirrored to the secondary site for disaster recovery purposes. The volume is healthy and not in a degraded or resyncing state.

Sample `status` of VR when the volume is promoted to `Primary` successfully:

```yaml
status:
  conditions:
    - message: volume is promoted to primary and replicating to secondary
      reason: Promoted
      status: "True"
      type: Completed
    - message: volume is healthy
      reason: Healthy
      status: "False"
      type: Degraded
    - message: volume is not resyncing
      reason: NotResyncing
      status: "False"
      type: Resyncing
    - message: volume is validated and met all prerequisites
      reason: PrerequisiteMet
      status: "True"
      type: Validated
    - message: "volume is replicating: local image is primary"
      reason: Replicating
      status: "True"
      type: Replicating
  lastCompletionTime: "2026-02-17T07:41:58Z"
  lastSyncBytes: 9793536
  lastSyncDuration: 0s
  lastSyncTime: "2026-02-17T07:40:01Z"
  message: volume is marked primary
  state: Primary
```

#### Secondary Status

**Description:** The Secondary status indicates that the volume is in a read-only state and is receiving replicated data from the primary cluster. The volume is not actively serving application I/O but is being kept in sync with the primary volume. This state is typical for the standby cluster in a disaster recovery setup.
The volume is marked as degraded because it's in secondary mode and not available for writes.

Sample `status` of VR when the volume is demoted to `Secondary`:

```yaml
status:
  conditions:
    - message: volume is demoted to secondary
      reason: Demoted
      status: "True"
      type: Completed
    - message: volume is degraded
      reason: VolumeDegraded
      status: "True"
      type: Degraded
    - message: volume is not resyncing
      reason: NotResyncing
      status: "False"
      type: Resyncing
    - message: volume is validated and met all prerequisites
      reason: PrerequisiteMet
      status: "True"
      type: Validated
  lastCompletionTime: "2026-02-17T07:41:58Z"
  message: volume is marked secondary
  state: Secondary
```

#### Demoted Status

**Description:** The Demoted status represents a transitional state where a volume that was previously Primary has been demoted. This typically occurs during a planned failover or relocate operation. The volume is no longer accepting writes and is in a degraded state. This is similar to the Secondary status but explicitly indicates the demotion operation has been completed.

Sample `status` of VR when the volume is `Demoted`:

```yaml
status:
  conditions:
    - message: volume is demoted to secondary
      reason: Demoted
      status: "True"
      type: Completed
    - message: volume is degraded
      reason: VolumeDegraded
      status: "True"
      type: Degraded
    - message: volume is not resyncing
      reason: NotResyncing
      status: "False"
      type: Resyncing
  lastCompletionTime: "2026-02-17T07:42:15Z"
  message: volume is marked secondary
  state: Secondary
```

#### Resyncing Status

**Description:** The Resyncing status indicates that the volume is actively synchronizing data between the primary and secondary sites. This state occurs when replication has been interrupted (due to network issues, cluster downtime, or other failures) and the volumes are now catching up.
During resyncing, the volume is marked as degraded and the Resyncing condition is set to True. The volume may experience degraded performance as it transfers the delta of changes that occurred during the interruption.

Sample `status` of VR when the volume is `Resyncing`:

```yaml
status:
  conditions:
    - message: volume is demoted to secondary
      reason: Demoted
      status: "True"
      type: Completed
    - message: volume is degraded
      reason: VolumeDegraded
      status: "True"
      type: Degraded
    - message: volume is resyncing changes from primary to secondary
      reason: ResyncTriggered
      status: "True"
      type: Resyncing
  lastCompletionTime: "2026-02-17T07:40:00Z"
  lastSyncBytes: 5242880
  lastSyncDuration: 15s
  lastSyncTime: "2026-02-17T07:42:30Z"
  message: volume is resyncing
  state: Secondary
```

## API definition of VolumeReplication (VR) CRD

- [VR Type Definition](https://github.com/csi-addons/kubernetes-csi-addons/blob/main/api/replication.storage/v1alpha1/volumereplication_types.go)
- [VR CRD Definition](https://github.com/csi-addons/kubernetes-csi-addons/blob/main/config/crd/bases/replication.storage.openshift.io_volumereplications.yaml)

## API definition of VolumeReplicationClass (VR) CRD

- [VRClass Type Definition](https://github.com/csi-addons/kubernetes-csi-addons/blob/main/api/replication.storage/v1alpha1/volumereplicationclass_types.go)
- [VRClass CRD Definition](https://github.com/csi-addons/kubernetes-csi-addons/blob/main/config/crd/bases/replication.storage.openshift.io_volumereplicationclasses.yaml)

## Example YAML files

- VolumeReplicationClass CR:

```yaml
apiVersion: replication.storage.openshift.io/v1alpha1
kind: VolumeReplicationClass
metadata:
  name: volumereplicationclass-sample
spec:
  provisioner: example.provisioner.io
  parameters:
    replication.storage.openshift.io/replication-secret-name: secret-name
    replication.storage.openshift.io/replication-secret-namespace: secret-namespace
    # schedulingInterval is a vendor specific parameter. It is used to set the
    # replication scheduling interval for storage volumes that are replication
    # enabled using related VolumeReplication resource
    schedulingInterval: 1m
```

- VolumeReplication CR:

```yaml
apiVersion: replication.storage.openshift.io/v1alpha1
kind: VolumeReplication
metadata:
  name: volumereplication-sample
  namespace: default
spec:
  volumeReplicationClass: volumereplicationclass-sample
  replicationState: primary
  replicationHandle: replicationHandle # optional
  dataSource:
    kind: PersistentVolumeClaim
    name: myPersistentVolumeClaim # should be in same namespace as VolumeReplication
```
