/*
Copyright 2026 The Kubernetes-CSI-Addons Authors.

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

package networkfence_test

import (
	"context"
	"flag"
	"fmt"
	"testing"
	"time"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/csiaddons/v1alpha1"
	"github.com/csi-addons/kubernetes-csi-addons/test/e2e/config"
	"github.com/csi-addons/kubernetes-csi-addons/test/e2e/framework"
)

func init() {
	// Register configuration flags
	config.RegisterFlags()
}

func TestNetworkFence(t *testing.T) {
	// Parse flags
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load E2E configuration: %v", err)
	}

	// Skip if NetworkFence tests are disabled
	if !cfg.Tests.NetworkFence {
		t.Skip("NetworkFence tests are disabled in configuration")
	}

	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "NetworkFence E2E Suite")
}

var _ = ginkgo.Describe("NetworkFence", ginkgo.Ordered, func() {
	var (
		f *framework.Framework
	)

	ginkgo.BeforeAll(func() {
		f = framework.NewFramework("networkfence-e2e")
	})

	ginkgo.AfterEach(func() {
		// Only cleanup resources created in each test, not the namespace
		if ginkgo.CurrentSpecReport().Failed() {
			f.CleanupOnFailure()
		} else {
			f.Cleanup()
		}
	})

	ginkgo.AfterAll(func() {
		// Cleanup namespace after all tests complete
		if f != nil && f.Namespace != nil && f.Config.DeleteNamespace {
			ginkgo.By(fmt.Sprintf("Deleting namespace %s after all tests", f.Namespace.Name))
			ctx, cancel := context.WithTimeout(context.Background(), f.Config.Timeout)
			defer cancel()
			err := f.Client.Delete(ctx, f.Namespace)
			if err != nil && !apierrors.IsNotFound(err) {
				ginkgo.By(fmt.Sprintf("Warning: Failed to delete namespace: %v", err))
				return
			}

			ginkgo.By(fmt.Sprintf("Waiting for namespace %s to be deleted", f.Namespace.Name))
			err = wait.PollUntilContextTimeout(ctx, 2*time.Second, f.Config.Timeout, true, func(ctx context.Context) (bool, error) {
				ns := &corev1.Namespace{}
				err := f.Client.Get(ctx, client.ObjectKey{Name: f.Namespace.Name}, ns)
				return apierrors.IsNotFound(err), nil
			})
			if err != nil {
				ginkgo.By(fmt.Sprintf("Warning: Namespace deletion timed out: %v", err))
			}
		}
	})

	ginkgo.Context("NetworkFence Operations with Provisioner", func() {
		ginkgo.It("should fence network access with CIDRs from config", func() {
			ginkgo.By("Getting CIDRs from configuration")
			cidrs := f.GetNetworkFenceCIDRs()
			gomega.Expect(cidrs).NotTo(gomega.BeEmpty(), "CIDRs must be configured in e2e-config.yaml")

			ginkgo.By("Creating a NetworkFence with Fenced state")
			nf := f.CreateNetworkFence(
				"test-nf-fenced",
				cidrs,
				csiaddonsv1alpha1.Fenced,
			)

			ginkgo.By("Verifying NetworkFence was created")
			gomega.Expect(nf.Name).To(gomega.Equal("test-nf-fenced"))
			gomega.Expect(nf.Spec.FenceState).To(gomega.Equal(csiaddonsv1alpha1.Fenced))
			gomega.Expect(nf.Spec.Cidrs).To(gomega.Equal(cidrs))
			gomega.Expect(nf.Spec.Driver).NotTo(gomega.BeEmpty())

			ginkgo.By("Waiting for NetworkFence operation to complete")
			nf = f.WaitForNetworkFenceResult(nf.Name, csiaddonsv1alpha1.FencingOperationResultSucceeded)
			gomega.Expect(nf.Status.Result).To(gomega.Equal(csiaddonsv1alpha1.FencingOperationResultSucceeded))
			gomega.Expect(nf.Status.Message).To(gomega.ContainSubstring("fencing operation successful"))
		})

		ginkgo.It("should unfence network access", func() {
			ginkgo.By("Getting CIDRs from configuration")
			cidrs := f.GetNetworkFenceCIDRs()
			gomega.Expect(cidrs).NotTo(gomega.BeEmpty(), "CIDRs must be configured in e2e-config.yaml")

			ginkgo.By("Creating a NetworkFence with Unfenced state")
			nf := f.CreateNetworkFence(
				"test-nf-unfenced",
				cidrs,
				csiaddonsv1alpha1.Unfenced,
			)

			ginkgo.By("Verifying NetworkFence was created")
			gomega.Expect(nf.Name).To(gomega.Equal("test-nf-unfenced"))
			gomega.Expect(nf.Spec.FenceState).To(gomega.Equal(csiaddonsv1alpha1.Unfenced))
			gomega.Expect(nf.Spec.Driver).NotTo(gomega.BeEmpty())

			ginkgo.By("Waiting for NetworkFence operation to complete")
			nf = f.WaitForNetworkFenceResult(nf.Name, csiaddonsv1alpha1.FencingOperationResultSucceeded)
			gomega.Expect(nf.Status.Result).To(gomega.Equal(csiaddonsv1alpha1.FencingOperationResultSucceeded))
			gomega.Expect(nf.Status.Message).To(gomega.ContainSubstring("unfencing operation successful"))
		})

		ginkgo.It("should transition from fenced to unfenced", func() {
			ginkgo.By("Getting CIDRs from configuration")
			cidrs := f.GetNetworkFenceCIDRs()
			gomega.Expect(cidrs).NotTo(gomega.BeEmpty(), "CIDRs must be configured in e2e-config.yaml")

			ginkgo.By("Creating a NetworkFence with Fenced state")
			nf := f.CreateNetworkFence(
				"test-nf-transition",
				cidrs,
				csiaddonsv1alpha1.Fenced,
			)

			ginkgo.By("Waiting for fencing operation to complete")
			nf = f.WaitForNetworkFenceResult(nf.Name, csiaddonsv1alpha1.FencingOperationResultSucceeded)
			gomega.Expect(nf.Status.Result).To(gomega.Equal(csiaddonsv1alpha1.FencingOperationResultSucceeded))
			gomega.Expect(nf.Status.Message).To(gomega.ContainSubstring("fencing operation successful"))

			ginkgo.By("Updating NetworkFence to Unfenced state")
			nf.Spec.FenceState = csiaddonsv1alpha1.Unfenced
			nf = f.UpdateNetworkFence(nf)
			gomega.Expect(nf.Spec.FenceState).To(gomega.Equal(csiaddonsv1alpha1.Unfenced))

			ginkgo.By("Waiting for unfencing operation to complete")
			nf = f.WaitForNetworkFenceMessage(nf.Name, "unfencing operation successful")
			gomega.Expect(nf.Status.Result).To(gomega.Equal(csiaddonsv1alpha1.FencingOperationResultSucceeded))
			gomega.Expect(nf.Status.Message).To(gomega.ContainSubstring("unfencing operation successful"))
		})

		ginkgo.It("should transition from unfenced to fenced", func() {
			ginkgo.By("Getting CIDRs from configuration")
			cidrs := f.GetNetworkFenceCIDRs()
			gomega.Expect(cidrs).NotTo(gomega.BeEmpty(), "CIDRs must be configured in e2e-config.yaml")

			ginkgo.By("Creating a NetworkFence with Unfenced state")
			nf := f.CreateNetworkFence(
				"test-nf-reverse-transition",
				cidrs,
				csiaddonsv1alpha1.Unfenced,
			)

			ginkgo.By("Waiting for unfencing operation to complete")
			nf = f.WaitForNetworkFenceResult(nf.Name, csiaddonsv1alpha1.FencingOperationResultSucceeded)
			gomega.Expect(nf.Status.Result).To(gomega.Equal(csiaddonsv1alpha1.FencingOperationResultSucceeded))
			gomega.Expect(nf.Status.Message).To(gomega.ContainSubstring("unfencing operation successful"))

			ginkgo.By("Updating NetworkFence to Fenced state")
			nf.Spec.FenceState = csiaddonsv1alpha1.Fenced
			nf = f.UpdateNetworkFence(nf)
			gomega.Expect(nf.Spec.FenceState).To(gomega.Equal(csiaddonsv1alpha1.Fenced))

			ginkgo.By("Waiting for fencing operation to complete")
			nf = f.WaitForNetworkFenceMessage(nf.Name, "fencing operation successful")
			gomega.Expect(nf.Status.Result).To(gomega.Equal(csiaddonsv1alpha1.FencingOperationResultSucceeded))
			gomega.Expect(nf.Status.Message).To(gomega.ContainSubstring("fencing operation successful"))
		})

		ginkgo.It("should handle multiple fence/unfence cycles", func() {
			ginkgo.By("Getting CIDRs from configuration")
			cidrs := f.GetNetworkFenceCIDRs()
			gomega.Expect(cidrs).NotTo(gomega.BeEmpty(), "CIDRs must be configured in e2e-config.yaml")

			ginkgo.By("Creating a NetworkFence with Fenced state")
			nf := f.CreateNetworkFence(
				"test-nf-cycles",
				cidrs,
				csiaddonsv1alpha1.Fenced,
			)

			ginkgo.By("Waiting for initial fencing operation to complete")
			nf = f.WaitForNetworkFenceResult(nf.Name, csiaddonsv1alpha1.FencingOperationResultSucceeded)
			gomega.Expect(nf.Status.Result).To(gomega.Equal(csiaddonsv1alpha1.FencingOperationResultSucceeded))

			// Cycle 1: Fence -> Unfence
			ginkgo.By("Cycle 1: Updating to Unfenced state")
			nf.Spec.FenceState = csiaddonsv1alpha1.Unfenced
			nf = f.UpdateNetworkFence(nf)
			nf = f.WaitForNetworkFenceMessage(nf.Name, "unfencing operation successful")
			gomega.Expect(nf.Status.Result).To(gomega.Equal(csiaddonsv1alpha1.FencingOperationResultSucceeded))

			// Cycle 2: Unfence -> Fence
			ginkgo.By("Cycle 2: Updating to Fenced state")
			nf.Spec.FenceState = csiaddonsv1alpha1.Fenced
			nf = f.UpdateNetworkFence(nf)
			nf = f.WaitForNetworkFenceMessage(nf.Name, "fencing operation successful")
			gomega.Expect(nf.Status.Result).To(gomega.Equal(csiaddonsv1alpha1.FencingOperationResultSucceeded))

			// Cycle 3: Fence -> Unfence
			ginkgo.By("Cycle 3: Updating to Unfenced state")
			nf.Spec.FenceState = csiaddonsv1alpha1.Unfenced
			nf = f.UpdateNetworkFence(nf)
			nf = f.WaitForNetworkFenceMessage(nf.Name, "unfencing operation successful")
			gomega.Expect(nf.Status.Result).To(gomega.Equal(csiaddonsv1alpha1.FencingOperationResultSucceeded))
		})
	})

	ginkgo.Context("NetworkFence Operations with NetworkFenceClass", func() {
		ginkgo.It("should fence network access using NetworkFenceClass", func() {
			ginkgo.By("Getting NetworkFenceClass configuration")
			provisioner := f.GetNetworkFenceProvisioner()
			gomega.Expect(provisioner).NotTo(gomega.BeEmpty(), "Provisioner must be configured")
			parameters := f.GetNetworkFenceParameters()
			cidrs := f.GetNetworkFenceCIDRs()
			gomega.Expect(cidrs).NotTo(gomega.BeEmpty(), "CIDRs must be configured in e2e-config.yaml")

			ginkgo.By("Creating a NetworkFenceClass")
			nfc := f.CreateNetworkFenceClass(
				"test-nf-class-fenced",
				provisioner,
				parameters,
			)
			gomega.Expect(nfc.Name).To(gomega.Equal("test-nf-class-fenced"))
			gomega.Expect(nfc.Spec.Provisioner).To(gomega.Equal(provisioner))

			ginkgo.By("Creating a NetworkFence with Fenced state using NetworkFenceClass")
			nf := f.CreateNetworkFenceWithClass(
				"test-nf-class-fenced",
				cidrs,
				csiaddonsv1alpha1.Fenced,
				nfc.Name,
			)

			ginkgo.By("Verifying NetworkFence was created with class")
			gomega.Expect(nf.Name).To(gomega.Equal("test-nf-class-fenced"))
			gomega.Expect(nf.Spec.NetworkFenceClassName).To(gomega.Equal(nfc.Name))
			gomega.Expect(nf.Spec.Cidrs).To(gomega.Equal(cidrs))
			gomega.Expect(nf.Spec.FenceState).To(gomega.Equal(csiaddonsv1alpha1.Fenced))

			ginkgo.By("Waiting for NetworkFence operation to complete")
			nf = f.WaitForNetworkFenceResult(nf.Name, csiaddonsv1alpha1.FencingOperationResultSucceeded)
			gomega.Expect(nf.Status.Result).To(gomega.Equal(csiaddonsv1alpha1.FencingOperationResultSucceeded))
			gomega.Expect(nf.Status.Message).To(gomega.ContainSubstring("fencing operation successful"))
		})

		ginkgo.It("should unfence network access using NetworkFenceClass", func() {
			ginkgo.By("Getting NetworkFenceClass configuration")
			provisioner := f.GetNetworkFenceProvisioner()
			gomega.Expect(provisioner).NotTo(gomega.BeEmpty(), "Provisioner must be configured")
			parameters := f.GetNetworkFenceParameters()
			cidrs := f.GetNetworkFenceCIDRs()
			gomega.Expect(cidrs).NotTo(gomega.BeEmpty(), "CIDRs must be configured in e2e-config.yaml")

			ginkgo.By("Creating a NetworkFenceClass")
			nfc := f.CreateNetworkFenceClass(
				"test-nf-class-unfenced",
				provisioner,
				parameters,
			)

			ginkgo.By("Creating a NetworkFence with Unfenced state using NetworkFenceClass")
			nf := f.CreateNetworkFenceWithClass(
				"test-nf-class-unfenced",
				cidrs,
				csiaddonsv1alpha1.Unfenced,
				nfc.Name,
			)

			ginkgo.By("Verifying NetworkFence was created with class")
			gomega.Expect(nf.Name).To(gomega.Equal("test-nf-class-unfenced"))
			gomega.Expect(nf.Spec.NetworkFenceClassName).To(gomega.Equal(nfc.Name))
			gomega.Expect(nf.Spec.FenceState).To(gomega.Equal(csiaddonsv1alpha1.Unfenced))

			ginkgo.By("Waiting for NetworkFence operation to complete")
			nf = f.WaitForNetworkFenceResult(nf.Name, csiaddonsv1alpha1.FencingOperationResultSucceeded)
			gomega.Expect(nf.Status.Result).To(gomega.Equal(csiaddonsv1alpha1.FencingOperationResultSucceeded))
			gomega.Expect(nf.Status.Message).To(gomega.ContainSubstring("unfencing operation successful"))
		})

		ginkgo.It("should transition from fenced to unfenced using NetworkFenceClass", func() {
			ginkgo.By("Getting NetworkFenceClass configuration")
			provisioner := f.GetNetworkFenceProvisioner()
			gomega.Expect(provisioner).NotTo(gomega.BeEmpty(), "Provisioner must be configured")
			parameters := f.GetNetworkFenceParameters()
			cidrs := f.GetNetworkFenceCIDRs()
			gomega.Expect(cidrs).NotTo(gomega.BeEmpty(), "CIDRs must be configured in e2e-config.yaml")

			ginkgo.By("Creating a NetworkFenceClass")
			nfc := f.CreateNetworkFenceClass(
				"test-nf-class-transition",
				provisioner,
				parameters,
			)

			ginkgo.By("Creating a NetworkFence with Fenced state using NetworkFenceClass")
			nf := f.CreateNetworkFenceWithClass(
				"test-nf-class-transition",
				cidrs,
				csiaddonsv1alpha1.Fenced,
				nfc.Name,
			)

			ginkgo.By("Waiting for fencing operation to complete")
			nf = f.WaitForNetworkFenceResult(nf.Name, csiaddonsv1alpha1.FencingOperationResultSucceeded)
			gomega.Expect(nf.Status.Result).To(gomega.Equal(csiaddonsv1alpha1.FencingOperationResultSucceeded))
			gomega.Expect(nf.Status.Message).To(gomega.ContainSubstring("fencing operation successful"))

			ginkgo.By("Updating NetworkFence to Unfenced state")
			nf.Spec.FenceState = csiaddonsv1alpha1.Unfenced
			nf = f.UpdateNetworkFence(nf)
			gomega.Expect(nf.Spec.FenceState).To(gomega.Equal(csiaddonsv1alpha1.Unfenced))

			ginkgo.By("Waiting for unfencing operation to complete")
			nf = f.WaitForNetworkFenceMessage(nf.Name, "unfencing operation successful")
			gomega.Expect(nf.Status.Result).To(gomega.Equal(csiaddonsv1alpha1.FencingOperationResultSucceeded))
			gomega.Expect(nf.Status.Message).To(gomega.ContainSubstring("unfencing operation successful"))
		})

		ginkgo.It("should transition from unfenced to fenced using NetworkFenceClass", func() {
			ginkgo.By("Getting NetworkFenceClass configuration")
			provisioner := f.GetNetworkFenceProvisioner()
			gomega.Expect(provisioner).NotTo(gomega.BeEmpty(), "Provisioner must be configured")
			parameters := f.GetNetworkFenceParameters()
			cidrs := f.GetNetworkFenceCIDRs()
			gomega.Expect(cidrs).NotTo(gomega.BeEmpty(), "CIDRs must be configured in e2e-config.yaml")

			ginkgo.By("Creating a NetworkFenceClass")
			nfc := f.CreateNetworkFenceClass(
				"test-nf-class-reverse-transition",
				provisioner,
				parameters,
			)

			ginkgo.By("Creating a NetworkFence with Unfenced state using NetworkFenceClass")
			nf := f.CreateNetworkFenceWithClass(
				"test-nf-class-reverse-transition",
				cidrs,
				csiaddonsv1alpha1.Unfenced,
				nfc.Name,
			)

			ginkgo.By("Waiting for unfencing operation to complete")
			nf = f.WaitForNetworkFenceResult(nf.Name, csiaddonsv1alpha1.FencingOperationResultSucceeded)
			gomega.Expect(nf.Status.Result).To(gomega.Equal(csiaddonsv1alpha1.FencingOperationResultSucceeded))
			gomega.Expect(nf.Status.Message).To(gomega.ContainSubstring("unfencing operation successful"))

			ginkgo.By("Updating NetworkFence to Fenced state")
			nf.Spec.FenceState = csiaddonsv1alpha1.Fenced
			nf = f.UpdateNetworkFence(nf)
			gomega.Expect(nf.Spec.FenceState).To(gomega.Equal(csiaddonsv1alpha1.Fenced))

			ginkgo.By("Waiting for fencing operation to complete")
			nf = f.WaitForNetworkFenceMessage(nf.Name, "fencing operation successful")
			gomega.Expect(nf.Status.Result).To(gomega.Equal(csiaddonsv1alpha1.FencingOperationResultSucceeded))
			gomega.Expect(nf.Status.Message).To(gomega.ContainSubstring("fencing operation successful"))
		})

		ginkgo.It("should handle multiple fence/unfence cycles using NetworkFenceClass", func() {
			ginkgo.By("Getting NetworkFenceClass configuration")
			provisioner := f.GetNetworkFenceProvisioner()
			gomega.Expect(provisioner).NotTo(gomega.BeEmpty(), "Provisioner must be configured")
			parameters := f.GetNetworkFenceParameters()
			cidrs := f.GetNetworkFenceCIDRs()
			gomega.Expect(cidrs).NotTo(gomega.BeEmpty(), "CIDRs must be configured in e2e-config.yaml")

			ginkgo.By("Creating a NetworkFenceClass")
			nfc := f.CreateNetworkFenceClass(
				"test-nf-class-cycles",
				provisioner,
				parameters,
			)

			ginkgo.By("Creating a NetworkFence with Fenced state using NetworkFenceClass")
			nf := f.CreateNetworkFenceWithClass(
				"test-nf-class-cycles",
				cidrs,
				csiaddonsv1alpha1.Fenced,
				nfc.Name,
			)

			ginkgo.By("Waiting for initial fencing operation to complete")
			nf = f.WaitForNetworkFenceResult(nf.Name, csiaddonsv1alpha1.FencingOperationResultSucceeded)
			gomega.Expect(nf.Status.Result).To(gomega.Equal(csiaddonsv1alpha1.FencingOperationResultSucceeded))

			// Cycle 1: Fence -> Unfence
			ginkgo.By("Cycle 1: Updating to Unfenced state")
			nf.Spec.FenceState = csiaddonsv1alpha1.Unfenced
			nf = f.UpdateNetworkFence(nf)
			nf = f.WaitForNetworkFenceMessage(nf.Name, "unfencing operation successful")
			gomega.Expect(nf.Status.Result).To(gomega.Equal(csiaddonsv1alpha1.FencingOperationResultSucceeded))

			// Cycle 2: Unfence -> Fence
			ginkgo.By("Cycle 2: Updating to Fenced state")
			nf.Spec.FenceState = csiaddonsv1alpha1.Fenced
			nf = f.UpdateNetworkFence(nf)
			nf = f.WaitForNetworkFenceMessage(nf.Name, "fencing operation successful")
			gomega.Expect(nf.Status.Result).To(gomega.Equal(csiaddonsv1alpha1.FencingOperationResultSucceeded))

			// Cycle 3: Fence -> Unfence
			ginkgo.By("Cycle 3: Updating to Unfenced state")
			nf.Spec.FenceState = csiaddonsv1alpha1.Unfenced
			nf = f.UpdateNetworkFence(nf)
			nf = f.WaitForNetworkFenceMessage(nf.Name, "unfencing operation successful")
			gomega.Expect(nf.Status.Result).To(gomega.Equal(csiaddonsv1alpha1.FencingOperationResultSucceeded))
		})
	})
})
