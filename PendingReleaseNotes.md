# v0.13.0 Pending Release Notes

## Breaking changes

## Features

- Allow override of precedence of key rotation and reclaim space related annotations
  using a ConfigMap key `schedule-precedence`. The default is `pvc` which reads the
  annotations in order of PVC > NS > SC. It can be set to `storageclass` to respect only
  the annotations found on the Storage Classes.
- Allow VolumeGroupReplication to be managed by a storage vendor specific implementation
  of the controller by specifying `external` as `true` in the VGR's `spec`. The default is
  `false`, which means VolumeGroupReplication will be reconciled by the csi-addons controller.

## NOTE

- `sc-only`, a once valid value for `schedule-precedence` key is being deprecated in favor of
  `storageclass` and will be removed in a later release.
