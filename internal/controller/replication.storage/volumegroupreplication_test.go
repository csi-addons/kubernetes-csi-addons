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
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	replicationv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/replication.storage/v1alpha1"
)

const (
	mockPV  = "test-vgr-pv"
	mockPVC = "test-vgr-pvc"
)

var mockVolumeGroupReplicationObj = &replicationv1alpha1.VolumeGroupReplication{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "volume-group-replication",
		Namespace: mockNamespace,
		UID:       "testname",
	},
	Spec: replicationv1alpha1.VolumeGroupReplicationSpec{
		VolumeGroupReplicationClassName: "volume-group-replication-class",
		VolumeReplicationClassName:      "volume-replication-class",
		Source: replicationv1alpha1.VolumeGroupReplicationSource{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"test": "vgr_test",
				},
			},
		},
	},
}

var mockVGRPersistentVolume = &corev1.PersistentVolume{
	ObjectMeta: metav1.ObjectMeta{
		Name: mockPV,
	},
	Spec: corev1.PersistentVolumeSpec{
		PersistentVolumeSource: corev1.PersistentVolumeSource{
			CSI: &corev1.CSIPersistentVolumeSource{
				Driver:       "test-driver",
				VolumeHandle: mockVolumeHandle,
			},
		},
	},
}

var mockVGRPersistentVolumeClaim = &corev1.PersistentVolumeClaim{
	ObjectMeta: metav1.ObjectMeta{
		Name:      mockPVC,
		Namespace: mockNamespace,
		Labels: map[string]string{
			"test": "vgr_test",
		},
	},
	Spec: corev1.PersistentVolumeClaimSpec{
		VolumeName: mockPV,
	},
	Status: corev1.PersistentVolumeClaimStatus{
		Phase: corev1.ClaimBound,
	},
}

func createFakeVolumeGroupReplicationReconciler(t *testing.T, obj ...runtime.Object) VolumeGroupReplicationReconciler {
	t.Helper()
	scheme := createFakeScheme(t)
	vgrInit := &replicationv1alpha1.VolumeGroupReplication{}
	vgrContentInit := &replicationv1alpha1.VolumeGroupReplicationContent{}
	client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(obj...).WithStatusSubresource(vgrInit, vgrContentInit).Build()
	logger := log.FromContext(context.TODO())
	reconcilerCtx := context.TODO()

	return VolumeGroupReplicationReconciler{
		Client:           client,
		Scheme:           scheme,
		log:              logger,
		ctx:              reconcilerCtx,
		MaxGroupPVCCount: 100,
	}
}

