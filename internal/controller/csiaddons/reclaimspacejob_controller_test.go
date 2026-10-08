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
	"testing"
	"time"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/csiaddons/v1alpha1"
	"github.com/csi-addons/kubernetes-csi-addons/internal/proto"
	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

func TestSkipReason(t *testing.T) {
	tests := []struct {
		name                string
		nodeRequested       bool
		controllerRequested bool
		want                string
	}{
		{
			name:                "both requested",
			nodeRequested:       true,
			controllerRequested: true,
			want: "the volume is not attached to any node for node-side reclaim, " +
				"and no controller was found for controller-side reclaim",
		},
		{
			name:          "only node requested",
			nodeRequested: true,
			want: "the volume is not attached to any node for node-side reclaim, " +
				"and controller-side reclaim is not requested",
		},
		{
			name:                "only controller requested",
			controllerRequested: true,
			want: "node-side reclaim is not requested, " +
				"and no controller was found for controller-side reclaim",
		},
		{
			name: "none requested",
			want: "node-side reclaim is not requested, " +
				"and controller-side reclaim is not requested",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := skipReason(tt.nodeRequested, tt.controllerRequested)
			assert.Equal(t, tt.want, got)
		})
	}
}

var _ = ginkgo.Describe("ReclaimSpaceJob spec.operations", func() {
	ctx := context.Background()

	newJob := func(name string, operations []csiaddonsv1alpha1.ReclaimSpaceOperation) *csiaddonsv1alpha1.ReclaimSpaceJob {
		return &csiaddonsv1alpha1.ReclaimSpaceJob{
			ObjectMeta: v1.ObjectMeta{
				Name:      name,
				Namespace: "default",
			},
			Spec: csiaddonsv1alpha1.ReclaimSpaceJobSpec{
				Target:     csiaddonsv1alpha1.TargetSpec{PersistentVolumeClaim: "data-pvc"},
				Operations: operations,
			},
		}
	}

	ginkgo.It("defaults to the node operation when unset", func() {
		job := newJob("rsjob-operations-unset", nil)
		gomega.Expect(k8sClient.Create(ctx, job)).To(gomega.Succeed())
		ginkgo.DeferCleanup(k8sClient.Delete, ctx, job)

		gomega.Expect(job.Spec.Operations).To(gomega.Equal(csiaddonsv1alpha1.DefaultReclaimSpaceOperations()))
	})

	ginkgo.It("accepts both operations", func() {
		job := newJob("rsjob-operations-both", []csiaddonsv1alpha1.ReclaimSpaceOperation{
			csiaddonsv1alpha1.ReclaimSpaceOperationController,
			csiaddonsv1alpha1.ReclaimSpaceOperationNode,
		})
		gomega.Expect(k8sClient.Create(ctx, job)).To(gomega.Succeed())
		ginkgo.DeferCleanup(k8sClient.Delete, ctx, job)
	})

	// An empty list cannot be expressed through the typed client, the
	// omitempty json tag drops it and the API server applies the default
	// instead. Submit it the way `kubectl apply` would.
	ginkgo.It("rejects an empty list", func() {
		job := &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": csiaddonsv1alpha1.GroupVersion.String(),
				"kind":       "ReclaimSpaceJob",
				"metadata": map[string]any{
					"name":      "rsjob-operations-empty",
					"namespace": "default",
				},
				"spec": map[string]any{
					"target":     map[string]any{"persistentVolumeClaim": "data-pvc"},
					"operations": []any{},
				},
			},
		}
		// minItems rejects this on every supported API server, the CEL rule
		// adds the friendlier message once CEL validation is available.
		gomega.Expect(k8sClient.Create(ctx, job)).To(gomega.MatchError(gomega.Or(
			gomega.ContainSubstring("at least one reclaim space operation must be specified"),
			gomega.ContainSubstring("should have at least 1 items"))))
	})

	ginkgo.It("rejects an unknown operation", func() {
		job := newJob("rsjob-operations-unknown", []csiaddonsv1alpha1.ReclaimSpaceOperation{"Sparsify"})
		gomega.Expect(k8sClient.Create(ctx, job)).To(gomega.MatchError(
			gomega.ContainSubstring(`Unsupported value: "Sparsify": supported values: "Controller", "Node"`)))
	})

	ginkgo.It("rejects duplicate operations", func() {
		job := newJob("rsjob-operations-duplicate", []csiaddonsv1alpha1.ReclaimSpaceOperation{
			csiaddonsv1alpha1.ReclaimSpaceOperationNode,
			csiaddonsv1alpha1.ReclaimSpaceOperationNode,
		})
		gomega.Expect(k8sClient.Create(ctx, job)).To(gomega.MatchError(
			gomega.ContainSubstring("Duplicate value")))
	})
})
