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
	"slices"

	replicationv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/replication.storage/v1alpha1"
	"github.com/csi-addons/kubernetes-csi-addons/internal/util"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	volumeReplicationFinalizer = "replication.storage.openshift.io"
	pvcReplicationFinalizer    = "replication.storage.openshift.io/pvc-protection"
	vgrReplicationFinalizer    = "replication.storage.openshift.io/vgr-protection"
)

// addFinalizerToVR adds the VR finalizer on the VolumeReplication instance.
func (r *VolumeReplicationReconciler) addFinalizerToVR(logger logr.Logger, vr *replicationv1alpha1.VolumeReplication,
) error {
	if !slices.Contains(vr.Finalizers, volumeReplicationFinalizer) {
		logger.Info("adding finalizer to volumeReplication object", "Finalizer", volumeReplicationFinalizer)
		vr.Finalizers = append(vr.Finalizers, volumeReplicationFinalizer)
		if err := r.Update(context.TODO(), vr); err != nil {
			return fmt.Errorf("failed to add finalizer (%s) to VolumeReplication resource"+
				" (%s/%s) %w",
				volumeReplicationFinalizer, vr.Namespace, vr.Name, err)
		}
	}

	return nil
}

// removeFinalizerFromVR removes the VR finalizer from the VolumeReplication instance.
func (r *VolumeReplicationReconciler) removeFinalizerFromVR(logger logr.Logger, vr *replicationv1alpha1.VolumeReplication) error {
	if slices.Contains(vr.Finalizers, volumeReplicationFinalizer) {
		logger.Info("removing finalizer from volumeReplication object", "Finalizer", volumeReplicationFinalizer)
		vr.Finalizers = util.RemoveFromSlice(vr.Finalizers, volumeReplicationFinalizer)
		if err := r.Update(context.TODO(), vr); err != nil {
			return fmt.Errorf("failed to remove finalizer (%s) from VolumeReplication resource"+
				" (%s/%s), %w",
				volumeReplicationFinalizer, vr.Namespace, vr.Name, err)
		}
	}

	return nil
}

// addFinalizerToPVC adds the VR, VGR finalizer on the PersistentVolumeClaim.
func addFinalizerToPVC(client client.Client, logger logr.Logger, pvc *corev1.PersistentVolumeClaim,
	replicationFinalizer string) error {
	if !slices.Contains(pvc.Finalizers, replicationFinalizer) {
		logger.Info("adding finalizer to PersistentVolumeClaim object", "Finalizer", replicationFinalizer)
		pvc.Finalizers = append(pvc.Finalizers, replicationFinalizer)
		if err := client.Update(context.TODO(), pvc); err != nil {
			return fmt.Errorf("failed to add finalizer (%s) to PersistentVolumeClaim resource"+
				" (%s/%s) %w",
				replicationFinalizer, pvc.Namespace, pvc.Name, err)
		}
	}

	return nil
}

// removeFinalizerFromPVC removes the VR, VGR finalizer on PersistentVolumeClaim.
func removeFinalizerFromPVC(client client.Client, logger logr.Logger, pvc *corev1.PersistentVolumeClaim,
	replicationFinalizer string) error {
	if slices.Contains(pvc.Finalizers, replicationFinalizer) {
		logger.Info("removing finalizer from PersistentVolumeClaim object", "Finalizer", replicationFinalizer)
		pvc.Finalizers = util.RemoveFromSlice(pvc.Finalizers, replicationFinalizer)
		if err := client.Update(context.TODO(), pvc); err != nil {
			return fmt.Errorf("failed to remove finalizer (%s) from PersistentVolumeClaim resource"+
				" (%s/%s), %w",
				replicationFinalizer, pvc.Namespace, pvc.Name, err)
		}
	}

	return nil
}

// addFinalizerToVGR adds the VGR finalizer on the VolumeGroupReplication resource
func addFinalizerToVGR(client client.Client, logger logr.Logger, vgr *replicationv1alpha1.VolumeGroupReplication) error {
	if !slices.Contains(vgr.Finalizers, vgrReplicationFinalizer) {
		logger.Info("adding finalizer to volumeGroupReplication object", "Finalizer", vgrReplicationFinalizer)
		vgr.Finalizers = append(vgr.Finalizers, vgrReplicationFinalizer)
		if err := client.Update(context.TODO(), vgr); err != nil {
			return fmt.Errorf("failed to add finalizer (%s) to VolumeGroupReplication resource"+
				" (%s/%s) %w",
				vgrReplicationFinalizer, vgr.Namespace, vgr.Name, err)
		}
	}

	return nil
}

// removeFinalizerFromVGR removes the VGR finalizer from the VolumeGroupReplication instance.
func removeFinalizerFromVGR(client client.Client, logger logr.Logger, vgr *replicationv1alpha1.VolumeGroupReplication) error {
	if slices.Contains(vgr.Finalizers, vgrReplicationFinalizer) {
		logger.Info("removing finalizer from volumeGroupReplication object", "Finalizer", vgrReplicationFinalizer)
		// Check if owner annotations are removed from the VGR resource
		if vgr.Annotations[replicationv1alpha1.VolumeGroupReplicationContentNameAnnotation] != "" ||
			vgr.Annotations[replicationv1alpha1.VolumeReplicationNameAnnotation] != "" {
			return fmt.Errorf("failed to remove finalizer from volumeGroupReplication object"+
				",dependent resources are not yet deleted (%s/%s)", vgr.Namespace, vgr.Name)
		}
		vgr.Finalizers = util.RemoveFromSlice(vgr.Finalizers, vgrReplicationFinalizer)
		if err := client.Update(context.TODO(), vgr); err != nil {
			return fmt.Errorf("failed to remove finalizer (%s) from VolumeGroupReplication resource"+
				" (%s/%s), %w",
				vgrReplicationFinalizer, vgr.Namespace, vgr.Name, err)
		}
	}

	return nil
}

// addFinalizerToVGRContent adds the VGR finalizer on the VolumeGroupReplicationContent resource
func addFinalizerToVGRContent(client client.Client, logger logr.Logger, vgrContent *replicationv1alpha1.VolumeGroupReplicationContent,
	replicationFinalizer string) error {
	if !slices.Contains(vgrContent.Finalizers, replicationFinalizer) {
		logger.Info("adding finalizer to volumeGroupReplicationContent object", "Finalizer", replicationFinalizer)
		vgrContent.Finalizers = append(vgrContent.Finalizers, replicationFinalizer)
		if err := client.Update(context.TODO(), vgrContent); err != nil {
			return fmt.Errorf("failed to add finalizer (%s) to VolumeGroupReplicationContent resource"+
				" (%s) %w",
				replicationFinalizer, vgrContent.Name, err)
		}
	}

	return nil
}

// removeFinalizerFromVGRContent removes the VGR finalizer from the VolumeGroupReplicationContent instance.
func removeFinalizerFromVGRContent(client client.Client, logger logr.Logger, vgrContent *replicationv1alpha1.VolumeGroupReplicationContent,
	replicationFinalizer string) error {
	if slices.Contains(vgrContent.Finalizers, replicationFinalizer) {
		logger.Info("removing finalizer from volumeGroupReplicationContent object", "Finalizer", replicationFinalizer)
		vgrContent.Finalizers = util.RemoveFromSlice(vgrContent.Finalizers, replicationFinalizer)
		if err := client.Update(context.TODO(), vgrContent); err != nil {
			return fmt.Errorf("failed to remove finalizer (%s) from VolumeGroupReplicationContent resource"+
				" (%s), %w",
				replicationFinalizer, vgrContent.Name, err)
		}
	}

	return nil
}
