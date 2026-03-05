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

package volumegroupreplication_test

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	replicationv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/replication.storage/v1alpha1"
	"github.com/csi-addons/kubernetes-csi-addons/test/e2e/config"
	"github.com/csi-addons/kubernetes-csi-addons/test/e2e/framework"
)

func init() {
	// Register configuration flags
	config.RegisterFlags()
}

func TestVolumeGroupReplication(t *testing.T) {
	// Parse flags
	flag.Parse()

	// Load configuration
	_, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load E2E configuration: %v", err)
	}

	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "VolumeGroupReplication E2E Suite")
}

var _ = ginkgo.Describe("VolumeGroupReplication", ginkgo.Ordered, func() {
	var (
		f *framework.Framework
	)

	ginkgo.BeforeAll(func() {
		f = framework.NewFramework("volumegroupreplication-e2e")
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
			}
		}
	})

	ginkgo.Context("VolumeGroupReplicationClass", func() {
		ginkgo.It("should create VolumeGroupReplicationClass with configuration from config", func() {
			ginkgo.By("Getting VolumeGroupReplicationClass configuration")
			provisioner := f.GetVolumeGroupReplicationProvisioner()
			gomega.Expect(provisioner).NotTo(gomega.BeEmpty(), "Provisioner must be configured")
			parameters := f.GetVolumeGroupReplicationParameters()

			ginkgo.By("Creating a VolumeGroupReplicationClass")
			vgrc := f.CreateVolumeGroupReplicationClass(
				"test-vgrc",
				provisioner,
				parameters,
			)

			ginkgo.By("Verifying VolumeGroupReplicationClass was created")
			gomega.Expect(vgrc.Name).To(gomega.Equal("test-vgrc"))
			gomega.Expect(vgrc.Spec.Provisioner).To(gomega.Equal(provisioner))
			gomega.Expect(vgrc.Spec.Parameters).To(gomega.Equal(parameters))

			ginkgo.By("Getting VolumeGroupReplicationClass")
			retrievedVGRC := f.GetVolumeGroupReplicationClass(vgrc.Name)
			gomega.Expect(retrievedVGRC).NotTo(gomega.BeNil())
			gomega.Expect(retrievedVGRC.Spec.Provisioner).To(gomega.Equal(provisioner))
		})
	})

	ginkgo.Context("VolumeGroupReplication Operations", func() {
		ginkgo.It("should create VolumeGroupReplication with multiple PVCs", func() {
			ginkgo.By("Getting VolumeGroupReplicationClass configuration")
			provisioner := f.GetVolumeGroupReplicationProvisioner()
			gomega.Expect(provisioner).NotTo(gomega.BeEmpty(), "Provisioner must be configured")
			parameters := f.GetVolumeGroupReplicationParameters()

			ginkgo.By("Creating a VolumeGroupReplicationClass")
			vgrc := createVolumeGroupReplicationClass(f, "test-vgrc-multi", provisioner, parameters)

			ginkgo.By("Creating a VolumeReplicationClass for individual volumes")
			vrc := f.CreateVolumeReplicationClass(
				"test-vrc-for-group",
				f.GetVolumeReplicationProvisioner(),
				f.GetVolumeReplicationParameters(),
			)

			ginkgo.By("Creating multiple PVCs with matching labels")
			groupLabel := "test-group"
			pvc1 := createPVCWithLabels(f, "test-pvc-1", map[string]string{"replication-group": groupLabel})
			pvc2 := createPVCWithLabels(f, "test-pvc-2", map[string]string{"replication-group": groupLabel})
			pvc3 := createPVCWithLabels(f, "test-pvc-3", map[string]string{"replication-group": groupLabel})

			ginkgo.By("Waiting for PVCs to be bound")
			pvc1 = f.WaitForPVCBound(pvc1.Name)
			pvc2 = f.WaitForPVCBound(pvc2.Name)
			pvc3 = f.WaitForPVCBound(pvc3.Name)
			gomega.Expect(pvc1.Status.Phase).To(gomega.Equal(corev1.ClaimBound))
			gomega.Expect(pvc2.Status.Phase).To(gomega.Equal(corev1.ClaimBound))
			gomega.Expect(pvc3.Status.Phase).To(gomega.Equal(corev1.ClaimBound))

			ginkgo.By("Creating VolumeGroupReplication with label selector")
			vgr := createVolumeGroupReplication(f, "test-vgr-multi", vgrc.Name, vrc.Name, groupLabel, replicationv1alpha1.Primary)

			ginkgo.By("Waiting for VolumeGroupReplication to reach primary state")
			vgr = waitForVolumeGroupReplicationState(f, vgr.Name, replicationv1alpha1.PrimaryState)
			gomega.Expect(vgr.Status.State).To(gomega.Equal(replicationv1alpha1.PrimaryState))

			ginkgo.By("Verifying all PVCs are in the group")
			gomega.Expect(vgr.Status.PersistentVolumeClaimsRefList).To(gomega.HaveLen(3))
			pvcNames := getPVCNamesFromStatus(vgr)
			gomega.Expect(pvcNames).To(gomega.ContainElement(pvc1.Name))
			gomega.Expect(pvcNames).To(gomega.ContainElement(pvc2.Name))
			gomega.Expect(pvcNames).To(gomega.ContainElement(pvc3.Name))
		})

		ginkgo.It("should add PVC to existing VolumeGroupReplication", func() {
			ginkgo.By("Getting VolumeGroupReplicationClass configuration")
			provisioner := f.GetVolumeGroupReplicationProvisioner()
			gomega.Expect(provisioner).NotTo(gomega.BeEmpty(), "Provisioner must be configured")
			parameters := f.GetVolumeGroupReplicationParameters()

			ginkgo.By("Creating a VolumeGroupReplicationClass")
			vgrc := createVolumeGroupReplicationClass(f, "test-vgrc-add", provisioner, parameters)

			ginkgo.By("Creating a VolumeReplicationClass for individual volumes")
			vrc := f.CreateVolumeReplicationClass(
				"test-vrc-add",
				f.GetVolumeReplicationProvisioner(),
				f.GetVolumeReplicationParameters(),
			)

			ginkgo.By("Creating initial PVCs with matching labels")
			groupLabel := "test-group-add"
			pvc1 := createPVCWithLabels(f, "test-pvc-add-1", map[string]string{"replication-group": groupLabel})
			pvc2 := createPVCWithLabels(f, "test-pvc-add-2", map[string]string{"replication-group": groupLabel})

			ginkgo.By("Waiting for initial PVCs to be bound")
			pvc1 = f.WaitForPVCBound(pvc1.Name)
			pvc2 = f.WaitForPVCBound(pvc2.Name)

			ginkgo.By("Creating VolumeGroupReplication")
			vgr := createVolumeGroupReplication(f, "test-vgr-add", vgrc.Name, vrc.Name, groupLabel, replicationv1alpha1.Primary)

			ginkgo.By("Waiting for VolumeGroupReplication to reach primary state")
			vgr = waitForVolumeGroupReplicationState(f, vgr.Name, replicationv1alpha1.PrimaryState)

			ginkgo.By("Verifying initial PVCs are in the group")
			gomega.Expect(vgr.Status.PersistentVolumeClaimsRefList).To(gomega.HaveLen(2))

			ginkgo.By("Creating a new PVC with matching label")
			pvc3 := createPVCWithLabels(f, "test-pvc-add-3", map[string]string{"replication-group": groupLabel})
			pvc3 = f.WaitForPVCBound(pvc3.Name)

			ginkgo.By("Waiting for VolumeGroupReplication to include the new PVC")
			vgr = waitForPVCInVolumeGroupReplication(f, vgr.Name, pvc3.Name, 2*time.Minute)

			ginkgo.By("Verifying all three PVCs are now in the group")
			gomega.Expect(vgr.Status.PersistentVolumeClaimsRefList).To(gomega.HaveLen(3))
			pvcNames := getPVCNamesFromStatus(vgr)
			gomega.Expect(pvcNames).To(gomega.ContainElement(pvc1.Name))
			gomega.Expect(pvcNames).To(gomega.ContainElement(pvc2.Name))
			gomega.Expect(pvcNames).To(gomega.ContainElement(pvc3.Name))
		})

		ginkgo.It("should remove PVC from VolumeGroupReplication when label is removed", func() {
			ginkgo.By("Getting VolumeGroupReplicationClass configuration")
			provisioner := f.GetVolumeGroupReplicationProvisioner()
			gomega.Expect(provisioner).NotTo(gomega.BeEmpty(), "Provisioner must be configured")
			parameters := f.GetVolumeGroupReplicationParameters()

			ginkgo.By("Creating a VolumeGroupReplicationClass")
			vgrc := createVolumeGroupReplicationClass(f, "test-vgrc-remove", provisioner, parameters)

			ginkgo.By("Creating a VolumeReplicationClass for individual volumes")
			vrc := f.CreateVolumeReplicationClass(
				"test-vrc-remove",
				f.GetVolumeReplicationProvisioner(),
				f.GetVolumeReplicationParameters(),
			)

			ginkgo.By("Creating PVCs with matching labels")
			groupLabel := "test-group-remove"
			pvc1 := createPVCWithLabels(f, "test-pvc-remove-1", map[string]string{"replication-group": groupLabel})
			pvc2 := createPVCWithLabels(f, "test-pvc-remove-2", map[string]string{"replication-group": groupLabel})
			pvc3 := createPVCWithLabels(f, "test-pvc-remove-3", map[string]string{"replication-group": groupLabel})

			ginkgo.By("Waiting for PVCs to be bound")
			pvc1 = f.WaitForPVCBound(pvc1.Name)
			pvc2 = f.WaitForPVCBound(pvc2.Name)
			pvc3 = f.WaitForPVCBound(pvc3.Name)

			ginkgo.By("Creating VolumeGroupReplication")
			vgr := createVolumeGroupReplication(f, "test-vgr-remove", vgrc.Name, vrc.Name, groupLabel, replicationv1alpha1.Primary)

			ginkgo.By("Waiting for VolumeGroupReplication to reach primary state")
			vgr = waitForVolumeGroupReplicationState(f, vgr.Name, replicationv1alpha1.PrimaryState)

			ginkgo.By("Verifying all three PVCs are in the group")
			gomega.Expect(vgr.Status.PersistentVolumeClaimsRefList).To(gomega.HaveLen(3))

			ginkgo.By("Removing label from one PVC")
			pvc2 = f.GetPVC(pvc2.Name)
			delete(pvc2.Labels, "replication-group")
			updatePVC(f, pvc2)

			ginkgo.By("Waiting for VolumeGroupReplication to remove the PVC")
			vgr = waitForPVCRemovedFromVolumeGroupReplication(f, vgr.Name, pvc2.Name, 2*time.Minute)

			ginkgo.By("Verifying only two PVCs remain in the group")
			gomega.Expect(vgr.Status.PersistentVolumeClaimsRefList).To(gomega.HaveLen(2))
			pvcNames := getPVCNamesFromStatus(vgr)
			gomega.Expect(pvcNames).To(gomega.ContainElement(pvc1.Name))
			gomega.Expect(pvcNames).NotTo(gomega.ContainElement(pvc2.Name))
			gomega.Expect(pvcNames).To(gomega.ContainElement(pvc3.Name))
		})

		ginkgo.It("should promote volume group to primary", func() {
			ginkgo.By("Getting VolumeGroupReplicationClass configuration")
			provisioner := f.GetVolumeGroupReplicationProvisioner()
			gomega.Expect(provisioner).NotTo(gomega.BeEmpty(), "Provisioner must be configured")
			parameters := f.GetVolumeGroupReplicationParameters()

			ginkgo.By("Creating a VolumeGroupReplicationClass")
			vgrc := createVolumeGroupReplicationClass(f, "test-vgrc-promote", provisioner, parameters)

			ginkgo.By("Creating a VolumeReplicationClass for individual volumes")
			vrc := f.CreateVolumeReplicationClass(
				"test-vrc-promote",
				f.GetVolumeReplicationProvisioner(),
				f.GetVolumeReplicationParameters(),
			)

			ginkgo.By("Creating PVCs with matching labels")
			groupLabel := "test-group-promote"
			pvc1 := createPVCWithLabels(f, "test-pvc-promote-1", map[string]string{"replication-group": groupLabel})
			pvc2 := createPVCWithLabels(f, "test-pvc-promote-2", map[string]string{"replication-group": groupLabel})

			ginkgo.By("Waiting for PVCs to be bound")
			f.WaitForPVCBound(pvc1.Name)
			f.WaitForPVCBound(pvc2.Name)

			ginkgo.By("Creating VolumeGroupReplication in primary state")
			vgr := createVolumeGroupReplication(f, "test-vgr-promote", vgrc.Name, vrc.Name, groupLabel, replicationv1alpha1.Primary)

			ginkgo.By("Waiting for VolumeGroupReplication to reach primary state")
			vgr = waitForVolumeGroupReplicationState(f, vgr.Name, replicationv1alpha1.PrimaryState)
			gomega.Expect(vgr.Status.State).To(gomega.Equal(replicationv1alpha1.PrimaryState))
			gomega.Expect(vgr.Status.PersistentVolumeClaimsRefList).To(gomega.HaveLen(2))
		})

		ginkgo.It("should demote volume group to secondary", func() {
			ginkgo.By("Getting VolumeGroupReplicationClass configuration")
			provisioner := f.GetVolumeGroupReplicationProvisioner()
			gomega.Expect(provisioner).NotTo(gomega.BeEmpty(), "Provisioner must be configured")
			parameters := f.GetVolumeGroupReplicationParameters()

			ginkgo.By("Creating a VolumeGroupReplicationClass")
			vgrc := createVolumeGroupReplicationClass(f, "test-vgrc-demote", provisioner, parameters)

			ginkgo.By("Creating a VolumeReplicationClass for individual volumes")
			vrc := f.CreateVolumeReplicationClass(
				"test-vrc-demote",
				f.GetVolumeReplicationProvisioner(),
				f.GetVolumeReplicationParameters(),
			)

			ginkgo.By("Creating PVCs with matching labels")
			groupLabel := "test-group-demote"
			pvc1 := createPVCWithLabels(f, "test-pvc-demote-1", map[string]string{"replication-group": groupLabel})
			pvc2 := createPVCWithLabels(f, "test-pvc-demote-2", map[string]string{"replication-group": groupLabel})

			ginkgo.By("Waiting for PVCs to be bound")
			f.WaitForPVCBound(pvc1.Name)
			f.WaitForPVCBound(pvc2.Name)

			ginkgo.By("Creating VolumeGroupReplication in secondary state")
			vgr := createVolumeGroupReplication(f, "test-vgr-demote", vgrc.Name, vrc.Name, groupLabel, replicationv1alpha1.Secondary)

			ginkgo.By("Waiting for VolumeGroupReplication to reach secondary state")
			vgr = waitForVolumeGroupReplicationState(f, vgr.Name, replicationv1alpha1.SecondaryState)
			gomega.Expect(vgr.Status.State).To(gomega.Equal(replicationv1alpha1.SecondaryState))
			gomega.Expect(vgr.Status.PersistentVolumeClaimsRefList).To(gomega.HaveLen(2))
		})

		ginkgo.It("should transition volume group from primary to secondary", func() {
			ginkgo.By("Getting VolumeGroupReplicationClass configuration")
			provisioner := f.GetVolumeGroupReplicationProvisioner()
			gomega.Expect(provisioner).NotTo(gomega.BeEmpty(), "Provisioner must be configured")
			parameters := f.GetVolumeGroupReplicationParameters()

			ginkgo.By("Creating a VolumeGroupReplicationClass")
			vgrc := createVolumeGroupReplicationClass(f, "test-vgrc-transition", provisioner, parameters)

			ginkgo.By("Creating a VolumeReplicationClass for individual volumes")
			vrc := f.CreateVolumeReplicationClass(
				"test-vrc-transition",
				f.GetVolumeReplicationProvisioner(),
				f.GetVolumeReplicationParameters(),
			)

			ginkgo.By("Creating PVCs with matching labels")
			groupLabel := "test-group-transition"
			pvc1 := createPVCWithLabels(f, "test-pvc-transition-1", map[string]string{"replication-group": groupLabel})
			pvc2 := createPVCWithLabels(f, "test-pvc-transition-2", map[string]string{"replication-group": groupLabel})

			ginkgo.By("Waiting for PVCs to be bound")
			f.WaitForPVCBound(pvc1.Name)
			f.WaitForPVCBound(pvc2.Name)

			ginkgo.By("Creating VolumeGroupReplication in primary state")
			vgr := createVolumeGroupReplication(f, "test-vgr-transition", vgrc.Name, vrc.Name, groupLabel, replicationv1alpha1.Primary)

			ginkgo.By("Waiting for VolumeGroupReplication to reach primary state")
			vgr = waitForVolumeGroupReplicationState(f, vgr.Name, replicationv1alpha1.PrimaryState)
			gomega.Expect(vgr.Status.State).To(gomega.Equal(replicationv1alpha1.PrimaryState))

			ginkgo.By("Updating VolumeGroupReplication to secondary state")
			vgr.Spec.ReplicationState = replicationv1alpha1.Secondary
			updateVolumeGroupReplication(f, vgr)

			ginkgo.By("Waiting for VolumeGroupReplication to reach secondary state")
			vgr = waitForVolumeGroupReplicationState(f, vgr.Name, replicationv1alpha1.SecondaryState)
			gomega.Expect(vgr.Status.State).To(gomega.Equal(replicationv1alpha1.SecondaryState))
		})

		ginkgo.It("should resync volume group", func() {
			ginkgo.By("Getting VolumeGroupReplicationClass configuration")
			provisioner := f.GetVolumeGroupReplicationProvisioner()
			gomega.Expect(provisioner).NotTo(gomega.BeEmpty(), "Provisioner must be configured")
			parameters := f.GetVolumeGroupReplicationParameters()

			ginkgo.By("Creating a VolumeGroupReplicationClass")
			vgrc := createVolumeGroupReplicationClass(f, "test-vgrc-resync", provisioner, parameters)

			ginkgo.By("Creating a VolumeReplicationClass for individual volumes")
			vrc := f.CreateVolumeReplicationClass(
				"test-vrc-resync",
				f.GetVolumeReplicationProvisioner(),
				f.GetVolumeReplicationParameters(),
			)

			ginkgo.By("Creating PVCs with matching labels")
			groupLabel := "test-group-resync"
			pvc1 := createPVCWithLabels(f, "test-pvc-resync-1", map[string]string{"replication-group": groupLabel})
			pvc2 := createPVCWithLabels(f, "test-pvc-resync-2", map[string]string{"replication-group": groupLabel})

			ginkgo.By("Waiting for PVCs to be bound")
			f.WaitForPVCBound(pvc1.Name)
			f.WaitForPVCBound(pvc2.Name)

			ginkgo.By("Creating VolumeGroupReplication in secondary state")
			vgr := createVolumeGroupReplication(f, "test-vgr-resync", vgrc.Name, vrc.Name, groupLabel, replicationv1alpha1.Secondary)

			ginkgo.By("Waiting for VolumeGroupReplication to reach secondary state")
			vgr = waitForVolumeGroupReplicationState(f, vgr.Name, replicationv1alpha1.SecondaryState)

			ginkgo.By("Triggering resync operation")
			vgr.Spec.ReplicationState = replicationv1alpha1.Resync
			updateVolumeGroupReplication(f, vgr)

			ginkgo.By("Verifying resync state is set")
			vgr = getVolumeGroupReplication(f, vgr.Name)
			gomega.Expect(vgr.Spec.ReplicationState).To(gomega.Equal(replicationv1alpha1.Resync))
		})
	})

	ginkgo.Context("VolumeGroupReplicationContent", func() {
		ginkgo.It("should create and manage VolumeGroupReplicationContent lifecycle", func() {
			ginkgo.By("Getting VolumeGroupReplicationClass configuration")
			provisioner := f.GetVolumeGroupReplicationProvisioner()
			gomega.Expect(provisioner).NotTo(gomega.BeEmpty(), "Provisioner must be configured")
			parameters := f.GetVolumeGroupReplicationParameters()

			ginkgo.By("Creating a VolumeGroupReplicationClass")
			vgrc := createVolumeGroupReplicationClass(f, "test-vgrc-content", provisioner, parameters)

			ginkgo.By("Creating a VolumeReplicationClass for individual volumes")
			vrc := f.CreateVolumeReplicationClass(
				"test-vrc-content",
				f.GetVolumeReplicationProvisioner(),
				f.GetVolumeReplicationParameters(),
			)

			ginkgo.By("Creating PVCs with matching labels")
			groupLabel := "test-group-content"
			pvc1 := createPVCWithLabels(f, "test-pvc-content-1", map[string]string{"replication-group": groupLabel})
			pvc2 := createPVCWithLabels(f, "test-pvc-content-2", map[string]string{"replication-group": groupLabel})

			ginkgo.By("Waiting for PVCs to be bound")
			pvc1 = f.WaitForPVCBound(pvc1.Name)
			pvc2 = f.WaitForPVCBound(pvc2.Name)

			ginkgo.By("Getting PV names from PVCs")
			pv1Name := pvc1.Spec.VolumeName
			pv2Name := pvc2.Spec.VolumeName
			gomega.Expect(pv1Name).NotTo(gomega.BeEmpty(), "PVC1 should have a bound PV")
			gomega.Expect(pv2Name).NotTo(gomega.BeEmpty(), "PVC2 should have a bound PV")

			ginkgo.By("Creating VolumeGroupReplication")
			vgr := createVolumeGroupReplication(f, "test-vgr-content", vgrc.Name, vrc.Name, groupLabel, replicationv1alpha1.Primary)

			ginkgo.By("Waiting for VolumeGroupReplication to reach primary state")
			vgr = waitForVolumeGroupReplicationState(f, vgr.Name, replicationv1alpha1.PrimaryState)

			ginkgo.By("Verifying VolumeGroupReplicationContent is created and bound")
			gomega.Expect(vgr.Spec.VolumeGroupReplicationContentName).NotTo(gomega.BeEmpty(), "VolumeGroupReplicationContentName should be set in VGR spec")

			ginkgo.By("Getting the VolumeGroupReplicationContent")
			vgrcontent := getVolumeGroupReplicationContent(f, vgr.Spec.VolumeGroupReplicationContentName)
			gomega.Expect(vgrcontent).NotTo(gomega.BeNil())

			ginkgo.By("Verifying VolumeGroupReplicationContent spec fields")
			gomega.Expect(vgrcontent.Spec.VolumeGroupReplicationClassName).To(gomega.Equal(vgrc.Name), "VGRC class name should match")
			gomega.Expect(vgrcontent.Spec.Provisioner).To(gomega.Equal(provisioner), "Provisioner should match")
			gomega.Expect(vgrcontent.Spec.VolumeGroupReplicationRef).NotTo(gomega.BeNil(), "VGR reference should be set")
			gomega.Expect(vgrcontent.Spec.VolumeGroupReplicationRef.Name).To(gomega.Equal(vgr.Name), "VGR reference name should match")
			gomega.Expect(vgrcontent.Spec.VolumeGroupReplicationRef.Namespace).To(gomega.Equal(vgr.Namespace), "VGR reference namespace should match")
			gomega.Expect(vgrcontent.Spec.VolumeGroupReplicationRef.UID).To(gomega.Equal(vgr.UID), "VGR reference UID should match")

			ginkgo.By("Verifying VolumeGroupReplicationContent source contains volume handles")
			gomega.Expect(vgrcontent.Spec.Source.VolumeHandles).NotTo(gomega.BeEmpty(), "Volume handles should be populated")
			gomega.Expect(len(vgrcontent.Spec.Source.VolumeHandles)).To(gomega.Equal(2), "Should have 2 volume handles")

			ginkgo.By("Verifying VolumeGroupReplicationContent has a group handle")
			gomega.Eventually(func() string {
				vgrcontent = getVolumeGroupReplicationContent(f, vgrcontent.Name)
				return vgrcontent.Spec.VolumeGroupReplicationHandle
			}, f.GetTimeout("replication-sync"), 5*time.Second).ShouldNot(gomega.BeEmpty(), "VolumeGroupReplicationHandle should be set")

			ginkgo.By("Verifying VolumeGroupReplicationContent status contains PVs")
			gomega.Eventually(func() int {
				vgrcontent = getVolumeGroupReplicationContent(f, vgrcontent.Name)
				return len(vgrcontent.Status.PersistentVolumeRefList)
			}, f.GetTimeout("replication-sync"), 5*time.Second).Should(gomega.Equal(2), "VolumeGroupReplicationContent should have 2 PVs in status")

			ginkgo.By("Verifying PV names in VolumeGroupReplicationContent status")
			pvNames := getPVNamesFromVGRContentStatus(vgrcontent)
			gomega.Expect(pvNames).To(gomega.ContainElement(pv1Name), "Should contain PV1")
			gomega.Expect(pvNames).To(gomega.ContainElement(pv2Name), "Should contain PV2")

			ginkgo.By("Verifying VolumeGroupReplicationContent has proper labels and annotations")
			gomega.Expect(vgrcontent.Labels).NotTo(gomega.BeNil(), "VGRC should have labels")
			gomega.Expect(vgrcontent.Annotations).NotTo(gomega.BeNil(), "VGRC should have annotations")
		})

		ginkgo.It("should update VolumeGroupReplicationContent when PVCs are added or removed", func() {
			ginkgo.By("Getting VolumeGroupReplicationClass configuration")
			provisioner := f.GetVolumeGroupReplicationProvisioner()
			gomega.Expect(provisioner).NotTo(gomega.BeEmpty(), "Provisioner must be configured")
			parameters := f.GetVolumeGroupReplicationParameters()

			ginkgo.By("Creating a VolumeGroupReplicationClass")
			vgrc := createVolumeGroupReplicationClass(f, "test-vgrc-content-update", provisioner, parameters)

			ginkgo.By("Creating a VolumeReplicationClass for individual volumes")
			vrc := f.CreateVolumeReplicationClass(
				"test-vrc-content-update",
				f.GetVolumeReplicationProvisioner(),
				f.GetVolumeReplicationParameters(),
			)

			ginkgo.By("Creating initial PVCs with matching labels")
			groupLabel := "test-group-content-update"
			pvc1 := createPVCWithLabels(f, "test-pvc-content-update-1", map[string]string{"replication-group": groupLabel})
			pvc2 := createPVCWithLabels(f, "test-pvc-content-update-2", map[string]string{"replication-group": groupLabel})

			ginkgo.By("Waiting for PVCs to be bound")
			pvc1 = f.WaitForPVCBound(pvc1.Name)
			pvc2 = f.WaitForPVCBound(pvc2.Name)
			initialPV1 := pvc1.Spec.VolumeName
			initialPV2 := pvc2.Spec.VolumeName

			ginkgo.By("Creating VolumeGroupReplication")
			vgr := createVolumeGroupReplication(f, "test-vgr-content-update", vgrc.Name, vrc.Name, groupLabel, replicationv1alpha1.Primary)

			ginkgo.By("Waiting for VolumeGroupReplication to reach primary state")
			vgr = waitForVolumeGroupReplicationState(f, vgr.Name, replicationv1alpha1.PrimaryState)

			ginkgo.By("Getting the VolumeGroupReplicationContent")
			vgrcontent := getVolumeGroupReplicationContent(f, vgr.Spec.VolumeGroupReplicationContentName)

			ginkgo.By("Verifying initial state of VolumeGroupReplicationContent")
			gomega.Eventually(func() int {
				vgrcontent = getVolumeGroupReplicationContent(f, vgrcontent.Name)
				return len(vgrcontent.Status.PersistentVolumeRefList)
			}, f.GetTimeout("replication-sync"), 5*time.Second).Should(gomega.Equal(2), "Should start with 2 PVs")

			initialVolumeHandles := vgrcontent.Spec.Source.VolumeHandles
			gomega.Expect(len(initialVolumeHandles)).To(gomega.Equal(2), "Should have 2 volume handles initially")

			ginkgo.By("Adding a new PVC to the group")
			pvc3 := createPVCWithLabels(f, "test-pvc-content-update-3", map[string]string{"replication-group": groupLabel})
			pvc3 = f.WaitForPVCBound(pvc3.Name)
			newPV3 := pvc3.Spec.VolumeName

			ginkgo.By("Waiting for VolumeGroupReplication to include the new PVC")
			vgr = waitForPVCInVolumeGroupReplication(f, vgr.Name, pvc3.Name, 2*time.Minute)

			ginkgo.By("Verifying VolumeGroupReplicationContent is updated with new PV")
			gomega.Eventually(func() int {
				vgrcontent = getVolumeGroupReplicationContent(f, vgrcontent.Name)
				return len(vgrcontent.Status.PersistentVolumeRefList)
			}, f.GetTimeout("replication-sync"), 5*time.Second).Should(gomega.Equal(3), "Should have 3 PVs after adding")

			ginkgo.By("Verifying volume handles are updated")
			gomega.Eventually(func() int {
				vgrcontent = getVolumeGroupReplicationContent(f, vgrcontent.Name)
				return len(vgrcontent.Spec.Source.VolumeHandles)
			}, f.GetTimeout("replication-sync"), 5*time.Second).Should(gomega.Equal(3), "Should have 3 volume handles")

			ginkgo.By("Verifying all PVs are present in status")
			pvNames := getPVNamesFromVGRContentStatus(vgrcontent)
			gomega.Expect(pvNames).To(gomega.ContainElement(initialPV1))
			gomega.Expect(pvNames).To(gomega.ContainElement(initialPV2))
			gomega.Expect(pvNames).To(gomega.ContainElement(newPV3))

			ginkgo.By("Removing a PVC from the group")
			pvc2 = f.GetPVC(pvc2.Name)
			delete(pvc2.Labels, "replication-group")
			updatePVC(f, pvc2)

			ginkgo.By("Waiting for VolumeGroupReplication to remove the PVC")
			waitForPVCRemovedFromVolumeGroupReplication(f, vgr.Name, pvc2.Name, 2*time.Minute)

			ginkgo.By("Verifying VolumeGroupReplicationContent is updated after removal")
			gomega.Eventually(func() int {
				vgrcontent = getVolumeGroupReplicationContent(f, vgrcontent.Name)
				return len(vgrcontent.Status.PersistentVolumeRefList)
			}, f.GetTimeout("replication-sync"), 5*time.Second).Should(gomega.Equal(2), "Should have 2 PVs after removal")

			ginkgo.By("Verifying volume handles are updated after removal")
			gomega.Eventually(func() int {
				vgrcontent = getVolumeGroupReplicationContent(f, vgrcontent.Name)
				return len(vgrcontent.Spec.Source.VolumeHandles)
			}, f.GetTimeout("replication-sync"), 5*time.Second).Should(gomega.Equal(2), "Should have 2 volume handles after removal")

			ginkgo.By("Verifying correct PVs remain in status")
			pvNames = getPVNamesFromVGRContentStatus(vgrcontent)
			gomega.Expect(pvNames).To(gomega.ContainElement(initialPV1))
			gomega.Expect(pvNames).NotTo(gomega.ContainElement(initialPV2))
			gomega.Expect(pvNames).To(gomega.ContainElement(newPV3))
		})

		ginkgo.It("should delete VolumeGroupReplicationContent when VolumeGroupReplication is deleted", func() {
			ginkgo.By("Getting VolumeGroupReplicationClass configuration")
			provisioner := f.GetVolumeGroupReplicationProvisioner()
			gomega.Expect(provisioner).NotTo(gomega.BeEmpty(), "Provisioner must be configured")
			parameters := f.GetVolumeGroupReplicationParameters()

			ginkgo.By("Creating a VolumeGroupReplicationClass")
			vgrc := createVolumeGroupReplicationClass(f, "test-vgrc-content-delete", provisioner, parameters)

			ginkgo.By("Creating a VolumeReplicationClass for individual volumes")
			vrc := f.CreateVolumeReplicationClass(
				"test-vrc-content-delete",
				f.GetVolumeReplicationProvisioner(),
				f.GetVolumeReplicationParameters(),
			)

			ginkgo.By("Creating PVCs with matching labels")
			groupLabel := "test-group-content-delete"
			pvc1 := createPVCWithLabels(f, "test-pvc-content-delete-1", map[string]string{"replication-group": groupLabel})
			pvc2 := createPVCWithLabels(f, "test-pvc-content-delete-2", map[string]string{"replication-group": groupLabel})

			ginkgo.By("Waiting for PVCs to be bound")
			f.WaitForPVCBound(pvc1.Name)
			f.WaitForPVCBound(pvc2.Name)

			ginkgo.By("Creating VolumeGroupReplication")
			vgr := createVolumeGroupReplication(f, "test-vgr-content-delete", vgrc.Name, vrc.Name, groupLabel, replicationv1alpha1.Primary)

			ginkgo.By("Waiting for VolumeGroupReplication to reach primary state")
			vgr = waitForVolumeGroupReplicationState(f, vgr.Name, replicationv1alpha1.PrimaryState)

			ginkgo.By("Getting the VolumeGroupReplicationContent name")
			vgrcontentName := vgr.Spec.VolumeGroupReplicationContentName
			gomega.Expect(vgrcontentName).NotTo(gomega.BeEmpty())

			ginkgo.By("Verifying VolumeGroupReplicationContent exists")
			vgrcontent := getVolumeGroupReplicationContent(f, vgrcontentName)
			gomega.Expect(vgrcontent).NotTo(gomega.BeNil())

			ginkgo.By("Deleting VolumeGroupReplication")
			f.DeleteResource(vgr)

			ginkgo.By("Waiting for VolumeGroupReplication to be deleted")
			f.WaitForResourceDeleted(vgr, f.GetTimeout("operation"))

			ginkgo.By("Verifying VolumeGroupReplicationContent is also deleted")
			gomega.Eventually(func() bool {
				_, err := getVolumeGroupReplicationContentWithError(f, vgrcontentName)
				return err != nil
			}, f.GetTimeout("operation"), 2*time.Second).Should(gomega.BeTrue(), "VolumeGroupReplicationContent should be deleted")
		})
	})

	ginkgo.Context("VolumeGroupReplication Lifecycle", func() {
		ginkgo.It("should create VGR in primary state, transition to secondary, back to primary, and delete", func() {
			ginkgo.By("Getting VolumeGroupReplicationClass configuration")
			provisioner := f.GetVolumeGroupReplicationProvisioner()
			gomega.Expect(provisioner).NotTo(gomega.BeEmpty(), "Provisioner must be configured")
			parameters := f.GetVolumeGroupReplicationParameters()

			ginkgo.By("Creating a VolumeGroupReplicationClass")
			vgrc := createVolumeGroupReplicationClass(f, "test-vgrc-lifecycle", provisioner, parameters)

			ginkgo.By("Creating a VolumeReplicationClass for individual volumes")
			vrc := f.CreateVolumeReplicationClass(
				"test-vrc-lifecycle",
				f.GetVolumeReplicationProvisioner(),
				f.GetVolumeReplicationParameters(),
			)

			ginkgo.By("Creating PVCs with matching labels")
			groupLabel := "test-group-lifecycle"
			pvc1 := createPVCWithLabels(f, "test-pvc-lifecycle-1", map[string]string{"replication-group": groupLabel})
			pvc2 := createPVCWithLabels(f, "test-pvc-lifecycle-2", map[string]string{"replication-group": groupLabel})

			ginkgo.By("Waiting for PVCs to be bound")
			pvc1 = f.WaitForPVCBound(pvc1.Name)
			pvc2 = f.WaitForPVCBound(pvc2.Name)
			gomega.Expect(pvc1.Status.Phase).To(gomega.Equal(corev1.ClaimBound))
			gomega.Expect(pvc2.Status.Phase).To(gomega.Equal(corev1.ClaimBound))

			ginkgo.By("Creating VolumeGroupReplication in primary state")
			vgr := createVolumeGroupReplication(f, "test-vgr-lifecycle", vgrc.Name, vrc.Name, groupLabel, replicationv1alpha1.Primary)

			ginkgo.By("Waiting for VolumeGroupReplication to reach primary state")
			vgr = waitForVolumeGroupReplicationState(f, vgr.Name, replicationv1alpha1.PrimaryState)
			gomega.Expect(vgr.Status.State).To(gomega.Equal(replicationv1alpha1.PrimaryState))
			gomega.Expect(vgr.Status.Conditions).NotTo(gomega.BeEmpty(), "VolumeGroupReplication should have conditions")

			ginkgo.By("Updating VolumeGroupReplication to secondary state")
			vgr.Spec.ReplicationState = replicationv1alpha1.Secondary
			updateVolumeGroupReplication(f, vgr)

			ginkgo.By("Waiting for VolumeGroupReplication to reach secondary state")
			vgr = waitForVolumeGroupReplicationState(f, vgr.Name, replicationv1alpha1.SecondaryState)
			gomega.Expect(vgr.Status.State).To(gomega.Equal(replicationv1alpha1.SecondaryState))
			gomega.Expect(vgr.Status.Conditions).NotTo(gomega.BeEmpty(), "VolumeGroupReplication should have conditions")

			ginkgo.By("Updating VolumeGroupReplication back to primary state")
			vgr.Spec.ReplicationState = replicationv1alpha1.Primary
			updateVolumeGroupReplication(f, vgr)

			ginkgo.By("Waiting for VolumeGroupReplication to reach primary state again")
			vgr = waitForVolumeGroupReplicationState(f, vgr.Name, replicationv1alpha1.PrimaryState)
			gomega.Expect(vgr.Status.State).To(gomega.Equal(replicationv1alpha1.PrimaryState))

			ginkgo.By("Deleting VolumeGroupReplication")
			f.DeleteResource(vgr)

			ginkgo.By("Verifying VolumeGroupReplication is deleted")
			f.WaitForResourceDeleted(vgr, f.GetTimeout("operation"))
		})

	})

	ginkgo.Context("PVC Deletion Protection in VolumeGroup", func() {
		ginkgo.It("should prevent PVC deletion when VGR is in primary state", func() {
			ginkgo.By("Getting VolumeGroupReplicationClass configuration")
			provisioner := f.GetVolumeGroupReplicationProvisioner()
			gomega.Expect(provisioner).NotTo(gomega.BeEmpty(), "Provisioner must be configured")
			parameters := f.GetVolumeGroupReplicationParameters()

			ginkgo.By("Creating a VolumeGroupReplicationClass")
			vgrc := createVolumeGroupReplicationClass(f, "test-vgrc-pvc-delete-primary", provisioner, parameters)

			ginkgo.By("Creating a VolumeReplicationClass for individual volumes")
			vrc := f.CreateVolumeReplicationClass(
				"test-vrc-pvc-delete-primary",
				f.GetVolumeReplicationProvisioner(),
				f.GetVolumeReplicationParameters(),
			)

			ginkgo.By("Creating PVCs with matching labels")
			groupLabel := "test-group-pvc-delete-primary"
			pvc1 := createPVCWithLabels(f, "test-pvc-delete-primary-1", map[string]string{"replication-group": groupLabel})
			pvc2 := createPVCWithLabels(f, "test-pvc-delete-primary-2", map[string]string{"replication-group": groupLabel})

			ginkgo.By("Waiting for PVCs to be bound")
			pvc1 = f.WaitForPVCBound(pvc1.Name)
			f.WaitForPVCBound(pvc2.Name)

			ginkgo.By("Creating VolumeGroupReplication in primary state")
			vgr := createVolumeGroupReplication(f, "test-vgr-pvc-delete-primary", vgrc.Name, vrc.Name, groupLabel, replicationv1alpha1.Primary)
			vgr = waitForVolumeGroupReplicationState(f, vgr.Name, replicationv1alpha1.PrimaryState)

			ginkgo.By("Attempting to delete PVC while VGR is in primary state")
			f.DeletePVC(pvc1.Name)

			ginkgo.By("Verifying PVC is not deleted (should have finalizer)")
			time.Sleep(5 * time.Second)
			pvcCheck := f.GetPVC(pvc1.Name)
			gomega.Expect(pvcCheck).NotTo(gomega.BeNil(), "PVC should still exist")
			gomega.Expect(pvcCheck.DeletionTimestamp).NotTo(gomega.BeNil(), "PVC should have deletion timestamp")

			ginkgo.By("Attempting to change VGR state while PVC deletion is pending")
			vgr = getVolumeGroupReplication(f, vgr.Name)
			vgr.Spec.ReplicationState = replicationv1alpha1.Secondary
			updateVolumeGroupReplication(f, vgr)

			ginkgo.By("Verifying VGR state change is processed")
			time.Sleep(3 * time.Second)
			vgr = getVolumeGroupReplication(f, vgr.Name)
			gomega.Expect(vgr.Spec.ReplicationState).To(gomega.Equal(replicationv1alpha1.Secondary))

			ginkgo.By("Deleting VGR to allow PVC deletion")
			f.DeleteResource(vgr)

			ginkgo.By("Verifying VGR is deleted")
			f.WaitForResourceDeleted(vgr, f.GetTimeout("operation"))

			ginkgo.By("Verifying PVC is eventually deleted after VGR removal")
			ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("pvc-bound"))
			defer cancel()
			err := wait.PollUntilContextTimeout(ctx, 2*time.Second, f.GetTimeout("pvc-bound"), true, func(ctx context.Context) (bool, error) {
				err := f.Client.Get(ctx, client.ObjectKey{
					Name:      pvc1.Name,
					Namespace: f.GetNamespaceName(),
				}, pvc1)
				return apierrors.IsNotFound(err), nil
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred(), "PVC should be deleted after VGR removal")
		})

		ginkgo.It("should prevent PVC deletion when VGR is in secondary state", func() {
			ginkgo.By("Getting VolumeGroupReplicationClass configuration")
			provisioner := f.GetVolumeGroupReplicationProvisioner()
			gomega.Expect(provisioner).NotTo(gomega.BeEmpty(), "Provisioner must be configured")
			parameters := f.GetVolumeGroupReplicationParameters()

			ginkgo.By("Creating a VolumeGroupReplicationClass")
			vgrc := createVolumeGroupReplicationClass(f, "test-vgrc-pvc-delete-secondary", provisioner, parameters)

			ginkgo.By("Creating a VolumeReplicationClass for individual volumes")
			vrc := f.CreateVolumeReplicationClass(
				"test-vrc-pvc-delete-secondary",
				f.GetVolumeReplicationProvisioner(),
				f.GetVolumeReplicationParameters(),
			)

			ginkgo.By("Creating PVCs with matching labels")
			groupLabel := "test-group-pvc-delete-secondary"
			pvc1 := createPVCWithLabels(f, "test-pvc-delete-secondary-1", map[string]string{"replication-group": groupLabel})
			pvc2 := createPVCWithLabels(f, "test-pvc-delete-secondary-2", map[string]string{"replication-group": groupLabel})

			ginkgo.By("Waiting for PVCs to be bound")
			pvc1 = f.WaitForPVCBound(pvc1.Name)
			f.WaitForPVCBound(pvc2.Name)

			ginkgo.By("Creating VolumeGroupReplication in primary state")
			vgr := createVolumeGroupReplication(f, "test-vgr-pvc-delete-secondary", vgrc.Name, vrc.Name, groupLabel, replicationv1alpha1.Primary)
			vgr = waitForVolumeGroupReplicationState(f, vgr.Name, replicationv1alpha1.SecondaryState)

			ginkgo.By("Waiting for VolumeGroupReplication to reach primary state")
			vgr = waitForVolumeGroupReplicationState(f, vgr.Name, replicationv1alpha1.PrimaryState)
			gomega.Expect(vgr.Status.State).To(gomega.Equal(replicationv1alpha1.PrimaryState))
			gomega.Expect(vgr.Status.Conditions).NotTo(gomega.BeEmpty(), "VolumeGroupReplication should have conditions")

			ginkgo.By("Updating VolumeGroupReplication secondary state")
			vgr.Spec.ReplicationState = replicationv1alpha1.Secondary
			updateVolumeGroupReplication(f, vgr)

			ginkgo.By("Attempting to delete PVC while VGR is in secondary state")
			f.DeletePVC(pvc1.Name)

			ginkgo.By("Verifying PVC is not deleted (should have finalizer)")
			time.Sleep(5 * time.Second)
			pvcCheck := f.GetPVC(pvc1.Name)
			gomega.Expect(pvcCheck).NotTo(gomega.BeNil(), "PVC should still exist")
			gomega.Expect(pvcCheck.DeletionTimestamp).NotTo(gomega.BeNil(), "PVC should have deletion timestamp")

			ginkgo.By("Deleting VGR to allow PVC deletion")
			f.DeleteResource(vgr)

			ginkgo.By("Verifying VGR is deleted")
			f.WaitForResourceDeleted(vgr, f.GetTimeout("operation"))

			ginkgo.By("Verifying PVC is eventually deleted after VGR removal")
			ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("pvc-bound"))
			defer cancel()
			err := wait.PollUntilContextTimeout(ctx, 2*time.Second, f.GetTimeout("pvc-bound"), true, func(ctx context.Context) (bool, error) {
				err := f.Client.Get(ctx, client.ObjectKey{
					Name:      pvc1.Name,
					Namespace: f.GetNamespaceName(),
				}, pvc1)
				return apierrors.IsNotFound(err), nil
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred(), "PVC should be deleted after VGR removal")
		})
	})

	ginkgo.Context("Dynamic Grouping Advanced Scenarios", func() {
		ginkgo.It("should handle multiple PVC additions and removals dynamically", func() {
			ginkgo.By("Getting VolumeGroupReplicationClass configuration")
			provisioner := f.GetVolumeGroupReplicationProvisioner()
			gomega.Expect(provisioner).NotTo(gomega.BeEmpty(), "Provisioner must be configured")
			parameters := f.GetVolumeGroupReplicationParameters()

			ginkgo.By("Creating a VolumeGroupReplicationClass")
			vgrc := createVolumeGroupReplicationClass(f, "test-vgrc-dynamic-multi", provisioner, parameters)

			ginkgo.By("Creating a VolumeReplicationClass for individual volumes")
			vrc := f.CreateVolumeReplicationClass(
				"test-vrc-dynamic-multi",
				f.GetVolumeReplicationProvisioner(),
				f.GetVolumeReplicationParameters(),
			)

			ginkgo.By("Creating initial PVC with matching label")
			groupLabel := "test-group-dynamic-multi"
			pvc1 := createPVCWithLabels(f, "test-pvc-dynamic-multi-1", map[string]string{"replication-group": groupLabel})
			pvc1 = f.WaitForPVCBound(pvc1.Name)

			ginkgo.By("Creating VolumeGroupReplication")
			vgr := createVolumeGroupReplication(f, "test-vgr-dynamic-multi", vgrc.Name, vrc.Name, groupLabel, replicationv1alpha1.Primary)
			vgr = waitForVolumeGroupReplicationState(f, vgr.Name, replicationv1alpha1.PrimaryState)

			ginkgo.By("Verifying initial state with 1 PVC")
			gomega.Expect(vgr.Status.PersistentVolumeClaimsRefList).To(gomega.HaveLen(1))

			ginkgo.By("Adding 3 more PVCs dynamically")
			pvc2 := createPVCWithLabels(f, "test-pvc-dynamic-multi-2", map[string]string{"replication-group": groupLabel})
			pvc3 := createPVCWithLabels(f, "test-pvc-dynamic-multi-3", map[string]string{"replication-group": groupLabel})
			pvc4 := createPVCWithLabels(f, "test-pvc-dynamic-multi-4", map[string]string{"replication-group": groupLabel})

			pvc2 = f.WaitForPVCBound(pvc2.Name)
			pvc3 = f.WaitForPVCBound(pvc3.Name)
			pvc4 = f.WaitForPVCBound(pvc4.Name)

			ginkgo.By("Waiting for all PVCs to be added to the group")
			gomega.Eventually(func() int {
				vgr = getVolumeGroupReplication(f, vgr.Name)
				return len(vgr.Status.PersistentVolumeClaimsRefList)
			}, 2*time.Minute, 5*time.Second).Should(gomega.Equal(4), "Should have 4 PVCs in the group")

			ginkgo.By("Removing labels from 2 PVCs")
			pvc2 = f.GetPVC(pvc2.Name)
			delete(pvc2.Labels, "replication-group")
			updatePVC(f, pvc2)

			pvc4 = f.GetPVC(pvc4.Name)
			delete(pvc4.Labels, "replication-group")
			updatePVC(f, pvc4)

			ginkgo.By("Waiting for PVCs to be removed from the group")
			gomega.Eventually(func() int {
				vgr = getVolumeGroupReplication(f, vgr.Name)
				return len(vgr.Status.PersistentVolumeClaimsRefList)
			}, 2*time.Minute, 5*time.Second).Should(gomega.Equal(2), "Should have 2 PVCs remaining in the group")

			ginkgo.By("Verifying correct PVCs remain")
			pvcNames := getPVCNamesFromStatus(vgr)
			gomega.Expect(pvcNames).To(gomega.ContainElement(pvc1.Name))
			gomega.Expect(pvcNames).NotTo(gomega.ContainElement(pvc2.Name))
			gomega.Expect(pvcNames).To(gomega.ContainElement(pvc3.Name))
			gomega.Expect(pvcNames).NotTo(gomega.ContainElement(pvc4.Name))
		})

		ginkgo.It("should handle PVC addition during state transition", func() {
			ginkgo.By("Getting VolumeGroupReplicationClass configuration")
			provisioner := f.GetVolumeGroupReplicationProvisioner()
			gomega.Expect(provisioner).NotTo(gomega.BeEmpty(), "Provisioner must be configured")
			parameters := f.GetVolumeGroupReplicationParameters()

			ginkgo.By("Creating a VolumeGroupReplicationClass")
			vgrc := createVolumeGroupReplicationClass(f, "test-vgrc-dynamic-transition", provisioner, parameters)

			ginkgo.By("Creating a VolumeReplicationClass for individual volumes")
			vrc := f.CreateVolumeReplicationClass(
				"test-vrc-dynamic-transition",
				f.GetVolumeReplicationProvisioner(),
				f.GetVolumeReplicationParameters(),
			)

			ginkgo.By("Creating initial PVCs with matching labels")
			groupLabel := "test-group-dynamic-transition"
			pvc1 := createPVCWithLabels(f, "test-pvc-dynamic-transition-1", map[string]string{"replication-group": groupLabel})
			pvc2 := createPVCWithLabels(f, "test-pvc-dynamic-transition-2", map[string]string{"replication-group": groupLabel})

			pvc1 = f.WaitForPVCBound(pvc1.Name)
			pvc2 = f.WaitForPVCBound(pvc2.Name)

			ginkgo.By("Creating VolumeGroupReplication in primary state")
			vgr := createVolumeGroupReplication(f, "test-vgr-dynamic-transition", vgrc.Name, vrc.Name, groupLabel, replicationv1alpha1.Primary)
			vgr = waitForVolumeGroupReplicationState(f, vgr.Name, replicationv1alpha1.PrimaryState)

			ginkgo.By("Initiating state transition to secondary")
			vgr.Spec.ReplicationState = replicationv1alpha1.Secondary
			updateVolumeGroupReplication(f, vgr)

			ginkgo.By("Adding a new PVC during state transition")
			pvc3 := createPVCWithLabels(f, "test-pvc-dynamic-transition-3", map[string]string{"replication-group": groupLabel})
			pvc3 = f.WaitForPVCBound(pvc3.Name)

			ginkgo.By("Waiting for VGR to reach secondary state")
			vgr = waitForVolumeGroupReplicationState(f, vgr.Name, replicationv1alpha1.SecondaryState)

			ginkgo.By("Verifying all 3 PVCs are in the group after transition")
			gomega.Eventually(func() int {
				vgr = getVolumeGroupReplication(f, vgr.Name)
				return len(vgr.Status.PersistentVolumeClaimsRefList)
			}, 2*time.Minute, 5*time.Second).Should(gomega.Equal(3), "Should have 3 PVCs in the group")

			pvcNames := getPVCNamesFromStatus(vgr)
			gomega.Expect(pvcNames).To(gomega.ContainElement(pvc1.Name))
			gomega.Expect(pvcNames).To(gomega.ContainElement(pvc2.Name))
			gomega.Expect(pvcNames).To(gomega.ContainElement(pvc3.Name))
		})
	})
})

