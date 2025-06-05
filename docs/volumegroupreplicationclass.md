# VolumeGroupReplicationClass

VolumeGroupReplicationClass is a cluster scoped resource that contains driver related configuration parameters for volume group replication.

`provisioner` is name of the storage provisioner.

`parameters` contains key-value pairs that are passed down to the driver. Users can add their own key-value pairs. Keys with `replication.storage.openshift.io/` prefix are reserved by operator and not passed down to the driver.

## Reserved parameter keys

- `replication.storage.openshift.io/group-replication-secret-name`
- `replication.storage.openshift.io/group-replication-secret-namespace`

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
