# NetworkFence

NetworkFence is a cluster-scoped custom resource that allows Kubernetes to invoke "Network fence" operation on a storage provider.

The user needs to specify the list of CIDR blocks on which network fencing operation will be performed along with either of
`networkFenceClassName` or the csi driver name. When a `networkFenceClassName` is specified, the secret name, namespace
and parameters are read from the `NetworkFenceClass`.

When both `networkFenceClassName` and `driver` are specified `networkFenceClassName` has the higher precedence.

> **Note:** Specifying `driver`, `secret` and `parameters` inside `NetworkFence` is deprecated, users are encouraged
> to use `networkFenceClassName` along with a `NetworkFenceClass` instead.

The creation of NetworkFence CR will add a network fence, and its deletion will undo the operation.

## Fence Operation

```yaml
apiVersion: csiaddons.openshift.io/v1alpha1
kind: NetworkFence
metadata:
  name: network-fence-sample
spec:
  networkFenceClassName: network-fence-class
  cidrs:
    - 10.90.89.66/32
    - 11.67.12.42/24
  # The fields driver, secret and parameters are deprecated.
  # It is recommended to use networkFenceClassName to specify these.
  # Note: `driver` is referred to as the `provisioner` in NetworkFenceClass.
  driver: example.driver
  secret:
    name: fence-secret
    namespace: default
  parameters:
    key: value
```

> **Note**: Creation of a NetworkFence CR blocks access to the corresponding CIDR block; which is then unblocked the CR deletion.

- `networkFenceClassName`: specifies the name of the NetworkFenceClass.
- `driver`: specifies the name of storage provisioner.
- `cidrs`: refers to the CIDR blocks on which the mentioned fence/unfence operation is to be performed.
- `secret`: refers to the kubernetes secret required for network fencing operation.
  - `name`: specifies the name of the secret
  - `namespace`: specifies the namespace in which the secret is located.
- `parameters`: specifies storage provider specific parameters.
