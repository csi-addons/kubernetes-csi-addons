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
	"encoding/json"
	stdError "errors"
	"fmt"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/csiaddons/v1alpha1"
	"github.com/csi-addons/kubernetes-csi-addons/internal/util"
	"github.com/go-logr/logr"
)

const (
	driverName                     = "spec.driver.name" // FieldIndexer for CSIAddonsNode
	provisionerName                = "spec.provisioner" // FieldIndexer for NetworkFenceClass
	networkFenceClassAnnotationKey = "csiaddons.openshift.io/networkfenceclass-names"

	// NetworkFenceClass Parameters prefixed with networkFenceParameterPrefix are not passed through
	// to the driver on RPC calls. Instead these are the parameters used by the
	// operator to get the required object from kubernetes and pass it to the
	// Driver.
	networkFenceParameterPrefix = "csiaddons.openshift.io/"

	prefixedNetworkFenceSecretNameKey      = networkFenceParameterPrefix + "networkfence-secret-name"      // name key for secret
	prefixedNetworkFenceSecretNamespaceKey = networkFenceParameterPrefix + "networkfence-secret-namespace" // namespace key secret
)

// NetworkFenceClassReconciler reconciles a NetworkFenceClass object
type NetworkFenceClassReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=csiaddons.openshift.io,resources=networkfenceclasses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=csiaddons.openshift.io,resources=networkfenceclasses/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=csiaddons.openshift.io,resources=networkfenceclasses/finalizers,verbs=update
//+kubebuilder:rbac:groups=csiaddons.openshift.io,resources=csiaddonsnodes,verbs=list;watch;update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the NetworkFenceClass object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *NetworkFenceClassReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx, "Request.Name", req.Name)

	// Fetch NetworkFenceClass instance
	instance, err := r.getNetworkFenceClass(ctx, req)
	if err != nil {
		return ctrl.Result{}, err
	}

	// NetworkFenceClass not found
	if instance == nil {
		return ctrl.Result{}, nil
	}

	err = validatePrefixedParameters(instance.Spec.Parameters)
	if err != nil {
		return ctrl.Result{}, err
	}

	nfUnderDeletion := instance.DeletionTimestamp != nil

	if !nfUnderDeletion {
		// Add finalizer to NetworkFenceClass
		if err := r.addFinalizer(ctx, &logger, instance); err != nil {
			return ctrl.Result{}, err
		}
	}

	// List all relevant CSIAddonsNode objects
	csiaddonsNodeList, err := r.listCSIAddonsNodes(ctx, instance.Spec.Provisioner)
	if err != nil {
		return ctrl.Result{}, err
	}

	if csiaddonsNodeList == nil {
		return ctrl.Result{}, nil
	}

	var errs []string
	// Process each CSIAddonsNode
	for _, csiaddonsnode := range csiaddonsNodeList.Items {
		// skip if the object is being deleted
		if csiaddonsnode.DeletionTimestamp == nil {
			if err := r.processCSIAddonsNode(ctx, &logger, &csiaddonsnode, instance, nfUnderDeletion); err != nil {
				errs = append(errs, fmt.Sprintf("error processing node %s: for NetworkFenceClass %s: %v", csiaddonsnode.Name, instance.Name, err))
			}
		}
	}

	if len(errs) > 0 {
		return ctrl.Result{}, fmt.Errorf("multiple errors occurred: %s", strings.Join(errs, ", "))
	}

	if nfUnderDeletion {
		// Remove finalizer to NetworkFenceClass
		if err := r.removeFinalizer(ctx, &logger, instance); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// addFinalizer adds finalizer to networkFenceClass if it is not present.
func (r *NetworkFenceClassReconciler) addFinalizer(
	ctx context.Context,
	logger *logr.Logger,
	networkFenceClass *csiaddonsv1alpha1.NetworkFenceClass) error {

	if !slices.Contains(networkFenceClass.Finalizers, csiAddonsNodeFinalizer) {
		logger.Info("Adding finalizer")

		networkFenceClass.Finalizers = append(networkFenceClass.Finalizers, csiAddonsNodeFinalizer)
		if err := r.Client.Update(ctx, networkFenceClass); err != nil {
			logger.Error(err, "Failed to add finalizer")

			return err
		}
	}

	return nil
}

// removeFinalizer removes finalizer from networkFenceClass if it is present.
func (r *NetworkFenceClassReconciler) removeFinalizer(
	ctx context.Context,
	logger *logr.Logger,
	networkFenceClass *csiaddonsv1alpha1.NetworkFenceClass) error {

	if slices.Contains(networkFenceClass.Finalizers, csiAddonsNodeFinalizer) {
		logger.Info("Removing finalizer")

		networkFenceClass.Finalizers = util.RemoveFromSlice(networkFenceClass.Finalizers, csiAddonsNodeFinalizer)
		if err := r.Client.Update(ctx, networkFenceClass); err != nil {
			logger.Error(err, "Failed to remove finalizer")

			return err
		}
	}

	return nil
}

// getNetworkFenceClass fetches the NetworkFenceClass object.
func (r *NetworkFenceClassReconciler) getNetworkFenceClass(ctx context.Context, req ctrl.Request) (*csiaddonsv1alpha1.NetworkFenceClass, error) {
	instance := &csiaddonsv1alpha1.NetworkFenceClass{}
	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			log.FromContext(ctx).Info("NetworkFenceClass resource not found")
			return nil, nil
		}
		return nil, err
	}

	return instance, nil
}

