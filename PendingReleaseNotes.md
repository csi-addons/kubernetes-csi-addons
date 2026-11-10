# v0.15.0 Pending Release Notes

## Breaking changes

- The controller-side reclaim space operation (`ControllerReclaimSpace`, for example `rbd sparsify`)
  is no longer performed unless `Controller` is listed in `ReclaimSpaceJob.spec.operations`. The
  field defaults to `["Node"]`, so existing `ReclaimSpaceJob` and `ReclaimSpaceCronJob` CRs only
  perform the node-side reclaim after an upgrade.
- A `ReclaimSpaceJob` now fails when none of the requested operations is supported by the driver:
  the RPC returns `Unimplemented`, the node plugin does not advertise `ReclaimSpace_ONLINE`, or the
  controller does not advertise `ReclaimSpace_OFFLINE`. Previously, jobs whose RPCs returned
  `Unimplemented` reported success without reclaiming any space, and jobs with a missing
  capability were retried until they failed.
- Node-side reclaim requires a `VolumeAttachment`. A `ReclaimSpaceJob` that requests `Node` for a
  volume that is not attached to any node fails once its `backOffLimit` or `retryDeadlineSeconds`
  is reached, even if `Controller` succeeded. CSI drivers that set `attachRequired: false` in their
  `CSIDriver` object don't have `VolumeAttachment`, so their `ReclaimSpaceJob` CRs must request
  `Controller` explicitly when the driver supports it.
- `operations` is immutable on a `ReclaimSpaceJob`. A new job must be created to run different operations.
  `operations` can be modified on the `ReclaimSpaceCronJob`, future runs will use the updated `operations`.

## Features

- `ReclaimSpaceJob.spec.operations` selects which reclaim space operations are performed on a
  volume. Valid values are `Controller` and `Node`, at least one must be specified.

- `ReclaimSpaceJob.status.operations` reports the progress of each requested operation with a
  `Pending`, `Succeeded`, `Skipped` or `Failed` state, a message, the reclaimed space and the
  completion time. Operations that already succeeded or were skipped are not attempted again
  when the job is retried.

- Added NetworkPolicy for the controller-manager pod. Included in all generated manifests by default. Denies all ingress and allows open egress for API server and sidecar gRPC connectivity.
- Volume Condition Reporter now uses `NodeGetVolumeHealth` (CSI spec v1.13.0) as the primary method for health detection. Drivers that advertise the legacy `VOLUME_CONDITION` capability (CSI spec v1.12.0 and earlier) continue to work via `NodeGetVolumeStats`.

## Fixes

- A `ReclaimSpaceJob` that requests both `Controller` and `Node` now performs the controller-side
  reclaim even when the node sidecar has not registered yet, instead of exhausting the retry limit
  without reclaiming any space. The node-side operation stays `Pending` and is retried until the
  job's retry limit is reached, after which the job is marked `Failed`; the controller-side result
  and reclaimed space are kept in `status.operations`.

## NOTE