// Helper functions

// createVolumeGroupReplicationClass creates a VolumeGroupReplicationClass
func createVolumeGroupReplicationClass(f *framework.Framework, name string, provisioner string, parameters map[string]string) *replicationv1alpha1.VolumeGroupReplicationClass {
	return f.CreateVolumeGroupReplicationClass(name, provisioner, parameters)
}

// createPVCWithLabels creates a PVC with specified labels
func createPVCWithLabels(f *framework.Framework, name string, labels map[string]string) *corev1.PersistentVolumeClaim {
	ginkgo.By(fmt.Sprintf("Creating PVC %s with labels %v", name, labels))

	pvc := f.CreatePVC(name, "", f.GetVolumeGroupReplicationStorageClassName())

	// Update PVC with labels, retrying on conflict in case the PVC was
	// modified by a controller between creation and the label update.
	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
	defer cancel()

	var err error
	for {
		if err = f.Client.Get(ctx, client.ObjectKey{
			Name:      pvc.Name,
			Namespace: pvc.Namespace,
		}, pvc); err != nil {
			break
		}
		pvc.Labels = labels
		err = f.Client.Update(ctx, pvc)
		if !apierrors.IsConflict(err) {
			break
		}
	}
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to update PVC with labels")

	return pvc
}

// createVolumeGroupReplication creates a VolumeGroupReplication
func createVolumeGroupReplication(f *framework.Framework, name string, vgrcName string, vrcName string, labelValue string, replicationState replicationv1alpha1.ReplicationState) *replicationv1alpha1.VolumeGroupReplication {
	ginkgo.By(fmt.Sprintf("Creating VolumeGroupReplication %s", name))

	vgr := &replicationv1alpha1.VolumeGroupReplication{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: f.GetNamespaceName(),
		},
		Spec: replicationv1alpha1.VolumeGroupReplicationSpec{
			VolumeGroupReplicationClassName: vgrcName,
			VolumeReplicationClassName:      vrcName,
			ReplicationState:                replicationState,
			Source: replicationv1alpha1.VolumeGroupReplicationSource{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"replication-group": labelValue,
					},
				},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
	defer cancel()

	err := f.Client.Create(ctx, vgr)
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to create VolumeGroupReplication")

	f.TrackResource(vgr)

	return vgr
}

