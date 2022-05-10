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
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/v1alpha1"
	conn "github.com/csi-addons/kubernetes-csi-addons/internal/connection"
	"github.com/csi-addons/kubernetes-csi-addons/internal/proto"
	"github.com/csi-addons/kubernetes-csi-addons/internal/util"
	"github.com/csi-addons/spec/lib/go/identity"
)

// NetworkFenceReconciler reconciles a NetworkFence object.
type NetworkFenceReconciler struct {
	client.Client
	// Scheme defines methods for serializing and deserializing API objects.
	Scheme *runtime.Scheme
	// ConnectionPool consists of map of Connection objects
	Connpool *conn.ConnectionPool
	// Timeout for the Reconcile operation.
	Timeout time.Duration
}

const (
	networkFenceFinalizer = "csiaddons.openshift.io/network-fence"
)

// validateNetworkFenceSpec validates the NetworkFence spec and checks if values are neither nil nor empty.
func validateNetworkFenceSpec(nwFence *csiaddonsv1alpha1.NetworkFence) error {
	if nwFence == nil {
		return errors.New("NetworkFence resource is empty")
	}
	if nwFence.Spec.Driver == "" {
		return errors.New("required parameter driver is not specified")
	}
	if nwFence.Spec.Cidrs == nil {
		return errors.New("required parameter cidrs is not specified")
	}

	return nil
}

