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

package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/csiaddons/v1alpha1"
	"github.com/csi-addons/kubernetes-csi-addons/internal/connection"
	"github.com/csi-addons/kubernetes-csi-addons/internal/proto"
	"github.com/csi-addons/kubernetes-csi-addons/internal/util"
	"github.com/csi-addons/spec/lib/go/identity"

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

const (
	// The base duration to use for exponential backoff while retrying connection
	baseRetryDelay = 2 * time.Second

	// Maximum attempts to make while retrying the connection
	maxRetries = 3

	// The duration after which a new reconcile should be triggered
	// to validate the cluster state. Used only when reconciliation
	// completes without any errors.
	baseRequeueAfter = 1 * time.Hour
)

var (
	csiAddonsNodeFinalizer = csiaddonsv1alpha1.GroupVersion.Group + "/csiaddonsnode"
)

// CSIAddonsNodeReconciler reconciles a CSIAddonsNode object
type CSIAddonsNodeReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	ConnPool   *connection.ConnectionPool
	EnableAuth bool
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
	err := r.Get(ctx, req.NamespacedName, csiAddonsNode)
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
		statusErr := r.Status().Update(ctx, csiAddonsNode)
		if statusErr != nil {
			logger.Error(statusErr, "Failed to update status")

			return ctrl.Result{}, statusErr
		}

		// invalid parameter, do not requeue
		return ctrl.Result{}, nil
	}

	nodeID := csiAddonsNode.Spec.Driver.NodeID
	driverName := csiAddonsNode.Spec.Driver.Name

	logger = logger.WithValues("NodeID", nodeID, "DriverName", driverName)

	podName, endPoint, err := r.resolveEndpoint(ctx, csiAddonsNode.Spec.Driver.EndPoint)
	// In case of CR is marked for deletion, we dont need the connection to be established.
	if err != nil && podName == "" && csiAddonsNode.DeletionTimestamp.IsZero() {
		logger.Error(err, "Failed to resolve endpoint")

		// We will either:
		// - requeue and try again with exponential backoff
		// - delete the csiaddonsnode after max retries and stop the reconicle phase
		return r.backoffAndRetry(
			ctx,
			logger,
			csiAddonsNode,
			fmt.Errorf("failed to resolve endpoint %q: %w", csiAddonsNode.Spec.Driver.EndPoint, err),
		)
	}

	// namespace + "/" + leader identity(pod name) is the key for the connection.
	// this key is used by GetLeaderByDriver to get the connection
	// util.NormalizeLeaseName() is used to sanitize the leader identity used for the leases
	// csiaddonsnode need to store the key with same format so that it can be used to get the connection.
	key := csiAddonsNode.Namespace + "/" + util.NormalizeLeaseName(podName)

	logger = logger.WithValues("EndPoint", endPoint)

	if !csiAddonsNode.DeletionTimestamp.IsZero() {
		// if deletion timestamp is set, the CSIAddonsNode is getting deleted,
		// delete connections and remove finalizer.
		logger.Info("Deleting connection", "Key", key)
		r.ConnPool.Delete(key)
		err = r.removeFinalizer(ctx, &logger, csiAddonsNode)
		return ctrl.Result{}, err
	}

	if err := r.addFinalizer(ctx, &logger, csiAddonsNode); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Connecting to sidecar")
	newConn, err := connection.NewConnection(ctx, endPoint, nodeID, driverName, csiAddonsNode.Namespace, csiAddonsNode.Name, r.EnableAuth)

	// If error occurs, we retry with exponential backoff until we reach `maxRetries`
	if err != nil {
		// We will either:
		// - requeue and try again with exponential backoff
		// - delete the csiaddonsnode after max retries and stop the reconicle phase
		return r.backoffAndRetry(ctx, logger, csiAddonsNode, err)
	}

	nfsc, err := r.getNetworkFenceClientStatus(ctx, &logger, newConn, csiAddonsNode)
	if err != nil {
		return ctrl.Result{}, err
	}

	csiAddonsNode.Status.NetworkFenceClientStatus = nfsc

	logger.Info("Successfully connected to sidecar")
	r.ConnPool.Put(key, newConn)
	logger.Info("Added connection to connection pool", "Key", key)

	csiAddonsNode.Status.State = csiaddonsv1alpha1.CSIAddonsNodeStateConnected
	csiAddonsNode.Status.Message = "Successfully established connection with sidecar"
	csiAddonsNode.Status.Reason = ""
	csiAddonsNode.Status.Capabilities = parseCapabilities(newConn.Capabilities)
	err = r.Status().Update(ctx, csiAddonsNode)
	if err != nil {
		logger.Error(err, "Failed to update status")

		return ctrl.Result{}, err
	}

	// Reconciled successfully, requeue to validate state periodically
	return ctrl.Result{RequeueAfter: baseRequeueAfter}, nil
}

