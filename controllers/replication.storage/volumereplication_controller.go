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
	"fmt"
	"time"

	replicationv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/apis/replication.storage/v1alpha1"
	"github.com/csi-addons/kubernetes-csi-addons/controllers/replication.storage/replication"
	grpcClient "github.com/csi-addons/kubernetes-csi-addons/internal/client"
	conn "github.com/csi-addons/kubernetes-csi-addons/internal/connection"
	"github.com/csi-addons/kubernetes-csi-addons/internal/proto"
	"github.com/csi-addons/kubernetes-csi-addons/internal/util"

	"github.com/csi-addons/spec/lib/go/identity"
	"github.com/go-logr/logr"
	"google.golang.org/grpc/codes"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	pvcDataSource          = "PersistentVolumeClaim"
	volumeReplicationClass = "VolumeReplicationClass"
	volumeReplication      = "VolumeReplication"
	defaultScheduleTime    = time.Hour
)

var (
	volumePromotionKnownErrors    = []codes.Code{codes.FailedPrecondition}
	disableReplicationKnownErrors = []codes.Code{codes.NotFound}
	getReplicationInfoKnownErrors = []codes.Code{codes.NotFound}
)

// VolumeReplicationReconciler reconciles a VolumeReplication object.
type VolumeReplicationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// ConnectionPool consists of map of Connection objects
	Connpool *conn.ConnectionPool
	// Timeout for the Reconcile operation.
	Timeout     time.Duration
	Replication grpcClient.VolumeReplication
}

