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

Further extensions like `ReclaimSpaceCronJob` and controller to automatically create `ReclaimSpaceCronJob` based on annotation on PersistentVolumeClaim are being planned.