// getNetworkFenceClassesForDriver gets the networkfenceclasses for the driver.
func (r *CSIAddonsNodeReconciler) getNetworkFenceClassesForDriver(ctx context.Context, logger *logr.Logger,
	instance *csiaddonsv1alpha1.CSIAddonsNode) ([]csiaddonsv1alpha1.NetworkFenceClass, error) {
	// get the networkfenceclasses from the annotation
	nfclasses := make([]csiaddonsv1alpha1.NetworkFenceClass, 0)
	classesJSON, ok := instance.GetAnnotations()[networkFenceClassAnnotationKey]
	if !ok {
		logger.Info("No networkfenceclasses found in annotation")
		return nfclasses, nil
	}

	var classes []string

	// Unmarshal the existing JSON into a slice of strings.
	if err := json.Unmarshal([]byte(classesJSON), &classes); err != nil {
		logger.Error(err, "Failed to unmarshal existing networkFenceClasses annotation", "name", instance.Name)
		return nfclasses, err
	}

	for _, class := range classes {
		logger.Info("Found networkfenceclass ", "name", class)
		nfc := csiaddonsv1alpha1.NetworkFenceClass{}
		err := r.Get(ctx, client.ObjectKey{Name: class}, &nfc)
		if err != nil {
			logger.Error(err, "Failed to get networkfenceclass", "name", class)
			return nil, err
		}
		nfclasses = append(nfclasses, nfc)
	}

	return nfclasses, nil
}

func (r *CSIAddonsNodeReconciler) getNetworkFenceClientStatus(ctx context.Context, logger *logr.Logger, conn *connection.Connection, csiAddonsNode *csiaddonsv1alpha1.CSIAddonsNode) ([]csiaddonsv1alpha1.NetworkFenceClientStatus, error) {

	nfclasses, err := r.getNetworkFenceClassesForDriver(ctx, logger, csiAddonsNode)
	if err != nil {
		logger.Error(err, "Failed to get network fence classes")
		return nil, err
	}

	var nfsc []csiaddonsv1alpha1.NetworkFenceClientStatus

	for _, nfc := range nfclasses {
		clients, err := getFenceClientDetails(ctx, conn, logger, nfc)
		if err != nil {
			logger.Error(err, "Failed to get clients to fence", "networkFenceClass", nfc.Name)
			return nil, err
		}

		// If no clients are found, skip this network fence class
		if clients == nil {
			continue
		}

		// process the client details for this network fence class
		clientDetails := r.getClientDetails(clients)
		nfsc = append(nfsc, csiaddonsv1alpha1.NetworkFenceClientStatus{
			NetworkFenceClassName: nfc.Name,
			ClientDetails:         clientDetails,
		})
	}

	return nfsc, nil
}

// getClientDetails processes the client details to create the necessary status
func (r *CSIAddonsNodeReconciler) getClientDetails(clients *proto.FenceClientsResponse) []csiaddonsv1alpha1.ClientDetail {
	var clientDetails []csiaddonsv1alpha1.ClientDetail
	for _, client := range clients.Clients {
		clientDetails = append(clientDetails, csiaddonsv1alpha1.ClientDetail{
			Id:    client.Id,
			Cidrs: client.Cidrs,
		})
	}
	return clientDetails
}

