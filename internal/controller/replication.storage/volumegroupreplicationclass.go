/*
Copyright 2025 The Kubernetes-CSI-Addons Authors.

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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	replicationv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/replication.storage/v1alpha1"
)

// getVolumeGroupReplicationClass fetches the volumegroupreplicationclass object in the given namespace and return the same.
// nolint:unused // Ignore unused function, will be updated later
func (r VolumeGroupReplicationReconciler) getVolumeGroupReplicationClass(logger logr.Logger, vgrClassName string) (*replicationv1alpha1.VolumeGroupReplicationClass, error) {
	vgrClassObj := &replicationv1alpha1.VolumeGroupReplicationClass{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: vgrClassName}, vgrClassObj)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Error(err, "VolumeGroupReplicationClass not found", "VolumeGroupReplicationClass", vgrClassName)
		} else {
			logger.Error(err, "Got an unexpected error while fetching VolumeGroupReplicationClass", "VolumeGroupReplicationClass", vgrClassName)
		}

		return nil, err
	}

	return vgrClassObj, nil
}
