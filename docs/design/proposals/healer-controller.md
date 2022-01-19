# Introduction
There is [external health monitoring](https://github.com/kubernetes-csi/external-health-monitor/) which does mostly similar stuff, but actions are not in the scope of it, all it can do is log the health condition of the mounts.

# Volume Healer Controller

**Now the obvious question we expect is "what do we achieve with the new controller?"**
* Achieve a way to monitor the health of volumes.
* We currently do not have a way to act on failed volumes. Once the controller detects that the volume is not healthy, it will have an opportunity to act on it. For example userspace mounters such as NBD has a process running per volume, this controller should ideally help restart such processes in case if they are not-started or crashed.
* This will be a generic controller design for any mounters added in the future, for userspace mounters it will offer healing in addition, like restarting the respective processes if the volume goes abnormal. Some example userspace mounters would be rbd-nbd, cephfs-fuse, etc.

# Details

                    .-------------------.
                    |     CSI-Addons    |
                    | Healer Controller |
                    '-------------------'
                              |
                              | gRPC
                              |
            .---------+------------------------------.
            |         |                              |
            |  .------------.        .------------.  |
            |  | CSI-Addons |  gRPC  |    CSI     |  |
            |  |  side-car  |--------| NodePlugin |  |
            |  '------------'        '------------'  |
            | CSI-driver Pod                         |
            '----------------------------------------'

## CSI-Addons Healer Controller
* The healer controller will be implemented as part of the CSI-Addons controller. It will trigger controller RPC to check the health condition of the CSI volumes.
* Watch for the `CSIAddonsNode` object, if the object is created/modified by the side-car do below operations
    * Once the connection is established, fetch the capabilities list and check for nodes that supports `VOLUME_HEALER` capability
    * Fetch all the VolumeAttachment` list
    * Filter the `VolumeAttachment` list through matching driver name, node name and status attached
    * For each `VolumeAttachment` get the respective PersistentVolume information and check the criteria of PersistentVolume Bound (also we can have configuration options like mounter type)
    * Send the RPC request for side-car to check if the volume is healthy along with the PersistentVolume details and may attempt to heal the volume if it is found abnormal
    * Log the final response returned from the side-car

## CSI-Addons side-car
* When a nodeplugin pod is restarted the side-car will create [CSIAddonsNode](https://github.com/csi-addons/kubernetes-csi-addons/blob/main/api/v1alpha1/csiaddonsnode_types.go) object.
* The side-car works as relay, upon receiving a RPC call from healer controller, the CSI-Addons side-car will then send a VolumeMonitor request to CSI driver. The request contains some internal details like secrets, staging path, volume-id etc.
* Upon receiving the VolumeMonitor request, the CSI driver check volume's mounting conditions, in case if any volume condition is reported abnormal, it may act on it by performing required healing and finally returns back the response to the controller.
