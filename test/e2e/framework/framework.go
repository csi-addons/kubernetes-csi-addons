/*
Copyright 2024 The Kubernetes-CSI-Addons Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package framework

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/csiaddons/v1alpha1"
	replicationv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/replication.storage/v1alpha1"
	"github.com/csi-addons/kubernetes-csi-addons/test/e2e/config"
)

// Framework provides a collection of helpful functions for e2e testing
type Framework struct {
	// BaseName is the base name for the test
	BaseName string

	// Config is the test configuration
	Config *config.E2EConfig

	// Namespace is the namespace for the test
	Namespace *corev1.Namespace

	// ClientSet is the Kubernetes clientset
	ClientSet kubernetes.Interface

	// Client is the controller-runtime client
	Client client.Client

	// RestConfig is the rest config
	RestConfig *rest.Config

	// Scheme is the runtime scheme
	Scheme *runtime.Scheme

	// CreatedResources tracks resources created during the test
	CreatedResources []client.Object

	// namespaceName is the actual namespace name being used
	namespaceName string
}

// NewFramework creates a new test framework
func NewFramework(baseName string) *Framework {
	f := &Framework{
		BaseName: baseName,
		Config:   config.TestConfig,
	}

	// Initialize Kubernetes clients
	f.initializeClients()

	// Setup test namespace
	f.setupNamespace()

	return f
}

// initializeClients initializes the Kubernetes clients
func (f *Framework) initializeClients() {
	By("Initializing Kubernetes clients")

	// Build config
	cfg, err := clientcmd.BuildConfigFromFlags("", f.Config.Kubeconfig)
	Expect(err).NotTo(HaveOccurred(), "Failed to build kubeconfig")
	f.RestConfig = cfg

	// Create clientset
	clientset, err := kubernetes.NewForConfig(cfg)
	Expect(err).NotTo(HaveOccurred(), "Failed to create clientset")
	f.ClientSet = clientset

	// Create scheme
	f.Scheme = runtime.NewScheme()
	Expect(scheme.AddToScheme(f.Scheme)).To(Succeed())
	Expect(csiaddonsv1alpha1.AddToScheme(f.Scheme)).To(Succeed())
	Expect(replicationv1alpha1.AddToScheme(f.Scheme)).To(Succeed())

	// Create controller-runtime client
	c, err := client.New(cfg, client.Options{Scheme: f.Scheme})
	Expect(err).NotTo(HaveOccurred(), "Failed to create controller-runtime client")
	f.Client = c
}

// setupNamespace sets up the test namespace
func (f *Framework) setupNamespace() {
	By(fmt.Sprintf("Setting up test namespace for %s", f.BaseName))

	// Use configured namespace name
	f.namespaceName = f.Config.Namespace

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: f.namespaceName,
			Labels: map[string]string{
				"e2e-test":      "true",
				"e2e-framework": f.BaseName,
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), f.Config.Timeouts.OperationComplete)
	defer cancel()

	// Try to get existing namespace
	existingNs := &corev1.Namespace{}
	err := f.Client.Get(ctx, client.ObjectKey{Name: f.namespaceName}, existingNs)
	if err == nil {
		// Namespace exists, use it
		f.Namespace = existingNs
		By(fmt.Sprintf("Using existing namespace: %s", f.namespaceName))
		return
	}

	if !apierrors.IsNotFound(err) {
		Expect(err).NotTo(HaveOccurred(), "Failed to check for existing namespace")
	}

	// Create new namespace
	err = f.Client.Create(ctx, ns)
	Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")
	f.Namespace = ns
	By(fmt.Sprintf("Created namespace: %s", f.namespaceName))
}

// Cleanup cleans up resources created during the test
func (f *Framework) Cleanup() {
	By(fmt.Sprintf("Cleaning up resources for %s", f.BaseName))

	ctx, cancel := context.WithTimeout(context.Background(), f.Config.Timeout)
	defer cancel()

	// Delete created resources in reverse order
	for i := len(f.CreatedResources) - 1; i >= 0; i-- {
		resource := f.CreatedResources[i]
		kind := resource.GetObjectKind().GroupVersionKind().Kind
		if kind == "" {
			kind = fmt.Sprintf("%T", resource)
		}
		By(fmt.Sprintf("Deleting %s/%s", kind, resource.GetName()))
		err := f.Client.Delete(ctx, resource)
		if err != nil && !apierrors.IsNotFound(err) {
			By(fmt.Sprintf("Warning: Failed to delete resource: %v", err))
		}
	}

	// Delete namespace if configured
	if f.Namespace != nil && f.Config.DeleteNamespace {
		By(fmt.Sprintf("Deleting namespace %s", f.Namespace.Name))
		err := f.Client.Delete(ctx, f.Namespace)
		if err != nil && !apierrors.IsNotFound(err) {
			By(fmt.Sprintf("Warning: Failed to delete namespace: %v", err))
		}
	}
}

// CleanupOnFailure cleans up resources on test failure
func (f *Framework) CleanupOnFailure() {
	if f.Config.DeleteNamespaceOnFailure {
		f.Cleanup()
	} else {
		By(fmt.Sprintf("Skipping cleanup on failure, namespace %s preserved for debugging", f.namespaceName))
	}
}

// TrackResource adds a resource to the list of created resources for cleanup
func (f *Framework) TrackResource(obj client.Object) {
	f.CreatedResources = append(f.CreatedResources, obj)
}

// GetNamespaceName returns the namespace name
func (f *Framework) GetNamespaceName() string {
	return f.namespaceName
}

// GetTimeout returns the timeout for a specific operation
func (f *Framework) GetTimeout(operation string) time.Duration {
	switch operation {
	case "pvc-create":
		return f.Config.Timeouts.PVCCreate
	case "pvc-bound":
		return f.Config.Timeouts.PVCBound
	case "pod-start":
		return f.Config.Timeouts.PodStart
	case "pod-delete":
		return f.Config.Timeouts.PodDelete
	case "job-complete":
		return f.Config.Timeouts.JobComplete
	case "replication-sync":
		return f.Config.Timeouts.ReplicationSync
	default:
		return f.Config.Timeouts.OperationComplete
	}
}

// GetCSIDriverName returns the CSI driver name from config
func (f *Framework) GetCSIDriverName() string {
	return f.Config.CSIDriver.Name
}

// GetStorageClassName returns the storage class name from config
func (f *Framework) GetStorageClassName() string {
	return f.Config.Storage.StorageClassName
}

// GetVolumeSize returns the default volume size from config
func (f *Framework) GetVolumeSize() string {
	return f.Config.Storage.VolumeSize
}

// GetAccessMode returns the default access mode from config
func (f *Framework) GetAccessMode() corev1.PersistentVolumeAccessMode {
	return corev1.PersistentVolumeAccessMode(f.Config.Storage.AccessMode)
}

// GetNetworkFenceCIDRs returns the CIDRs for NetworkFence tests from config
func (f *Framework) GetNetworkFenceCIDRs() []string {
	return f.Config.NetworkFence.Cidrs
}

// GetNetworkFenceProvisioner returns the provisioner for NetworkFenceClass from config
func (f *Framework) GetNetworkFenceProvisioner() string {
	if f.Config.NetworkFence.Provisioner != "" {
		return f.Config.NetworkFence.Provisioner
	}
	// Fallback to CSI driver name if provisioner not specified
	return f.GetCSIDriverName()
}

// GetNetworkFenceParameters returns the parameters for NetworkFenceClass from config
func (f *Framework) GetNetworkFenceParameters() map[string]string {
	return f.Config.NetworkFence.Parameters
}

// GetVolumeReplicationProvisioner returns the provisioner for VolumeReplicationClass from config
func (f *Framework) GetVolumeReplicationProvisioner() string {
	if f.Config.VolumeReplication.Provisioner != "" {
		return f.Config.VolumeReplication.Provisioner
	}
	// Fallback to CSI driver name if provisioner not specified
	return f.GetCSIDriverName()
}

// GetVolumeReplicationParameters returns the parameters for VolumeReplicationClass from config
func (f *Framework) GetVolumeReplicationParameters() map[string]string {
	return f.Config.VolumeReplication.Parameters
}

// GetVolumeGroupReplicationProvisioner returns the provisioner for VolumeGroupReplicationClass from config
func (f *Framework) GetVolumeGroupReplicationProvisioner() string {
	if f.Config.VolumeGroupReplication.Provisioner != "" {
		return f.Config.VolumeGroupReplication.Provisioner
	}
	// Fallback to CSI driver name if provisioner not specified
	return f.GetCSIDriverName()
}

// GetVolumeGroupReplicationParameters returns the parameters for VolumeGroupReplicationClass from config
func (f *Framework) GetVolumeGroupReplicationParameters() map[string]string {
	return f.Config.VolumeGroupReplication.Parameters
}
