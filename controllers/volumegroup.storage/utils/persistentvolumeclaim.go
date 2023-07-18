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

	volumegroupv1 "github.com/csi-addons/kubernetes-csi-addons/apis/volumegroup.storage/v1"
	"github.com/csi-addons/kubernetes-csi-addons/controllers/volumegroup.storage/pkg/messages"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func getPVCListVolumeIds(logger logr.Logger, client runtimeclient.Client, pvcList []corev1.PersistentVolumeClaim) ([]string, error) {
	volumeIds := []string{}
	for _, pvc := range pvcList {
		pv, err := GetPVFromPVC(logger, client, &pvc)
		if err != nil {
			return nil, err
		}
		if pv != nil {
			volumeIds = append(volumeIds, string(pv.Spec.CSI.VolumeHandle))
		}
	}
	return volumeIds, nil
}

func IsPVCCanBeAddedToVG(logger logr.Logger, pvc *corev1.PersistentVolumeClaim, vgs []volumegroupv1.VolumeGroup) error {
	vgsWithPVC := []string{}
	newVGsForPVC := []string{}
	for _, vg := range vgs {
		if IsPVCInPVCList(pvc, vg.Status.PVCList) {
			vgsWithPVC = append(vgsWithPVC, vg.Name)
		} else if isPVCMatchesVG, _ := IsPVCMatchesVG(logger, pvc, vg); isPVCMatchesVG {
			newVGsForPVC = append(newVGsForPVC, vg.Name)
		}
	}
	return checkIfPVCCanBeAddedToVG(logger, pvc, vgsWithPVC, newVGsForPVC)
}

func checkIfPVCCanBeAddedToVG(logger logr.Logger, pvc *corev1.PersistentVolumeClaim, vgsWithPVC, newVGsForPVC []string) error {
	if len(vgsWithPVC) > 0 && len(newVGsForPVC) > 0 {
		message := fmt.Sprintf(messages.PVCIsAlreadyBelongToGroup, pvc.Namespace, pvc.Name, newVGsForPVC, vgsWithPVC)
		logger.Info(message)
		return fmt.Errorf(message)
	}
	if len(newVGsForPVC) > 1 {
		message := fmt.Sprintf(messages.PVCMatchedWithMultipleNewGroups, pvc.Namespace, pvc.Name, newVGsForPVC)
		logger.Info(message)
		return fmt.Errorf(message)
	}
	return nil
}

func IsPVCNeedToBeHandled(reqLogger logr.Logger, pvc *corev1.PersistentVolumeClaim, client runtimeclient.Client, driverName string) (bool, error) {
	isPVCHasMatchingDriver, err := IsPVCHasMatchingDriver(reqLogger, client, pvc, driverName)
	if err != nil {
		return false, err
	}
	if !isPVCHasMatchingDriver {
		return false, nil
	}
	if pvc.Status.Phase != corev1.ClaimBound {
		reqLogger.Info(messages.PVCIsNotInBoundPhase)
		return false, nil
	}
	isSCHasVGParam, err := IsPVCInStaticVG(reqLogger, client, pvc)
	if err != nil {
		return false, err
	}
	if isSCHasVGParam {
		storageClassName, sErr := GetPVCClass(pvc)
		if sErr != nil {
			return false, sErr
		}
		msg := fmt.Sprintf(messages.StorageClassHasVGParameter, storageClassName, pvc.Namespace, pvc.Name)
		reqLogger.Info(msg)
		mErr := fmt.Errorf(msg)
		err = HandlePVCErrorMessage(reqLogger, client, pvc, mErr, addingPVC)
		if err != nil {
			return false, err
		}
		return false, nil
	}
	return true, nil
}

func IsPVCInStaticVG(logger logr.Logger, client runtimeclient.Client, pvc *corev1.PersistentVolumeClaim) (bool, error) {
	storageClassName, sErr := GetPVCClass(pvc)
	if sErr != nil {
		return false, sErr
	}
	sc, err := getStorageClass(logger, client, storageClassName)
	if err != nil {
		return false, err
	}
	return isSCHasParam(sc, storageClassVGParameter), nil
}

func getMatchingPVCFromPVCListToPV(logger logr.Logger, client runtimeclient.Client,
	pvName, driver string) (corev1.PersistentVolumeClaim, error) {
	pvcList, err := GetPVCList(logger, client, driver)
	if err != nil {
		return corev1.PersistentVolumeClaim{}, err
	}
	for _, pvc := range pvcList.Items {
		pvNameFromPVC := getPVNameFromPVC(&pvc)
		if pvNameFromPVC == pvName {
			return pvc, nil
		}
	}
	return corev1.PersistentVolumeClaim{}, nil
}

