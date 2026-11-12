# Summary

This document discusses the csi-addons API that allows replication of volume groups to a secondary cluster.

## Terminologies

For the sake of simplicity, we will be using some abbreviations in the document which are as follows:

- DR - Disaster Recovery
- K8s - Kubernetes
- PVC - Persistent Volume Claim
- PV - Persistent Volume
- Primary - The cluster where the application workloads are actively running add data is written to. It initiates the replication to the secondary cluster.
- Secondary - The cluster that receives replicated data from the primary cluster.
- SC - Storage Class
- VGR - VolumeGroupReplication
- VGRC - VolumeGroupReplicationClass
- VR - VolumeReplication
- VRC - VolumeReplicationClass

## Motivation

Most of the storage vendors provide a mechanism to replicate data to other nodes, clusters, peers. But, there are no K8s APIs that provide replication of a group of volumes at real time. We have K8s VolumeGroupSnapshot APIs provided but that again involves adding another application to take backup and restore it on the secondary cluster.
Therefore, there was a need to have K8s APIs that represents group of volumes and replicate the group of volumes at real time to provide support for Disaster Recovery (DR).

## Goal

- Provide APIs to support replication of a group of volumes.

## Design for VolumeGroupReplication

This section discusses the API definition, implementation nuances of VolumeGroupReplication (VGR), VolumeGroupReplicationContent (VGRContent) and VolumeGroupReplicationClass (VGRC).

VolumeGroupReplication (VGR) is a namespaced scope resource that contains references to the PVCs that are to be grouped and replicated to a secondary cluster.

VolumeGroupReplicationContent (VGRContent) is a clusterscoped resource that contains volume grouping related information and reference to the PVs part of the group.

VolumeGroupReplicationClass (VGRC) is a clusterscoped resource that contains driver related configuration parameters required for group replication. It is similar to the definition of a StorageClass (SC) in K8s but crafted for VolumeGroupReplication.

### Implementation/Workflow Details

![VolumeGroupReplication Workflow Diagram](volumegroupreplication_arch.svg)

To start with, admin should create the VRC, VGRC which contains the provisioner (csi driver) that supports replication capability and the volumes (PVCs) should be created using the same provisioner. Also, add the necessary parameters required by the storage vendor to mirror the group to a secondary cluster like the secret name, namespace.

To start replication of the group, the user needs to create a VGR that contains the PVC label selector as the source which would be used to filter the PVCs that are supposed to be part of the same group and replicated to a secondary cluster, along with other required fields like the VRC (created above), VGRC (created above), and the desired replicationState of the VGR.
The VGR only groups PVCs that are part of the same namespace as that of VGR.
The `replicationState` indicates the current role of the VGR such as `primary` means the VGR is in the active replication role, sending data to a secondary cluster, or `secondary` that means the VGR is in the passive role, receiving replicated data from the primary cluster (having VGR is not manadatory here when its in passive role).

VGR’s spec consists of a field named external which signifies if the CR should be reconciled and managed by the csi-addons volumegroupreplication_controller or by the storage vendor specific implementation of the VGR controller.
When set to true the CR won’t be reconciled by the csi-addons “volumegroupreplication_controller”, and the storage vendor is expected to deploy a controller that manages the CR. The value is set to `false` by default.

When a vendor implements their storage specific controller to manage the VGR using the external field, it is the responsibility of the operator/controller watching the VGR to add the necessary validations and issue the required RPCs to enable mirroring on the group at the storage level.
The VGR creation flow involves creating a VGRContent and a VR as well respectively for it. VGRContent is a cluster scoped CR that is created per VGR. It contains a reference to the VGR in its spec that is used to identify the VGR that manages it.

When a VGR is created, the controller should list all the PVCs that match the label selector and can be a part of the volume group. The controller should then be creating a VGRContent for the VGR that manages the group level operations like creating, modifying, deleting etc, which can again be delegated to another controller that manages the VGRContent.
Once the group is created the VGRContent should be updated with the `volumeGroupReplicationHandle` that can be checked by the VGR controller to go ahead and create the VR which would manage the mirroring based operations.
The VGRContent’s Status has to be populated with `volumeHandles` of all the PV’s that are part of the group, where these PVs belong to the PVCs which are filtered using the source in VGR’s Spec.And, in a similar way the VGR’s Status.persistentVolumeClaimsRefList field should be updated with the names of all the PVCs that are part of the group.

In other words, the VGR controller shouldn’t be managing the group managing and mirroring operations on its own, instead it can make use of the existing VR controller which manages mirroring operations of a volume and delegate the group operations to a separate VGRContent controller.

For reference, one can check the implementation of csi-addons controllers for the same:

