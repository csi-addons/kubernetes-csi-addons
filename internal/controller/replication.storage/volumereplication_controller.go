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
	stderrors "errors"
	"fmt"
	"slices"
	"time"

	replicationv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/replication.storage/v1alpha1"
	grpcClient "github.com/csi-addons/kubernetes-csi-addons/internal/client"
	conn "github.com/csi-addons/kubernetes-csi-addons/internal/connection"
	"github.com/csi-addons/kubernetes-csi-addons/internal/controller/replication.storage/replication"
	"github.com/csi-addons/kubernetes-csi-addons/internal/proto"
	"github.com/csi-addons/kubernetes-csi-addons/internal/util"

	"github.com/csi-addons/spec/lib/go/identity"
	"github.com/go-logr/logr"
	"google.golang.org/grpc/codes"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	pvcDataSource                    = "PersistentVolumeClaim"
	volumeGroupReplicationDataSource = "VolumeGroupReplication"
	volumeReplicationClass           = "VolumeReplicationClass"
	volumeReplication                = "VolumeReplication"
	defaultScheduleTime              = time.Hour
)

var (
	volumePromotionKnownErrors    = []codes.Code{codes.FailedPrecondition}
	disableReplicationKnownErrors = []codes.Code{codes.NotFound}
	enableReplicationKnownErrors  = []codes.Code{codes.FailedPrecondition}
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

//+kubebuilder:rbac:groups=replication.storage.openshift.io,resources=volumereplications,verbs=get;list;watch;update
//+kubebuilder:rbac:groups=replication.storage.openshift.io,resources=volumereplications/status,verbs=update
//+kubebuilder:rbac:groups=replication.storage.openshift.io,resources=volumereplications/finalizers,verbs=update
//+kubebuilder:rbac:groups=replication.storage.openshift.io,resources=volumereplicationclasses,verbs=get;list;watch
//+kubebuilder:rbac:groups=replication.storage.openshift.io,resources=volumegroupreplications,verbs=get;list;watch
//+kubebuilder:rbac:groups=replication.storage.openshift.io,resources=volumegroupreplications/finalizers,verbs=update
//+kubebuilder:rbac:groups=replication.storage.openshift.io,resources=volumegroupreplicationcontents,verbs=get;list;watch
//+kubebuilder:rbac:groups=replication.storage.openshift.io,resources=volumegroupreplicationcontents/finalizers,verbs=update
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
	err := r.Get(context.TODO(), req.NamespacedName, instance)
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
		setFailureCondition(instance, "failed to get volumeReplication class", err.Error(), instance.Spec.DataSource.Kind)
		uErr := r.updateReplicationStatus(instance, logger, GetCurrentReplicationState(instance.Status.State), err.Error())
		if uErr != nil {
			logger.Error(uErr, "failed to update volumeReplication status", "VRName", instance.Name)
		}

		return ctrl.Result{}, err
	}

	err = validatePrefixedParameters(vrcObj.Spec.Parameters)
	if err != nil {
		logger.Error(err, "failed to validate parameters of volumeReplicationClass", "VRCName", instance.Spec.VolumeReplicationClass)
		setFailureCondition(instance, "failed to validate parameters of volumeReplicationClass", err.Error(), instance.Spec.DataSource.Kind)
		uErr := r.updateReplicationStatus(instance, logger, GetCurrentReplicationState(instance.Status.State), err.Error())
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
		// var for pvc replication
		volumeHandle string
		pvc          *corev1.PersistentVolumeClaim
		pv           *corev1.PersistentVolume
		pvErr        error
		// var for volume group replication
		groupHandle string
		vgrc        *replicationv1alpha1.VolumeGroupReplicationContent
		vgr         *replicationv1alpha1.VolumeGroupReplication
		vgrErr      error
	)

	replicationHandle := instance.Spec.ReplicationHandle

	nameSpacedName := types.NamespacedName{Name: instance.Spec.DataSource.Name, Namespace: req.Namespace}
	switch instance.Spec.DataSource.Kind {
	case pvcDataSource:
		pvc, pv, pvErr = r.getPVCDataSource(ctx, logger, nameSpacedName)
		if pvErr != nil {
			logger.Error(pvErr, "failed to get PVC", "PVCName", instance.Spec.DataSource.Name)
			setFailureCondition(instance, "failed to find PVC", pvErr.Error(), instance.Spec.DataSource.Name)
			uErr := r.updateReplicationStatus(instance, logger, GetCurrentReplicationState(instance.Status.State), pvErr.Error())
			if uErr != nil {
				logger.Error(uErr, "failed to update volumeReplication status", "VRName", instance.Name)
			}

			return ctrl.Result{}, pvErr
		}

		volumeHandle = pv.Spec.CSI.VolumeHandle
		logger.Info("volume handle", "VolumeHandleName", volumeHandle)
	case volumeGroupReplicationDataSource:
		vgr, vgrc, vgrErr = r.getVolumeGroupReplicationDataSource(logger, nameSpacedName)
		if vgrErr != nil {
			if errors.IsNotFound(vgrErr) && !instance.DeletionTimestamp.IsZero() {
				logger.Info("volumeGroupReplication resource not found, as volumeReplication resource is getting garbage collected")
				break
			}
			logger.Error(vgrErr, "failed to get VolumeGroupReplication", "VGRName", instance.Spec.DataSource.Name)
			setFailureCondition(instance, "failed to get VolumeGroupReplication", vgrErr.Error(), instance.Spec.DataSource.Name)
			uErr := r.updateReplicationStatus(instance, logger, GetCurrentReplicationState(instance.Status.State), vgrErr.Error())
			if uErr != nil {
				logger.Error(uErr, "failed to update volumeReplication status", "VRName", instance.Name)
			}
			return ctrl.Result{}, vgrErr
		}
		groupHandle = vgrc.Spec.VolumeGroupReplicationHandle
		logger.Info("Group handle", "GroupHandleName", groupHandle)
	default:
		err = fmt.Errorf("unsupported datasource kind")
		logger.Error(err, "given kind not supported", "Kind", instance.Spec.DataSource.Kind)
		setFailureCondition(instance, "unsupported datasource", err.Error(), "")
		uErr := r.updateReplicationStatus(instance, logger, GetCurrentReplicationState(instance.Status.State), err.Error())
		if uErr != nil {
			logger.Error(uErr, "failed to update volumeReplication status", "VRName", instance.Name)
		}

		return ctrl.Result{}, nil
	}

	if replicationHandle != "" {
		logger.Info("Replication handle", "ReplicationHandleName", replicationHandle)
	}

	replicationClient, err := r.getReplicationClient(ctx, vrcObj.Spec.Provisioner, instance.Spec.DataSource.Kind)
	if err != nil {
		logger.Error(err, "Failed to get ReplicationClient")

		return ctrl.Result{}, err
	}

	vr := &volumeReplicationInstance{
		logger:   logger,
		instance: instance,
		commonRequestParameters: replication.CommonRequestParameters{
			VolumeID:        volumeHandle,
			GroupID:         groupHandle,
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
		switch instance.Spec.DataSource.Kind {
		case pvcDataSource:
			reqOwner := fmt.Sprintf("%s/%s", instance.Namespace, instance.Name)
			err = annotatePVCWithOwner(r.Client, ctx, logger, reqOwner, pvc, replicationv1alpha1.VolumeReplicationNameAnnotation)
			if err != nil {
				logger.Error(err, "Failed to annotate PVC owner")
				return ctrl.Result{}, err
			}

			if err = addFinalizerToPVC(r.Client, logger, pvc, pvcReplicationFinalizer); err != nil {
				logger.Error(err, "Failed to add PersistentVolumeClaim finalizer")
				return ctrl.Result{}, err
			}
		case volumeGroupReplicationDataSource:
			err = r.annotateVolumeGroupReplicationWithOwner(ctx, logger, req.Name, vgr)
			if err != nil {
				logger.Error(err, "Failed to annotate VolumeGroupReplication owner")
				return ctrl.Result{}, err
			}

			if err = addFinalizerToVGRContent(r.Client, logger, vgrc, volumeReplicationFinalizer); err != nil {
				logger.Error(err, "Failed to add VolumeReplication finalizer to VolumeGroupReplicationContent")
				return reconcile.Result{}, err
			}
		}
	} else {
		if slices.Contains(instance.GetFinalizers(), volumeReplicationFinalizer) {
			// The image/group doesn't exist, so we shouldn't bother disabling mirroring for it
			if vr.commonRequestParameters.VolumeID != "" || vr.commonRequestParameters.GroupID != "" {
				err = r.disableVolumeReplication(vr)
				if err != nil {
					logger.Error(err, "failed to disable replication")
					return ctrl.Result{}, err
				}
			}
			switch instance.Spec.DataSource.Kind {
			case pvcDataSource:
				if err = removeOwnerFromPVCAnnotation(r.Client, ctx, logger, pvc, replicationv1alpha1.VolumeReplicationNameAnnotation); err != nil {
					logger.Error(err, "Failed to remove VolumeReplication annotation from PersistentVolumeClaim")
					return reconcile.Result{}, err
				}

				if err = removeFinalizerFromPVC(r.Client, logger, pvc, pvcReplicationFinalizer); err != nil {
					logger.Error(err, "Failed to remove PersistentVolumeClaim finalizer")
					return reconcile.Result{}, err
				}
			case volumeGroupReplicationDataSource:
				if err = removeFinalizerFromVGRContent(r.Client, logger, vgrc, volumeReplicationFinalizer); err != nil {
					logger.Error(err, "Failed to remove VolumeReplication finalizer from VolumeGroupReplicationContent resource")
					return reconcile.Result{}, err
				}

				err = r.removeOwnerFromVGRAnnotation(ctx, logger, vgr)
				if err != nil {
					logger.Error(err, "Failed to remove VolumeReplication owner annotation from VolumeGroupReplication resource")
					return reconcile.Result{}, err
				}
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
	if err = r.Update(context.TODO(), instance); err != nil {
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
				msg := replication.GetMessageFromError(err)
				uErr := r.updateReplicationStatus(instance, logger, GetCurrentReplicationState(instance.Status.State), msg)
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
				err = r.updateReplicationStatus(instance, logger, GetReplicationState(instance.Spec.ReplicationState), "volume is marked secondary and is degraded")
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
		setFailureCondition(instance, "unsupported volume state", replicationErr.Error(), instance.Spec.DataSource.Kind)
		err = r.updateReplicationStatus(instance, logger, GetCurrentReplicationState(instance.Status.State), replicationErr.Error())
		if err != nil {
			logger.Error(err, "failed to update volumeReplication status", "VRName", instance.Name)
		}

		return ctrl.Result{}, nil
	}

	if replicationErr != nil {
		msg := replication.GetMessageFromError(replicationErr)
		logger.Error(replicationErr, "failed to Replicate", "ReplicationState", instance.Spec.ReplicationState)
		err = r.updateReplicationStatus(instance, logger, GetCurrentReplicationState(instance.Status.State), msg)
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

		return reconcile.Result{}, replicationErr
	}

	if requeueForResync {
		logger.Info("volume is not ready to use, requeuing for resync")

		err = r.updateReplicationStatus(instance, logger, GetCurrentReplicationState(instance.Status.State), "volume is degraded")
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
	err = r.updateReplicationStatus(instance, logger, GetReplicationState(instance.Spec.ReplicationState), msg)
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

func (r *VolumeReplicationReconciler) getReplicationClient(ctx context.Context, driverName, dataSource string) (grpcClient.VolumeReplication, error) {
	conn, err := r.Connpool.GetLeaderByDriver(ctx, r.Client, driverName)
	if err != nil {
		return nil, fmt.Errorf("no leader for the ControllerService of driver %q: %w", driverName, err)
	}

	for _, cap := range conn.Capabilities {
		// validate if VOLUME_REPLICATION capability is supported by the driver.
		if cap.GetVolumeReplication() == nil {
			continue
		}

		// validate of VOLUME_REPLICATION capability is enabled by the storage driver.
		if cap.GetVolumeReplication().GetType() == identity.Capability_VolumeReplication_VOLUME_REPLICATION {
			switch dataSource {
			case pvcDataSource:
				return grpcClient.NewVolumeReplicationClient(conn.Client, r.Timeout), nil
			case volumeGroupReplicationDataSource:
				return grpcClient.NewVolumeGroupReplicationClient(conn.Client, r.Timeout), nil
			}
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
	if err := r.Status().Update(context.TODO(), instance); err != nil {
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

	err := WaitForVolumeReplicationResource(r.Client, logger, volumeReplicationClass)
	if err != nil {
		logger.Error(err, "failed to wait for VolumeReplicationClass CRD")
		return err
	}

	err = WaitForVolumeReplicationResource(r.Client, logger, volumeReplication)
	if err != nil {
		logger.Error(err, "failed to wait for VolumeReplication CRD")
		return err
	}

	return nil
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
				setFailedPromotionCondition(&vr.instance.Status.Conditions, vr.instance.Generation, vr.instance.Spec.DataSource.Kind, "failed to promote volume", resp.Error.Error())

				return resp.Error
			}
		} else {
			// force promotion
			vr.logger.Info("force promoting volume due to known grpc error", "error", resp.Error)
			volumeReplication.Force = true
			resp := volumeReplication.Promote()
			if resp.Error != nil {
				vr.logger.Error(resp.Error, "failed to force promote volume")
				setFailedPromotionCondition(&vr.instance.Status.Conditions, vr.instance.Generation, vr.instance.Spec.DataSource.Kind, "failed to force promote volume", resp.Error.Error())

				return resp.Error
			}
		}
	}

	setPromotedCondition(&vr.instance.Status.Conditions, vr.instance.Generation, vr.instance.Spec.DataSource.Kind)

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
		setFailedDemotionCondition(&vr.instance.Status.Conditions, vr.instance.Generation, vr.instance.Spec.DataSource.Kind, "failed to demote volume", resp.Error.Error())

		return resp.Error
	}

	setDemotedCondition(&vr.instance.Status.Conditions, vr.instance.Generation, vr.instance.Spec.DataSource.Kind)

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
		setFailedResyncCondition(&vr.instance.Status.Conditions, vr.instance.Generation, vr.instance.Spec.DataSource.Kind, "failed to resync volume", resp.Error.Error())

		return false, resp.Error
	}
	resyncResponse, ok := resp.Response.(*proto.ResyncVolumeResponse)
	if !ok {
		err := fmt.Errorf("received response of unexpected type")
		vr.logger.Error(err, "unable to parse response")
		setFailedResyncCondition(&vr.instance.Status.Conditions, vr.instance.Generation, vr.instance.Spec.DataSource.Kind, "unable to parse resync response", "received response of unexpected type")

		return false, err
	}

	setResyncCondition(&vr.instance.Status.Conditions, vr.instance.Generation, vr.instance.Spec.DataSource.Kind)

	if !resyncResponse.GetReady() {
		return true, nil
	}

	// No longer degraded, as volume is fully synced
	setNotDegradedCondition(&vr.instance.Status.Conditions, vr.instance.Generation, vr.instance.Spec.DataSource.Kind)

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

	if resp.Error == nil {
		return nil
	}

	vr.logger.Error(resp.Error, "failed to enable volume replication")

	if resp.HasKnownGRPCError(enableReplicationKnownErrors) {
		setFailedValidationCondition(&vr.instance.Status.Conditions, vr.instance.Generation, vr.instance.Spec.DataSource.Kind, "failed to enable volume replication", resp.Error.Error())
	} else {
		setFailedPromotionCondition(&vr.instance.Status.Conditions, vr.instance.Generation, vr.instance.Spec.DataSource.Kind, "failed to enable volume replication", resp.Error.Error())
	}
	return resp.Error
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

func setFailureCondition(instance *replicationv1alpha1.VolumeReplication, errMessage string, errFromCephCSI string, dataSource string) {
	switch instance.Spec.ReplicationState {
	case replicationv1alpha1.Primary:
		setFailedPromotionCondition(&instance.Status.Conditions, instance.Generation, dataSource, errMessage, errFromCephCSI)
	case replicationv1alpha1.Secondary:
		setFailedDemotionCondition(&instance.Status.Conditions, instance.Generation, dataSource, errMessage, errFromCephCSI)
	case replicationv1alpha1.Resync:
		setFailedResyncCondition(&instance.Status.Conditions, instance.Generation, dataSource, errMessage, errFromCephCSI)
	}
}

func getCurrentTime() *metav1.Time {
	metav1NowTime := metav1.NewTime(time.Now())

	return &metav1NowTime
}

// annotateVolumeGroupReplicationWithOwner will add the VolumeReplication details to the VGR annotations.
func (r *VolumeReplicationReconciler) annotateVolumeGroupReplicationWithOwner(ctx context.Context, logger logr.Logger, reqOwnerName string, vgr *replicationv1alpha1.VolumeGroupReplication) error {
	if vgr.Annotations == nil {
		vgr.Annotations = map[string]string{}
	}

	currentOwnerName := vgr.Annotations[replicationv1alpha1.VolumeReplicationNameAnnotation]
	if currentOwnerName == "" {
		logger.Info("setting owner on VGR annotation", "Name", vgr.Name, "owner", reqOwnerName)
		vgr.Annotations[replicationv1alpha1.VolumeReplicationNameAnnotation] = reqOwnerName
		err := r.Update(ctx, vgr)
		if err != nil {
			logger.Error(err, "Failed to update VGR annotation", "Name", vgr.Name)

			return fmt.Errorf("failed to update VGR %q annotation for VolumeReplication: %w",
				vgr.Name, err)
		}

		return nil
	}

	if currentOwnerName != reqOwnerName {
		logger.Info("cannot change the owner of vgr",
			"VGR name", vgr.Name,
			"current owner", currentOwnerName,
			"requested owner", reqOwnerName)

		return fmt.Errorf("VGR %q not owned by VolumeReplication %q",
			vgr.Name, reqOwnerName)
	}

	return nil
}

func (r *VolumeReplicationReconciler) getVolumeGroupReplicationDataSource(logger logr.Logger, req types.NamespacedName) (*replicationv1alpha1.VolumeGroupReplication, *replicationv1alpha1.VolumeGroupReplicationContent, error) {
	volumeGroupReplication := &replicationv1alpha1.VolumeGroupReplication{}
	err := r.Get(context.TODO(), req, volumeGroupReplication)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Error(err, "VolumeGroupReplication not found", "VolumeGroupReplication Name", req.Name)
		}

		return nil, nil, err
	}
	vgrcName := volumeGroupReplication.Spec.VolumeGroupReplicationContentName
	if vgrcName == "" {
		logger.Error(err, "VolumeGroupReplicationContentName is empty", "VolumeGroupReplication Name", req.Name)

		return nil, nil, stderrors.New("VolumeGroupReplicationContentName is empty")
	}

	vgrcReq := types.NamespacedName{Name: vgrcName}
	volumeGroupReplicationContent := &replicationv1alpha1.VolumeGroupReplicationContent{}
	err = r.Get(context.TODO(), vgrcReq, volumeGroupReplicationContent)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Error(err, "VolumeGroupReplicationContent not found", "VolumeGroupReplicationContent Name", vgrcName)
		}

		return nil, nil, err
	}

	return volumeGroupReplication, volumeGroupReplicationContent, nil
}

// removeOwnerFromVGRAnnotation removes the VolumeReplication owner from the VGR annotations.
func (r *VolumeReplicationReconciler) removeOwnerFromVGRAnnotation(ctx context.Context, logger logr.Logger, vgr *replicationv1alpha1.VolumeGroupReplication) error {
	if _, ok := vgr.Annotations[replicationv1alpha1.VolumeReplicationNameAnnotation]; ok {
		logger.Info("removing owner annotation from VolumeGroupReplication object", "Annotation", replicationv1alpha1.VolumeReplicationNameAnnotation)
		delete(vgr.Annotations, replicationv1alpha1.VolumeReplicationNameAnnotation)
		if err := r.Update(ctx, vgr); err != nil {
			return fmt.Errorf("failed to remove annotation %q from VolumeGroupReplication "+
				"%q %w",
				replicationv1alpha1.VolumeReplicationNameAnnotation, vgr.Name, err)
		}
	}

	return nil
}
