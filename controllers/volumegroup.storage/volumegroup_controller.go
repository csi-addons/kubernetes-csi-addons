/*
Copyright 2023 The Kubernetes-CSI-Addons Authors.

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

package vg_controller

import (
	"context"
	"fmt"
	"time"

	volumegroupv1 "github.com/csi-addons/kubernetes-csi-addons/apis/volumegroup.storage/v1"
	commonUtils "github.com/csi-addons/kubernetes-csi-addons/controllers/volumegroup.storage/common/utils"
	"github.com/csi-addons/kubernetes-csi-addons/controllers/volumegroup.storage/pkg/config"
	"github.com/csi-addons/kubernetes-csi-addons/controllers/volumegroup.storage/pkg/messages"
	"github.com/csi-addons/kubernetes-csi-addons/controllers/volumegroup.storage/utils"
	grpcClient "github.com/csi-addons/kubernetes-csi-addons/internal/client"
	conn "github.com/csi-addons/kubernetes-csi-addons/internal/connection"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	VolumeGroup        = "VolumeGroup"
	VolumeGroupClass   = "VolumeGroupClass"
	VolumeGroupContent = "VolumeGroupContent"
)

type VolumeGroupReconciler struct {
	client.Client
	Log          logr.Logger
	Scheme       *runtime.Scheme
	DriverConfig *config.DriverConfig
	Connpool     *conn.ConnectionPool
	Timeout      time.Duration
	VGClient     grpcClient.VolumeGroup
}

//+kubebuilder:rbac:groups=volumegroup.storage.openshift.io,resources=volumegroups,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=volumegroup.storage.openshift.io,resources=volumegroups/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=volumegroup.storage.openshift.io,resources=volumegroups/finalizers,verbs=update
//+kubebuilder:rbac:groups=volumegroup.storage.openshift.io,resources=volumegroupclasses,verbs=get;list;watch
//+kubebuilder:rbac:groups=volumegroup.storage.openshift.io,resources=volumegroupcontents,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=volumegroup.storage.openshift.io,resources=volumegroupcontents/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=volumegroup.storage.openshift.io,resources=volumegroupcontents/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims/status,verbs=get;update;patch
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=events,verbs=create;get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch

func (r *VolumeGroupReconciler) Reconcile(_ context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("Request.Name", req.Name, "Request.Namespace", req.Namespace)
	logger.Info(messages.ReconcileVG)

	instance := &volumegroupv1.VolumeGroup{}
	if err := r.Client.Get(context.TODO(), req.NamespacedName, instance); err != nil {
		if errors.IsNotFound(err) {

			logger.Info("VolumeGroup resource not found")

			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, utils.HandleErrorMessage(logger, r.Client, instance, err, vgReconcile)
	}

	vgClass, err := utils.GetVGClass(r.Client, logger, utils.GetStringField(instance.Spec, "VolumeGroupClassName"))
	if err != nil {
		return ctrl.Result{}, utils.HandleErrorMessage(logger, r.Client, instance, err, vgReconcile)
	}

	if err = utils.ValidatePrefixedParameters(vgClass.Parameters); err != nil {
		logger.Error(err, "failed to validate parameters of volumegroupClass", "VGClassName", vgClass.Name)
		if uErr := utils.UpdateVGStatusError(r.Client, instance, logger, err.Error()); uErr != nil {
			return ctrl.Result{}, uErr
		}
		return ctrl.Result{}, err
	}
	r.DriverConfig, err = config.GetDriverConfig(vgClass.Driver, r.Connpool)
	if err != nil {
		return ctrl.Result{}, utils.HandleErrorMessage(logger, r.Client, instance, err, vgReconcile)
	}

	r.VGClient, err = utils.GetVolumeGroupClient(r.DriverConfig.DriverName, r.Connpool, r.Timeout)
	if err != nil {
		return ctrl.Result{}, utils.HandleErrorMessage(logger, r.Client, instance, err, vgReconcile)
	}

	if instance.GetDeletionTimestamp().IsZero() {
		if err = utils.AddFinalizerToVG(r.Client, logger, instance); err != nil {
			return ctrl.Result{}, utils.HandleErrorMessage(logger, r.Client, instance, err, createVG)
		}

	} else {
		if commonUtils.Contains(instance.GetFinalizers(), utils.VGFinalizer) && !utils.IsContainOtherFinalizers(instance, logger) {
			if err = r.removeInstance(logger, instance); err != nil {
				return ctrl.Result{}, utils.HandleErrorMessage(logger, r.Client, instance, err, deleteVG)
			}
			logger.Info("volumeGroup object is terminated, skipping reconciliation")
		}
		return ctrl.Result{}, nil
	}

	groupCreationTime := utils.GetCurrentTime()

	err, isStaticProvisioned := r.handleStaticProvisionedVG(instance, logger, groupCreationTime, vgClass)
	if isStaticProvisioned {
		return ctrl.Result{}, err
	}

	vgName, err := utils.MakeVGName(utils.VGNamePrefix, string(instance.UID))
	if err != nil {
		return ctrl.Result{}, utils.HandleErrorMessage(logger, r.Client, instance, err, createVG)
	}

	secretName, secretNamespace := utils.GetSecretCred(vgClass)
	vgc := utils.GenerateVGC(vgName, instance, vgClass, secretName, secretNamespace)
	logger.Info(fmt.Sprintf("GenerateVolumeGroupContent vgc: %v", vgc))
	if err = utils.CreateVGC(r.Client, logger, vgc); err != nil {
		return ctrl.Result{}, utils.HandleErrorMessage(logger, r.Client, instance, err, createVGC)
	}
	if isVGCReady, err := r.isVGCReady(logger, vgc); err != nil {
		return ctrl.Result{}, utils.HandleErrorMessage(logger, r.Client, instance, err, createVGC)
	} else if !isVGCReady {
		return ctrl.Result{Requeue: true}, nil
	}

	err = r.updateItems(instance, logger, groupCreationTime, vgc.Name)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.updatePVCs(logger, instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.createSuccessVGEvent(logger, instance)
	if err != nil {
		return ctrl.Result{}, utils.HandleErrorMessage(logger, r.Client, instance, err, vgReconcile)
	}
	return ctrl.Result{}, nil
}

func (r *VolumeGroupReconciler) updatePVCs(logger logr.Logger, vg *volumegroupv1.VolumeGroup) error {
	if !r.DriverConfig.ModifyVolumeGroup {
		return nil
	}
	matchingPvcs, err := r.getMatchingPVCs(logger, *vg)
	if err != nil {
		return utils.HandleErrorMessage(logger, r.Client, vg, err, vgReconcile)
	}
	if utils.IsPVCListEqual(matchingPvcs, vg.Status.PVCList) {
		return nil
	}
	err = utils.ModifyVolumesInVG(logger, r.Client, r.VGClient, matchingPvcs, *vg)
	if err != nil {
		return utils.HandleErrorMessage(logger, r.Client, vg, err, vgReconcile)
	}
	err = utils.UpdatePvcAndPvList(logger, vg, r.Client, r.DriverConfig.DriverName, matchingPvcs)
	if err != nil {
		return err
	}
	return nil
}

func (r *VolumeGroupReconciler) handleStaticProvisionedVG(vg *volumegroupv1.VolumeGroup, logger logr.Logger, groupCreationTime *metav1.Time, vgClass *volumegroupv1.VolumeGroupClass) (error, bool) {
	if vg.Spec.Source.VolumeGroupContentName != nil {
		err := r.updateItems(vg, logger, groupCreationTime, *vg.Spec.Source.VolumeGroupContentName)
		if err != nil {
			return err, true
		}
		err = utils.UpdateStaticVGCFromVG(r.Client, vg, vgClass, logger)
		if err != nil {
			return err, true
		}
		err = r.updatePVCs(logger, vg)
		if err != nil {
			return err, true
		}
		return nil, true
	}
	return nil, false
}

func (r *VolumeGroupReconciler) updateItems(instance *volumegroupv1.VolumeGroup, logger logr.Logger, groupCreationTime *metav1.Time, vgcName string) error {
	if err := utils.UpdateVGSourceContent(r.Client, instance, vgcName, logger); err != nil {
		return utils.HandleErrorMessage(logger, r.Client, instance, err, updateVGC)
	}
	if err := utils.UpdateVGStatus(r.Client, instance, vgcName, groupCreationTime, true, logger); err != nil {
		return utils.HandleErrorMessage(logger, r.Client, instance, err, updateStatusVG)
	}
	return nil
}

func (r *VolumeGroupReconciler) removeInstance(logger logr.Logger, instance *volumegroupv1.VolumeGroup) error {
	vgc, err := utils.GetVGC(r.Client, logger, utils.GetStringField(instance.Spec.Source, "VolumeGroupContentName"), instance.Namespace)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}

	} else {
		err = r.removeVGCObject(logger, vgc)
		if err != nil {
			return err
		}
	}
	if err = utils.RemoveFinalizerFromVG(r.Client, logger, instance); err != nil {
		return err
	}
	return nil
}

func (r *VolumeGroupReconciler) removeVGCObject(logger logr.Logger, vgc *volumegroupv1.VolumeGroupContent) error {
	if *vgc.Spec.VolumeGroupDeletionPolicy == volumegroupv1.VolumeGroupContentDelete {
		if err := r.Client.Delete(context.TODO(), vgc); err != nil {
			logger.Error(err, fmt.Sprintf("Failed to delete %s volume group content", vgc.Name))
			return err
		}
	}
	return nil
}

func (r *VolumeGroupReconciler) isPVCShouldBeInVg(logger logr.Logger, vg volumegroupv1.VolumeGroup,
	pvc *corev1.PersistentVolumeClaim) (bool, error) {

	isPVCMatchesVG, err := utils.IsPVCMatchesVG(logger, pvc, vg)
	if err != nil {
		return false, err
	}
	if !isPVCMatchesVG {
		return false, nil
	}

	if err := r.isPVCCanBeAddedToVG(logger, pvc); err != nil {
		return false, err
	}
	return true, nil
}

func (r VolumeGroupReconciler) isPVCCanBeAddedToVG(logger logr.Logger, pvc *corev1.PersistentVolumeClaim) error {
	if r.DriverConfig.MultipleVGsToPVC {
		return nil
	}

	vgList, err := utils.GetVGList(logger, r.Client, r.DriverConfig.DriverName)
	if err != nil {
		return err
	}
	err = utils.IsPVCCanBeAddedToVG(logger, pvc, vgList.Items)
	return err
}

func (r VolumeGroupReconciler) createSuccessVGEvent(logger logr.Logger, vg *volumegroupv1.VolumeGroup) error {
	message := fmt.Sprintf(messages.VGCreated, vg.Namespace, vg.Name)
	err := utils.HandleSuccessMessage(logger, r.Client, vg, message, vgReconcile)
	if err != nil {
		return nil
	}
	return nil
}

func (r *VolumeGroupReconciler) SetupWithManager(mgr ctrl.Manager, ctrlOptions controller.Options) error {
	logger := r.Log.WithName("SetupWithManager")
	err := r.waitForCrds(logger)
	if err != nil {
		r.Log.Error(err, "failed to wait for crds")

		return err
	}
	generationPred := predicate.GenerationChangedPredicate{}
	pred := predicate.Or(generationPred, utils.FinalizerPredicate)

	return ctrl.NewControllerManagedBy(mgr).
		For(&volumegroupv1.VolumeGroup{}, builder.WithPredicates(pred)).
		Watches(&corev1.PersistentVolumeClaim{}, utils.CreateRequests(r.Client), builder.WithPredicates(utils.PvcPredicate)).
		WithOptions(ctrlOptions).
		Complete(r)
}

func (r *VolumeGroupReconciler) waitForCrds(logger logr.Logger) error {
	err := r.waitForVGResource(logger, VolumeGroup)
	if err != nil {
		logger.Error(err, "failed to wait for VolumeGroup CRD")

		return err
	}

	err = r.waitForVGResource(logger, VolumeGroupClass)
	if err != nil {
		logger.Error(err, "failed to wait for VolumeGroupClass CRD")

		return err
	}

	err = r.waitForVGResource(logger, VolumeGroupContent)
	if err != nil {
		logger.Error(err, "failed to wait for VolumeGroupContent CRD")

		return err
	}

	return nil
}

func (r *VolumeGroupReconciler) waitForVGResource(logger logr.Logger, resourceName string) error {
	unstructuredResource := &unstructured.UnstructuredList{}
	unstructuredResource.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   volumegroupv1.GroupVersion.Group,
		Kind:    resourceName,
		Version: volumegroupv1.GroupVersion.Version,
	})
	for {
		err := r.Client.List(context.TODO(), unstructuredResource)
		if err == nil {
			return nil
		}
		// return errors other than NoMatch
		if !meta.IsNoMatchError(err) {
			logger.Error(err, fmt.Sprintf("got an unexpected error while waiting for %s resource", resourceName))

			return err
		}
		logger.Info("resource does not exist", "Resource", resourceName)
		time.Sleep(5 * time.Second)
	}
}

func (r *VolumeGroupReconciler) getMatchingPVCs(logger logr.Logger, vg volumegroupv1.VolumeGroup) ([]corev1.PersistentVolumeClaim, error) {
	var matchingPvcs []corev1.PersistentVolumeClaim
	pvcList, err := utils.GetPVCList(logger, r.Client, r.DriverConfig.DriverName)
	if err != nil {
		return nil, err
	}
	for _, pvc := range pvcList.Items {
		isPVCShouldBeInVg, err := r.isPVCShouldBeInVg(logger, vg, &pvc)
		if err != nil {
			return nil, err
		}
		isPVCShouldBeHandled, err := utils.IsPVCNeedToBeHandled(logger, &pvc, r.Client, r.DriverConfig.DriverName)
		if err != nil {
			return nil, err
		}
		if isPVCShouldBeInVg && !utils.IsPVCInPVCList(&pvc, matchingPvcs) && isPVCShouldBeHandled {
			matchingPvcs = append(matchingPvcs, pvc)
		}
	}
	return matchingPvcs, err
}

func (r *VolumeGroupReconciler) isVGCReady(logger logr.Logger, vgc *volumegroupv1.VolumeGroupContent) (bool, error) {
	vgcFromCluster, err := utils.GetVGC(r.Client, logger, vgc.Name, vgc.Namespace)
	if err != nil {
		if !errors.IsNotFound(err) {
			return false, err
		}
		return false, nil
	}
	return utils.GetBoolField(vgcFromCluster.Status, "Ready"), nil
}