- [volumereplication_controller](https://github.com/csi-addons/kubernetes-csi-addons/blob/main/internal/controller/replication.storage/volumereplication_controller.go) - It manages the mirroring operations for a volume/group.
- [volumegroupreplication_controller](https://github.com/csi-addons/kubernetes-csi-addons/blob/main/internal/controller/replication.storage/volumegroupreplication_controller.go) - It manages the flow of VGR and necessary validations to keep the VGR as the source of truth for the user.
- [volumegroupreplicationcontent_controller](https://github.com/csi-addons/kubernetes-csi-addons/blob/main/internal/controller/replication.storage/volumegroupreplicationcontent_controller.go) - It manages the group level operations for the VGR.

VGR has a Status field that is basically the same as that of VolumeReplication's Status field and it can be utilized by the storage vendor to populate the current status of replication to the end user and report any errors as such at the CR level.

If a DR orchestrator is being used to manage DR for the workloads then, the storage vendor needs to add the below set of `Status` and `status.Conditions` to the VGR for the orchestrator to read the VGR status and perform failover/relocate on the workload properly.

### VolumeGroupReplication Status Examples

The following sections provide detailed examples of VolumeGroupReplication status for different replication states. Each status example includes the necessary conditions and state information that a DR orchestrator uses to manage disaster recovery operations.
The condition's `Reason`, `Status`, and `Type` and the `status.State` must match the patterns shown below for a DR orchestrator to correctly identify and manage the replication state.

#### Primary Status

**Description:** The Primary status indicates that the volume group is actively serving I/O operations and is replicating data to a secondary cluster. This is the active state where the volume group is being written to by applications, and changes are being mirrored to the secondary site for disaster recovery purposes. The volume group is healthy and not in a degraded or resyncing state.

Sample `status` of VGR when the volume group is promoted to `Primary` successfully:

```yaml
status:
  conditions:
    - message: volume group is promoted to primary and replicating to secondary
      reason: Promoted
      status: "True"
      type: Completed
    - message: volume group is healthy
      reason: Healthy
      status: "False"
      type: Degraded
    - message: volume group is not resyncing
      reason: NotResyncing
      status: "False"
      type: Resyncing
    - message: volume group is validated and met all prerequisites
      reason: PrerequisiteMet
      status: "True"
      type: Validated
    - message: "volume group is replicating: local group is primary"
      reason: Replicating
      status: "True"
      type: Replicating
  lastCompletionTime: "2026-02-17T07:41:58Z"
  lastSyncBytes: 9793536
  lastSyncDuration: 0s
  lastSyncTime: "2026-02-17T07:40:01Z"
  message: volume group is marked primary
  state: Primary
```

#### Secondary Status

**Description:** The Secondary status indicates that the volume group is in a read-only state and is receiving replicated data from the primary cluster. The volume group is not actively serving application I/O but is being kept in sync with the primary volume group. This state is typical for the standby cluster in a disaster recovery setup.
The volume group is marked as degraded because it's in secondary mode and not available for writes.

Sample `status` of VGR when the volume group is demoted to `Secondary`:

```yaml
status:
  conditions:
    - message: volume group is demoted to secondary
      reason: Demoted
      status: "True"
      type: Completed
    - message: volume group is degraded
      reason: VolumeDegraded
      status: "True"
      type: Degraded
    - message: volume group is not resyncing
      reason: NotResyncing
      status: "False"
      type: Resyncing
    - message: volume group is validated and met all prerequisites
      reason: PrerequisiteMet
      status: "True"
      type: Validated
  lastCompletionTime: "2026-02-17T07:41:58Z"
  message: volume group is marked secondary
  state: Secondary
```

#### Resyncing Status

**Description:** The Resyncing status indicates that the volume group is actively synchronizing data between the primary and secondary sites. This state occurs when replication has been interrupted (due to network issues, cluster downtime, or other failures) and the volume groups are now catching up.
During resyncing, the volume group is marked as degraded and the Resyncing condition is set to True. The volume group may experience degraded performance as it transfers the delta of changes that occurred during the interruption.

Sample `status` of VGR when the volume group is `Resyncing`:

```yaml
status:
  conditions:
    - message: volume group is demoted to secondary
      reason: Demoted
      status: "True"
      type: Completed
    - message: volume group is degraded
      reason: VolumeDegraded
      status: "True"
      type: Degraded
    - message: volume group is resyncing changes from primary to secondary
      reason: ResyncTriggered
      status: "True"
      type: Resyncing
  lastCompletionTime: "2026-02-17T07:40:00Z"
  lastSyncBytes: 5242880
  lastSyncDuration: 15s
  lastSyncTime: "2026-02-17T07:42:30Z"
  message: volume group is resyncing
  state: Secondary
```

## Disaster Recovery Workflows

This section describes the workflows for performing disaster recovery operations using VolumeGroupReplication with a DR orchestrator. These workflows explain how to perform failover (unplanned recovery) and relocate (planned migration) operations for volume groups, along with the conditions that must be met before proceeding.

### Failover Workflow

**Failover** is an unplanned disaster recovery operation performed when the primary cluster becomes unavailable due to failure, network partition, or other catastrophic events. The goal is to quickly restore application availability on a surviving secondary cluster.

#### Prerequisites for Failover

Before initiating a failover operation, verify the following conditions:

1. **Primary Cluster Unavailable**: The primary cluster is confirmed to be down or unreachable
2. **Secondary Cluster Healthy**: The secondary cluster is operational and accessible
3. **VolumeGroupReplication Exists**: A VolumeGroupReplication resource must exist on the secondary cluster (see note below)
4. **VolumeGroupReplication Status**: The VGR on the secondary cluster should show one of these states:
   - `state: Secondary` with `Degraded: True` and `Completed: True`
   - `state: Secondary` with `Resyncing: True` (data may be slightly out of sync)

**Important Note**: The VolumeGroupReplication resource might not be created on the secondary cluster in some deployment scenarios. If the VGR does not exist on the secondary cluster, you must create it before proceeding with the failover.
The VGR should use the same label selector to identify PVCs and reference the appropriate VolumeReplicationClass and VolumeGroupReplicationClass with `replicationState: secondary`.

### Pre-Failover State

Before a failover operation can be performed, the system must be in a specific state. This section describes what a healthy pre-failover setup looks like, based on existing DR orchestrator patterns where VolumeGroupReplication resources exist on both clusters.

#### Cluster Setup Overview for Failover

In a typical DR setup with a DR orchestrator:

- **Primary Cluster**: Runs the application workload and has a VGR in `primary` state
- **Secondary Cluster**: Receives replicated data and has a VGR in `secondary` state
- **Single VR per VGR**: Each VGR creates one VolumeReplication resource that manages the entire volume group

#### Pre-Failover State on Primary Cluster

On the primary cluster, before a disaster occurs, the VolumeGroupReplication should be in a healthy replicating state:

```yaml
apiVersion: replication.storage.openshift.io/v1alpha1
kind: VolumeGroupReplication
metadata:
  name: app-vgr
  namespace: app-namespace
spec:
  volumeReplicationClassName: vrc-sample
  volumeGroupReplicationClassName: vgrc-sample
  volumeReplicationName: app-vgr-vr # Single VR for the entire group
  volumeGroupReplicationContentName: vgrc-app-vgr
  replicationState: primary
  autoResync: true
  source:
    selector:
      matchLabels:
        app: myapp
        replication-group: group1
status:
  conditions:
    - lastTransitionTime: "2026-03-20T10:00:00Z"
      message: volume group is promoted to primary and replicating to secondary
      reason: Promoted
      status: "True"
      type: Completed
    - lastTransitionTime: "2026-03-20T10:00:00Z"
      message: volume group is healthy
      reason: Healthy
      status: "False"
      type: Degraded
    - lastTransitionTime: "2026-03-20T10:00:00Z"
      message: volume group is not resyncing
      reason: NotResyncing
      status: "False"
      type: Resyncing
    - lastTransitionTime: "2026-03-20T10:00:00Z"
      message: volume group is validated and met all prerequisites
      reason: PrerequisiteMet
      status: "True"
      type: Validated
    - lastTransitionTime: "2026-03-20T10:00:00Z"
      message: "volume group is replicating: local group is primary"
      reason: Replicating
      status: "True"
      type: Replicating
  lastCompletionTime: "2026-03-20T10:00:00Z"
  lastSyncBytes: 10485760
  lastSyncDuration: 2s
  lastSyncTime: "2026-03-23T10:30:00Z"
  message: volume group is marked primary
  observedGeneration: 1
  state: Primary
  persistentVolumeClaimsRefList:
    - name: data-pvc-1
    - name: data-pvc-2
    - name: data-pvc-3
```

**Key Indicators of Healthy Primary State:**

- `state: Primary` - Volume group is in primary role
- `Completed: True` with `reason: Promoted` - Last operation completed successfully
- `Degraded: False` with `reason: Healthy` - Volume group is healthy
- `Resyncing: False` - No resync in progress
- `Replicating: True` - Actively replicating to secondary
- `Validated: True` - All prerequisites met
- Recent `lastSyncTime` - Replication is active
- `persistentVolumeClaimsRefList` populated with all PVCs in the group

#### Pre-Failover State on Secondary Cluster

On the secondary cluster, the VolumeGroupReplication should be receiving replicated data:

```yaml
apiVersion: replication.storage.openshift.io/v1alpha1
kind: VolumeGroupReplication
metadata:
  name: app-vgr
  namespace: app-namespace
spec:
  volumeReplicationClassName: vrc-sample
  volumeGroupReplicationClassName: vgrc-sample
  volumeReplicationName: app-vgr-vr # Single VR for the entire group
  volumeGroupReplicationContentName: vgrc-app-vgr
  replicationState: secondary
  autoResync: true
  source:
    selector:
      matchLabels:
        app: myapp
        replication-group: group1
status:
  conditions:
    - lastTransitionTime: "2026-03-20T10:00:00Z"
      message: volume group is demoted to secondary
      reason: Demoted
      status: "True"
      type: Completed
    - lastTransitionTime: "2026-03-20T10:00:00Z"
      message: volume group is degraded
      reason: VolumeDegraded
      status: "True"
      type: Degraded
    - lastTransitionTime: "2026-03-20T10:00:00Z"
      message: volume group is not resyncing
      reason: NotResyncing
      status: "False"
      type: Resyncing
    - lastTransitionTime: "2026-03-20T10:00:00Z"
      message: volume group is validated and met all prerequisites
      reason: PrerequisiteMet
      status: "True"
      type: Validated
  lastCompletionTime: "2026-03-20T10:00:00Z"
  lastSyncBytes: 10485760
  lastSyncDuration: 2s
  lastSyncTime: "2026-03-23T10:30:00Z"
  message: volume group is marked secondary
  observedGeneration: 1
  state: Secondary
  persistentVolumeClaimsRefList:
    - name: data-pvc-1
    - name: data-pvc-2
    - name: data-pvc-3
```

**Key Indicators of Healthy Secondary State:**

- `state: Secondary` - Volume group is in secondary role
- `Completed: True` with `reason: Demoted` - In secondary state as expected
- `Degraded: True` with `reason: VolumeDegraded` - Expected for secondary (read-only)
- `Resyncing: False` - Fully synchronized with primary
- `Validated: True` - All prerequisites met
- Recent `lastSyncTime` - Receiving replication updates
- `persistentVolumeClaimsRefList` matches primary cluster

#### Associated VolumeReplication Resource

Each VGR creates a single VolumeReplication resource that manages the entire volume group. On the primary cluster:

```yaml
apiVersion: replication.storage.openshift.io/v1alpha1
kind: VolumeReplication
metadata:
  name: app-vgr-vr
  namespace: app-namespace
  labels:
    replication.storage.openshift.io/volume-group-replication-name: app-vgr
spec:
  volumeReplicationClass: vrc-sample
  replicationState: primary
  dataSource:
    apiGroup: ""
    kind: PersistentVolumeClaim
    name: data-pvc-1 # Reference to first PVC in group
  autoResync: true
status:
  conditions:
    - type: Completed
      status: "True"
      reason: Promoted
    - type: Degraded
      status: "False"
      reason: Healthy
    - type: Resyncing
      status: "False"
      reason: NotResyncing
  state: Primary
  lastSyncTime: "2026-03-23T10:30:00Z"
```

**Important Notes:**

- The VR's `dataSource` references one PVC from the group, but it manages replication for the entire volume group
- The VR's state and conditions should match the VGR's state
- A DR orchestrator should use the VGR as the primary interface and the VR is managed automatically

#### What Happens During Primary Cluster Failure

When the primary cluster fails:

1. The primary cluster becomes unreachable
2. The secondary cluster's VGR remains in `secondary` state with `Degraded: True`
3. The secondary cluster's VGR may show `Resyncing: True` if the failure interrupted replication
4. The `lastSyncTime` on the secondary stops updating
5. A DR orchestrator (like RamenDR) detects the failure and initiates failover

#### Failover Steps

1. **Verify or Create VolumeGroupReplication on Secondary Cluster**

Check if VolumeGroupReplication exists on the secondary cluster:

```bash
kubectl get volumegroupreplication <vgr-name> -n <namespace>
```

If the VGR does not exist, create it:

```yaml
apiVersion: replication.storage.openshift.io/v1alpha1
kind: VolumeGroupReplication
metadata:
  name: <vgr-name>
  namespace: <namespace>
spec:
  volumeReplicationClassName: <vrc-name>
  volumeGroupReplicationClassName: <vgrc-name>
  replicationState: secondary
  autoResync: true
  source:
    selector:
      matchLabels:
        <label-key>: <label-value>
```

Apply the VGR:

```bash
kubectl apply -f volumegroupreplication.yaml
```

Wait for the VGR to be created and reach a stable secondary state before proceeding. The controller will automatically create the VolumeGroupReplicationContent and a VolumeReplication resource for the volume group.

2. **Verify Secondary Cluster Status**

Check the VolumeGroupReplication status on the secondary cluster:

```bash
kubectl get volumegroupreplication <vgr-name> -n <namespace> -o yaml
```

Ensure the status shows:

```yaml
status:
  conditions:
    - type: Completed
      status: "True"
      reason: Demoted
    - type: Degraded
      status: "True"
      reason: VolumeDegraded
  state: Secondary
  persistentVolumeClaimsRefList:
    - pvc-1
    - pvc-2
    - pvc-3
```

Verify that all PVCs in the group are listed in `persistentVolumeClaimsRefList`.

3. **Promote Secondary to Primary**

Update the VolumeGroupReplication on the secondary cluster to promote it to primary:

```bash
kubectl patch volumegroupreplication <vgr-name> -n <namespace> --type merge -p '{"spec":{"replicationState":"primary"}}'
```

4. **Wait for Promotion to Complete**

Monitor the VGR status until it shows successful promotion:

```yaml
status:
  conditions:
    - type: Completed
      status: "True"
      reason: Promoted
    - type: Degraded
      status: "False"
      reason: Healthy
    - type: Resyncing
      status: "False"
      reason: NotResyncing
    - type: Replicating
      status: "True"
      reason: Replicating
  state: Primary
  persistentVolumeClaimsRefList:
    - pvc-1
    - pvc-2
    - pvc-3
```

**Wait Condition**: Do not proceed until `Completed: True`, `Degraded: False`, and `state: Primary` are confirmed.

5. **Verify Application Readiness**

Once the volume group is promoted to primary:

- Ensure all PVCs in the group are bound and accessible
- Verify that applications can mount and write to all volumes in the group
- Check that replication to a new secondary (if configured) is functioning for the entire group

#### Failover Considerations

- **VGR Creation**: Always verify that VolumeGroupReplication exists on the secondary cluster before attempting failover. If it doesn't exist, create it with `replicationState: secondary` and the correct label selector first
- **Consistency Group**: All volumes in the group are promoted together, maintaining crash consistency across the volume group
- **Data Loss Risk**: If the secondary was in `Resyncing` state during failover, some data written to the primary after the last successful sync may be lost
- **Split-Brain Prevention**: Ensure the failed primary cluster is properly fenced before promoting the secondary to prevent split-brain scenarios
- **RamenDR Integration**: When using RamenDR, it automatically handles VGR creation, state transitions, and ensures proper sequencing of operations for the entire volume group

### Relocate Workflow

**Relocate** is a planned migration operation that moves the application workload from one cluster to another. This is typically performed for maintenance, load balancing, or returning to a preferred cluster after a failover (failback).

#### Prerequisites for Relocate

Before initiating a relocate operation, verify the following conditions:

1. **Both Clusters Healthy**: Both source (current primary) and target (current secondary) clusters are operational
2. **VolumeGroupReplication Exists**: VolumeGroupReplication resources must exist on both clusters
3. **Replication Healthy**: The VGR on the primary cluster shows healthy replication status for all volumes in the group
4. **No Resyncing**: The secondary cluster is fully synchronized (not in resyncing state) for all volumes
5. **Application Quiesced**: Applications should be in a quiesced state or stopped to ensure data consistency across the volume group

**Important Note**: Similar to failover, the VolumeGroupReplication resource might not exist on the target secondary cluster. If it doesn't exist, create it with `replicationState: secondary` and the same label selector before proceeding with the relocate operation.

### Pre-Relocate State

Before a relocate (planned migration) operation, both clusters must be healthy and fully synchronized. This section describes the expected state based on existing DR orchestrator patterns.

#### Cluster Setup Overview for Relocate

In a healthy pre-relocate setup:

- **Current Primary Cluster**: Running application workload with VGR in `primary` state
- **Current Secondary Cluster**: Receiving replicated data with VGR in `secondary` state
- **Both Clusters Operational**: Both clusters are accessible and healthy
- **Full Synchronization**: Secondary is fully caught up with primary (no resyncing)

#### Pre-Relocate State on Current Primary Cluster

The VolumeGroupReplication on the current primary must show healthy replication:

```yaml
apiVersion: replication.storage.openshift.io/v1alpha1
kind: VolumeGroupReplication
metadata:
  name: app-vgr
  namespace: app-namespace
spec:
  volumeReplicationClassName: vrc-sample
  volumeGroupReplicationClassName: vgrc-sample
  volumeReplicationName: app-vgr-vr
  volumeGroupReplicationContentName: vgrc-app-vgr
  replicationState: primary
  autoResync: true
  source:
    selector:
      matchLabels:
        app: myapp
        replication-group: group1
status:
  conditions:
    - lastTransitionTime: "2026-03-20T10:00:00Z"
      message: volume group is promoted to primary and replicating to secondary
      reason: Promoted
      status: "True"
      type: Completed
    - lastTransitionTime: "2026-03-20T10:00:00Z"
      message: volume group is healthy
      reason: Healthy
      status: "False"
      type: Degraded
    - lastTransitionTime: "2026-03-20T10:00:00Z"
      message: volume group is not resyncing
      reason: NotResyncing
      status: "False"
      type: Resyncing
    - lastTransitionTime: "2026-03-20T10:00:00Z"
      message: volume group is validated and met all prerequisites
      reason: PrerequisiteMet
      status: "True"
      type: Validated
    - lastTransitionTime: "2026-03-20T10:00:00Z"
      message: "volume group is replicating: local group is primary"
      reason: Replicating
      status: "True"
      type: Replicating
  lastCompletionTime: "2026-03-20T10:00:00Z"
  lastSyncBytes: 8388608
  lastSyncDuration: 1s
  lastSyncTime: "2026-03-23T10:40:00Z"
  message: volume group is marked primary
  observedGeneration: 1
  state: Primary
  persistentVolumeClaimsRefList:
    - name: data-pvc-1
    - name: data-pvc-2
    - name: data-pvc-3
```

**Critical Pre-Relocate Checks for Primary:**

- `state: Primary` - Confirmed primary role
- `Degraded: False` - Volume group is healthy, not degraded
- `Resyncing: False` - No resync operations in progress
- `Replicating: True` - Actively replicating to secondary
- `Validated: True` - All prerequisites met
- Recent `lastSyncTime` (within last few minutes) - Active replication
- Low `lastSyncDuration` - Replication is performing well

#### Pre-Relocate State on Current Secondary Cluster

The VolumeGroupReplication on the target secondary must be fully synchronized:

```yaml
apiVersion: replication.storage.openshift.io/v1alpha1
kind: VolumeGroupReplication
metadata:
  name: app-vgr
  namespace: app-namespace
spec:
  volumeReplicationClassName: vrc-sample
  volumeGroupReplicationClassName: vgrc-sample
  volumeReplicationName: app-vgr-vr
  volumeGroupReplicationContentName: vgrc-app-vgr
  replicationState: secondary
  autoResync: true
  source:
    selector:
      matchLabels:
        app: myapp
        replication-group: group1
status:
  conditions:
    - lastTransitionTime: "2026-03-20T10:00:00Z"
      message: volume group is demoted to secondary
      reason: Demoted
      status: "True"
      type: Completed
    - lastTransitionTime: "2026-03-20T10:00:00Z"
      message: volume group is degraded
      reason: VolumeDegraded
      status: "True"
      type: Degraded
    - lastTransitionTime: "2026-03-20T10:00:00Z"
      message: volume group is not resyncing
      reason: NotResyncing
      status: "False"
      type: Resyncing
    - lastTransitionTime: "2026-03-20T10:00:00Z"
      message: volume group is validated and met all prerequisites
      reason: PrerequisiteMet
      status: "True"
      type: Validated
  lastCompletionTime: "2026-03-20T10:00:00Z"
  lastSyncBytes: 8388608
  lastSyncDuration: 1s
  lastSyncTime: "2026-03-23T10:40:00Z"
  message: volume group is marked secondary
  observedGeneration: 1
  state: Secondary
  persistentVolumeClaimsRefList:
    - name: data-pvc-1
    - name: data-pvc-2
    - name: data-pvc-3
```

**Critical Pre-Relocate Checks for Secondary:**

- `state: Secondary` - Confirmed secondary role
- `Degraded: True` - Expected for secondary (read-only state)
- `Resyncing: False` - **CRITICAL**: Must be fully synchronized, no resync in progress
- `Validated: True` - All prerequisites met
- Recent `lastSyncTime` matching primary - Receiving latest updates
- `lastSyncBytes` and `lastSyncDuration` similar to primary - Healthy replication
- `persistentVolumeClaimsRefList` matches primary exactly

#### Pre-Relocate Validation Checklist

Before initiating a relocate operation, verify:

1. **Both Clusters Accessible**

   ```bash
   # Test primary cluster
   kubectl --context=primary-cluster get nodes

   # Test secondary cluster
   kubectl --context=secondary-cluster get nodes
   ```

2. **VGR Exists on Both Clusters**

   ```bash
   # Primary cluster
   kubectl --context=primary-cluster get vgr app-vgr -n app-namespace

   # Secondary cluster
   kubectl --context=secondary-cluster get vgr app-vgr -n app-namespace
   ```

3. **Primary is Healthy and Replicating**

   ```bash
   kubectl --context=primary-cluster get vgr app-vgr -n app-namespace -o jsonpath='{.status.conditions[?(@.type=="Degraded")].status}'
   # Should return: False

   kubectl --context=primary-cluster get vgr app-vgr -n app-namespace -o jsonpath='{.status.conditions[?(@.type=="Replicating")].status}'
   # Should return: True
   ```

4. **Secondary is Synchronized (Not Resyncing)**

   ```bash
   kubectl --context=secondary-cluster get vgr app-vgr -n app-namespace -o jsonpath='{.status.conditions[?(@.type=="Resyncing")].status}'
   # Should return: False
   ```

5. **Recent Sync Times on Both Clusters**

   ```bash
   # Primary
   kubectl --context=primary-cluster get vgr app-vgr -n app-namespace -o jsonpath='{.status.lastSyncTime}'

   # Secondary
   kubectl --context=secondary-cluster get vgr app-vgr -n app-namespace -o jsonpath='{.status.lastSyncTime}'
   # Times should be very recent and close to each other
   ```

6. **PVC Lists Match**

   ```bash
   # Primary
   kubectl --context=primary-cluster get vgr app-vgr -n app-namespace -o jsonpath='{.status.persistentVolumeClaimsRefList[*].name}'

   # Secondary
   kubectl --context=secondary-cluster get vgr app-vgr -n app-namespace -o jsonpath='{.status.persistentVolumeClaimsRefList[*].name}'
   # Lists should be identical
   ```

#### What Makes Relocate Different from Failover

| Aspect                   | Failover                      | Relocate                            |
| ------------------------ | ----------------------------- | ----------------------------------- |
| **Primary Cluster**      | Unavailable/Failed            | Healthy and accessible              |
| **Planning**             | Unplanned/Emergency           | Planned/Scheduled                   |
| **Secondary Sync State** | May be resyncing              | Must be fully synchronized          |
| **Data Loss Risk**       | Possible if resyncing         | Zero (when done correctly)          |
| **Application Downtime** | Immediate (forced)            | Controlled (scheduled)              |
| **Primary Demotion**     | Not possible (cluster down)   | Required before secondary promotion |
| **Validation**           | Limited (primary unavailable) | Full validation on both clusters    |

#### Relocate Steps

1. **Verify or Create VolumeGroupReplication on Both Clusters**

   Check if VolumeGroupReplication exists on both primary and secondary clusters:

   ```bash
   # On primary cluster
   kubectl get volumegroupreplication <vgr-name> -n <namespace>

   # On secondary cluster
   kubectl get volumegroupreplication <vgr-name> -n <namespace>
   ```

   If the VGR does not exist on the secondary cluster, create it following the same process as described in the Failover workflow step 1 but without the `autoResync: true` field.

2. **Verify Primary Cluster Status**

Check the VolumeGroupReplication status on the current primary cluster:

```bash
 kubectl get volumegroupreplication <vgr-name> -n <namespace> -o yaml
```

Ensure healthy replication:

```yaml
status:
  conditions:
    - type: Completed
      status: "True"
      reason: Promoted
    - type: Degraded
      status: "False"
      reason: Healthy
    - type: Resyncing
      status: "False"
      reason: NotResyncing
    - type: Replicating
      status: "True"
      reason: Replicating
  state: Primary
  persistentVolumeClaimsRefList:
    - pvc-1
    - pvc-2
    - pvc-3
```

**Wait Condition**: Ensure `Degraded: False` and `Resyncing: False` before proceeding.

3. **Verify Secondary Cluster Status**

Check the VolumeGroupReplication status on the target secondary cluster:

```bash
kubectl get volumegroupreplication <vgr-name> -n <namespace> -o yaml
```

Ensure it's synchronized:

```yaml
status:
  conditions:
    - type: Completed
      status: "True"
      reason: Demoted
    - type: Degraded
      status: "True"
      reason: VolumeDegraded
    - type: Resyncing
      status: "False"
      reason: NotResyncing
  state: Secondary
  persistentVolumeClaimsRefList:
    - pvc-1
    - pvc-2
    - pvc-3
```

**Wait Condition**: Ensure `Resyncing: False` to confirm all volumes in the group are fully synchronized.

4. **Quiesce Application (if applicable)**

Stop or quiesce the application on the primary cluster to ensure no new writes occur during the transition:

```bash
kubectl scale deployment <app-deployment> -n <namespace> --replicas=0
```

This is especially important for volume groups to maintain consistency across all volumes.

5. **Perform Final Sync**

Wait for any final data synchronization to complete for all volumes in the group. Monitor the `lastSyncTime` in the VGR status to ensure recent synchronization.

6. **Demote Primary to Secondary**

Update the VolumeGroupReplication on the current primary cluster:

```bash
kubectl patch volumegroupreplication <vgr-name> -n <namespace> --type merge -p '{"spec":{"replicationState":"secondary"}}'
```

Wait for demotion to complete:

```yaml
status:
  conditions:
    - type: Completed
      status: "True"
      reason: Demoted
    - type: Degraded
      status: "True"
      reason: VolumeDegraded
  state: Secondary
  persistentVolumeClaimsRefList:
    - pvc-1
    - pvc-2
    - pvc-3
```

**Wait Condition**: Ensure `Completed: True` and `state: Secondary` before proceeding. Verify that the VR has also been demoted.

7. **Promote Secondary to Primary**

Update the VolumeGroupReplication on the target cluster:

```bash
kubectl patch volumegroupreplication <vgr-name> -n <namespace> --type merge -p '{"spec":{"replicationState":"primary"}}'
```

Wait for promotion to complete:

```yaml
status:
  conditions:
    - type: Completed
      status: "True"
      reason: Promoted
    - type: Degraded
      status: "False"
      reason: Healthy
    - type: Replicating
      status: "True"
      reason: Replicating
  state: Primary
  persistentVolumeClaimsRefList:
    - pvc-1
    - pvc-2
    - pvc-3
```

**Wait Condition**: Ensure `Completed: True`, `Degraded: False`, and `state: Primary` before starting applications. Verify that the VR has been promoted.

8. **Start Application on New Primary**

Deploy or scale up the application on the new primary cluster:

```bash
 kubectl scale deployment <app-deployment> -n <namespace> --replicas=<desired-count>
```

9. **Verify Application Health**

Confirm that: - Applications are running and healthy - All PVCs in the group are bound and accessible - Data is being written successfully to all volumes - Replication to the new secondary is active for the entire volume group

#### Relocate Considerations

- **VGR Creation**: Always verify that VolumeGroupReplication exists on both clusters before attempting relocate. Create missing VGRs with appropriate `replicationState` values and matching label selectors
- **Consistency Group**: All volumes in the group are demoted and promoted together, maintaining crash consistency across the volume group throughout the relocate operation
- **Zero Data Loss**: When performed correctly with proper synchronization checks, relocate operations should result in zero data loss for all volumes in the group
- **Planned Downtime**: Applications will experience downtime during the transition. Plan the relocate during a maintenance window
- **Rollback Plan**: Have a rollback plan ready in case issues arise during the relocate operation
- **RamenDR Integration**: RamenDR automates the entire relocate workflow, including VGR creation, application quiescing, VGR state transitions, and application restart

### Key Status Conditions to Monitor

When performing failover or relocate operations on volume groups, always monitor these critical conditions:

| Condition Type | Status  | Meaning                                                                                     |
| -------------- | ------- | ------------------------------------------------------------------------------------------- |
| `Completed`    | `True`  | The last replication operation (promote/demote) completed successfully for the volume group |
| `Degraded`     | `False` | Volume group is healthy and available for I/O (Primary state)                               |
| `Degraded`     | `True`  | Volume group is in read-only or unavailable state (Secondary state)                         |
| `Resyncing`    | `False` | Volume group is fully synchronized with its peer                                            |
| `Resyncing`    | `True`  | Volume group is actively catching up with changes from the peer                             |
| `Replicating`  | `True`  | Volume group is actively replicating data to the peer (Primary only)                        |
| `Validated`    | `True`  | Volume group has passed all prerequisite checks                                             |

### Troubleshooting

**Issue**: VGR does not exist on secondary cluster

- **Cause**: VGR was not created during initial setup or was deleted
- **Resolution**: Create the VGR manually with `replicationState: secondary` referencing the correct label selector, VolumeReplicationClass, and VolumeGroupReplicationClass

**Issue**: VGR stuck in `Resyncing: True` state

- **Cause**: Network issues, storage backend problems, or large data delta across the volume group
- **Resolution**: Check network connectivity, verify storage backend health, monitor `lastSyncBytes` and `lastSyncDuration` for progress. Check the VR status for the volume group

**Issue**: Some volumes in the group are not synchronized

- **Cause**: VR issues, PVC label mismatch, or storage backend problems
- **Resolution**: Check `persistentVolumeClaimsRefList` in VGR status to verify all expected PVCs are included. Review the VR status for the volume group. Verify PVC labels match the VGR selector

**Issue**: Promotion fails with validation errors

- **Cause**: Prerequisites not met for the volume group, storage backend issues
- **Resolution**: Check the `Validated` condition message for specific prerequisite failures. Review the VR status for the volume group

**Issue**: Application cannot access volumes after promotion

- **Cause**: PVCs not bound, node affinity issues, or storage driver problems
- **Resolution**: Verify all PVC statuses, check node labels, review CSI driver logs. Ensure VolumeGroupReplicationContent is properly configured

**Issue**: VGR shows different PVC count than expected

- **Cause**: Label selector mismatch, PVCs created after VGR, or PVCs in different namespace
- **Resolution**: Verify label selector matches PVC labels. Remember VGR only groups PVCs in the same namespace. Check VolumeGroupReplicationContent status for the list of volume handles

### Pre-Provisioned VolumeGroup

Admin can create a VGRContent specifying an existing `volumeGroupReplicationHandle` in the storage system and specifying an existing VolumeGroup name and namespace. Then, the user can create a VolumeGroup that points to the VGRContent name.

The VGR controller will filter all the PVCs that match the label selector of the VGR, and the VGRContent controller will issue a Modify RPC call to update the group with any new PVC, or shall return if the group is unchanged. Then, the VGR controller can manage mirroring on the volume group and enable mirroring on the group.

### Delete VolumeGroupReplication

When a VolumeGroupReplication is deleted the controller should first remove the volumes from the group, and then delete the group. Or, it can choose to disable mirroring and then remove the volumes from the group as well.
Then, delete the VolumeGroup from the storage system. And, delete the VGRContent and perform deletion of VR which would disable the mirroring operations, remove any sanitary annotations from PVCs, VGR if any. Then, at the end the VolumeGroup CR should be deleted by the controller.

## API definition of VolumeGroupReplication (VGR) CRD

- [VR Type Definition](https://github.com/csi-addons/kubernetes-csi-addons/blob/main/api/replication.storage/v1alpha1/volumegroupreplication_types.go)
- [VR CRD Definition](https://github.com/csi-addons/kubernetes-csi-addons/blob/main/config/crd/bases/replication.storage.openshift.io_volumegroupreplications.yaml)

## API definition of VolumeGroupReplicationContent CRD

- [VR Type Definition](https://github.com/csi-addons/kubernetes-csi-addons/blob/main/api/replication.storage/v1alpha1/volumegroupreplicationcontent_types.go)
- [VR CRD Definition](https://github.com/csi-addons/kubernetes-csi-addons/blob/main/config/crd/bases/replication.storage.openshift.io_volumegroupreplicationcontents.yaml)

## API definition of VolumeGroupReplicationClass (VGRC) CRD

- [VR Type Definition](https://github.com/csi-addons/kubernetes-csi-addons/blob/main/api/replication.storage/v1alpha1/volumegroupreplicationclass_types.go)
- [VR CRD Definition](https://github.com/csi-addons/kubernetes-csi-addons/blob/main/config/crd/bases/replication.storage.openshift.io_volumegroupreplicationclasses.yaml)

## Example YAML files

- VolumeGroupReplicationClass (VGRC) CR:

```yaml
apiVersion: replication.storage.openshift.io/v1alpha1
kind: VolumeGroupReplicationClass
metadata:
  name: volumegroupreplicationclass-sample
spec:
  provisioner: example.provisioner.io
  parameters:
    clusterID: my-cluster
    replication.storage.openshift.io/group-replication-secret-name: secret-name
    replication.storage.openshift.io/group-replication-secret-namespace: secret-namespace
```

- VolumeGroupReplication (VGR) CR:

```yaml
apiVersion: replication.storage.openshift.io/v1alpha1
kind: VolumeGroupReplication
metadata:
  name: volumegroupreplication-sample
  namespace: default
spec:
  volumeReplicationClassName: volumereplicationclass-sample
  volumeGroupReplicationClassName: volumegroupreplicationclass-sample
  replicationState: primary
  source:
    selector:
      matchLabels:
        appname: test
status:
  persistentVolumeClaimsRefList:
    - test-pvc
  state: primary
```