// +kubebuilder:rbac:groups=replication.storage.openshift.io,resources=volumereplications,verbs=get;list;watch;update
// +kubebuilder:rbac:groups=replication.storage.openshift.io,resources=volumereplications/status,verbs=update
// +kubebuilder:rbac:groups=replication.storage.openshift.io,resources=volumereplications/finalizers,verbs=update
// +kubebuilder:rbac:groups=replication.storage.openshift.io,resources=volumereplicationclasses,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.0/pkg/reconcile
func (r *VolumeReplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx, "Request.Name", req.Name, "Request.Namespace", req.Namespace)

	// Fetch VolumeReplication instance
	instance := &replicationv1alpha1.VolumeReplication{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			logger.Info("volumeReplication resource not found")

			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Get VolumeReplicationClass
	vrcObj, err := r.getVolumeReplicationClass(logger, instance.Spec.VolumeReplicationClass)
	if err != nil {
		setFailureCondition(instance)
		uErr := r.updateReplicationStatus(instance, logger, getCurrentReplicationState(instance), err.Error())
		if uErr != nil {
			logger.Error(uErr, "failed to update volumeReplication status", "VRName", instance.Name)
		}

		return ctrl.Result{}, err
	}

	err = validatePrefixedParameters(vrcObj.Spec.Parameters)
	if err != nil {
		logger.Error(err, "failed to validate parameters of volumeReplicationClass", "VRCName", instance.Spec.VolumeReplicationClass)
		setFailureCondition(instance)
		uErr := r.updateReplicationStatus(instance, logger, getCurrentReplicationState(instance), err.Error())
		if uErr != nil {
			logger.Error(uErr, "failed to update volumeReplication status", "VRName", instance.Name)
		}

		return ctrl.Result{}, err
	}
	// remove the prefix keys in volume replication class parameters
	parameters := filterPrefixedParameters(replicationParameterPrefix, vrcObj.Spec.Parameters)

	// get secret
	secretName := vrcObj.Spec.Parameters[prefixedReplicationSecretNameKey]
	secretNamespace := vrcObj.Spec.Parameters[prefixedReplicationSecretNamespaceKey]

	var (
		volumeHandle string
		pvc          *corev1.PersistentVolumeClaim
		pv           *corev1.PersistentVolume
		pvErr        error
	)

	replicationHandle := instance.Spec.ReplicationHandle

	nameSpacedName := types.NamespacedName{Name: instance.Spec.DataSource.Name, Namespace: req.Namespace}
	switch instance.Spec.DataSource.Kind {
	case pvcDataSource:
		pvc, pv, pvErr = r.getPVCDataSource(logger, nameSpacedName)
		if pvErr != nil {
			logger.Error(pvErr, "failed to get PVC", "PVCName", instance.Spec.DataSource.Name)
			setFailureCondition(instance)
			uErr := r.updateReplicationStatus(instance, logger, getCurrentReplicationState(instance), pvErr.Error())
			if uErr != nil {
				logger.Error(uErr, "failed to update volumeReplication status", "VRName", instance.Name)
			}

			return ctrl.Result{}, pvErr
		}

		volumeHandle = pv.Spec.CSI.VolumeHandle
	default:
		err = fmt.Errorf("unsupported datasource kind")
		logger.Error(err, "given kind not supported", "Kind", instance.Spec.DataSource.Kind)
		setFailureCondition(instance)
		uErr := r.updateReplicationStatus(instance, logger, getCurrentReplicationState(instance), err.Error())
		if uErr != nil {
			logger.Error(uErr, "failed to update volumeReplication status", "VRName", instance.Name)
		}

		return ctrl.Result{}, nil
	}

	logger.Info("volume handle", "VolumeHandleName", volumeHandle)
	if replicationHandle != "" {
		logger.Info("Replication handle", "ReplicationHandleName", replicationHandle)
	}

	replicationClient, err := r.getReplicationClient(ctx, vrcObj.Spec.Provisioner)
	if err != nil {
		logger.Error(err, "Failed to get ReplicationClient")

		return ctrl.Result{}, err
	}

	vr := &volumeReplicationInstance{
		logger:   logger,
		instance: instance,
		commonRequestParameters: replication.CommonRequestParameters{
			VolumeID:        volumeHandle,
			ReplicationID:   replicationHandle,
			Parameters:      parameters,
			SecretName:      secretName,
			SecretNamespace: secretNamespace,
			Replication:     replicationClient,
		},
		force: false,
	}

	// check if the object is being deleted
	if instance.GetDeletionTimestamp().IsZero() {
		if err = r.addFinalizerToVR(logger, instance); err != nil {
			logger.Error(err, "Failed to add VolumeReplication finalizer")

			return reconcile.Result{}, err
		}

		err = r.annotatePVCWithOwner(ctx, logger, req.Name, pvc)
		if err != nil {
			logger.Error(err, "Failed to annotate PVC owner")
			return ctrl.Result{}, err
		}

		if err = r.addFinalizerToPVC(logger, pvc); err != nil {
			logger.Error(err, "Failed to add PersistentVolumeClaim finalizer")

			return reconcile.Result{}, err
		}
	} else {
		if util.ContainsInSlice(instance.GetFinalizers(), volumeReplicationFinalizer) {
			err = r.disableVolumeReplication(vr)
			if err != nil {
				logger.Error(err, "failed to disable replication")

				return ctrl.Result{}, err
			}

			if err = r.removeOwnerFromPVCAnnotation(ctx, logger, pvc); err != nil {
				logger.Error(err, "Failed to remove VolumeReplication annotation from PersistentVolumeClaim")

				return reconcile.Result{}, err
			}

			if err = r.removeFinalizerFromPVC(logger, pvc); err != nil {
				logger.Error(err, "Failed to remove PersistentVolumeClaim finalizer")

				return reconcile.Result{}, err
			}

			// once all finalizers have been removed, the object will be
			// deleted
			if err = r.removeFinalizerFromVR(logger, instance); err != nil {
				logger.Error(err, "Failed to remove VolumeReplication finalizer")

				return reconcile.Result{}, err
			}
		}
		logger.Info("volumeReplication object is terminated, skipping reconciliation")

		return ctrl.Result{}, nil
	}

	instance.Status.LastStartTime = getCurrentTime()
	if err = r.Client.Update(context.TODO(), instance); err != nil {
		logger.Error(err, "failed to update status")

		return reconcile.Result{}, err
	}

	var replicationErr error
	var requeueForResync bool

	switch instance.Spec.ReplicationState {
	case replicationv1alpha1.Primary:
		// Avoid extra RPC calls as request will be requested again for
		// updating the LastSyncTime in the status. The volume needs to be
		// replication enabled and promoted only once, not always during
		// each reconciliation.
		if instance.Status.State != replicationv1alpha1.PrimaryState {
			// enable replication only if its not primary
			if err = r.enableReplication(vr); err != nil {
				logger.Error(err, "failed to enable replication")
				setFailureCondition(instance)
				msg := replication.GetMessageFromError(err)
				uErr := r.updateReplicationStatus(instance, logger, getCurrentReplicationState(instance), msg)
				if uErr != nil {
					logger.Error(uErr, "failed to update volumeReplication status", "VRName", instance.Name)
				}

				return reconcile.Result{}, err
			}
			replicationErr = r.markVolumeAsPrimary(vr)
		}

	case replicationv1alpha1.Secondary:
		// For the first time, mark the volume as secondary and requeue the
		// request. For some storage providers it takes some time to determine
		// whether the volume need correction example:- correcting split brain.
		if instance.Status.State != replicationv1alpha1.SecondaryState {
			replicationErr = r.markVolumeAsSecondary(vr)
			if replicationErr == nil {
				logger.Info("volume is not ready to use")
				// set the status.State to secondary as the
				// instance.Status.State is primary for the first time.
				err = r.updateReplicationStatus(instance, logger, getReplicationState(instance), "volume is marked secondary and is degraded")
				if err != nil {
					return ctrl.Result{}, err
				}

				return ctrl.Result{
					Requeue: true,
					// Setting Requeue time for 15 seconds
					RequeueAfter: time.Second * 15,
				}, nil
			}
		} else {
			replicationErr = r.markVolumeAsSecondary(vr)
			// resync volume if successfully marked Secondary
			if replicationErr == nil {
				vr.force = vr.instance.Spec.AutoResync
				requeueForResync, replicationErr = r.resyncVolume(vr)
			}
		}

	case replicationv1alpha1.Resync:
		vr.force = true
		requeueForResync, replicationErr = r.resyncVolume(vr)

	default:
		replicationErr = fmt.Errorf("unsupported volume state")
		logger.Error(replicationErr, "given volume state is not supported", "ReplicationState", instance.Spec.ReplicationState)
		setFailureCondition(instance)
		err = r.updateReplicationStatus(instance, logger, getCurrentReplicationState(instance), replicationErr.Error())
		if err != nil {
			logger.Error(err, "failed to update volumeReplication status", "VRName", instance.Name)
		}

		return ctrl.Result{}, nil
	}

	if replicationErr != nil {
		msg := replication.GetMessageFromError(replicationErr)
		logger.Error(replicationErr, "failed to Replicate", "ReplicationState", instance.Spec.ReplicationState)
		err = r.updateReplicationStatus(instance, logger, getCurrentReplicationState(instance), msg)
		if err != nil {
			logger.Error(err, "failed to update volumeReplication status", "VRName", instance.Name)
		}

		if instance.Status.State == replicationv1alpha1.SecondaryState {
			return ctrl.Result{
				Requeue: true,
				// in case of any error during secondary state, requeue for every 15 seconds.
				RequeueAfter: time.Second * 15,
			}, nil
		}

		return ctrl.Result{}, replicationErr
	}

	if requeueForResync {
		logger.Info("volume is not ready to use, requeuing for resync")

		err = r.updateReplicationStatus(instance, logger, getCurrentReplicationState(instance), "volume is degraded")
		if err != nil {
			logger.Error(err, "failed to update volumeReplication status", "VRName", instance.Name)
		}

		return ctrl.Result{
			Requeue: true,
			// Setting Requeue time for 30 seconds as the resync can take time
			// and having default Requeue exponential backoff time can affect
			// the RTO time.
			RequeueAfter: time.Second * 30,
		}, nil
	}

	var msg string
	if instance.Spec.ReplicationState == replicationv1alpha1.Resync {
		msg = "volume is marked for resyncing"
	} else {
		msg = fmt.Sprintf("volume is marked %s", string(instance.Spec.ReplicationState))
	}

	instance.Status.LastCompletionTime = getCurrentTime()

	requeueForInfo := false

	if instance.Spec.ReplicationState == replicationv1alpha1.Primary {
		info, err := r.getVolumeReplicationInfo(vr)
		if err == nil {
			ts := info.GetLastSyncTime()
			if ts != nil {
				lastSyncTime := metav1.NewTime(ts.AsTime())
				instance.Status.LastSyncTime = &lastSyncTime
				instance.Status.LastSyncDuration = nil
				instance.Status.LastSyncBytes = nil
				td := info.GetLastSyncDuration()
				if td != nil {
					lastSyncDuration := metav1.Duration{Duration: td.AsDuration()}
					instance.Status.LastSyncDuration = &lastSyncDuration
				}
				tb := info.GetLastSyncBytes()
				if tb != 0 {
					instance.Status.LastSyncBytes = &tb
				}
			}
			requeueForInfo = true
		} else if !util.IsUnimplementedError(err) {
			logger.Error(err, "Failed to get volume replication info")
			return ctrl.Result{}, err
		}
	}
	if instance.Spec.ReplicationState == replicationv1alpha1.Secondary {
		instance.Status.LastSyncTime = nil
	}
	err = r.updateReplicationStatus(instance, logger, getReplicationState(instance), msg)
	if err != nil {
		return ctrl.Result{}, err
	}

	logger.Info(msg)

	if requeueForInfo {
		reconcileInternal := getInfoReconcileInterval(parameters, logger)
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: reconcileInternal,
		}, nil
	}

	return ctrl.Result{}, nil
}

