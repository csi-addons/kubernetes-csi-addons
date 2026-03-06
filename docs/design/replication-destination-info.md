# Design: Replication Destination Info

## Problem Statement

Current assumption is that the volume IDs and volume group IDs are same on
the source and the destination clusters. This is not true for all. Some
storage providers have different volume IDs and volume group IDs on the
source and destination sides of a replication relationship. The current
spec and implementation provide no mechanism for the storage provider (SP)
to communicate destination details back to the Container Orchestrator (CO).

## Proposal

Introduce a new RPC `GetReplicationDestinationInfo` with a corresponding
capability `GET_REPLICATION_DESTINATION_INFO`. This RPC can be called at any
time to retrieve the current destination volume or volume group details,
including per-volume ID mappings for dynamic groups.

## Spec Changes (csi-addons/spec)

### 1. New Capability: `GET_REPLICATION_DESTINATION_INFO`

A new capability enum value is added under `VolumeReplication.Type` in the
`Capability` message:

```protobuf
message VolumeReplication {
  enum Type {
    UNKNOWN = 0;
    VOLUME_REPLICATION = 1;
    // GET_REPLICATION_DESTINATION_INFO indicates that the CSI-driver
    // supports getting the destination volume or volume group details
    // for a replication relationship. This capability is relevant for
    // storage providers where source and destination volume or volume
    // group identifiers differ.
    GET_REPLICATION_DESTINATION_INFO = 2;
  }
  Type type = 1;
}
```

This capability is OPTIONAL. Storage providers where source and destination
volume IDs are always identical do not need to advertise this capability.
When the CO detects this capability, it knows that the driver supports the
`GetReplicationDestinationInfo` RPC and will call it after replication
operations to retrieve destination-side identifiers.

### 2. New RPC: `GetReplicationDestinationInfo`

A new RPC is added to the `Controller` service:

```protobuf
service Controller {
  // ... existing RPCs ...
  // GetReplicationDestinationInfo RPC call to get the destination
  // volume or volume group details for an existing replication.
  rpc GetReplicationDestinationInfo (GetReplicationDestinationInfoRequest)
  returns (GetReplicationDestinationInfoResponse) {}
}
```

### 3. New Messages

#### GetReplicationDestinationInfoRequest

```protobuf
// GetReplicationDestinationInfoRequest holds the required information to
// get the destination volume or volume group details for an existing
// replication.
message GetReplicationDestinationInfoRequest {
  // The source volume or volume group for which destination
  // details are requested.
  // This field is REQUIRED.
  ReplicationSource replication_source = 1;
  // Secrets required by the plugin to complete the request.
  map<string, string> secrets = 2 [(csi.v1.csi_secret) = true];
}

// Specifies the source for a replication. One of the type fields
// MUST be specified.
message ReplicationSource {
  // VolumeSource contains the source details for a replicated volume.
  message VolumeSource {
    // The source volume ID for which destination details are requested.
    // This field is REQUIRED.
    string volume_id = 1;
  }
  // VolumeGroupSource contains the source details for a replicated
  // volume group.
  message VolumeGroupSource {
    // The source volume group ID for which destination details are
    // requested.
    // This field is REQUIRED.
    string volume_group_id = 1;
  }

  oneof type {
    // Volume source type
    VolumeSource volume = 1;
    // Volume group source type
    VolumeGroupSource volumegroup = 2;
  }
}
```

#### GetReplicationDestinationInfoResponse

```protobuf
// GetReplicationDestinationInfoResponse holds the destination volume
// or volume group details for an existing replication.
message GetReplicationDestinationInfoResponse {
  // The destination details for the replication.
  // This field is REQUIRED.
  ReplicationDestination replication_destination = 1;
}

// Specifies the destination details for a replication. One of the
// type fields MUST be specified.
message ReplicationDestination {
  // VolumeDestination contains the destination details for a
  // replicated volume.
  message VolumeDestination {
    // The destination volume ID on the remote/target cluster.
    // This field is REQUIRED.
    string volume_id = 1;
  }
  // VolumeGroupDestination contains the destination details for a
  // replicated volume group.
  message VolumeGroupDestination {
    // The destination volume group ID on the remote/target cluster.
    // This field is REQUIRED.
    string volume_group_id = 1;
    // Mapping of source volume IDs to their corresponding
    // destination volume IDs. Key is source volume_id,
    // value is destination volume_id.
    // This field is OPTIONAL.
    // This map reflects the current group membership at the
    // time of the call, accounting for dynamic group changes.
    // When provided, this map MUST be complete — it MUST
    // contain an entry for every volume currently in the group.
    // If the SP cannot determine the destination ID for all
    // volumes (e.g., newly added volumes have not finished
    // initial sync), the SP MUST return an error instead of
    // a partial map.
    map<string, string> volume_ids = 2;
  }

  oneof type {
    // Volume destination type
    VolumeDestination volume = 1;
    // Volume group destination type
    VolumeGroupDestination volumegroup = 2;
  }
}
```

### 4. Error Scheme for `GetReplicationDestinationInfo`

| Condition                                | gRPC Code             | Description                                                                        | Recovery Behavior                                                                                                     |
| ---------------------------------------- | --------------------- | ---------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------- |
| Missing required field                   | 3 INVALID_ARGUMENT    | A required field is missing from the request.                                      | Caller MUST fix the request by adding the missing required field before retrying.                                     |
| Replication Source does not exist        | 5 NOT_FOUND           | The specified source does not exist.                                               | Caller MUST verify that the `replication_source` is correct and accessible before retrying with exponential back off. |
| Replication Source is not replicated     | 9 FAILED_PRECONDITION | Destination information could not be retrieved because replication is not enabled. | Caller SHOULD ensure that replication is enabled on the `replication_source`.                                         |
| Destination info not yet available       | 14 UNAVAILABLE        | Destination details are not yet available. The SP needs time after replication.    | Caller SHOULD retry with exponential back off. This is transient and resolves once replication is fully established.  |
| Operation pending for Replication Source | 10 ABORTED            | There is already an operation pending for the specified `replication_source`.      | Caller SHOULD ensure no other calls are pending for the `replication_source`, then retry with exponential back off.   |
| Call not implemented                     | 12 UNIMPLEMENTED      | The invoked RPC is not implemented by the Plugin.                                  | Caller MUST NOT retry.                                                                                                |
| Not authenticated                        | 16 UNAUTHENTICATED    | The invoked RPC does not carry valid secrets for authentication.                   | Caller SHALL fix or regenerate the secrets, then retry.                                                               |
| Error is Unknown                         | 2 UNKNOWN             | An unknown error occurred.                                                         | Caller MUST study the logs before retrying.                                                                           |

## kubernetes-csi-addons Changes

### 1. Proto Definition

Add the RPC to the `Replication` service and add the following messages:

```protobuf
// GetReplicationDestinationInfoRequest holds the required information to
// get the destination volume or volume group details for an existing
// replication.
message GetReplicationDestinationInfoRequest {
  // The source volume or volume group for which destination
  // details are requested.
  // This field is REQUIRED.
  ReplicationSource replication_source = 1;
  // Secrets required by the plugin to complete the request.
  string secret_name = 2;
  string secret_namespace = 3;

}

// GetReplicationDestinationInfoResponse holds the destination volume
// or volume group details for an existing replication.
message GetReplicationDestinationInfoResponse {
  // The destination details for the replication.
  // This field is REQUIRED.
  ReplicationDestination replication_destination = 1;
}

// Specifies the destination details for a replication. One of the
// type fields MUST be specified.
message ReplicationDestination {
  // VolumeDestination contains the destination details for a
  // replicated volume.
  message VolumeDestination {
    // The destination volume ID on the remote/target cluster.
    // This field is REQUIRED.
    string volume_id = 1;
  }
  // VolumeGroupDestination contains the destination details for a
  // replicated volume group.
  message VolumeGroupDestination {
    // The destination volume group ID on the remote/target cluster.
    // This field is REQUIRED.
    string volume_group_id = 1;
    // Mapping of source volume IDs to their corresponding
    // destination volume IDs. Key is source volume_id,
    // value is destination volume_id.
    // This field is OPTIONAL.
    map<string, string> volume_ids = 2;
  }

  oneof type {
    // Volume destination type
    VolumeDestination volume = 1;
    // Volume group destination type
    VolumeGroupDestination volumegroup = 2;
  }
}
```

### 2. Client Interface

Add to the `VolumeReplication` interface:

```go
// GetReplicationDestinationInfo RPC call to get destination info
// for a single volume.
GetReplicationDestinationInfo(volumeID, secretName, secretNamespace string) (*proto.GetReplicationDestinationInfoResponse, error)
```

Add to the `VolumeGroupReplication` interface:

```go
// GetReplicationDestinationInfo RPC call to get destination info
// for a volume group.
GetReplicationDestinationInfo(volumeGroupID, secretName, secretNamespace string) (*proto.GetReplicationDestinationInfoResponse, error)
```

### 3. Client Implementations

The `VolumeReplication` client implementation populates
`ReplicationSource` with `ReplicationSource_Volume` using the
provided `volumeID`.

The `VolumeGroupReplication` client implementation populates
`ReplicationSource` with `ReplicationSource_VolumeGroup` using the
provided `volumeGroupID`.

### 4. Replication Wrapper

Add `GetDestinationInfo()` method following the same pattern as `GetInfo()`:

```go
func (r *Replication) GetDestinationInfo() *Response {
    id, err := r.getID()
    if err != nil {
        return &Response{Error: err}
    }
    resp, err := r.Params.Replication.GetReplicationDestinationInfo(
        id,
        r.Params.SecretName,
        r.Params.SecretNamespace,
    )
    return &Response{Response: resp, Error: err}
}
```

### 5. API Types

Add a new condition type and associated constants:

```go
// Condition type
const (
    // ... existing conditions ...
    ConditionDestinationInfoAvailable = "DestinationInfoAvailable"
)

// Messages
const (
    // ... existing messages ...
    MessageDestinationInfoAvailable = "destination info is available"
    MessageDestinationInfoPending   = "destination info is pending update"
    MessageDestinationInfoFailed    = "failed to get destination info"
)

// Reasons
const (
    // ... existing reasons ...
    DestinationInfoUpdated = "DestinationInfoUpdated"
    DestinationInfoPending = "DestinationInfoPending"
    FailedToGetDestinationInfo = "FailedToGetDestinationInfo"
)
```

Add destination fields to `VolumeReplicationStatus`:

```go
type VolumeReplicationStatus struct {
    // ... existing fields ...

    // DestinationVolumeID is the volume ID on the destination/target side.
    // This field is set when the SP reports different source and destination
    // volume IDs.
    // +optional
    DestinationVolumeID string `json:"destinationVolumeID,omitempty"`
}
```

Add a new type for enriched PVC references that includes volume handle
correlation. This replaces the current `[]corev1.LocalObjectReference`
with a richer type that enables DR orchestrators to directly read
destination volume handles per PVC without cross-referencing PV objects:

```go
// VolumeHandleMapping contains the source and destination volume
// handles for a PVC. This is populated as a unit only when both
// source and destination handles are known, so consumers never
// see partial data.
type VolumeHandleMapping struct {
    // SourceVolumeHandle is the CSI volume handle of the PV bound to
    // this PVC on the source/primary cluster.
    SourceVolumeHandle string `json:"sourceVolumeHandle"`

    // DestinationVolumeHandle is the CSI volume handle on the
    // destination/target cluster.
    DestinationVolumeHandle string `json:"destinationVolumeHandle"`
}

// VolumeGroupReplicationPVCStatus contains the PVC reference along
// with an optional volume handle mapping. The mapping is populated
// only when the SP supports GET_REPLICATION_DESTINATION_INFO and
// destination info is available. When the mapping is nil, the PVC
// entry behaves like the original LocalObjectReference (name only).
type VolumeGroupReplicationPVCStatus struct {
    // Name is the name of the PVC.
    Name string `json:"name"`

    // VolumeHandleMapping holds the source-to-destination volume
    // handle correlation. This pointer is nil until both source and
    // destination handles are known, avoiding partial data.
    // +optional
    VolumeHandleMapping *VolumeHandleMapping `json:"volumeHandleMapping,omitempty"`
}
```

Update `VolumeGroupReplicationStatus` to use the enriched PVC reference
type and add the destination volume group ID:

```go
type VolumeGroupReplicationStatus struct {
    // ... existing fields ...

    // PersistentVolumeClaimsRefList is the list of PVCs for the volume
    // group replication, enriched with source and destination volume
    // handles for each PVC.
    // The maximum number of allowed PVCs in the group is 100.
    // +optional
    PersistentVolumeClaimsRefList []VolumeGroupReplicationPVCStatus `json:"persistentVolumeClaimsRefList,omitempty"`

    // DestinationVolumeGroupID is the volume group ID on the
    // destination/target side.
    // +optional
    DestinationVolumeGroupID string `json:"destinationVolumeGroupID,omitempty"`
}
```

> **Note:** The `VolumeHandleMapping` pointer is `nil` until both source
> and destination handles are available. This means:
>
> - Before destination info is fetched: entries have only `name` (backward
>   compatible with the original `LocalObjectReference` shape).
> - After destination info is available: entries have `name` and the full
>   `volumeHandleMapping` with both handles populated together.
> - The raw SP mapping (source handle → destination handle) is still stored
>   in `VolumeGroupReplicationContentStatus.DestinationVolumeIDs`. The VGR
>   controller translates the raw map to PVC-keyed entries when propagating.

