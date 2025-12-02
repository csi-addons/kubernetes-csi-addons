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
	"github.com/csi-addons/kubernetes-csi-addons/internal/util"

	"github.com/csi-addons/spec/lib/go/identity"
	"github.com/go-logr/logr"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	scv1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
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
			logger.Error(err, "Failed to update status")
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

	if rsJob.Status.Result == "" && rsJob.Status.Retries == rsJob.Spec.BackoffLimit {
		logger.Info("Maximum retry limit reached")
		rsJob.Status.Result = csiaddonsv1alpha1.OperationResultFailed
		rsJob.Status.Message = "Maximum retry limit reached"
		rsJob.Status.CompletionTime = &metav1.Time{Time: time.Now()}
	}

	if statusErr := r.Status().Update(ctx, rsJob); statusErr != nil {
		logger.Error(statusErr, "Failed to update status")

		return ctrl.Result{}, statusErr
	}

	if rsJob.Status.Result != "" {
		// since result is already set, just dequeue.
		return ctrl.Result{}, nil
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
	nodeID     string
	timeout    time.Duration
}

// canNodeReclaimSpace returns true if nodeID is not empty,
// indicating volume is mounted to a pod and node reclaimspace
// request can be sent.
func (td *targetDetails) canNodeReclaimSpace() bool {
	return td.nodeID != ""
}