// waitForVolumeGroupReplicationState waits for a VolumeGroupReplication to reach a specific state
func waitForVolumeGroupReplicationState(f *framework.Framework, name string, expectedState replicationv1alpha1.State) *replicationv1alpha1.VolumeGroupReplication {
	ginkgo.By(fmt.Sprintf("Waiting for VolumeGroupReplication %s to reach state %s", name, expectedState))

	vgr := &replicationv1alpha1.VolumeGroupReplication{}
	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("replication-sync"))
	defer cancel()

	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, f.GetTimeout("replication-sync"), true, func(ctx context.Context) (bool, error) {
		err := f.Client.Get(ctx, client.ObjectKey{
			Name:      name,
			Namespace: f.GetNamespaceName(),
		}, vgr)
		if err != nil {
			return false, err
		}
		return vgr.Status.State == expectedState, nil
	})

	gomega.Expect(err).NotTo(gomega.HaveOccurred(), fmt.Sprintf("VolumeGroupReplication did not reach state %s in time", expectedState))
	return vgr
}

// getVolumeGroupReplication gets a VolumeGroupReplication
func getVolumeGroupReplication(f *framework.Framework, name string) *replicationv1alpha1.VolumeGroupReplication {
	vgr := &replicationv1alpha1.VolumeGroupReplication{}
	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
	defer cancel()

	err := f.Client.Get(ctx, client.ObjectKey{
		Name:      name,
		Namespace: f.GetNamespaceName(),
	}, vgr)
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to get VolumeGroupReplication")
	return vgr
}

