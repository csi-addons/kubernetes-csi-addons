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
	"errors"
	"fmt"
	"time"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/csiaddons/v1alpha1"
	"github.com/csi-addons/kubernetes-csi-addons/internal/connection"
	"github.com/csi-addons/kubernetes-csi-addons/internal/proto"
	"github.com/csi-addons/kubernetes-csi-addons/internal/util"
	"github.com/csi-addons/spec/lib/go/identity"

	"github.com/go-logr/logr"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	scv1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// EncryptionKeyRotationJobReconciler reconciles a EncryptionKeyRotationJob object
type EncryptionKeyRotationJobReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	ConnPool *connection.ConnectionPool
	Timeout  time.Duration
}

//+kubebuilder:rbac:groups=csiaddons.openshift.io,resources=encryptionkeyrotationjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=csiaddons.openshift.io,resources=encryptionkeyrotationjobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=csiaddons.openshift.io,resources=encryptionkeyrotationjobs/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;update
//+kubebuilder:rbac:groups="",resources=persistentvolumes,verbs=get;list;watch
//+kubebuilder:rbac:groups=storage.k8s.io,resources=volumeattachments,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *EncryptionKeyRotationJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the EncrytionKeyRotaionJob
	krJob := &csiaddonsv1alpha1.EncryptionKeyRotationJob{}
	err := r.Get(ctx, req.NamespacedName, krJob)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("encryptionkeyrotationjob resource not found")

			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	// Check if resource is being deleted
	if !krJob.DeletionTimestamp.IsZero() {
		logger.Info("encryptionkeyrotationjob is being deleted, exiting reconcile")
		return ctrl.Result{}, nil
	}

	// If already in progress, ignore
	if krJob.Status.Result != "" {
		logger.Info(fmt.Sprintf("encryptionkeyrotationjob is already in %q state", krJob.Status.Result))

		return ctrl.Result{}, nil
	}

	// Validate the spec
	err = validateEncryptionKeyRotationJobSpec(krJob)
	if err != nil {
		logger.Error(err, "failed to validate encryptionkeyrotationjob spec")

		krJob.Status.Result = csiaddonsv1alpha1.OperationResultFailed
		krJob.Status.Message = fmt.Sprintf("failed to validate encryptionkeyrotationjob spec: %v", err)
		krJob.Status.CompletionTime = &v1.Time{Time: time.Now()}
		if statusUpdErr := r.Status().Update(ctx, krJob); statusUpdErr != nil {
			logger.Error(statusUpdErr, "failed to update status")
			return ctrl.Result{}, statusUpdErr
		}

		return ctrl.Result{}, nil
	}

	// Set defaults for optional values
	if krJob.Spec.BackoffLimit == 0 {
		krJob.Spec.BackoffLimit = defaultBackoffLimit
	}
	if krJob.Spec.RetryDeadlineSeconds == 0 {
		krJob.Spec.RetryDeadlineSeconds = defaultRetryDeadlineSeconds
	}

	// Reconcile the resource
	err = r.reconcileEncryptionKeyRotationJob(
		ctx,
		&logger,
		krJob,
		req.Namespace,
	)

	if krJob.Status.Result == "" && krJob.Status.Retries == krJob.Spec.BackoffLimit {
		logger.Info("Maximum retry limit reached")
		krJob.Status.Result = csiaddonsv1alpha1.OperationResultFailed
		krJob.Status.Message = "Maximum retry limit reached"
		krJob.Status.CompletionTime = &v1.Time{Time: time.Now()}
	}

	if statusUpdErr := r.Status().Update(ctx, krJob); statusUpdErr != nil {
		logger.Error(statusUpdErr, "Failed to update status")

		return ctrl.Result{}, statusUpdErr
	}

	if krJob.Status.Result != "" {
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, err
}

// validateEncryptionKeyRotationJobSpec validates the encryptionkeyrotationjob
func validateEncryptionKeyRotationJobSpec(
	krJob *csiaddonsv1alpha1.EncryptionKeyRotationJob) error {
	if krJob.Spec.Target.PersistentVolumeClaim == "" {
		return errors.New("required parameter 'PersistentVolumeClaim' is missing")
	}

	return nil
}

