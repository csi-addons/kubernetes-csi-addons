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

### `csi-addons` executable

The `csi-addons` executable can be used to call CSI-Addons operations against a
CSI-driver. It is included in the side-car container image, so that manual
execution by admins and (automated) testing can easily be done.

See the [`csi-addons` tool documentation](cmd/csi-addons/README.md) for more
details.

## Controller

The CSI-Addons Controller handles the requests from users to initiate an
operation. Users create a CR that the controller inspects, and forwards a
request to one or more CSI-Addons side-cars for execution.

By listing the `CSIAddonsNode` CRs, the CSI-Addons Controller knows how to
connect to the side-cars. By checking the supported capabilities of the
side-cars, it can decide where to execute operations that the user requested.

### Installation by operator

A CSI-Addons bundle can be used to install the CSI-Addons Controller with the
following steps:

```console
$ kubectl create namespace storage-csi-addons
$ make operator-sdk
$ ./bin/operator-sdk run bundle -n storage-csi-addons quay.io/csiaddons/k8s-bundle:latest
```

In the future, the bundle is expected to become available in the
[OperatorHub][operatorhub].

### Installation with `kustomize`

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

### Installation for versioned deployments

The CSI-Addons Controller can also be installed  using the yaml files in `deploy/controller`.
The versioned deployment is possible with the yaml files that get generated for a release,
like [release-v0.3.0](https://github.com/csi-addons/kubernetes-csi-addons/releases/tag/v0.3.0).
You can download the yaml files from there, or use them directly with kubectl.

```console
$ cd deploy/controller

$ kubectl create -f crds.yaml
...
customresourcedefinition.apiextensions.k8s.io/csiaddonsnodes.csiaddons.openshift.io created
customresourcedefinition.apiextensions.k8s.io/networkfences.csiaddons.openshift.io created
customresourcedefinition.apiextensions.k8s.io/reclaimspacecronjobs.csiaddons.openshift.io created
customresourcedefinition.apiextensions.k8s.io/reclaimspacejobs.csiaddons.openshift.io created

$ kubectl create -f rbac.yaml
... 
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

$ kubectl create -f setup-controller.yaml
...
deployment.apps/csi-addons-controller-manager created
```

* The crds.yaml create the required crds for reclaimspace operation.

* The rbac.yaml creates the required rbac.

* The setup-controller creates the csi-addons-controller-manager.

## Contributing

The [Contribution Guidelines](CONTRIBUTING.md) contain details on the process
to contribute to this project.

For feature enhancements, or questions about particular features or design
choices, there is a mailinglist. All regular contributors are encouraged to
subscribe to the list, and participate in the discussions.

Subscribing can be done through the [mailman web interface][mailman] or by
[sending an email to csi-addons-request@redhat.com][subscribe] with subject
`subscribe`.

[csi_addons]: https://github.com/csi-addons/spec/
[csi]: https://kubernetes-csi.github.io/docs/
[operatorhub]: https://operatorhub.io/
[mailman]: https://listman.redhat.com/mailman/listinfo/csi-addons
[subscribe]: mailto:csi-addons-request@redhat.com?subject=subscribe
