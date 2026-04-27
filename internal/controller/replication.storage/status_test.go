/*
Copyright 2026 The Kubernetes-CSI-Addons Authors.

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

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/csi-addons/kubernetes-csi-addons/api/replication.storage/v1alpha1"
)

func TestIsDestinationInfoAvailable(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		conditions []metav1.Condition
		want       bool
	}{
		{
			name:       "no conditions",
			conditions: nil,
			want:       false,
		},
		{
			name: "condition absent",
			conditions: []metav1.Condition{
				{
					Type:   v1alpha1.ConditionCompleted,
					Status: metav1.ConditionTrue,
				},
			},
			want: false,
		},
		{
			name: "condition present and true",
			conditions: []metav1.Condition{
				{
					Type:   v1alpha1.ConditionDestinationInfoAvailable,
					Status: metav1.ConditionTrue,
				},
			},
			want: true,
		},
		{
			name: "condition present but false",
			conditions: []metav1.Condition{
				{
					Type:   v1alpha1.ConditionDestinationInfoAvailable,
					Status: metav1.ConditionFalse,
				},
			},
			want: false,
		},
		{
			name: "condition present but unknown",
			conditions: []metav1.Condition{
				{
					Type:   v1alpha1.ConditionDestinationInfoAvailable,
					Status: metav1.ConditionUnknown,
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		newtt := tt
		t.Run(newtt.name, func(t *testing.T) {
			t.Parallel()
			got := isDestinationInfoAvailable(newtt.conditions)
			assert.Equal(t, newtt.want, got)
		})
	}
}

func TestSetDestinationInfoAvailableCondition(t *testing.T) {
	t.Parallel()
	conditions := []metav1.Condition{}
	setDestinationInfoAvailableCondition(&conditions, 1, pvcDataSource)

	assert.Len(t, conditions, 1)
	assert.Equal(t, v1alpha1.ConditionDestinationInfoAvailable, conditions[0].Type)
	assert.Equal(t, metav1.ConditionTrue, conditions[0].Status)
	assert.Equal(t, v1alpha1.DestinationInfoUpdated, conditions[0].Reason)
	assert.Contains(t, conditions[0].Message, string(v1alpha1.Volume))
	assert.Contains(t, conditions[0].Message, v1alpha1.MessageDestinationInfoAvailable)
	assert.Equal(t, int64(1), conditions[0].ObservedGeneration)
}

func TestSetDestinationInfoPendingCondition(t *testing.T) {
	t.Parallel()
	conditions := []metav1.Condition{}
	setDestinationInfoPendingCondition(&conditions, 2, volumeGroupReplicationDataSource)

	assert.Len(t, conditions, 1)
	assert.Equal(t, v1alpha1.ConditionDestinationInfoAvailable, conditions[0].Type)
	assert.Equal(t, metav1.ConditionFalse, conditions[0].Status)
	assert.Equal(t, v1alpha1.DestinationInfoPending, conditions[0].Reason)
	assert.Contains(t, conditions[0].Message, string(v1alpha1.VolumeGroup))
	assert.Contains(t, conditions[0].Message, v1alpha1.MessageDestinationInfoPending)
	assert.Equal(t, int64(2), conditions[0].ObservedGeneration)
}

func TestSetDestinationInfoFailedCondition(t *testing.T) {
	t.Parallel()
	conditions := []metav1.Condition{}
	setDestinationInfoFailedCondition(&conditions, 3, pvcDataSource, "rpc unavailable")

	assert.Len(t, conditions, 1)
	assert.Equal(t, v1alpha1.ConditionDestinationInfoAvailable, conditions[0].Type)
	assert.Equal(t, metav1.ConditionFalse, conditions[0].Status)
	assert.Equal(t, v1alpha1.FailedToGetDestinationInfo, conditions[0].Reason)
	assert.Contains(t, conditions[0].Message, v1alpha1.MessageDestinationInfoFailed)
	assert.Contains(t, conditions[0].Message, "rpc unavailable")
	assert.Equal(t, int64(3), conditions[0].ObservedGeneration)
}

func TestDestinationInfoConditionTransitions(t *testing.T) {
	t.Parallel()

	// Start with pending
	conditions := []metav1.Condition{}
	setDestinationInfoPendingCondition(&conditions, 1, pvcDataSource)
	assert.False(t, isDestinationInfoAvailable(conditions))

	// Transition to failed
	setDestinationInfoFailedCondition(&conditions, 1, pvcDataSource, "not ready")
	assert.False(t, isDestinationInfoAvailable(conditions))
	assert.Len(t, conditions, 1) // should update in place, not add

	// Transition to available
	setDestinationInfoAvailableCondition(&conditions, 2, pvcDataSource)
	assert.True(t, isDestinationInfoAvailable(conditions))
	assert.Len(t, conditions, 1) // still one condition, updated in place

	// Transition back to pending (e.g., membership change)
	setDestinationInfoPendingCondition(&conditions, 3, pvcDataSource)
	assert.False(t, isDestinationInfoAvailable(conditions))
	assert.Len(t, conditions, 1)
}
