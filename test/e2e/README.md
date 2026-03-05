# End-to-End Tests for CSI-Addons Controller

This directory contains end-to-end tests for the CSI-Addons controller functionality. These tests are designed to validate the complete workflow of CSI-Addons features in a real Kubernetes environment.

## Overview

The end-to-end tests verify the following CSI-Addons features:

- **ReclaimSpace**: Volume space reclamation operations
- **EncryptionKeyRotation**: Encryption key rotation for volumes
- **NetworkFence**: Network fencing operations
- **VolumeReplication**: Volume replication (Primary/Secondary/Resync)
- **VolumeGroupReplication**: Volume group replication operations

## Prerequisites

Before running the end-to-end tests, ensure you have:

1. A running Kubernetes cluster (v1.31+)
2. CSI-Addons controller deployed in the cluster
3. A CSI driver with CSI-Addons sidecar deployed
4. `kubectl` configured to access the cluster
5. Go 1.25+ installed

## Test Structure

```console
test/e2e/
├── README.md                          # This file
├── suite_test.go                      # Test suite setup
├── framework/                         # Test framework utilities
│   ├── framework.go                   # Core framework
│   ├── kubernetes.go                  # Kubernetes helpers
│   ├── csiaddons.go                   # CSI-Addons specific helpers
│   └── util.go                        # Utility functions
├── reclaimspace/                      # ReclaimSpace tests
│   └── reclaimspace_test.go
├── encryptionkeyrotation/             # EncryptionKeyRotation tests
│   └── encryptionkeyrotation_test.go
├── networkfence/                      # NetworkFence tests
│   └── networkfence_test.go
├── volumereplication/                 # VolumeReplication tests
│   └── volumereplication_test.go
└── volumegroupreplication/            # VolumeGroupReplication tests
    └── volumegroupreplication_test.go
```

## Running Tests

### Run All end-to-end Tests

```bash
make test-e2e
```

### Run Specific Feature Tests

```bash
# Run only ReclaimSpace tests
go test -v ./test/e2e/reclaimspace -timeout 30m

# Run only VolumeReplication tests
go test -v ./test/e2e/volumereplication -timeout 30m

# Run only EncryptionKeyRotation tests
go test -v ./test/e2e/encryptionkeyrotation -timeout 30m
```

### Run with Custom Kubeconfig

```bash
KUBECONFIG=/path/to/kubeconfig make test-e2e
```

### Run with Verbose Output

```bash
go test -v ./test/e2e/... -ginkgo.v -timeout 60m
```

## Environment Variables

The following environment variables can be used to configure the tests:

- `KUBECONFIG`: Path to kubeconfig file (default: `~/.kube/config`)
- `E2E_TIMEOUT`: Global timeout for tests (default: `30m`)
- `E2E_NAMESPACE`: Namespace for test resources (default: `csi-addons-e2e`)
- `CSI_DRIVER_NAME`: Name of the CSI driver to test (default: auto-detected)
- `STORAGE_CLASS`: Storage class to use for PVCs (default: auto-detected)

Example:

```bash
E2E_NAMESPACE=my-test-ns CSI_DRIVER_NAME=my-csi-driver make test-e2e
```

## Writing New Tests

To add tests for a new CSI-Addons feature:

1. Create a new directory under `test/e2e/` for your feature
2. Create a test file following the Ginkgo/Gomega pattern
3. Use the framework utilities from `test/e2e/framework/`
4. Follow the existing test patterns for consistency

Example test structure:

```go
package myfeature_test

import (
    "context"
    "testing"
    "time"

    ginkgo "github.com/onsi/ginkgo/v2"
    gomega "github.com/onsi/gomega"

    "github.com/csi-addons/kubernetes-csi-addons/test/e2e/framework"
)

func TestMyFeature(t *testing.T) {
    gomega.RegisterFailHandler(inkgo.Fail)
    ginkgo.RunSpecs(t, "MyFeature E2E Suite")
}

var _ = ginkgo.Describe("MyFeature", func() {
    var (
        f *framework.Framework
    )

    ginkgo.BeforeEach(func() {
        f = framework.NewFramework("myfeature-e2e")
    })

    ginkgo.AfterEach(func() {
        f.Cleanup()
    })

    ginkgo.It("should perform feature operation successfully", func() {
        // Test implementation
    })
})
```

## Test Framework Features

The framework provides:

- **Automatic cleanup**: Resources are cleaned up after each test
- **Kubernetes helpers**: Easy creation and management of K8s resources
- **CSI-Addons helpers**: Utilities for CSI-Addons specific operations
- **Retry logic**: Built-in retry mechanisms for flaky operations
- **Logging**: Structured logging for debugging
- **Resource tracking**: Automatic tracking of created resources

## Debugging Failed Tests

When tests fail:

1. Check the test output for error messages
2. Inspect the resources in the test namespace:

   ```bash
   kubectl get all -n csi-addons-e2e
   kubectl describe <resource> -n csi-addons-e2e
   ```

3. Check controller logs:

   ```bash
   kubectl logs -n csi-addons-system deployment/csi-addons-controller-manager
   ```

4. Check sidecar logs:

   ```bash
   kubectl logs -n <driver-namespace> <csi-driver-pod> -c csi-addons
   ```

## Contributing

When contributing new tests:

1. Ensure tests are idempotent and can run in parallel
2. Use descriptive test names
3. Add appropriate timeouts
4. Clean up resources properly
5. Update this readme with any new features or requirements

## Troubleshooting

### Common Issues

**Issue**: Tests timeout

- **Solution**: Increase timeout with `-timeout` flag or check if resources are stuck

**Issue**: Permission denied errors

- **Solution**: Ensure RBAC is properly configured for the test service account

## References

- [CSI-Addons Specification](https://github.com/csi-addons/spec)
- [Ginkgo Testing Framework](https://onsi.github.io/ginkgo/)
- [Gomega Matcher Library](https://onsi.github.io/gomega/)