// reconcile performs time based validation, fetches required details and makes
// grpc request for controller and node reclaim space operation.
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

	// check whether currentTime > CreationTime + RetryDeadlineSeconds,
	// if true, mark it as Time limit reached and fail.
	if time.Now().After(rsJob.CreationTimestamp.Add(time.Second * time.Duration(rsJob.Spec.RetryDeadlineSeconds))) {
		logger.Info("Time limit reached")
		rsJob.Status.Result = csiaddonsv1alpha1.OperationResultFailed
		rsJob.Status.Message = "Time limit reached"
		rsJob.Status.CompletionTime = &metav1.Time{Time: time.Now()}

		return nil
	}

	target, err := r.getTargetDetails(ctx, logger, rsJob.Spec, namespace)
	if err != nil {
		logger.Error(err, "Failed to get target details")
		setFailedCondition(
			&rsJob.Status.Conditions,
			"Failed to get target details",
			rsJob.Generation)

		return err
	}

	var (
		nodeFound          = false
		nodeReclaimedSpace *int64
	)
	if target.canNodeReclaimSpace() {
		nodeFound = true
		nodeReclaimedSpace, err = r.nodeReclaimSpace(ctx, logger, target)
		if err != nil {
			logger.Error(err, "Failed to make node request")
			setFailedCondition(
				&rsJob.Status.Conditions,
				fmt.Sprintf("Failed to make node request: %v", util.GetErrorMessage(err)),
				rsJob.Generation)

			return err
		}
	}

	controllerFound, controllerReclaimedSpace, err := r.controllerReclaimSpace(ctx, logger, target)
	if err != nil {
		logger.Error(err, "Failed to make controller request")
		setFailedCondition(
			&rsJob.Status.Conditions,
			fmt.Sprintf("Failed to make controller request: %v", util.GetErrorMessage(err)),
			rsJob.Generation)

		return err
	}

	if !controllerFound && !nodeFound {
		err = fmt.Errorf("controller and Node Client not found for %q nodeID", target.nodeID)
		setFailedCondition(
			&rsJob.Status.Conditions,
			err.Error(),
			rsJob.Generation)

		return err
	}

	reclaimedSpace := int64(0)
	if controllerFound && controllerReclaimedSpace != nil {
		reclaimedSpace += *controllerReclaimedSpace
	}
	if nodeFound && nodeReclaimedSpace != nil {
		reclaimedSpace += *nodeReclaimedSpace
	}

	rsJob.Status.Result = csiaddonsv1alpha1.OperationResultSucceeded
	rsJob.Status.Message = "Reclaim Space operation successfully completed."
	if nodeReclaimedSpace != nil || controllerReclaimedSpace != nil {
		rsJob.Status.ReclaimedSpace = resource.NewQuantity(reclaimedSpace, resource.DecimalSI)
	}
	rsJob.Status.CompletionTime = &metav1.Time{Time: time.Now()}
	logger.Info("Successfully completed reclaim space operation")

	return nil
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
		// Set global default timeout.
		timeout: r.Timeout,
	}
	for _, v := range volumeAttachments.Items {
		if v.DeletionTimestamp.IsZero() && v.Status.Attached && *v.Spec.Source.PersistentVolumeName == pv.Name {
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

// getLeadingRSClient returns ReclaimSpaceClient that holds the lease given driverName.
func (r *ReclaimSpaceJobReconciler) getLeadingRSClient(
	ctx context.Context,
	client client.Client,
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

	return nil, fmt.Errorf("leading controller for driver %q does not support ReclaimSpace_OFFLINE", driverName)
}

// getRSClientByNode returns ReclaimSpaceClient given driverName and nodeID.
func (r *ReclaimSpaceJobReconciler) getRSClientByNode(
	driverName, nodeID string) (string, proto.ReclaimSpaceClient, error) {
	conns, err := r.ConnPool.GetByNodeID(driverName, nodeID)
	if err != nil {
		return "", nil, err
	}
	for k, v := range conns {
		for _, cap := range v.Capabilities {
			if cap.GetReclaimSpace() == nil {
				continue
			}
			if cap.GetReclaimSpace().Type == identity.Capability_ReclaimSpace_ONLINE {
				return k, proto.NewReclaimSpaceClient(v.Client), nil
			}
		}
	}

	return "", nil, nil
}

// controllerReclaimSpace makes controller reclaim space request if controller client is found
// and returns amount of reclaimed space.
// This function returns
// - boolean to indicate client was found or not
// - pointer to int64 indicating amount of reclaimed space, it is nil if not available
// - error
func (r *ReclaimSpaceJobReconciler) controllerReclaimSpace(
	ctx context.Context,
	logger *logr.Logger,
	target *targetDetails) (bool, *int64, error) {
	controllerClient, err := r.getLeadingRSClient(ctx, r.Client, target.driverName)
	if err != nil {
		logger.Error(err, "Failed to get controller client", "driverName", target.driverName)
		return false, nil, err
	}

	logger.Info("Making controller reclaim space request")
	req := &proto.ReclaimSpaceRequest{
		PvName: target.pvName,
	}
	newCtx, cancel := context.WithTimeout(ctx, target.timeout)
	defer cancel()
	resp, err := controllerClient.ControllerReclaimSpace(newCtx, req)
	if err != nil {
		// Unimplemented suggests that the function is not supported
		if status.Code(err) == codes.Unimplemented {
			logger.Info(fmt.Sprintf("ControllerReclaimSpace is not implemented by driver: %v", err))
			return true, nil, nil
		}
		return true, nil, err
	}

	return true, calculateReclaimedSpace(resp.PreUsage, resp.PostUsage), nil
}

// nodeReclaimSpace makes node reclaim space request if node client is found
// and returns amount of reclaimed space.
// This function returns
// - pointer to int64 indicating amount of reclaimed space, it is nil if not available
// - error
func (r *ReclaimSpaceJobReconciler) nodeReclaimSpace(
	ctx context.Context,
	logger *logr.Logger,
	target *targetDetails) (*int64, error) {
	clientName, nodeClient, err := r.getRSClientByNode(
		target.driverName,
		target.nodeID)
	if err != nil {
		return nil, err
	}
	if nodeClient == nil {
		return nil, fmt.Errorf("node Client not found for %q nodeID", target.nodeID)
	}
	*logger = logger.WithValues("nodeClient", clientName)

	logger.Info("Making node reclaim space request")
	req := &proto.ReclaimSpaceRequest{
		PvName: target.pvName,
	}
	newCtx, cancel := context.WithTimeout(ctx, target.timeout)
	defer cancel()
	resp, err := nodeClient.NodeReclaimSpace(newCtx, req)
	if err != nil {
		// Unimplemented suggests that the function is not supported
		if status.Code(err) == codes.Unimplemented {
			logger.Info(fmt.Sprintf("NodeReclaimSpace is not implemented by driver: %v", err))
			return nil, nil
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

// setFailedCondition updates failedConditin type if it exists else
// appends a new condition.
func setFailedCondition(
	conditions *[]metav1.Condition,
	message string,
	observedGeneration int64) {
	newCondition := metav1.Condition{
		Type:               conditionFailed,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: observedGeneration,
		LastTransitionTime: metav1.NewTime(time.Now()),
		Message:            message,
		Reason:             reasonFailed,
	}
	for i := range *conditions {
		if (*conditions)[i].Type == conditionFailed {
			(*conditions)[i] = newCondition
			return
		}
	}

	*conditions = append(*conditions, newCondition)
}
