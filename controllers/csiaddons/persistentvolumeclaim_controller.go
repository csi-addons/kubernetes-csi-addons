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
	"fmt"
	"reflect"
	"time"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/apis/csiaddons/v1alpha1"

	"github.com/go-logr/logr"
	"github.com/robfig/cron/v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// PersistentVolumeClaimReconciler reconciles a PersistentVolumeClaim object
type PersistentVolumeClaimReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

var (
	rsCronJobScheduleTimeAnnotation = "reclaimspace." + csiaddonsv1alpha1.GroupVersion.Group + "/schedule"
	rsCronJobNameAnnotation         = "reclaimspace." + csiaddonsv1alpha1.GroupVersion.Group + "/cronjob"
)

const (
	defaultSchedule = "@weekly"
)

//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;patch
//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims/finalizers,verbs=update
//+kubebuilder:rbac:groups=csiaddons.openshift.io,resources=reclaimspacecronjobs,verbs=get;list;watch;create;delete;update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// This is triggered when `reclaimspace.csiaddons.openshift/schedule` annotation
// is found on newly created PVC or if there is a change in value of the annotation.
// It is also triggered by any changes to the child cronjob.
func (r *PersistentVolumeClaimReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch PersistentVolumeClaim instance
	pvc := &corev1.PersistentVolumeClaim{}
	err := r.Client.Get(ctx, req.NamespacedName, pvc)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			logger.Info("PersistentVolumeClaim resource not found")

			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	rsCronJob, err := r.findChildCronJob(ctx, &logger, &req)
	if err != nil {
		return ctrl.Result{}, err
	}
	if rsCronJob != nil {
		logger = logger.WithValues("ReclaimSpaceCronJobName", rsCronJob.Name)
	}

	annotations := pvc.GetAnnotations()
	schedule, scheduleFound := getScheduleFromAnnotation(&logger, annotations)
	if !scheduleFound {
		// if schedule is not found,
		// delete cron job.
		if rsCronJob != nil {
			err = r.deleteChildCronJob(ctx, &logger, rsCronJob)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		// delete name from annotation.
		_, nameFound := annotations[rsCronJobNameAnnotation]
		if nameFound {
			// remove name annotation by patching it to null.
			patch := []byte(fmt.Sprintf(`{"metadata":{"annotations":{%q: null}}}`, rsCronJobNameAnnotation))
			err = r.Client.Patch(ctx, pvc, client.RawPatch(types.StrategicMergePatchType, patch))
			if err != nil {
				logger.Error(err, "Failed to remove annotation")

				return ctrl.Result{}, err
			}
		}
		logger.Info("Annotation not set, exiting reconcile")
		// no schedule annotation set, just dequeue.
		return ctrl.Result{}, nil
	}
	logger = logger.WithValues("Schedule", schedule)

	if rsCronJob != nil {
		newRSCronJob := constructRSCronJob(rsCronJob.Name, req.Namespace, schedule, pvc.Name)
		if reflect.DeepEqual(newRSCronJob.Spec, rsCronJob.Spec) {
			logger.Info("No change in reclaimSpaceCronJob.Spec, exiting reconcile")

			return ctrl.Result{}, nil
		}
		// update rsCronJob spec
		rsCronJob.Spec = newRSCronJob.Spec
		err = r.Client.Update(ctx, rsCronJob)
		if err != nil {
			logger.Error(err, "Failed to update reclaimSpaceCronJob")

			return ctrl.Result{}, err
		}
		logger.Info("Successfully updated reclaimSpaceCronJob")

		return ctrl.Result{}, nil
	}

	rsCronJobName := generateCronJobName(req.Name)
	logger = logger.WithValues("ReclaimSpaceCronJobName", rsCronJobName)
	// add cronjob name in annotations.
	patch := []byte(fmt.Sprintf(`{"metadata":{"annotations":{%q:%q}}}`, rsCronJobNameAnnotation, rsCronJobName))
	err = r.Client.Patch(ctx, pvc, client.RawPatch(types.StrategicMergePatchType, patch))
	if err != nil {
		logger.Error(err, "Failed to update annotation")

		return ctrl.Result{}, err
	}

	rsCronJob = constructRSCronJob(rsCronJobName, req.Namespace, schedule, pvc.Name)
	err = ctrl.SetControllerReference(pvc, rsCronJob, r.Scheme)
	if err != nil {
		logger.Error(err, "Failed to set controllerReference")

		return ctrl.Result{}, err
	}

	err = r.Client.Create(ctx, rsCronJob)
	if err != nil {
		logger.Error(err, "Failed to create reclaimSpaceCronJob")

		return ctrl.Result{}, err
	}
	logger.Info("Successfully created reclaimSpaceCronJob")

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PersistentVolumeClaimReconciler) SetupWithManager(mgr ctrl.Manager) error {
	err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&csiaddonsv1alpha1.ReclaimSpaceCronJob{},
		jobOwnerKey,
		extractOwnerNameFromPVCObj)
	if err != nil {
		return err
	}

	pred := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			if e.Object == nil {
				return false
			}
			// reconcile only if schedule annotation is found.
			_, ok := e.Object.GetAnnotations()[rsCronJobScheduleTimeAnnotation]

			return ok
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.ObjectNew == nil || e.ObjectOld == nil {
				return false
			}
			// reconcile only if schedule annotation between old and new objects have changed.
			oldSchdeule, oldOk := e.ObjectOld.GetAnnotations()[rsCronJobScheduleTimeAnnotation]
			newSchdeule, newOk := e.ObjectNew.GetAnnotations()[rsCronJobScheduleTimeAnnotation]

			return oldOk != newOk || oldSchdeule != newSchdeule
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.PersistentVolumeClaim{}).
		Owns(&csiaddonsv1alpha1.ReclaimSpaceCronJob{}).
		WithEventFilter(pred).
		Complete(r)
}