//+kubebuilder:rbac:groups=csiaddons.openshift.io,resources=networkfences,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=csiaddons.openshift.io,resources=networkfences/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=csiaddons.openshift.io,resources=networkfences/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *NetworkFenceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// fetch NetworkFence object instance
	nwFence := &csiaddonsv1alpha1.NetworkFence{}
	err := r.Get(ctx, req.NamespacedName, nwFence)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			logger.Info("NetworkFence resource not found or deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// validate NetworkFence object so as its parameters are neither empty nor nil.
	err = validateNetworkFenceSpec(nwFence)
	if err != nil {
		logger.Error(err, "failed to validate NetworkFence spec")

		nwFence.Status.Result = csiaddonsv1alpha1.FencingOperationResultFailed
		nwFence.Status.Message = fmt.Sprintf("Failed to validate Networkfence parameters: %v", util.GetErrorMessage(err))
		statusErr := r.Client.Status().Update(ctx, nwFence)
		if statusErr != nil {
			logger.Error(statusErr, "Failed to update networkfence status")

			return ctrl.Result{}, statusErr
		}

		// invalid parameter, do not requeue
		return ctrl.Result{}, nil
	}

	logger = logger.WithValues("DriverName", nwFence.Spec.Driver, "CIDRs", nwFence.Spec.Cidrs)

	client, err := r.getNetworkFenceClient(nwFence.Spec.Driver, "")
	if err != nil {
		logger.Error(err, "Failed to get NetworkFenceClient")
		return ctrl.Result{}, err
	}

	nf := NetworkFenceInstance{
		reconciler:       r,
		logger:           logger,
		instance:         nwFence,
		controllerClient: client,
	}

	// check if the networkfence object is getting deleted and handle it.
	if !nf.instance.GetDeletionTimestamp().IsZero() {
		if util.ContainsInSlice(nwFence.GetFinalizers(), networkFenceFinalizer) {

			err := nf.removeFinalizerFromNetworkFence(ctx)
			if err != nil {
				logger.Error(err, "failed to remove finalizer on NetworkFence resource")
				return ctrl.Result{}, err
			}
		}

		logger.Info("NetworkFence object is terminated, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	if nwFence.Spec.FenceState == csiaddonsv1alpha1.Fenced {
		nf.logger.Info("FenceClusterNetwork Request", "namespaced name", req.NamespacedName.String())
	} else {
		nf.logger.Info("UnFenceClusterNetwork Request", "namespaced name", req.NamespacedName.String())
	}

	err = nf.processFencing(ctx)
	if err != nil {
		logger.Error(err, "failed to fence cluster network")
		updateStatusErr := nf.updateStatus(ctx, csiaddonsv1alpha1.FencingOperationResultFailed, err.Error())
		if updateStatusErr != nil {
			logger.Error(updateStatusErr, "failed to update status")
		}

		return ctrl.Result{}, err
	}

	err = nf.updateStatus(ctx, csiaddonsv1alpha1.FencingOperationResultSucceeded, "fencing operation successful")
	if err != nil {
		logger.Error(err, "failed to update status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// NetworkFenceInstance contains the attributes
// that can be useful in reconciling a particular
// instance of the NetworkFence resource.
type NetworkFenceInstance struct {
	reconciler       *NetworkFenceReconciler
	controllerClient proto.NetworkFenceClient
	logger           logr.Logger
	instance         *csiaddonsv1alpha1.NetworkFence
}

func (nf *NetworkFenceInstance) updateStatus(ctx context.Context,
	result csiaddonsv1alpha1.FencingOperationResult, message string) error {
	nf.instance.Status.Result = result
	nf.instance.Status.Message = message
	if err := nf.reconciler.Client.Status().Update(ctx, nf.instance); err != nil {
		nf.logger.Error(err, "failed to update status")

		return err
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NetworkFenceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&csiaddonsv1alpha1.NetworkFence{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}

// processFencing adds a finalizer and handles the fencing request.
func (nf *NetworkFenceInstance) processFencing(ctx context.Context) error {

	// add finalizer to the networkfence object if not already present.
	if err := nf.addFinalizerToNetworkFence(ctx); err != nil {
		nf.logger.Error(err, "Failed to add NetworkFence finalizer")
		return err
	}

	return nf.processFencingRequest(ctx)
}

// processFencingRequest creates the fencing request based on
// the spec and then calls appropriate function to either
// fence or unfence based on the spec.
func (nf *NetworkFenceInstance) processFencingRequest(ctx context.Context) error {
	// send FenceClusterNetwork request.
	request := &proto.NetworkFenceRequest{
		Parameters:      nf.instance.Spec.Parameters,
		SecretName:      nf.instance.Spec.Secret.Name,
		SecretNamespace: nf.instance.Spec.Secret.Namespace,
		Cidrs:           nf.instance.Spec.Cidrs,
	}

	if nf.instance.Spec.FenceState == csiaddonsv1alpha1.Fenced {
		return nf.fenceClusterNetwork(ctx, request)
	}

	return nf.unfenceClusterNetwork(ctx, request)
}

// fenceClusterNetwork sends the fencing request
func (nf *NetworkFenceInstance) fenceClusterNetwork(ctx context.Context, request *proto.NetworkFenceRequest) error {
	timeoutContext, cancel := context.WithTimeout(ctx, nf.reconciler.Timeout)
	defer cancel()

	_, err := nf.controllerClient.FenceClusterNetwork(timeoutContext, request)

	if err != nil {
		nf.logger.Error(err, "failed to fence cluster network")
		return err
	}

	nf.logger.Info("FenceClusterNetwork Request Succeeded")

	return nil
}

// unfenceClusterNetwork sends the unfencing request
func (nf *NetworkFenceInstance) unfenceClusterNetwork(ctx context.Context, request *proto.NetworkFenceRequest) error {
	timeoutContext, cancel := context.WithTimeout(ctx, nf.reconciler.Timeout)
	defer cancel()

	_, err := nf.controllerClient.UnFenceClusterNetwork(timeoutContext, request)

	if err != nil {
		nf.logger.Error(err, "failed to unfence cluster network")
		return err
	}

	nf.logger.Info("UnFenceClusterNetwork Request Succeeded")

	return nil
}

// addFinalizerToNetworkFence adds a finalizer to the Networkfence instance.
func (nf *NetworkFenceInstance) addFinalizerToNetworkFence(ctx context.Context) error {
	if !util.ContainsInSlice(nf.instance.Finalizers, networkFenceFinalizer) {
		nf.logger.Info("adding finalizer to NetworkFence object", "Finalizer", networkFenceFinalizer)

		nf.instance.Finalizers = append(nf.instance.Finalizers, networkFenceFinalizer)
		if err := nf.reconciler.Client.Update(ctx, nf.instance); err != nil {
			return fmt.Errorf("failed to add finalizer (%s) to NetworkFence resource"+
				" (%s): %w", networkFenceFinalizer, nf.instance.GetName(), err)
		}
	}

	return nil
}

// removeFinalizerFromNetworkFence removes the finalizer from the Networkfence instance.
func (nf *NetworkFenceInstance) removeFinalizerFromNetworkFence(ctx context.Context) error {
	if util.ContainsInSlice(nf.instance.Finalizers, networkFenceFinalizer) {
		nf.logger.Info("removing finalizer from NetworkFence object", "Finalizer", networkFenceFinalizer)

		nf.instance.Finalizers = util.RemoveFromSlice(nf.instance.Finalizers, networkFenceFinalizer)
		if err := nf.reconciler.Client.Update(ctx, nf.instance); err != nil {
			return fmt.Errorf("failed to remove finalizer (%s) from NetworkFence resource"+
				" %s: %w", networkFenceFinalizer, nf.instance.Name, err)
		}
	}

	return nil
}

// getNetworkFenceClient returns a NetworkFenceClient for the given driver.
func (r *NetworkFenceReconciler) getNetworkFenceClient(drivername, nodeID string) (proto.NetworkFenceClient, error) {
	conns := r.Connpool.GetByNodeID(drivername, nodeID)

	// Iterate through the connections and find the one that matches the driver name
	// provided in the NetworkFence spec; so that corresponding network fence and
	// unfence operations can be performed.
	for _, v := range conns {
		for _, cap := range v.Capabilities {
			// validate if NETWORK_FENCE capability is supported by the driver.
			if cap.GetNetworkFence() == nil {
				continue
			}

			// validate of NETWORK_FENCE capability is enabled by the storage driver.
			if cap.GetNetworkFence().GetType() == identity.Capability_NetworkFence_NETWORK_FENCE {
				return proto.NewNetworkFenceClient(v.Client), nil
			}
		}
	}

	return nil, fmt.Errorf("no connections for driver: %s", drivername)
}
