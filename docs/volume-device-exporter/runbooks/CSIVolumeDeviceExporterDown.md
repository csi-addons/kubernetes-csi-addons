# CSIVolumeDeviceExporterDown

## Meaning

No `csi-volume-device-exporter` targets have been scraped by Prometheus for at
least 5 minutes. The exporter DaemonSet has either been removed from the cluster
or all of its pods are failing to start or stay running.

## Impact

Storage path health for CSI volumes cannot be reported. The
`CSIVolumeMultipathDegraded` alert will not fire even if multipath paths are
faulty, creating a blind spot in storage observability.

## Diagnosis

1. Check the DaemonSet status:

   ```bash
   kubectl get daemonset csi-volume-device-exporter -n <namespace>
   ```

   Look for pods that are not in the `Running` state.

2. Identify failing pods and inspect their logs:

   ```bash
   kubectl get pods -n <namespace> -l app=csi-volume-device-exporter
   kubectl logs -n <namespace> <pod-name> --previous
   ```

3. Check pod events for scheduling or mount failures:

   ```bash
   kubectl describe pod -n <namespace> <pod-name>
   ```

4. Verify the DaemonSet is present and not accidentally deleted:

   ```bash
   kubectl get daemonset -n <namespace>
   ```

5. Confirm Prometheus can reach the exporter endpoint:

   ```bash
   kubectl get podmonitor -n <namespace> csi-volume-device-exporter
   ```

## Mitigation

- If pods are crashlooping, review the logs from step 2 above for startup
  errors (missing `NODE_NAME`, inaccessible host paths, permission errors).

- If pods are in `Pending` state, a node may lack resources or have a taint
  that prevents scheduling. Review `kubectl describe node <node>`.

- If the DaemonSet was deleted, redeploy it:

  ```bash
  kubectl apply -f deploy/daemonset.yaml
  ```

- On OpenShift, also verify the SecurityContextConstraint is still bound:

  ```bash
  oc get scc csi-volume-device-exporter
  oc adm policy who-can use scc csi-volume-device-exporter
  ```

The alert will clear automatically once Prometheus successfully scrapes at least
one exporter target.
