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
	"fmt"
	"strings"

	volumegroupv1 "github.com/csi-addons/kubernetes-csi-addons/apis/volumegroup.storage/v1"
	commonUtils "github.com/csi-addons/kubernetes-csi-addons/controllers/volumegroup.storage/common/utils"
	"github.com/csi-addons/kubernetes-csi-addons/controllers/volumegroup.storage/pkg/messages"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func AddFinalizerToVG(client runtimeclient.Client, logger logr.Logger, vg *volumegroupv1.VolumeGroup) error {
	if !commonUtils.Contains(vg.ObjectMeta.Finalizers, VGFinalizer) {
		logger.Info("adding finalizer to VolumeGroup object", "Finalizer", VGFinalizer)
		vg.ObjectMeta.Finalizers = append(vg.ObjectMeta.Finalizers, VGFinalizer)
		if err := updateFinalizer(logger, client, vg.ObjectMeta.Finalizers, vg); err != nil {
			logger.Error(err, "failed to add finalizer to volumeGroup resource", "finalizer", VGFinalizer)
			return err
		}
	}

	return nil
}

func AddFinalizerToVGC(client runtimeclient.Client, logger logr.Logger, vgc *volumegroupv1.VolumeGroupContent) error {
	if !commonUtils.Contains(vgc.ObjectMeta.Finalizers, VgcFinalizer) {
		logger.Info("adding finalizer to volumeGroupContent object", "Name", vgc.Name, "Finalizer", VgcFinalizer)
		vgc.ObjectMeta.Finalizers = append(vgc.ObjectMeta.Finalizers, VgcFinalizer)
		if err := updateFinalizer(logger, client, vgc.ObjectMeta.Finalizers, vgc); err != nil {
			logger.Error(err, "failed to add finalizer to volumeGroupContent resource", "finalizer", VGFinalizer)
			return err
		}
	}

	return nil
}

func RemoveFinalizerFromVG(client runtimeclient.Client, logger logr.Logger, vg *volumegroupv1.VolumeGroup) error {
	if commonUtils.Contains(vg.ObjectMeta.Finalizers, VGFinalizer) {
		logger.Info("removing finalizer from VolumeGroup object", "Finalizer", VGFinalizer)
		vg.ObjectMeta.Finalizers = commonUtils.Remove(vg.ObjectMeta.Finalizers, VGFinalizer)
		if err := updateFinalizer(logger, client, vg.ObjectMeta.Finalizers, vg); err != nil {
			logger.Error(err, "failed to remove finalizer to VolumeGroup resource", "finalizer", VGFinalizer)
			return err
		}
	}

	return nil
}

func RemoveFinalizerFromVGC(client runtimeclient.Client, logger logr.Logger, vgc *volumegroupv1.VolumeGroupContent) error {
	if commonUtils.Contains(vgc.ObjectMeta.Finalizers, VgcFinalizer) {
		logger.Info("removing finalizer from VolumeGroupContent object", "Name", vgc.Name, "Finalizer", VgcFinalizer)
		vgc.ObjectMeta.Finalizers = commonUtils.Remove(vgc.ObjectMeta.Finalizers, VgcFinalizer)
		if err := updateFinalizer(logger, client, vgc.ObjectMeta.Finalizers, vgc); err != nil {
			logger.Error(err, "failed to remove finalizer to VolumeGroupContent resource", "finalizer", VGFinalizer)
			return err
		}
	}

	return nil
}

func AddFinalizerToPVC(client runtimeclient.Client, logger logr.Logger, pvc *corev1.PersistentVolumeClaim) error {
	if !commonUtils.Contains(pvc.ObjectMeta.Finalizers, pvcVGFinalizer) {
		logger.Info("adding finalizer to PersistentVolumeClaim object", "Namespace", pvc.Namespace, "Name", pvc.Name, "Finalizer", pvcVGFinalizer)
		pvc.ObjectMeta.Finalizers = append(pvc.ObjectMeta.Finalizers, pvcVGFinalizer)
		if err := updateFinalizer(logger, client, pvc.ObjectMeta.Finalizers, pvc); err != nil {
			logger.Error(err, "failed to add finalizer to PersistentVolumeClaim resource", "finalizer", VGFinalizer)
			return err
		}
	}

	return nil
}

func RemoveFinalizerFromPVC(client runtimeclient.Client, logger logr.Logger, driver string,
	pvc *corev1.PersistentVolumeClaim) error {
	removeFinalizer, err := isFinalizerShouldBeREmovedFromPVC(logger, client, driver, pvc)
	if err != nil {
		return err
	}

	if removeFinalizer {
		logger.Info("removing finalizer from PersistentVolumeClaim object", "Namespace", pvc.Namespace, "Name", pvc.Name, "Finalizer", pvcVGFinalizer)
		uErr := getNamespacedObject(client, pvc)
		if uErr != nil {
			return uErr
		}
		pvc.ObjectMeta.Finalizers = commonUtils.Remove(pvc.ObjectMeta.Finalizers, pvcVGFinalizer)
		if err := updateFinalizer(logger, client, pvc.ObjectMeta.Finalizers, pvc); err != nil {
			logger.Error(err, "failed to remove finalizer to PersistentVolumeClaim resource", "finalizer", VGFinalizer)
			return err
		}
	}

	return nil
}

func isFinalizerShouldBeREmovedFromPVC(logger logr.Logger, client runtimeclient.Client, driver string,
	pvc *corev1.PersistentVolumeClaim) (bool, error) {
	pvc, err := GetPVC(logger, client, pvc.Name, pvc.Namespace)
	if err != nil {
		return false, err
	}
	vgList, err := GetVGList(logger, client, driver)
	if err != nil {
		return false, err
	}
	return !IsPVCPartAnyVG(pvc, vgList.Items) && commonUtils.Contains(pvc.ObjectMeta.Finalizers, pvcVGFinalizer), nil
}

func updateFinalizer(logger logr.Logger, client runtimeclient.Client,
	finalizers []string, obj runtimeclient.Object) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return finalizerRetryOnConflictFunc(logger, client, finalizers, obj)
	})
	return err
}

func finalizerRetryOnConflictFunc(logger logr.Logger, client runtimeclient.Client,
	finalizers []string, obj runtimeclient.Object) error {
	obj.SetFinalizers(finalizers)
	err := UpdateObject(client, obj)
	if apierrors.IsConflict(err) {
		uErr := getNamespacedObject(client, obj)
		if uErr != nil {
			return uErr
		}
		logger.Info(fmt.Sprintf(messages.RetryUpdateFinalizer))
	}
	return err
}

func IsContainOtherFinalizers(object runtimeclient.Object, logger logr.Logger) bool {
	finalizers := object.GetFinalizers()
	for _, finalizer := range finalizers {
		if !strings.Contains(finalizer, VGFinalizer) {
			logger.Info(fmt.Sprintf(messages.NonVolumeGroupFinalizers, object.GetObjectKind().GroupVersionKind().Kind, object.GetName()))
			return true
		}
	}
	return false
}
