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

VGR has a Status field that is basically the same as that of VolumeReplication’s Status field and it can be utilized by the storage vendor to populate the current status of replication to the end user and report any errors as such at the CR level.

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
