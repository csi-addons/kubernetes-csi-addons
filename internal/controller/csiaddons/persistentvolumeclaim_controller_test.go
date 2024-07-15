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
	"strings"
	"testing"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/csiaddons/v1alpha1"
	"github.com/csi-addons/kubernetes-csi-addons/internal/connection"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func TestConstructRSCronJob(t *testing.T) {
	type args struct {
		name      string
		namespace string
		schedule  string
		pvcName   string
	}

	failedJobsHistoryLimit := defaultFailedJobsHistoryLimit
	successfulJobsHistoryLimit := defaultSuccessfulJobsHistoryLimit
	tests := []struct {
		name string
		args args
		want *csiaddonsv1alpha1.ReclaimSpaceCronJob
	}{
		{
			name: "check output",
			args: args{
				name:      "hello",
				namespace: "default",
				schedule:  "@yearly",
				pvcName:   "pvc-1",
			},
			want: &csiaddonsv1alpha1.ReclaimSpaceCronJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hello",
					Namespace: "default",
				},
				Spec: csiaddonsv1alpha1.ReclaimSpaceCronJobSpec{
					Schedule: "@yearly",
					JobSpec: csiaddonsv1alpha1.ReclaimSpaceJobTemplateSpec{
						Spec: csiaddonsv1alpha1.ReclaimSpaceJobSpec{
							Target:               csiaddonsv1alpha1.TargetSpec{PersistentVolumeClaim: "pvc-1"},
							BackoffLimit:         defaultBackoffLimit,
							RetryDeadlineSeconds: defaultRetryDeadlineSeconds,
						},
					},
					FailedJobsHistoryLimit:     &failedJobsHistoryLimit,
					SuccessfulJobsHistoryLimit: &successfulJobsHistoryLimit,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := constructRSCronJob(tt.args.name, tt.args.namespace, tt.args.schedule, tt.args.pvcName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractOwnerNameFromPVCObj(t *testing.T) {
	type args struct {
		rawObj client.Object
	}
	boolTrue := true
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "nil obj",
			args: args{
				rawObj: nil,
			},
			want: nil,
		},
		{
			name: "non reclaimSpaceCronJob obj",
			args: args{
				rawObj: &csiaddonsv1alpha1.ReclaimSpaceJob{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "v1",
								Kind:       "PersistentVolumeClaim",
								Name:       "owner",
								Controller: &boolTrue,
							},
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "reclaimSpaceCron obj with pvc owner",
			args: args{
				rawObj: &csiaddonsv1alpha1.ReclaimSpaceCronJob{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "v1",
								Kind:       "PersistentVolumeClaim",
								Name:       "owner",
								Controller: &boolTrue,
							},
						},
					},
				},
			},
			want: []string{"owner"},
		},
		{
			name: "reclaimSpaceCron obj with pv owner",
			args: args{
				rawObj: &csiaddonsv1alpha1.ReclaimSpaceCronJob{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "v1",
								Kind:       "PersistentVolume",
								Name:       "owner",
								Controller: &boolTrue,
							},
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "pvc obj with no owner",
			args: args{
				rawObj: &csiaddonsv1alpha1.ReclaimSpaceCronJob{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{},
					},
				},
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractOwnerNameFromPVCObj(tt.args.rawObj)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGenerateCronJobName(t *testing.T) {
	type args struct {
		parentName string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "test 1",
			args: args{
				parentName: "sample",
			},
			want: "sample-",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateCronJobName(tt.args.parentName)
			assert.True(t, strings.HasPrefix(got, tt.want))
		})
	}
}

func TestGetScheduleFromAnnotation(t *testing.T) {
	type args struct {
		logger      *logr.Logger
		annotations map[string]string
	}
	logger := log.FromContext(context.TODO())
	tests := []struct {
		name  string
		args  args
		want  string
		want1 bool
	}{
		{
			name: "no scheduling annotation set",
			args: args{
				logger:      &logger,
				annotations: map[string]string{},
			},
			want:  "",
			want1: false,
		},
		{
			name: "valid scheduling annotation set",
			args: args{
				logger: &logger,
				annotations: map[string]string{
					rsCronJobScheduleTimeAnnotation: "@weekly",
				},
			},
			want:  "@weekly",
			want1: true,
		},
		{
			name: "invalid scheduling annotation set",
			args: args{
				logger: &logger,
				annotations: map[string]string{
					rsCronJobScheduleTimeAnnotation: "@daytime",
				},
			},
			want:  defaultSchedule,
			want1: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := getScheduleFromAnnotation(rsCronJobScheduleTimeAnnotation, tt.args.logger, tt.args.annotations)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.want1, got1)
		})
	}
}