// getInfoReconcileInterval takes parameters and returns the half of the scheduling
// interval after converting it to time.Duration. The interval is converted
// into half to keep the LastSyncTime and Storage LastSyncTime to be in sync.
// If the schedulingInterval is empty or there is error parsing, it is set to
// the default value.
func getInfoReconcileInterval(parameters map[string]string, logger logr.Logger) time.Duration {
	// the schedulingInterval looks like below, which is the part of volumereplicationclass
	// and is an optional parameter.
	// ```parameters:
	// 		replication.storage.openshift.io/replication-secret-name: rook-csi-rbd-provisioner
	//		replication.storage.openshift.io/replication-secret-namespace: rook-ceph
	//		schedulingInterval: 1m```
	rawScheduleTime := parameters["schedulingInterval"]
	if rawScheduleTime == "" {
		return defaultScheduleTime
	}
	scheduleTime, err := time.ParseDuration(rawScheduleTime)
	if err != nil {
		logger.Error(err, "failed to parse time: %v", rawScheduleTime)
		return defaultScheduleTime
	}
	// Return schedule internal to avoid frequent reconcile if the
	// schedulingInterval is less than 2 minutes.
	if scheduleTime < 2*time.Minute {
		logger.Info("scheduling interval is less than 2 minutes, not decreasing it to half")
		return scheduleTime
	}

	// Reduce the schedule time by half to get the latest update and also to
	// avoid the inconsistency between the last sync time in the VR and the
	// Storage system.
	return scheduleTime / 2
}