func (r *NetworkFenceClassReconciler) listCSIAddonsNodes(ctx context.Context, provisioner string) (*csiaddonsv1alpha1.CSIAddonsNodeList, error) {
	csiaddonsNodeList := &csiaddonsv1alpha1.CSIAddonsNodeList{}
	err := r.Client.List(ctx, csiaddonsNodeList, client.MatchingFields{driverName: provisioner})
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to list CSIAddonsNode objects")
		return nil, err
	}
	return csiaddonsNodeList, nil
}

func (r *NetworkFenceClassReconciler) processCSIAddonsNode(ctx context.Context, logger *logr.Logger, csiaddonsnode *csiaddonsv1alpha1.CSIAddonsNode, instance *csiaddonsv1alpha1.NetworkFenceClass, nfUnderDeletion bool) error {
	if len(csiaddonsnode.Status.Capabilities) == 0 {
		return stdError.New("CSIAddonsNode status capabilities not found")
	}

	if csiaddonsnode.Annotations == nil {
		csiaddonsnode.Annotations = make(map[string]string)
	}

	// Retrieve the existing annotation (if any).
	classesJSON := csiaddonsnode.Annotations[networkFenceClassAnnotationKey]
	var classes []string
	if classesJSON != "" {
		// Unmarshal the existing JSON into a slice of strings.
		if err := json.Unmarshal([]byte(classesJSON), &classes); err != nil {
			logger.Error(err, "Failed to unmarshal existing networkFenceClasses annotation", "name", csiaddonsnode.Name)
			return err
		}
	}

	for _, capability := range csiaddonsnode.Status.Capabilities {
		if strings.Contains(capability, "GET_CLIENTS_TO_FENCE") {
			logger.Info("Found GET_CLIENTS_TO_FENCE capability", "name", csiaddonsnode.Name, "driverName", csiaddonsnode.Spec.Driver.Name, "nodeID", csiaddonsnode.Spec.Driver.NodeID)

			ok := slices.Contains(classes, instance.Name)
			if !ok && !nfUnderDeletion {
				// If the class is not already in the annotation and the node is not under deletion, add it.
				classes = append(classes, instance.Name)
				updatedClassesJSON, err := json.Marshal(classes)
				if err != nil {
					logger.Error(err, "Failed to marshal updated classes into JSON", "name", csiaddonsnode.Name)
					return err
				}

				// Store the updated JSON in the annotation.
				csiaddonsnode.Annotations[networkFenceClassAnnotationKey] = string(updatedClassesJSON)
				logger.Info("Adding NetworkFenceClass to csiaddonsnode annotations", "name", csiaddonsnode.Name, "NetworkFenceClass", instance.Name)
				return r.Client.Update(ctx, csiaddonsnode)
			}

			if ok && nfUnderDeletion {
				// Remove the NetworkFenceClass from the annotation (if it exists).
				classes = removeClassFromList(classes, instance.Name)
				updatedClassesJSON, err := json.Marshal(classes)
				if err != nil {
					logger.Error(err, "Failed to marshal updated classes after removal into JSON", "name", csiaddonsnode.Name)
					return err
				}

				if len(classes) == 0 {
					// If the list of classes is empty, remove the annotation.
					delete(csiaddonsnode.Annotations, networkFenceClassAnnotationKey)
				} else {
					// Update the annotation with the modified list of classes.
					csiaddonsnode.Annotations[networkFenceClassAnnotationKey] = string(updatedClassesJSON)
				}
				logger.Info("Removing NetworkFenceClass from csiaddonsnode annotation", "name", csiaddonsnode.Name, "NetworkFenceClass", instance.Name)
				return r.Client.Update(ctx, csiaddonsnode)
			}
		}
	}

	return nil
}

