# CSIVolumeNVMeSubsystemDegraded

## Meaning

A Kubernetes PersistentVolume backed by an NVMe-oF (NVMe over Fabrics) subsystem
has at least one controller path that is not in the `live` state.

The subsystem is still accessible via remaining live controllers, but redundancy
is reduced.

## Impact

Workloads (including VMs) using the affected PersistentVolume are running with
degraded NVMe-oF path redundancy. If the remaining controllers also fail, the
workload will experience I/O errors.

## Diagnosis

```bash
NODE=<node label from alert>
SUBSYSTEM=<subsystem label from alert>
PV=<persistentvolume label from alert>
```

1. Check the NVMe subsystem controller states:

   ```bash
   oc debug node/$NODE -- chroot /host nvme list-subsys /dev/$SUBSYSTEM
   ```

2. Check for NVMe errors in kernel logs:

   ```bash
   oc debug node/$NODE -- chroot /host dmesg | grep -i nvme | tail -30
   ```

3. Identify the affected workload:

   ```bash
   kubectl get pv $PV -o jsonpath='{.spec.claimRef.namespace}/{.spec.claimRef.name}'
   ```

## Mitigation

**Network connectivity issue (NVMe/TCP or NVMe/RDMA):**

- Check network connectivity between the node and the storage target.
- Verify the NVMe-oF target portal is reachable from the node.

**Controller failure on the storage array:**

- Contact your storage administrator to check the health of the NVMe target controllers.

**Reconnection:**

- The NVMe-oF driver should automatically reconnect. If it does not:

  ```bash
  oc debug node/$NODE -- chroot /host nvme connect-all
  ```

The alert will clear once all controllers return to `live` state.
