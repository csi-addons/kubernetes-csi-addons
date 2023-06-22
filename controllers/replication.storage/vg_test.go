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
	"testing"

	volumegroupv1 "github.com/IBM/csi-volume-group-operator/apis/volumegroup.storage/v1"
	replicationv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/apis/replication.storage/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	mockVGCName                = "test-vgc"
	mockVGName                 = "test-vg"
	readyTrue                  = true
	mockVGVolumeReplicationObj = getMockVolumeReplicationObject(mockVGName)
)

var mockVolumeGroupContent = &volumegroupv1.VolumeGroupContent{
	ObjectMeta: metav1.ObjectMeta{
		Name:      mockVGCName,
		Namespace: mockNamespace,
	},
	Spec: volumegroupv1.VolumeGroupContentSpec{
		Source: &volumegroupv1.VolumeGroupContentSource{
			VolumeGroupHandle: mockVolumeHandle,
		},
	},
}

var mockVolumeGroup = &volumegroupv1.VolumeGroup{
	ObjectMeta: metav1.ObjectMeta{
		Name:      mockVGName,
		Namespace: mockNamespace,
	},
	Spec: volumegroupv1.VolumeGroupSpec{
		Source: volumegroupv1.VolumeGroupSource{
			VolumeGroupContentName: &mockVGCName,
		},
	},
	Status: volumegroupv1.VolumeGroupStatus{
		Ready: &readyTrue,
	},
}

func TestGetVGVolumeHandle(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		name                      string
		vgc                       *volumegroupv1.VolumeGroupContent
		vg                        *volumegroupv1.VolumeGroup
		expectedVolumeGroupHandle string
		errorExpected             bool
	}{
		{
			name:                      "case 1: volume group handle available",
			vgc:                       mockVolumeGroupContent,
			vg:                        mockVolumeGroup,
			expectedVolumeGroupHandle: mockVolumeHandle,
			errorExpected:             false,
		},
		{
			name: "case 2: vg name in VolumeReplication CR not found",
			vgc:  mockVolumeGroupContent,
			vg: &volumegroupv1.VolumeGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bad-vg-name",
					Namespace: mockNamespace,
				},
			},
			expectedVolumeGroupHandle: mockVolumeHandle,
			errorExpected:             true,
		},
		{
			name: "case 3: vg not bound",
			vgc:  mockVolumeGroupContent,
			vg: &volumegroupv1.VolumeGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      mockVGName,
					Namespace: mockNamespace,
				},
			},
			expectedVolumeGroupHandle: mockVolumeHandle,
			errorExpected:             true,
		},
	}

	for _, tc := range testcases {
		volumeReplication := &replicationv1alpha1.VolumeReplication{}
		mockVGVolumeReplicationObj.DeepCopyInto(volumeReplication)

		testVGC := &volumegroupv1.VolumeGroupContent{}
		tc.vgc.DeepCopyInto(testVGC)

		testVG := &volumegroupv1.VolumeGroup{}
		tc.vg.DeepCopyInto(testVG)

		namespacedName := types.NamespacedName{
			Name:      volumeReplication.Spec.DataSource.Name,
			Namespace: volumeReplication.Namespace,
		}

		reconciler := createFakeVolumeReplicationReconciler(t, testVGC, testVG, volumeReplication)
		resultVG, resultVGC, err := reconciler.getVGDataSource(log.FromContext(context.TODO()), namespacedName)
		if tc.errorExpected {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.NotEqual(t, nil, resultVG)
			assert.NotEqual(t, nil, resultVGC)
			assert.Equal(t, tc.expectedVolumeGroupHandle, resultVGC.Spec.Source.VolumeGroupHandle)
		}
	}
}

func TestVolumeReplicationReconciler_annotateVGWithOwner(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		name          string
		vg            *volumegroupv1.VolumeGroup
		errorExpected bool
	}{
		{
			name:          "case 1: no VR is owning the vg",
			vg:            mockVolumeGroup,
			errorExpected: false,
		},
		{
			name: "case 2: vg is already owned by same VR",
			vg: &volumegroupv1.VolumeGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      mockVGName,
					Namespace: mockNamespace,
					Annotations: map[string]string{
						replicationv1alpha1.VolumeReplicationNameAnnotation: vrName,
					},
				},
			},
			errorExpected: false,
		},
		{
			name: "case 2: vg is owned by different VR",
			vg: &volumegroupv1.VolumeGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      mockVGName,
					Namespace: mockNamespace,
					Annotations: map[string]string{
						replicationv1alpha1.VolumeReplicationNameAnnotation: "test-vr-1",
					},
				},
			},
			errorExpected: true,
		},
	}

	for _, tc := range testcases {
		volumeReplication := &replicationv1alpha1.VolumeReplication{}
		mockVGVolumeReplicationObj.DeepCopyInto(volumeReplication)

		testVG := &volumegroupv1.VolumeGroup{}
		tc.vg.DeepCopyInto(testVG)

		ctx := context.TODO()
		reconciler := createFakeVolumeReplicationReconciler(t, testVG, volumeReplication)
		err := reconciler.annotateVGWithOwner(ctx, log.FromContext(context.TODO()), vrName, testVG)
		if tc.errorExpected {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)

			vgNamespacedName := types.NamespacedName{
				Name:      testVG.Name,
				Namespace: testVG.Namespace,
			}

			// check annotation is added
			err = reconciler.Get(ctx, vgNamespacedName, testVG)
			assert.NoError(t, err)

			assert.Equal(t, testVG.ObjectMeta.Annotations[replicationv1alpha1.VolumeReplicationNameAnnotation], vrName)
		}

		err = reconciler.removeOwnerFromVGAnnotation(context.TODO(), log.FromContext(context.TODO()), testVG)
		assert.NoError(t, err)

		// try calling delete again, it should not fail
		err = reconciler.removeOwnerFromVGAnnotation(context.TODO(), log.FromContext(context.TODO()), testVG)
		assert.NoError(t, err)

	}

	// try removeOwnerFromVGAnnotation for empty map
	vg := &volumegroupv1.VolumeGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mockVGName,
			Namespace: mockNamespace,
		},
	}
	volumeReplication := &replicationv1alpha1.VolumeReplication{}
	reconciler := createFakeVolumeReplicationReconciler(t, vg, volumeReplication)
	err := reconciler.removeOwnerFromVGAnnotation(context.TODO(), log.FromContext(context.TODO()), vg)
	assert.NoError(t, err)
}
