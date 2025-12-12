/*
Copyright 2024 The Kubernetes-CSI-Addons Authors.

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

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	replicationv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/replication.storage/v1alpha1"
	"github.com/csi-addons/kubernetes-csi-addons/internal/controller/replication.storage/replication"
)

const (
	volumeGroupReplicationClass   = "VolumeGroupReplicationClass"
	volumeGroupReplication        = "VolumeGroupReplication"
	volumeGroupReplicationContent = "VolumeGroupReplicationContent"
	volumeGroupReplicationRef     = "replication.storage.openshift.io/volumegroupref"
	pvcSelector                   = "pvcSelector"
	// This annotation is added to a VGRC that has been dynamically provisioned by
	// csi-addons. Its value is name of driver that created the volume group.
	annDynamicallyProvisioned = "replication.storage.openshift.io/provisioned-by"
)

// VolumeGroupReplicationReconciler reconciles a VolumeGroupReplication object
type VolumeGroupReplicationReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	ctx              context.Context
	log              logr.Logger
	Recorder         record.EventRecorder
	MaxGroupPVCCount int
}

//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=replication.storage.openshift.io,resources=volumegroupreplications,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=replication.storage.openshift.io,resources=volumegroupreplications/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=replication.storage.openshift.io,resources=volumegroupreplications/finalizers,verbs=update
//+kubebuilder:rbac:groups=replication.storage.openshift.io,resources=volumegroupreplicationclasses,verbs=get;list;watch
//+kubebuilder:rbac:groups=replication.storage.openshift.io,resources=volumegroupreplicationcontents,verbs=get;list;watch;create;update;delete
//+kubebuilder:rbac:groups=replication.storage.openshift.io,resources=volumereplications,verbs=get;list;watch;create;update;delete
//+kubebuilder:rbac:groups=replication.storage.openshift.io,resources=volumereplications/status,verbs=get;list
//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims;persistentvolumes,verbs=get;list;watch;update
//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims/finalizers,verbs=update

/*
Steps performed by the reconcile loop:
- Fetch and validate the VGRClass CR
- Fetch the matching PVCs based on the selector provided in the VGR CR, and check if they are already bounded to a CSI volume and the driver matches the driver provided in the VGRClass CR.
- Annotate to the PVCs with owner and add the VGR finalizer to them.
- Add the label selector to the VGR annotation, such that the PVC triggering a reconcile can fetch the VGR to reconcile
- Create the VGRContent with the PVs list fetched above, add VGR name/namespace as the annotation to it
- Wait for the volumes to be grouped, and the VGRContent to be updated with the group handle
- Then, create the VR CR and add VGR name/namespace as the annotation to it
- Update the VGR status with the VR status.

In case of deletion:
- Remove the owner annotations and finalizers from the PVC
- Check if VR exists, then delete
- Check if VGRContent exists, then delete
- Remove VGR finalizer <- This won't happen until the dependent VR and VRContent is deleted. Validated using owner annotations set in both the dependent CRs.
*/

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *VolumeGroupReplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log = log.FromContext(ctx, "Request.Name", req.Name, "Request.Namespace", req.Namespace)
	r.ctx = log.IntoContext(ctx, r.log)

	// Fetch VolumeGroupReplication instance
	instance := &replicationv1alpha1.VolumeGroupReplication{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			r.log.Info("volumeGroupReplication resource not found")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Skip reconcile if external VGR
	if instance.Spec.External {
		r.log.Info("skipping reconcile for non csi-addons managed VGR")
		return reconcile.Result{}, nil
	}

	// Get VolumeGroupReplicationClass instance
	vgrClassObj, err := r.getVolumeGroupReplicationClass(instance.Spec.VolumeGroupReplicationClassName)
	if err != nil {
		r.log.Error(err, "failed to get volumeGroupReplicationClass resource", "VGRClassName", instance.Spec.VolumeGroupReplicationClassName)
		_ = r.setGroupReplicationFailure(instance, err)
		return reconcile.Result{}, err
	}

	// Validate that required parameters are present in the VGRClass resource
	err = validatePrefixedParameters(vgrClassObj.Spec.Parameters)
	if err != nil {
		r.log.Error(err, "failed to validate parameters of volumeGroupReplicationClass", "VGRClassName", instance.Spec.VolumeGroupReplicationClassName)
		_ = r.setGroupReplicationFailure(instance, err)
		return reconcile.Result{}, err
	}

	secretName := vgrClassObj.Spec.Parameters[prefixedGroupReplicationSecretNameKey]
	secretNamespace := vgrClassObj.Spec.Parameters[prefixedGroupReplicationSecretNamespaceKey]

	// Declare all dependent resources
	vgrContentName := fmt.Sprintf("vgrcontent-%s", instance.UID)
	if instance.Spec.VolumeGroupReplicationContentName != "" {
		vgrContentName = instance.Spec.VolumeGroupReplicationContentName
	}
	vgrContentObj := &replicationv1alpha1.VolumeGroupReplicationContent{
		ObjectMeta: metav1.ObjectMeta{
			Name: vgrContentName,
		},
	}
	if secretName != "" && secretNamespace != "" {
		metav1.SetMetaDataAnnotation(&vgrContentObj.ObjectMeta, prefixedGroupReplicationSecretNameKey, secretName)
		metav1.SetMetaDataAnnotation(&vgrContentObj.ObjectMeta, prefixedGroupReplicationSecretNamespaceKey, secretNamespace)
	}

	vrName := fmt.Sprintf("vr-%s", instance.UID)
	if instance.Spec.VolumeReplicationName != "" {
		vrName = instance.Spec.VolumeReplicationName
	}
	vrObj := &replicationv1alpha1.VolumeReplication{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vrName,
			Namespace: instance.Namespace,
		},
	}

	// Create/Update dependent resources only if the instance is not marked for deletion
	if instance.GetDeletionTimestamp().IsZero() {
		// Add finalizer to VGR instance
		if err = addFinalizerToVGR(r.Client, r.log, instance); err != nil {
			r.log.Error(err, "failed to add VolumeGroupReplication finalizer")
			return reconcile.Result{}, err
		}

		// Check if PVCs exist based on provided selectors
		pvcList, labelSelector, err := r.getMatchingPVCsFromSource(instance)
		if err != nil {
			r.log.Error(err, "failed to get PVCs using selector")
			_ = r.setGroupReplicationFailure(instance, err)
			return reconcile.Result{}, err
		}
		if len(pvcList) > r.MaxGroupPVCCount {
			err = fmt.Errorf("more than %q PVCs match the given selector", r.MaxGroupPVCCount)
			r.log.Error(err, "only %q PVCs are allowed for volume group replication", r.MaxGroupPVCCount)
			_ = r.setGroupReplicationFailure(instance, err)
			return reconcile.Result{}, err
		}

		// Add the string representation of the labelSelector to the VGR annotation
		if instance.Annotations == nil {
			instance.Annotations = make(map[string]string)
		}

		// We need to save the label selector in the annotation, so that an event in PVC
		// triggers the reconcile particularly for the VGR that the PVC is part of by comparing
		// the labels on the pvc with the pvcSelector annotation in VGR
		if instance.Annotations[pvcSelector] != labelSelector {
			instance.Annotations[pvcSelector] = labelSelector
			err = r.Update(ctx, instance)
			if err != nil {
				r.log.Error(err, "failed to add pvc selector annotation to VGR")
				_ = r.setGroupReplicationFailure(instance, err)
				return reconcile.Result{}, err
			}
		}

		// Update annotation,finalizers for old,new PVCs
		pvcRefList, err := r.updateFinalizerAndAnnotationOnPVCs(instance, pvcList)
		if err != nil {
			_ = r.setGroupReplicationFailure(instance, err)
			return reconcile.Result{}, err
		}

		// Update PersistentVolumeClaimsRefList in VGR Status
		if !reflect.DeepEqual(instance.Status.PersistentVolumeClaimsRefList, pvcRefList) {
			instance.Status.PersistentVolumeClaimsRefList = pvcRefList
			err = r.Status().Update(ctx, instance)
			if err != nil {
				r.log.Error(err, "failed to update VolumeGroupReplication resource")
				_ = r.setGroupReplicationFailure(instance, err)
				return reconcile.Result{}, err
			}
		}

		// Create/Update VolumeGroupReplicationContent CR
		pvHandlesList, err := r.getPVHandles(vgrClassObj, pvcList)
		if err != nil {
			r.log.Error(err, "failed to get PV handles for PVCs")
			_ = r.setGroupReplicationFailure(instance, err)
			return reconcile.Result{}, err
		}
		err = r.createOrUpdateVolumeGroupReplicationContentCR(instance, vgrContentObj, vgrClassObj.Spec.Provisioner, pvHandlesList)
		if err != nil {
			r.log.Error(err, "failed to create/update volumeGroupReplicationContent resource", "VGRContentName", vgrContentObj.Name)
			_ = r.setGroupReplicationFailure(instance, err)
			return reconcile.Result{}, err
		}

		// Update the VGR with VGRContentName, if empty
		if instance.Spec.VolumeGroupReplicationContentName == "" {
			instance.Spec.VolumeGroupReplicationContentName = vgrContentObj.Name
			err = r.Update(ctx, instance)
			if err != nil {
				r.log.Error(err, "failed to update volumeGroupReplication instance", "VGRName", instance.Name)
				_ = r.setGroupReplicationFailure(instance, err)
				return reconcile.Result{}, err
			}
		}

		// Since, the grouping may take few seconds to happen, just exit and wait for the reconcile
		// to be triggered when the group handle is updated in the vgrcontent resource.
		if vgrContentObj.Spec.VolumeGroupReplicationHandle == "" {
			r.log.Info("Either volumegroupreplicationcontent is not yet created or it is still grouping the volumes to be replicated")
			return reconcile.Result{}, nil
		} else {
			// Create/Update VolumeReplication CR
			err = r.createOrUpdateVolumeReplicationCR(instance, vrObj)
			if err != nil {
				r.log.Error(err, "failed to create/update volumeReplication resource", "VRName", vrObj.Name)
				_ = r.setGroupReplicationFailure(instance, err)
				return reconcile.Result{}, err
			}

			// Update the VGR with VolumeReplication resource name, if not present
			if instance.Spec.VolumeReplicationName == "" {
				instance.Spec.VolumeReplicationName = vrObj.Name
				err = r.Update(ctx, instance)
				if err != nil {
					r.log.Error(err, "failed to update volumeGroupReplication instance", "VGRName", instance.Name)
					_ = r.setGroupReplicationFailure(instance, err)
					return reconcile.Result{}, err
				}
			}
		}
	} else {
		// When the VGR resource is being deleted
		// If dependent VR was created, delete it
		if instance.Spec.VolumeReplicationName != "" {
			err = r.cleanupVR(instance, vrObj)
			if err != nil {
				return reconcile.Result{}, err
			}
		}

		// If dependent VGRContent was created, delete it
		if instance.Spec.VolumeGroupReplicationContentName != "" {
			err = r.cleanupVGRContent(instance, vgrContentObj)
			if err != nil {
				return reconcile.Result{}, err
			}
		}

		// Check if owner annotations are removed from the VGR resource
		if isSafeToDeleteVGR(instance) {
			// Remove the owner annotation and the finalizer from pvcs that are part of VGR,
			// as the dependent resources like VR, VGRContent are already deleted and we can
			// safely delete/remove finalizer from VGR in the next step.
			if instance.Status.PersistentVolumeClaimsRefList != nil {
				err = r.cleanupGroupPVC(instance)
				if err != nil {
					return reconcile.Result{}, err
				}
			}

			// Just log error, and exit reconcile without error. The dependent resource will update the VGR
			// to remove their names from the CR, that will trigger a reconcile.
			if err = removeFinalizerFromVGR(r.Client, r.log, instance); err != nil {
				return reconcile.Result{}, nil
			}
		} else {
			r.log.Info("cannot delete volumeGroupReplication object yet, as the "+
				"dependent resources are not yet deleted", "namespace", instance.Namespace, "name", instance.Name)
			return reconcile.Result{
				RequeueAfter: 10 * time.Second,
			}, nil
		}

		r.log.Info("volumeGroupReplication object is terminated, skipping reconciliation")
		return reconcile.Result{}, nil
	}

	// Update VGR status based on VR Status
	instance.Status.VolumeReplicationStatus = vrObj.Status
	instance.Status.ObservedGeneration = instance.Generation
	for i := range instance.Status.Conditions {
		instance.Status.Conditions[i].ObservedGeneration = instance.Generation
	}
	err = r.Status().Update(ctx, instance)
	if err != nil {
		r.log.Error(err, "failed to update volumeGroupReplication instance's status", "VGRName", instance.Name)
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *VolumeGroupReplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Wait for the group CRDs to be present, i.e, VolumeGroupReplication, VolumeGroupReplicationClass and
	// VolumeGroupReplicationContent
	err := r.waitForGroupCrds()
	if err != nil {
		return err
	}

	// Only reconcile for spec/status update events
	skipUpdates := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}

	// Watch for only status updates of the VR resource
	watchOnlyStatusUpdates := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.ObjectOld == nil || e.ObjectNew == nil {
				return false
			}
			oldObj := e.ObjectOld.(*replicationv1alpha1.VolumeReplication)
			newObj := e.ObjectNew.(*replicationv1alpha1.VolumeReplication)
			return !reflect.DeepEqual(oldObj.Status, newObj.Status)
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return false
		},
	}

	// Watch for only spec updates of the VGRContent resource
	watchOnlySpecUpdates := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.ObjectOld == nil || e.ObjectNew == nil {
				return false
			}
			oldObj := e.ObjectOld.(*replicationv1alpha1.VolumeGroupReplicationContent)
			newObj := e.ObjectNew.(*replicationv1alpha1.VolumeGroupReplicationContent)
			return !reflect.DeepEqual(oldObj.Spec, newObj.Spec)
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return false
		},
	}

	// Enqueue the VGR reconcile with the VGR name,namespace based on the annotation of the VR and VGRContent CR
	enqueueVGRRequest := handler.EnqueueRequestsFromMapFunc(
		func(context context.Context, obj client.Object) []reconcile.Request {
			// Get the VolumeGroupReplication name,namespace
			var vgrName, vgrNamespace string
			objAnnotations := obj.GetAnnotations()
			for k, v := range objAnnotations {
				if k == volumeGroupReplicationRef {
					vgrNamespace = strings.Split(v, "/")[0]
					vgrName = strings.Split(v, "/")[1]
					break
				}
			}

			// Skip reconcile if the triggering resource is not a sub-resource of VGR
			if vgrName == "" || vgrNamespace == "" {
				return []reconcile.Request{}
			}

			// Return a reconcile request with the name, namespace of the VolumeGroupReplication resource
			return []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Namespace: vgrNamespace,
						Name:      vgrName,
					},
				},
			}
		},
	)

	// Enqueue the VGR reconcile with the VGR name,namespace based on the labels of the VGR CR
	enqueueVGRForPVCRequest := handler.EnqueueRequestsFromMapFunc(
		func(context context.Context, obj client.Object) []reconcile.Request {
			// Check if the PVC has any labels defined
			objLabels := obj.GetLabels()
			if len(objLabels) == 0 {
				return []reconcile.Request{}
			}

			// Check if the resource is present in the cluster
			vgrObjsList := &replicationv1alpha1.VolumeGroupReplicationList{}
			logger := log.FromContext(context)
			err := r.List(context, vgrObjsList)
			if err != nil {
				logger.Error(err, "failed to list VolumeGroupReplication instances")
				return []reconcile.Request{}
			}

			// Check if the pvc labels match any VGRs based on selectors present in it's annotation
			for _, vgr := range vgrObjsList.Items {
				// Skip reconcile for PVC that belongs to an external VGR
				if vgr.Spec.External {
					continue
				}
				if vgr.Annotations != nil && vgr.Annotations[pvcSelector] != "" {
					labelSelector, err := labels.Parse(vgr.Annotations[pvcSelector])
					if err != nil {
						logger.Error(err, "failed to parse selector from VolumeGroupReplication's annotation")
						return []reconcile.Request{}
					}
					objLabelsSet := labels.Set(objLabels)
					if labelSelector.Matches(objLabelsSet) {
						return []reconcile.Request{
							{
								NamespacedName: types.NamespacedName{
									Namespace: vgr.Namespace,
									Name:      vgr.Name,
								},
							},
						}
					}
				}
			}

			return []reconcile.Request{}
		},
	)

	return ctrl.NewControllerManagedBy(mgr).
		For(&replicationv1alpha1.VolumeGroupReplication{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&replicationv1alpha1.VolumeGroupReplicationContent{}, builder.WithPredicates(skipUpdates)).
		Owns(&replicationv1alpha1.VolumeReplication{}, builder.WithPredicates(skipUpdates)).
		Watches(&replicationv1alpha1.VolumeGroupReplicationContent{}, enqueueVGRRequest, builder.WithPredicates(watchOnlySpecUpdates)).
		Watches(&replicationv1alpha1.VolumeReplication{}, enqueueVGRRequest, builder.WithPredicates(watchOnlyStatusUpdates)).
		Watches(&corev1.PersistentVolumeClaim{}, enqueueVGRForPVCRequest, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
}

// waitForGroupCrds waits for dependent CRDs to the available in the cluster
func (r *VolumeGroupReplicationReconciler) waitForGroupCrds() error {
	err := WaitForVolumeReplicationResource(r.Client, r.log, volumeGroupReplicationClass)
	if err != nil {
		r.log.Error(err, "failed to wait for VolumeGroupReplicationClass CRD")
		return err
	}

	err = WaitForVolumeReplicationResource(r.Client, r.log, volumeGroupReplication)
	if err != nil {
		r.log.Error(err, "failed to wait for VolumeGroupReplication CRD")
		return err
	}

	err = WaitForVolumeReplicationResource(r.Client, r.log, volumeGroupReplicationContent)
	if err != nil {
		r.log.Error(err, "failed to wait for VolumeGroupReplicationContent CRD")
		return err
	}

	return nil
}

// setGroupReplicationFailure sets the failure replication status on the VolumeGroupReplication resource
func (r *VolumeGroupReplicationReconciler) setGroupReplicationFailure(
	instance *replicationv1alpha1.VolumeGroupReplication, err error) error {

	instance.Status.State = GetCurrentReplicationState(instance.Status.State)
	instance.Status.Message = replication.GetMessageFromError(err)
	instance.Status.ObservedGeneration = instance.Generation
	if err := r.Status().Update(r.ctx, instance); err != nil {
		r.log.Error(err, "failed to update volumeGroupReplication status", "VGRName", instance.Name)
		return err
	}

	return nil
}

// getMatchingPVCsFromSource fecthes the PVCs based on the selectors defined in the VolumeGroupReplication resource
func (r *VolumeGroupReplicationReconciler) getMatchingPVCsFromSource(instance *replicationv1alpha1.VolumeGroupReplication) ([]corev1.PersistentVolumeClaim, string, error) {

	pvcList := corev1.PersistentVolumeClaimList{}

	selector, err := metav1.LabelSelectorAsSelector(instance.Spec.Source.Selector)
	if err != nil {
		return nil, "", err
	}

	opts := []client.ListOption{
		client.MatchingLabelsSelector{Selector: selector},
		client.InNamespace(instance.Namespace),
	}
	err = r.List(r.ctx, &pvcList, opts...)
	if err != nil {
		r.log.Error(err, "failed to list pvcs with the given selectors", "Selector", selector.String())
		return nil, "", err
	}

	// Update events if PVC is marked for deletion, but contains the pvc selector label of group and
	// also remove the PVCs from the list if they are already marked for deletion before being part
	// of the group
	removeDeletingPVC := []corev1.PersistentVolumeClaim{}
	for _, pvc := range pvcList.Items {
		if !pvc.DeletionTimestamp.IsZero() {
			if slices.Contains(pvc.Finalizers, vgrReplicationFinalizer) {
				// PVC is marked for deletion, but not deleted because it is still a part of
				// group using label selectors. Add an event to the PVC mentioning the same
				msg := fmt.Sprintf("PersistentVolumeClaim is part of the group(%s/%s) using matching label selector. Remove label from PersistentVolumeClaim (%s/%s) to allow deletion",
					instance.Namespace, instance.Name, pvc.Namespace, pvc.Name)
				r.Recorder.Event(&pvc, "Warning", "PersistentVolumeClaimDeletionBlocked", msg)
			} else {
				removeDeletingPVC = append(removeDeletingPVC, pvc)
			}
		}
	}

	updatedPVCList := []corev1.PersistentVolumeClaim{}
	if len(removeDeletingPVC) > 0 {
		for _, pvc := range pvcList.Items {
			if !slices.ContainsFunc(removeDeletingPVC, func(removePVC corev1.PersistentVolumeClaim) bool {
				return removePVC.Name == pvc.Name
			}) {
				updatedPVCList = append(updatedPVCList, pvc)
			}
		}
	} else {
		updatedPVCList = pvcList.Items
	}

	return updatedPVCList, selector.String(), nil
}

// getPVHandles fetches the PV handles for the respective PVCs
func (r *VolumeGroupReplicationReconciler) getPVHandles(vgrClass *replicationv1alpha1.VolumeGroupReplicationClass,
	pvcList []corev1.PersistentVolumeClaim) ([]string, error) {
	pvHandlesList := []string{}
	for _, pvc := range pvcList {
		pvName := pvc.Spec.VolumeName
		pv := &corev1.PersistentVolume{}
		err := r.Get(r.ctx, types.NamespacedName{Name: pvName}, pv)
		if err != nil {
			r.log.Error(err, "failed to get pv for corresponding pvc", "PVC Name", pvc.Name)
			return nil, err
		}
		if pv.Spec.CSI == nil {
			err = fmt.Errorf("pvc %s is not bound to a CSI PV", pvc.Name)
			return nil, err
		}
		if pv.Spec.CSI.Driver != vgrClass.Spec.Provisioner {
			err = fmt.Errorf("driver (%s) of PV for PVC %s is different than the VolumeGroupReplicationClass driver (%s)", pvc.Name, pv.Spec.CSI.Driver, vgrClass.Spec.Provisioner)
			return nil, err
		}
		pvHandlesList = append(pvHandlesList, pv.Spec.CSI.VolumeHandle)
	}

	// Sort the pvHandles list and then pass, because reflect.DeepEqual checks for positional equality
	slices.Sort(pvHandlesList)

	return pvHandlesList, nil
}

func (r *VolumeGroupReplicationReconciler) updateFinalizerAndAnnotationOnPVCs(vgr *replicationv1alpha1.VolumeGroupReplication,
	pvcList []corev1.PersistentVolumeClaim) ([]corev1.LocalObjectReference, error) {

	// Create a set of old PVC names
	oldPVCs := make(map[string]struct{}, len(vgr.Status.PersistentVolumeClaimsRefList))
	for _, pvc := range vgr.Status.PersistentVolumeClaimsRefList {
		oldPVCs[pvc.Name] = struct{}{}
	}

	// Track PVCs to add and remove
	toAddPVCs := make([]corev1.PersistentVolumeClaim, 0)
	toRemovePVCs := make([]corev1.PersistentVolumeClaim, 0)

	// Create a set for current PVC names to determine removals
	currentPVCs := make(map[string]struct{}, len(pvcList))
	pvcRefList := []corev1.LocalObjectReference{}
	for _, pvc := range pvcList {
		currentPVCs[pvc.Name] = struct{}{}
		pvcRefList = append(pvcRefList, corev1.LocalObjectReference{
			Name: pvc.Name,
		})

		// If it's not in oldPVCs, it's an addition
		if _, exists := oldPVCs[pvc.Name]; !exists {
			toAddPVCs = append(toAddPVCs, pvc)
		}
	}

	// Identify PVCs that should be removed
	for name := range oldPVCs {
		if _, exists := currentPVCs[name]; !exists {
			tmpPVC := corev1.PersistentVolumeClaim{}
			err := r.Get(r.ctx, types.NamespacedName{Name: name, Namespace: vgr.Namespace}, &tmpPVC)
			if err != nil {
				r.log.Error(err, "failed to get pvc", "PVC Name", name)
				return nil, err
			}
			toRemovePVCs = append(toRemovePVCs, tmpPVC)
		}
	}

	// Annotate each new PVC with owner and add finalizer to it
	for _, pvc := range toAddPVCs {
		err := annotatePVCWithOwner(r.Client, r.ctx, r.log, vgr.Name, &pvc, replicationv1alpha1.VolumeGroupReplicationNameAnnotation)
		if err != nil {
			r.log.Error(err, "Failed to add VGR owner annotation on PVC")
			return nil, err
		}

		if err = addFinalizerToPVC(r.Client, r.log, &pvc, vgrReplicationFinalizer); err != nil {
			r.log.Error(err, "Failed to add VGR finalizer on PVC")
			return nil, err
		}
	}

	// Remove annotation and finalizer from PVCs that are not part of group
	for _, pvc := range toRemovePVCs {
		err := removeOwnerFromPVCAnnotation(r.Client, r.ctx, r.log, &pvc, replicationv1alpha1.VolumeGroupReplicationNameAnnotation)
		if err != nil {
			r.log.Error(err, "Failed to remove VGR owner annotation from PVC")
			return nil, err
		}

		if err = removeFinalizerFromPVC(r.Client, r.log, &pvc, vgrReplicationFinalizer); err != nil {
			r.log.Error(err, "Failed to remove VGR finalizer from PVC")
			return nil, err
		}
	}

	return pvcRefList, nil
}

func (r *VolumeGroupReplicationReconciler) cleanupGroupPVC(vgr *replicationv1alpha1.VolumeGroupReplication) error {
	for _, pvcRef := range vgr.Status.PersistentVolumeClaimsRefList {
		pvc := &corev1.PersistentVolumeClaim{}
		err := r.Get(r.ctx, types.NamespacedName{Name: pvcRef.Name, Namespace: vgr.Namespace}, pvc)
		if err != nil {
			r.log.Error(err, "failed to fetch pvc from VGR status")
			return err
		}

		if err = removeOwnerFromPVCAnnotation(r.Client, r.ctx, r.log, pvc, replicationv1alpha1.VolumeGroupReplicationNameAnnotation); err != nil {
			r.log.Error(err, "Failed to remove VolumeReplication annotation from PersistentVolumeClaim")

			return err
		}

		if err = removeFinalizerFromPVC(r.Client, r.log, pvc, vgrReplicationFinalizer); err != nil {
			r.log.Error(err, "Failed to remove VGR finalizer from PersistentVolumeClaim")
			return err
		}
	}

	return nil
}

func (r *VolumeGroupReplicationReconciler) createOrUpdateVolumeGroupReplicationContentCR(vgr *replicationv1alpha1.VolumeGroupReplication,
	vgrContentObj *replicationv1alpha1.VolumeGroupReplicationContent, driver string, pvHandlesList []string) error {
	vgrRef := fmt.Sprintf("%s/%s", vgr.Namespace, vgr.Name)
	_, err := controllerutil.CreateOrUpdate(r.ctx, r.Client, vgrContentObj, func() error {
		if vgr.Spec.VolumeGroupReplicationContentName != "" && vgrContentObj.CreationTimestamp.IsZero() {
			return errors.New("dependent VGRContent resource is not yet created, waiting for it to be created")
		}
		if vgrContentObj.CreationTimestamp.IsZero() {
			metav1.SetMetaDataAnnotation(&vgrContentObj.ObjectMeta, volumeGroupReplicationRef, vgrRef)
			vgrContentObj.Spec = replicationv1alpha1.VolumeGroupReplicationContentSpec{
				VolumeGroupReplicationRef: &corev1.ObjectReference{
					APIVersion: vgr.APIVersion,
					Kind:       vgr.Kind,
					Name:       vgr.Name,
					Namespace:  vgr.Namespace,
					UID:        vgr.UID,
				},
				Provisioner:                     driver,
				VolumeGroupReplicationClassName: vgr.Spec.VolumeGroupReplicationClassName,
				Source: replicationv1alpha1.VolumeGroupReplicationContentSource{
					VolumeHandles: pvHandlesList,
				},
			}

			return nil
		}

		if vgrContentObj.Annotations[volumeGroupReplicationRef] != vgrRef {
			metav1.SetMetaDataAnnotation(&vgrContentObj.ObjectMeta, volumeGroupReplicationRef, vgrRef)
		}
		metav1.SetMetaDataAnnotation(&vgrContentObj.ObjectMeta, annDynamicallyProvisioned, driver)

		if vgrContentObj.Spec.VolumeGroupReplicationRef == nil {
			vgrContentObj.Spec.VolumeGroupReplicationRef = &corev1.ObjectReference{
				APIVersion: vgr.APIVersion,
				Kind:       vgr.Kind,
				Name:       vgr.Name,
				Namespace:  vgr.Namespace,
				UID:        vgr.UID,
			}
		}

		vgrContentObj.Spec.Source = replicationv1alpha1.VolumeGroupReplicationContentSource{
			VolumeHandles: pvHandlesList,
		}

		return nil
	})

	return err
}

func (r *VolumeGroupReplicationReconciler) cleanupVGRContent(vgr *replicationv1alpha1.VolumeGroupReplication,
	vgrContentObj *replicationv1alpha1.VolumeGroupReplicationContent) error {
	req := types.NamespacedName{Name: vgr.Spec.VolumeGroupReplicationContentName}
	err := r.Get(r.ctx, req, vgrContentObj)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			r.log.Error(err, "failed to fetch dependent volumeGroupReplicationContent resource")
			return err
		}
		r.log.Info("volumeGroupReplicationContent resource not found")
		return nil
	}

	err = r.Delete(r.ctx, vgrContentObj)
	if err != nil {
		r.log.Error(err, "failed to delete dependent volumeGroupReplicationContent resource")
	}

	return err
}