func (r *VolumeReplicationReconciler) getReplicationClient(ctx context.Context, driverName string) (grpcClient.VolumeReplication, error) {
	conn, err := r.Connpool.GetLeaderByDriver(ctx, r.Client, driverName)
	if err != nil {
		return nil, fmt.Errorf("no leader for the ControllerService of driver %q", driverName)
	}

	for _, cap := range conn.Capabilities {
		// validate if VOLUME_REPLICATION capability is supported by the driver.
		if cap.GetVolumeReplication() == nil {
			continue
		}

		// validate of VOLUME_REPLICATION capability is enabled by the storage driver.
		if cap.GetVolumeReplication().GetType() == identity.Capability_VolumeReplication_VOLUME_REPLICATION {
			return grpcClient.NewReplicationClient(conn.Client, r.Timeout), nil
		}
	}

	return nil, fmt.Errorf("leading CSIAddonsNode %q for driver %q does not support VolumeReplication", conn.Name, driverName)

}

func (r *VolumeReplicationReconciler) updateReplicationStatus(
	instance *replicationv1alpha1.VolumeReplication,
	logger logr.Logger,
	state replicationv1alpha1.State,
	message string) error {
	instance.Status.State = state
	instance.Status.Message = message
	instance.Status.ObservedGeneration = instance.Generation
	if err := r.Client.Status().Update(context.TODO(), instance); err != nil {
		logger.Error(err, "failed to update status")

		return err
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *VolumeReplicationReconciler) SetupWithManager(mgr ctrl.Manager, ctrlOptions controller.Options) error {
	err := r.waitForCrds()
	if err != nil {

		return err
	}

	pred := predicate.GenerationChangedPredicate{}

	return ctrl.NewControllerManagedBy(mgr).
		For(&replicationv1alpha1.VolumeReplication{}).
		WithEventFilter(pred).
		WithOptions(ctrlOptions).
		Complete(r)
}

func (r *VolumeReplicationReconciler) waitForCrds() error {
	logger := log.FromContext(context.TODO(), "Name", "checkingDependencies")

	err := r.waitForVolumeReplicationResource(logger, volumeReplicationClass)
	if err != nil {
		logger.Error(err, "failed to wait for VolumeReplicationClass CRD")

		return err
	}

	err = r.waitForVolumeReplicationResource(logger, volumeReplication)
	if err != nil {
		logger.Error(err, "failed to wait for VolumeReplication CRD")

		return err
	}

	return nil
}

func (r *VolumeReplicationReconciler) waitForVolumeReplicationResource(logger logr.Logger, resourceName string) error {
	unstructuredResource := &unstructured.UnstructuredList{}
	unstructuredResource.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   replicationv1alpha1.GroupVersion.Group,
		Kind:    resourceName,
		Version: replicationv1alpha1.GroupVersion.Version,
	})
	for {
		err := r.Client.List(context.TODO(), unstructuredResource)
		if err == nil {
			return nil
		}
		// return errors other than NoMatch
		if !meta.IsNoMatchError(err) {
			logger.Error(err, "got an unexpected error while waiting for resource", "Resource", resourceName)

			return err
		}
		logger.Info("resource does not exist", "Resource", resourceName)
		time.Sleep(5 * time.Second)
	}
}