// reconcileEncryptionKeyRotationJob reconciles the encryptionkeyrotationjob object
func (r *EncryptionKeyRotationJobReconciler) reconcileEncryptionKeyRotationJob(
	ctx context.Context,
	logger *logr.Logger,
	krJob *csiaddonsv1alpha1.EncryptionKeyRotationJob,
	ns string) error {

	if krJob.Status.StartTime == nil {
		krJob.Status.StartTime = &v1.Time{Time: time.Now()}
	} else {
		// This is a retry attempt
		krJob.Status.Retries++
	}

	// check whether currentTime > CreationTime + RetryDeadlineSeconds,
	// if true, mark it as Time limit reached and fail.
	if time.Now().After(krJob.CreationTimestamp.Add(time.Second * time.Duration(krJob.Spec.RetryDeadlineSeconds))) {
		logger.Info("Time limit reached")
		krJob.Status.Result = csiaddonsv1alpha1.OperationResultFailed
		krJob.Status.Message = "Time limit reached"
		krJob.Status.CompletionTime = &v1.Time{Time: time.Now()}

		return nil
	}

	// Get target details
	target, err := r.getTargetDetails(ctx, logger, krJob.Spec, ns)
	if err != nil {
		logger.Error(err, "Failed to get target details")
		setFailedCondition(
			&krJob.Status.Conditions,
			"Failed to get target details",
			krJob.Generation)

		return err
	}

	if target.nodeID == "" {
		return fmt.Errorf("unable to find nodeID for pv: %s", krJob.Spec.Target.PersistentVolumeClaim)
	}

	err = r.rotateEncryptionKey(ctx, logger, target)
	if err != nil {
		logger.Error(err, "failed to make gprc request for encryptionkeyrotation")
		setFailedCondition(
			&krJob.Status.Conditions,
			fmt.Sprintf("Failed to make node request: %v", util.GetErrorMessage(err)),
			krJob.Generation)

		return err
	}

	krJob.Status.Result = csiaddonsv1alpha1.OperationResultSucceeded
	krJob.Status.Message = "Key rotation operation successfully completed."
	krJob.Status.CompletionTime = &v1.Time{Time: time.Now()}
	logger.Info("Successfully completed key rotation operation")

	return nil
}

// getTargetDetails fetches driverName, pvName and nodeID in targetDetails struct.
func (r *EncryptionKeyRotationJobReconciler) getTargetDetails(
	ctx context.Context,
	logger *logr.Logger,
	spec csiaddonsv1alpha1.EncryptionKeyRotationJobSpec,
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

func (r *EncryptionKeyRotationJobReconciler) rotateEncryptionKey(
	ctx context.Context,
	logger *logr.Logger,
	target *targetDetails) error {
	clientName, client, err := r.getClientByNode(target.driverName, target.nodeID)
	if err != nil {
		return err
	}
	if client == nil {
		return fmt.Errorf("node client not found for node id: %s", target.nodeID)
	}

	*logger = logger.WithValues("nodeClient", clientName)
	logger.Info("calling encryptionkeyrotation grpc in sidecar")

	req := &proto.EncryptionKeyRotateRequest{
		PvName: target.pvName,
	}
	timedCtx, cFunc := context.WithTimeout(ctx, target.timeout)
	defer cFunc()

	_, err = client.EncryptionKeyRotate(timedCtx, req)
	if err != nil {
		if status.Code(err) == codes.Unimplemented {
			logger.Info("encryptionkeyrotation not implemented by driver")
			return nil
		}

		return err
	}

	return nil
}

func (r *EncryptionKeyRotationJobReconciler) getClientByNode(driver, nodeID string) (string, proto.EncryptionKeyRotationClient, error) {
	conns, err := r.ConnPool.GetByNodeID(driver, nodeID)
	if err != nil {
		return "", nil, err
	}
	for k, v := range conns {
		for _, cap := range v.Capabilities {
			if cap.GetEncryptionKeyRotation() == nil {
				continue
			}

			if cap.GetEncryptionKeyRotation().Type == identity.Capability_EncryptionKeyRotation_ENCRYPTIONKEYROTATION {
				return k, proto.NewEncryptionKeyRotationClient(v.Client), nil
			}
		}
	}

	return "", nil, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *EncryptionKeyRotationJobReconciler) SetupWithManager(mgr ctrl.Manager, ctrlOptions controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&csiaddonsv1alpha1.EncryptionKeyRotationJob{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		WithOptions(ctrlOptions).
		Complete(r)
}
