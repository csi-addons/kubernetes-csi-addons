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

package controllers

import (
	"context"
	"fmt"
	"reflect"

	volumegroupv1 "github.com/IBM/csi-volume-group-operator/apis/volumegroup.storage/v1"
	replicationv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/apis/replication.storage/v1alpha1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

// getVGDataSource get vg content, vg object from the request.
func (r VolumeReplicationReconciler) getVGDataSource(logger logr.Logger, req types.NamespacedName) (*volumegroupv1.VolumeGroup, *volumegroupv1.VolumeGroupContent, error) {
	vg := &volumegroupv1.VolumeGroup{}
	err := r.Client.Get(context.TODO(), req, vg)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Error(err, "VG not found", "VG Name", req.Name)
		}

		return nil, nil, err
	}
	volumeGroupContentName := getStringField(vg.Spec.Source, "VolumeGroupContentName")
	// Validate VG bound to VGC
	if volumeGroupContentName == "" {
		return vg, nil, fmt.Errorf("VG %q is not bound to any VGC", req.Name)
	}

	vgc := &volumegroupv1.VolumeGroupContent{}
	namespacedVGC := types.NamespacedName{Name: volumeGroupContentName, Namespace: vg.Namespace}
	err = r.Client.Get(context.TODO(), namespacedVGC, vgc)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Error(err, "VolumeGroupContent not found", "VolumeGroupContent Name", volumeGroupContentName)
		}
		return vg, nil, err
	}

	return vg, vgc, nil
}

// annotateVGWithOwner will add the VolumeReplication details to the VG annotations.
func (r *VolumeReplicationReconciler) annotateVGWithOwner(ctx context.Context, logger logr.Logger, reqOwnerName string, vg *volumegroupv1.VolumeGroup) error {
	if vg.ObjectMeta.Annotations == nil {
		vg.ObjectMeta.Annotations = map[string]string{}
	}

	currentOwnerName := vg.ObjectMeta.Annotations[replicationv1alpha1.VolumeReplicationNameAnnotation]
	if currentOwnerName == "" {
		logger.Info("setting owner on PVC annotation", "Name", vg.Name, "owner", reqOwnerName)
		vg.ObjectMeta.Annotations[replicationv1alpha1.VolumeReplicationNameAnnotation] = reqOwnerName
		err := r.Update(ctx, vg)
		if err != nil {
			logger.Error(err, "Failed to update VG annotation", "Name", vg.Name)

			return fmt.Errorf("failed to update VG %q annotation for VolumeReplication: %w",
				vg.Name, err)
		}

		return nil
	}

	if currentOwnerName != reqOwnerName {
		logger.Info("cannot change the owner of VG",
			"VG name", vg.Name,
			"current owner", currentOwnerName,
			"requested owner", reqOwnerName)

		return fmt.Errorf("VG %q not owned by VolumeReplication %q",
			vg.Name, reqOwnerName)
	}

	return nil
}

// removeOwnerFromVGAnnotation removes the VolumeReplication owner from the PVC annotations.
func (r *VolumeReplicationReconciler) removeOwnerFromVGAnnotation(ctx context.Context, logger logr.Logger, vg *volumegroupv1.VolumeGroup) error {
	if _, ok := vg.ObjectMeta.Annotations[replicationv1alpha1.VolumeReplicationNameAnnotation]; ok {
		logger.Info("removing owner annotation from PersistentVolumeClaim object", "Annotation", replicationv1alpha1.VolumeReplicationNameAnnotation)
		delete(vg.ObjectMeta.Annotations, replicationv1alpha1.VolumeReplicationNameAnnotation)
		if err := r.Client.Update(ctx, vg); err != nil {
			return fmt.Errorf("failed to remove annotation %q from VolumeGroup "+
				"%q %w",
				replicationv1alpha1.VolumeReplicationNameAnnotation, vg.Name, err)
		}
	}

	return nil
}

func getStringField(object interface{}, fieldName string) string {
	fieldValue := getObjectField(object, fieldName)
	if fieldValue.Kind() == reflect.Ptr && fieldValue.IsNil() {
		return ""
	}
	return fieldValue.Elem().String()
}

func getObjectField(object interface{}, fieldName string) reflect.Value {
	objectValue := reflect.ValueOf(object)
	for i := 0; i < objectValue.NumField(); i++ {
		fieldValue := objectValue.Field(i)
		fieldType := objectValue.Type().Field(i)
		if fieldType.Name == fieldName {
			return fieldValue
		}
	}
	return reflect.ValueOf(nil)
}
