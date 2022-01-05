# `csi-addons` admin and testing tool

The `csi-addons` executable connects to a socket where the CSI-driver listens
for CSI-Addons requests. The tool can be executed in the side-car that is
running alongside CSI-driver containers.

When running the tool with the `-h` parameter, it displays the help text,
similar to:

```console
$ kubectl exec -c csi-addons csi-backend-nodeplugin -- csi-addons -h
  -endpoint string
    	CSI-Addons endpoint (default "unix:///tmp/csi-addons.sock")
  -operation string
    	csi-addons operation
  -persistentvolume string
    	name of the PersistentVolume
  -stagingpath string
    	staging path (default "/var/lib/kubelet/plugins/kubernetes.io/csi/pv/")

The following operations are supported:
 - GetIdentity
 - GetCapabilities
 - Probe
 - ControllerReclaimSpace
 - NodeReclaimSpace
```

The above command assumes the running `csi-backend-nodeplugin` Pod has the
`quay.io/csiaddons/k8s-sidecar` as a container named `csi-addons`. Executing
the `csi-addons -h` command then shows the help text.
