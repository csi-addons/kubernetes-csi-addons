# NetworkFenceClass

NetworkFence is a cluster-scoped custom resource that allows Kubernetes to invoke "GetFenceClients" operation on a storage provider.

The user needs to specify the csi provisioner name, parameters and the secret required to perform GetFenceClients operation.

## Fence Operation

```yaml
apiVersion: csiaddons.openshift.io/v1alpha1
kind: NetworkFenceClass
metadata:
  name: network-fence-class
spec:
  provisioner: driver.example.com
  parameters:
    key: value
    csiaddons.openshift.io/networkfence-secret-name: secret-name
    csiaddons.openshift.io/networkfence-secret-namespace: secret-namespace
```

- `provisioner`: specifies the name of storage provisioner.
- `parameters`: specifies storage provider specific parameters.

Resereved parameters:

- `csiaddons.openshift.io/networkfence-secret-name`: specifies the name of the secret required for network fencing operation.
- `csiaddons.openshift.io/networkfence-secret-namespace`: specifies the namespace in which the secret is located.

Once the NetworkFenceClass is processed, the CSI Addons controller will call the GetFenceClients operation on the storage provider associated with the provisioner name that registered the `GET_CLIENTS_TO_FENCE` capability. The resulting data will then be stored in the CSIAddonsNode status.

The NetworkFenceStatus object will contain the list of clients that need to be fenced.

```yaml
apiVersion: csiaddons.openshift.io/v1alpha1
kind: CSIAddonsNode
metadata:
  annotations:
    csiaddons.openshift.io/networkfenceclass-names: '["network-fence-class"]'
  creationTimestamp: "2024-11-11T07:31:20Z"
  finalizers:
  - csiaddons.openshift.io/csiaddonsnode
  generation: 1
  name: plugin
  namespace: default
  ...
status:
  capabilities:
  - service.NODE_SERVICE
  - reclaim_space.ONLINE
  - encryption_key_rotation.ENCRYPTIONKEYROTATION
  - network_fence.GET_CLIENTS_TO_FENCE
  message: Successfully established connection with sidecar
  networkFenceClientStatus:
  - networkFenceClassName: network-fence-class
    clientDetails:
    - cidrs:
      - 10.244.0.1/32
      id: a815fe8e-eabd-4e87-a6e8-78cebfb67d08
  state: Connected
```
