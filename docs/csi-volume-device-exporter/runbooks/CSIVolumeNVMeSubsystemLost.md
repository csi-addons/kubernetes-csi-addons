# CSIVolumeNVMeSubsystemLost

## Meaning

All NVMe-oF controller paths for a subsystem backing a Kubernetes
PersistentVolume are dead. The subsystem has no live controllers and I/O is
likely failing.

## Impact

Workloads (including VMs) using the affected PersistentVolume are experiencing
I/O errors or are hung. This is a critical condition requiring immediate action.

## Diagnosis

```bash
NODE=<node label from alert>
SUBSYSTEM=<subsystem label from alert>
PV=<persistentvolume label from alert>
```

1. Check NVMe subsystem state:

   ```bash
   oc debug node/$NODE -- chroot /host nvme list-subsys /dev/$SUBSYSTEM
   ```

   All controllers should show as `dead` or `connecting`.

2. Check kernel messages:

   ```bash
   oc debug node/$NODE -- chroot /host dmesg | grep -i nvme | tail -50
   ```

3. Identify affected workloads:

   ```bash
   kubectl get pv $PV -o jsonpath='{.spec.claimRef.namespace}/{.spec.claimRef.name}'
   ```

## Mitigation

All NVMe-oF controllers are down simultaneously, which typically indicates:

- Complete loss of network connectivity to the NVMe-oF target
- Storage array failure or maintenance
- Network fabric failure

**Immediate actions:**

1. Contact your storage administrator to verify the NVMe-oF target is healthy.
2. Check network connectivity between the node and all target portals.
3. If the workload is a VM, consider live-migrating it to a node with working connectivity (if the storage is reachable from other nodes).

**After connectivity is restored:**

- The NVMe-oF driver should reconnect automatically.
- If it does not, attempt manual reconnection:

  ```bash
  oc debug node/$NODE -- chroot /host nvme connect-all
  ```

The alert will clear once at least one controller returns to `live` state.
