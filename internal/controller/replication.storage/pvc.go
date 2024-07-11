/*
Copyright 2022 The Kubernetes-CSI-Addons Authors.

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
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	replicationv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/replication.storage/v1alpha1"
)

// getPVCDataSource get pvc, pv object from the request.
func (r VolumeReplicationReconciler) getPVCDataSource(ctx context.Context, logger logr.Logger, req types.NamespacedName) (*corev1.PersistentVolumeClaim, *corev1.PersistentVolume, error) {
	pvc := &corev1.PersistentVolumeClaim{}
	err := r.Get(ctx, req, pvc)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Error(err, "PVC not found", "PVC Name", req.Name)
		}

		return nil, nil, err
	}
	// Validate PVC in bound state
	if pvc.Status.Phase != corev1.ClaimBound {
		return pvc, nil, fmt.Errorf("PVC %q is not bound to any PV", req.Name)
	}

	// Get PV object for the PVC
	pvName := pvc.Spec.VolumeName
	pv := &corev1.PersistentVolume{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: pvName}, pv)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Error(err, "PV not found", "PV Name", pvName)
		}

		return pvc, nil, err
	}

	return pvc, pv, nil
}

// annotatePVCWithOwner will add the VolumeReplication/VolumeGroupReplication details to the PVC annotations.
func annotatePVCWithOwner(client client.Client, ctx context.Context, logger logr.Logger, reqOwnerName string,
	pvc *corev1.PersistentVolumeClaim, pvcAnnotation string) error {
	if pvc.Annotations == nil {
		pvc.Annotations = map[string]string{}
	}

	if pvc.Annotations[replicationv1alpha1.VolumeReplicationNameAnnotation] != "" &&
		pvc.Annotations[replicationv1alpha1.VolumeGroupReplicationNameAnnotation] != "" {
		logger.Info("PVC can't be part of both VolumeGroupReplication and VolumeReplication")
		return fmt.Errorf("PVC (%s/%s) can't be owned by both VolumeReplication and VolumeGroupReplication", pvc.Namespace, pvc.Name)
	}

	currentOwnerName := pvc.Annotations[pvcAnnotation]
	if currentOwnerName == "" {
		logger.Info("setting owner on PVC annotation", "PVC Name", pvc.Name, "PVC Namespace", pvc.Namespace, "owner", reqOwnerName)
		pvc.Annotations[pvcAnnotation] = reqOwnerName
		err := client.Update(ctx, pvc)
		if err != nil {
			logger.Error(err, "Failed to update PVC annotation", "Name", pvc.Name, "Namespace", pvc.Namespace)
			return fmt.Errorf("failed to update PVC (%s/%s) annotation for replication: %w", pvc.Namespace, pvc.Name, err)
		}

		return nil
	}

	if currentOwnerName != reqOwnerName {
		logger.Info("cannot change the owner of PVC",
			"PVC name", pvc.Name,
			"PVC Namespace", pvc.Namespace,
			"current owner", currentOwnerName,
			"requested owner", reqOwnerName)

		return fmt.Errorf("PVC (%s/%s) not owned by correct VolumeReplication/VolumeGroupReplication %q",
			pvc.Namespace, pvc.Name, reqOwnerName)
	}

	return nil
}

// removeOwnerFromPVCAnnotation removes the VolumeReplication/VolumeGroupReplication owner from the PVC annotations.
func removeOwnerFromPVCAnnotation(client client.Client, ctx context.Context, logger logr.Logger, pvc *corev1.PersistentVolumeClaim,
	pvcAnnotation string) error {
	if _, ok := pvc.Annotations[pvcAnnotation]; ok {
		logger.Info("removing owner annotation from PersistentVolumeClaim object", "PVC Name", pvc.Name, "PVC Namespace", pvc.Namespace, "Annotation", pvcAnnotation)
		delete(pvc.Annotations, pvcAnnotation)
		if err := client.Update(ctx, pvc); err != nil {
			return fmt.Errorf("failed to remove annotation %q from PersistentVolumeClaim "+
				"(%s/%s) %w",
				pvcAnnotation, pvc.Namespace, pvc.Name, err)
		}
	}

	return nil
}
