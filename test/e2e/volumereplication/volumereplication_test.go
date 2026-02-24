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

package volumereplication_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	replicationv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/replication.storage/v1alpha1"
	"github.com/csi-addons/kubernetes-csi-addons/test/e2e/framework"
)

func TestVolumeReplication(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "VolumeReplication E2E Suite")
}

var _ = Describe("VolumeReplication", func() {
	var (
		f *framework.Framework
	)

	BeforeEach(func() {
		f = framework.NewFramework("volumereplication-e2e")
	})

	AfterEach(func() {
		if CurrentSpecReport().Failed() {
			f.CleanupOnFailure()
		} else {
			f.Cleanup()
		}
	})

	Context("Primary-Secondary Replication", func() {
		It("should promote volume to primary", func() {
			By("Getting VolumeReplicationClass configuration")
			provisioner := f.GetVolumeReplicationProvisioner()
			Expect(provisioner).NotTo(BeEmpty(), "Provisioner must be configured")
			parameters := f.GetVolumeReplicationParameters()

			By("Creating a VolumeReplicationClass")
			vrc := f.CreateVolumeReplicationClass(
				"test-vrc",
				provisioner,
				parameters,
			)

			By("Creating a PVC")
			pvc := f.CreatePVC("test-pvc", "", "")
			pvc = f.WaitForPVCBound(pvc.Name)
			Expect(pvc.Status.Phase).To(Equal(corev1.ClaimBound))

			By("Creating a VolumeReplication in primary state")
			vr := f.CreateVolumeReplication(
				"test-vr",
				pvc.Name,
				vrc.Name,
				replicationv1alpha1.Primary,
			)

			By("Waiting for VolumeReplication to reach primary state")
			vr = f.WaitForVolumeReplicationState(vr.Name, replicationv1alpha1.PrimaryState)
			Expect(vr.Status.State).To(Equal(replicationv1alpha1.PrimaryState))
		})

		It("should demote volume to secondary", func() {
			By("Getting VolumeReplicationClass configuration")
			provisioner := f.GetVolumeReplicationProvisioner()
			Expect(provisioner).NotTo(BeEmpty(), "Provisioner must be configured")
			parameters := f.GetVolumeReplicationParameters()

			By("Creating a VolumeReplicationClass")
			vrc := f.CreateVolumeReplicationClass(
				"test-vrc-secondary",
				provisioner,
				parameters,
			)

			By("Creating a PVC")
			pvc := f.CreatePVC("test-pvc-secondary", "", "")
			pvc = f.WaitForPVCBound(pvc.Name)

			By("Creating a VolumeReplication in secondary state")
			vr := f.CreateVolumeReplication(
				"test-vr-secondary",
				pvc.Name,
				vrc.Name,
				replicationv1alpha1.Secondary,
			)

			By("Waiting for VolumeReplication to reach secondary state")
			vr = f.WaitForVolumeReplicationState(vr.Name, replicationv1alpha1.SecondaryState)
			Expect(vr.Status.State).To(Equal(replicationv1alpha1.SecondaryState))
		})

		It("should transition from primary to secondary", func() {
			By("Getting VolumeReplicationClass configuration")
			provisioner := f.GetVolumeReplicationProvisioner()
			Expect(provisioner).NotTo(BeEmpty(), "Provisioner must be configured")
			parameters := f.GetVolumeReplicationParameters()

			By("Creating a VolumeReplicationClass")
			vrc := f.CreateVolumeReplicationClass(
				"test-vrc-transition",
				provisioner,
				parameters,
			)

			By("Creating a PVC")
			pvc := f.CreatePVC("test-pvc-transition", "", "")
			pvc = f.WaitForPVCBound(pvc.Name)

			By("Creating a VolumeReplication in primary state")
			vr := f.CreateVolumeReplication(
				"test-vr-transition",
				pvc.Name,
				vrc.Name,
				replicationv1alpha1.Primary,
			)

			By("Waiting for VolumeReplication to reach primary state")
			vr = f.WaitForVolumeReplicationState(vr.Name, replicationv1alpha1.PrimaryState)

			By("Updating VolumeReplication to secondary state")
			vr.Spec.ReplicationState = replicationv1alpha1.Secondary
			f.UpdateVolumeReplication(vr)

			By("Waiting for VolumeReplication to reach secondary state")
			vr = f.WaitForVolumeReplicationState(vr.Name, replicationv1alpha1.SecondaryState)
			Expect(vr.Status.State).To(Equal(replicationv1alpha1.SecondaryState))
		})
	})
})
