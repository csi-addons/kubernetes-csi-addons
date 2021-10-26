# CSI-Addons for Kubernetes

This repository contains the implementation for the [CSI-Addons][csi_addons]
specification that can be used with Kubernetes.

The [CSI API][csi] is tightly integrated with Kubernetes. In order to extend
the interface, a new CSI-Addons Controller is needed. The CSI-Addons Controller
will watch for Kubernetes events (CRs) and relay operation initiated by the
user to the CSI-driver.

```
.------.   CR  .------------.
| User |-------| CSI-Addons |
'------'       | Controller |
               '------------'
                      |
                      | gRPC
                      |
            .---------+------------------------------.
            |         |                              |
            |  .------------.        .------------.  |
            |  | CSI-Addons |  gRPC  |    CSI     |  |
            |  |  side-car  |--------| Controller |  |
            |  '------------'        | NodePlugin |  |
            |                        '------------'  |
            | CSI-driver Pod                         |
            '----------------------------------------'
```

A CSI-Addons side-car will be running in the CSI-driver (provisioner and
node-plugin) Pods. The side-car calls gRPC procedures for CSI-Addons
operations.

## CSI-driver side-car

The CSI-driver side-car is located with the CSI-Controller (provisioner) and
the CSI-nodeplugin containers. The side-car registers itself by creating a
`CSIAddonsNode` CR that the CSI-Addons Controller can use to connect to the
side-car and execute operations.

## Controller

The CSI-Addons Controller handles the requests from users to initiate an
operation. Users create a CR that the controller inspects, and forwards a
request to one or more CSI-Addons side-cars for execution.

By listing the `CSIAddonsNode` CRs, the CSI-Addons Controller knows how to
connect to the side-cars. By checking the supported capabilities of the
side-cars, it can decide where to execute operations that the user requested.

[csi_addons]: https://github.com/csi-addons/spec/
[csi]: https://kubernetes-csi.github.io/docs/