// updateVolumeGroupReplication updates a VolumeGroupReplication
func updateVolumeGroupReplication(f *framework.Framework, vgr *replicationv1alpha1.VolumeGroupReplication) {
	ginkgo.By(fmt.Sprintf("Updating VolumeGroupReplication %s", vgr.Name))

	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
	defer cancel()

	err := f.Client.Update(ctx, vgr)
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to update VolumeGroupReplication")
}

// updatePVC updates a PVC
func updatePVC(f *framework.Framework, pvc *corev1.PersistentVolumeClaim) {
	ginkgo.By(fmt.Sprintf("Updating PVC %s", pvc.Name))

	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
	defer cancel()

	err := f.Client.Update(ctx, pvc)
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to update PVC")
}

// getPVCNamesFromStatus extracts PVC names from VolumeGroupReplication status
func getPVCNamesFromStatus(vgr *replicationv1alpha1.VolumeGroupReplication) []string {
	names := make([]string, 0, len(vgr.Status.PersistentVolumeClaimsRefList))
	for _, ref := range vgr.Status.PersistentVolumeClaimsRefList {
		names = append(names, ref.Name)
	}
	return names
}

// waitForPVCInVolumeGroupReplication waits for a PVC to be added to VolumeGroupReplication
func waitForPVCInVolumeGroupReplication(f *framework.Framework, vgrName string, pvcName string, timeout time.Duration) *replicationv1alpha1.VolumeGroupReplication {
	ginkgo.By(fmt.Sprintf("Waiting for PVC %s to be added to VolumeGroupReplication %s", pvcName, vgrName))

	vgr := &replicationv1alpha1.VolumeGroupReplication{}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		err := f.Client.Get(ctx, client.ObjectKey{
			Name:      vgrName,
			Namespace: f.GetNamespaceName(),
		}, vgr)
		if err != nil {
			return false, err
		}

		pvcNames := getPVCNamesFromStatus(vgr)
		for _, name := range pvcNames {
			if name == pvcName {
				return true, nil
			}
		}
		return false, nil
	})

	gomega.Expect(err).NotTo(gomega.HaveOccurred(), fmt.Sprintf("PVC %s was not added to VolumeGroupReplication in time", pvcName))
	return vgr
}

