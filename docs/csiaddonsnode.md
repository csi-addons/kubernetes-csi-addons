# CSIAddonsNode

CSIAddonsNode is a custom resource designed to allow the CSI-Addons Controller(s) to discover CSI-Addons side-cars that are running alongside of CSI-driver components. The information provided in a CSIAddonsNode CR contain details on how to connect to the CSI-Addons side-car.

```yaml
apiVersion: csiaddons.openshift.io/v1alpha1
kind: CSIAddonsNode
metadata:
  name: csiaddonsnode-sample
spec:
  driver:
    name: driver.csi.example.io
    endpoint: pod://csiaddonsnode-sample.csi-addons-system:9070
    nodeID: node-1
```

- `driver` contains the required information about the CSI driver.
  - `name` contains the name of the driver. The name of the driver is in the format: `driver.csi.example.io`
  - `endpoint` contains the URL that contains the name of the Pod and its Namespace that can be used by the controller to connect to.
  - `nodeID` contains the ID of node to identify on which node the side-car is running.

## Lifecycle of CSIAddonsNode CR

The CSIAddonsNode custom resource (CR) is created by the sidecar that runs with the CSI driver.
The owner of the CSIAddonsNode CR is the main owner of the sidecar pod, whether it's a DaemonSet or a Deployment.
When the sidecar pod is deleted, the CSIAddonsNode CR is not removed; instead, it gets updated with new information when the pod is recreated.
The CSIAddonsNode CR is deleted only when the CSI driver’s Deployment or DaemonSet is deleted.