func GetPVCList(logger logr.Logger, client runtimeclient.Client, driver string) (corev1.PersistentVolumeClaimList, error) {
	pvcList, err := getPVCList(logger, client)
	if err != nil {
		return corev1.PersistentVolumeClaimList{}, err
	}
	boundPVCList, err := getBoundPVCList(pvcList)
	if err != nil {
		return corev1.PersistentVolumeClaimList{}, err
	}

	return getProvisionedPVCList(logger, client, driver, boundPVCList)
}

func getPVCList(logger logr.Logger, client runtimeclient.Client) (corev1.PersistentVolumeClaimList, error) {
	logger.Info(messages.ListPVCs)
	pvcList := &corev1.PersistentVolumeClaimList{}
	if err := client.List(context.TODO(), pvcList); err != nil {
		logger.Error(err, messages.FailedToListPVC)
		return corev1.PersistentVolumeClaimList{}, err
	}
	return *pvcList, nil
}

func getBoundPVCList(pvcList corev1.PersistentVolumeClaimList) (corev1.PersistentVolumeClaimList, error) {
	newPVCList := corev1.PersistentVolumeClaimList{}
	for _, pvc := range pvcList.Items {
		if pvc.Status.Phase == corev1.ClaimBound {
			newPVCList.Items = append(newPVCList.Items, pvc)
		}
	}
	return newPVCList, nil
}

func getProvisionedPVCList(logger logr.Logger, client runtimeclient.Client, driver string,
	pvcList corev1.PersistentVolumeClaimList) (corev1.PersistentVolumeClaimList, error) {
	newPVCList := corev1.PersistentVolumeClaimList{}
	for _, pvc := range pvcList.Items {
		isPVCHasMatchingDriver, err := IsPVCHasMatchingDriver(logger, client, &pvc, driver)
		if err != nil {
			return corev1.PersistentVolumeClaimList{}, err
		}
		if isPVCHasMatchingDriver {
			newPVCList.Items = append(newPVCList.Items, pvc)
		}
	}
	return newPVCList, nil
}

func IsPVCHasMatchingDriver(logger logr.Logger, client runtimeclient.Client,
	pvc *corev1.PersistentVolumeClaim, driver string) (bool, error) {
	storageClassName, sErr := GetPVCClass(pvc)
	if sErr != nil {
		return false, sErr
	}
	scProvisioner, err := getStorageClassProvisioner(logger, client, storageClassName)
	if err != nil {
		return false, err
	}
	return scProvisioner == driver, nil
}

func getPVCNameFromPV(pv corev1.PersistentVolume) string {
	return GetStringField(pv.Spec.ClaimRef, "Name")
}

func getPVCNamespaceFromPV(pv corev1.PersistentVolume) string {
	return GetStringField(pv.Spec.ClaimRef, "Namespace")
}

func deletePVC(logger logr.Logger, client runtimeclient.Client, name, namespace, driver string) error {
	logger.Info(fmt.Sprintf(messages.DeletePVC, namespace, name))
	pvc, err := GetPVC(logger, client, name, namespace)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	err = RemoveFinalizerFromPVC(client, logger, driver, pvc)
	if err != nil {
		return err
	}
	err = removePVCObject(logger, client, pvc)
	if err != nil {
		return err
	}
	return nil
}

func GetPVC(logger logr.Logger, client runtimeclient.Client, name, namespace string) (*corev1.PersistentVolumeClaim, error) {
	logger.Info(fmt.Sprintf(messages.GetPVC, namespace, name))
	pvc := &corev1.PersistentVolumeClaim{}
	namespacedPVC := types.NamespacedName{Name: name, Namespace: namespace}
	err := client.Get(context.TODO(), namespacedPVC, pvc)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Error(err, fmt.Sprintf(messages.PVCNotFound, namespace, name))
		} else {
			logger.Error(err, fmt.Sprintf(messages.UnExpectedPVCError, namespace, name))
		}
		return nil, err
	}
	return pvc, nil
}

func removePVCObject(logger logr.Logger, client runtimeclient.Client, pvc *corev1.PersistentVolumeClaim) error {
	if err := client.Delete(context.TODO(), pvc); err != nil {
		logger.Error(err, fmt.Sprintf(messages.FailToRemovePVCObject, pvc.Namespace, pvc.Name))
		return err
	}
	return nil
}