// findChildCronJob lists child cronjobs, returns the first cronjob and
// deletes the rest if there are more than one cronjob.
func (r *PersistentVolumeClaimReconciler) findChildCronJob(
	ctx context.Context,
	logger *logr.Logger,
	req *reconcile.Request) (*csiaddonsv1alpha1.ReclaimSpaceCronJob, error) {
	var childJobs csiaddonsv1alpha1.ReclaimSpaceCronJobList
	var activeJob *csiaddonsv1alpha1.ReclaimSpaceCronJob
	err := r.List(ctx,
		&childJobs,
		client.InNamespace(req.Namespace),
		client.MatchingFields{jobOwnerKey: req.Name})
	if err != nil {
		logger.Error(err, "Failed to list child reclaimSpaceCronJobs")

		return activeJob, fmt.Errorf("Failed to list child reclaimSpaceCronJob: %v", err)
	}

	for i, job := range childJobs.Items {
		if i == 0 {
			activeJob = &job
			continue
		}
		// there should be only one child cronjob, delete rest if they
		// exist
		err = r.deleteChildCronJob(ctx, logger, &job)
		if err != nil {
			return nil, err
		}
	}

	return activeJob, nil
}

// deleteChildCronJob deletes child cron job, ignoring not found error.
func (r *PersistentVolumeClaimReconciler) deleteChildCronJob(
	ctx context.Context,
	logger *logr.Logger,
	job *csiaddonsv1alpha1.ReclaimSpaceCronJob) error {
	err := r.Delete(ctx, job)
	if client.IgnoreNotFound(err) != nil {
		logger.Error(err, "Failed to delete child reclaimSpaceCronJob",
			"ReclaimSpaceCronJobName", job.Name)

		return fmt.Errorf("Failed to delete child reclaimSpaceCronJob %q: %w",
			job.Name, err)
	}

	return nil
}

// getScheduleFromAnnotation parses schedule and returns it.
// A error is logged and default schedule is returned if it
// is not in cron format.
func getScheduleFromAnnotation(
	logger *logr.Logger,
	annotations map[string]string) (string, bool) {
	schedule, ok := annotations[rsCronJobScheduleTimeAnnotation]
	if !ok {
		return "", false
	}
	_, err := cron.ParseStandard(schedule)
	if err != nil {
		logger.Info(fmt.Sprintf("Parsing given schedule %q failed, using default schedule %q",
			schedule,
			defaultSchedule),
			"error",
			err)

		return defaultSchedule, true
	}

	return schedule, true
}

// constructRSCronJob constructs and returns ReclaimSpaceCronJob.
func constructRSCronJob(name, namespace, schedule, pvcName string) *csiaddonsv1alpha1.ReclaimSpaceCronJob {
	failedJobsHistoryLimit := defaultFailedJobsHistoryLimit
	successfulJobsHistoryLimit := defaultSuccessfulJobsHistoryLimit
	return &csiaddonsv1alpha1.ReclaimSpaceCronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: csiaddonsv1alpha1.ReclaimSpaceCronJobSpec{
			Schedule: schedule,
			JobSpec: csiaddonsv1alpha1.ReclaimSpaceJobTemplateSpec{
				Spec: csiaddonsv1alpha1.ReclaimSpaceJobSpec{
					Target:               csiaddonsv1alpha1.TargetSpec{PersistentVolumeClaim: pvcName},
					BackoffLimit:         defaultBackoffLimit,
					RetryDeadlineSeconds: defaultRetryDeadlineSeconds,
				},
			},
			FailedJobsHistoryLimit:     &failedJobsHistoryLimit,
			SuccessfulJobsHistoryLimit: &successfulJobsHistoryLimit,
		},
	}
}

// extractOwnerNameFromPVCObj extracts owner.Name from the object if it is
// of type ReclaimSpaceCronJob and has a PVC as its owner.
func extractOwnerNameFromPVCObj(rawObj client.Object) []string {
	// extract the owner from job object.
	job, ok := rawObj.(*csiaddonsv1alpha1.ReclaimSpaceCronJob)
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

// generateCronJobName returns unique name by suffixing parent name
// with time hash.
func generateCronJobName(parentName string) string {
	return fmt.Sprintf("%s-%d", parentName, time.Now().Unix())
}
