# ReclaimSpaceJob

ReclaimSpaceJob is a namespaced custom resource designed to invoke reclaim space operation on target volume.

```yaml
apiVersion: csiaddons.openshift.io/v1alpha1
kind: ReclaimSpaceJob
metadata:
  name: sample-1
spec:
  target:
    persistentVolumeClaim: pvc-1
  backOffLimit: 10
  retryDeadlineSeconds: 900
  timeout: 600
```

- `target` represents volume target on which the operation will be performed.
  - `persistentVolumeClaim` contains a string indicating the name of `PersistentVolumeClaim`.
- `backOfflimit` specifies the number of retries before marking reclaim space operation as failed. If not specified, defaults to 6. Maximum allowed value is 60 and minimum allowed value is 0.
- `retryDeadlineSeconds` specifies the duration in seconds relative to the start time that the operation may be retried; value must be positive integer. If not specified, defaults to 600 seconds. Maximum allowed value is 1800.
- `timeout` specifies the timeout in seconds for the grpc request sent to the CSI driver. If not specified, defaults to global reclaimspace timeout. Minimum allowed value is 60.

## ReclaimSpaceCronJob

The `ReclaimSpaceCronJob` offers an interface very similar to the [Kubernetes
`batch/CronJob`][batch_cronjob]. With the `schedule` attribute, the CSI-Addons
Controller will create a `ReclaimSpaceJob` at the requested time and interval.
The Kubernetes documentation for the `CronJob` and the [Golang cron
package][go_cron] explain the format of the `schedule` attribute in more
detail.

```yaml
apiVersion: csiaddons.openshift.io/v1alpha1
kind: ReclaimSpaceCronJob
metadata:
  name: reclaimspacecronjob-sample
spec:
  concurrencyPolicy: Forbid
  failedJobsHistoryLimit: 1
  jobTemplate:
    spec:
      backOffLimit: 6
      retryDeadlineSeconds: 600
      target:
        persistentVolumeClaim: data-pvc
  schedule: "@weekly"
  successfulJobsHistoryLimit: 3
```

- `concurrencyPolicy` describes what happens when a new `ReclaimSpaceJob` is
  scheduled by the `ReclaimSpaceCronJob`, while a previous `ReclaimSpaceJob` is
  still running. The default `Forbid` prevents starting new job, whereas
  `Replace` can be used to delete the running job (potentially in a failure
  state) and create a new one.
- `failedJobsHistoryLimit` keeps at most the number of failed
  `ReclaimSpaceJobs` around for troubleshooting
- `jobTemplate` contains the `ReclaimSpaceJob.spec` structure, which describes
  the details of the requested `ReclaimSpaceJob` operation.
- `schedule` is in the same [format as Kubernetes CronJobs][batch_cronjob] that
  sets the and/or interval of the recurring operation request.
- `successfulJobsHistoryLimit` can be used to keep at most number of successful
  `ReclaimSpaceJob` operations.

## Annotating PerstentVolumeClaims

`ReclaimSpaceCronJob` CR can also be automatically created by adding
`reclaimspace.csiaddons.openshift.io/schedule: "@midnight"` annotation
to PersistentVolumeClaim object.

```console
$ kubectl get pvc data-pvc
NAME      STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS      AGE
data-pvc  Bound    pvc-f37b8582-4b04-4676-88dd-e1b95c6abf74   1Gi        RWO            default           20h

$ kubectl annotate pvc data-pvc "reclaimspace.csiaddons.openshift.io/schedule=@midnight"
persistentvolumeclaim/data-pvc annotated

$ kubectl get reclaimspacecronjobs.csiaddons.openshift.io
NAME                    SCHEDULE    SUSPEND   ACTIVE   LASTSCHEDULE   AGE
data-pvc-1642663516   @midnight                                     3s

$ kubectl annotate pvc data-pvc "reclaimspace.csiaddons.openshift.io/schedule=*/1 * * * *" --overwrite=true
persistentvolumeclaim/data-pvc annotated

$ kubectl get reclaimspacecronjobs.csiaddons.openshift.io
NAME                  SCHEDULE    SUSPEND   ACTIVE   LASTSCHEDULE   AGE
data-pvc-1642664617   */1 * * * *                                   3s
```

