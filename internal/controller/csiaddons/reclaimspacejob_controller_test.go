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
	"github.com/csi-addons/kubernetes-csi-addons/internal/controller/utils"
	"github.com/csi-addons/kubernetes-csi-addons/internal/proto"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	scv1 "k8s.io/api/storage/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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

func TestGetTargetDetailsSkipsInlineVolumeAttachment(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, corev1.AddToScheme(scheme))
	assert.NoError(t, scv1.AddToScheme(scheme))
	assert.NoError(t, csiaddonsv1alpha1.AddToScheme(scheme))

	pvName := "pv-csi-1"
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: v1.ObjectMeta{
			Name:      "pvc-1",
			Namespace: "default",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			VolumeName: pvName,
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: corev1.ClaimBound,
		},
	}
	pv := &corev1.PersistentVolume{
		ObjectMeta: v1.ObjectMeta{
			Name: pvName,
		},
		Spec: corev1.PersistentVolumeSpec{
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver:       "rbd.csi.ceph.com",
					VolumeHandle: "vol-1",
				},
			},
		},
	}
	inlineVA := &scv1.VolumeAttachment{
		ObjectMeta: v1.ObjectMeta{
			Name: "va-inline-0",
		},
		Spec: scv1.VolumeAttachmentSpec{
			Attacher: "inline.csi.example.com",
			NodeName: "worker-inline",
			Source: scv1.VolumeAttachmentSource{
				PersistentVolumeName: nil,
				InlineVolumeSpec:     &corev1.PersistentVolumeSpec{},
			},
		},
		Status: scv1.VolumeAttachmentStatus{
			Attached: true,
		},
	}
	matchingVA := &scv1.VolumeAttachment{
		ObjectMeta: v1.ObjectMeta{
			Name: "va-pv-1",
		},
		Spec: scv1.VolumeAttachmentSpec{
			Attacher: "rbd.csi.ceph.com",
			NodeName: "worker-target",
			Source: scv1.VolumeAttachmentSource{
				PersistentVolumeName: &pvName,
			},
		},
		Status: scv1.VolumeAttachmentStatus{
			Attached: true,
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(pvc, pv, inlineVA, matchingVA).
		Build()

	logger := logr.Discard()
	rsReconciler := &ReclaimSpaceJobReconciler{
		Client:  cl,
		Scheme:  scheme,
		Timeout: time.Minute,
	}
	rsDetails, err := rsReconciler.getTargetDetails(
		context.Background(),
		&logger,
		csiaddonsv1alpha1.ReclaimSpaceJobSpec{
			Target: csiaddonsv1alpha1.TargetSpec{
				PersistentVolumeClaim: "pvc-1",
			},
		},
		"default",
	)
	assert.NoError(t, err)
	assert.Equal(t, "worker-target", rsDetails.nodeID)
	assert.Equal(t, "rbd.csi.ceph.com", rsDetails.driverName)

	krReconciler := &EncryptionKeyRotationJobReconciler{
		Client:  cl,
		Scheme:  scheme,
		Timeout: time.Minute,
	}
	krDetails, err := krReconciler.getTargetDetails(
		context.Background(),
		&logger,
		csiaddonsv1alpha1.EncryptionKeyRotationJobSpec{
			Target: csiaddonsv1alpha1.TargetSpec{
				PersistentVolumeClaim: "pvc-1",
			},
		},
		"default",
	)
	assert.NoError(t, err)
	assert.Equal(t, "worker-target", krDetails.nodeID)
	assert.Equal(t, "rbd.csi.ceph.com", krDetails.driverName)
}

func TestCronJobNilHistoryLimitsAndReplaceConcurrent(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, corev1.AddToScheme(scheme))
	assert.NoError(t, csiaddonsv1alpha1.AddToScheme(scheme))

	createdAt := v1.NewTime(time.Now().Add(-2 * time.Minute))
	rsCron := &csiaddonsv1alpha1.ReclaimSpaceCronJob{
		ObjectMeta: v1.ObjectMeta{
			Name:              "rs-cron-1",
			Namespace:         "default",
			UID:               "uid-rs-cron-1",
			CreationTimestamp: createdAt,
		},
		Spec: csiaddonsv1alpha1.ReclaimSpaceCronJobSpec{
			Schedule:          "* * * * *",
			ConcurrencyPolicy: csiaddonsv1alpha1.ReplaceConcurrent,
			JobSpec: csiaddonsv1alpha1.ReclaimSpaceJobTemplateSpec{
				Spec: csiaddonsv1alpha1.ReclaimSpaceJobSpec{
					Target: csiaddonsv1alpha1.TargetSpec{
						PersistentVolumeClaim: "pvc-1",
					},
				},
			},
		},
	}
	krCron := &csiaddonsv1alpha1.EncryptionKeyRotationCronJob{
		ObjectMeta: v1.ObjectMeta{
			Name:              "kr-cron-1",
			Namespace:         "default",
			UID:               "uid-kr-cron-1",
			CreationTimestamp: createdAt,
		},
		Spec: csiaddonsv1alpha1.EncryptionKeyRotationCronJobSpec{
			Schedule:          "* * * * *",
			ConcurrencyPolicy: csiaddonsv1alpha1.ReplaceConcurrent,
			JobSpec: csiaddonsv1alpha1.EncryptionKeyRotationJobTemplateSpec{
				Spec: csiaddonsv1alpha1.EncryptionKeyRotationJobSpec{
					Target: csiaddonsv1alpha1.TargetSpec{
						PersistentVolumeClaim: "pvc-1",
					},
				},
			},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(rsCron, krCron).
		WithIndex(&csiaddonsv1alpha1.ReclaimSpaceJob{}, utils.JobOwnerKey, func(rawObj client.Object) []string {
			job := rawObj.(*csiaddonsv1alpha1.ReclaimSpaceJob)
			owner := v1.GetControllerOf(job)
			if owner == nil {
				return nil
			}
			return []string{owner.Name}
		}).
		WithIndex(&csiaddonsv1alpha1.EncryptionKeyRotationJob{}, utils.JobOwnerKey, func(rawObj client.Object) []string {
			job := rawObj.(*csiaddonsv1alpha1.EncryptionKeyRotationJob)
			owner := v1.GetControllerOf(job)
			if owner == nil {
				return nil
			}
			return []string{owner.Name}
		}).
		WithObjects(rsCron, krCron).
		Build()

	rsCronReconciler := &ReclaimSpaceCronJobReconciler{
		Client: cl,
		Scheme: scheme,
	}
	_, err := rsCronReconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "rs-cron-1", Namespace: "default"},
	})
	assert.NoError(t, err)

	var rsJobs csiaddonsv1alpha1.ReclaimSpaceJobList
	assert.NoError(t, cl.List(context.Background(), &rsJobs))
	assert.Len(t, rsJobs.Items, 1)

	// Mark the existing active job as scheduled 2 minutes ago so the next schedule is due,
	// and verify ReplaceConcurrent deletes the active job and creates a replacement job.
	oldJob := &rsJobs.Items[0]
	oldJob.Annotations[scheduledTimeAnnotation] = time.Now().Add(-2 * time.Minute).Format(time.RFC3339)
	assert.NoError(t, cl.Update(context.Background(), oldJob))

	_, err = rsCronReconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "rs-cron-1", Namespace: "default"},
	})
	assert.NoError(t, err)
	assert.NoError(t, cl.List(context.Background(), &rsJobs))
	assert.Len(t, rsJobs.Items, 1)
	assert.NotEqual(t, oldJob.Annotations[scheduledTimeAnnotation], rsJobs.Items[0].Annotations[scheduledTimeAnnotation])

	krCronReconciler := &EncryptionKeyRotationCronJobReconciler{
		Client: cl,
		Scheme: scheme,
	}
	_, err = krCronReconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "kr-cron-1", Namespace: "default"},
	})
	assert.NoError(t, err)

	var krJobs csiaddonsv1alpha1.EncryptionKeyRotationJobList
	assert.NoError(t, cl.List(context.Background(), &krJobs))
	assert.Len(t, krJobs.Items, 1)
}
