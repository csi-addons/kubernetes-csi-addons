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
	"slices"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/csiaddons/v1alpha1"
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

	if nwFence.Spec.NetworkFenceClassName != "" {
		if nwFence.Spec.Cidrs == nil {
			return errors.New("required parameter spec.cidrs is not specified")
		}

		// Driver name and secrets will be read (and validated) in NetworkFenceClass
		return nil
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

	// check if the networkfence object is getting deleted and handle it.
	if !nwFence.GetDeletionTimestamp().IsZero() {
		if slices.Contains(nwFence.GetFinalizers(), networkFenceFinalizer) {
			logger.Info("removing finalizer from NetworkFence object", "Finalizer", networkFenceFinalizer)

			nwFence.Finalizers = util.RemoveFromSlice(nwFence.Finalizers, networkFenceFinalizer)
			if err := r.Update(ctx, nwFence); err != nil {
				logger.Error(err, "failed to remove finalizer on NetworkFence resource")
				return ctrl.Result{}, fmt.Errorf("failed to remove finalizer (%s) from NetworkFence resource %s: %w",
					networkFenceFinalizer, nwFence.Name, err)
			}
		}

		logger.Info("NetworkFence object is terminated, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// validate NetworkFence object so as its parameters are neither empty nor nil.
	err = validateNetworkFenceSpec(nwFence)
	if err != nil {
		logger.Error(err, "failed to validate NetworkFence spec")

		nwFence.Status.Result = csiaddonsv1alpha1.FencingOperationResultFailed
		nwFence.Status.Message = fmt.Sprintf("Failed to validate Networkfence parameters: %v", util.GetErrorMessage(err))
		statusErr := r.Status().Update(ctx, nwFence)
		if statusErr != nil {
			logger.Error(statusErr, "Failed to update networkfence status")

			return ctrl.Result{}, statusErr
		}

		// invalid parameter, do not requeue
		return ctrl.Result{}, nil
	}

	nf, err := r.getNetworkFenceInstance(ctx, logger, nwFence)
	if err != nil {
		logger.Error(err, "failed to get the networkfenceinstance")

		return ctrl.Result{}, err
	}

	if nwFence.Spec.FenceState == csiaddonsv1alpha1.Fenced {
		nf.logger.Info("FenceClusterNetwork Request", "namespaced name", req.String())
	} else {
		nf.logger.Info("UnFenceClusterNetwork Request", "namespaced name", req.String())
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

	var successMsg string
	switch nf.instance.Spec.FenceState {
	case csiaddonsv1alpha1.Fenced:
		successMsg = csiaddonsv1alpha1.FenceOperationSuccessfulMessage
	case csiaddonsv1alpha1.Unfenced:
		successMsg = csiaddonsv1alpha1.UnFenceOperationSuccessfulMessage
	}

	err = nf.updateStatus(ctx, csiaddonsv1alpha1.FencingOperationResultSucceeded, successMsg)
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
	nfClass          *csiaddonsv1alpha1.NetworkFenceClass
}

func (nf *NetworkFenceInstance) updateStatus(ctx context.Context,
	result csiaddonsv1alpha1.FencingOperationResult, message string) error {
	nf.instance.Status.Result = result
	nf.instance.Status.Message = message
	if err := nf.reconciler.Status().Update(ctx, nf.instance); err != nil {
		nf.logger.Error(err, "failed to update status")

		return err
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NetworkFenceReconciler) SetupWithManager(mgr ctrl.Manager, ctrlOptions controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&csiaddonsv1alpha1.NetworkFence{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		WithOptions(ctrlOptions).
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

	if nf.nfClass != nil {
		nfParams := nf.nfClass.Spec.Parameters

		request.SecretName = nfParams[prefixedNetworkFenceSecretNameKey]
		request.SecretNamespace = nfParams[prefixedNetworkFenceSecretNamespaceKey]

		if request.Parameters == nil {
			request.Parameters = make(map[string]string)
		}

		for k, v := range nfParams {
			if k == prefixedNetworkFenceSecretNameKey ||
				k == prefixedNetworkFenceSecretNamespaceKey {
				continue
			}
			request.Parameters[k] = v
		}
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
	if !slices.Contains(nf.instance.Finalizers, networkFenceFinalizer) {
		nf.logger.Info("adding finalizer to NetworkFence object", "Finalizer", networkFenceFinalizer)

		nf.instance.Finalizers = append(nf.instance.Finalizers, networkFenceFinalizer)
		if err := nf.reconciler.Update(ctx, nf.instance); err != nil {
			return fmt.Errorf("failed to add finalizer (%s) to NetworkFence resource"+
				" (%s): %w", networkFenceFinalizer, nf.instance.GetName(), err)
		}
	}

	return nil
}

// getNetworkFenceClient returns a NetworkFenceClient that is the leader for
// the given driver.
// The NetworkFenceClient should only run on a CONTROLLER_SERVICE capable
// CSI-Addons plugin, there can only be one plugin that holds the lease.
func (r *NetworkFenceReconciler) getNetworkFenceClient(ctx context.Context, drivername string) (proto.NetworkFenceClient, error) {
	conn, err := r.Connpool.GetLeaderByDriver(ctx, r.Client, drivername)
	if err != nil {
		return nil, err
	}

	// verify that the CSI-Addons plugin holding the lease supports
	// NetworkFence, it probably is a bug if it doesn't
	for _, capability := range conn.Capabilities {
		// validate if NETWORK_FENCE capability is supported by the driver.
		if capability.GetNetworkFence() == nil {
			continue
		}

		// validate of NETWORK_FENCE capability is enabled by the storage driver.
		if capability.GetNetworkFence().GetType() == identity.Capability_NetworkFence_NETWORK_FENCE {
			return proto.NewNetworkFenceClient(conn.Client), nil
		}
	}

	return nil, fmt.Errorf("leading CSIAddonsNode %q for driver %q does not support NetworkFence", conn.Name, drivername)
}

// getNetworkFenceInstance returns a new NetworkFenceInstance object
// by setting its logger and controller client. If NetworkFenceClassName is
// present, it uses the values from NetworkFenceClass else it uses the
// spec of the NetworkFence object.
func (r *NetworkFenceReconciler) getNetworkFenceInstance(
	ctx context.Context,
	logger logr.Logger,
	nf *csiaddonsv1alpha1.NetworkFence,
) (*NetworkFenceInstance, error) {
	nfInstance := &NetworkFenceInstance{
		reconciler: r,
		instance:   nf,
	}

	var driverName string
	var err error

	// If NetworkFenceClassName is empty, use the driver from NetworkFence spec
	// and log a warning for the same.
	if nf.Spec.NetworkFenceClassName == "" {
		logger.Info("WARNING: Specifying driver, secrets and parameters inside NetworkFence is deprecated, please use NetworkFenceClass instead")

		driverName = nf.Spec.Driver
	} else {
		// We need to fetch the driverName from the NetworkFenceClass
		nfc := &csiaddonsv1alpha1.NetworkFenceClass{}
		if err = r.Get(ctx, client.ObjectKey{Name: nf.Spec.NetworkFenceClassName}, nfc); err != nil {
			return nil, fmt.Errorf("failed to get networkfenceclass with name %q due to error: %w", nf.Spec.NetworkFenceClassName, err)
		}

		nfInstance.nfClass = nfc
		driverName = nfc.Spec.Provisioner
	}

	// Set the logger and client
	nfInstance.logger = logger.WithValues("DriverName", driverName, "CIDRs", nf.Spec.Cidrs)
	nfInstance.controllerClient, err = r.getNetworkFenceClient(ctx, driverName)
	if err != nil {
		return nil, fmt.Errorf("failed to get networkfenceclient using driver %q due to error: %w", driverName, err)
	}

	return nfInstance, nil
}
