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

### Installation

This project uses `kustomize` and a `Makefile` for deploying. By running the
command

```console
$ make deploy
...
customresourcedefinition.apiextensions.k8s.io/csiaddonsnodes.csiaddons.openshift.io created
customresourcedefinition.apiextensions.k8s.io/reclaimspacejobs.csiaddons.openshift.io created
serviceaccount/csi-addons-controller-manager created
role.rbac.authorization.k8s.io/csi-addons-leader-election-role created
clusterrole.rbac.authorization.k8s.io/csi-addons-manager-role created
clusterrole.rbac.authorization.k8s.io/csi-addons-metrics-reader created
clusterrole.rbac.authorization.k8s.io/csi-addons-proxy-role created
rolebinding.rbac.authorization.k8s.io/csi-addons-leader-election-rolebinding created
clusterrolebinding.rbac.authorization.k8s.io/csi-addons-manager-rolebinding created
clusterrolebinding.rbac.authorization.k8s.io/csi-addons-proxy-rolebinding created
configmap/csi-addons-manager-config created
service/csi-addons-controller-manager-metrics-service created
deployment.apps/csi-addons-controller-manager created
```

the different components for the Controller will get deployed in the
`csi-addons-system` Namespace.

```console
$ kubectl -n csi-addons-system get all
NAME                                                 READY   STATUS    RESTARTS   AGE
pod/csi-addons-controller-manager-687d47b8c7-9m56f   2/2     Running   0          49s

NAME                                                    TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)    AGE
service/csi-addons-controller-manager-metrics-service   ClusterIP   172.30.153.17   <none>        8443/TCP   49s

NAME                                            READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/csi-addons-controller-manager   1/1     1            1           49s

NAME                                                       DESIRED   CURRENT   READY   AGE
replicaset.apps/csi-addons-controller-manager-687d47b8c7   1         1         1       49s
```

[csi_addons]: https://github.com/csi-addons/spec/
[csi]: https://kubernetes-csi.github.io/docs/