### 6. Status Condition Helpers

```go
// setDestinationInfoAvailableCondition sets DestinationInfoAvailable=True
func setDestinationInfoAvailableCondition(conditions *[]metav1.Condition,
    observedGeneration int64, dataSource string) {
    source := getSource(dataSource)
    setStatusCondition(conditions, &metav1.Condition{
        Message:            fmt.Sprintf("%s %s", source, v1alpha1.MessageDestinationInfoAvailable),
        Type:               v1alpha1.ConditionDestinationInfoAvailable,
        Reason:             v1alpha1.DestinationInfoUpdated,
        ObservedGeneration: observedGeneration,
        Status:             metav1.ConditionTrue,
    })
}

// setDestinationInfoPendingCondition sets DestinationInfoAvailable=False
// (destination details are stale or not yet fetched)
func setDestinationInfoPendingCondition(conditions *[]metav1.Condition,
    observedGeneration int64, dataSource string) {
    source := getSource(dataSource)
    setStatusCondition(conditions, &metav1.Condition{
        Message:            fmt.Sprintf("%s %s", source, v1alpha1.MessageDestinationInfoPending),
        Type:               v1alpha1.ConditionDestinationInfoAvailable,
        Reason:             v1alpha1.DestinationInfoPending,
        ObservedGeneration: observedGeneration,
        Status:             metav1.ConditionFalse,
    })
}

// setDestinationInfoFailedCondition sets DestinationInfoAvailable=False
// with error details
func setDestinationInfoFailedCondition(conditions *[]metav1.Condition,
    observedGeneration int64, dataSource, errorMessage string) {
    source := getSource(dataSource)
    setStatusCondition(conditions, &metav1.Condition{
        Message:            fmt.Sprintf("%s %s: %s", source, v1alpha1.MessageDestinationInfoFailed, errorMessage),
        Type:               v1alpha1.ConditionDestinationInfoAvailable,
        Reason:             v1alpha1.FailedToGetDestinationInfo,
        ObservedGeneration: observedGeneration,
        Status:             metav1.ConditionFalse,
    })
}
```

### 7. Capability Check

Add a method to check if the driver supports `GET_REPLICATION_DESTINATION_INFO`:

```go
func (r *VolumeReplicationReconciler) supportsGetReplicationDestinationInfo(
    ctx context.Context, driverName string) bool {
    conn, err := r.Connpool.GetLeaderByDriver(ctx, r.Client, driverName)
    if err != nil {
        return false
    }
    for _, cap := range conn.Capabilities {
        if cap.GetVolumeReplication() == nil {
            continue
        }
        if cap.GetVolumeReplication().GetType() ==
            identity.Capability_VolumeReplication_GET_REPLICATION_DESTINATION_INFO {
            return true
        }
    }
    return false
}
```

### 8. Controller Flow - VolumeReplication

If the driver supports the capability, call `GetReplicationDestinationInfo`
on every reconcile where the `DestinationInfoAvailable` condition is not
`True`. This ensures that destination info fetching is **idempotent and
retried independently** of replication operations. If a replication
operation succeeds but the subsequent status update (with destination info)
fails due to a conflict, the next reconcile will retry the destination info
fetch without needing to re-execute the replication operation.

Some SPs may not have destination details available immediately after
replication is enabled (e.g., initial sync in progress, destination volume
being provisioned). The SP returns `UNAVAILABLE` in this case. Since the
condition remains `False` (with reason `FailedToGetDestinationInfo`), the
controller will retry on the next reconcile until the SP is ready.

```go
// Fetch destination info independently of replication operations.
// This runs on every reconcile where destination info is not yet available,
// ensuring retries are decoupled from replication state changes.
if r.supportsGetReplicationDestinationInfo(ctx, driverName) &&
    !isDestinationInfoAvailable(instance.Status.Conditions) {
    destInfo, err := r.getReplicationDestinationInfo(vr)
    if err != nil {
        setDestinationInfoFailedCondition(&instance.Status.Conditions,
            instance.Generation, instance.Spec.DataSource.Kind, err.Error())
    } else if destInfo != nil {
        if vol := destInfo.GetReplicationDestination().GetVolume(); vol != nil {
            instance.Status.DestinationVolumeID = vol.GetVolumeId()
        }
        setDestinationInfoAvailableCondition(&instance.Status.Conditions,
            instance.Generation, instance.Spec.DataSource.Kind)
    }
}
```

### 9. Controller Flow - VolumeGroupReplication (Dynamic Grouping)

This is the critical path for dynamic groups. The design follows the
**single-writer-per-resource** principle: VGRContent controller writes
only to VGRContent status, and VGR controller writes only to VGR status.
VGR reads VGRContent status and propagates validated data.

#### 9a. VGRContent Status Fields

Add destination fields to `VolumeGroupReplicationContentStatus`:

```go
type VolumeGroupReplicationContentStatus struct {
    // ... existing fields ...

    // DestinationVolumeGroupID is the volume group ID on the
    // destination/target side, as reported by the SP.
    // +optional
    DestinationVolumeGroupID string `json:"destinationVolumeGroupID,omitempty"`

    // DestinationVolumeIDs is a mapping of source volume IDs to their
    // corresponding destination volume IDs, as reported by the SP.
    // Key is source volume_id, value is destination volume_id.
    // +optional
    DestinationVolumeIDs map[string]string `json:"destinationVolumeIDs,omitempty"`
}
```

#### 9b. Watch Configuration

The VGR controller currently watches VGRContent with `watchOnlySpecUpdates`.
This must be extended to also watch VGRContent **status** changes so that
VGR reconciles when VGRContent destination info is updated:

```go
// In VGR controller setup
Owns(&v1alpha1.VolumeGroupReplicationContent{},
    builder.WithPredicates(watchSpecAndStatusUpdates))
```

#### 9c. Flow Overview

```console
Group membership changes (PVC labels change)
    |
    v
VGR reconcile detects PVC list change
    |
    v
VGRContent.Spec.Source.VolumeHandles is updated
    |
    v
VGR sets DestinationInfoAvailable=False (destination details stale)
    |
    v
VGRContent controller reconciles:
  1. Calls ModifyVolumeGroupMembership
  2. Calls GetReplicationDestinationInfo
     - If SP returns UNAVAILABLE (destination not ready yet):
       VGRContent requeues with exponential back off, retries later
     - If SP returns success:
       Stores result in VGRContent.Status (destination group ID + volume map)
    |
    v  (once SP returns success and VGRContent.Status is updated)
    |
VGR controller reconciles (triggered by VGRContent status watch):
  1. Reads VGRContent.Status destination fields
  2. Validates map covers all source volume handles in PVC ref list
  3. If complete: populates VolumeHandleMapping on each PVC entry
     (with both source and destination handles), sets
     DestinationInfoAvailable=True
  4. If incomplete: keeps DestinationInfoAvailable=False, requeues
```