// getFenceClientDetails gets the list of clients to fence from the driver.
func getFenceClientDetails(ctx context.Context, conn *connection.Connection, logger *logr.Logger, nfc csiaddonsv1alpha1.NetworkFenceClass) (*proto.FenceClientsResponse, error) {

	param := nfc.Spec.Parameters
	secretName := param[prefixedNetworkFenceSecretNameKey]
	secretNamespace := param[prefixedNetworkFenceSecretNamespaceKey]
	// Remove secret from the parameters
	delete(param, prefixedNetworkFenceSecretNameKey)
	delete(param, prefixedNetworkFenceSecretNamespaceKey)

	// check if the driver contains the GET_CLIENTS_TO_FENCE capability
	// if it does, we need to get the list of clients to fence
	for _, cap := range conn.Capabilities {
		if cap.GetNetworkFence() != nil &&
			cap.GetNetworkFence().GetType() == identity.Capability_NetworkFence_GET_CLIENTS_TO_FENCE {
			logger.Info("Driver support GET_CLIENTS_TO_FENCE capability")
			client := proto.NewNetworkFenceClient(conn.Client)
			req := &proto.FenceClientsRequest{
				Parameters:      nfc.Spec.Parameters,
				SecretName:      secretName,
				SecretNamespace: secretNamespace,
			}
			clients, err := client.GetFenceClients(ctx, req)
			if err != nil {
				logger.Error(err, "Failed to get clients to fence")
				return nil, err
			}
			return clients, nil
		}
	}
	return nil, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CSIAddonsNodeReconciler) SetupWithManager(mgr ctrl.Manager, ctrlOptions controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&csiaddonsv1alpha1.CSIAddonsNode{}).
		WithEventFilter(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{})).
		WithOptions(ctrlOptions).
		Complete(r)
}

// addFinalizer adds finalizer to csiAddonsNode if it is not present.
func (r *CSIAddonsNodeReconciler) addFinalizer(
	ctx context.Context,
	logger *logr.Logger,
	csiAddonsNode *csiaddonsv1alpha1.CSIAddonsNode) error {

	if !slices.Contains(csiAddonsNode.Finalizers, csiAddonsNodeFinalizer) {
		logger.Info("Adding finalizer")

		csiAddonsNode.Finalizers = append(csiAddonsNode.Finalizers, csiAddonsNodeFinalizer)
		if err := r.Update(ctx, csiAddonsNode); err != nil {
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

	if slices.Contains(csiAddonsNode.Finalizers, csiAddonsNodeFinalizer) {
		logger.Info("Removing finalizer")

		csiAddonsNode.Finalizers = util.RemoveFromSlice(csiAddonsNode.Finalizers, csiAddonsNodeFinalizer)
		if err := r.Update(ctx, csiAddonsNode); err != nil {
			logger.Error(err, "Failed to remove finalizer")

			return err
		}
	}

	return nil
}

// resolveEndpoint parses the endpoint and returned a endpoint and pod name that can be used
// by GRPC to connect to the sidecar.
func (r *CSIAddonsNodeReconciler) resolveEndpoint(ctx context.Context, rawURL string) (string, string, error) {
	namespace, podname, port, err := parseEndpoint(rawURL)
	if err != nil {
		return "", "", err
	}
	pod := &corev1.Pod{}
	err = r.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      podname,
	}, pod)
	if err != nil {
		// do not return podname if the pod does not exist
		if apierrors.IsNotFound(err) {
			podname = ""
		}
		return podname, "", fmt.Errorf("failed to get pod %s/%s: %w", namespace, podname, err)
	} else if pod.Status.PodIP == "" {
		return podname, "", fmt.Errorf("pod %s/%s does not have an IP-address", namespace, podname)
	}

	ip := pod.Status.PodIP

	// Detect if the IP has more than one colon in it, indicating an IPv6
	// We need this check to format it correctly, as we will append a port to the IP
	if strings.Count(ip, ":") >= 2 {
		ip = fmt.Sprintf("[%s]", ip)
	}

	return podname, fmt.Sprintf("%s:%s", ip, port), nil
}

// parseEndpoint returns the rawURL if it is in the legacy <IP-address>:<port>
// format. When the recommended format is used, it returns the Namespace,
// PodName, Port and error instead.
func parseEndpoint(rawURL string) (string, string, string, error) {
	endpoint, err := url.Parse(rawURL)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to parse endpoint %q: %w", rawURL, err)
	}

	if endpoint.Scheme != "pod" {
		return "", "", "", fmt.Errorf("endpoint scheme %q not supported", endpoint.Scheme)
	}

	// split hostname -> pod.namespace
	hostName := endpoint.Hostname()
	lastIndex := strings.LastIndex(hostName, ".")

	// check for empty podname and namespace
	if lastIndex <= 0 || lastIndex == len(hostName)-1 {
		return "", "", "", fmt.Errorf("hostname %q is not in <pod>.<namespace> format", hostName)
	}

	podname := hostName[0:lastIndex]
	namespace := hostName[lastIndex+1:]

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

