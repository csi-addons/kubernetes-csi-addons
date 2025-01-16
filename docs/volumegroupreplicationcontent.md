# VolumeGroupReplicationContent

VolumeGroupReplicationContent is a cluster scoped resource that contains volume grouping related information.

`volumeGroupReplicationRef` contains object reference of the volumeGroupReplication resource that created this resource.

`volumeGroupReplicationHandle` (optional) is an existing (but new) group replication ID.

`volumeGroupReplicationClassName` is the name of the VolumeGroupReplicationClass that contains the driver related info
for volume grouping.

`source` (optional) contains the VolumeGroupReplicationContentSource struct.

- `VolumeHandles` is the list of volume handles that this resource is responsible for grouping.

```yaml
apiVersion: replication.storage.openshift.io/v1alpha1
kind: VolumeGroupReplicationContent
metadata:
  name: volumegroupreplicationcontent-sample
spec:
  volumeGroupReplicationRef:
    kind: VolumeGroupReplication
    name: volumegroupreplication-sample
    namespace: default
  volumeGroupReplicationClassName: volumegroupreplicationclass-sample
  provisioner: example.provisioner.io
  source:
    volumeHandles:
      - myPersistentVolumeHandle
      - myPersistentVolumeHandle1
      - myPersistentVolumeHandle2
```
