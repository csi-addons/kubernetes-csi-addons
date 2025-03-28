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
	"testing"

	replicationv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/replication.storage/v1alpha1"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var mockVolumeGroupReplicationClassObj = &replicationv1alpha1.VolumeGroupReplicationClass{
	ObjectMeta: metav1.ObjectMeta{
		Name: "volume-group-replication-class",
	},
	Spec: replicationv1alpha1.VolumeGroupReplicationClassSpec{
		Provisioner: "test-driver",
	},
}

func TestGetVolumeGroupReplicationClass(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		createVgrc      bool
		errorExpected   bool
		isErrorNotFound bool
	}{
		{createVgrc: true, errorExpected: false, isErrorNotFound: false},
		{createVgrc: false, errorExpected: true, isErrorNotFound: true},
	}

	for _, tc := range testcases {
		var objects []runtime.Object

		volumeGroupReplication := &replicationv1alpha1.VolumeGroupReplication{}
		mockVolumeGroupReplicationObj.DeepCopyInto(volumeGroupReplication)
		objects = append(objects, volumeGroupReplication)

		if tc.createVgrc {
			volumeGroupReplicationClass := &replicationv1alpha1.VolumeGroupReplicationClass{}
			mockVolumeGroupReplicationClassObj.DeepCopyInto(volumeGroupReplicationClass)
			objects = append(objects, volumeGroupReplicationClass)
		}

		reconciler := createFakeVolumeGroupReplicationReconciler(t, objects...)
		vgrClassObj, err := reconciler.getVolumeGroupReplicationClass(mockVolumeGroupReplicationClassObj.Name)

		if tc.errorExpected {
			assert.Error(t, err)
			if tc.isErrorNotFound {
				assert.True(t, errors.IsNotFound(err))
			}
		} else {
			assert.NoError(t, err)
			assert.NotEqual(t, nil, vgrClassObj)
		}
	}
}
