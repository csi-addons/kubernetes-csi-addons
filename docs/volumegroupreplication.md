# VolumeGroupReplication

VolumeGroupReplication is a namespaced resource that contains references to storage object to be grouped and replicated, VolumeGroupReplicationClass corresponding to the driver providing replication, VolumeGroupReplicationContent and VolumeReplication CRs.

`volumeGroupReplicationClassName` is the name of the class providing group replication.

`volumeReplicationClassName` is the name of the class providing the replication for volumeReplication CR.

`volumeReplicationName` is the name of the volumeReplication CR created by this volumeGroupReplication CR.

`volumeGroupReplicationContentName` is the name of the volumeGroupReplicationContent CR created for this volumeGroupReplication CR.

`replicationState` is the state of the volume group being referenced. Possible values are `primary`, `secondary` and `resync`.

- `primary` denotes that the volume group is primary
- `secondary` denotes that the volume group is secondary
- `resync` denotes that the volume group needs to be resynced

`source` contains the source of the volumeGroupReplication i.e, the selectors to match the PVC/PVs to be replicated.

- `selector` is a label selector to filter the pvcs that are to be included in the group replication

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
  autoResync: false
status:
  persistentVolumeClaimsRefList:
    - test-pvc
  state: primary
```

Example PVC having matching labels:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: test-pvc
  labels:
    appname: test
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: rook-ceph-block
```