// waitForPVCRemovedFromVolumeGroupReplication waits for a PVC to be removed from VolumeGroupReplication
func waitForPVCRemovedFromVolumeGroupReplication(f *framework.Framework, vgrName string, pvcName string, timeout time.Duration) *replicationv1alpha1.VolumeGroupReplication {
	ginkgo.By(fmt.Sprintf("Waiting for PVC %s to be removed from VolumeGroupReplication %s", pvcName, vgrName))

	vgr := &replicationv1alpha1.VolumeGroupReplication{}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		err := f.Client.Get(ctx, client.ObjectKey{
			Name:      vgrName,
			Namespace: f.GetNamespaceName(),
		}, vgr)
		if err != nil {
			return false, err
		}

		pvcNames := getPVCNamesFromStatus(vgr)
		for _, name := range pvcNames {
			if name == pvcName {
				return false, nil
			}
		}
		return true, nil
	})

	gomega.Expect(err).NotTo(gomega.HaveOccurred(), fmt.Sprintf("PVC %s was not removed from VolumeGroupReplication in time", pvcName))
	return vgr
}

// getVolumeGroupReplicationContent gets a VolumeGroupReplicationContent
func getVolumeGroupReplicationContent(f *framework.Framework, name string) *replicationv1alpha1.VolumeGroupReplicationContent {
	vgrcontent := &replicationv1alpha1.VolumeGroupReplicationContent{}
	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
	defer cancel()

	err := f.Client.Get(ctx, client.ObjectKey{Name: name}, vgrcontent)
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to get VolumeGroupReplicationContent")
	return vgrcontent
}

// getVolumeGroupReplicationContentWithError gets a VolumeGroupReplicationContent and returns error
func getVolumeGroupReplicationContentWithError(f *framework.Framework, name string) (*replicationv1alpha1.VolumeGroupReplicationContent, error) {
	vgrcontent := &replicationv1alpha1.VolumeGroupReplicationContent{}
	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
	defer cancel()

	err := f.Client.Get(ctx, client.ObjectKey{Name: name}, vgrcontent)
	return vgrcontent, err
}

// getPVNamesFromVGRContentStatus extracts PV names from VolumeGroupReplicationContent status
func getPVNamesFromVGRContentStatus(vgrcontent *replicationv1alpha1.VolumeGroupReplicationContent) []string {
	names := make([]string, 0, len(vgrcontent.Status.PersistentVolumeRefList))
	for _, ref := range vgrcontent.Status.PersistentVolumeRefList {
		names = append(names, ref.Name)
	}
	return names
}
