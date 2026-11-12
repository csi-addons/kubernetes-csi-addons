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
	"errors"
	"fmt"
	"math"
	"time"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/csiaddons/v1alpha1"
	"github.com/csi-addons/kubernetes-csi-addons/internal/connection"
	"github.com/csi-addons/kubernetes-csi-addons/internal/controller/utils"
	"github.com/csi-addons/kubernetes-csi-addons/internal/proto"

	"github.com/csi-addons/spec/lib/go/identity"
	"github.com/go-logr/logr"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	scv1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	// default values for Spec parameters.
	defaultBackoffLimit         = 6
	defaultRetryDeadlineSeconds = 600

	// failed condition type.
	conditionFailed = "Failed"
	// failed reason type.
	// TODO: add more useful reason types.
	reasonFailed = "failed"

	// nodeClientRequeueInterval is the interval to requeue when the
	// volume is not attached to a node yet, or the node client is not
	// found in the connection pool, allowing time for the
	// VolumeAttachment to be updated or the node plugin to register.
	nodeClientRequeueInterval = 30 * time.Second
)

var (
	// errNodeClientNotFound is a sentinel error returned when the node
	// client for the given nodeID is not found in the connection pool.
	errNodeClientNotFound = errors.New("node client not found")

	// errVolumeNotAttached is a sentinel error returned when a node-side
	// reclaim was requested but the volume is not attached to a node yet.
	errVolumeNotAttached = errors.New("volume is not attached to any node")

	// errNodeReclaimUnsupported is a sentinel error returned when the node
	// plugin for the given nodeID is registered but does not advertise the
	// ReclaimSpace_ONLINE capability, or the NodeReclaimSpace RPC returns
	// Unimplemented. Retrying will not make it available.
	errNodeReclaimUnsupported = errors.New("NodeReclaimSpace is not supported")

	// errControllerReclaimUnsupported is a sentinel error returned when the
	// leading controller does not advertise the ReclaimSpace_OFFLINE
	// capability, or the ControllerReclaimSpace RPC returns Unimplemented.
	// Retrying will not make it available.
	errControllerReclaimUnsupported = errors.New("ControllerReclaimSpace is not supported")
)

// ReclaimSpaceJobReconciler reconciles a ReclaimSpaceJob object.
type ReclaimSpaceJobReconciler struct {
	client.Client
	// Scheme defines methods for serializing and deserializing API objects.
	Scheme *runtime.Scheme
	// ConnectionPool consists of map of Connection objects.
	ConnPool *connection.ConnectionPool
	// Timeout for the Reconcile operation.
	Timeout time.Duration
}

