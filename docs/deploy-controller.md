# Deploying the CSI-Addons Controller

The CSI-Addons Controller can be deployed by different ways:

## Configuration

**Available command line arguments:**

| Option                        | Default value   | Description                 |
| ----------------------------- | --------------- | --------------------------------------------- |
| `--metrics-bind-address`      | `:8080`         | The address the metric endpoint binds to.     |
| `--health-probe-bind-address` | `:8081`         | The address the probe endpoint binds to.      |
| `--leader-elect`              | `false`         | Enable leader election for controller manager.|
| `--reclaim-space-timeout`     | `3m`            | Timeout for reclaimspace operation            |
| `--max-concurrent-reconciles` | 100             | Maximum number of concurrent reconciles       |
| `--enable-admission-webhooks` | `true`          | Enable the admission webhooks                 |

## Installation for versioned deployments

The CSI-Addons Controller can also be installed  using the yaml files in `deploy/controller`.
The versioned deployment is possible with the yaml files that get generated for the
[latest release](https://github.com/csi-addons/kubernetes-csi-addons/releases/latest).
You can download the yaml files from there, or use them directly with kubectl.
This is the recommended and easiest way to deploy the controller.

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

* The "crds.yaml" create the required crds for reclaimspace operation.

* The "rbac.yaml" creates the required rbac.

* The "setup-controller.yaml" creates the csi-addons-controller-manager.

## Installation by operator-sdk

A CSI-Addons bundle can be used to install the CSI-Addons Controller with the
following steps:

```console
$ kubectl create namespace storage-csi-addons
$ make operator-sdk
$ ./bin/operator-sdk run bundle -n storage-csi-addons quay.io/csiaddons/k8s-bundle:latest
```

In the future, the bundle is expected to become available in the
[OperatorHub](https://operatorhub.io/).

## Installation with `kustomize`

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