- Upon adding annotation as shown above, a `ReclaimSpaceCronJob` with name
  `"<pvc-name>-xxxxxxx"` is created (pvc name suffixed with current time hash when
  the job was created).
- `schedule` value is in the same [format as Kubernetes CronJobs][batch_cronjob]
  that sets the and/or interval of the recurring operation request.
- Default schedule value `"@weekly"` is used if `schedule` value is empty or in invalid format.
- `ReclaimSpaceCronJob` is recreated when `schedule` is modified and deleted when
  the annotation is removed.

[batch_cronjob]: https://kubernetes.io/docs/concepts/workloads/controllers/cron-jobs/
[go_cron]: https://pkg.go.dev/github.com/robfig/cron/v3#hdr-CRON_Expression_Format

## Annotating Namespace

You can create `ReclaimSpaceCronJob` CR automatically by adding the
`reclaimspace.csiaddons.openshift.io/schedule: "@midnight"` and (optional)
`reclaimspace.csiaddons.openshift.io/drivers: drivernames` annotations to the
Namespace object. This will only inspect new PersistentVolumeClaims in the
Namespace for ReclaimSpace operations. You must restart the controller if you
want kubernetes-csi-addons to consider existing PersistentVolumeClaims.

`drivernames` can be `,` separated driver names that supports reclaimspace operations.

```console
$ kubectl get namespace default
NAME      STATUS   AGE
default   Active   5d2h

$ kubectl annotate namespace default "reclaimspace.csiaddons.openshift.io/schedule=@midnight"
namespace/default annotated
```

**Note** Please note that the PersistentVolumeClaim annotation takes priority over Namespace
annotation. The kubernetes-csi-addons only generate a `ReclaimSpaceCronJob` if the annotation
exists on the Namespace. If an admin needs to modify or delete the annotation on the Namespace,
they must perform the same action on all the PersistentVolumeClaims within that Namespace.

## Annotating StorageClass

You can create `ReclaimSpaceCronJob` CR automatically by adding the
`reclaimspace.csiaddons.openshift.io/schedule: "@midnight"` annotations to the StorageClass object.

```console
$ kubectl get storageclass rbd-sc
NAME       PROVISIONER        RECLAIMPOLICY   VOLUMEBINDINGMODE   ALLOWVOLUMEEXPANSION   AGE
rbd-sc     rbd.csi.ceph.com   Delete          Immediate           true                   5d2h

$ kubectl annotate storageclass rbd-sc "reclaimspace.csiaddons.openshift.io/schedule=@midnight"
storageclass.storage.k8s.io/rbd-sc annotated
```

**Note** Please note that the PersistentVolumeClaim and Namespace annotation takes priority
over StorageClass annotation. The kubernetes-csi-addons only generate a `ReclaimSpaceCronJob`
if the annotation exists on the StorageClass. If an admin needs to modify or delete the
annotation on the StorageClass, they must perform the same action on all the PersistentVolumeClaims
created from this StorageClass.

## Annotating ReclaimSpaceCronJob

In cases where a certain `ReclaimSpaceCronJob` is to have a different schedule that the one present
in Namespace, StorageClass or PersistentVolumeClaim annotations one could annotate the `ReclaimSpaceCronJob`
itself by adding the `csiaddons.openshift.io/state: "unmanaged"` annotation.

CSI Addons will not perform any further modifications on the `ReclaimSpaceCronJob` with the `unmanaged` state.

To have a custom schedule the user can then modify the `schedule` field of the `ReclaimSpaceCronJob` spec.
