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

package networkfence_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/csiaddons/v1alpha1"
	"github.com/csi-addons/kubernetes-csi-addons/test/e2e/framework"
)

func TestNetworkFence(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "NetworkFence E2E Suite")
}

var _ = Describe("NetworkFence", func() {
	var (
		f *framework.Framework
	)

	BeforeEach(func() {
		f = framework.NewFramework("networkfence-e2e")
	})

	AfterEach(func() {
		if CurrentSpecReport().Failed() {
			f.CleanupOnFailure()
		} else {
			f.Cleanup()
		}
	})

	Context("NetworkFence Operations", func() {
		It("should fence network access with CIDRs from config", func() {
			By("Getting CIDRs from configuration")
			cidrs := f.GetNetworkFenceCIDRs()
			Expect(cidrs).NotTo(BeEmpty(), "CIDRs must be configured in e2e-config.yaml")

			By("Creating a NetworkFence with Fenced state")
			nf := f.CreateNetworkFence(
				"test-nf-fenced",
				cidrs,
				csiaddonsv1alpha1.Fenced,
			)

			By("Verifying NetworkFence was created")
			Expect(nf.Name).To(Equal("test-nf-fenced"))
			Expect(nf.Spec.FenceState).To(Equal(csiaddonsv1alpha1.Fenced))
			Expect(nf.Spec.Cidrs).To(Equal(cidrs))

			By("Waiting for NetworkFence operation to complete")
			nf = f.WaitForNetworkFenceResult(nf.Name, csiaddonsv1alpha1.FencingOperationResultSucceeded)
			Expect(nf.Status.Result).To(Equal(csiaddonsv1alpha1.FencingOperationResultSucceeded))
			Expect(nf.Status.Message).To(ContainSubstring("fencing operation successful"))
		})

		It("should unfence network access", func() {
			By("Getting CIDRs from configuration")
			cidrs := f.GetNetworkFenceCIDRs()
			Expect(cidrs).NotTo(BeEmpty(), "CIDRs must be configured in e2e-config.yaml")

			By("Creating a NetworkFence with Unfenced state")
			nf := f.CreateNetworkFence(
				"test-nf-unfenced",
				cidrs,
				csiaddonsv1alpha1.Unfenced,
			)

			By("Verifying NetworkFence was created")
			Expect(nf.Name).To(Equal("test-nf-unfenced"))
			Expect(nf.Spec.FenceState).To(Equal(csiaddonsv1alpha1.Unfenced))

			By("Waiting for NetworkFence operation to complete")
			nf = f.WaitForNetworkFenceResult(nf.Name, csiaddonsv1alpha1.FencingOperationResultSucceeded)
			Expect(nf.Status.Result).To(Equal(csiaddonsv1alpha1.FencingOperationResultSucceeded))
			Expect(nf.Status.Message).To(ContainSubstring("unfencing operation successful"))
		})

		It("should handle multiple CIDR blocks from config", func() {
			By("Getting CIDRs from configuration")
			cidrs := f.GetNetworkFenceCIDRs()
			Expect(cidrs).NotTo(BeEmpty(), "CIDRs must be configured in e2e-config.yaml")

			By("Creating a NetworkFence with multiple CIDRs")
			nf := f.CreateNetworkFence(
				"test-nf-multiple-cidrs",
				cidrs,
				csiaddonsv1alpha1.Fenced,
			)

			By("Verifying all CIDRs are configured")
			Expect(nf.Spec.Cidrs).To(HaveLen(len(cidrs)))
			Expect(nf.Spec.Cidrs).To(Equal(cidrs))

			By("Waiting for NetworkFence operation to complete")
			nf = f.WaitForNetworkFenceResult(nf.Name, csiaddonsv1alpha1.FencingOperationResultSucceeded)
			Expect(nf.Status.Result).To(Equal(csiaddonsv1alpha1.FencingOperationResultSucceeded))
		})

		It("should transition from fenced to unfenced", func() {
			By("Getting CIDRs from configuration")
			cidrs := f.GetNetworkFenceCIDRs()
			Expect(cidrs).NotTo(BeEmpty(), "CIDRs must be configured in e2e-config.yaml")

			By("Creating a NetworkFence with Fenced state")
			nf := f.CreateNetworkFence(
				"test-nf-transition",
				cidrs,
				csiaddonsv1alpha1.Fenced,
			)

			By("Waiting for fencing operation to complete")
			nf = f.WaitForNetworkFenceResult(nf.Name, csiaddonsv1alpha1.FencingOperationResultSucceeded)
			Expect(nf.Status.Result).To(Equal(csiaddonsv1alpha1.FencingOperationResultSucceeded))
			Expect(nf.Status.Message).To(ContainSubstring("fencing operation successful"))

			By("Updating NetworkFence to Unfenced state")
			nf.Spec.FenceState = csiaddonsv1alpha1.Unfenced
			nf = f.UpdateNetworkFence(nf)
			Expect(nf.Spec.FenceState).To(Equal(csiaddonsv1alpha1.Unfenced))

			By("Waiting for unfencing operation to complete")
			nf = f.WaitForNetworkFenceResult(nf.Name, csiaddonsv1alpha1.FencingOperationResultSucceeded)
			Expect(nf.Status.Result).To(Equal(csiaddonsv1alpha1.FencingOperationResultSucceeded))
			Expect(nf.Status.Message).To(ContainSubstring("unfencing operation successful"))
		})
	})

	Context("NetworkFenceClass", func() {
		It("should use NetworkFenceClass for configuration", func() {
			By("Getting NetworkFenceClass configuration")
			provisioner := f.GetNetworkFenceProvisioner()
			Expect(provisioner).NotTo(BeEmpty(), "Provisioner must be configured")
			parameters := f.GetNetworkFenceParameters()
			cidrs := f.GetNetworkFenceCIDRs()
			Expect(cidrs).NotTo(BeEmpty(), "CIDRs must be configured in e2e-config.yaml")

			By("Creating a NetworkFenceClass")
			nfc := f.CreateNetworkFenceClass(
				"test-nf-class",
				provisioner,
				parameters,
			)
			Expect(nfc.Name).To(Equal("test-nf-class"))
			Expect(nfc.Spec.Provisioner).To(Equal(provisioner))

			By("Creating a NetworkFence using NetworkFenceClass")
			nf := f.CreateNetworkFenceWithClass(
				"test-nf-with-class",
				cidrs,
				csiaddonsv1alpha1.Fenced,
				nfc.Name,
			)

			By("Verifying NetworkFence was created with class")
			Expect(nf.Spec.NetworkFenceClassName).To(Equal(nfc.Name))
			Expect(nf.Spec.Cidrs).To(Equal(cidrs))
			Expect(nf.Spec.FenceState).To(Equal(csiaddonsv1alpha1.Fenced))

			By("Waiting for NetworkFence operation to complete")
			nf = f.WaitForNetworkFenceResult(nf.Name, csiaddonsv1alpha1.FencingOperationResultSucceeded)
			Expect(nf.Status.Result).To(Equal(csiaddonsv1alpha1.FencingOperationResultSucceeded))
		})

		It("should transition NetworkFence with class from fenced to unfenced", func() {
			By("Getting NetworkFenceClass configuration")
			provisioner := f.GetNetworkFenceProvisioner()
			Expect(provisioner).NotTo(BeEmpty(), "Provisioner must be configured")
			parameters := f.GetNetworkFenceParameters()
			cidrs := f.GetNetworkFenceCIDRs()
			Expect(cidrs).NotTo(BeEmpty(), "CIDRs must be configured in e2e-config.yaml")

			By("Creating a NetworkFenceClass")
			nfc := f.CreateNetworkFenceClass(
				"test-nf-class-transition",
				provisioner,
				parameters,
			)

			By("Creating a NetworkFence with Fenced state using class")
			nf := f.CreateNetworkFenceWithClass(
				"test-nf-class-transition",
				cidrs,
				csiaddonsv1alpha1.Fenced,
				nfc.Name,
			)

			By("Waiting for fencing operation to complete")
			nf = f.WaitForNetworkFenceResult(nf.Name, csiaddonsv1alpha1.FencingOperationResultSucceeded)
			Expect(nf.Status.Result).To(Equal(csiaddonsv1alpha1.FencingOperationResultSucceeded))

			By("Updating NetworkFence to Unfenced state")
			nf.Spec.FenceState = csiaddonsv1alpha1.Unfenced
			nf = f.UpdateNetworkFence(nf)

			By("Waiting for unfencing operation to complete")
			nf = f.WaitForNetworkFenceResult(nf.Name, csiaddonsv1alpha1.FencingOperationResultSucceeded)
			Expect(nf.Status.Result).To(Equal(csiaddonsv1alpha1.FencingOperationResultSucceeded))
			Expect(nf.Status.Message).To(ContainSubstring("unfencing operation successful"))
		})
	})
})
