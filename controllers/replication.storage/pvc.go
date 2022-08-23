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

package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	replicationv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/apis/replication.storage/v1alpha1"
)

// getPVCDataSource get pvc, pv object from the request.
func (r VolumeReplicationReconciler) getPVCDataSource(logger logr.Logger, req types.NamespacedName) (*corev1.PersistentVolumeClaim, *corev1.PersistentVolume, error) {
	pvc := &corev1.PersistentVolumeClaim{}
	err := r.Client.Get(context.TODO(), req, pvc)
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
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: pvName}, pv)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Error(err, "PV not found", "PV Name", pvName)
		}

		return pvc, nil, err
	}

	return pvc, pv, nil
}

// annotatePVCWithOwner will add the VolumeReplication details to the PVC annotations.
func (r *VolumeReplicationReconciler) annotatePVCWithOwner(ctx context.Context, logger logr.Logger, req types.NamespacedName, pvc *corev1.PersistentVolumeClaim) error {
	if pvc.ObjectMeta.Annotations == nil {
		pvc.ObjectMeta.Annotations = map[string]string{}
	}

	ownerName := pvc.ObjectMeta.Annotations[replicationv1alpha1.VolumeReplicationNameAnnotation]
	if ownerName == "" {
		pvc.ObjectMeta.Annotations[replicationv1alpha1.VolumeReplicationNameAnnotation] = req.Name
		err := r.Update(ctx, pvc)
		if err != nil {
			logger.Error(err, "Failed to update PVC annotation", "Name", pvc.Name)

			return fmt.Errorf("failed to update PVC %q annotation for VolumeReplication: %w",
				pvc.Name, err)
		}

		return nil
	}

	if ownerName != req.Name {
		logger.Info("cannot change the owner of PVC",
			"PVC name", pvc.Name,
			"current owner", ownerName,
			"requested owner", req.Name)

		return fmt.Errorf("PVC %q not owned by VolumeReplication %q",
			pvc.Name, req.Name)
	}

	return nil
}
