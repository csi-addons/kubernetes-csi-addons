# CSIVolumeMultipathLost

## Meaning

All paths to a DM-multipath block device backing a Kubernetes PersistentVolume
are down. The device has no remaining active paths and I/O is likely failing.

## Impact

Workloads (including VMs) using the affected PersistentVolume are experiencing
I/O errors or are hung waiting for I/O. This is a critical condition that
requires immediate attention.

## Diagnosis

The alert labels identify the affected resources:

```bash
NODE=<node label from alert>
DEVICE=<device label from alert, e.g. mpathA>
PV=<persistentvolume label from alert>
```

1. Check the multipath device state:

   ```bash
   oc debug node/$NODE -- chroot /host multipath -ll $DEVICE
   ```

   All paths should show as failed/offline/dead.

2. Check kernel messages for storage errors:

   ```bash
   oc debug node/$NODE -- chroot /host dmesg | grep -i "scsi\|fc\|iscsi\|nvme\|multipath" | tail -50
   ```

3. Identify affected workloads:

   ```bash
   kubectl get pv $PV -o jsonpath='{.spec.claimRef.namespace}/{.spec.claimRef.name}'
   ```

## Mitigation

This is a critical condition. All storage paths are down simultaneously, which
typically indicates:

- Complete loss of network connectivity to the storage array
- Storage array failure or maintenance
- Multiple simultaneous fabric/switch failures

**Immediate actions:**

1. Contact your storage administrator to verify the storage array is healthy.
2. Check all network paths between the node and the storage array (switches, HBAs, cables).
3. If the workload is a VM, consider live-migrating it to a node with working storage paths (if the VM's storage is accessible from other nodes).

**After storage connectivity is restored:**

- Paths should recover automatically via `multipathd`.
- If paths do not recover, restart `multipathd`:

  ```bash
  oc debug node/$NODE -- chroot /host systemctl restart multipathd
  ```

The alert will clear once at least one path returns to active state. The
CSIVolumeMultipathDegraded alert may remain until all paths are restored.
