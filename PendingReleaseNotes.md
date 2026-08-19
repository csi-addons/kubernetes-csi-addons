# v0.15.0 Pending Release Notes

## Breaking changes

## Features

- Added NetworkPolicy for the controller-manager pod. Included in all generated manifests by default. Denies all ingress and allows open egress for API server and sidecar gRPC connectivity.
- Volume Condition Reporter now uses `NodeGetVolumeHealth` (CSI spec v1.13.0) as the primary method for health detection. Drivers that advertise the legacy `VOLUME_CONDITION` capability (CSI spec v1.12.0 and earlier) continue to work via `NodeGetVolumeStats`.

## NOTE
