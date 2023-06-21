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
	volumegroupv1 "github.com/csi-addons/kubernetes-csi-addons/apis/volumegroup.storage/v1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func HandleErrorMessage(logger logr.Logger, client client.Client, vg *volumegroupv1.VolumeGroup,
	err error, reason string) error {
	if err != nil {
		errorMessage := GetMessageFromError(err)
		uErr := UpdateVGStatusError(client, vg, logger, errorMessage)
		if uErr != nil {
			return uErr
		}
		uErr = createNamespacedObjectErrorEvent(logger, client, vg, errorMessage, reason)
		if uErr != nil {
			return uErr
		}
		return err
	}
	return nil
}

func HandleSuccessMessage(logger logr.Logger, client client.Client, vg *volumegroupv1.VolumeGroup, message, reason string) error {
	err := UpdateVGStatusError(client, vg, logger, "")
	if err != nil {
		return err
	}
	err = createSuccessNamespacedObjectEvent(logger, client, vg, message, reason)
	if err != nil {
		return err
	}
	return nil
}

func HandlePVCErrorMessage(logger logr.Logger, client client.Client, pvc *corev1.PersistentVolumeClaim,
	err error, reason string) error {
	if err != nil {
		errorMessage := GetMessageFromError(err)
		if uErr := createNamespacedObjectErrorEvent(logger, client, pvc, errorMessage, reason); uErr != nil {
			return uErr
		}
	}
	return nil
}

func HandleVGCErrorMessage(logger logr.Logger, client client.Client, vgc *volumegroupv1.VolumeGroupContent,
	err error, reason string) error {
	if err != nil {
		errorMessage := GetMessageFromError(err)
		if uErr := createNamespacedObjectErrorEvent(logger, client, vgc, errorMessage, reason); uErr != nil {
			return uErr
		}
		return err
	}
	return err
}
