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
	"fmt"

	"k8s.io/apimachinery/pkg/types"

	volumegroupv1 "github.com/csi-addons/kubernetes-csi-addons/apis/volumegroup.storage/v1"
	"github.com/csi-addons/kubernetes-csi-addons/controllers/volumegroup.storage/pkg/messages"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetVG(client client.Client, logger logr.Logger, vgName string, vgNamespace string) (*volumegroupv1.VolumeGroup, error) {
	logger.Info(fmt.Sprintf(messages.GetVG, vgName, vgNamespace))
	vg := &volumegroupv1.VolumeGroup{}
	namespacedVG := types.NamespacedName{Name: vgName, Namespace: vgNamespace}
	err := client.Get(context.TODO(), namespacedVG, vg)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Error(err, "VolumeGroup not found", "VolumeGroup Name", vgName)
		}
		return nil, err
	}
	return vg, nil
}

func IsVgExist(client client.Client, logger logr.Logger, vgc *volumegroupv1.VolumeGroupContent) (bool, error) {
	if !GetObjectField(vgc.Spec, "VolumeGroupRef").IsNil() {
		if vg, err := GetVG(client, logger, vgc.Spec.VolumeGroupRef.Name, vgc.Spec.VolumeGroupRef.Namespace); err != nil {
			if !errors.IsNotFound(err) {
				return false, err
			}
		} else {
			return vg != nil, nil
		}
	}
	return false, nil
}

func UpdateVGSourceContent(client client.Client, instance *volumegroupv1.VolumeGroup,
	vgcName string, logger logr.Logger) error {
	instance.Spec.Source.VolumeGroupContentName = &vgcName
	if err := UpdateObject(client, instance); err != nil {
		logger.Error(err, "failed to update source", "VGName", instance.Name)
		return err
	}
	return nil
}

func updateVGStatus(client client.Client, vg *volumegroupv1.VolumeGroup, logger logr.Logger) error {
	logger.Info(fmt.Sprintf(messages.UpdateVGStatus, vg.Namespace, vg.Name))
	if err := UpdateObjectStatus(client, vg); err != nil {
		if apierrors.IsConflict(err) {
			return err
		}
		logger.Error(err, "failed to update volumeGroup status", "VGName", vg.Name)
		return err
	}
	return nil
}

func UpdateVGStatus(client client.Client, vg *volumegroupv1.VolumeGroup, vgcName string,
	groupCreationTime *metav1.Time, ready bool, logger logr.Logger) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		vg.Status.BoundVolumeGroupContentName = &vgcName
		vg.Status.GroupCreationTime = groupCreationTime
		vg.Status.Ready = &ready
		vg.Status.Error = nil
		err := vgRetryOnConflictFunc(client, vg, logger)
		return err
	})
	if err != nil {
		return err
	}

	return updateVGStatus(client, vg, logger)
}

func updateVGStatusPVCList(client client.Client, vg *volumegroupv1.VolumeGroup, logger logr.Logger,
	pvcList []corev1.PersistentVolumeClaim) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		vg.Status.PVCList = pvcList
		err := vgRetryOnConflictFunc(client, vg, logger)
		return err
	})
	if err != nil {
		return err
	}

	return nil
}

func UpdateVGStatusError(client client.Client, vg *volumegroupv1.VolumeGroup, logger logr.Logger, message string) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		vg.Status.Error = &volumegroupv1.VolumeGroupError{Message: &message}
		err := vgRetryOnConflictFunc(client, vg, logger)
		return err
	})
	if err != nil {
		return err
	}

	return nil
}

func vgRetryOnConflictFunc(client client.Client, vg *volumegroupv1.VolumeGroup, logger logr.Logger) error {
	err := updateVGStatus(client, vg, logger)
	if apierrors.IsConflict(err) {
		uErr := getNamespacedObject(client, vg)
		if uErr != nil {
			return uErr
		}
		logger.Info(fmt.Sprintf(messages.RetryUpdateVGStatus, vg.Namespace, vg.Name))
	}
	return err
}

func GetVGList(logger logr.Logger, client client.Client, driver string) (volumegroupv1.VolumeGroupList, error) {
	logger.Info(messages.ListVGs)
	vg := &volumegroupv1.VolumeGroupList{}
	err := client.List(context.TODO(), vg)
	if err != nil {
		return volumegroupv1.VolumeGroupList{}, err
	}
	vgList, err := getProvisionedVGs(logger, client, vg, driver)
	if err != nil {
		return volumegroupv1.VolumeGroupList{}, err
	}
	return vgList, nil
}

func getProvisionedVGs(logger logr.Logger, client client.Client, vgList *volumegroupv1.VolumeGroupList,
	driver string) (volumegroupv1.VolumeGroupList, error) {
	newVgList := volumegroupv1.VolumeGroupList{}
	for _, vg := range vgList.Items {
		isVGHasMatchingDriver, err := isVGHasMatchingDriver(logger, client, vg, driver)
		if err != nil {
			return volumegroupv1.VolumeGroupList{}, err
		}
		if isVGHasMatchingDriver {
			newVgList.Items = append(newVgList.Items, vg)
		}
	}
	return newVgList, nil
}