func (r *VolumeGroupReplicationReconciler) createOrUpdateVolumeReplicationCR(vgr *replicationv1alpha1.VolumeGroupReplication,
	vrObj *replicationv1alpha1.VolumeReplication) error {
	apiGroup := "replication.storage.openshift.io"
	_, err := controllerutil.CreateOrUpdate(r.ctx, r.Client, vrObj, func() error {
		if vrObj.CreationTimestamp.IsZero() {
			vrObj.Annotations = map[string]string{
				volumeGroupReplicationRef: fmt.Sprintf("%s/%s", vgr.Namespace, vgr.Name),
			}
			vrObj.Spec = replicationv1alpha1.VolumeReplicationSpec{
				VolumeReplicationClass: vgr.Spec.VolumeReplicationClassName,
				DataSource: corev1.TypedLocalObjectReference{
					APIGroup: &apiGroup,
					Kind:     vgr.Kind,
					Name:     vgr.Name,
				},
			}
		}

		vrObj.Spec.AutoResync = vgr.Spec.AutoResync
		vrObj.Spec.ReplicationState = vgr.Spec.ReplicationState

		return controllerutil.SetOwnerReference(vgr, vrObj, r.Scheme)
	})

	return err
}

func (r *VolumeGroupReplicationReconciler) cleanupVR(vgr *replicationv1alpha1.VolumeGroupReplication,
	vrObj *replicationv1alpha1.VolumeReplication) error {
	req := types.NamespacedName{Name: vgr.Spec.VolumeReplicationName, Namespace: vgr.Namespace}
	err := r.Get(r.ctx, req, vrObj)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			r.log.Error(err, "failed to fetch dependent volumeReplication resource")
			return err
		}
		r.log.Info("volumeReplication resource not found")
		return nil
	}

	err = r.Delete(r.ctx, vrObj)
	if err != nil {
		r.log.Error(err, "failed to delete dependent volumeReplication resource")
	}

	return err
}
