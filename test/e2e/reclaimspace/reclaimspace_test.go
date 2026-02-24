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

package reclaimspace_test

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/csiaddons/v1alpha1"
	"github.com/csi-addons/kubernetes-csi-addons/test/e2e/framework"
)

func TestReclaimSpace(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ReclaimSpace E2E Suite")
}

var _ = Describe("ReclaimSpace", func() {
	var (
		f *framework.Framework
	)

	BeforeEach(func() {
		f = framework.NewFramework("reclaimspace-e2e")
	})

	AfterEach(func() {
		if CurrentSpecReport().Failed() {
			f.CleanupOnFailure()
		} else {
			f.Cleanup()
		}
	})

	Context("ReclaimSpaceJob", func() {
		It("should successfully reclaim space on a PVC", func() {
			By("Creating a PVC")
			pvc := f.CreatePVC("test-pvc", "", "")
			pvc = f.WaitForPVCBound(pvc.Name)
			Expect(pvc.Status.Phase).To(Equal(corev1.ClaimBound))

			By("Creating a pod to write data to the PVC")
			pod := f.CreatePod("test-pod", pvc.Name, []string{
				"sh", "-c",
				"dd if=/dev/zero of=/mnt/test/testfile bs=1M count=100 && sync",
			})
			f.WaitForPodRunning(pod.Name)

			By("Waiting for pod to complete writing data")
			// In a real scenario, you might wait for pod completion
			// For now, we'll just delete the pod
			f.DeletePod(pod.Name)
			f.WaitForPodDeleted(pod.Name)

			By("Creating a ReclaimSpaceJob")
			rsJob := f.CreateReclaimSpaceJob("test-rsjob", pvc.Name)

			By("Waiting for ReclaimSpaceJob to complete")
			rsJob = f.WaitForReclaimSpaceJobComplete(rsJob.Name)

			By("Verifying ReclaimSpaceJob succeeded")
			Expect(rsJob.Status.Result).To(Equal(csiaddonsv1alpha1.OperationResultSucceeded))
			Expect(rsJob.Status.Message).To(ContainSubstring("successfully"))
		})

		It("should handle ReclaimSpaceJob on unbound PVC", func() {
			By("Creating a ReclaimSpaceJob for non-existent PVC")
			rsJob := f.CreateReclaimSpaceJob("test-rsjob-fail", "non-existent-pvc")

			By("Waiting for ReclaimSpaceJob to complete")
			rsJob = f.WaitForReclaimSpaceJobComplete(rsJob.Name)

			By("Verifying ReclaimSpaceJob failed")
			Expect(rsJob.Status.Result).To(Equal(csiaddonsv1alpha1.OperationResultFailed))
		})

		It("should reclaim space on mounted volume", func() {
			By("Creating a PVC")
			pvc := f.CreatePVC("test-pvc-mounted", "", "")
			pvc = f.WaitForPVCBound(pvc.Name)

			By("Creating a pod that keeps the volume mounted")
			pod := f.CreatePod("test-pod-mounted", pvc.Name, []string{"sleep", "3600"})
			f.WaitForPodRunning(pod.Name)

			By("Creating a ReclaimSpaceJob while volume is mounted")
			rsJob := f.CreateReclaimSpaceJob("test-rsjob-mounted", pvc.Name)

			By("Waiting for ReclaimSpaceJob to complete")
			rsJob = f.WaitForReclaimSpaceJobComplete(rsJob.Name)

			By("Verifying ReclaimSpaceJob succeeded")
			Expect(rsJob.Status.Result).To(Equal(csiaddonsv1alpha1.OperationResultSucceeded))

			By("Cleaning up pod")
			f.DeletePod(pod.Name)
			f.WaitForPodDeleted(pod.Name)
		})
	})

	Context("ReclaimSpaceCronJob", func() {
		It("should create ReclaimSpaceJobs on schedule", func() {
			By("Creating a PVC for cron job testing")
			pvc := f.CreatePVC("test-pvc-cronjob", "", "")
			pvc = f.WaitForPVCBound(pvc.Name)
			Expect(pvc.Status.Phase).To(Equal(corev1.ClaimBound))

			By("Creating a ReclaimSpaceCronJob with 1-minute schedule")
			cronJob := f.CreateReclaimSpaceCronJob("test-rs-cronjob", pvc.Name, "*/1 * * * *")
			Expect(cronJob).NotTo(BeNil())

			By("Waiting for the first ReclaimSpaceJob to be created")
			labelSelector := map[string]string{
				"reclaimspacecronjob": cronJob.Name,
			}
			createdJob := f.WaitForReclaimSpaceJobCreation(labelSelector, 90*time.Second)
			Expect(createdJob).NotTo(BeNil())
			Expect(createdJob.Spec.Target.PersistentVolumeClaim).To(Equal(pvc.Name))

			By("Waiting for the ReclaimSpaceJob to complete")
			completedJob := f.WaitForReclaimSpaceJobComplete(createdJob.Name)
			Expect(completedJob.Status.Result).To(Equal(csiaddonsv1alpha1.OperationResultSucceeded))

			By("Verifying CronJob status is updated")
			updatedCronJob := f.GetReclaimSpaceCronJob(cronJob.Name)
			Expect(updatedCronJob.Status.LastScheduleTime).NotTo(BeNil())
		})
	})
})
