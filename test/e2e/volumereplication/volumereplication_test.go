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

package volumereplication_test

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

func TestVolumeReplication(t *testing.T) {
	// Parse flags
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load E2E configuration: %v", err)
	}

	// Skip if VolumeReplication tests are disabled
	if !cfg.Tests.VolumeReplication {
		t.Skip("VolumeReplication tests are disabled in configuration")
	}

	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "VolumeReplication E2E Suite")
}

var _ = ginkgo.Describe("VolumeReplication", ginkgo.Ordered, func() {
	var (
		f *framework.Framework
	)

	ginkgo.BeforeAll(func() {
		f = framework.NewFramework("volumereplication-e2e")
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

	ginkgo.Context("Primary-Secondary Replication Lifecycle", func() {
		ginkgo.It("should create VR in primary state, transition to secondary, back to primary, and delete", func() {
			ginkgo.By("Getting VolumeReplicationClass configuration")
			provisioner := f.GetVolumeReplicationProvisioner()
			gomega.Expect(provisioner).NotTo(gomega.BeEmpty(), "Provisioner must be configured")
			parameters := f.GetVolumeReplicationParameters()

			ginkgo.By("Creating a VolumeReplicationClass")
			vrc := f.CreateVolumeReplicationClass(
				"test-vrc-lifecycle",
				provisioner,
				parameters,
			)

			ginkgo.By("Creating a PVC")
			pvc := f.CreatePVC("test-pvc-lifecycle", "", f.GetVolumeReplicationStorageClassName())
			pvc = f.WaitForPVCBound(pvc.Name)
			gomega.Expect(pvc.Status.Phase).To(gomega.Equal(corev1.ClaimBound))

			ginkgo.By("Creating a VolumeReplication in primary state")
			vr := f.CreateVolumeReplication(
				"test-vr-lifecycle",
				pvc.Name,
				vrc.Name,
				replicationv1alpha1.Primary,
			)

			ginkgo.By("Waiting for VolumeReplication to reach primary state")
			vr = f.WaitForVolumeReplicationState(vr.Name, replicationv1alpha1.PrimaryState)
			gomega.Expect(vr.Status.State).To(gomega.Equal(replicationv1alpha1.PrimaryState))
			gomega.Expect(vr.Status.Conditions).NotTo(gomega.BeEmpty(), "VolumeReplication should have conditions")

			ginkgo.By("Updating VolumeReplication to secondary state")
			vr.Spec.ReplicationState = replicationv1alpha1.Secondary
			f.UpdateVolumeReplication(vr)

			ginkgo.By("Waiting for VolumeReplication to reach secondary state")
			vr = f.WaitForVolumeReplicationState(vr.Name, replicationv1alpha1.SecondaryState)
			gomega.Expect(vr.Status.State).To(gomega.Equal(replicationv1alpha1.SecondaryState))
			gomega.Expect(vr.Status.Conditions).NotTo(gomega.BeEmpty(), "VolumeReplication should have conditions")

			ginkgo.By("Updating VolumeReplication back to primary state")
			vr.Spec.ReplicationState = replicationv1alpha1.Primary
			f.UpdateVolumeReplication(vr)

			ginkgo.By("Waiting for VolumeReplication to reach primary state again")
			vr = f.WaitForVolumeReplicationState(vr.Name, replicationv1alpha1.PrimaryState)
			gomega.Expect(vr.Status.State).To(gomega.Equal(replicationv1alpha1.PrimaryState))

			ginkgo.By("Deleting VolumeReplication")
			f.DeleteResource(vr)

			ginkgo.By("Verifying VolumeReplication is deleted")
			ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
			defer cancel()
			err := wait.PollUntilContextTimeout(ctx, 2*time.Second, f.GetTimeout("operation"), true, func(ctx context.Context) (bool, error) {
				err := f.Client.Get(ctx, client.ObjectKey{
					Name:      vr.Name,
					Namespace: f.GetNamespaceName(),
				}, vr)
				return apierrors.IsNotFound(err), nil
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred(), "VolumeReplication should be deleted")
		})

	})

	ginkgo.Context("PVC Deletion Protection", func() {
		ginkgo.It("should prevent PVC deletion when VR is in primary state", func() {
			ginkgo.By("Getting VolumeReplicationClass configuration")
			provisioner := f.GetVolumeReplicationProvisioner()
			gomega.Expect(provisioner).NotTo(gomega.BeEmpty(), "Provisioner must be configured")
			parameters := f.GetVolumeReplicationParameters()

			ginkgo.By("Creating a VolumeReplicationClass")
			vrc := f.CreateVolumeReplicationClass(
				"test-vrc-pvc-delete-primary",
				provisioner,
				parameters,
			)

			ginkgo.By("Creating a PVC")
			pvc := f.CreatePVC("test-pvc-delete-primary", "", f.GetVolumeReplicationStorageClassName())
			pvc = f.WaitForPVCBound(pvc.Name)

			ginkgo.By("Creating a VolumeReplication in primary state")
			vr := f.CreateVolumeReplication(
				"test-vr-pvc-delete-primary",
				pvc.Name,
				vrc.Name,
				replicationv1alpha1.Primary,
			)
			vr = f.WaitForVolumeReplicationState(vr.Name, replicationv1alpha1.PrimaryState)

			ginkgo.By("Attempting to delete PVC while VR is in primary state")
			f.DeletePVC(pvc.Name)

			ginkgo.By("Verifying PVC is not deleted (should have finalizer)")
			time.Sleep(5 * time.Second)
			pvcCheck := f.GetPVC(pvc.Name)
			gomega.Expect(pvcCheck).NotTo(gomega.BeNil(), "PVC should still exist")
			gomega.Expect(pvcCheck.DeletionTimestamp).NotTo(gomega.BeNil(), "PVC should have deletion timestamp")

			ginkgo.By("Attempting to delete VR while PVC deletion is pending")
			f.DeleteResource(vr)

			ginkgo.By("Verifying VR deletion is processed")
			ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
			defer cancel()
			err := wait.PollUntilContextTimeout(ctx, 2*time.Second, f.GetTimeout("operation"), true, func(ctx context.Context) (bool, error) {
				err := f.Client.Get(ctx, client.ObjectKey{
					Name:      vr.Name,
					Namespace: f.GetNamespaceName(),
				}, vr)
				return apierrors.IsNotFound(err), nil
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred(), "VolumeReplication should be deleted")

			ginkgo.By("Verifying PVC is eventually deleted after VR removal")
			ctx, cancel = context.WithTimeout(context.Background(), f.GetTimeout("pvc-bound"))
			defer cancel()
			err = wait.PollUntilContextTimeout(ctx, 2*time.Second, f.GetTimeout("pvc-bound"), true, func(ctx context.Context) (bool, error) {
				err := f.Client.Get(ctx, client.ObjectKey{
					Name:      pvc.Name,
					Namespace: f.GetNamespaceName(),
				}, pvc)
				return apierrors.IsNotFound(err), nil
			})
			gomega.Expect(err).NotTo(gomega.HaveOccurred(), "PVC should be deleted after VR removal")
		})

	})

	ginkgo.Context("Error Handling", func() {
		ginkgo.It("should fail to enable mirroring on non-existing PVC", func() {
			ginkgo.By("Getting VolumeReplicationClass configuration")
			provisioner := f.GetVolumeReplicationProvisioner()
			gomega.Expect(provisioner).NotTo(gomega.BeEmpty(), "Provisioner must be configured")
			parameters := f.GetVolumeReplicationParameters()

			ginkgo.By("Creating a VolumeReplicationClass")
			vrc := f.CreateVolumeReplicationClass(
				"test-vrc-nonexistent-pvc",
				provisioner,
				parameters,
			)

			ginkgo.By("Attempting to create VolumeReplication for non-existent PVC")
			vr := &replicationv1alpha1.VolumeReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vr-nonexistent-pvc",
					Namespace: f.GetNamespaceName(),
				},
				Spec: replicationv1alpha1.VolumeReplicationSpec{
					VolumeReplicationClass: vrc.Name,
					ReplicationState:       replicationv1alpha1.Primary,
					DataSource: corev1.TypedLocalObjectReference{
						Kind: "PersistentVolumeClaim",
						Name: "non-existent-pvc",
					},
				},
			}

			ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
			defer cancel()

			err := f.Client.Create(ctx, vr)
			gomega.Expect(err).NotTo(gomega.HaveOccurred(), "VolumeReplication creation should succeed")
			f.TrackResource(vr)

			ginkgo.By("Verifying VolumeReplication does not reach ready state")
			time.Sleep(10 * time.Second)
			vr = f.GetVolumeReplication(vr.Name)
			gomega.Expect(vr.Status.State).NotTo(gomega.Equal(replicationv1alpha1.PrimaryState), "VolumeReplication should not reach primary state with non-existent PVC")

			ginkgo.By("Checking for error conditions")
			hasErrorCondition := false
			for _, condition := range vr.Status.Conditions {
				if condition.Type == replicationv1alpha1.ConditionCompleted && condition.Status != "True" {
					hasErrorCondition = true
					break
				}
			}
			gomega.Expect(hasErrorCondition).To(gomega.BeTrue(), "VolumeReplication should have error condition")
		})
	})
})