#### 9d. VGR Controller: Detect Membership Change and Mark Stale

In the VGR reconciler, after updating `PersistentVolumeClaimsRefList` when
it differs from the previous list (indicating membership change):

```go
// When group membership changes, mark destination info as pending.
// Build PVC ref list with name only — VolumeHandleMapping is left nil
// until VGRContent fetches destination info and VGR propagates it.
if !pvcListEqual(instance.Status.PersistentVolumeClaimsRefList, pvcRefList) {
    refs := make([]VolumeGroupReplicationPVCStatus, 0, len(pvcRefList))
    for _, pvcRef := range pvcRefList {
        refs = append(refs, VolumeGroupReplicationPVCStatus{
            Name: pvcRef.Name,
            // VolumeHandleMapping is nil — populate it later
        })
    }
    instance.Status.PersistentVolumeClaimsRefList = refs
    // Mark destination info as stale since group membership changed
    setDestinationInfoPendingCondition(&instance.Status.Conditions,
        instance.Generation, volumeGroupReplicationDataSource)
    // Clear stale destination group ID
    instance.Status.DestinationVolumeGroupID = ""
    // ...
}
```

#### 9e. VGRContent Controller: Fetch and Store Destination Info

After `ModifyVolumeGroupMembership` completes (or on any reconcile where
the VGRContent has a valid group handle), the VGRContent controller calls
`GetReplicationDestinationInfo` using `ReplicationSource_VolumeGroup` with
the group ID. The response `VolumeGroupDestination` provides:

- Updated `volume_group_id` for the destination
- Updated `volume_ids` map (source -> destination) reflecting current membership

The VGRContent controller stores the result in its **own status only**.

Some SPs may not have destination details available immediately after
replication is enabled or group membership is modified (e.g., initial sync
in progress, destination volumes being provisioned on the remote cluster).
The SP returns `UNAVAILABLE` in this case. The VGRContent controller
handles this by requeuing for retry with exponential back off. During
this period, VGRContent status destination fields remain empty and VGR
keeps `DestinationInfoAvailable=False`.

```go
// In VGRContent controller, after ModifyVolumeGroupMembership
if r.supportsGetReplicationDestinationInfo(ctx, driverName) {
    destInfo, err := r.getReplicationDestinationInfo(vgrc)
    if err != nil {
        // SP may return UNAVAILABLE if destination details are not
        // yet available (e.g., initial sync still in progress).
        // Requeue to retry — VGRContent.Status destination fields
        // remain empty, so VGR keeps DestinationInfoAvailable=False.
        log.Error(err, "failed to get destination info, will retry")
        return ctrl.Result{RequeueAfter: requeueInterval}, nil
    }
    if vgDest := destInfo.GetReplicationDestination().GetVolumegroup(); vgDest != nil {
        vgrcInstance.Status.DestinationVolumeGroupID = vgDest.GetVolumeGroupId()
        vgrcInstance.Status.DestinationVolumeIDs = vgDest.GetVolumeIds()
        // Update VGRContent status only — VGR will read and propagate
    }
}
```

#### 9f. VGR Controller: Validate and Propagate

On each reconcile, the VGR controller reads the VGRContent status and
validates that the destination map is complete. If complete, it populates
the `VolumeHandleMapping` pointer on each PVC ref entry with both source
and destination handles as a unit, so DR orchestrators can directly read
per-PVC destination handles without cross-referencing PV objects:

```go
// In VGR controller reconcile
if r.supportsGetReplicationDestinationInfo(ctx, driverName) &&
    !isDestinationInfoAvailable(instance.Status.Conditions) {

    vgrc := getVolumeGroupReplicationContent(instance)
    if vgrc.Status.DestinationVolumeIDs != nil {
        // Build source handle lookup: PVC name -> source volume handle
        sourceHandles := make(map[string]string)
        for _, pvcStatus := range instance.Status.PersistentVolumeClaimsRefList {
            pvc := getPVC(pvcStatus.Name)
            pv := getPV(pvc.Spec.VolumeName)
            sourceHandles[pvcStatus.Name] = pv.Spec.CSI.VolumeHandle
        }

        // Validate: destination map must cover all source volume handles
        allCovered := true
        for _, srcHandle := range sourceHandles {
            if _, ok := vgrc.Status.DestinationVolumeIDs[srcHandle]; !ok {
                allCovered = false
                break
            }
        }
        if allCovered {
            // Populate VolumeHandleMapping on each PVC ref entry
            for i := range instance.Status.PersistentVolumeClaimsRefList {
                name := instance.Status.PersistentVolumeClaimsRefList[i].Name
                srcHandle := sourceHandles[name]
                instance.Status.PersistentVolumeClaimsRefList[i].VolumeHandleMapping =
                    &VolumeHandleMapping{
                        SourceVolumeHandle:      srcHandle,
                        DestinationVolumeHandle: vgrc.Status.DestinationVolumeIDs[srcHandle],
                    }
            }
            instance.Status.DestinationVolumeGroupID = vgrc.Status.DestinationVolumeGroupID
            setDestinationInfoAvailableCondition(&instance.Status.Conditions,
                instance.Generation, volumeGroupReplicationDataSource)
        }
        // If not all covered, remain False — VGRContent is still processing
    }
}
```

> **Note:** The VR controller handles individual volume destination info
> using `ReplicationSource_Volume`. The VGRContent controller handles
> group-level destination info using `ReplicationSource_VolumeGroup`.
> These are separate code paths — the VR controller does not call the
> group-level RPC.

## Condition State Transitions

```console
Initial state (no replication):

DestinationInfoAvailable: not present

After EnableVolumeReplication (if capability supported):

DestinationInfoAvailable: False (Reason: DestinationInfoPending)
        |
        v
GetReplicationDestinationInfo called
        |
        +--> SP returns UNAVAILABLE (destination not ready yet)
        |    DestinationInfoAvailable: False (Reason: FailedToGetDestinationInfo)
        |    Controller requeues, retries on next reconcile
        |    (repeats until SP returns success)
        |
        +--> SP returns success
             DestinationInfoAvailable: True (Reason: DestinationInfoUpdated)

Dynamic group membership change (VGR + VGRContent coordination):

DestinationInfoAvailable: True
        |
        v
VGR detects PVC list change, clears VGR destination fields
DestinationInfoAvailable: False (Reason: DestinationInfoPending)
        |
        v
VGRContent controller calls ModifyVolumeGroupMembership
VGRContent controller calls GetReplicationDestinationInfo
VGRContent stores result in VGRContent.Status
        |
        v
VGR reconciles (triggered by VGRContent status watch)
VGR validates map covers all VGRContent.Spec.Source.VolumeHandles
VGR copies validated data to VGR.Status
DestinationInfoAvailable: True (Reason: DestinationInfoUpdated)

GetReplicationDestinationInfo fails or SP returns UNAVAILABLE (VGRContent controller):
VGRContent.Status destination fields remain empty/stale
VGRContent requeues with exponential back off, retries until SP is ready
VGR sees incomplete map, keeps:
DestinationInfoAvailable: False (Reason: DestinationInfoPending)
```

