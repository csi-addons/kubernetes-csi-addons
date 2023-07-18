# CSI Volume Group Operator

## Overview

Volume Group Operator is based on [CSIAddons/volume-group spec](https://github.com/csi-addons/spec/tree/main/volumegroup) specification and can be used by any storage
provider.

## Design

Volume Group Operator follows controller pattern and provides extended APIs for volume grouping.
The extended APIs are provided via Custom Resource Definition (CRD).

### [VolumeGroupClass](config/crd/bases/volumegroup.storage.openshift.io_volumegroupclasses.yaml)

`VolumeGroupClass` is a cluster scoped resource that contains driver related configuration parameters.

`driver` is name of the storage provisioner

`parameters` contains key-value pairs that are passed down to the driver. Users can add their own key-value pairs.
Keys with `volumegroup.storage.openshift.io/` prefix are reserved by operator and not passed down to the driver.

#### Reserved parameter keys

- `volumegroup.storage.openshift.io/secret-name`
- `volumegroup.storage.openshift.io/secret-namespace`

```yaml
apiVersion: volumegroup.storage.openshift.io/v1
kind: VolumeGroupClass
metadata:
  name: volume-group-class-sample
spec:
  driver: example.provisioner.io
  parameters:
    volumegroup.storage.openshift.io/secret-name: demo-secret
    volumegroup.storage.openshift.io/secret-namespace: default
```

### [VolumeGroupContent](config/crd/bases/volumegroup.storage.openshift.io_volumegroupcontents.yaml)

VolumeGroupContent is a namespaced resource that contains references to storage objects that are part of a group.

`driver` is name of the storage provisioner

`volumeGroupHandle` is the unique identifier for the group

`VolumeGroupDeletionPolicy` is the deletion policy for the group. Possible values are `Delete` and `Retain`.

`VolumeGroupClassName` is the name of the `VolumeGroupClass` that contains driver related configuration parameters.

```yaml
apiVersion: volumegroup.storage.openshift.io/v1
kind: VolumeGroupContent
metadata:
  name: volume-group-content-sample
  namespace: default
spec:
  source:
    driver: example.provisioner.io
    volumeGroupHandle: volume-group-handle-sample
  VolumeGroupDeletionPolicy: Delete
  VolumeGroupClassName: volume-group-class-sample
```

### [VolumeGroup](config/crd/bases/volumegroup.storage.openshift.io_volumegroups.yaml)

VolumeGroup is a namespaced resource that contains references to storage objects that are part of a group.

`driver` is name of the storage provisioner

`selector` is a label selector that is used to select `PVC` objects that are part of the group.

`VolumeGroupClassName` is the name of the `VolumeGroupClass` that contains driver related configuration parameters.

```yaml
apiVersion: volumegroup.storage.openshift.io/v1
kind: VolumeGroup
metadata:
  name: volume-group-sample
  namespace: default
spec:
  VolumeGroupClassName: volume-group-class-sample
  source:
    driver: example.provisioner.io
    selector:
      matchLabels:
        volume-group-key: volume-group-value
```

## VolumeGroup Capabilities

- `Capability_VolumeGroup_VOLUME_GROUP` - When a driver supports this capability then it supports volume group feature.
- `Capability_VolumeGroup_LIMIT_VOLUME_TO_ONE_VG` - When a driver supports this capability then it means that it can't support that one PVC will be in multiple VGs. By default if a Driver doesn't add this capability then it will allow the controller to add a PVC to more than one VG.
- `Capability_VolumeGroup_DO_NOT_ALLOW_VG_TO_DELETE_VOLUMES` - When a driver supports this capability then it Doesn't allow the controller to delete PVCs that are under a VG in a VG deletion, so when a VG will be deleted the controller will only be able to delete the VG. By default if a driver doesn't add this capability then it will allow the controller to delete the PVCs that are under this VG first and then it will delete the VG.