func TestUpdateReplicationDestinationCondition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		pvInfoMap       PersistentVolumeInfoMap
		pvMappingList   []replicationv1alpha1.PersistentVolumeMapping
		expectAvailable bool
		expectedReason  string
	}{
		{
			name:            "empty pvInfoMap sets available",
			pvInfoMap:       PersistentVolumeInfoMap{},
			pvMappingList:   nil,
			expectAvailable: true,
			expectedReason:  replicationv1alpha1.DestinationInfoUpdated,
		},
		{
			name: "all PVs present in mapping with destination handles sets available",
			pvInfoMap: PersistentVolumeInfoMap{
				"pvc-1": {PvName: "pv-1", PvHandle: "handle-1"},
				"pvc-2": {PvName: "pv-2", PvHandle: "handle-2"},
			},
			pvMappingList: []replicationv1alpha1.PersistentVolumeMapping{
				{Name: "pv-1", DestinationVolumeHandle: "dest-handle-1"},
				{Name: "pv-2", DestinationVolumeHandle: "dest-handle-2"},
			},
			expectAvailable: true,
			expectedReason:  replicationv1alpha1.DestinationInfoUpdated,
		},
		{
			name: "PV not in persistentVolumeMappingList sets pending",
			pvInfoMap: PersistentVolumeInfoMap{
				"pvc-1": {PvName: "pv-1", PvHandle: "handle-1"},
				"pvc-2": {PvName: "pv-2", PvHandle: "handle-2"},
			},
			pvMappingList: []replicationv1alpha1.PersistentVolumeMapping{
				{Name: "pv-1", DestinationVolumeHandle: "dest-handle-1"},
			},
			expectAvailable: false,
			expectedReason:  replicationv1alpha1.DestinationInfoPending,
		},
		{
			name: "PV in mapping but missing destination handle sets pending",
			pvInfoMap: PersistentVolumeInfoMap{
				"pvc-1": {PvName: "pv-1", PvHandle: "handle-1"},
				"pvc-2": {PvName: "pv-2", PvHandle: "handle-2"},
			},
			pvMappingList: []replicationv1alpha1.PersistentVolumeMapping{
				{Name: "pv-1", DestinationVolumeHandle: "dest-handle-1"},
				{Name: "pv-2", DestinationVolumeHandle: ""},
			},
			expectAvailable: false,
			expectedReason:  replicationv1alpha1.DestinationInfoPending,
		},
		{
			name: "mapping list shorter than pvInfoMap sets pending without checking entries",
			pvInfoMap: PersistentVolumeInfoMap{
				"pvc-1": {PvName: "pv-1", PvHandle: "handle-1"},
				"pvc-2": {PvName: "pv-2", PvHandle: "handle-2"},
				"pvc-3": {PvName: "pv-3", PvHandle: "handle-3"},
			},
			pvMappingList: []replicationv1alpha1.PersistentVolumeMapping{
				{Name: "pv-1", DestinationVolumeHandle: "dest-handle-1"},
			},
			expectAvailable: false,
			expectedReason:  replicationv1alpha1.DestinationInfoPending,
		},
		{
			name: "nil mapping list with non-empty pvInfoMap sets pending",
			pvInfoMap: PersistentVolumeInfoMap{
				"pvc-1": {PvName: "pv-1", PvHandle: "handle-1"},
			},
			pvMappingList:   nil,
			expectAvailable: false,
			expectedReason:  replicationv1alpha1.DestinationInfoPending,
		},
		{
			name: "single PVC with valid mapping sets available",
			pvInfoMap: PersistentVolumeInfoMap{
				"pvc-1": {PvName: "pv-1", PvHandle: "handle-1"},
			},
			pvMappingList: []replicationv1alpha1.PersistentVolumeMapping{
				{Name: "pv-1", DestinationVolumeHandle: "dest-handle-1"},
			},
			expectAvailable: true,
			expectedReason:  replicationv1alpha1.DestinationInfoUpdated,
		},
		{
			name: "extra mappings beyond pvInfoMap still available if all PVs covered",
			pvInfoMap: PersistentVolumeInfoMap{
				"pvc-1": {PvName: "pv-1", PvHandle: "handle-1"},
			},
			pvMappingList: []replicationv1alpha1.PersistentVolumeMapping{
				{Name: "pv-1", DestinationVolumeHandle: "dest-handle-1"},
				{Name: "pv-extra", DestinationVolumeHandle: "dest-handle-extra"},
			},
			expectAvailable: true,
			expectedReason:  replicationv1alpha1.DestinationInfoUpdated,
		},
		{
			name: "mapping has enough entries but wrong PV names sets pending",
			pvInfoMap: PersistentVolumeInfoMap{
				"pvc-1": {PvName: "pv-1", PvHandle: "handle-1"},
			},
			pvMappingList: []replicationv1alpha1.PersistentVolumeMapping{
				{Name: "pv-other", DestinationVolumeHandle: "dest-handle-other"},
			},
			expectAvailable: false,
			expectedReason:  replicationv1alpha1.DestinationInfoPending,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vgr := &replicationv1alpha1.VolumeGroupReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-vgr",
					Namespace:  mockNamespace,
					Generation: 1,
				},
			}

			r := &VolumeGroupReplicationReconciler{}
			r.updateReplicationDestinationCondition(vgr, tc.pvInfoMap, tc.pvMappingList)

			cond := findCondition(vgr.Status.Conditions, replicationv1alpha1.ConditionDestinationInfoAvailable)
			assert.NotNil(t, cond)
			assert.Equal(t, tc.expectedReason, cond.Reason)
			if tc.expectAvailable {
				assert.Equal(t, metav1.ConditionTrue, cond.Status)
			} else {
				assert.Equal(t, metav1.ConditionFalse, cond.Status)
			}
			assert.Equal(t, int64(1), cond.ObservedGeneration)
		})
	}
}