//+kubebuilder:rbac:groups=csiaddons.openshift.io,resources=reclaimspacejobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=csiaddons.openshift.io,resources=reclaimspacejobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=csiaddons.openshift.io,resources=reclaimspacejobs/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;update
//+kubebuilder:rbac:groups="",resources=persistentvolumes,verbs=get;list;watch
//+kubebuilder:rbac:groups=storage.k8s.io,resources=volumeattachments,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ReclaimSpaceJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch ReclaimSpaceJob instance.
	rsJob := &csiaddonsv1alpha1.ReclaimSpaceJob{}
	err := r.Get(ctx, req.NamespacedName, rsJob)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			logger.Info("ReclaimSpaceJob resource not found")

			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	if !rsJob.DeletionTimestamp.IsZero() {
		logger.Info("ReclaimSpaceJob resource is being deleted, exiting reconcile")
		return ctrl.Result{}, nil
	}

	if rsJob.Status.Result != "" {
		logger.Info(fmt.Sprintf("ReclaimSpaceJob is already in %q state, exiting reconcile",
			rsJob.Status.Result))
		// since result is already set, just dequeue.
		return ctrl.Result{}, nil
	}

	err = validateReclaimSpaceJobSpec(rsJob)
	if err != nil {
		logger.Error(err, "Failed to validate ReclaimSpaceJob.Spec")

		rsJob.Status.Result = csiaddonsv1alpha1.OperationResultFailed
		rsJob.Status.Message = fmt.Sprintf("Failed to validate ReclaimSpaceJob.Spec: %v", err)
		rsJob.Status.CompletionTime = &metav1.Time{Time: time.Now()}
		if statusErr := r.Status().Update(ctx, rsJob); statusErr != nil {
			logger.Error(statusErr, "Failed to update status")
			return ctrl.Result{}, statusErr
		}

		// invalid parameters, do not requeue.
		return ctrl.Result{}, nil
	}

	// set default values if equal to 0.
	if rsJob.Spec.BackoffLimit == 0 {
		rsJob.Spec.BackoffLimit = defaultBackoffLimit
	}
	if rsJob.Spec.RetryDeadlineSeconds == 0 {
		rsJob.Spec.RetryDeadlineSeconds = defaultRetryDeadlineSeconds
	}

	err = r.reconcile(
		ctx,
		&logger,
		rsJob,
		req.Namespace,
	)

	if statusErr := r.Status().Update(ctx, rsJob); statusErr != nil {
		logger.Error(statusErr, "Failed to update status")

		return ctrl.Result{}, statusErr
	}

	if rsJob.Status.Result != "" {
		// since result is already set, just dequeue.
		return ctrl.Result{}, nil
	}

	// If the volume is not attached yet or the node client is not found,
	// requeue after an interval to allow the VolumeAttachment to be updated
	// instead of using the fast rate-limited requeue. This also applies when
	// another operation failed in the same pass, so that the fast backoff
	// does not exhaust the retries before the node becomes available.
	if errors.Is(err, errNodeClientNotFound) || errors.Is(err, errVolumeNotAttached) {
		requeueAfter := nodeClientRequeueInterval
		if result, ok := utils.ResultForAbortedError(err, rsJob.Status.Retries); ok && result.RequeueAfter > requeueAfter {
			requeueAfter = result.RequeueAfter
		}
		logger.Info("Node not available, requeuing", "err", err, "after", requeueAfter)

		return ctrl.Result{RequeueAfter: requeueAfter}, nil
	}

	if result, ok := utils.ResultForAbortedError(err, rsJob.Status.Retries); ok {
		logger.Info("Operation aborted, requeuing with backoff", "err", err, "backoff", result.RequeueAfter)
		return result, nil
	}

	return ctrl.Result{}, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *ReclaimSpaceJobReconciler) SetupWithManager(mgr ctrl.Manager, ctrlOptions controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&csiaddonsv1alpha1.ReclaimSpaceJob{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		// This is to avoid "stop-the-world" events and wait for cache sync when we list VA
		Watches(
			&scv1.VolumeAttachment{},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(utils.SilentPredicate()),
		).
		WithOptions(ctrlOptions).
		Complete(r)
}

// targetDetails contains required information to make controller and
// node reclaimspace grpc requests.
type targetDetails struct {
	driverName string
	pvName     string
	pvcName    string
	nodeID     string
	timeout    time.Duration
}

// reconcile performs time based validation, fetches required details and
// attempts the requested reclaim space operations. Operations that already
// reached a terminal state are not attempted again.
func (r *ReclaimSpaceJobReconciler) reconcile(
	ctx context.Context,
	logger *logr.Logger,
	rsJob *csiaddonsv1alpha1.ReclaimSpaceJob,
	namespace string) error {

	if rsJob.Status.StartTime == nil {
		// this is the first reconcile, add StartTime
		rsJob.Status.StartTime = &metav1.Time{Time: time.Now()}
	} else {
		// not first reconcile, increment retries
		rsJob.Status.Retries++
	}

	// Ensure there is a status entry for every requested operation before
	// any early return, so the reported state always covers the whole spec.
	syncOperationStatuses(rsJob)

	// check whether currentTime > CreationTime + RetryDeadlineSeconds,
	// if true, mark it as Time limit reached and fail.
	if time.Now().After(rsJob.CreationTimestamp.Add(time.Second * time.Duration(rsJob.Spec.RetryDeadlineSeconds))) {
		logger.Info("Time limit reached")
		r.failJob(rsJob, "Time limit reached")

		return nil
	}

	target, err := r.getTargetDetails(ctx, logger, rsJob.Spec, namespace)
	if err != nil {
		logger.Error(err, "Failed to get target details")
		message := fmt.Sprintf("failed to get target details: %v", err)
		setPendingOperationsMessage(rsJob.Status.Operations, message)

		if retryLimitReached(rsJob) {
			logger.Info("Maximum retry limit reached")
			r.failJob(rsJob, "Maximum retry limit reached")

			return nil
		}

		setFailedCondition(&rsJob.Status.Conditions, message, rsJob.Generation)

		return err
	}

	var retryErrs []error
	for i := range rsJob.Status.Operations {
		opStatus := &rsJob.Status.Operations[i]
		if opStatus.State.IsTerminal() {
			continue
		}

		outcome := r.runOperation(ctx, *logger, target, opStatus.Operation)
		setOperationOutcomeStatus(opStatus, outcome, time.Now())
		if outcome.state == csiaddonsv1alpha1.ReclaimSpaceOperationStateSkipped {
			logger.Info("Reclaim space operation skipped",
				"operation", opStatus.Operation, "reason", outcome.message)
		}
		if outcome.err != nil {
			retryErrs = append(retryErrs, outcome.err)
		}
	}

	if allOperationsTerminal(rsJob.Status.Operations) {
		r.completeJob(rsJob)
		logger.Info("Reclaim space operation completed",
			"result", rsJob.Status.Result, "message", rsJob.Status.Message)

		return nil
	}

	if retryLimitReached(rsJob) {
		logger.Info("Maximum retry limit reached")
		r.failJob(rsJob, "Maximum retry limit reached")

		return nil
	}

	setFailedCondition(
		&rsJob.Status.Conditions,
		incompleteMessage(rsJob.Status.Operations),
		rsJob.Generation)

	return retryError(retryErrs)
}

// getTargetDetails fetches driverName, pvName and nodeID in targetDetails struct.
func (r *ReclaimSpaceJobReconciler) getTargetDetails(
	ctx context.Context,
	logger *logr.Logger,
	spec csiaddonsv1alpha1.ReclaimSpaceJobSpec,
	namespace string) (*targetDetails, error) {
	*logger = logger.WithValues("PVCName", spec.Target.PersistentVolumeClaim, "PVCNamespace", namespace)
	req := types.NamespacedName{Name: spec.Target.PersistentVolumeClaim, Namespace: namespace}
	pvc := &corev1.PersistentVolumeClaim{}

	err := r.Get(ctx, req, pvc)
	if err != nil {
		return nil, err
	}

	// Validate PVC in bound state
	if pvc.Status.Phase != corev1.ClaimBound {
		return nil, fmt.Errorf("PVC %q is not bound to any PV", req.Name)
	}

	*logger = logger.WithValues("PVName", pvc.Spec.VolumeName)
	pv := &corev1.PersistentVolume{}
	req = types.NamespacedName{Name: pvc.Spec.VolumeName}

	err = r.Get(ctx, req, pv)
	if err != nil {
		return nil, err
	}

	if pv.Spec.CSI == nil {
		return nil, fmt.Errorf("%q PV is not a CSI PVC", pv.Name)
	}

	volumeAttachments := &scv1.VolumeAttachmentList{}
	err = r.List(ctx, volumeAttachments)
	if err != nil {
		return nil, err
	}

	details := targetDetails{
		driverName: pv.Spec.CSI.Driver,
		pvName:     pv.Name,
		pvcName:    pvc.Name,
		// Set global default timeout.
		timeout: r.Timeout,
	}
	for _, v := range volumeAttachments.Items {
		if v.DeletionTimestamp.IsZero() && v.Status.Attached && v.Spec.Source.PersistentVolumeName != nil && *v.Spec.Source.PersistentVolumeName == pv.Name {
			*logger = logger.WithValues("NodeID", v.Spec.NodeName)
			details.nodeID = v.Spec.NodeName
			break
		}
	}
	// Override global default timeout if timeout is specified
	// in spec.
	if spec.Timeout != nil {
		details.timeout = time.Second * time.Duration(*spec.Timeout)
	}
	*logger = logger.WithValues("Timeout", details.timeout)

	return &details, nil
}

// getLeadingRSClient returns the ReclaimSpaceClient of the sidecar that holds
// the lease for driverName.
func (r *ReclaimSpaceJobReconciler) getLeadingRSClient(
	ctx context.Context,
	driverName string) (proto.ReclaimSpaceClient, error) {
	conn, err := r.ConnPool.GetLeaderByDriver(ctx, r.Client, driverName)
	if err != nil {
		return nil, err
	}

	for _, cap := range conn.Capabilities {
		if cap.GetReclaimSpace() == nil {
			continue
		}
		if cap.GetReclaimSpace().Type == identity.Capability_ReclaimSpace_OFFLINE {
			return proto.NewReclaimSpaceClient(conn.Client), nil
		}
	}

	// The required cap is missing, the op is unsupported.
	return nil, fmt.Errorf("%w: the leading controller of driver %q does not advertise the ReclaimSpace_OFFLINE capability",
		errControllerReclaimUnsupported, driverName)
}

// getRSClientByNode returns ReclaimSpaceClient given driverName and nodeID.
// A connection counts as a node plugin only when it advertises the
// NODE_SERVICE capability. It returns errNodeClientNotFound when no node
// plugin connection is registered for the node, and errNodeReclaimUnsupported
// when a node plugin is registered but no connection on the node advertises
// the ReclaimSpace_ONLINE capability.
func (r *ReclaimSpaceJobReconciler) getRSClientByNode(
	driverName, nodeID string) (string, proto.ReclaimSpaceClient, error) {
	conns, err := r.ConnPool.GetByNodeID(driverName, nodeID)
	if err != nil {
		return "", nil, err
	}

	nodePluginSeen := false
	for k, v := range conns {
		if v.HasNodeService() {
			nodePluginSeen = true
		}
		for _, cap := range v.Capabilities {
			if cap.GetReclaimSpace() == nil {
				continue
			}
			if cap.GetReclaimSpace().Type == identity.Capability_ReclaimSpace_ONLINE {
				return k, proto.NewReclaimSpaceClient(v.Client), nil
			}
		}
	}

	// Only report the operation as unsupported once a node plugin has actually registered.
	// Otherwise, it might still be starting up.
	if !nodePluginSeen {
		return "", nil, fmt.Errorf("%w for %q nodeID", errNodeClientNotFound, nodeID)
	}

	return "", nil, fmt.Errorf("%w: the node plugin on node %q does not advertise the ReclaimSpace_ONLINE capability",
		errNodeReclaimUnsupported, nodeID)
}

// controllerReclaimSpace makes a ControllerReclaimSpace request and returns the
// amount of reclaimed space, if available.
func (r *ReclaimSpaceJobReconciler) controllerReclaimSpace(
	ctx context.Context,
	logger logr.Logger,
	target *targetDetails) (*int64, error) {
	controllerClient, err := r.getLeadingRSClient(ctx, target.driverName)
	if err != nil {
		// An unsupported operation is expected and logged as skipped by the caller.
		if !errors.Is(err, errControllerReclaimUnsupported) {
			logger.Error(err, "Failed to get controller client", "driverName", target.driverName)
		}

		return nil, err
	}

	logger.Info("Making controller reclaim space request")
	req := &proto.ReclaimSpaceRequest{
		PvName: target.pvName,
	}
	newCtx, cancel := context.WithTimeout(ctx, target.timeout)
	defer cancel()

	resp, err := controllerClient.ControllerReclaimSpace(newCtx, req)
	if err != nil {
		// Unimplemented suggests that the function is not supported.
		if status.Code(err) == codes.Unimplemented {
			return nil, fmt.Errorf("%w: %v", errControllerReclaimUnsupported, err)
		}

		return nil, err
	}

	return calculateReclaimedSpace(resp.PreUsage, resp.PostUsage), nil
}

// nodeReclaimSpace makes a NodeReclaimSpace request and returns the amount of
// reclaimed space, if available.
func (r *ReclaimSpaceJobReconciler) nodeReclaimSpace(
	ctx context.Context,
	logger logr.Logger,
	target *targetDetails) (*int64, error) {
	if target.nodeID == "" {
		return nil, fmt.Errorf("%w: PVC %q", errVolumeNotAttached, target.pvcName)
	}

	clientName, nodeClient, err := r.getRSClientByNode(
		target.driverName,
		target.nodeID)
	if err != nil {
		return nil, err
	}
	logger = logger.WithValues("nodeClient", clientName)

	logger.Info("Making node reclaim space request")
	req := &proto.ReclaimSpaceRequest{
		PvName: target.pvName,
	}
	newCtx, cancel := context.WithTimeout(ctx, target.timeout)
	defer cancel()

	resp, err := nodeClient.NodeReclaimSpace(newCtx, req)
	if err != nil {
		// Unimplemented suggests that the function is not supported.
		if status.Code(err) == codes.Unimplemented {
			return nil, fmt.Errorf("%w: %v", errNodeReclaimUnsupported, err)
		}

		return nil, err
	}

	return calculateReclaimedSpace(resp.PreUsage, resp.PostUsage), nil
}

// calculateReclaimedSpace returns amount of reclaimed space.
func calculateReclaimedSpace(PreUsage, PostUsage *proto.StorageConsumption) *int64 {
	if PreUsage == nil || PostUsage == nil {
		return nil
	}
	preUsage := PreUsage.UsageBytes
	postUsage := PostUsage.UsageBytes

	result := int64(math.Max(float64(postUsage)-float64(preUsage), 0))
	return &result
}

// validateReclaimSpaceJob validates and sets default values for ReclaimSpaceJob.Spec.
func validateReclaimSpaceJobSpec(
	rsJob *csiaddonsv1alpha1.ReclaimSpaceJob) error {
	if rsJob.Spec.Target.PersistentVolumeClaim == "" {
		return errors.New("required parameter 'PersistentVolumeClaim' in ReclaimSpaceJob.Spec.Target is empty")
	}

	return nil
}

// setFailedCondition updates failedCondition type if it exists else
// appends a new condition.
func setFailedCondition(
	conditions *[]metav1.Condition,
	message string,
	observedGeneration int64) {
	meta.SetStatusCondition(conditions, metav1.Condition{
		Type:               conditionFailed,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: observedGeneration,
		Message:            message,
		Reason:             reasonFailed,
	})
}

// clearFailedCondition removes the failed condition, if present.
func clearFailedCondition(conditions *[]metav1.Condition) {
	meta.RemoveStatusCondition(conditions, conditionFailed)
}