## Workflow: How Third-Party Tools Should Use Destination IDs

Third-party tools (DR orchestrators like Ramen, or any custom
disaster recovery controller) that consume VolumeReplication and
VolumeGroupReplication CRDs need to coordinate across **primary** and
**secondary** clusters. The destination IDs enable them to establish the
correct volume identity on the remote cluster during failover and failback.

### Terminology

| Term                  | Meaning                                                                 |
| --------------------- | ----------------------------------------------------------------------- |
| Primary Cluster       | The cluster where volumes are actively serving workloads                |
| Secondary Cluster     | The cluster receiving replicated data                                   |
| DR Orchestrator       | Third-party tool that manages failover/failback across clusters         |
| Source Volume ID      | The CSI volume handle on the primary cluster                            |
| Destination Volume ID | The CSI volume handle on the secondary cluster (may differ from source) |

### Workflow 1: Individual Volume Replication

#### Step 1: Setup Replication on Primary Cluster

The DR orchestrator creates the `VolumeReplication` CR on the primary
cluster referencing a PVC.

```yaml
apiVersion: replication.storage.openshift.io/v1alpha1
kind: VolumeReplication
metadata:
  name: vr-pvc-data
spec:
  volumeReplicationClass: example-replication
  dataSource:
    kind: PersistentVolumeClaim
    name: pvc-data
  replicationState: primary
```

#### Step 2: Read Destination Info from VR Status on Primary Cluster

Once replication is established and `DestinationInfoAvailable` is `True`,
the DR orchestrator reads the destination volume ID from VR status:

```yaml
status:
  state: Primary
  destinationVolumeID: "dest-vol-0001-abcdef"
  conditions:
    - type: DestinationInfoAvailable
      status: "True"
      reason: DestinationInfoUpdated
```

The DR orchestrator MUST:

1. Watch the `DestinationInfoAvailable` condition.
2. Only use destination IDs when the condition is `True`.
3. Store the mapping `source-vol-id -> destination-vol-id` in its own state
   (e.g., a DRPlacementControl or similar CR) so it is available during
   failover.

#### Step 3: Failover to Secondary Cluster

When a disaster is detected and failover is triggered, the DR orchestrator
performs the following steps **on the secondary cluster**:

1. **Create PV with the destination volume ID** - Use the destination volume
   ID (not the source volume ID) as the CSI volume handle:

   ```yaml
   apiVersion: v1
   kind: PersistentVolume
   metadata:
     name: pv-data-failover
   spec:
     csi:
       driver: example.csi.com
       volumeHandle: "dest-vol-0001-abcdef" # destination ID, NOT source ID
       # ... other CSI attributes
     claimRef:
       name: pvc-data
       namespace: app-namespace
   ```

2. **Create PVC** bound to the PV above.

3. **Create VolumeReplication CR** on the secondary cluster and promote:

   ```yaml
   apiVersion: replication.storage.openshift.io/v1alpha1
   kind: VolumeReplication
   metadata:
     name: vr-pvc-data
   spec:
     volumeReplicationClass: example-replication
     dataSource:
       kind: PersistentVolumeClaim
       name: pvc-data
     replicationState: primary
   ```

4. **Demote on the old primary** (if still accessible):

   Update the VR on the old primary cluster to `replicationState: secondary`.

#### Step 4: Failback to Original Primary

When failing back, the DR orchestrator reverses the process:

1. Read `destinationVolumeID` from the VR status on the **current primary**
   (which was the secondary). This gives the volume ID on the original
   primary.
2. Demote on the current primary.
3. Create PV/PVC on the original primary using the correct volume ID.
4. Promote on the original primary.

### Workflow 2: Volume Group Replication (Dynamic Grouping)

Dynamic grouping adds complexity because the set of volumes in the group
changes over time as PVCs matching the label selector are added or removed.

#### Step 1: Setup Group Replication on Primary Cluster

```yaml
apiVersion: replication.storage.openshift.io/v1alpha1
kind: VolumeGroupReplication
metadata:
  name: vgr-app-data
spec:
  volumeGroupReplicationClassName: example-group-replication
  volumeReplicationClassName: example-replication
  source:
    selector:
      matchLabels:
        app: myapp
  replicationState: primary
```

#### Step 2: Read Destination Info from VGR Status on Primary Cluster

Once replication is established, the VGR status contains the full mapping
with per-PVC volume handle correlation:

```yaml
status:
  state: Primary
  destinationVolumeGroupID: "dest-group-0001"
  persistentVolumeClaimsRefList:
    - name: pvc-data-1
      volumeHandleMapping:
        sourceVolumeHandle: "src-vol-001"
        destinationVolumeHandle: "dest-vol-001"
    - name: pvc-data-2
      volumeHandleMapping:
        sourceVolumeHandle: "src-vol-002"
        destinationVolumeHandle: "dest-vol-002"
    - name: pvc-data-3
      volumeHandleMapping:
        sourceVolumeHandle: "src-vol-003"
        destinationVolumeHandle: "dest-vol-003"
  conditions:
    - type: DestinationInfoAvailable
      status: "True"
      reason: DestinationInfoUpdated
```

The DR orchestrator MUST:

1. Watch the `DestinationInfoAvailable` condition on the VGR.
2. Only use destination IDs when the condition is `True`.
3. Store the `persistentVolumeClaimsRefList` and
   `destinationVolumeGroupID` in its own state.
4. **Read destination volume handles directly from each PVC entry's
   `volumeHandleMapping`** — no need to look up PV objects or
   cross-reference maps. When `volumeHandleMapping` is nil, destination
   info is not yet available for that PVC.

#### Step 3: Handle Dynamic Group Membership Changes on Primary Cluster

When a new PVC with matching labels is created (or an existing PVC's labels
change), the group membership updates dynamically:

1. The VGR controller detects the PVC list change.
2. `DestinationInfoAvailable` transitions to `False`
   (Reason: `DestinationInfoPending`).
3. The DR orchestrator observes this and MUST:
   - **Stop relying on the current destination mapping** as it is stale.
   - **Wait** for `DestinationInfoAvailable` to become `True` again.
4. After `ModifyVolumeGroupMembership` completes and
   `GetReplicationDestinationInfo` returns updated mappings,
   `DestinationInfoAvailable` transitions back to `True`.
5. The DR orchestrator reads the updated `volumeHandleMapping` on each
   PVC entry, which now includes mappings for the newly added volumes
   (and omits mappings for removed volumes).
6. The DR orchestrator updates its stored mapping accordingly.

