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
	"net/url"
	"strings"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/apis/csiaddons/v1alpha1"
	"github.com/csi-addons/kubernetes-csi-addons/internal/connection"
	"github.com/csi-addons/kubernetes-csi-addons/internal/util"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var (
	csiAddonsNodeFinalizer = csiaddonsv1alpha1.GroupVersion.Group + "/csiaddonsnode"

	errLegacyEndpoint = errors.New("legacy formatted endpoint")
)

// CSIAddonsNodeReconciler reconciles a CSIAddonsNode object
type CSIAddonsNodeReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	ConnPool *connection.ConnectionPool
}

//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
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

	key := csiAddonsNode.Namespace + "/" + csiAddonsNode.Name
	logger = logger.WithValues("NodeID", nodeID, "DriverName", driverName)

	if !csiAddonsNode.DeletionTimestamp.IsZero() {
		// if deletion timestamp is set, the CSIAddonsNode is getting deleted,
		// delete connections and remove finalizer.
		logger.Info("Deleting connection")
		r.ConnPool.Delete(key)
		err = r.removeFinalizer(ctx, &logger, csiAddonsNode)
		return ctrl.Result{}, err
	}

	endPoint, err := r.resolveEndpoint(ctx, csiAddonsNode.Spec.Driver.EndPoint)
	if err != nil {
		logger.Error(err, "Failed to resolve endpoint")
		return ctrl.Result{}, fmt.Errorf("failed to resolve endpoint %q: %w", csiAddonsNode.Spec.Driver.EndPoint, err)
	}

	logger = logger.WithValues("EndPoint", endPoint)

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
func (r *CSIAddonsNodeReconciler) SetupWithManager(mgr ctrl.Manager, ctrlOptions controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&csiaddonsv1alpha1.CSIAddonsNode{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		WithOptions(ctrlOptions).
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

// resolveEndpoint parses the endpoint and returned a string that can be used
// by GRPC to connect to the sidecar.
func (r *CSIAddonsNodeReconciler) resolveEndpoint(ctx context.Context, rawURL string) (string, error) {
	namespace, podname, port, err := parseEndpoint(rawURL)
	if err != nil && errors.Is(err, errLegacyEndpoint) {
		return rawURL, nil
	} else if err != nil {
		return "", err
	} else if namespace == "" {
		return "", fmt.Errorf("failed to get namespace from endpoint %q", rawURL)
	} else if podname == "" {
		return "", fmt.Errorf("failed to get pod from endpoint %q", rawURL)
	}

	pod := &corev1.Pod{}
	err = r.Client.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      podname,
	}, pod)
	if err != nil {
		return "", fmt.Errorf("failed to get pod %s/%s: %w", namespace, podname, err)
	} else if pod.Status.PodIP == "" {
		return "", fmt.Errorf("pod %s/%s does not have an IP-address", namespace, podname)
	}

	return fmt.Sprintf("%s:%s", pod.Status.PodIP, port), nil
}

// parseEndpoint returns the rawURL if it is in the legacy <IP-address>:<port>
// format. When the recommended format is used, it returns the Namespace,
// PodName, Port and error instead.
func parseEndpoint(rawURL string) (string, string, string, error) {
	// assume old formatted endpoint, don't parse it
	if !strings.Contains(rawURL, "://") {
		return "", "", "", errLegacyEndpoint
	}

	endpoint, err := url.Parse(rawURL)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to parse endpoint %q: %w", rawURL, err)
	}

	if endpoint.Scheme != "pod" {
		return "", "", "", fmt.Errorf("endpoint scheme %q not supported", endpoint.Scheme)
	}

	// split hostname -> pod.namespace
	parts := strings.Split(endpoint.Hostname(), ".")
	podname := parts[0]
	namespace := ""
	if len(parts) == 2 {
		namespace = parts[1]
	} else if len(parts) > 2 {
		return "", "", "", fmt.Errorf("hostname %q is not in <pod>.<namespace> format", endpoint.Hostname())
	}

	return namespace, podname, endpoint.Port(), nil
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
