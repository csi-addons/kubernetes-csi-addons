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
	"slices"
	"time"

	replicationv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/replication.storage/v1alpha1"
	grpcClient "github.com/csi-addons/kubernetes-csi-addons/internal/client"
	conn "github.com/csi-addons/kubernetes-csi-addons/internal/connection"
	"github.com/go-logr/logr"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/csi-addons/spec/lib/go/identity"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// VolumeGroupReplicationContentReconciler reconciles a VolumeGroupReplicationContent object
type VolumeGroupReplicationContentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// ConnectionPool consists of map of Connection objects
	Connpool *conn.ConnectionPool
	// Timeout for the Reconcile operation.
	Timeout time.Duration
}

//+kubebuilder:rbac:groups=replication.storage.openshift.io,resources=volumegroupreplicationcontents,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=replication.storage.openshift.io,resources=volumegroupreplicationcontents/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=replication.storage.openshift.io,resources=volumegroupreplicationcontents/finalizers,verbs=update
//+kubebuilder:rbac:groups=replication.storage.openshift.io,resources=volumegroupreplicationclasses,verbs=get;list;watch

/*
Steps performed by the reconcile loop:
- Watch for VGRContent CR
- Fetch the VGRClass using the name from the VGRContent CR, and extract the secrets from it
- Add VGRContent owner annotation to the VGR resource
- Add finalizer to the VGRContent resource
- Create/Modify the volume group based on the handle field
- Update the group handle in VGRContent CR
- Update the VGRContent status with the PV list
*/

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *VolumeGroupReplicationContentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx, "Request.Name", req.Name, "Request.Namespace", req.Namespace)

	// Fetch VolumeGroupReplicationContent instance
	instance := &replicationv1alpha1.VolumeGroupReplicationContent{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("volumeGroupReplicationContent resource not found")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	volumeGroupClient, err := r.getVolumeGroupClient(ctx, instance.Spec.Provisioner)
	if err != nil {
		logger.Error(err, "Failed to get VolumeGroupClient")
		return reconcile.Result{}, err
	}

	// Fetch VolumeGroupReplicationClass
	vgrClass := &replicationv1alpha1.VolumeGroupReplicationClass{}
	err = r.Get(ctx, types.NamespacedName{Name: instance.Spec.VolumeGroupReplicationClassName}, vgrClass)
	if err != nil {
		logger.Error(err, "failed to fetch volumeGroupReplicationClass resource")
		return reconcile.Result{}, err
	}

	// Get secrets and parameters
	parameters := filterPrefixedParameters(replicationParameterPrefix, vgrClass.Spec.Parameters)
	secretName := vgrClass.Spec.Parameters[prefixedGroupReplicationSecretNameKey]
	secretNamespace := vgrClass.Spec.Parameters[prefixedGroupReplicationSecretNamespaceKey]

	// Get the VolumeGroupReplication resource
	if instance.Spec.VolumeGroupReplicationRef == nil {
		logger.Info("waiting for owner VolumeGroupReplication to update the owner ref")
		return reconcile.Result{}, nil
	}
	vgrObj := &replicationv1alpha1.VolumeGroupReplication{}
	namespacedObj := types.NamespacedName{Namespace: instance.Spec.VolumeGroupReplicationRef.Namespace,
		Name: instance.Spec.VolumeGroupReplicationRef.Name}
	err = r.Get(ctx, namespacedObj, vgrObj)
	if err != nil {
		logger.Error(err, "failed to get owner VolumeGroupReplication")
		return reconcile.Result{}, err
	}

	// store group id for easy usage later
	groupID := instance.Spec.VolumeGroupReplicationHandle

	// Check if object is being deleted
	if instance.GetDeletionTimestamp().IsZero() {
		// add vgr as the owner annotation for vgrContent
		err = r.annotateVolumeGroupReplicationWithVGRContentOwner(ctx, logger, req.Name, vgrObj)
		if err != nil {
			logger.Error(err, "Failed to annotate VolumeGroupReplication owner")
			return ctrl.Result{}, err
		}
		// add vgr finalizer to vgrContent
		if err = addFinalizerToVGRContent(r.Client, logger, instance, vgrReplicationFinalizer); err != nil {
			logger.Error(err, "failed to add VolumeGroupReplicationContent finalizer")
			return reconcile.Result{}, err
		}
	} else {
		// Check if the owner VGR is marked for deletion, only then proceed with deletion of VGRContent resource
		if vgrObj.GetDeletionTimestamp().IsZero() {
			logger.Info("cannot delete VolumeGroupReplicationContent resource, until owner VolumeGroupReplication instance is deleted")
			return reconcile.Result{}, nil
		} else if !slices.Contains(instance.Finalizers, volumeReplicationFinalizer) {
			// Delete the volume group, if exists
			if groupID != "" {
				// Empty the volume group before deleting the volume group, if not empty
				if len(instance.Spec.Source.VolumeHandles) != 0 {
					_, err := volumeGroupClient.ModifyVolumeGroupMembership(groupID, []string{}, secretName, secretNamespace, parameters)
					if err != nil {
						if status.Code(err) != codes.NotFound {
							logger.Error(err, "failed to get the volume group to be modified", "Volume Group", groupID)
							return reconcile.Result{}, err
						}
						logger.Info("failed to get volume group to be modified, the volume group has already been deleted or doesn't exists")
					}
				}
				_, err := volumeGroupClient.DeleteVolumeGroup(groupID, secretName, secretNamespace)
				if err != nil {
					logger.Error(err, "failed to delete volume group", "Volume Group", groupID)
					return reconcile.Result{}, err
				}
			}

			// Remove the vgrcontent owner annotation from the VGR resource
			if err = r.removeVGRContentOwnerFromVGRAnnotation(ctx, logger, vgrObj); err != nil {
				logger.Error(err, "Failed to remove VolumeReplication annotation from VolumeGroupReplication")
				return reconcile.Result{}, err
			}

			// Remove the vgr finalizer from the vgrcontent resource
			if err = removeFinalizerFromVGRContent(r.Client, logger, instance, vgrReplicationFinalizer); err != nil {
				logger.Error(err, "failed to remove finalizer from VolumeGroupReplicationContent resource")
				return reconcile.Result{}, err
			}

			logger.Info("volumeGroupReplicationContent object is terminated, skipping reconciliation")
			return reconcile.Result{}, nil
		} else {
			err = fmt.Errorf("cannot delete VolumeGroupReplicationContent resource, until dependent VolumeReplication instance is deleted")
			logger.Error(err, "failed to delete VolumeGroupReplicationContent resource")
			return reconcile.Result{}, err
		}
	}

	// Create/Update volume group
	if groupID == "" {
		groupName := instance.Name
		resp, err := volumeGroupClient.CreateVolumeGroup(groupName, instance.Spec.Source.VolumeHandles, secretName, secretNamespace, parameters)
		if err != nil {
			logger.Error(err, "failed to group volumes")
			return reconcile.Result{}, err
		}

		// Update the group handle in the VolumeGroupReplicationContent CR
		instance.Spec.VolumeGroupReplicationHandle = resp.GetVolumeGroup().VolumeGroupId
		err = r.Update(ctx, instance)
		if err != nil {
			logger.Error(err, "failed to update group id in VGRContent")
			return reconcile.Result{}, err
		}
	} else {
		_, err := volumeGroupClient.ModifyVolumeGroupMembership(groupID, instance.Spec.Source.VolumeHandles, secretName, secretNamespace, parameters)
		if err != nil {
			logger.Error(err, "failed to modify volume group")
			return reconcile.Result{}, err
		}
	}

	// Update VGRContent resource status
	pvList := &corev1.PersistentVolumeList{}
	err = r.List(ctx, pvList)
	if err != nil {
		logger.Error(err, "failed to list PVs")
		return reconcile.Result{}, err
	}

	pvRefList := []corev1.LocalObjectReference{}
	for _, pv := range pvList.Items {
		if slices.ContainsFunc(instance.Spec.Source.VolumeHandles, func(handle string) bool {
			return pv.Spec.CSI != nil && pv.Spec.CSI.VolumeHandle == handle
		}) {
			pvRefList = append(pvRefList, corev1.LocalObjectReference{
				Name: pv.Name,
			})
		}
	}
	instance.Status.PersistentVolumeRefList = pvRefList
	err = r.Status().Update(ctx, instance)
	if err != nil {
		logger.Error(err, "failed to update VGRContent status")
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *VolumeGroupReplicationContentReconciler) SetupWithManager(mgr ctrl.Manager) error {

	return ctrl.NewControllerManagedBy(mgr).
		For(&replicationv1alpha1.VolumeGroupReplicationContent{}).
		Complete(r)
}

func (r *VolumeGroupReplicationContentReconciler) getVolumeGroupClient(ctx context.Context, driverName string) (grpcClient.VolumeGroup, error) {
	conn, err := r.Connpool.GetLeaderByDriver(ctx, r.Client, driverName)
	if err != nil {
		return nil, fmt.Errorf("no leader for the ControllerService of driver %q", driverName)
	}

	for _, cap := range conn.Capabilities {
		// validate if VOLUME_GROUP capability is supported by the driver.
		if cap.GetVolumeGroup() == nil {
			continue
		}

		// validate of VOLUME_GROUP capability is enabled by the storage driver.
		if cap.GetVolumeGroup().GetType() == identity.Capability_VolumeGroup_VOLUME_GROUP {
			return grpcClient.NewVolumeGroupClient(conn.Client, r.Timeout), nil
		}
	}

	return nil, fmt.Errorf("leading CSIAddonsNode %q for driver %q does not support VolumeGroup", conn.Name, driverName)

}

// annotateVolumeGroupReplicationWithVGRContentOwner will add the VolumeGroupReplicationContent owner to the VGR annotations.
func (r *VolumeGroupReplicationContentReconciler) annotateVolumeGroupReplicationWithVGRContentOwner(ctx context.Context, logger logr.Logger, reqOwnerName string, vgr *replicationv1alpha1.VolumeGroupReplication) error {
	if vgr.Annotations == nil {
		vgr.Annotations = map[string]string{}
	}

	currentOwnerName := vgr.Annotations[replicationv1alpha1.VolumeGroupReplicationContentNameAnnotation]
	if currentOwnerName == "" {
		logger.Info("setting vgrcontent owner on VGR annotation", "Name", vgr.Name, "owner", reqOwnerName)
		vgr.Annotations[replicationv1alpha1.VolumeGroupReplicationContentNameAnnotation] = reqOwnerName
		err := r.Update(ctx, vgr)
		if err != nil {
			logger.Error(err, "Failed to update VGR annotation", "Name", vgr.Name)

			return fmt.Errorf("failed to update VGR %q annotation for VolumeGroupReplicationContent: %w",
				vgr.Name, err)
		}

		return nil
	}

	if currentOwnerName != reqOwnerName {
		logger.Info("cannot change the owner of vgr",
			"VGR name", vgr.Name,
			"current owner", currentOwnerName,
			"requested owner", reqOwnerName)

		return fmt.Errorf("VGRContent %q not owned by correct VolumeGroupReplication %q",
			reqOwnerName, vgr.Name)
	}

	return nil
}

// removeVGRContentOwnerFromVGRAnnotation removes the VolumeGroupReplicationContent owner from the VGR annotations.
func (r *VolumeGroupReplicationContentReconciler) removeVGRContentOwnerFromVGRAnnotation(ctx context.Context, logger logr.Logger, vgr *replicationv1alpha1.VolumeGroupReplication) error {
	if _, ok := vgr.Annotations[replicationv1alpha1.VolumeGroupReplicationContentNameAnnotation]; ok {
		logger.Info("removing vgrcontent owner annotation from VolumeGroupReplication object", "Annotation", replicationv1alpha1.VolumeGroupReplicationContentNameAnnotation)
		delete(vgr.Annotations, replicationv1alpha1.VolumeGroupReplicationContentNameAnnotation)
		if err := r.Update(ctx, vgr); err != nil {
			return fmt.Errorf("failed to remove annotation %q from VolumeGroupReplication "+
				"%q %w",
				replicationv1alpha1.VolumeGroupReplicationContentNameAnnotation, vgr.Name, err)
		}
	}

	return nil
}
