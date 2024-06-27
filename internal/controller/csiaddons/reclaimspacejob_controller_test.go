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
	"testing"
	"time"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/csiaddons/v1alpha1"
	"github.com/csi-addons/kubernetes-csi-addons/internal/proto"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSetFailedCondition(t *testing.T) {
	type args struct {
		conditions         *[]v1.Condition
		message            string
		observedGeneration int64
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "overwrite existing failed condition",
			args: args{
				conditions: &[]v1.Condition{
					{
						Type:               conditionFailed,
						Status:             v1.ConditionTrue,
						ObservedGeneration: 0,
						LastTransitionTime: v1.NewTime(time.Now()),
						Reason:             reasonFailed,
						Message:            "err 1",
					},
				},
				message:            "err 2",
				observedGeneration: 3,
			},
		},
		{
			name: "add failed condition",
			args: args{
				conditions:         &[]v1.Condition{},
				message:            "err 1",
				observedGeneration: 3,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setFailedCondition(tt.args.conditions, tt.args.message, tt.args.observedGeneration)
			assert.Equal(t, tt.args.message, (*tt.args.conditions)[0].Message)
			assert.Equal(t, tt.args.observedGeneration, (*tt.args.conditions)[0].ObservedGeneration)
		})
	}
}

func TestValidateReclaimSpaceJobSpec(t *testing.T) {
	type args struct {
		rsJob *csiaddonsv1alpha1.ReclaimSpaceJob
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "empty ReclaimSpaceJob.Spec.Target.PersistentVolumeClaim",
			args: args{
				rsJob: &csiaddonsv1alpha1.ReclaimSpaceJob{},
			},
			wantErr: true,
		},
		{
			name: "filled ReclaimSpaceJob.Spec.Target.PersistentVolumeClaim",
			args: args{
				rsJob: &csiaddonsv1alpha1.ReclaimSpaceJob{
					Spec: csiaddonsv1alpha1.ReclaimSpaceJobSpec{
						Target: csiaddonsv1alpha1.TargetSpec{
							PersistentVolumeClaim: "pvc-1",
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateReclaimSpaceJobSpec(tt.args.rsJob); (err != nil) != tt.wantErr {
				t.Errorf("validateReclaimSpaceJobSpec() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCalculateReclaimedSpace(t *testing.T) {
	type args struct {
		PreUsage  *proto.StorageConsumption
		PostUsage *proto.StorageConsumption
	}
	pre := int64(0)
	post := int64(5)
	result := post - pre
	result2 := int64(0)
	tests := []struct {
		name string
		args args
		want *int64
	}{
		{
			name: "both pre and post usage present",
			args: args{
				PreUsage: &proto.StorageConsumption{
					UsageBytes: pre,
				},
				PostUsage: &proto.StorageConsumption{
					UsageBytes: post,
				},
			},
			want: &result,
		},
		{
			name: "only post usage present",
			args: args{
				PreUsage: nil,
				PostUsage: &proto.StorageConsumption{
					UsageBytes: post,
				},
			},
			want: nil,
		},
		{
			name: "only pre usage present",
			args: args{
				PreUsage: &proto.StorageConsumption{
					UsageBytes: pre,
				},
				PostUsage: nil,
			},
			want: nil,
		},
		{
			name: "reclaimed space is negative",
			args: args{
				PreUsage: &proto.StorageConsumption{
					UsageBytes: pre,
				},
				PostUsage: &proto.StorageConsumption{
					UsageBytes: -post,
				},
			},
			want: &result2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateReclaimedSpace(tt.args.PreUsage, tt.args.PostUsage)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCanNodeReclaimSpace(t *testing.T) {
	tests := []struct {
		name string
		td   targetDetails
		want bool
	}{
		{
			name: "empty nodeID",
			td: targetDetails{
				driverName: "csi.example.com",
				pvName:     "pvc-a8a5c531-9f88-4fc8-b35d-564585fb42a8",
				nodeID:     "",
			},
			want: false,
		},
		{
			name: "non-empty nodeID",
			td: targetDetails{
				driverName: "csi.example.com",
				pvName:     "pvc-a8a5c531-9f88-4fc8-b35d-564585fb42a8",
				nodeID:     "worker-1",
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.td.canNodeReclaimSpace()
			assert.Equal(t, tt.want, got)
		})
	}
}