```console
  PVC-4 created with label app=myapp
      |
      v
  VGR reconcile: PVC list changed
  VGR clears destination fields, sets condition=False
  DestinationInfoAvailable: False (DestinationInfoPending)
      |  DR orchestrator sees condition=False, stops using stale mapping
      v
  VGRContent.Spec.Source.VolumeHandles updated with new handle
  VGRContent controller reconciles:
    - ModifyVolumeGroupMembership called
    - GetReplicationDestinationInfo called
    - VGRContent.Status updated with new destination map
      |
      v
  VGR reconciles (triggered by VGRContent status watch):
    - Reads VGRContent.Status destination map
    - Validates map covers all volume handles
    - Copies to VGR.Status
  DestinationInfoAvailable: True (DestinationInfoUpdated)
  PVC ref for pvc-data-4 now includes volumeHandleMapping:
    sourceVolumeHandle: "src-vol-004", destinationVolumeHandle: "dest-vol-004"
      |
      v
  DR orchestrator reads updated mapping, stores it
```

#### Step 4: Failover to Secondary Cluster

The DR orchestrator performs failover using the **complete** destination
mapping. This requires recreating the full VolumeGroup resource hierarchy
on the secondary cluster with the correct destination identifiers.

##### Step 4a: Create PVs with Destination Volume IDs

For each PVC in `persistentVolumeClaimsRefList`, create a PV on the
secondary cluster using `volumeHandleMapping.destinationVolumeHandle`
directly from the PVC entry — no need to look up PV objects or
cross-reference maps:

```yaml
# For each entry in persistentVolumeClaimsRefList:
#   name: pvc-data-1, volumeHandleMapping.destinationVolumeHandle: "dest-vol-001"
#   name: pvc-data-2, volumeHandleMapping.destinationVolumeHandle: "dest-vol-002"
#   name: pvc-data-3, volumeHandleMapping.destinationVolumeHandle: "dest-vol-003"

apiVersion: v1
kind: PersistentVolume
metadata:
  name: pv-data-1-failover
spec:
  csi:
    driver: example.csi.com
    volumeHandle: "dest-vol-001" # destination ID from the map
    # ... other CSI attributes (fsType, volumeAttributes, etc.)
  claimRef:
    name: pvc-data-1
    namespace: app-namespace
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: pv-data-2-failover
spec:
  csi:
    driver: example.csi.com
    volumeHandle: "dest-vol-002"
    # ...
  claimRef:
    name: pvc-data-2
    namespace: app-namespace
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: pv-data-3-failover
spec:
  csi:
    driver: example.csi.com
    volumeHandle: "dest-vol-003"
    # ...
  claimRef:
    name: pvc-data-3
    namespace: app-namespace
```

##### Step 4b: Create PVCs

Create PVCs bound to the PVs above, preserving the original PVC names and
labels so the application can start without configuration changes and the
label selector in the VGR can match them:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: pvc-data-1
  namespace: app-namespace
  labels:
    app: myapp # same labels as on primary, required for VGR selector
spec:
  volumeName: pv-data-1-failover
  # ... storageClassName, accessModes, resources
```

##### Step 4c: Create VolumeGroupReplicationContent with Destination Group Handle

The DR orchestrator MUST create the `VolumeGroupReplicationContent` on the
secondary cluster **before** creating the VGR, using the destination
volume group ID as the `volumeGroupReplicationHandle` and the destination
volume IDs as the `volumeHandles`:

```yaml
apiVersion: replication.storage.openshift.io/v1alpha1
kind: VolumeGroupReplicationContent
metadata:
  name: vgrcontent-failover
spec:
  provisioner: example.csi.com
  volumeGroupReplicationClassName: example-group-replication
  # Use the DESTINATION group handle, not the source group handle
  volumeGroupReplicationHandle: "dest-group-0001"
  source:
    # List the DESTINATION volume handles, not the source handles
    volumeHandles:
      - "dest-vol-001"
      - "dest-vol-002"
      - "dest-vol-003"
  # volumeGroupReplicationRef will be set once VGR is created
```

This is critical because:

- The CSI driver on the secondary cluster knows the volumes by their
  **destination** IDs. Using source IDs would cause the driver to fail
  to find the volumes.
- The `volumeGroupReplicationHandle` tells the driver which existing
  volume group to manage, without creating a new one.

##### Step 4d: Create VolumeGroupReplication and Promote

Create the VGR on the secondary cluster, referencing the pre-created
VGRContent:

```yaml
apiVersion: replication.storage.openshift.io/v1alpha1
kind: VolumeGroupReplication
metadata:
  name: vgr-app-data
  namespace: app-namespace
spec:
  volumeGroupReplicationClassName: example-group-replication
  volumeReplicationClassName: example-replication
  # Reference the pre-created VGRContent
  volumeGroupReplicationContentName: vgrcontent-failover
  source:
    selector:
      matchLabels:
        app: myapp
  replicationState: primary # promote on secondary
```

The VGR controller on the secondary cluster will:

1. Find the existing VGRContent (already created with destination handles).
2. Skip creating a new volume group since
   `volumeGroupReplicationHandle` is already set.
3. Create a VR CR referencing the VGR.
4. The VR controller promotes the group to primary.

##### Step 4e: Demote on the Old Primary

If the old primary cluster is still accessible, update the VGR to
`replicationState: secondary`:

```yaml
# On old primary cluster
spec:
  replicationState: secondary
```

##### Failover Summary Diagram

```console
Primary Cluster (old)                Secondary Cluster (new primary)
=====================                ==============================

VGR (vgr-app-data)                   PV (dest-vol-001) <-- Step 4a
  state: primary                     PV (dest-vol-002)
  destinationVolumeGroupID:          PV (dest-vol-003)
    "dest-group-0001"                    |
  persistentVolumeClaimsRefList:     PVC (pvc-data-1)  <-- Step 4b
    pvc-data-1:                      PVC (pvc-data-2)
      volumeHandleMapping:           PVC (pvc-data-3)
        src: src-vol-001                 |
        dest: dest-vol-001
    pvc-data-2:
      volumeHandleMapping:
        src: src-vol-002
        dest: dest-vol-002
    pvc-data-3:
      volumeHandleMapping:
        src: src-vol-003
        dest: dest-vol-003               |
        |                            VGRContent         <-- Step 4c
        | DR reads                     handle: "dest-group-0001"
        | destination                  volumeHandles:
        | info                           - dest-vol-001
        |                                - dest-vol-002
        +------>                         - dest-vol-003
                                         |
                                     VGR (vgr-app-data) <-- Step 4d
                                       state: primary
                                       contentName: vgrcontent-failover
                                         |
                                     VR (auto-created by VGR controller)
                                       state: primary