// parseCapabilities returns a list of capabilities in the format
// capability.Type
// e.g. A cap.String with value "service:{type:NODE_SERVICE}"
// Will be parsed and returned as "service.NODE_SERVICE"
func parseCapabilities(caps []*identity.Capability) []string {
	if len(caps) == 0 {
		return []string{}
	}

	capabilities := make([]string, len(caps))

	for i, cap := range caps {
		capStr := strings.ReplaceAll(cap.String(), ":{type:", ".")
		capStr = strings.ReplaceAll(capStr, "}", "")
		capabilities[i] = capStr
	}

	return capabilities
}

// getRetryCountFromReason expects a string and tries to extract
// and return the retry count from the string.
// If the reason string is empty, it assumes the first attempt and returns 0.
// An error is returned if the parsing is not successful.
func getRetryCountFromReason(reason string) (int, error) {
	// Might not be updated yet, likely the 1st attempt
	if reason == "" {
		return 0, nil
	}

	parts := strings.SplitN(reason, ":", 2)
	if len(parts) < 2 {
		return 0, errors.New("got an unexpected length after splitting the reason string")
	}

	// Parse
	if c, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
		return c, nil
	}

	return 0, errors.New("failed to parse the reason string to an integer")
}

// backoffAndRetry handles the retry mechanism with exponential backoff for establishing
// a connection with the sidecar. It updates the status of the CSIAddonsNode object to
// reflect the current retry attempt and reason for failure. If the maximum number of retries
// is reached, the CSIAddonsNode object is deleted to abort further attempts.
func (r *CSIAddonsNodeReconciler) backoffAndRetry(
	ctx context.Context,
	logger logr.Logger,
	csiAddonsNode *csiaddonsv1alpha1.CSIAddonsNode,
	err error,
) (ctrl.Result, error) {
	// Only continue if we get a valid/initial retry count
	currentRetries, e := getRetryCountFromReason(csiAddonsNode.Status.Reason)
	if e != nil {
		logger.Error(e, "failed to get the retry count from csiAddonsNode status", "csiAddonsNodeStatus", csiAddonsNode.Status)

		return ctrl.Result{}, e
	}
	logger.Error(err, "Failed to establish connection with sidecar", "attempt", currentRetries+1)

	// If reached max retries, abort and cleanup
	if currentRetries >= maxRetries {
		logger.Info(fmt.Sprintf("Failed to establish connection with sidecar after %d attempts, deleting the object", maxRetries))

		if delErr := r.Delete(ctx, csiAddonsNode); client.IgnoreNotFound(delErr) != nil {
			logger.Error(delErr, "failed to delete CSIAddonsNode object after max retries")

			return ctrl.Result{}, delErr
		}

		// Object is deleted, stop the reconcile phase
		logger.Info("successfully deleted CSIAddonsNode object due to reaching max reconnection attempts")
		return ctrl.Result{}, nil
	}

	errMessage := util.GetErrorMessage(err)
	csiAddonsNode.Status.State = csiaddonsv1alpha1.CSIAddonsNodeStateRetrying
	csiAddonsNode.Status.Message = fmt.Sprintf("Connection failed: %v. Retrying attempt %d/%d.", errMessage, currentRetries+1, maxRetries)
	csiAddonsNode.Status.Reason = fmt.Sprintf(csiaddonsv1alpha1.CSIAddonsNodeStateRetryingFmtStr, currentRetries+1)
	statusErr := r.Status().Update(ctx, csiAddonsNode)
	if statusErr != nil {
		logger.Error(statusErr, "Failed to update status")

		return ctrl.Result{}, statusErr
	}

	// Calculate backoff; baseRetryDelay * 1, 2, 4....
	backoff := baseRetryDelay * time.Duration(math.Pow(2, float64(currentRetries)))
	logger.Info("Requeuing request for attempting the connection again", "backoff", backoff)

	return ctrl.Result{RequeueAfter: backoff}, nil
}
