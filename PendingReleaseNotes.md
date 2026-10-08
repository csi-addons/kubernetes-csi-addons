# v0.15.0 Pending Release Notes

## Breaking changes

- The controller-side reclaim space operation (`ControllerReclaimSpace`, for example `rbd sparsify`)
  is no longer performed unless `Controller` is listed in `ReclaimSpaceJob.spec.operations`. The
  field defaults to `["Node"]`, so existing `ReclaimSpaceJob` and `ReclaimSpaceCronJob` CRs only
  perform the node-side reclaim after an upgrade.

## Features

- `ReclaimSpaceJob.spec.operations` selects which reclaim space operations are performed on a
  volume. Valid values are `Controller` and `Node`, at least one must be specified.

- Added NetworkPolicy for the controller-manager pod. Included in all generated manifests by default. Denies all ingress and allows open egress for API server and sidecar gRPC connectivity.
- Volume Condition Reporter now uses `NodeGetVolumeHealth` (CSI spec v1.13.0) as the primary method for health detection. Drivers that advertise the legacy `VOLUME_CONDITION` capability (CSI spec v1.12.0 and earlier) continue to work via `NodeGetVolumeStats`.

## NOTE