// volumeReplicationInstance contains the attributes
// that can be useful in reconciling a particular
// instance of the VolumeReplication resource.
type volumeReplicationInstance struct {
	logger                  logr.Logger
	instance                *replicationv1alpha1.VolumeReplication
	commonRequestParameters replication.CommonRequestParameters
	force                   bool
}

// markVolumeAsPrimary defines and runs a set of tasks required to mark a volume as primary.
func (r *VolumeReplicationReconciler) markVolumeAsPrimary(vr *volumeReplicationInstance) error {
	volumeReplication := replication.Replication{
		Params: vr.commonRequestParameters,
	}

	resp := volumeReplication.Promote()
	if resp.Error != nil {
		isKnownError := resp.HasKnownGRPCError(volumePromotionKnownErrors)
		if !isKnownError {
			if resp.Error != nil {
				vr.logger.Error(resp.Error, "failed to promote volume")
				setFailedPromotionCondition(&vr.instance.Status.Conditions, vr.instance.Generation)

				return resp.Error
			}
		} else {
			// force promotion
			vr.logger.Info("force promoting volume due to known grpc error", "error", resp.Error)
			volumeReplication.Force = true
			resp := volumeReplication.Promote()
			if resp.Error != nil {
				vr.logger.Error(resp.Error, "failed to force promote volume")
				setFailedPromotionCondition(&vr.instance.Status.Conditions, vr.instance.Generation)

				return resp.Error
			}
		}
	}

	setPromotedCondition(&vr.instance.Status.Conditions, vr.instance.Generation)

	return nil
}

// markVolumeAsSecondary defines and runs a set of tasks required to mark a volume as secondary.
func (r *VolumeReplicationReconciler) markVolumeAsSecondary(vr *volumeReplicationInstance) error {
	volumeReplication := replication.Replication{
		Params: vr.commonRequestParameters,
	}

	resp := volumeReplication.Demote()

	if resp.Error != nil {
		vr.logger.Error(resp.Error, "failed to demote volume")
		setFailedDemotionCondition(&vr.instance.Status.Conditions, vr.instance.Generation)

		return resp.Error
	}

	setDemotedCondition(&vr.instance.Status.Conditions, vr.instance.Generation)

	return nil
}

