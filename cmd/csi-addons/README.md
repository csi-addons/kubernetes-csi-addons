# `csi-addons` admin and testing tool

The `csi-addons` executable connects to a socket where the CSI-driver listens
for CSI-Addons requests. The tool can be executed in the side-car that is
running alongside CSI-driver containers.

When running the tool with the `-h` parameter, it displays the help text,
similar to:

```console
$ kubectl exec -c csi-addons csi-backend-nodeplugin -- csi-addons -h
  -cidrs string
      comma separated list of cidrs to fence/unfence
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
  -secretname string
      name of the kubernetes secret
  -secretnamespace string
      namespace of the kubernetes secret
  -stagingpath string
      staging path (default "/var/lib/kubelet/plugins/kubernetes.io/csi/")
  -version
      print Version details

The following operations are supported:
 - NodeReclaimSpace
 - GetIdentity
 - GetCapabilities
 - Probe
 - NetworkFence
 - NetworkUnFence
 - ControllerReclaimSpace
```

The above command assumes the running `csi-backend-nodeplugin` Pod has the
`quay.io/csiaddons/k8s-sidecar` as a container named `csi-addons`. Executing
the `csi-addons -h` command then shows the help text.
