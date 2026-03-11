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

package encryptionkeyrotation_test

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

func TestEncryptionKeyRotation(t *testing.T) {
	// Parse flags
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load E2E configuration: %v", err)
	}

	// Skip if EncryptionKeyRotation tests are disabled
	if !cfg.Tests.EncryptionKeyRotation {
		t.Skip("EncryptionKeyRotation tests are disabled in configuration")
	}

	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "EncryptionKeyRotation E2E Suite")
}

var _ = ginkgo.Describe("EncryptionKeyRotation", ginkgo.Ordered, func() {
	var (
		f *framework.Framework
	)

	ginkgo.BeforeAll(func() {
		f = framework.NewFramework("encryptionkeyrotation-e2e")
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

	ginkgo.Context("EncryptionKeyRotationJob", func() {
		ginkgo.Context("Filesystem mode PVC", func() {
			ginkgo.It("should rotate key on mounted filesystem volume", func() {
				ginkgo.By("Creating an encrypted filesystem PVC")
				pvc := f.CreatePVC("test-pvc-fs-mounted-encrypted", "", f.GetEncryptionKeyRotationStorageClassName())
				pvc = f.WaitForPVCBound(pvc.Name)

				ginkgo.By("Creating a pod that keeps the filesystem volume mounted")
				pod := f.CreatePod("test-pod-fs-encrypted", pvc.Name, []string{"sleep", "3600"})
				f.WaitForPodRunning(pod.Name)

				ginkgo.By("Creating an EncryptionKeyRotationJob while filesystem volume is mounted")
				ekrJob := f.CreateEncryptionKeyRotationJob("test-ekrjob-fs-mounted", pvc.Name)

				ginkgo.By("Waiting for EncryptionKeyRotationJob to complete")
				ekrJob = f.WaitForEncryptionKeyRotationJobComplete(ekrJob.Name)

				ginkgo.By("Verifying EncryptionKeyRotationJob succeeded")
				gomega.Expect(ekrJob.Status.Result).To(gomega.Equal(csiaddonsv1alpha1.OperationResultSucceeded))

				ginkgo.By("Cleaning up pod")
				f.DeletePod(pod.Name)
				f.WaitForPodDeleted(pod.Name)
			})
		})

		ginkgo.Context("Block mode PVC", func() {
			ginkgo.It("should rotate key on mounted block volume", func() {
				ginkgo.By("Creating an encrypted block mode PVC")
				pvc := f.CreateBlockPVC("test-pvc-block-mounted-encrypted", "", f.GetEncryptionKeyRotationStorageClassName())
				pvc = f.WaitForPVCBound(pvc.Name)

				ginkgo.By("Creating a pod that keeps the block volume mounted")
				pod := f.CreatePodWithBlockDevice("test-pod-block-encrypted", pvc.Name, []string{"sleep", "3600"})
				f.WaitForPodRunning(pod.Name)

				ginkgo.By("Creating an EncryptionKeyRotationJob while block volume is mounted")
				ekrJob := f.CreateEncryptionKeyRotationJob("test-ekrjob-block-mounted", pvc.Name)

				ginkgo.By("Waiting for EncryptionKeyRotationJob to complete")
				ekrJob = f.WaitForEncryptionKeyRotationJobComplete(ekrJob.Name)

				ginkgo.By("Verifying EncryptionKeyRotationJob succeeded")
				gomega.Expect(ekrJob.Status.Result).To(gomega.Equal(csiaddonsv1alpha1.OperationResultSucceeded))

				ginkgo.By("Cleaning up pod")
				f.DeletePod(pod.Name)
				f.WaitForPodDeleted(pod.Name)
			})
		})
	})

	ginkgo.Context("EncryptionKeyRotationCronJob", func() {
		ginkgo.It("should create EncryptionKeyRotationJobs on schedule for filesystem PVC", func() {
			ginkgo.By("Creating a filesystem PVC for cron job testing")
			pvc := f.CreatePVC("test-pvc-cronjob-fs", "", f.GetEncryptionKeyRotationStorageClassName())
			pvc = f.WaitForPVCBound(pvc.Name)
			gomega.Expect(pvc.Status.Phase).To(gomega.Equal(corev1.ClaimBound))

			ginkgo.By("Creating a pod that mounts the filesystem volume")
			pod := f.CreatePod("test-pod-cronjob-fs-encrypted", pvc.Name, []string{"sleep", "3600"})
			f.WaitForPodRunning(pod.Name)

			ginkgo.By("Creating an EncryptionKeyRotationCronJob with 1-minute schedule")
			cronJob := f.CreateEncryptionKeyRotationCronJob("test-ekr-cronjob-fs", pvc.Name, "*/1 * * * *")
			gomega.Expect(cronJob).NotTo(gomega.BeNil())

			ginkgo.By("Waiting for the first EncryptionKeyRotationJob to be created")
			createdJob := f.WaitForEncryptionKeyRotationJobCreation(cronJob.Name, 3*time.Minute)
			gomega.Expect(createdJob).NotTo(gomega.BeNil())
			gomega.Expect(createdJob.Spec.Target.PersistentVolumeClaim).To(gomega.Equal(pvc.Name))

			ginkgo.By("Waiting for the EncryptionKeyRotationJob to complete")
			completedJob := f.WaitForEncryptionKeyRotationJobComplete(createdJob.Name)
			gomega.Expect(completedJob.Status.Result).To(gomega.Equal(csiaddonsv1alpha1.OperationResultSucceeded))

			ginkgo.By("Verifying CronJob status is updated")
			updatedCronJob := f.GetEncryptionKeyRotationCronJob(cronJob.Name)
			gomega.Expect(updatedCronJob.Status.LastScheduleTime).NotTo(gomega.BeNil())
		})

		ginkgo.It("should create EncryptionKeyRotationJobs on schedule for block PVC", func() {
			ginkgo.By("Creating a block mode PVC for cron job testing")
			pvc := f.CreateBlockPVC("test-pvc-cronjob-block", "", f.GetEncryptionKeyRotationStorageClassName())
			pvc = f.WaitForPVCBound(pvc.Name)
			gomega.Expect(pvc.Status.Phase).To(gomega.Equal(corev1.ClaimBound))

			ginkgo.By("Creating a pod that mounts the block volume")
			pod := f.CreatePodWithBlockDevice("test-pod-block-encrypted", pvc.Name, []string{"sleep", "3600"})
			f.WaitForPodRunning(pod.Name)

			ginkgo.By("Creating an EncryptionKeyRotationCronJob with 1-minute schedule")
			cronJob := f.CreateEncryptionKeyRotationCronJob("test-ekr-cronjob-block", pvc.Name, "*/1 * * * *")
			gomega.Expect(cronJob).NotTo(gomega.BeNil())

			ginkgo.By("Waiting for the first EncryptionKeyRotationJob to be created")
			createdJob := f.WaitForEncryptionKeyRotationJobCreation(cronJob.Name, 3*time.Minute)
			gomega.Expect(createdJob).NotTo(gomega.BeNil())
			gomega.Expect(createdJob.Spec.Target.PersistentVolumeClaim).To(gomega.Equal(pvc.Name))

			ginkgo.By("Waiting for the EncryptionKeyRotationJob to complete")
			completedJob := f.WaitForEncryptionKeyRotationJobComplete(createdJob.Name)
			gomega.Expect(completedJob.Status.Result).To(gomega.Equal(csiaddonsv1alpha1.OperationResultSucceeded))

			ginkgo.By("Verifying CronJob status is updated")
			updatedCronJob := f.GetEncryptionKeyRotationCronJob(cronJob.Name)
			gomega.Expect(updatedCronJob.Status.LastScheduleTime).NotTo(gomega.BeNil())
		})
	})
})