// resyncVolume defines and runs a set of tasks required to resync the volume.
func (r *VolumeReplicationReconciler) resyncVolume(vr *volumeReplicationInstance) (bool, error) {
	volumeReplication := replication.Replication{
		Params: vr.commonRequestParameters,
		Force:  vr.force,
	}

	resp := volumeReplication.Resync()

	if resp.Error != nil {
		vr.logger.Error(resp.Error, "failed to resync volume")
		setFailedResyncCondition(&vr.instance.Status.Conditions, vr.instance.Generation)

		return false, resp.Error
	}
	resyncResponse, ok := resp.Response.(*proto.ResyncVolumeResponse)
	if !ok {
		err := fmt.Errorf("received response of unexpected type")
		vr.logger.Error(err, "unable to parse response")
		setFailedResyncCondition(&vr.instance.Status.Conditions, vr.instance.Generation)

		return false, err
	}

	setResyncCondition(&vr.instance.Status.Conditions, vr.instance.Generation)

	if !resyncResponse.GetReady() {
		return true, nil
	}

	// No longer degraded, as volume is fully synced
	setNotDegradedCondition(&vr.instance.Status.Conditions, vr.instance.Generation)

	return false, nil
}

// disableVolumeReplication defines and runs a set of tasks required to disable volume replication.
func (r *VolumeReplicationReconciler) disableVolumeReplication(vr *volumeReplicationInstance) error {
	volumeReplication := replication.Replication{
		Params: vr.commonRequestParameters,
	}

	resp := volumeReplication.Disable()

	if resp.Error != nil {
		if isKnownError := resp.HasKnownGRPCError(disableReplicationKnownErrors); isKnownError {
			vr.logger.Info("volume not found", "volumeID", vr.commonRequestParameters.VolumeID)

			return nil
		}
		vr.logger.Error(resp.Error, "failed to disable volume replication")

		return resp.Error
	}

	return nil
}

// enableReplication enable volume replication on the first reconcile.
func (r *VolumeReplicationReconciler) enableReplication(vr *volumeReplicationInstance) error {
	volumeReplication := replication.Replication{
		Params: vr.commonRequestParameters,
	}

	resp := volumeReplication.Enable()

	if resp.Error != nil {
		vr.logger.Error(resp.Error, "failed to enable volume replication")

		return resp.Error
	}

	return nil
}

// getVolumeReplicationInfo gets volume replication info.
func (r *VolumeReplicationReconciler) getVolumeReplicationInfo(vr *volumeReplicationInstance) (*proto.GetVolumeReplicationInfoResponse, error) {
	volumeReplication := replication.Replication{
		Params: vr.commonRequestParameters,
	}

	resp := volumeReplication.GetInfo()
	if resp.Error != nil {
		vr.logger.Error(resp.Error, "failed to get volume replication info")
		if isKnownError := resp.HasKnownGRPCError(getReplicationInfoKnownErrors); isKnownError {
			vr.logger.Info("volume/replication info not found", "volumeID", vr.commonRequestParameters.VolumeID)

			return nil, nil
		}
		return nil, resp.Error
	}

	infoResponse, ok := resp.Response.(*proto.GetVolumeReplicationInfoResponse)
	if !ok {
		err := fmt.Errorf("received response of unexpected type")
		vr.logger.Error(err, "unable to parse response")

		return nil, err
	}

	return infoResponse, nil
}

func getReplicationState(instance *replicationv1alpha1.VolumeReplication) replicationv1alpha1.State {
	switch instance.Spec.ReplicationState {
	case replicationv1alpha1.Primary:
		return replicationv1alpha1.PrimaryState
	case replicationv1alpha1.Secondary:
		return replicationv1alpha1.SecondaryState
	case replicationv1alpha1.Resync:
		return replicationv1alpha1.SecondaryState
	}

	return replicationv1alpha1.UnknownState
}

func getCurrentReplicationState(instance *replicationv1alpha1.VolumeReplication) replicationv1alpha1.State {
	if instance.Status.State == "" {
		return replicationv1alpha1.UnknownState
	}

	return instance.Status.State
}

func setFailureCondition(instance *replicationv1alpha1.VolumeReplication) {
	switch instance.Spec.ReplicationState {
	case replicationv1alpha1.Primary:
		setFailedPromotionCondition(&instance.Status.Conditions, instance.Generation)
	case replicationv1alpha1.Secondary:
		setFailedDemotionCondition(&instance.Status.Conditions, instance.Generation)
	case replicationv1alpha1.Resync:
		setFailedResyncCondition(&instance.Status.Conditions, instance.Generation)
	}
}

func getCurrentTime() *metav1.Time {
	metav1NowTime := metav1.NewTime(time.Now())

	return &metav1NowTime
}
