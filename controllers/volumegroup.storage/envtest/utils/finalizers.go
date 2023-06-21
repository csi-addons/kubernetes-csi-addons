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

package utils

import (
	"context"

	commonUtils "github.com/csi-addons/kubernetes-csi-addons/controllers/volumegroup.storage/common/utils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func RemoveResourceObjectFinalizers(name, namespace string, obj runtimeclient.Object, client runtimeclient.Client) error {
	err := GetNamespacedResourceObject(name, namespace, obj, client)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if err == nil {
		obj.SetFinalizers([]string{})
		err := client.Update(context.TODO(), obj)
		if err != nil {
			return err
		}
	}
	return nil
}

func RemoveFinalizerFromPVC(name, namespace, finalizer string, client runtimeclient.Client) error {
	pvc := &corev1.PersistentVolumeClaim{}
	if err := GetNamespacedResourceObject(name, namespace, pvc, client); err != nil {
		return err
	}
	if commonUtils.Contains(pvc.ObjectMeta.Finalizers, finalizer) {
		pvc.ObjectMeta.Finalizers = commonUtils.Remove(pvc.ObjectMeta.Finalizers, finalizer)
	}
	err := client.Update(context.TODO(), pvc)
	return err
}
