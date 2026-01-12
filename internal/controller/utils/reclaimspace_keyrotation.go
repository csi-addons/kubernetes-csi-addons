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
	"errors"
	"hash/fnv"
	"time"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/csiaddons/v1alpha1"
	"github.com/robfig/cron/v3"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiTypes "k8s.io/apimachinery/pkg/types"
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

// GetStaggeredNext returns a deterministic, UID-based staggered time computed from a schedule.
// The staggered window is capped at a maximum of 2hrs but is adjusted for smaller intervals.
func GetStaggeredNext(uid apiTypes.UID, nextTime time.Time, sched cron.Schedule) time.Time {
	// cron does not expose interval directly
	// We can determine it by subtracting two consecutive intervals
	afterNext := sched.Next(nextTime)
	interval := afterNext.Sub(nextTime)

	// We do not want the stagger window to be any larger than a max of 2hrs
	const maxCap = 2 * time.Hour
	staggerWindow := min(interval, maxCap)

	// To prevent the schedule from jumping in bw the reconciles,
	// we use the UID and hash it, this way it remains deterministic
	h := fnv.New32a()
	if _, err := h.Write([]byte(string(uid))); err != nil {
		return nextTime
	}
	hash := h.Sum32()

	// Just a safety net
	stgrWindowSeconds := int64(staggerWindow.Seconds())
	if stgrWindowSeconds <= 0 {
		return nextTime
	}
	offsetSeconds := int64(hash) % stgrWindowSeconds // Modulo ensures it to be < stgrWindowSeconds

	return nextTime.Add(time.Duration(offsetSeconds) * time.Second)
}