```

#### Step 5: Failback to Original Primary

When failing back, the DR orchestrator reverses the process. The key
difference from initial setup is that the DR orchestrator now reads
destination info from the **current primary** (the secondary cluster after
failover) to discover the volume IDs on the original primary.

##### Step 5a: Read Destination Info from Current Primary

On the secondary cluster (now acting as primary), read the VGR status:

```yaml
# On secondary cluster (current primary)
status:
  state: Primary
  destinationVolumeGroupID: "src-group-0001" # points back to original primary
  persistentVolumeClaimsRefList:
    - name: pvc-data-1
      volumeHandleMapping:
        sourceVolumeHandle: "dest-vol-001" # source on this cluster
        destinationVolumeHandle: "src-vol-001" # destination = original primary
    - name: pvc-data-2
      volumeHandleMapping:
        sourceVolumeHandle: "dest-vol-002"
        destinationVolumeHandle: "src-vol-002"
    - name: pvc-data-3
      volumeHandleMapping:
        sourceVolumeHandle: "dest-vol-003"
        destinationVolumeHandle: "src-vol-003"
```

Note: The destination handles from the secondary cluster's perspective
point to the original primary's volume IDs. The DR orchestrator reads
`volumeHandleMapping.destinationVolumeHandle` from each PVC entry directly.

> **SP Requirement:** The storage provider MUST return symmetric destination
> mappings. When cluster A is primary and reports volume X maps to
> destination volume Y on cluster B, then after failover when cluster B
> becomes primary, calling `GetReplicationDestinationInfo` on cluster B
> MUST report that volume Y maps to destination volume X on cluster A.

##### Step 5b: Demote on Current Primary

Update VGR on the secondary cluster to `replicationState: secondary`.

##### Step 5c: Recreate Resources on Original Primary

On the original primary cluster:

1. **Create PVs** using the volume IDs from the destination mapping
   (which are the original source IDs):

   ```yaml
   apiVersion: v1
   kind: PersistentVolume
   metadata:
     name: pv-data-1
   spec:
     csi:
       driver: example.csi.com
       volumeHandle: "src-vol-001" # original source ID
   ```

2. **Create PVCs** bound to those PVs with matching labels.

3. **Create VolumeGroupReplicationContent** with the original group handle
   and source volume handles:

   ```yaml
   apiVersion: replication.storage.openshift.io/v1alpha1
   kind: VolumeGroupReplicationContent
   metadata:
     name: vgrcontent-failback
   spec:
     provisioner: example.csi.com
     volumeGroupReplicationClassName: example-group-replication
     volumeGroupReplicationHandle: "src-group-0001"
     source:
       volumeHandles:
         - "src-vol-001"
         - "src-vol-002"
         - "src-vol-003"
   ```

4. **Create VolumeGroupReplication** referencing the VGRContent and promote:

   ```yaml
   apiVersion: replication.storage.openshift.io/v1alpha1
   kind: VolumeGroupReplication
   metadata:
     name: vgr-app-data
   spec:
     volumeGroupReplicationClassName: example-group-replication
     volumeReplicationClassName: example-replication
     volumeGroupReplicationContentName: vgrcontent-failback
     source:
       selector:
         matchLabels:
           app: myapp
     replicationState: primary
   ```

### Summary: DR Orchestrator Responsibilities

| Responsibility          | Primary Cluster                                                                  | Secondary Cluster                            |
| ----------------------- | -------------------------------------------------------------------------------- | -------------------------------------------- |
| Watch condition         | `DestinationInfoAvailable` on VR/VGR                                             | N/A (until failover)                         |
| Store mapping           | Read `volumeHandleMapping.destinationVolumeHandle` from each PVC entry in status | N/A                                          |
| Handle staleness        | Stop using mapping when condition is `False`, wait for `True`                    | N/A                                          |
| Failover: create PVs    | N/A                                                                              | Use destination volume IDs as `volumeHandle` |
| Failover: create PVCs   | N/A                                                                              | Bind to PVs, preserve original PVC names     |
| Failover: create VR/VGR | Demote to secondary                                                              | Create and promote to primary                |
| Failback                | Reverse: read destination IDs from current primary, create PVs                   | Demote to secondary                          |

### Workflow 3: When GET_REPLICATION_DESTINATION_INFO Is Not Supported

The `GET_REPLICATION_DESTINATION_INFO` capability is optional. Storage
providers where source and destination volume IDs are identical do not need
to implement this RPC. The DR orchestrator MUST handle this case gracefully.

#### How to Detect the Legacy Case

The DR orchestrator can determine that destination info is not available
through these indicators:

1. **`DestinationInfoAvailable` condition is absent** from the VR/VGR
   status conditions list entirely. This means the controller never
   attempted to fetch destination info because the driver does not advertise
   the `GET_REPLICATION_DESTINATION_INFO` capability.

2. **`destinationVolumeID` field is empty** on VR status (or
   `destinationVolumeGroupID` is empty and `volumeHandleMapping` is nil
   on all PVC entries in VGR status).

The DR orchestrator SHOULD use the following check:

```go
func getDestinationVolumeID(vrStatus VolumeReplicationStatus, sourceVolumeID string) string {
    if vrStatus.DestinationVolumeID != "" {
        return vrStatus.DestinationVolumeID
    }
    // Capability not supported: source ID equals destination ID
    return sourceVolumeID
}
```

#### Individual Volume Replication Without Destination Info

##### Step 1: Setup (Same as Workflow 1)

Create VR CR on the primary cluster. No difference.

##### Step 2: Read VR Status on Primary Cluster

The VR status will have replication state and sync metrics but no
destination fields:

```yaml
status:
  state: Primary
  # destinationVolumeID is absent
  # DestinationInfoAvailable condition is absent
  conditions:
    - type: Completed
      status: "True"
      reason: Promoted
    - type: Replicating
      status: "True"
      reason: Replicating
```

The DR orchestrator notes that `destinationVolumeID` is empty and
`DestinationInfoAvailable` condition is absent. It concludes that source
and destination IDs are identical and records the source volume ID
(from `PV.spec.csi.volumeHandle`) as the volume ID to use on both
clusters.

##### Step 3: Failover to Secondary Cluster for VR

The DR orchestrator uses the **source volume ID** directly as the volume
handle when creating the PV on the secondary cluster:

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: pv-data-failover
spec:
  csi:
    driver: example.csi.com
    volumeHandle: "src-vol-0001" # same as source, since no destination info
    # ... other CSI attributes
  claimRef:
    name: pvc-data
    namespace: app-namespace
```

The remaining steps (create PVC, create VR, promote, demote old primary)
are identical to Workflow 1.

##### Step 4: Failback

Same as Workflow 1 Step 4, using the source volume ID since it is the same
on both clusters.

#### Volume Group Replication Without Destination Info

##### Step 1-2: Setup and Read Status (Same as Workflow 2)

Create VGR CR. The VGR status will have `persistentVolumeClaimsRefList`
but no destination fields:

