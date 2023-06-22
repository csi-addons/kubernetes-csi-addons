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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	mockPVName  = "test-pv"
	mockPVCName = "test-pvc"
)

var mockPVCVolumeReplicationObj = getMockVolumeReplicationObject(mockPVCName)

var mockPersistentVolume = &corev1.PersistentVolume{
	ObjectMeta: metav1.ObjectMeta{
		Name: mockPVName,
	},
	Spec: corev1.PersistentVolumeSpec{
		PersistentVolumeSource: corev1.PersistentVolumeSource{
			CSI: &corev1.CSIPersistentVolumeSource{
				VolumeHandle: mockVolumeHandle,
			},
		},
	},
}

var mockPersistentVolumeClaim = &corev1.PersistentVolumeClaim{
	ObjectMeta: metav1.ObjectMeta{
		Name:      mockPVCName,
		Namespace: mockNamespace,
	},
	Spec: corev1.PersistentVolumeClaimSpec{
		VolumeName: mockPVName,
	},
	Status: corev1.PersistentVolumeClaimStatus{
		Phase: corev1.ClaimBound,
	},
}

func TestGetPVCVolumeHandle(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		name                 string
		pv                   *corev1.PersistentVolume
		pvc                  *corev1.PersistentVolumeClaim
		expectedVolumeHandle string
		errorExpected        bool
	}{
		{
			name:                 "case 1: volume handle available",
			pv:                   mockPersistentVolume,
			pvc:                  mockPersistentVolumeClaim,
			expectedVolumeHandle: mockVolumeHandle,
			errorExpected:        false,
		},
		{
			name: "case 2: pvc name in VolumeReplication CR not found",
			pv:   mockPersistentVolume,
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pvc-name",
					Namespace: mockNamespace,
				},
			},
			expectedVolumeHandle: mockVolumeHandle,
			errorExpected:        true,
		},
		{
			name: "case 3: pvc not bound",
			pv:   mockPersistentVolume,
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      mockPVCName,
					Namespace: mockNamespace,
				},
			},
			expectedVolumeHandle: mockVolumeHandle,
			errorExpected:        true,
		},
	}

	for _, tc := range testcases {
		volumeReplication := &replicationv1alpha1.VolumeReplication{}
		mockPVCVolumeReplicationObj.DeepCopyInto(volumeReplication)

		testPV := &corev1.PersistentVolume{}
		tc.pv.DeepCopyInto(testPV)

		testPVC := &corev1.PersistentVolumeClaim{}
		tc.pvc.DeepCopyInto(testPVC)

		namespacedName := types.NamespacedName{
			Name:      mockPVCName,
			Namespace: volumeReplication.Namespace,
		}

		reconciler := createFakeVolumeReplicationReconciler(t, testPV, testPVC, volumeReplication)
		resultPVC, resultPV, err := reconciler.getPVCDataSource(log.FromContext(context.TODO()), namespacedName)
		if tc.errorExpected {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.NotEqual(t, nil, resultPVC)
			assert.NotEqual(t, nil, resultPV)
			assert.Equal(t, tc.expectedVolumeHandle, resultPV.Spec.CSI.VolumeHandle)
		}
	}
}

func TestVolumeReplicationReconciler_annotatePVCWithOwner(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		name          string
		pvc           *corev1.PersistentVolumeClaim
		errorExpected bool
	}{
		{
			name:          "case 1: no VR is owning the PVC",
			pvc:           mockPersistentVolumeClaim,
			errorExpected: false,
		},
		{
			name: "case 2: pvc is already owned by same VR",
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pvc-name",
					Namespace: mockNamespace,
					Annotations: map[string]string{
						replicationv1alpha1.VolumeReplicationNameAnnotation: vrName,
					},
				},
			},
			errorExpected: false,
		},
		{
			name: "case 2: pvc is owned by different VR",
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pvc-name",
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
		mockPVCVolumeReplicationObj.DeepCopyInto(volumeReplication)

		testPVC := &corev1.PersistentVolumeClaim{}
		tc.pvc.DeepCopyInto(testPVC)

		ctx := context.TODO()
		reconciler := createFakeVolumeReplicationReconciler(t, testPVC, volumeReplication)
		err := reconciler.annotatePVCWithOwner(ctx, log.FromContext(context.TODO()), vrName, testPVC)
		if tc.errorExpected {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)

			pvcNamespacedName := types.NamespacedName{
				Name:      testPVC.Name,
				Namespace: testPVC.Namespace,
			}

			// check annotation is added
			err = reconciler.Get(ctx, pvcNamespacedName, testPVC)
			assert.NoError(t, err)

			assert.Equal(t, testPVC.ObjectMeta.Annotations[replicationv1alpha1.VolumeReplicationNameAnnotation], vrName)
		}

		err = reconciler.removeOwnerFromPVCAnnotation(context.TODO(), log.FromContext(context.TODO()), testPVC)
		assert.NoError(t, err)

		// try calling delete again, it should not fail
		err = reconciler.removeOwnerFromPVCAnnotation(context.TODO(), log.FromContext(context.TODO()), testPVC)
		assert.NoError(t, err)

	}

	// try removeOwnerFromPVCAnnotation for empty map
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc-name",
			Namespace: mockNamespace,
		},
	}
	volumeReplication := &replicationv1alpha1.VolumeReplication{}
	reconciler := createFakeVolumeReplicationReconciler(t, pvc, volumeReplication)
	err := reconciler.removeOwnerFromPVCAnnotation(context.TODO(), log.FromContext(context.TODO()), pvc)
	assert.NoError(t, err)
}
