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
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type mockObject struct {
	client.Object
	annotations map[string]string
}

func (m *mockObject) GetAnnotations() map[string]string {
	return m.annotations
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
			got := extractOwnerNameFromPVCObj[*csiaddonsv1alpha1.ReclaimSpaceCronJob](tt.args.rawObj)
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

	// test for StorageClassName is nil
	t.Run("StorageClassName is nil", func(t *testing.T) {
		pvc.Spec.StorageClassName = nil
		pvc.Annotations = nil
		schedule, error := r.determineScheduleAndRequeue(ctx, &logger, pvc, driverName, rsCronJobScheduleTimeAnnotation)
		assert.ErrorIs(t, error, ErrScheduleNotFound)
		assert.Equal(t, "", schedule)
	})
}

func TestAnnotationValueMissing(t *testing.T) {
	tests := []struct {
		name           string
		scAnnotations  map[string]string
		pvcAnnotations map[string]string
		keys           []string
		expected       bool
	}{
		{
			name:           "No annotations",
			scAnnotations:  map[string]string{},
			pvcAnnotations: map[string]string{},
			keys:           []string{"key1", "key2"},
			expected:       false,
		},
		{
			name:           "SC has annotation, PVC doesn't",
			scAnnotations:  map[string]string{"key1": "value1"},
			pvcAnnotations: map[string]string{},
			keys:           []string{"key1"},
			expected:       true,
		},
		{
			name:           "Both SC and PVC have annotation",
			scAnnotations:  map[string]string{"key1": "value1"},
			pvcAnnotations: map[string]string{"key1": "value1"},
			keys:           []string{"key1"},
			expected:       false,
		},
		{
			name:           "SC has multiple annotations, PVC missing one",
			scAnnotations:  map[string]string{"key1": "value1", "key2": "value2"},
			pvcAnnotations: map[string]string{"key1": "value1"},
			keys:           []string{"key1", "key2"},
			expected:       true,
		},
		{
			name:           "SC has annotation not in keys",
			scAnnotations:  map[string]string{"key3": "value3"},
			pvcAnnotations: map[string]string{},
			keys:           []string{"key1", "key2"},
			expected:       false,
		},
		{
			name:           "Empty keys slice",
			scAnnotations:  map[string]string{"key1": "value1"},
			pvcAnnotations: map[string]string{},
			keys:           []string{},
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := annotationValueMissing(tt.scAnnotations, tt.pvcAnnotations, tt.keys)
			if result != tt.expected {
				t.Errorf("AnnotationValueMissing() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestAnnotationValueChanged(t *testing.T) {
	tests := []struct {
		name           string
		oldAnnotations map[string]string
		newAnnotations map[string]string
		keys           []string
		expected       bool
	}{
		{
			name:           "No changes",
			oldAnnotations: map[string]string{"key1": "value1", "key2": "value2"},
			newAnnotations: map[string]string{"key1": "value1", "key2": "value2"},
			keys:           []string{"key1", "key2"},
			expected:       false,
		},
		{
			name:           "Value changed",
			oldAnnotations: map[string]string{"key1": "value1", "key2": "value2"},
			newAnnotations: map[string]string{"key1": "value1", "key2": "newvalue2"},
			keys:           []string{"key1", "key2"},
			expected:       true,
		},
		{
			name:           "Key added",
			oldAnnotations: map[string]string{"key1": "value1"},
			newAnnotations: map[string]string{"key1": "value1", "key2": "value2"},
			keys:           []string{"key1", "key2"},
			expected:       true,
		},
		{
			name:           "Key removed",
			oldAnnotations: map[string]string{"key1": "value1", "key2": "value2"},
			newAnnotations: map[string]string{"key1": "value1"},
			keys:           []string{"key1", "key2"},
			expected:       true,
		},
		{
			name:           "Change in non-specified key",
			oldAnnotations: map[string]string{"key1": "value1", "key2": "value2", "key3": "value3"},
			newAnnotations: map[string]string{"key1": "value1", "key2": "value2", "key3": "newvalue3"},
			keys:           []string{"key1", "key2"},
			expected:       false,
		},
		{
			name:           "Empty keys slice",
			oldAnnotations: map[string]string{"key1": "value1"},
			newAnnotations: map[string]string{"key1": "newvalue1"},
			keys:           []string{},
			expected:       false,
		},
		{
			name:           "Nil maps",
			oldAnnotations: nil,
			newAnnotations: nil,
			keys:           []string{"key1"},
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := annotationValueChanged(tt.oldAnnotations, tt.newAnnotations, tt.keys)
			if result != tt.expected {
				t.Errorf("AnnotationValueChanged() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCreateAnnotationPredicate(t *testing.T) {
	tests := []struct {
		name           string
		oldAnnotations map[string]string
		newAnnotations map[string]string
		annotations    []string
		expected       bool
	}{
		{
			name:           "No change in specified annotations",
			oldAnnotations: map[string]string{"key1": "value1", "key2": "value2"},
			newAnnotations: map[string]string{"key1": "value1", "key2": "value2"},
			annotations:    []string{"key1", "key2"},
			expected:       false,
		},
		{
			name:           "Change in specified annotation",
			oldAnnotations: map[string]string{"key1": "value1", "key2": "value2"},
			newAnnotations: map[string]string{"key1": "newvalue1", "key2": "value2"},
			annotations:    []string{"key1"},
			expected:       true,
		},
		{
			name:           "Change in unspecified annotation",
			oldAnnotations: map[string]string{"key1": "value1", "key2": "value2"},
			newAnnotations: map[string]string{"key1": "value1", "key2": "newvalue2"},
			annotations:    []string{"key1"},
			expected:       false,
		},
		{
			name:           "New annotation added",
			oldAnnotations: map[string]string{"key1": "value1"},
			newAnnotations: map[string]string{"key1": "value1", "key2": "value2"},
			annotations:    []string{"key1", "key2"},
			expected:       true,
		},
		{
			name:           "Specified annotation removed",
			oldAnnotations: map[string]string{"key1": "value1", "key2": "value2"},
			newAnnotations: map[string]string{"key1": "value1"},
			annotations:    []string{"key1", "key2"},
			expected:       true,
		},
		{
			name:           "Empty annotations list",
			oldAnnotations: map[string]string{"key1": "value1", "key2": "value2"},
			newAnnotations: map[string]string{"key1": "newvalue1", "key2": "newvalue2"},
			annotations:    []string{},
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			predicate := createAnnotationPredicate(tt.annotations...)
			result := predicate.UpdateFunc(event.UpdateEvent{
				ObjectOld: &mockObject{annotations: tt.oldAnnotations},
				ObjectNew: &mockObject{annotations: tt.newAnnotations},
			})
			if result != tt.expected {
				t.Errorf("CreateAnnotationPredicate() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestConstructKRCronJob(t *testing.T) {
	failedJobHistoryLimit := defaultFailedJobsHistoryLimit
	successfulJobsHistoryLimit := defaultSuccessfulJobsHistoryLimit
	tests := []struct {
		name      string
		cronName  string
		namespace string
		schedule  string
		pvcName   string
		expected  *csiaddonsv1alpha1.EncryptionKeyRotationCronJob
	}{
		{
			name:      "Basic KR CronJob",
			cronName:  "test-kr-cron",
			namespace: "default",
			schedule:  "0 1 * * *",
			pvcName:   "test-pvc",
			expected: &csiaddonsv1alpha1.EncryptionKeyRotationCronJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kr-cron",
					Namespace: "default",
				},
				Spec: csiaddonsv1alpha1.EncryptionKeyRotationCronJobSpec{
					Schedule: "0 1 * * *",
					JobSpec: csiaddonsv1alpha1.EncryptionKeyRotationJobTemplateSpec{
						Spec: csiaddonsv1alpha1.EncryptionKeyRotationJobSpec{
							Target: csiaddonsv1alpha1.TargetSpec{
								PersistentVolumeClaim: "test-pvc",
							},
							BackoffLimit:         defaultBackoffLimit,
							RetryDeadlineSeconds: defaultRetryDeadlineSeconds,
						},
					},
					FailedJobsHistoryLimit:     &failedJobHistoryLimit,
					SuccessfulJobsHistoryLimit: &successfulJobsHistoryLimit,
				},
			},
		},
		{
			name:      "KR CronJob with empty schedule",
			cronName:  "empty-schedule-cron",
			namespace: "kube-system",
			schedule:  "",
			pvcName:   "data-pvc",
			expected: &csiaddonsv1alpha1.EncryptionKeyRotationCronJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "empty-schedule-cron",
					Namespace: "kube-system",
				},
				Spec: csiaddonsv1alpha1.EncryptionKeyRotationCronJobSpec{
					Schedule: "",
					JobSpec: csiaddonsv1alpha1.EncryptionKeyRotationJobTemplateSpec{
						Spec: csiaddonsv1alpha1.EncryptionKeyRotationJobSpec{
							Target: csiaddonsv1alpha1.TargetSpec{
								PersistentVolumeClaim: "data-pvc",
							},
							BackoffLimit:         defaultBackoffLimit,
							RetryDeadlineSeconds: defaultRetryDeadlineSeconds,
						},
					},
					FailedJobsHistoryLimit:     &failedJobHistoryLimit,
					SuccessfulJobsHistoryLimit: &successfulJobsHistoryLimit,
				},
			},
		},
		{
			name:      "KR CronJob with special characters",
			cronName:  "special-!@#$%^&*()-cron",
			namespace: "test-ns",
			schedule:  "*/5 * * * *",
			pvcName:   "pvc-with-special-chars-!@#$%^&*()",
			expected: &csiaddonsv1alpha1.EncryptionKeyRotationCronJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "special-!@#$%^&*()-cron",
					Namespace: "test-ns",
				},
				Spec: csiaddonsv1alpha1.EncryptionKeyRotationCronJobSpec{
					Schedule: "*/5 * * * *",
					JobSpec: csiaddonsv1alpha1.EncryptionKeyRotationJobTemplateSpec{
						Spec: csiaddonsv1alpha1.EncryptionKeyRotationJobSpec{
							Target: csiaddonsv1alpha1.TargetSpec{
								PersistentVolumeClaim: "pvc-with-special-chars-!@#$%^&*()",
							},
							BackoffLimit:         defaultBackoffLimit,
							RetryDeadlineSeconds: defaultRetryDeadlineSeconds,
						},
					},
					FailedJobsHistoryLimit:     &failedJobHistoryLimit,
					SuccessfulJobsHistoryLimit: &successfulJobsHistoryLimit,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := constructKRCronJob(tt.cronName, tt.namespace, tt.schedule, tt.pvcName)
			assert.Equal(t, tt.expected, result)
		})
	}
}
func TestConstructRSCronJob(t *testing.T) {
	failedJobHistoryLimit := defaultFailedJobsHistoryLimit
	successfulJobsHistoryLimit := defaultSuccessfulJobsHistoryLimit
	tests := []struct {
		name      string
		cronName  string
		namespace string
		schedule  string
		pvcName   string
		expected  *csiaddonsv1alpha1.ReclaimSpaceCronJob
	}{
		{
			name:      "Basic RS CronJob",
			cronName:  "test-rs-cron",
			namespace: "default",
			schedule:  "0 2 * * *",
			pvcName:   "test-pvc",
			expected: &csiaddonsv1alpha1.ReclaimSpaceCronJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rs-cron",
					Namespace: "default",
				},
				Spec: csiaddonsv1alpha1.ReclaimSpaceCronJobSpec{
					Schedule: "0 2 * * *",
					JobSpec: csiaddonsv1alpha1.ReclaimSpaceJobTemplateSpec{
						Spec: csiaddonsv1alpha1.ReclaimSpaceJobSpec{
							Target: csiaddonsv1alpha1.TargetSpec{
								PersistentVolumeClaim: "test-pvc",
							},
							BackoffLimit:         defaultBackoffLimit,
							RetryDeadlineSeconds: defaultRetryDeadlineSeconds,
						},
					},
					FailedJobsHistoryLimit:     &failedJobHistoryLimit,
					SuccessfulJobsHistoryLimit: &successfulJobsHistoryLimit,
				},
			},
		},
		{
			name:      "RS CronJob with empty schedule",
			cronName:  "empty-schedule-cron",
			namespace: "kube-system",
			schedule:  "",
			pvcName:   "data-pvc",
			expected: &csiaddonsv1alpha1.ReclaimSpaceCronJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "empty-schedule-cron",
					Namespace: "kube-system",
				},
				Spec: csiaddonsv1alpha1.ReclaimSpaceCronJobSpec{
					Schedule: "",
					JobSpec: csiaddonsv1alpha1.ReclaimSpaceJobTemplateSpec{
						Spec: csiaddonsv1alpha1.ReclaimSpaceJobSpec{
							Target: csiaddonsv1alpha1.TargetSpec{
								PersistentVolumeClaim: "data-pvc",
							},
							BackoffLimit:         defaultBackoffLimit,
							RetryDeadlineSeconds: defaultRetryDeadlineSeconds,
						},
					},
					FailedJobsHistoryLimit:     &failedJobHistoryLimit,
					SuccessfulJobsHistoryLimit: &successfulJobsHistoryLimit,
				},
			},
		},
		{
			name:      "RS CronJob with special characters",
			cronName:  "special-!@#$%^&*()-cron",
			namespace: "test-ns",
			schedule:  "*/10 * * * *",
			pvcName:   "pvc-with-special-chars-!@#$%^&*()",
			expected: &csiaddonsv1alpha1.ReclaimSpaceCronJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "special-!@#$%^&*()-cron",
					Namespace: "test-ns",
				},
				Spec: csiaddonsv1alpha1.ReclaimSpaceCronJobSpec{
					Schedule: "*/10 * * * *",
					JobSpec: csiaddonsv1alpha1.ReclaimSpaceJobTemplateSpec{
						Spec: csiaddonsv1alpha1.ReclaimSpaceJobSpec{
							Target: csiaddonsv1alpha1.TargetSpec{
								PersistentVolumeClaim: "pvc-with-special-chars-!@#$%^&*()",
							},
							BackoffLimit:         defaultBackoffLimit,
							RetryDeadlineSeconds: defaultRetryDeadlineSeconds,
						},
					},
					FailedJobsHistoryLimit:     &failedJobHistoryLimit,
					SuccessfulJobsHistoryLimit: &successfulJobsHistoryLimit,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := constructRSCronJob(tt.cronName, tt.namespace, tt.schedule, tt.pvcName)
			assert.Equal(t, tt.expected, result)
		})
	}
}