```yaml
status:
  state: Primary
  # destinationVolumeGroupID is absent
  # volumeHandleMapping is absent on all entries
  # DestinationInfoAvailable condition is absent
  persistentVolumeClaimsRefList:
    - name: pvc-data-1
    - name: pvc-data-2
    - name: pvc-data-3
```

The DR orchestrator concludes that source IDs equal destination IDs. It
builds the volume mapping by reading each PVC's bound PV and using
`PV.spec.csi.volumeHandle` as both source and destination ID.

##### Step 3: Handle Dynamic Group Membership Changes

Without destination info, the DR orchestrator cannot observe the
`DestinationInfoAvailable` condition (it is absent). Instead:

1. Watch `persistentVolumeClaimsRefList` on VGR status for membership
   changes.
2. When PVCs are added or removed, update the stored mapping by reading
   the new PVCs' bound PVs.
3. Since source ID equals destination ID, no additional RPC response is
   needed. The PV volume handle is the correct ID for both clusters.

##### Step 4: Failover

Since source and destination IDs are identical, the DR orchestrator uses
the source volume handles (from PVs on the primary cluster) directly.

###### Step 4a: Create PVs on Secondary Cluster

For each PVC in `persistentVolumeClaimsRefList`, read the bound PV's
`spec.csi.volumeHandle` and create PVs on the secondary cluster using the
same handle:

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: pv-data-1
spec:
  capacity:
    storage: 10Gi
  accessModes:
    - ReadWriteOnce
  persistentVolumeReclaimPolicy: Retain
  csi:
    driver: example.csi.com
    volumeHandle: "src-vol-001" # same as source (no destination mapping)
    # ... other CSI attributes (fsType, volumeAttributes, nodeStageSecretRef)
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: pv-data-2
spec:
  capacity:
    storage: 10Gi
  accessModes:
    - ReadWriteOnce
  persistentVolumeReclaimPolicy: Retain
  csi:
    driver: example.csi.com
    volumeHandle: "src-vol-002"
---
apiVersion: v1
kind: PersistentVolume
metadata:
  name: pv-data-3
spec:
  capacity:
    storage: 10Gi
  accessModes:
    - ReadWriteOnce
  persistentVolumeReclaimPolicy: Retain
  csi:
    driver: example.csi.com
    volumeHandle: "src-vol-003"
```

###### Step 4b: Create PVCs on Secondary Cluster

Create PVCs that bind to the PVs above. The PVCs must have matching labels
so the VGR's label selector can discover them:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: pvc-data-1
  namespace: app-namespace
  labels:
    app: myapp # must match VGR source.selector.matchLabels
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
  volumeName: pv-data-1 # bind to the pre-created PV
---
# Repeat for pvc-data-2 and pvc-data-3
```

###### Step 4c: Create VolumeGroupReplicationContent on Secondary Cluster

Since the group handle is the same on both clusters (source = destination),
use the original group handle from the primary cluster's VGRContent:

```yaml
apiVersion: replication.storage.openshift.io/v1alpha1
kind: VolumeGroupReplicationContent
metadata:
  name: vgrcontent-failover
spec:
  provisioner: example.csi.com
  volumeGroupReplicationClassName: example-group-replication
  volumeGroupReplicationHandle: "src-group-0001" # same as source
  source:
    volumeHandles:
      - "src-vol-001" # same as source (no destination mapping)
      - "src-vol-002"
      - "src-vol-003"
```

###### Step 4d: Create VolumeGroupReplication on Secondary Cluster

Create the VGR referencing the pre-created VGRContent and promote:

```yaml
apiVersion: replication.storage.openshift.io/v1alpha1
kind: VolumeGroupReplication
metadata:
  name: vgr-app-data
  namespace: app-namespace
spec:
  volumeGroupReplicationClassName: example-group-replication
  volumeReplicationClassName: example-replication
  volumeGroupReplicationContentName: vgrcontent-failover
  source:
    selector:
      matchLabels:
        app: myapp
  replicationState: primary
```

The VGR controller will:

1. Bind VGR to the pre-existing VGRContent.
2. Create individual VR CRs for each PVC.
3. Call `PromoteVolume` for each volume in the group.

###### Step 4e: Demote on Old Primary

Update VGR on the old primary cluster to `replicationState: secondary`.

##### Step 5: Failback

Since source and destination IDs are identical, failback is
straightforward:

1. **Demote** VGR on the current primary (secondary cluster) to secondary.
2. **Recreate resources** on the original primary using the same source
   volume IDs (since they are the same on both clusters). Follow the same
   pattern as Step 4a-4d with the original source IDs.
3. **Promote** VGR on the original primary.

#### Decision Flow for DR Orchestrator

```console
Start: VR/VGR status available
    |
    v
Is DestinationInfoAvailable condition present?
    |                           |
   Yes                         No
    |                           |
    v                           v
Is condition status "True"?    Capability not supported.
    |           |              Use source volume ID as
   Yes         No              destination volume ID.
    |           |              (Legacy behavior)
    v           v
Use             Wait for
destination     condition to
IDs from        become "True"
status          before
fields.         proceeding.
```

### Important Considerations for DR Orchestrator Implementers

1. **Never assume source ID equals destination ID when destination info is
   available.** When `destinationVolumeID` / `destinationVolumeIDs` fields
   are populated, always use them. Only fall back to source ID when these
   fields are absent.

2. **Always check the `DestinationInfoAvailable` condition before using
   destination IDs.** If the condition is `False`, the mapping may be stale.
   If the condition is absent entirely, the capability is not supported and
   source IDs should be used.

3. **For volume groups, iterate `persistentVolumeClaimsRefList`.** Each
   entry provides the PVC name, source volume handle, and destination
   volume handle directly. No need to cross-reference PV objects or
   look up maps.

4. **Implement a unified volume ID resolution function.** The DR
   orchestrator should have a single code path that resolves the correct
   volume ID for a given cluster:

   ```go
   func resolveVolumeID(vrStatus VolumeReplicationStatus, sourceVolumeID string) string {
       // If destination info is available, use it
       if vrStatus.DestinationVolumeID != "" {
           return vrStatus.DestinationVolumeID
       }
       // Otherwise, source and destination are identical
       return sourceVolumeID
   }
   ```

5. **During dynamic group changes when capability is supported, the DR
   orchestrator must not trigger failover while `DestinationInfoAvailable`
   is `False`.** The destination mapping is incomplete and failover would
   result in missing or incorrect PVs on the secondary cluster.

6. **During dynamic group changes when capability is NOT supported, the DR
   orchestrator can proceed with failover at any time.** Since source and
   destination IDs are identical, the PVC list in VGR status is sufficient
   to build the complete mapping. The orchestrator only needs to wait for
   `persistentVolumeClaimsRefList` to stabilize.
