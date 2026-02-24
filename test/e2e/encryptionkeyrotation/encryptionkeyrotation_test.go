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

package encryptionkeyrotation_test

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/csiaddons/v1alpha1"
	"github.com/csi-addons/kubernetes-csi-addons/test/e2e/framework"
)

func TestEncryptionKeyRotation(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "EncryptionKeyRotation E2E Suite")
}

var _ = Describe("EncryptionKeyRotation", func() {
	var (
		f *framework.Framework
	)

	BeforeEach(func() {
		f = framework.NewFramework("encryptionkeyrotation-e2e")
	})

	AfterEach(func() {
		if CurrentSpecReport().Failed() {
			f.CleanupOnFailure()
		} else {
			f.Cleanup()
		}
	})

	Context("EncryptionKeyRotationJob", func() {
		Context("Filesystem mode PVC", func() {
			It("should successfully rotate encryption key on a filesystem PVC", func() {
				By("Creating a filesystem PVC with encryption enabled")
				pvc := f.CreatePVC("test-pvc-fs-encrypted", "", "")
				pvc = f.WaitForPVCBound(pvc.Name)
				Expect(pvc.Status.Phase).To(Equal(corev1.ClaimBound))

				By("Creating an EncryptionKeyRotationJob")
				ekrJob := f.CreateEncryptionKeyRotationJob("test-ekrjob-fs", pvc.Name)

				By("Waiting for EncryptionKeyRotationJob to complete")
				ekrJob = f.WaitForEncryptionKeyRotationJobComplete(ekrJob.Name)

				By("Verifying EncryptionKeyRotationJob succeeded")
				Expect(ekrJob.Status.Result).To(Equal(csiaddonsv1alpha1.OperationResultSucceeded))
				Expect(ekrJob.Status.Message).To(ContainSubstring("successfully"))
			})

			It("should rotate key on mounted filesystem volume", func() {
				By("Creating an encrypted filesystem PVC")
				pvc := f.CreatePVC("test-pvc-fs-mounted-encrypted", "", "")
				pvc = f.WaitForPVCBound(pvc.Name)

				By("Creating a pod that keeps the filesystem volume mounted")
				pod := f.CreatePod("test-pod-fs-encrypted", pvc.Name, []string{"sleep", "3600"})
				f.WaitForPodRunning(pod.Name)

				By("Creating an EncryptionKeyRotationJob while filesystem volume is mounted")
				ekrJob := f.CreateEncryptionKeyRotationJob("test-ekrjob-fs-mounted", pvc.Name)

				By("Waiting for EncryptionKeyRotationJob to complete")
				ekrJob = f.WaitForEncryptionKeyRotationJobComplete(ekrJob.Name)

				By("Verifying EncryptionKeyRotationJob succeeded")
				Expect(ekrJob.Status.Result).To(Equal(csiaddonsv1alpha1.OperationResultSucceeded))

				By("Cleaning up pod")
				f.DeletePod(pod.Name)
				f.WaitForPodDeleted(pod.Name)
			})
		})

		Context("Block mode PVC", func() {
			It("should successfully rotate encryption key on a block PVC", func() {
				By("Creating a block mode PVC with encryption enabled")
				pvc := f.CreateBlockPVC("test-pvc-block-encrypted", "", "")
				pvc = f.WaitForPVCBound(pvc.Name)
				Expect(pvc.Status.Phase).To(Equal(corev1.ClaimBound))

				By("Creating an EncryptionKeyRotationJob")
				ekrJob := f.CreateEncryptionKeyRotationJob("test-ekrjob-block", pvc.Name)

				By("Waiting for EncryptionKeyRotationJob to complete")
				ekrJob = f.WaitForEncryptionKeyRotationJobComplete(ekrJob.Name)

				By("Verifying EncryptionKeyRotationJob succeeded")
				Expect(ekrJob.Status.Result).To(Equal(csiaddonsv1alpha1.OperationResultSucceeded))
				Expect(ekrJob.Status.Message).To(ContainSubstring("successfully"))
			})

			It("should rotate key on mounted block volume", func() {
				By("Creating an encrypted block mode PVC")
				pvc := f.CreateBlockPVC("test-pvc-block-mounted-encrypted", "", "")
				pvc = f.WaitForPVCBound(pvc.Name)

				By("Creating a pod that keeps the block volume mounted")
				pod := f.CreatePodWithBlockDevice("test-pod-block-encrypted", pvc.Name, []string{"sleep", "3600"})
				f.WaitForPodRunning(pod.Name)

				By("Creating an EncryptionKeyRotationJob while block volume is mounted")
				ekrJob := f.CreateEncryptionKeyRotationJob("test-ekrjob-block-mounted", pvc.Name)

				By("Waiting for EncryptionKeyRotationJob to complete")
				ekrJob = f.WaitForEncryptionKeyRotationJobComplete(ekrJob.Name)

				By("Verifying EncryptionKeyRotationJob succeeded")
				Expect(ekrJob.Status.Result).To(Equal(csiaddonsv1alpha1.OperationResultSucceeded))

				By("Cleaning up pod")
				f.DeletePod(pod.Name)
				f.WaitForPodDeleted(pod.Name)
			})
		})
	})

	Context("EncryptionKeyRotationCronJob", func() {
		It("should create EncryptionKeyRotationJobs on schedule for filesystem PVC", func() {
			By("Creating a filesystem PVC for cron job testing")
			pvc := f.CreatePVC("test-pvc-cronjob-fs", "", "")
			pvc = f.WaitForPVCBound(pvc.Name)
			Expect(pvc.Status.Phase).To(Equal(corev1.ClaimBound))

			By("Creating an EncryptionKeyRotationCronJob with 1-minute schedule")
			cronJob := f.CreateEncryptionKeyRotationCronJob("test-ekr-cronjob-fs", pvc.Name, "*/1 * * * *")
			Expect(cronJob).NotTo(BeNil())

			By("Waiting for the first EncryptionKeyRotationJob to be created")
			labelSelector := map[string]string{
				"encryptionkeyrotationcronjob": cronJob.Name,
			}
			createdJob := f.WaitForEncryptionKeyRotationJobCreation(labelSelector, 90*time.Second)
			Expect(createdJob).NotTo(BeNil())
			Expect(createdJob.Spec.Target.PersistentVolumeClaim).To(Equal(pvc.Name))

			By("Waiting for the EncryptionKeyRotationJob to complete")
			completedJob := f.WaitForEncryptionKeyRotationJobComplete(createdJob.Name)
			Expect(completedJob.Status.Result).To(Equal(csiaddonsv1alpha1.OperationResultSucceeded))

			By("Verifying CronJob status is updated")
			updatedCronJob := f.GetEncryptionKeyRotationCronJob(cronJob.Name)
			Expect(updatedCronJob.Status.LastScheduleTime).NotTo(BeNil())
		})

		It("should create EncryptionKeyRotationJobs on schedule for block PVC", func() {
			By("Creating a block mode PVC for cron job testing")
			pvc := f.CreateBlockPVC("test-pvc-cronjob-block", "", "")
			pvc = f.WaitForPVCBound(pvc.Name)
			Expect(pvc.Status.Phase).To(Equal(corev1.ClaimBound))

			By("Creating an EncryptionKeyRotationCronJob with 1-minute schedule")
			cronJob := f.CreateEncryptionKeyRotationCronJob("test-ekr-cronjob-block", pvc.Name, "*/1 * * * *")
			Expect(cronJob).NotTo(BeNil())

			By("Waiting for the first EncryptionKeyRotationJob to be created")
			labelSelector := map[string]string{
				"encryptionkeyrotationcronjob": cronJob.Name,
			}
			createdJob := f.WaitForEncryptionKeyRotationJobCreation(labelSelector, 90*time.Second)
			Expect(createdJob).NotTo(BeNil())
			Expect(createdJob.Spec.Target.PersistentVolumeClaim).To(Equal(pvc.Name))

			By("Waiting for the EncryptionKeyRotationJob to complete")
			completedJob := f.WaitForEncryptionKeyRotationJobComplete(createdJob.Name)
			Expect(completedJob.Status.Result).To(Equal(csiaddonsv1alpha1.OperationResultSucceeded))

			By("Verifying CronJob status is updated")
			updatedCronJob := f.GetEncryptionKeyRotationCronJob(cronJob.Name)
			Expect(updatedCronJob.Status.LastScheduleTime).NotTo(BeNil())
		})
	})
})
