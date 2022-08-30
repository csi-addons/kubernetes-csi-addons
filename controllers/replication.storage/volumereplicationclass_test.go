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
	"testing"

	replicationv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/apis/replication.storage/v1alpha1"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var mockVolumeReplicationClassObj = &replicationv1alpha1.VolumeReplicationClass{
	ObjectMeta: metav1.ObjectMeta{
		Name: "volume-replication-class",
	},
	Spec: replicationv1alpha1.VolumeReplicationClassSpec{
		Provisioner: "test-driver",
	},
}

func TestGetVolumeReplicaClass(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		createVrc       bool
		errorExpected   bool
		isErrorNotFound bool
	}{
		{createVrc: true, errorExpected: false, isErrorNotFound: false},
		{createVrc: false, errorExpected: true, isErrorNotFound: true},
	}

	for _, tc := range testcases {
		var objects []runtime.Object

		volumeReplication := &replicationv1alpha1.VolumeReplication{}
		mockVolumeReplicationObj.DeepCopyInto(volumeReplication)
		objects = append(objects, volumeReplication)

		if tc.createVrc {
			volumeReplicationClass := &replicationv1alpha1.VolumeReplicationClass{}
			mockVolumeReplicationClassObj.DeepCopyInto(volumeReplicationClass)
			objects = append(objects, volumeReplicationClass)
		}

		reconciler := createFakeVolumeReplicationReconciler(t, objects...)
		vrcObj, err := reconciler.getVolumeReplicationClass(log.FromContext(context.TODO()), mockVolumeReplicationClassObj.Name)

		if tc.errorExpected {
			assert.Error(t, err)
			if tc.isErrorNotFound {
				assert.True(t, errors.IsNotFound(err))
			}
		} else {
			assert.NoError(t, err)
			assert.NotEqual(t, nil, vrcObj)
		}
	}
}
