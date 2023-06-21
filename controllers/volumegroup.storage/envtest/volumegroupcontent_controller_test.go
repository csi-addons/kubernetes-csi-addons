/*
Copyright 2023 The Kubernetes-CSI-Addons Authors.

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

package envtest

import (
	"context"
	"time"

	volumegroupv1 "github.com/csi-addons/kubernetes-csi-addons/apis/volumegroup.storage/v1"
	"github.com/csi-addons/kubernetes-csi-addons/controllers/volumegroup.storage/envtest/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

var _ = Describe("Test controllers", func() {
	Context("Test VGC controllers", func() {

		BeforeEach(func() {
			err := cleanTestNamespace()
			Expect(err).ToNot(HaveOccurred())
		})
		It("Should not delete vgc when vgclass deletion policy is retain", func(done Done) {
			By("Creating a volumeGroup resources and set VGClass deletion policy to retain")
			err := utils.CreateResourceObject(Secret, k8sClient)
			Expect(err).NotTo(HaveOccurred())

			err = createVolumeGroupObjects(volumegroupv1.VolumeGroupContentRetain)
			Expect(err).NotTo(HaveOccurred())

			vgObj := &volumegroupv1.VolumeGroup{}
			err = utils.GetNamespacedResourceObject(VGName, Namespace, vgObj, k8sClient)
			Expect(err).NotTo(HaveOccurred())
			time.Sleep(1 * time.Second)

			By("Deleting VG")
			err = k8sClient.Delete(context.TODO(), vgObj)
			Expect(err).NotTo(HaveOccurred())
			time.Sleep(1 * time.Second)

			By("Validating VG deleted")
			vgErr := utils.GetNamespacedResourceObject(VGName, Namespace, vgObj, k8sClient)
			Expect(apierrors.IsNotFound(vgErr)).To(BeTrue())

			By("Validating VGC has not been deleted")
			vgcName := utils.GetVGCName(vgObj.GetUID())
			vgcObj := &volumegroupv1.VolumeGroupContent{}
			vgcErr := utils.GetNamespacedResourceObject(vgcName, Namespace, vgcObj, k8sClient)
			Expect(vgcErr).NotTo(HaveOccurred())

			close(done)
		}, Timeout.Seconds())
		It("Should delete pvcs when deleting vg", func(done Done) {
			By("Creating a volumeGroup and volume resources")
			err := createNonVolumeK8SResources()
			Expect(err).NotTo(HaveOccurred())
			err = createVolumeGroupObjects(volumegroupv1.VolumeGroupContentDelete)
			Expect(err).NotTo(HaveOccurred())
			err = createVolumeObjects()
			Expect(err).NotTo(HaveOccurred())
			err = utils.RemoveFinalizerFromPVC(PVCName, Namespace, PVCProtectionFinalizer, k8sClient)
			Expect(err).NotTo(HaveOccurred())

			vgObj := &volumegroupv1.VolumeGroup{}
			err = utils.GetNamespacedResourceObject(VGName, Namespace, vgObj, k8sClient)
			Expect(err).NotTo(HaveOccurred())
			time.Sleep(1 * time.Second)

			By("Deleting VG")
			err = k8sClient.Delete(context.TODO(), vgObj)
			Expect(err).NotTo(HaveOccurred())
			time.Sleep(1 * time.Second)

			By("Validating that VG deleted")
			vgErr := utils.GetNamespacedResourceObject(VGName, Namespace, vgObj, k8sClient)
			Expect(apierrors.IsNotFound(vgErr)).To(BeTrue())

			By("Validating that VGC deleted")
			vgcName := utils.GetVGCName(vgObj.GetUID())
			vgcObj := &volumegroupv1.VolumeGroupContent{}
			vgcErr := utils.GetNamespacedResourceObject(vgcName, Namespace, vgcObj, k8sClient)
			Expect(apierrors.IsNotFound(vgcErr)).To(BeTrue())

			By("Validating that PVC has been deleted")
			pvcObj := &corev1.PersistentVolumeClaim{}
			pvcErr := utils.GetNamespacedResourceObject(PVCName, Namespace, pvcObj, k8sClient)
			Expect(apierrors.IsNotFound(pvcErr)).To(BeTrue())

			close(done)
		}, Timeout.Seconds())
	})
})
