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
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/csiaddons/v1alpha1"
	"github.com/csi-addons/kubernetes-csi-addons/internal/connection"

	"github.com/go-logr/logr"
	"github.com/robfig/cron/v3"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// PersistentVolumeClaimReconciler reconciles a PersistentVolumeClaim object
type PersistentVolumeClaimReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// ConnectionPool consists of map of Connection objects.
	ConnPool *connection.ConnectionPool
}

var (
	rsCronJobScheduleTimeAnnotation = "reclaimspace." + csiaddonsv1alpha1.GroupVersion.Group + "/schedule"
	rsCronJobNameAnnotation         = "reclaimspace." + csiaddonsv1alpha1.GroupVersion.Group + "/cronjob"
	rsCSIAddonsDriverAnnotation     = "reclaimspace." + csiaddonsv1alpha1.GroupVersion.Group + "/drivers"

	krcJobScheduleTimeAnnotation = "keyrotation." + csiaddonsv1alpha1.GroupVersion.Group + "/schedule"
	krcJobNameAnnotation         = "keyrotation." + csiaddonsv1alpha1.GroupVersion.Group + "/cronjob"
	krCSIAddonsDriverAnnotation  = "keyrotation." + csiaddonsv1alpha1.GroupVersion.Group + "/drivers"

	ErrConnNotFoundRequeueNeeded = errors.New("connection not found, requeue needed")
	ErrScheduleNotFound          = errors.New("schedule not found")
)

const (
	defaultSchedule = "@weekly"
)

//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;patch
//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims/finalizers,verbs=update
//+kubebuilder:rbac:groups=csiaddons.openshift.io,resources=reclaimspacecronjobs,verbs=get;list;watch;create;delete;update
//+kubebuilder:rbac:groups=csiaddons.openshift.io,resources=encryptionkeyrotationcronjobs,verbs=get;list;watch;create;delete;update
//+kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch
//+kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.  This is
// triggered when `reclaimspace.csiaddons.openshift/schedule` annotation is
// found on newly created PVC or its found on the namespace or if there is a
// change in value of the annotation. It is also triggered by any changes to
// the child cronjob.
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

	// Validate PVC in bound state
	if pvc.Status.Phase != corev1.ClaimBound {
		logger.Info("PVC is not in bound state", "PVCPhase", pvc.Status.Phase)
		// requeue the request
		return ctrl.Result{Requeue: true}, nil
	}
	// get the driver name from PV to check if it supports space reclamation.
	pv := &corev1.PersistentVolume{}

	err = r.Client.Get(ctx, types.NamespacedName{Name: pvc.Spec.VolumeName}, pv)
	if err != nil {
		logger.Error(err, "Failed to get PV", "PVName", pvc.Spec.VolumeName)
		return ctrl.Result{}, err
	}

	if pv.Spec.CSI == nil {
		logger.Info("PV is not a CSI volume", "PVName", pv.Name)
		return ctrl.Result{}, nil
	}

	// reconcile key rotation
	keyRotationErr := r.processKeyRotation(ctx, &logger, &req, pvc, pv)
	if keyRotationErr != nil {
		// Log and let the loop continue so that we could process reclaimspace
		logger.Error(keyRotationErr, "reconcile loop failed for keyrotation for pvc", "PVCName", pvc.Name)
	}

	// reconcile reclaim space
	reclaimSpaceResult, reclaimSpaceErr := r.processReclaimSpace(ctx, &logger, &req, pvc, pv)

	// If any one of the above steps failed, we requeue
	// Reclaim space takes precedence over key rotation
	if reclaimSpaceErr != nil {
		logger.Error(reclaimSpaceErr, "failed to reconcile reclaim space for pvc", "PVCName", pvc.Name)

		return reclaimSpaceResult, reclaimSpaceErr
	}

	// Otherwise, return the value of key rotation operation
	// No need to log here as it is already logged at the
	// time of occurrence.
	return ctrl.Result{}, keyRotationErr
}

// checkDriverSupportReclaimsSpace checks if the driver supports space
// reclamation or not. If the driver does not support space reclamation, it
// returns false and if the driver supports space reclamation, it returns true.
// If the driver name is not registered in the connection pool, it returns
// false and requeues the request.
func (r *PersistentVolumeClaimReconciler) checkDriverSupportReclaimsSpace(logger *logr.Logger, annotations map[string]string, driver string) (bool, bool) {
	reclaimSpaceSupportedByDriver := false

	if drivers, ok := annotations[rsCSIAddonsDriverAnnotation]; ok && slices.Contains(strings.Split(drivers, ","), driver) {
		reclaimSpaceSupportedByDriver = true
	}

	ok := r.supportsReclaimSpace(driver)
	if reclaimSpaceSupportedByDriver && !ok {
		logger.Info("Driver supports spacereclamation but driver is not registered in the connection pool, Reqeueing request", "DriverName", driver)
		return true, false
	}

	if !ok {
		logger.Info("Driver does not support spacereclamation, skip Requeue", "DriverName", driver)
		return false, false
	}

	return false, true
}

