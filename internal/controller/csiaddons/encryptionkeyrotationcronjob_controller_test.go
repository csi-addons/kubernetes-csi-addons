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

package controller

import (
	"context"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/csiaddons/v1alpha1"

	"github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = ginkgo.Describe("EncryptionKeyRotationCronJob Controller", func() {
	ginkgo.Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		encryptionkeyrotationcronjob := &csiaddonsv1alpha1.EncryptionKeyRotationCronJob{}

		ginkgo.BeforeEach(func() {
			ginkgo.By("creating the custom resource for the Kind EncryptionKeyRotationCronJob")
			err := k8sClient.Get(ctx, typeNamespacedName, encryptionkeyrotationcronjob)
			if err != nil && errors.IsNotFound(err) {
				resource := &csiaddonsv1alpha1.EncryptionKeyRotationCronJob{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: csiaddonsv1alpha1.EncryptionKeyRotationCronJobSpec{
						Schedule: "@weekly",
						JobSpec: csiaddonsv1alpha1.EncryptionKeyRotationJobTemplateSpec{
							Spec: csiaddonsv1alpha1.EncryptionKeyRotationJobSpec{
								Target: csiaddonsv1alpha1.TargetSpec{
									PersistentVolumeClaim: "rbd-pvc",
								},
							},
						},
					},
				}
				gomega.Expect(k8sClient.Create(ctx, resource)).To(gomega.Succeed())
			}
		})

		ginkgo.AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &csiaddonsv1alpha1.EncryptionKeyRotationCronJob{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			ginkgo.By("Cleanup the specific resource instance EncryptionKeyRotationCronJob")
			gomega.Expect(k8sClient.Delete(ctx, resource)).To(gomega.Succeed())
		})
	})
})
