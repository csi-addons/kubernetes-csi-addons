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

package reclaimspace_test

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

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/csiaddons/v1alpha1"
	"github.com/csi-addons/kubernetes-csi-addons/test/e2e/config"
	"github.com/csi-addons/kubernetes-csi-addons/test/e2e/framework"
)

func init() {
	// Register configuration flags
	config.RegisterFlags()
}

func TestReclaimSpace(t *testing.T) {
	// Parse flags
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load E2E configuration: %v", err)
	}

	// Skip if ReclaimSpace tests are disabled
	if !cfg.Tests.ReclaimSpace {
		t.Skip("ReclaimSpace tests are disabled in configuration")
	}

	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "ReclaimSpace E2E Suite")
}

var _ = ginkgo.Describe("ReclaimSpace", ginkgo.Ordered, func() {
	var (
		f *framework.Framework
	)

	ginkgo.BeforeAll(func() {
		f = framework.NewFramework("reclaimspace-e2e")
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

	ginkgo.Context("ReclaimSpaceJob", func() {
		ginkgo.Context("Filesystem mode PVC", func() {
			ginkgo.It("should successfully reclaim space on a filesystem PVC", func() {
				ginkgo.By("Creating a filesystem PVC")
				pvc := f.CreatePVC("test-pvc-fs", "", f.GetReclaimSpaceStorageClassName())
				pvc = f.WaitForPVCBound(pvc.Name)
				gomega.Expect(pvc.Status.Phase).To(gomega.Equal(corev1.ClaimBound))

				ginkgo.By("Creating a pod to write data to the filesystem PVC")
				pod := f.CreatePod("test-pod-fs", pvc.Name, []string{
					"sh", "-c",
					"dd if=/dev/zero of=/mnt/test/testfile bs=1M count=100 && sync && sleep 600",
				})
				f.WaitForPodRunning(pod.Name)

				ginkgo.By("Waiting for pod to complete writing data")

				ginkgo.By("Creating a ReclaimSpaceJob")
				rsJob := f.CreateReclaimSpaceJob("test-rsjob-fs", pvc.Name)

				ginkgo.By("Waiting for ReclaimSpaceJob to complete")
				rsJob = f.WaitForReclaimSpaceJobComplete(rsJob.Name)

				// In a real scenario, you might wait for pod completion
				// For now, we'll just delete the pod
				f.DeletePod(pod.Name)
				f.WaitForPodDeleted(pod.Name)

				ginkgo.By("Waiting for ReclaimSpaceJob status to be updated after pod deletion")
				gomega.Eventually(func() csiaddonsv1alpha1.OperationResult {
					rsJob = f.GetReclaimSpaceJob(rsJob.Name)
					return rsJob.Status.Result
				}, 60*time.Second, 2*time.Second).Should(gomega.Equal(csiaddonsv1alpha1.OperationResultSucceeded),
					"ReclaimSpaceJob status was not updated after pod deletion")

				ginkgo.By("Verifying ReclaimSpaceJob succeeded")
				gomega.Expect(rsJob.Status.Message).To(gomega.ContainSubstring("successfully"))
			})

			ginkgo.It("should reclaim space on unmounted filesystem volume", func() {
				ginkgo.By("Creating a filesystem PVC")
				pvc := f.CreatePVC("test-pvc-fs-unmounted", "", f.GetReclaimSpaceStorageClassName())
				pvc = f.WaitForPVCBound(pvc.Name)

				ginkgo.By("Creating a ReclaimSpaceJob while filesystem volume is unmounted")
				rsJob := f.CreateReclaimSpaceJob("test-rsjob-fs-unmounted", pvc.Name)

				ginkgo.By("Waiting for ReclaimSpaceJob to complete")
				rsJob = f.WaitForReclaimSpaceJobComplete(rsJob.Name)

				ginkgo.By("Verifying ReclaimSpaceJob succeeded")
				gomega.Expect(rsJob.Status.Result).To(gomega.Equal(csiaddonsv1alpha1.OperationResultSucceeded))
			})

			ginkgo.It("should reclaim space on mounted filesystem volume", func() {
				ginkgo.By("Creating a filesystem PVC")
				pvc := f.CreatePVC("test-pvc-fs-mounted", "", f.GetReclaimSpaceStorageClassName())
				pvc = f.WaitForPVCBound(pvc.Name)

				ginkgo.By("Creating a pod that keeps the filesystem volume mounted")
				pod := f.CreatePod("test-pod-fs-mounted", pvc.Name, []string{"sleep", "3600"})
				f.WaitForPodRunning(pod.Name)

				ginkgo.By("Creating a ReclaimSpaceJob while filesystem volume is mounted")
				rsJob := f.CreateReclaimSpaceJob("test-rsjob-fs-mounted", pvc.Name)

				ginkgo.By("Waiting for ReclaimSpaceJob to complete")
				rsJob = f.WaitForReclaimSpaceJobComplete(rsJob.Name)

				ginkgo.By("Verifying ReclaimSpaceJob succeeded")
				gomega.Expect(rsJob.Status.Result).To(gomega.Equal(csiaddonsv1alpha1.OperationResultSucceeded))

				ginkgo.By("Cleaning up pod")
				f.DeletePod(pod.Name)
				f.WaitForPodDeleted(pod.Name)
			})
		})

		ginkgo.Context("Block mode PVC", func() {
			ginkgo.It("should successfully reclaim space on a block PVC", func() {
				ginkgo.By("Creating a block mode PVC")
				pvc := f.CreateBlockPVC("test-pvc-block", "", f.GetReclaimSpaceStorageClassName())
				pvc = f.WaitForPVCBound(pvc.Name)
				gomega.Expect(pvc.Status.Phase).To(gomega.Equal(corev1.ClaimBound))

				ginkgo.By("Creating a pod to write data to the block PVC")
				pod := f.CreatePodWithBlockDevice("test-pod-block", pvc.Name, []string{
					"sh", "-c",
					"dd if=/dev/zero of=/dev/xvda bs=1M count=100 && sync && sleep 600",
				})
				f.WaitForPodRunning(pod.Name)

				ginkgo.By("Waiting for pod to complete writing data")

				ginkgo.By("Creating a ReclaimSpaceJob")
				rsJob := f.CreateReclaimSpaceJob("test-rsjob-block", pvc.Name)

				ginkgo.By("Waiting for ReclaimSpaceJob to complete")
				rsJob = f.WaitForReclaimSpaceJobComplete(rsJob.Name)

				// Delete the pod
				f.DeletePod(pod.Name)
				f.WaitForPodDeleted(pod.Name)

				ginkgo.By("Waiting for ReclaimSpaceJob status to be updated after pod deletion")
				gomega.Eventually(func() csiaddonsv1alpha1.OperationResult {
					rsJob = f.GetReclaimSpaceJob(rsJob.Name)
					return rsJob.Status.Result
				}, 60*time.Second, 2*time.Second).Should(gomega.Equal(csiaddonsv1alpha1.OperationResultSucceeded),
					"ReclaimSpaceJob status was not updated after pod deletion")

				ginkgo.By("Verifying ReclaimSpaceJob succeeded")
				gomega.Expect(rsJob.Status.Message).To(gomega.ContainSubstring("successfully"))
			})

			ginkgo.It("should reclaim space on unmounted block volume", func() {
				ginkgo.By("Creating a block mode PVC")
				pvc := f.CreateBlockPVC("test-pvc-block-unmounted", "", f.GetReclaimSpaceStorageClassName())
				pvc = f.WaitForPVCBound(pvc.Name)

				ginkgo.By("Creating a ReclaimSpaceJob while block volume is unmounted")
				rsJob := f.CreateReclaimSpaceJob("test-rsjob-block-unmounted", pvc.Name)

				ginkgo.By("Waiting for ReclaimSpaceJob to complete")
				rsJob = f.WaitForReclaimSpaceJobComplete(rsJob.Name)

				ginkgo.By("Verifying ReclaimSpaceJob succeeded")
				gomega.Expect(rsJob.Status.Result).To(gomega.Equal(csiaddonsv1alpha1.OperationResultSucceeded))
			})

			ginkgo.It("should reclaim space on mounted block volume", func() {
				ginkgo.By("Creating a block mode PVC")
				pvc := f.CreateBlockPVC("test-pvc-block-mounted", "", f.GetReclaimSpaceStorageClassName())
				pvc = f.WaitForPVCBound(pvc.Name)

				ginkgo.By("Creating a pod that keeps the block volume mounted")
				pod := f.CreatePodWithBlockDevice("test-pod-block-mounted", pvc.Name, []string{"sleep", "3600"})
				f.WaitForPodRunning(pod.Name)

				ginkgo.By("Creating a ReclaimSpaceJob while block volume is mounted")
				rsJob := f.CreateReclaimSpaceJob("test-rsjob-block-mounted", pvc.Name)

				ginkgo.By("Waiting for ReclaimSpaceJob to complete")
				rsJob = f.WaitForReclaimSpaceJobComplete(rsJob.Name)

				ginkgo.By("Verifying ReclaimSpaceJob succeeded")
				gomega.Expect(rsJob.Status.Result).To(gomega.Equal(csiaddonsv1alpha1.OperationResultSucceeded))

				ginkgo.By("Cleaning up pod")
				f.DeletePod(pod.Name)
				f.WaitForPodDeleted(pod.Name)
			})
		})

		ginkgo.It("should handle ReclaimSpaceJob on non-existent PVC", func() {
			ginkgo.By("Creating a ReclaimSpaceJob for non-existent PVC")
			rsJob := f.CreateReclaimSpaceJob("test-rsjob-fail", "non-existent-pvc")

			ginkgo.By("Waiting for ReclaimSpaceJob to complete")
			rsJob = f.WaitForReclaimSpaceJobComplete(rsJob.Name)

			ginkgo.By("Verifying ReclaimSpaceJob failed")
			gomega.Expect(rsJob.Status.Result).To(gomega.Equal(csiaddonsv1alpha1.OperationResultFailed))
		})
	})

	ginkgo.Context("ReclaimSpaceCronJob", func() {
		ginkgo.It("should create ReclaimSpaceJobs on schedule for filesystem PVC", func() {
			ginkgo.By("Creating a filesystem PVC for cron job testing")
			pvc := f.CreatePVC("test-pvc-cronjob-fs", "", f.GetReclaimSpaceStorageClassName())
			pvc = f.WaitForPVCBound(pvc.Name)
			gomega.Expect(pvc.Status.Phase).To(gomega.Equal(corev1.ClaimBound))

			ginkgo.By("Creating a ReclaimSpaceCronJob with 1-minute schedule")
			cronJob := f.CreateReclaimSpaceCronJob("test-rs-cronjob-fs", pvc.Name, "*/1 * * * *")
			gomega.Expect(cronJob).NotTo(gomega.BeNil())

			ginkgo.By("Waiting for the first ReclaimSpaceJob to be created")
			createdJob := f.WaitForReclaimSpaceJobCreation(cronJob.Name, 3*time.Minute)
			gomega.Expect(createdJob).NotTo(gomega.BeNil())
			gomega.Expect(createdJob.Spec.Target.PersistentVolumeClaim).To(gomega.Equal(pvc.Name))

			ginkgo.By("Waiting for the ReclaimSpaceJob to complete")
			completedJob := f.WaitForReclaimSpaceJobComplete(createdJob.Name)
			gomega.Expect(completedJob.Status.Result).To(gomega.Equal(csiaddonsv1alpha1.OperationResultSucceeded))

			ginkgo.By("Waiting for CronJob status to be updated")
			gomega.Eventually(func() *metav1.Time {
				updatedCronJob := f.GetReclaimSpaceCronJob(cronJob.Name)
				return updatedCronJob.Status.LastScheduleTime
			}, 60*time.Second, 2*time.Second).ShouldNot(gomega.BeNil(), "CronJob status was not updated in time")
		})

		ginkgo.It("should create ReclaimSpaceJobs on schedule for block PVC", func() {
			ginkgo.By("Creating a block mode PVC for cron job testing")
			pvc := f.CreateBlockPVC("test-pvc-cronjob-block", "", f.GetReclaimSpaceStorageClassName())
			pvc = f.WaitForPVCBound(pvc.Name)
			gomega.Expect(pvc.Status.Phase).To(gomega.Equal(corev1.ClaimBound))

			ginkgo.By("Creating a ReclaimSpaceCronJob with 1-minute schedule")
			cronJob := f.CreateReclaimSpaceCronJob("test-rs-cronjob-block", pvc.Name, "*/1 * * * *")
			gomega.Expect(cronJob).NotTo(gomega.BeNil())

			ginkgo.By("Waiting for the first ReclaimSpaceJob to be created")
			createdJob := f.WaitForReclaimSpaceJobCreation(cronJob.Name, 3*time.Minute)
			gomega.Expect(createdJob).NotTo(gomega.BeNil())
			gomega.Expect(createdJob.Spec.Target.PersistentVolumeClaim).To(gomega.Equal(pvc.Name))

			ginkgo.By("Waiting for the ReclaimSpaceJob to complete")
			completedJob := f.WaitForReclaimSpaceJobComplete(createdJob.Name)
			gomega.Expect(completedJob.Status.Result).To(gomega.Equal(csiaddonsv1alpha1.OperationResultSucceeded))

			ginkgo.By("Waiting for CronJob status to be updated")
			gomega.Eventually(func() *metav1.Time {
				updatedCronJob := f.GetReclaimSpaceCronJob(cronJob.Name)
				return updatedCronJob.Status.LastScheduleTime
			}, 60*time.Second, 2*time.Second).ShouldNot(gomega.BeNil(), "CronJob status was not updated in time")
		})
	})
})
