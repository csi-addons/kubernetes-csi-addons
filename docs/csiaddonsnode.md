# CSIAddonsNode

CSIAddonsNode is a custom resource designed to allow the CSI-Addons Controller(s) to discover CSI-Addons side-cars that are running alongside of CSI-driver components. The information provided in a CSIAddonsNode CR contain details on how to connect to the CSI-Addons side-car.

```yaml
apiVersion: csiaddons.openshift.io/v1beta1
kind: CSIAddonsNode
metadata:
  name: csiaddonsnode-sample
spec:
    driver:
        name: example.csi.ceph.com
        endpoint: <url>
        nodeID: node-1
```
+ `driver` contains the required information about the CSI driver.
    + `name` contains the name of the driver. The name of the driver is in the format: `rbd/cephfs.csi.ceph.com`
    + `endpoint` contains the url that contains the ip-address to which the CSI-Addons side-car listens to.
    + `nodeID` contains the ID of node to identify on which node the side-car is running.