// removeClassFromList removes a class name from the list of class names.
func removeClassFromList(classes []string, className string) []string {
	for i, c := range classes {
		if c == className {
			// Remove the element from the slice.
			return append(classes[:i], classes[i+1:]...)
		}
	}

	return classes
}

// validatePrefixParameters checks for unknown reserved keys in parameters and
// empty values for reserved keys.
func validatePrefixedParameters(param map[string]string) error {
	for k, v := range param {
		if strings.HasPrefix(k, networkFenceParameterPrefix) {
			switch k {
			case prefixedNetworkFenceSecretNameKey:
				if v == "" {
					return stdError.New("secret name cannot be empty")
				}
			case prefixedNetworkFenceSecretNamespaceKey:
				if v == "" {
					return stdError.New("secret namespace cannot be empty")
				}
			// keep adding known prefixes to this list.
			default:
				return fmt.Errorf("found unknown parameter key %q with reserved prefix %s", k, networkFenceParameterPrefix)
			}
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NetworkFenceClassReconciler) SetupWithManager(mgr ctrl.Manager) error {
	ctx := context.Background()
	if err := mgr.GetFieldIndexer().IndexField(ctx, &csiaddonsv1alpha1.CSIAddonsNode{}, driverName, func(o client.Object) []string {
		if csiAddonsNode, ok := o.(*csiaddonsv1alpha1.CSIAddonsNode); ok && csiAddonsNode.Spec.Driver.Name != "" {
			return []string{csiAddonsNode.Spec.Driver.Name}
		}
		return nil
	}); err != nil {
		return fmt.Errorf("unable to set up FieldIndexer for CSIAddonsNode Provisioner: %v", err)
	}

	if err := mgr.GetFieldIndexer().IndexField(ctx, &csiaddonsv1alpha1.NetworkFenceClass{}, provisionerName, func(o client.Object) []string {
		if nfc, ok := o.(*csiaddonsv1alpha1.NetworkFenceClass); ok && nfc.Spec.Provisioner != "" {
			return []string{nfc.Spec.Provisioner}
		}
		return nil
	}); err != nil {
		return fmt.Errorf("unable to set up FieldIndexer for NetworkFenceClass Provisioner: %v", err)
	}

	csiAddonsNodePredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return true
		},
		// No need to reconcile the object when it is updated
		UpdateFunc: func(e event.UpdateEvent) bool {
			return false
		},
		// No need to reconcile the object when it is deleted
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
	}
	// Reconcile the OperatorConfigMap object when the cluster's version object is updated
	enqueueNFC := handler.EnqueueRequestsFromMapFunc(
		func(ctx context.Context, obj client.Object) []reconcile.Request {
			// get the object and list of all NetworkFenceClass objects based on the driver name
			csiAddonsNode, ok := obj.(*csiaddonsv1alpha1.CSIAddonsNode)
			if !ok {
				return []reconcile.Request{}
			}
			networkFenceClaimList := &csiaddonsv1alpha1.NetworkFenceClassList{}
			err := r.Client.List(ctx, networkFenceClaimList, client.MatchingFields{provisionerName: csiAddonsNode.Spec.Driver.Name})
			if err != nil {
				return []reconcile.Request{}
			}

			requests := make([]reconcile.Request, 0, len(networkFenceClaimList.Items))
			for _, networkFenceClaim := range networkFenceClaimList.Items {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: networkFenceClaim.Name,
					},
				})
			}

			return requests
		},
	)

	return ctrl.NewControllerManagedBy(mgr).
		For(&csiaddonsv1alpha1.NetworkFenceClass{}).
		Watches(&csiaddonsv1alpha1.CSIAddonsNode{}, enqueueNFC, builder.WithPredicates(csiAddonsNodePredicate)).
		Complete(r)
}