// checkDriverSupportsEncryptionKeyRotate checks if the driver supports key
// rotation or not. If the driver does not support key rotation, it
// returns false and if the driver supports key rotation, it returns true.
// If the driver name is not registered in the connection pool, it returns
// false and requeues the request.
func (r *PersistentVolumeClaimReconciler) checkDriverSupportsEncryptionKeyRotate(
	logger *logr.Logger,
	annotations map[string]string,
	driver string) (bool, bool) {
	keyRotationSupportedByDriver := false

	if drivers, ok := annotations[krCSIAddonsDriverAnnotation]; ok && slices.Contains(strings.Split(drivers, ","), driver) {
		keyRotationSupportedByDriver = true
	}

	ok := r.supportsEncryptionKeyRotation(driver)
	if keyRotationSupportedByDriver && !ok {
		logger.Info("Driver supports key rotation but driver is not registered in the connection pool, Reqeueing request", "DriverName", driver)
		return true, false
	}

	if !ok {
		logger.Info("Driver does not support encryptionkeyrotation, skip Requeue", "DriverName", driver)
		return false, false
	}

	return false, true
}

// determineScheduleAndRequeue determines the schedule using the following steps
//   - Check if the schedule is present in the PVC annotations. If yes, use that.
//   - Check if the schedule is present in the namespace annotations. If yes,
//     use that.
//   - Check if the schedule is present in the storageclass annotations. If yes,
//     use that.
func (r *PersistentVolumeClaimReconciler) determineScheduleAndRequeue(
	ctx context.Context,
	logger *logr.Logger,
	pvc *corev1.PersistentVolumeClaim,
	driverName string,
	annotationKey string,
) (string, error) {
	annotations := pvc.GetAnnotations()
	schedule, scheduleFound := getScheduleFromAnnotation(annotationKey, logger, annotations)
	if scheduleFound {
		return schedule, nil
	}

	// check for namespace schedule annotation.
	ns := &corev1.Namespace{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: pvc.Namespace}, ns)
	if err != nil {
		logger.Error(err, "Failed to get Namespace", "Namespace", pvc.Namespace)
		return "", err
	}
	schedule, scheduleFound = getScheduleFromAnnotation(annotationKey, logger, ns.Annotations)

	// If the schedule is found, check whether driver supports the
	// space reclamation using annotation on namespace and registered driver
	// capability for decision on requeue.
	if scheduleFound {
		// We cannot have a generic solution for all CSI drivers to get the driver
		// name from PV and check if driver supports reclaimspace/keyrotation or not and
		// requeue the request if the driver is not registered in the connection
		// pool. This can put the controller in a requeue loop. Hence we are
		// reading the driver name from the namespace annotation and checking if
		// the driver is registered in the connection pool and if not we are not
		// requeuing the request.
		// Depending on requeue value, it will return ErrorConnNotFoundRequeueNeeded.
		if annotationKey == krcJobScheduleTimeAnnotation {
			requeue, keyRotationSupported := r.checkDriverSupportsEncryptionKeyRotate(logger, ns.Annotations, driverName)
			if keyRotationSupported {
				return schedule, nil
			}
			if requeue {
				return "", ErrConnNotFoundRequeueNeeded
			}
		} else if annotationKey == rsCronJobScheduleTimeAnnotation {
			requeue, supportReclaimspace := r.checkDriverSupportReclaimsSpace(logger, ns.Annotations, driverName)
			if supportReclaimspace {
				// if driver supports space reclamation,
				// return schedule from ns annotation.
				return schedule, nil
			}
			if requeue {
				// The request needs to be requeued for checking
				// driver support again.
				return "", ErrConnNotFoundRequeueNeeded
			}
		}
	}

	// For static provisioned PVs, StorageClassName is empty.
	if len(*pvc.Spec.StorageClassName) == 0 {
		logger.Info("StorageClassName is empty")
		return "", ErrScheduleNotFound
	}

	// check for storageclass schedule annotation.
	sc := &storagev1.StorageClass{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: *pvc.Spec.StorageClassName}, sc)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Error(err, "StorageClass not found", "StorageClass", *pvc.Spec.StorageClassName)
			return "", ErrScheduleNotFound
		}

		logger.Error(err, "Failed to get StorageClass", "StorageClass", *pvc.Spec.StorageClassName)
		return "", err
	}
	schedule, scheduleFound = getScheduleFromAnnotation(annotationKey, logger, sc.Annotations)
	if scheduleFound {
		return schedule, nil
	}

	return "", ErrScheduleNotFound
}

