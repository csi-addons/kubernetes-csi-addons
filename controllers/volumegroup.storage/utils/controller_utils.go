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
	"math/rand"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	volumegroupv1 "github.com/csi-addons/kubernetes-csi-addons/apis/volumegroup.storage/v1"
	"github.com/csi-addons/kubernetes-csi-addons/controllers/volumegroup.storage/pkg/messages"
	grpcClient "github.com/csi-addons/kubernetes-csi-addons/internal/client"
	"github.com/go-logr/logr"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func UpdateObject(client client.Client, updateObject client.Object) error {
	if err := client.Update(context.TODO(), updateObject); err != nil {
		return fmt.Errorf("failed to update %s (%s/%s) %w", updateObject.GetObjectKind(), updateObject.GetNamespace(), updateObject.GetName(), err)
	}
	return nil
}

func UpdateObjectStatus(client client.Client, updateObject client.Object) error {
	if err := client.Status().Update(context.TODO(), updateObject); err != nil {
		if apierrors.IsConflict(err) {
			return err
		}
		return fmt.Errorf("failed to update %s (%s/%s) status %w", updateObject.GetObjectKind(), updateObject.GetNamespace(), updateObject.GetName(), err)
	}
	return nil
}

func getNamespacedObject(client client.Client, obj client.Object) error {
	namespacedObject := types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}
	err := client.Get(context.TODO(), namespacedObject, obj)
	if err != nil {
		return err
	}
	return nil
}

func GetMessageFromError(err error) string {
	s, ok := status.FromError(err)
	if !ok {
		// This is not gRPC error. The operation must have failed before gRPC
		// method was called, otherwise we would get gRPC error.
		return err.Error()
	}

	return s.Message()
}

func generateString() string {
	rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, 16)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func AddVolumeToPvcListAndPvList(logger logr.Logger, client client.Client,
	pvc *corev1.PersistentVolumeClaim, vg *volumegroupv1.VolumeGroup) error {
	err := AddPVCToVG(logger, client, pvc, vg)
	if err != nil {
		return err
	}

	err = AddMatchingPVToMatchingVGC(logger, client, pvc, vg)
	if err != nil {
		return err
	}

	if err = AddFinalizerToPVC(client, logger, pvc); err != nil {
		return err
	}

	message := fmt.Sprintf(messages.AddedPVCToVG, pvc.Namespace, pvc.Name, vg.Namespace, vg.Name)
	return HandleSuccessMessage(logger, client, vg, message, addingPVC)
}

func RemoveVolumeFromPvcListAndPvList(logger logr.Logger, client client.Client, driver string,
	pvc corev1.PersistentVolumeClaim, vg *volumegroupv1.VolumeGroup) error {
	err := RemovePVCFromVG(logger, client, &pvc, vg)
	if err != nil {
		return err
	}
	pv, err := GetPVFromPVC(logger, client, &pvc)
	if err != nil {
		return err
	}
	vgc, err := GetVGC(client, logger, GetStringField(vg.Spec.Source, "VolumeGroupContentName"), vg.Namespace)
	if err != nil {
		return err
	}

	if pv != nil {
		err = RemovePVFromVGC(logger, client, pv, vgc)
		if err != nil {
			return err
		}
	}
	err = RemoveFinalizerFromPVC(client, logger, driver, &pvc)
	if err != nil {
		return err
	}

	message := fmt.Sprintf(messages.RemovedPVCFromVG, pvc.Namespace, pvc.Name, vg.Namespace, vg.Name)
	return HandleSuccessMessage(logger, client, vg, message, removingPVC)
}

func ModifyVolumesInVG(logger logr.Logger, client client.Client, vgClient grpcClient.VolumeGroup,
	matchingPvcs []corev1.PersistentVolumeClaim, vg volumegroupv1.VolumeGroup) error {

	currentList := make([]corev1.PersistentVolumeClaim, len(vg.Status.PVCList))
	copy(currentList, vg.Status.PVCList)

	vg.Status.PVCList = matchingPvcs

	err := ModifyVG(logger, client, &vg, vgClient)
	if err != nil {
		vg.Status.PVCList = currentList
		return err
	}

	return nil
}

func UpdatePvcAndPvList(logger logr.Logger, vg *volumegroupv1.VolumeGroup, client client.Client, driver string,
	matchingPvcs []corev1.PersistentVolumeClaim) error {

	vgPvcList := make([]corev1.PersistentVolumeClaim, len(vg.Status.PVCList))
	copy(vgPvcList, vg.Status.PVCList)

	for _, pvc := range vgPvcList {
		if !IsPVCInPVCList(&pvc, matchingPvcs) {
			err := RemoveVolumeFromPvcListAndPvList(logger, client, driver, pvc, vg)
			if err != nil {
				return HandleErrorMessage(logger, client, vg, err, removingPVC)
			}
		}
	}
	for _, pvc := range matchingPvcs {
		if !IsPVCInPVCList(&pvc, vgPvcList) {
			err := AddVolumeToPvcListAndPvList(logger, client, &pvc, vg)
			if err != nil {
				return HandleErrorMessage(logger, client, vg, err, addingPVC)
			}
		}
	}
	return nil
}

func IsPVCListEqual(x []corev1.PersistentVolumeClaim, y []corev1.PersistentVolumeClaim) bool {
	less := func(a, b corev1.PersistentVolumeClaim) bool { return a.Name < b.Name }
	equalIgnoreOrder := cmp.Diff(x, y, cmpopts.SortSlices(less)) == ""
	return equalIgnoreOrder
}
