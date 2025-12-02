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

package utils

import (
	"context"
	"errors"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/csiaddons/v1alpha1"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DefaultFailedJobsHistoryLimit     int32 = 1
	DefaultSuccessfulJobsHistoryLimit int32 = 3

	DefaultBackoffLimit         = 6
	DefaultRetryDeadlineSeconds = 600
)

var (
	ErrConnNotFoundRequeueNeeded = errors.New("connection not found, requeue needed")
	ErrScheduleNotFound          = errors.New("schedule not found")
)

func setKeyrotationSpec(v *csiaddonsv1alpha1.EncryptionKeyRotationCronJob, schedule, pvcName string) {
	if v == nil {
		return
	}

	failedJobsHistoryLimit := DefaultFailedJobsHistoryLimit
	successfulJobsHistoryLimit := DefaultSuccessfulJobsHistoryLimit

	if v.Annotations == nil {
		v.Annotations = map[string]string{}
	}
	v.Annotations[CSIAddonsStateAnnotation] = CSIAddonsStateManaged

	v.Spec.Schedule = schedule
	v.Spec.FailedJobsHistoryLimit = &failedJobsHistoryLimit
	v.Spec.SuccessfulJobsHistoryLimit = &successfulJobsHistoryLimit

	v.Spec.JobSpec = csiaddonsv1alpha1.EncryptionKeyRotationJobTemplateSpec{
		Spec: csiaddonsv1alpha1.EncryptionKeyRotationJobSpec{
			Target:               csiaddonsv1alpha1.TargetSpec{PersistentVolumeClaim: pvcName},
			BackoffLimit:         DefaultBackoffLimit,
			RetryDeadlineSeconds: DefaultRetryDeadlineSeconds,
		},
	}
}

func setReclaimspaceSpec(v *csiaddonsv1alpha1.ReclaimSpaceCronJob, schedule, pvcName string) {
	if v == nil {
		return
	}

	failedJobsHistoryLimit := DefaultFailedJobsHistoryLimit
	successfulJobsHistoryLimit := DefaultSuccessfulJobsHistoryLimit

	if v.Annotations == nil {
		v.Annotations = map[string]string{}
	}
	v.Annotations[CSIAddonsStateAnnotation] = CSIAddonsStateManaged

	v.Spec.Schedule = schedule
	v.Spec.FailedJobsHistoryLimit = &failedJobsHistoryLimit
	v.Spec.SuccessfulJobsHistoryLimit = &successfulJobsHistoryLimit

	v.Spec.JobSpec = csiaddonsv1alpha1.ReclaimSpaceJobTemplateSpec{
		Spec: csiaddonsv1alpha1.ReclaimSpaceJobSpec{
			Target:               csiaddonsv1alpha1.TargetSpec{PersistentVolumeClaim: pvcName},
			BackoffLimit:         DefaultBackoffLimit,
			RetryDeadlineSeconds: DefaultRetryDeadlineSeconds,
		},
	}
}

func SetSpec(obj client.Object, schedule, pvcName string) {
	switch v := obj.(type) {
	case *csiaddonsv1alpha1.EncryptionKeyRotationCronJob:
		setKeyrotationSpec(v, schedule, pvcName)
	case *csiaddonsv1alpha1.ReclaimSpaceCronJob:
		setReclaimspaceSpec(v, schedule, pvcName)
	}
}

func GetSchedule(obj client.Object) string {
	switch v := obj.(type) {
	case *csiaddonsv1alpha1.EncryptionKeyRotationCronJob:
		return v.Spec.Schedule
	case *csiaddonsv1alpha1.ReclaimSpaceCronJob:
		return v.Spec.Schedule
	default:
		return ""
	}
}

// ExtractOwnerNameFromPVCObj extracts owner.Name from the object if it is
// of type `T` and has a PVC as its owner.
func ExtractOwnerNameFromPVCObj[T client.Object](rawObj client.Object) []string {
	// extract the owner from job object.
	job, ok := rawObj.(T)
	if !ok {
		return nil
	}
	owner := metav1.GetControllerOf(job)
	if owner == nil {
		return nil
	}
	if owner.APIVersion != "v1" || owner.Kind != "PersistentVolumeClaim" {
		return nil
	}

	return []string{owner.Name}
}

func CleanOldJobs(
	ctx context.Context,
	c client.Client,
	log logr.Logger,
	req ctrl.Request,
	objList client.ObjectList,
	expectedName string,
) (bool, error) {
	// We need to find all the CronJobs in the namespace of the PVC which
	// are owned by this controller and remove it if it doesn't match the expected name
	shouldRequeue := false

	if err := c.List(ctx, objList, client.InNamespace(req.Namespace), client.MatchingFields{JobOwnerKey: req.Name}); client.IgnoreNotFound(err) != nil {
		return shouldRequeue, err
	}

	items, err := meta.ExtractList(objList)
	if err != nil {
		return shouldRequeue, err
	}

	for _, item := range items {
		obj, ok := item.(client.Object)
		if !ok {
			// As long as objList is a k8s object
			// we will never hit this
			continue
		}
		objName := obj.GetName()

		// Only delete what we might have created
		if owner := metav1.GetControllerOf(obj); owner == nil ||
			owner.Kind != "PersistentVolumeClaim" ||
			owner.Name != req.Name {
			log.Info("Found an object without any owner", "jobName", objName)

			continue
		}

		// If the name does not match, delete the resource
		if objName != expectedName {
			if err := c.Delete(ctx, obj); client.IgnoreNotFound(err) != nil {
				return shouldRequeue, err
			}

			shouldRequeue = true
			log.Info("Deleted old job", "jobName", objName)
		}
	}

	return shouldRequeue, nil
}
