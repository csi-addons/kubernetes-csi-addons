# CSI-Addons Operator Configuration

CSI-Addons Operator can consume configuration from a ConfigMap named `csi-addons-config`
in the same namespace as the operator. This enables configuration of the operator to persist across
upgrades. The ConfigMap can support the following configuration options:

| Option                        | Default value | Description                                                                                         |
| ----------------------------- | ------------- | --------------------------------------------------------------------------------------------------- |
| `reclaim-space-timeout`       | `"3m"`        | Timeout for reclaimspace operation                                                                  |
| `max-concurrent-reconciles`   | `"100"`       | Maximum number of concurrent reconciles                                                             |
| `max-group-pvcs`              | `"100"`       | Maximum number of PVCs allowed in a volume group                                                    |
| `csi-addons-node-retry-delay` | `"5"`         | Duration, in seconds, that csi-addons reconcile must wait before retrying connection to the sidecar |
| `schedule-precedence`         | `"pvc"`       | The order in which the schedule annotation should be read                                           |

[`csi-addons-config` ConfigMap](../deploy/controller/csi-addons-config.yaml) is provided as an example.

> Note: The operator pod needs to be restarted for any change in configuration to take effect.
>
> Note: `max-group-pvcs` default value is set based on ceph's support/testing. User can tweak this value based on the supported count for their storage vendor.
>
> Note: The valid values for `schedule-precedence` are `storageclass` and `pvc`. If set to `storageclass` only the annotations from StorageClasses are considered for schedule of reclaim space and key rotation. `pvc` reads annotations in the order of: PVC > NS > StorageClasses.
