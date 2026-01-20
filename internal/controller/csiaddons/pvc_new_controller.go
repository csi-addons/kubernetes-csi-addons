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

package controller

import (
	"context"
	"fmt"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/csiaddons/v1alpha1"
	"github.com/csi-addons/kubernetes-csi-addons/internal/connection"
	"github.com/csi-addons/kubernetes-csi-addons/internal/controller/utils"

	"github.com/go-logr/logr"
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
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type PVCReconiler struct {
	client.Client
	Scheme *runtime.Scheme

	// ConnectionPool consists of map of Connection objects.
	ConnPool *connection.ConnectionPool
}

//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch
//+kubebuilder:rbac:groups=csiaddons.openshift.io,resources=reclaimspacecronjobs,verbs=get;list;watch;create;delete;update
//+kubebuilder:rbac:groups=csiaddons.openshift.io,resources=encryptionkeyrotationcronjobs,verbs=get;list;watch;create;delete;update
//+kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch

func (r *PVCReconiler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the PVC
	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.Get(ctx, req.NamespacedName, pvc); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Ignore if being deleted
	if !pvc.DeletionTimestamp.IsZero() {
		logger.Info("PVC is being deleted, exiting reconciliation", "PVCInfo", req.NamespacedName)

		return ctrl.Result{}, nil
	}

	logger.Info("Reconciling PVC", "PVCInfo", req.NamespacedName)

	// The PVC must be in a bound state, if not, we requeue
	if pvc.Status.Phase != corev1.ClaimBound {
		logger.Info("PVC is not yet bound, requeue the request", "PVCInfo", req.NamespacedName)

		return ctrl.Result{Requeue: true}, nil
	}

	// Fetch the PV and check if it is CSI provisioned, if not, do nothing
	pv := &corev1.PersistentVolume{}
	if err := r.Get(ctx, types.NamespacedName{Name: pvc.Spec.VolumeName}, pv); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Must be CSI provisioned to continue
	if pv.Spec.CSI == nil {
		logger.Info("PVC is not CSI provisioned, exiting reconciliation", "PVCInfo", req.NamespacedName)

		return ctrl.Result{}, nil
	}

	// Should not be a static PVC
	if !hasValidStorageClassName(pvc) {
		logger.Info("The PVC is statically provisioned, exiting reconciliation", "PVCInfo", req.NamespacedName)

		return ctrl.Result{}, nil
	}

	// Now we fetch the StorageClass
	sc := &storagev1.StorageClass{}
	if err := r.Get(ctx, types.NamespacedName{Name: *pvc.Spec.StorageClassName}, sc); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Reconcile for dependent features
	// Reconcile - Key rotation
	keyRotationName := fmt.Sprintf("%s-keyrotation", pvc.Name)
	keyRotationSched := sc.Annotations[utils.KrcJobScheduleTimeAnnotation]
	keyRotationEnabled := sc.Annotations[utils.KrEnableAnnotation]
	keyRotationChild := &csiaddonsv1alpha1.EncryptionKeyRotationCronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      keyRotationName,
			Namespace: pvc.Namespace,
		},
	}

	if err := r.reconcileFeature(ctx, logger, pvc, keyRotationChild, keyRotationSched, keyRotationEnabled); err != nil {
		return ctrl.Result{}, err
	}

	// Reconcile - Reclaim space
	reclaimSpaceName := fmt.Sprintf("%s-reclaimspace", pvc.Name)
	reclaimSpaceSched := sc.Annotations[utils.RsCronJobScheduleTimeAnnotation]
	reclaimSpaceChild := &csiaddonsv1alpha1.ReclaimSpaceCronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      reclaimSpaceName,
			Namespace: pvc.Namespace,
		},
	}

	if err := r.reconcileFeature(ctx, logger, pvc, reclaimSpaceChild, reclaimSpaceSched, "true"); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *PVCReconiler) reconcileFeature(
	ctx context.Context,
	logger logr.Logger,
	pvc *corev1.PersistentVolumeClaim,
	childObj client.Object,
	schedule string,
	enabledVal string,
) error {
	defer logger.Info("Completed reconcile for feature")

	// Determine if the object should exists in the cluster
	// Note: we do not return early if feature is disabled as we might need to garbage collect
	shouldExist := (schedule != "") && (enabledVal != "false")
	logger.Info("Reconciling feature for child object with values", "childObj", childObj, "schedule", schedule, "isEnabled", enabledVal, "shouldExist", shouldExist)

	// Now check if it actually exists in the cluster
	exists := true
	existingObj := childObj.DeepCopyObject().(client.Object)
	if err := r.Get(ctx, types.NamespacedName{Name: childObj.GetName(), Namespace: childObj.GetNamespace()}, existingObj); err != nil {
		if apierrors.IsNotFound(err) {
			exists = false
		} else {
			return err
		}
	}
	logger.Info("determined the state to reconcile with", "shouldExist", shouldExist, "exists", exists)

	// We allow the user to mark a feature as unmanged by editing an existing resource
	// and setting the CSIAddonsStateAnnotation to: unmanaged
	// Return and do nothing in such cases
	if exists && !utils.IsManagedByController(existingObj) {
		logger.Info("The existing object is not managed by the controller, doing nothing for it.", "objName", existingObj.GetName(), "objNamespace", existingObj.GetNamespace())

		return nil
	}

	// Object should not be present in the cluster, garbage collect or return
	if !shouldExist {
		if exists {
			logger.Info("Deleting the undersired object from the cluster", "childObj", existingObj)
			return client.IgnoreNotFound(r.Delete(ctx, existingObj))
		}

		return nil
	}

	// We reached here, means we need to either create or update the object in cluster
	// -- Create
	if !exists {
		// Set the required fields on the bare bones object
		utils.SetSpec(childObj, schedule, pvc.Name)

		// Set controller reference
		if err := ctrl.SetControllerReference(pvc, childObj, r.Scheme); err != nil {
			return err
		}

		logger.Info("creating a new object in the cluster", "newObj", childObj)

		// We use deterministic names for the resources we create
		// This saves us additional listing and managing of stale resources
		return client.IgnoreAlreadyExists(r.Create(ctx, childObj))
	}

	// -- Update
	currentSched := utils.GetSchedule(existingObj)
	if currentSched != schedule {
		// Update and set spec
		utils.SetSpec(existingObj, schedule, pvc.Name)

		// Update the object in the cluster
		logger.Info("calling update for new schedule on object", "newObj", childObj, "currentSchedule", currentSched, "desiredSchedule", schedule)
		return r.Update(ctx, existingObj)
	}

	return nil
}

func (r *PVCReconiler) SetupWithManager(mgr ctrl.Manager, ctrlOptions controller.Options) error {
	// Setup the required indexers to optimize lookups
	if err := utils.SetupPVCControllerIndexers(mgr); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		// Primary source
		For(&corev1.PersistentVolumeClaim{}).

		// Secondary sources
		Owns(&csiaddonsv1alpha1.ReclaimSpaceCronJob{}).
		Owns(&csiaddonsv1alpha1.EncryptionKeyRotationCronJob{}).

		// Watch the storageclass and fan out according to predicates
		Watches(
			&storagev1.StorageClass{},
			handler.EnqueueRequestsFromMapFunc(utils.ScMapFunc(r.Client)),
			builder.WithPredicates(utils.StorageClassPredicate()),
		).

		// Watch the PVs silently, this is to avoid client/server rate limits
		// And to make Get/List O(1)
		Watches(
			&corev1.PersistentVolume{},
			&handler.EnqueueRequestForObject{}, // This handler is never called due to `silentPredicate`
			builder.WithPredicates(utils.SilentPredicate()),
		).
		WithOptions(ctrlOptions).
		Complete(r)
}