func TestVolumeGroupReplication(t *testing.T) {
	t.Parallel()
	testcases := []struct {
		name            string
		pv              *corev1.PersistentVolume
		pvc             *corev1.PersistentVolumeClaim
		expectedPVCList []string
		pvcFound        bool
	}{
		{
			name:            "case 1: matching pvc available",
			pv:              mockVGRPersistentVolume,
			pvc:             mockVGRPersistentVolumeClaim,
			expectedPVCList: []string{mockPVC},
			pvcFound:        true,
		},
		{
			name: "case 2: matching pvc not found",
			pv:   mockVGRPersistentVolume,
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      mockPVC,
					Namespace: mockNamespace,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					VolumeName: mockPV,
				},
				Status: corev1.PersistentVolumeClaimStatus{
					Phase: corev1.ClaimBound,
				},
			},
			expectedPVCList: []string{},
			pvcFound:        false,
		},
	}
	for _, tc := range testcases {
		volumeGroupReplication := &replicationv1alpha1.VolumeGroupReplication{}
		mockVolumeGroupReplicationObj.DeepCopyInto(volumeGroupReplication)

		volumeGroupReplicationClass := &replicationv1alpha1.VolumeGroupReplicationClass{}
		mockVolumeGroupReplicationClassObj.DeepCopyInto(volumeGroupReplicationClass)

		volumeReplicationClass := &replicationv1alpha1.VolumeReplicationClass{}
		mockVolumeReplicationClassObj.DeepCopyInto(volumeReplicationClass)

		testPV := &corev1.PersistentVolume{}
		tc.pv.DeepCopyInto(testPV)

		testPVC := &corev1.PersistentVolumeClaim{}
		tc.pvc.DeepCopyInto(testPVC)

		r := createFakeVolumeGroupReplicationReconciler(t, testPV, testPVC, volumeReplicationClass, volumeGroupReplicationClass, volumeGroupReplication)
		nsKey := types.NamespacedName{
			Namespace: volumeGroupReplication.Namespace,
			Name:      volumeGroupReplication.Name,
		}
		req := reconcile.Request{
			NamespacedName: nsKey,
		}
		res, err := r.Reconcile(context.TODO(), req)

		if tc.pvcFound {
			// Check reconcile didn't return any error
			assert.Equal(t, reconcile.Result{}, res)
			assert.NoError(t, err)

			pvc := &corev1.PersistentVolumeClaim{}
			err = r.Get(context.TODO(), types.NamespacedName{Name: testPVC.Name, Namespace: testPVC.Namespace}, pvc)
			assert.NoError(t, err)

			vgr := &replicationv1alpha1.VolumeGroupReplication{}
			err = r.Get(context.TODO(), nsKey, vgr)
			assert.NoError(t, err)

			vgrPVCRefList := vgr.Status.PersistentVolumeClaimsRefList
			assert.Equal(t, 1, len(vgrPVCRefList))
			for _, pvc := range vgrPVCRefList {
				assert.Equal(t, pvc.Name, mockVGRPersistentVolumeClaim.Name)
			}
			// Check PVC annotation
			assert.Equal(t, volumeGroupReplication.Name, pvc.Annotations[replicationv1alpha1.VolumeGroupReplicationNameAnnotation])
			// Check VGRContent Created
			assert.NotEmpty(t, vgr.Spec.VolumeGroupReplicationContentName)
		} else {
			// Check reconcile didn't return any error
			assert.Equal(t, reconcile.Result{}, res)
			assert.NoError(t, err)

			vgr := &replicationv1alpha1.VolumeGroupReplication{}
			err = r.Get(context.TODO(), nsKey, vgr)
			assert.NoError(t, err)

			assert.Empty(t, vgr.Status.PersistentVolumeClaimsRefList)
		}
	}
}
