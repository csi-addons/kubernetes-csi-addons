# `csi-addons` admin and testing tool

The `csi-addons` executable connects to a socket where the CSI-driver listens
for CSI-Addons requests. The tool can be executed in the side-car that is
running alongside CSI-driver containers.

When running the tool with the `-h` parameter, it displays the help text,
similar to:

```console
$ kubectl exec -c csi-addons csi-backend-nodeplugin -- csi-addons -h
  -drivername string
    	name of the CSI driver
  -endpoint string
    	CSI-Addons endpoint (default "unix:///tmp/csi-addons.sock")
  -legacy
    	use legacy format for old Kubernetes versions
  -operation string
    	csi-addons operation
  -persistentvolume string
    	name of the PersistentVolume
  -stagingpath string
    	staging path (default "/var/lib/kubelet/plugins/kubernetes.io/csi/")

The following operations are supported:
 - NodeReclaimSpace
 - GetIdentity
 - GetCapabilities
 - Probe
 - ControllerReclaimSpace
```

The above command assumes the running `csi-backend-nodeplugin` Pod has the
`quay.io/csiaddons/k8s-sidecar` as a container named `csi-addons`. Executing
the `csi-addons -h` command then shows the help text.
