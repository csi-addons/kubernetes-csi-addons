# CSIVolumeMultipathDegraded

## Meaning

A Kubernetes PersistentVolume backed by a DM-multipath block device has at
least one faulty path on the node where it is currently attached.

The device is still accessible as long as at least one active path remains, but
redundancy is reduced and a second path failure will cause I/O errors for any
workload using this volume.

## Impact

Workloads (including VMs) using the affected PersistentVolume are running with
degraded storage redundancy. If the remaining paths also fail, the workload will
experience I/O errors and may become unavailable.

## Diagnosis

The alert labels identify the affected resources. Set shell variables from the
firing alert before running the commands below:

```bash
NODE=<node label from alert>
DEVICE=<device label from alert, e.g. dm-0>
PV=<persistentvolume label from alert>
```

1. Check the overall multipath state on the affected node:

   ```bash
   oc debug node/$NODE -- chroot /host multipath -ll $DEVICE
   ```

   Look for paths in `faulty`, `failed`, or `shaky` state.

2. Identify which physical paths are down:

   ```bash
   oc debug node/$NODE -- chroot /host multipathd show paths format "%d %t %T %s"
   ```

3. Check which workload is using the PV:

   ```bash
   kubectl get pv $PV -o jsonpath='{.spec.claimRef.namespace}/{.spec.claimRef.name}'
   ```

   Then find the pod or VM consuming that PVC:

   ```bash
   PVC=<output from above>
   kubectl get pods -A -o json | \
     jq -r '.items[] | select(.spec.volumes[]?.persistentVolumeClaim.claimName == "'${PVC#*/}'" ) | .metadata.namespace + "/" + .metadata.name'
   ```

4. Check the host HBA or NIC state for the failing path:

   ```bash
   oc debug node/$NODE -- chroot /host dmesg | grep -i "scsi\|fc\|iscsi\|nvme" | tail -50
   ```

## Mitigation

The correct action depends on the storage protocol and the root cause identified
in the Diagnosis section.

**Physical connectivity failure (FC/iSCSI/NVMe-oF):**

- Inspect and reseat any cables or SFPs on the host HBA/NIC.
- Check the switch port and zoning configuration.
- Contact your storage administrator to verify the storage array port state.

**Transient path error (path is recovering):**

- Reinstate the path manually if `multipathd` has not already done so:

  ```bash
  oc debug node/$NODE -- chroot /host multipathd reinstate path <path_device>
  ```

**`multipathd` daemon issue:**

- Restart `multipathd` on the node (only if no active I/O is in critical section):

  ```bash
  oc debug node/$NODE -- chroot /host systemctl restart multipathd
  ```

After remediation, confirm all paths are active:

```bash
oc debug node/$NODE -- chroot /host multipath -ll $DEVICE
```

The alert will clear automatically once no faulty paths are reported for the
device.
