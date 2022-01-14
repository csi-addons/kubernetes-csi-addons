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
```


+ `target` represents volume target on which the operation will be performed.
    + `persistentVolumeClaim` contains a string indicating the name of `PersistentVolumeClaim`.
+ `backOfflimit` specifies the number of retries before marking reclaim space operation as failed. If not specified, defaults to 6. Maximum allowed value is 60 and minimum allowed value is 0.
+ `retryDeadlineSeconds` specifies the duration in seconds relative to the start time that the operation may be retried; value must be positive integer. If not specified, defaults to 600 seconds. Maximum allowed value is 1800.

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
  schedule: '@weekly'
  successfulJobsHistoryLimit: 3
```

+ `concurrencyPolicy` describes what happens when a new `ReclaimSpaceJob` is
  scheduled by the `ReclaimSpaceCronJob`, while a previous `ReclaimSpaceJob` is
  still running. The default `Forbid` prevents starting new job, whereas
  `Replace` can be used to delete the running job (potentially in a failure
  state) and create a new one.
+ `failedJobsHistoryLimit` keeps at most the number of failed
  `ReclaimSpaceJobs` around for troubleshooting
+ `jobTemplate` contains the `ReclaimSpaceJob.spec` structure, which describes
  the details of the requested `ReclaimSpaceJob` operation.
+ `schedule` is in the same [format as Kubernetes CronJobs][batch_cronjob] that
  sets the and/or interval of the recurring operation request.
+ `successfulJobsHistoryLimit` can be used to keep at most number of successful
  `ReclaimSpaceJob` operations.

## Annotating PerstentVolumeClaims

An extension to the controller to automatically create `ReclaimSpaceCronJob`
based on annotation on PersistentVolumeClaim is being planned.

[batch_cronjob]: https://kubernetes.io/docs/concepts/workloads/controllers/cron-jobs/
[go_cron]: https://pkg.go.dev/github.com/robfig/cron/v3#hdr-CRON_Expression_Format