func isVGHasMatchingDriver(logger logr.Logger, client client.Client, vg volumegroupv1.VolumeGroup,
	driver string) (bool, error) {
	vgClassDriver, err := getVGClassDriver(client, logger, GetStringField(vg.Spec, "VolumeGroupClassName"))
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return vgClassDriver == driver, nil
}
func IsPVCMatchesVG(logger logr.Logger, pvc *corev1.PersistentVolumeClaim, vg volumegroupv1.VolumeGroup) (bool, error) {

	logger.Info(fmt.Sprintf(messages.CheckIfPVCMatchesVG,
		pvc.Namespace, pvc.Name, vg.Namespace, vg.Name))
	areLabelsMatchLabelSelector, err := areLabelsMatchLabelSelector(pvc.ObjectMeta.Labels, *vg.Spec.Source.Selector)

	if areLabelsMatchLabelSelector {
		logger.Info(fmt.Sprintf(messages.PVCMatchedToVG,
			pvc.Namespace, pvc.Name, vg.Namespace, vg.Name))
		return true, err
	} else {
		logger.Info(fmt.Sprintf(messages.PVCNotMatchedToVG,
			pvc.Namespace, pvc.Name, vg.Namespace, vg.Name))
		return false, err
	}
}

func RemovePVCFromVG(logger logr.Logger, client client.Client, pvc *corev1.PersistentVolumeClaim, vg *volumegroupv1.VolumeGroup) error {
	logger.Info(fmt.Sprintf(messages.RemovePVCFromVG,
		pvc.Namespace, pvc.Name, vg.Namespace, vg.Name))
	vg.Status.PVCList = removeFromPVCList(pvc, vg.Status.PVCList)
	err := updateVGStatusPVCList(client, vg, logger, vg.Status.PVCList)
	if err != nil {
		vg.Status.PVCList = appendPVC(vg.Status.PVCList, *pvc)
		logger.Error(err, fmt.Sprintf(messages.FailedToRemovePVCFromVG,
			pvc.Namespace, pvc.Name, vg.Namespace, vg.Name))
		return err
	}
	logger.Info(fmt.Sprintf(messages.RemovedPVCFromVG,
		pvc.Namespace, pvc.Name, vg.Namespace, vg.Name))
	return nil
}

func removeFromPVCList(pvc *corev1.PersistentVolumeClaim, pvcList []corev1.PersistentVolumeClaim) []corev1.PersistentVolumeClaim {
	for index, pvcFromList := range pvcList {
		if pvcFromList.Name == pvc.Name && pvcFromList.Namespace == pvc.Namespace {
			pvcList = removeByIndexFromPVCList(pvcList, index)
			return pvcList
		}
	}
	return pvcList
}

func getVgId(logger logr.Logger, client client.Client, vg *volumegroupv1.VolumeGroup) (string, error) {
	vgc, err := GetVGC(client, logger, GetStringField(vg.Spec.Source, "VolumeGroupContentName"), vg.Namespace)
	if err != nil {
		return "", err
	}
	return string(vgc.Spec.Source.VolumeGroupHandle), nil
}

func AddPVCToVG(logger logr.Logger, client client.Client, pvc *corev1.PersistentVolumeClaim, vg *volumegroupv1.VolumeGroup) error {
	logger.Info(fmt.Sprintf(messages.AddPVCToVG,
		pvc.Namespace, pvc.Name, vg.Namespace, vg.Name))
	vg.Status.PVCList = appendPVC(vg.Status.PVCList, *pvc)
	err := updateVGStatusPVCList(client, vg, logger, vg.Status.PVCList)
	if err != nil {
		vg.Status.PVCList = removeFromPVCList(pvc, vg.Status.PVCList)
		logger.Error(err, fmt.Sprintf(messages.FailedToAddPVCToVG,
			pvc.Namespace, pvc.Name, vg.Namespace, vg.Name))
		return err
	}
	logger.Info(fmt.Sprintf(messages.AddedPVCToVG,
		pvc.Namespace, pvc.Name, vg.Namespace, vg.Name))
	return nil
}

func appendPVC(pvcListInVG []corev1.PersistentVolumeClaim, pvc corev1.PersistentVolumeClaim) []corev1.PersistentVolumeClaim {
	for _, pvcFromList := range pvcListInVG {
		if pvcFromList.Name == pvc.Name && pvcFromList.Namespace == pvc.Namespace {
			return pvcListInVG
		}
	}
	pvcListInVG = append(pvcListInVG, pvc)
	return pvcListInVG
}

func IsPVCPartAnyVG(pvc *corev1.PersistentVolumeClaim, vgs []volumegroupv1.VolumeGroup) bool {
	for _, vg := range vgs {
		if IsPVCInPVCList(pvc, vg.Status.PVCList) {
			return true
		}
	}
	return false
}

func IsPVCInPVCList(pvc *corev1.PersistentVolumeClaim, pvcList []corev1.PersistentVolumeClaim) bool {
	for _, pvcFromList := range pvcList {
		if pvcFromList.Name == pvc.Name && pvcFromList.Namespace == pvc.Namespace {
			return true
		}
	}
	return false
}
