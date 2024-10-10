# `csi-addons` admin and testing tool

The `csi-addons` executable connects to a socket where the CSI-driver listens
for CSI-Addons requests. The tool can be executed in the side-car that is
running alongside CSI-driver containers.

When running the tool with the `-h` parameter, it displays the help text,
similar to:

```console
$ kubectl exec -c csi-addons csi-backend-nodeplugin -- csi-addons -h
  -cidrs string
        comma separated list of cidrs
  -clusterid string
        clusterID
  -drivername string
        name of the CSI driver
  -endpoint string
        CSI-Addons endpoint (default "unix:///tmp/csi-addons.sock")
  -legacy
        use legacy format for old Kubernetes versions
  -operation string
        csi-addons operation
  -parameters string
        parameters in key=value format separated by commas(Eg:- k1=v1,k2=v2...)
  -persistentvolume string
        name of the PersistentVolume
  -secret namespace/name
        kubernetes secret in the format namespace/name
  -stagingpath string
        staging path (default "/var/lib/kubelet/plugins/kubernetes.io/csi/")
  -version
        print Version details
  -volumeids string
        comma separated list of VolumeIDs
  -volumegroupid string
        ID of the volume group
  -volumegroupname string
        name of the Volume Group to be created

The following operations are supported:
 - ControllerReclaimSpace
 - NodeReclaimSpace
 - GetIdentity
 - GetCapabilities
 - Probe
 - NetworkFence
 - NetworkUnFence
 - EnableVolumeReplication
 - DisableVolumeReplication
 - PromoteVolume
 - DemoteVolume
 - ResyncVolume
 - GetVolumeReplicationInfo
 - CreateVolumeGroup
 - ModifyVolumeGroupMembership
 - DeleteVolumeGroup
 - ControllerGetVolumeGroup
```

The above command assumes the running `csi-backend-nodeplugin` Pod has the
`quay.io/csiaddons/k8s-sidecar` as a container named `csi-addons`. Executing
the `csi-addons -h` command then shows the help text.
