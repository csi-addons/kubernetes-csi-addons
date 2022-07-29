/*
Copyright 2021 The Kubernetes-CSI-Addons Authors.

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
	"errors"
	"fmt"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/apis/csiaddons/v1alpha1"
	"github.com/csi-addons/kubernetes-csi-addons/internal/connection"
	"github.com/csi-addons/kubernetes-csi-addons/internal/util"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var (
	csiAddonsNodeFinalizer = csiaddonsv1alpha1.GroupVersion.Group + "/csiaddonsnode"
)

// CSIAddonsNodeReconciler reconciles a CSIAddonsNode object
type CSIAddonsNodeReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	ConnPool *connection.ConnectionPool
}

//+kubebuilder:rbac:groups=csiaddons.openshift.io,resources=csiaddonsnodes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=csiaddons.openshift.io,resources=csiaddonsnodes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=csiaddons.openshift.io,resources=csiaddonsnodes/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *CSIAddonsNodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch CSIAddonsNode instance
	csiAddonsNode := &csiaddonsv1alpha1.CSIAddonsNode{}
	err := r.Client.Get(ctx, req.NamespacedName, csiAddonsNode)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			logger.Info("CSIAddonsNode resource not found")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	err = validateCSIAddonsNodeSpec(csiAddonsNode)
	if err != nil {
		logger.Error(err, "Failed to validate CSIAddonsNode parameters")

		csiAddonsNode.Status.State = csiaddonsv1alpha1.CSIAddonsNodeStateFailed
		csiAddonsNode.Status.Message = fmt.Sprintf("Failed to validate CSIAddonsNode parameters: %v", err)
		statusErr := r.Client.Status().Update(ctx, csiAddonsNode)
		if statusErr != nil {
			logger.Error(statusErr, "Failed to update status")

			return ctrl.Result{}, statusErr
		}

		// invalid parameter, do not requeue
		return ctrl.Result{}, nil
	}

	nodeID := csiAddonsNode.Spec.Driver.NodeID
	driverName := csiAddonsNode.Spec.Driver.Name
	endPoint := csiAddonsNode.Spec.Driver.EndPoint
	key := csiAddonsNode.Namespace + "/" + csiAddonsNode.Name
	logger = logger.WithValues("NodeID", nodeID, "EndPoint", endPoint, "DriverName", driverName)

	if !csiAddonsNode.DeletionTimestamp.IsZero() {
		// if deletion timestap is set, the CSIAddonsNode is getting deleted,
		// delete connections and remove finalizer.
		logger.Info("Deleting connection")
		r.ConnPool.Delete(key)
		err = r.removeFinalizer(ctx, &logger, csiAddonsNode)
		return ctrl.Result{}, err
	}

	if err := r.addFinalizer(ctx, &logger, csiAddonsNode); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Connecting to sidecar")
	newConn, err := connection.NewConnection(ctx, endPoint, nodeID, driverName)
	if err != nil {
		logger.Error(err, "Failed to establish connection with sidecar")

		errMessage := util.GetErrorMessage(err)
		csiAddonsNode.Status.State = csiaddonsv1alpha1.CSIAddonsNodeStateFailed
		csiAddonsNode.Status.Message = fmt.Sprintf("Failed to establish connection with sidecar: %v", errMessage)
		statusErr := r.Client.Status().Update(ctx, csiAddonsNode)
		if statusErr != nil {
			logger.Error(statusErr, "Failed to update status")

			return ctrl.Result{}, statusErr
		}

		return ctrl.Result{}, err
	}

	logger.Info("Successfully connected to sidecar")
	r.ConnPool.Put(key, newConn)
	logger.Info("Added connection to connection pool")

	csiAddonsNode.Status.State = csiaddonsv1alpha1.CSIAddonsNodeStateConnected
	csiAddonsNode.Status.Message = "Successfully established connection with sidecar"
	err = r.Client.Status().Update(ctx, csiAddonsNode)
	if err != nil {
		logger.Error(err, "Failed to update status")

		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CSIAddonsNodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&csiaddonsv1alpha1.CSIAddonsNode{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}

// addFinalizer adds finalizer to csiAddonsNode if it is not present.
func (r *CSIAddonsNodeReconciler) addFinalizer(
	ctx context.Context,
	logger *logr.Logger,
	csiAddonsNode *csiaddonsv1alpha1.CSIAddonsNode) error {

	if !util.ContainsInSlice(csiAddonsNode.Finalizers, csiAddonsNodeFinalizer) {
		logger.Info("Adding finalizer")

		csiAddonsNode.Finalizers = append(csiAddonsNode.Finalizers, csiAddonsNodeFinalizer)
		if err := r.Client.Update(ctx, csiAddonsNode); err != nil {
			logger.Error(err, "Failed to add finalizer")

			return err
		}
	}

	return nil
}

// removeFinalizer removes finalizer from csiAddonsNode if it is present.
func (r *CSIAddonsNodeReconciler) removeFinalizer(
	ctx context.Context,
	logger *logr.Logger,
	csiAddonsNode *csiaddonsv1alpha1.CSIAddonsNode) error {

	if util.ContainsInSlice(csiAddonsNode.Finalizers, csiAddonsNodeFinalizer) {
		logger.Info("Removing finalizer")

		csiAddonsNode.Finalizers = util.RemoveFromSlice(csiAddonsNode.Finalizers, csiAddonsNodeFinalizer)
		if err := r.Client.Update(ctx, csiAddonsNode); err != nil {
			logger.Error(err, "Failed to remove finalizer")

			return err
		}
	}

	return nil
}

// validateCSIAddonsNodeSpec validates if Name and Endpoint are not empty.
func validateCSIAddonsNodeSpec(csiaddonsnode *csiaddonsv1alpha1.CSIAddonsNode) error {
	if csiaddonsnode.Spec.Driver.Name == "" {
		return errors.New("required parameter 'Name' in CSIAddonsNode.Spec.Driver is empty")
	}
	if csiaddonsnode.Spec.Driver.EndPoint == "" {
		return errors.New("required parameter 'EndPoint' in CSIAddonsNode.Spec.Driver is empty")
	}

	return nil
}
