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

	vgerrors "github.com/csi-addons/kubernetes-csi-addons/controllers/volumegroup.storage/pkg/errors"
	"github.com/csi-addons/kubernetes-csi-addons/controllers/volumegroup.storage/pkg/messages"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetPVFromPVC(logger logr.Logger, client client.Client, pvc *corev1.PersistentVolumeClaim) (*corev1.PersistentVolume, error) {
	logger.Info(fmt.Sprintf(messages.GetPVOfPVC, pvc.Namespace, pvc.Name))
	pvName := getPVNameFromPVC(pvc)
	if pvName == "" {
		logger.Info(messages.PVCDoesNotHavePV)
		return nil, nil
	}

	pv, err := getPV(logger, client, pvName)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, &vgerrors.PVDoesNotExist{PVName: pvName, PVNamespace: pvc.Namespace, ErrorMessage: err.Error()}
		}
		return nil, err
	}
	return pv, nil
}

func getPVNameFromPVC(pvc *corev1.PersistentVolumeClaim) string {
	return GetStringField(pvc.Spec, "VolumeName")
}

func getPV(logger logr.Logger, client client.Client, pvName string) (*corev1.PersistentVolume, error) {
	logger.Info(fmt.Sprintf(messages.GetPV, pvName))
	pv := &corev1.PersistentVolume{}
	namespacedPV := types.NamespacedName{Name: pvName}
	err := client.Get(context.TODO(), namespacedPV, pv)
	if err != nil {
		logger.Error(err, fmt.Sprintf(messages.FailedToGetPV, pvName))
		return nil, err
	}
	return pv, nil
}
