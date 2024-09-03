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
	"fmt"
	"reflect"
	"strings"

	"github.com/csi-addons/kubernetes-csi-addons/internal/controller/replication.storage/replication"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
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
	"github.com/go-logr/logr"
)

const (
	volumeGroupReplicationClass   = "VolumeGroupReplicationClass"
	volumeGroupReplication        = "VolumeGroupReplication"
	volumeGroupReplicationContent = "VolumeGroupReplicationContent"
	volumeGroupReplicationRef     = "replication.storage.openshift.io/volumegroupref"
)

// VolumeGroupReplicationReconciler reconciles a VolumeGroupReplication object
type VolumeGroupReplicationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

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
	logger := log.FromContext(ctx, "Request.Name", req.Name, "Request.Namespace", req.Namespace)

	// Fetch VolumeGroupReplication instance
	instance := &replicationv1alpha1.VolumeGroupReplication{}
	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("volumeGroupReplication resource not found")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Get VolumeGroupReplicationClass instance
	vgrClassObj, err := r.getVolumeGroupReplicationClass(logger, instance.Spec.VolumeGroupReplicationClassName)
	if err != nil {
		logger.Error(err, "failed to get volumeGroupReplicationClass resource", "VGRClassName", instance.Spec.VolumeGroupReplicationClassName)
		_ = r.setGroupReplicationFailure(instance, logger, err)
		return reconcile.Result{}, err
	}

	// Validate that required parameters are present in the VGRClass resource
	err = validatePrefixedParameters(vgrClassObj.Spec.Parameters)
	if err != nil {
		logger.Error(err, "failed to validate parameters of volumeGroupReplicationClass", "VGRClassName", instance.Spec.VolumeGroupReplicationClassName)
		_ = r.setGroupReplicationFailure(instance, logger, err)
		return reconcile.Result{}, err
	}

	// Declare all dependent resources
	vgrContentObj := &replicationv1alpha1.VolumeGroupReplicationContent{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("vgrcontent-%s", instance.UID),
		},
	}
	vrObj := &replicationv1alpha1.VolumeReplication{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("vr-%s", instance.UID),
			Namespace: instance.Namespace,
		},
	}

	// Create/Update dependent resources only if the instance is not marked for deletion
	if instance.GetDeletionTimestamp().IsZero() {

		// Add finalizer to VGR instance
		if err = AddFinalizerToVGR(r.Client, logger, instance); err != nil {
			logger.Error(err, "failed to add VolumeGroupReplication finalizer")
			return reconcile.Result{}, err
		}

		// Check if PVCs exist based on provided selectors
		pvcList, pvHandlesList, labelSelector, err := r.getMatchingPVCsFromSource(instance, logger, vgrClassObj)
		if err != nil {
			logger.Error(err, "failed to get PVCs using selector")
			_ = r.setGroupReplicationFailure(instance, logger, err)
			return reconcile.Result{}, err
		}
		if len(pvcList.Items) == 0 {
			err = fmt.Errorf("no matching PVCs found for the given selectors")
			logger.Error(err, "provided selector should match at least 1 PVC")
			_ = r.setGroupReplicationFailure(instance, logger, err)
			return reconcile.Result{}, err
		} else if len(pvcList.Items) > 100 {
			err = fmt.Errorf("more than 100 PVCs match the given selector")
			logger.Error(err, "only 100 PVCs are allowed for volume group replication")
			_ = r.setGroupReplicationFailure(instance, logger, err)
			return reconcile.Result{}, err
		}

		// Add the string representation of the labelSelector to the VGR annotation
		if instance.ObjectMeta.Annotations == nil {
			instance.ObjectMeta.Annotations = make(map[string]string)
		}

		if instance.ObjectMeta.Annotations["pvcSelector"] != labelSelector {
			instance.ObjectMeta.Annotations["pvcSelector"] = labelSelector
			err = r.Client.Update(ctx, instance)
			if err != nil {
				logger.Error(err, "failed to add pvc selector annotation to VGR")
				_ = r.setGroupReplicationFailure(instance, logger, err)
				return reconcile.Result{}, err
			}
		}

		// Update PersistentVolumeClaimsRefList in VGR Status
		tmpRefList := []corev1.LocalObjectReference{}
		for _, pvc := range pvcList.Items {
			tmpRefList = append(tmpRefList, corev1.LocalObjectReference{
				Name: pvc.Name,
			})
		}

		// Annotate each PVC with owner and add finalizer to it
		for _, pvc := range pvcList.Items {
			reqOwner := fmt.Sprintf("%s/%s", instance.Name, instance.Namespace)
			err = AnnotatePVCWithOwner(r.Client, logger, reqOwner, &pvc, replicationv1alpha1.VolumeGroupReplicationNameAnnotation)
			if err != nil {
				logger.Error(err, "Failed to add VGR owner annotation on PVC")
				return ctrl.Result{}, err
			}

			if err = AddFinalizerToPVC(r.Client, logger, &pvc, vgrReplicationFinalizer); err != nil {
				logger.Error(err, "Failed to add VGR finalizer on PersistentVolumeClaim")
				return reconcile.Result{}, err
			}
		}

		// Update PersistentVolumeClaimsRefList in VGR Status
		if !reflect.DeepEqual(instance.Status.PersistentVolumeClaimsRefList, tmpRefList) {
			instance.Status.PersistentVolumeClaimsRefList = tmpRefList
			err = r.Client.Status().Update(ctx, instance)
			if err != nil {
				logger.Error(err, "failed to update VolumeGroupReplication resource")
				_ = r.setGroupReplicationFailure(instance, logger, err)
				return reconcile.Result{}, err
			}
		}

		// Create/Update VolumeGroupReplicationContent CR
		err = r.createVolumeGroupReplicationContentCR(instance, vgrContentObj, vgrClassObj.Spec.Provisioner, pvHandlesList)
		if err != nil {
			logger.Error(err, "failed to create/update volumeGroupReplicationContent resource", "VGRContentName", vgrContentObj.Name)
			_ = r.setGroupReplicationFailure(instance, logger, err)
			return reconcile.Result{}, err
		}

		// Update the VGR with VGRContentName, if empty
		if instance.Spec.VolumeGroupReplicationContentName == "" {
			instance.Spec.VolumeGroupReplicationContentName = vgrContentObj.Name
			err = r.Client.Update(ctx, instance)
			if err != nil {
				logger.Error(err, "failed to update volumeGroupReplication instance", "VGRName", instance.Name)
				_ = r.setGroupReplicationFailure(instance, logger, err)
				return reconcile.Result{}, err
			}
		}

		// Since, the grouping may take few seconds to happen, just exit and wait for the reconcile
		// to be triggered when the group handle is updated in the vgrcontent resource.
		if vgrContentObj.Spec.VolumeGroupReplicationHandle == "" {
			logger.Info("Either volumegroupreplicationcontent is not yet created or it is still grouping the volumes to be replicated")
			return reconcile.Result{}, nil
		} else {
			// Create/Update VolumeReplication CR
			err = r.createVolumeReplicationCR(instance, vrObj)
			if err != nil {
				logger.Error(err, "failed to create/update volumeReplication resource", "VRName", vrObj.Name)
				_ = r.setGroupReplicationFailure(instance, logger, err)
				return reconcile.Result{}, err
			}

			// Update the VGR with VolumeReplication resource name, if not present
			if instance.Spec.VolumeReplicationName == "" {
				instance.Spec.VolumeReplicationName = vrObj.Name
				err = r.Client.Update(ctx, instance)
				if err != nil {
					logger.Error(err, "failed to update volumeGroupReplication instance", "VGRName", instance.Name)
					_ = r.setGroupReplicationFailure(instance, logger, err)
					return reconcile.Result{}, err
				}
			}
		}
	} else {
		// When the VGR resource is being deleted
		// Remove the owner annotation and the finalizer from pvcs that exist in VGR resource's status
		if instance.Status.PersistentVolumeClaimsRefList != nil {
			for _, pvcRef := range instance.Status.PersistentVolumeClaimsRefList {
				pvc := &corev1.PersistentVolumeClaim{}
				err = r.Client.Get(ctx, types.NamespacedName{Name: pvcRef.Name, Namespace: req.Namespace}, pvc)
				if err != nil {
					logger.Error(err, "failed to fetch pvc from VGR status")
					return reconcile.Result{}, err
				}

				if err = RemoveOwnerFromPVCAnnotation(r.Client, logger, pvc, replicationv1alpha1.VolumeGroupReplicationNameAnnotation); err != nil {
					logger.Error(err, "Failed to remove VolumeReplication annotation from PersistentVolumeClaim")

					return reconcile.Result{}, err
				}

				if err = RemoveFinalizerFromPVC(r.Client, logger, pvc, vgrReplicationFinalizer); err != nil {
					logger.Error(err, "Failed to remove VGR finalizer from PersistentVolumeClaim")
					return reconcile.Result{}, err
				}
			}
		}
		// If dependent VR was created, delete it
		if instance.Spec.VolumeReplicationName != "" {
			req := types.NamespacedName{Name: instance.Spec.VolumeReplicationName, Namespace: req.Namespace}
			err = r.Client.Get(ctx, req, vrObj)
			if err != nil {
				if errors.IsNotFound(err) {
					logger.Info("volumeReplication resource not found")
				} else {
					logger.Error(err, "failed to fetch dependent volumeReplication resource")
					return reconcile.Result{}, err
				}
			} else {
				err = r.Client.Delete(ctx, vrObj)
				if err != nil {
					logger.Error(err, "failed to delete dependent volumeReplication resource")
					return reconcile.Result{}, err
				}
			}
		}

		// If dependent VGRContent was created, delete it
		if instance.Spec.VolumeGroupReplicationContentName != "" {
			req := types.NamespacedName{Name: instance.Spec.VolumeGroupReplicationContentName}
			err = r.Client.Get(ctx, req, vgrContentObj)
			if err != nil {
				if errors.IsNotFound(err) {
					logger.Info("volumeGroupReplicationContent resource not found")
				} else {
					logger.Error(err, "failed to fetch dependent volumeGroupReplicationContent resource")
					return reconcile.Result{}, err
				}
			} else {
				err = r.Client.Delete(ctx, vgrContentObj)
				if err != nil {
					logger.Error(err, "failed to delete dependent volumeGroupReplicationContent resource")
					return reconcile.Result{}, err
				}
			}
		}

		// Just log error, and exit reconcile without error. The dependent resource will update the VGR
		// to remove their names from the CR, that will trigger a reconcile.
		if err = RemoveFinalizerFromVGR(r.Client, logger, instance); err != nil {
			logger.Error(err, "failed to remove VolumeGroupReplication finalizer")
		}

		return reconcile.Result{}, nil
	}

	// Update VGR status based on VR Status
	instance.Status.VolumeReplicationStatus = vrObj.Status
	err = r.Client.Status().Update(ctx, instance)
	if err != nil {
		logger.Error(err, "failed to update volumeGroupReplication instance's status", "VGRName", instance.Name)
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

	// Enqueue the VGR reconcile with the VGR name,namespace based on the annotation of the VR and VRContent CR
	enqueueVGRRequest := handler.EnqueueRequestsFromMapFunc(
		func(context context.Context, obj client.Object) []reconcile.Request {
			// Get the VolumeGroupReplication name,namespace
			var vgrName, vgrNamespace string
			objAnnotations := obj.GetAnnotations()
			for k, v := range objAnnotations {
				if k == volumeGroupReplicationRef {
					vgrName = strings.Split(v, "/")[0]
					vgrNamespace = strings.Split(v, "/")[1]
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
			err := r.Client.List(context, vgrObjsList)
			if err != nil {
				logger.Error(err, "failed to list VolumeGroupReplication instances")
				return []reconcile.Request{}
			}

			// Check if the pvc labels match any VGRs based on selectors present in it's annotation
			for _, vgr := range vgrObjsList.Items {
				if vgr.Annotations != nil && vgr.Annotations["pvcSelector"] != "" {
					labelSelector, err := labels.Parse(vgr.Annotations["pvcSelector"])
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
		For(&replicationv1alpha1.VolumeGroupReplication{}).
		Owns(&replicationv1alpha1.VolumeGroupReplicationContent{}, builder.WithPredicates(skipUpdates)).
		Owns(&replicationv1alpha1.VolumeReplication{}, builder.WithPredicates(skipUpdates)).
		Watches(&replicationv1alpha1.VolumeGroupReplicationContent{}, enqueueVGRRequest, builder.WithPredicates(watchOnlySpecUpdates)).
		Watches(&replicationv1alpha1.VolumeReplication{}, enqueueVGRRequest, builder.WithPredicates(watchOnlyStatusUpdates)).
		Watches(&corev1.PersistentVolumeClaim{}, enqueueVGRForPVCRequest, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
}

// waitForGroupCrds waits for dependent CRDs to the available in the cluster
func (r *VolumeGroupReplicationReconciler) waitForGroupCrds() error {
	logger := log.FromContext(context.TODO(), "Name", "checkingGroupDependencies")

	err := WaitForVolumeReplicationResource(r.Client, logger, volumeGroupReplicationClass)
	if err != nil {
		logger.Error(err, "failed to wait for VolumeGroupReplicationClass CRD")
		return err
	}

	err = WaitForVolumeReplicationResource(r.Client, logger, volumeGroupReplication)
	if err != nil {
		logger.Error(err, "failed to wait for VolumeGroupReplication CRD")
		return err
	}

	err = WaitForVolumeReplicationResource(r.Client, logger, volumeGroupReplicationContent)
	if err != nil {
		logger.Error(err, "failed to wait for VolumeGroupReplicationContent CRD")
		return err
	}

	return nil
}

// setGroupReplicationFailure sets the failure replication status on the VolumeGroupReplication resource
func (r *VolumeGroupReplicationReconciler) setGroupReplicationFailure(
	instance *replicationv1alpha1.VolumeGroupReplication,
	logger logr.Logger, err error) error {

	instance.Status.State = GetCurrentReplicationState(instance.Status.State)
	instance.Status.Message = replication.GetMessageFromError(err)
	instance.Status.ObservedGeneration = instance.Generation
	if err := r.Client.Status().Update(context.TODO(), instance); err != nil {
		logger.Error(err, "failed to update volumeGroupReplication status", "VGRName", instance.Name)
		return err
	}

	return nil
}

// getMatchingPVCsFromSource fecthes the PVCs based on the selectors defined in the VolumeGroupReplication resource
func (r *VolumeGroupReplicationReconciler) getMatchingPVCsFromSource(instance *replicationv1alpha1.VolumeGroupReplication,
	logger logr.Logger,
	vgrClass *replicationv1alpha1.VolumeGroupReplicationClass) (corev1.PersistentVolumeClaimList, []string, string, error) {

	pvcList := corev1.PersistentVolumeClaimList{}
	newSelector := labels.NewSelector()

	if instance.Spec.Source.Selector.MatchLabels != nil {
		for key, value := range instance.Spec.Source.Selector.MatchLabels {
			req, err := labels.NewRequirement(key, selection.Equals, []string{value})
			if err != nil {
				logger.Error(err, "failed to add label selector requirement")
				return pvcList, nil, "", err
			}
			newSelector = newSelector.Add(*req)
		}
	}

	if instance.Spec.Source.Selector.MatchExpressions != nil {
		for _, labelExp := range instance.Spec.Source.Selector.MatchExpressions {
			req, err := labels.NewRequirement(labelExp.Key, selection.Operator(labelExp.Operator), labelExp.Values)
			if err != nil {
				logger.Error(err, "failed to add label selector requirement")
				return pvcList, nil, "", err
			}
			newSelector = newSelector.Add(*req)
		}
	}
	opts := []client.ListOption{
		client.MatchingLabelsSelector{Selector: newSelector},
		client.InNamespace(instance.Namespace),
	}
	err := r.Client.List(context.TODO(), &pvcList, opts...)
	if err != nil {
		logger.Error(err, "failed to list pvcs with the given selectors")
		return pvcList, nil, "", err
	}

	pvHandlesList := []string{}
	for _, pvc := range pvcList.Items {
		pvName := pvc.Spec.VolumeName
		pv := &corev1.PersistentVolume{}
		err := r.Client.Get(context.TODO(), types.NamespacedName{Name: pvName}, pv)
		if err != nil {
			logger.Error(err, "failed to get pv for corresponding pvc", "PVC Name", pvc.Name)
			return pvcList, nil, "", err
		}
		if pv.Spec.CSI == nil {
			err = fmt.Errorf("pvc %s is not bound to a CSI PV", pvc.Name)
			return pvcList, nil, "", err
		}
		if pv.Spec.CSI.Driver != vgrClass.Spec.Provisioner {
			err = fmt.Errorf("driver of PV for PVC %s is different than the VolumeGroupReplicationClass driver", pvc.Name)
			return pvcList, nil, "", err
		}
		pvHandlesList = append(pvHandlesList, pv.Spec.CSI.VolumeHandle)
	}

	return pvcList, pvHandlesList, newSelector.String(), nil
}

func (r *VolumeGroupReplicationReconciler) createVolumeGroupReplicationContentCR(vgr *replicationv1alpha1.VolumeGroupReplication,
	vgrContentObj *replicationv1alpha1.VolumeGroupReplicationContent, vgrClass string, pvHandlesList []string) error {
	_, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, vgrContentObj, func() error {
		if vgrContentObj.CreationTimestamp.IsZero() {
			vgrContentObj.Annotations = map[string]string{
				volumeGroupReplicationRef: fmt.Sprintf("%s/%s", vgr.Name, vgr.Namespace),
			}
			vgrContentObj.Spec = replicationv1alpha1.VolumeGroupReplicationContentSpec{
				VolumeGroupReplicationRef: corev1.ObjectReference{
					APIVersion: vgr.APIVersion,
					Kind:       vgr.Kind,
					Name:       vgr.Name,
					Namespace:  vgr.Namespace,
					UID:        vgr.UID,
				},
				Provisioner:                     vgrClass,
				VolumeGroupReplicationClassName: vgr.Spec.VolumeGroupReplicationClassName,
			}
		}

		vgrContentObj.Spec.Source = replicationv1alpha1.VolumeGroupReplicationContentSource{
			VolumeHandles: pvHandlesList,
		}

		return nil
	})

	return err
}

func (r *VolumeGroupReplicationReconciler) createVolumeReplicationCR(vgr *replicationv1alpha1.VolumeGroupReplication,
	vrObj *replicationv1alpha1.VolumeReplication) error {
	apiGroup := "replication.storage.openshift.io"
	_, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, vrObj, func() error {
		if vrObj.CreationTimestamp.IsZero() {
			vrObj.Annotations = map[string]string{
				volumeGroupReplicationRef: fmt.Sprintf("%s/%s", vgr.Name, vgr.Namespace),
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