// storageClassEventHandler returns an EventHandler that responds to changes
// in StorageClass objects and generates reconciliation requests for all
// PVCs associated with the changed StorageClass.
// PVCs with rsCronJobScheduleTimeAnnotation are not enqueued.
func (r *PersistentVolumeClaimReconciler) storageClassEventHandler() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(
		func(ctx context.Context, obj client.Object) []reconcile.Request {
			ok := false
			obj, ok = obj.(*storagev1.StorageClass)
			if !ok {
				return nil
			}

			// get all PVCs with the same storageclass.
			pvcList := &corev1.PersistentVolumeClaimList{}
			err := r.List(ctx, pvcList, client.MatchingFields{"spec.storageClassName": obj.GetName()})
			if err != nil {
				log.FromContext(ctx).Error(err, "Failed to list PVCs")

				return nil
			}

			var requests []reconcile.Request
			for _, pvc := range pvcList.Items {
				if _, ok := pvc.GetAnnotations()[rsCronJobScheduleTimeAnnotation]; ok {
					continue
				}
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      pvc.Name,
						Namespace: pvc.Namespace,
					},
				})
			}

			return requests
		},
	)
}

// SetupWithManager sets up the controller with the Manager.
func (r *PersistentVolumeClaimReconciler) SetupWithManager(mgr ctrl.Manager, ctrlOptions controller.Options) error {
	err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&csiaddonsv1alpha1.ReclaimSpaceCronJob{},
		jobOwnerKey,
		extractOwnerNameFromPVCObj)
	if err != nil {
		return err
	}

	err = mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&corev1.PersistentVolumeClaim{},
		"spec.storageClassName",
		func(rawObj client.Object) []string {
			pvc, ok := rawObj.(*corev1.PersistentVolumeClaim)
			if !ok {
				return nil
			}
			return []string{*pvc.Spec.StorageClassName}
		})
	if err != nil {
		return err
	}

	err = mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&csiaddonsv1alpha1.EncryptionKeyRotationCronJob{},
		jobOwnerKey,
		func(rawObj client.Object) []string {
			job, ok := rawObj.(*csiaddonsv1alpha1.EncryptionKeyRotationCronJob)
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
		})
	if err != nil {
		return err
	}

	pred := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.ObjectNew == nil || e.ObjectOld == nil {
				return false
			}
			// reconcile only if schedule annotation between old and new objects have changed.
			oldSchdeule, oldOk := e.ObjectOld.GetAnnotations()[rsCronJobScheduleTimeAnnotation]
			newSchdeule, newOk := e.ObjectNew.GetAnnotations()[rsCronJobScheduleTimeAnnotation]

			krcOldSchdeule, krcOldOk := e.ObjectOld.GetAnnotations()[krcJobScheduleTimeAnnotation]
			krcNewSchdeule, krcNewOk := e.ObjectNew.GetAnnotations()[krcJobScheduleTimeAnnotation]

			return (oldOk != newOk || oldSchdeule != newSchdeule) || (krcOldOk != krcNewOk || krcOldSchdeule != krcNewSchdeule)
		},
	}

	scPred := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.ObjectNew == nil || e.ObjectOld == nil {
				return false
			}
			// reconcile only if reclaimspace annotation between old and new objects have changed.
			oldSchdeule, oldOk := e.ObjectOld.GetAnnotations()[rsCronJobScheduleTimeAnnotation]
			newSchdeule, newOk := e.ObjectNew.GetAnnotations()[rsCronJobScheduleTimeAnnotation]

			krcOldSchdeule, krcOldOk := e.ObjectOld.GetAnnotations()[krcJobScheduleTimeAnnotation]
			krcNewSchdeule, krcNewOk := e.ObjectNew.GetAnnotations()[krcJobScheduleTimeAnnotation]

			return (oldOk != newOk || oldSchdeule != newSchdeule) || (krcOldOk != krcNewOk || krcOldSchdeule != krcNewSchdeule)
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.PersistentVolumeClaim{}).
		Watches(
			&storagev1.StorageClass{},
			r.storageClassEventHandler(),
			builder.WithPredicates(scPred),
		).
		Owns(&csiaddonsv1alpha1.ReclaimSpaceCronJob{}).
		Owns(&csiaddonsv1alpha1.EncryptionKeyRotationCronJob{}).
		WithEventFilter(pred).
		WithOptions(ctrlOptions).
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
	key string,
	logger *logr.Logger,
	annotations map[string]string) (string, bool) {
	schedule, ok := annotations[key]
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

// supportsReclaimSpace checks if the CSI driver supports ReclaimSpace.
func (r PersistentVolumeClaimReconciler) supportsReclaimSpace(driverName string) bool {
	conns := r.ConnPool.GetByNodeID(driverName, "")
	for _, v := range conns {
		for _, cap := range v.Capabilities {
			if cap.GetReclaimSpace() != nil {
				return true
			}
		}
	}

	return false
}

// supportsEncryptionKeyRotation checks if the CSI driver supports EncryptionKeyRotation.
func (r PersistentVolumeClaimReconciler) supportsEncryptionKeyRotation(driverName string) bool {
	conns := r.ConnPool.GetByNodeID(driverName, "")
	for _, v := range conns {
		for _, cap := range v.Capabilities {
			if cap.GetEncryptionKeyRotation() != nil {
				return true
			}
		}
	}

	return false
}

// processReclaimSpace reconciles ReclaimSpace based on annotations
func (r *PersistentVolumeClaimReconciler) processReclaimSpace(
	ctx context.Context,
	logger *logr.Logger,
	req *reconcile.Request,
	pvc *corev1.PersistentVolumeClaim,
	pv *corev1.PersistentVolume) (ctrl.Result, error) {
	rsCronJob, err := r.findChildCronJob(ctx, logger, req)
	if err != nil {
		return ctrl.Result{}, err
	}
	if rsCronJob != nil {
		*logger = logger.WithValues("ReclaimSpaceCronJobName", rsCronJob.Name)
	}

	schedule, err := r.determineScheduleAndRequeue(ctx, logger, pvc, pv.Spec.CSI.Driver, rsCronJobScheduleTimeAnnotation)
	if errors.Is(err, ErrConnNotFoundRequeueNeeded) {
		return ctrl.Result{Requeue: true}, nil
	}
	if errors.Is(err, ErrScheduleNotFound) {
		// if schedule is not found,
		// delete cron job.
		if rsCronJob != nil {
			err = r.deleteChildCronJob(ctx, logger, rsCronJob)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		// delete name from annotation.
		_, nameFound := pvc.Annotations[rsCronJobNameAnnotation]
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
	if err != nil {
		return ctrl.Result{}, err
	}

	*logger = logger.WithValues("Schedule", schedule)

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
	*logger = logger.WithValues("ReclaimSpaceCronJobName", rsCronJobName)
	// add cronjob name and schedule in annotations.
	// adding annotation is required for the case when pvc does not have
	// have schedule annotation but namespace has.
	patch := []byte(fmt.Sprintf(`{"metadata":{"annotations":{%q:%q,%q:%q}}}`,
		rsCronJobNameAnnotation,
		rsCronJobName,
		rsCronJobScheduleTimeAnnotation,
		schedule,
	))
	logger.Info("Adding annotation", "Annotation", string(patch))
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

// findChildEncryptionKeyRotationCronJob returns the active job from a list of
// EncryptionKeyRotationCronJobs in the request's namespace.
func (r *PersistentVolumeClaimReconciler) findChildEncryptionKeyRotationCronJob(
	ctx context.Context,
	logger *logr.Logger,
	req *reconcile.Request) (*csiaddonsv1alpha1.EncryptionKeyRotationCronJob, error) {
	var childJobs csiaddonsv1alpha1.EncryptionKeyRotationCronJobList
	var activeJob *csiaddonsv1alpha1.EncryptionKeyRotationCronJob

	err := r.List(ctx, &childJobs, client.InNamespace(req.Namespace), client.MatchingFields{jobOwnerKey: req.Name})
	if err != nil {
		logger.Error(err, "failed to list child encryptionkeyrotationcronjobs")
		return activeJob, fmt.Errorf("failed to list encryptionkeyrotationcronjobs: %v", err)
	}

	for i, job := range childJobs.Items {
		// max allowed job is 1
		if i == 0 {
			activeJob = &job
			continue
		}

		// delete the rest if found
		if err = r.Delete(ctx, &job); err != nil {
			if client.IgnoreNotFound(err) != nil {
				logger.Error(err, "failed to delete extraneous child encryptionkeyrotationcronjob", "EncryptionKeyrotationCronJobName", &job.Name)
				return nil, fmt.Errorf("failed to delete extraneous child encryptionkeyrotationcronjob: %w", err)
			}
		}
	}

	return activeJob, nil
}

// constructKRCronJob constructs an EncryptionKeyRotationCronJob object
func constructKRCronJob(name, namespace, schedule, pvcName string) *csiaddonsv1alpha1.EncryptionKeyRotationCronJob {
	failedJobHistoryLimit := defaultFailedJobsHistoryLimit
	successfulJobsHistoryLimit := defaultSuccessfulJobsHistoryLimit

	return &csiaddonsv1alpha1.EncryptionKeyRotationCronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: csiaddonsv1alpha1.EncryptionKeyRotationCronJobSpec{
			Schedule: schedule,
			JobSpec: csiaddonsv1alpha1.EncryptionKeyRotationJobTemplateSpec{
				Spec: csiaddonsv1alpha1.EncryptionKeyRotationJobSpec{
					Target: csiaddonsv1alpha1.TargetSpec{
						PersistentVolumeClaim: pvcName,
					},
					BackoffLimit:         defaultBackoffLimit,
					RetryDeadlineSeconds: defaultRetryDeadlineSeconds,
				},
			},
			FailedJobsHistoryLimit:     &failedJobHistoryLimit,
			SuccessfulJobsHistoryLimit: &successfulJobsHistoryLimit,
		},
	}
}

// processKeyRotation reconciles EncryptionKeyRotation based on annotations
func (r *PersistentVolumeClaimReconciler) processKeyRotation(
	ctx context.Context,
	logger *logr.Logger,
	req *reconcile.Request,
	pvc *corev1.PersistentVolumeClaim,
	pv *corev1.PersistentVolume) error {
	krcJob, err := r.findChildEncryptionKeyRotationCronJob(ctx, logger, req)
	if err != nil {
		return err
	}
	if krcJob != nil {
		*logger = logger.WithValues("EncryptionKeyrotationCronJobName", krcJob.Name)
	}

	// Determine schedule
	sched, err := r.determineScheduleAndRequeue(ctx, logger, pvc, pv.Spec.CSI.Driver, krcJobScheduleTimeAnnotation)
	if errors.Is(err, ErrScheduleNotFound) {
		// No schedule, delete the job
		if krcJob != nil {
			err = r.Delete(ctx, krcJob)
			if client.IgnoreNotFound(err) != nil {
				logger.Error(err, "failed to delete child encryptionkeyrotationcronjob")

				return fmt.Errorf("failed to delete child encryptionkeyrotationcronjob %q:%w", krcJob.Name, err)
			}
		}

		logger.Info("annotation not found for key rotation, exiting reconcile")
		return nil
	}
	if err != nil {
		return err
	}

	*logger = logger.WithValues("KeyRotationSchedule", sched)

	if krcJob != nil {
		newKrcJob := constructKRCronJob(krcJob.Name, req.Namespace, sched, pvc.Name)
		if reflect.DeepEqual(newKrcJob.Spec, krcJob.Spec) {
			logger.Info("no change in encryptionkeyrotationjob spec, exiting reconcile")
			return nil
		}

		// Update the spec
		krcJob.Spec = newKrcJob.Spec
		err = r.Client.Update(ctx, krcJob)
		if err != nil {
			logger.Error(err, "failed to update encryptionkeyrotationcronjob")
			return err //ctr.Result
		}

		logger.Info("successfully updated encryptionkeyrotationcronjob")
		return nil
	}

	// Add the annotation to the pvc, this will help us optimize reconciles
	krcJobName := generateCronJobName(req.Name)
	patch := []byte(fmt.Sprintf(`{"metadata":{"annotations":{%q:%q,%q:%q}}}`,
		krcJobNameAnnotation,
		krcJobName,
		krcJobScheduleTimeAnnotation,
		sched,
	))
	logger.Info("Adding keyrotation annotation to the pvc", "annotation", string(patch))
	err = r.Client.Patch(ctx, pvc, client.RawPatch(types.StrategicMergePatchType, patch))
	if err != nil {
		logger.Error(err, "Failed to set annotation for keyrotation on the pvc")

		return err
	}

	// Construct a new cron job
	krcJob = constructKRCronJob(krcJobName, req.Namespace, sched, pvc.Name)

	// Set owner ref
	err = ctrl.SetControllerReference(pvc, krcJob, r.Scheme)
	if err != nil {
		logger.Error(err, "failed to set controller ref for encryptionkeyrotationcronjob")
		return err
	}

	// Update the cluster with the new resource
	err = r.Client.Create(ctx, krcJob)
	if err != nil {
		logger.Error(err, "failed to create new encryptionkeyrotationcronjob")
		return err
	}

	logger.Info("successfully created new encryptionkeyrotationcronjob")
	return nil
}