func TestDetermineScheduleAndRequeue(t *testing.T) {
	type args struct {
		pvcAnnotations map[string]string
		nsAnnotations  map[string]string
		scAnnotations  map[string]string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "pvc annotation set",
			args: args{
				pvcAnnotations: map[string]string{rsCronJobScheduleTimeAnnotation: "@daily"},
			},
			want: "@daily",
		},
		{
			name: "sc annotation set",
			args: args{
				scAnnotations: map[string]string{rsCronJobScheduleTimeAnnotation: "@monthly"},
			},
			want: "@monthly",
		},
		{
			name: "pvc & sc annotation set",
			args: args{
				pvcAnnotations: map[string]string{rsCronJobScheduleTimeAnnotation: "@daily"},
				scAnnotations:  map[string]string{rsCronJobScheduleTimeAnnotation: "@weekly"},
			},
			want: "@daily",
		},
	}

	ctx := context.TODO()
	logger := logr.Discard()
	client := fake.NewClientBuilder().Build()
	driverName := "test-driver"

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-namespace",
		},
	}
	sc := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-sc",
		},
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pvc",
			Namespace: ns.Name,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &sc.Name,
		},
	}

	r := &PersistentVolumeClaimReconciler{
		Client:   client,
		ConnPool: connection.NewConnectionPool(),
	}

	// Create the namespace, storage class, and PVC
	err := r.Client.Create(ctx, ns)
	assert.NoError(t, err)
	err = r.Client.Create(ctx, sc)
	assert.NoError(t, err)
	err = r.Client.Create(ctx, pvc)
	assert.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pvc.Annotations = tt.args.pvcAnnotations
			ns.Annotations = tt.args.nsAnnotations
			sc.Annotations = tt.args.scAnnotations

			err = r.Client.Update(ctx, ns)
			assert.NoError(t, err)
			err = r.Client.Update(ctx, sc)
			assert.NoError(t, err)
			err = r.Client.Update(ctx, pvc)
			assert.NoError(t, err)

			schedule, error := r.determineScheduleAndRequeue(ctx, &logger, pvc, driverName, rsCronJobScheduleTimeAnnotation)
			assert.NoError(t, error)
			assert.Equal(t, tt.want, schedule)
		})
	}

	t.Run("empty StorageClassName for static pv", func(t *testing.T) {
		emptyScName := ""
		pvc.Spec.StorageClassName = &emptyScName
		pvc.Annotations = nil
		schedule, error := r.determineScheduleAndRequeue(ctx, &logger, pvc, driverName, rsCronJobScheduleTimeAnnotation)
		assert.ErrorIs(t, error, ErrScheduleNotFound)
		assert.Equal(t, "", schedule)
	})

	// test for StorageClassName not found
	t.Run("StorageClassName not found", func(t *testing.T) {
		sc.Name = "non-existent-sc"
		pvc.Spec.StorageClassName = &sc.Name
		pvc.Annotations = nil
		schedule, error := r.determineScheduleAndRequeue(ctx, &logger, pvc, driverName, rsCronJobScheduleTimeAnnotation)
		assert.ErrorIs(t, error, ErrScheduleNotFound)
		assert.Equal(t, "", schedule)
	})

}
