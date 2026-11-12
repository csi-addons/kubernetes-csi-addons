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
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/csiaddons/v1alpha1"
	"github.com/csi-addons/kubernetes-csi-addons/internal/connection"
	"github.com/csi-addons/kubernetes-csi-addons/internal/controller/utils"
	"github.com/csi-addons/kubernetes-csi-addons/internal/proto"
	"github.com/csi-addons/kubernetes-csi-addons/internal/util"
	"github.com/csi-addons/spec/lib/go/identity"
	"github.com/go-logr/logr"
	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	scv1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestSetFailedCondition(t *testing.T) {
	type args struct {
		conditions         *[]v1.Condition
		message            string
		observedGeneration int64
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "overwrite existing failed condition",
			args: args{
				conditions: &[]v1.Condition{
					{
						Type:               conditionFailed,
						Status:             v1.ConditionTrue,
						ObservedGeneration: 0,
						LastTransitionTime: v1.NewTime(time.Now()),
						Reason:             reasonFailed,
						Message:            "err 1",
					},
				},
				message:            "err 2",
				observedGeneration: 3,
			},
		},
		{
			name: "add failed condition",
			args: args{
				conditions:         &[]v1.Condition{},
				message:            "err 1",
				observedGeneration: 3,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setFailedCondition(tt.args.conditions, tt.args.message, tt.args.observedGeneration)
			assert.Equal(t, tt.args.message, (*tt.args.conditions)[0].Message)
			assert.Equal(t, tt.args.observedGeneration, (*tt.args.conditions)[0].ObservedGeneration)
		})
	}
}

func TestClearFailedCondition(t *testing.T) {
	conditions := []v1.Condition{
		{Type: "Other", Message: "keep"},
		{Type: conditionFailed, Message: "remove"},
	}

	clearFailedCondition(&conditions)
	assert.Len(t, conditions, 1)
	assert.Equal(t, "Other", conditions[0].Type)

	// Clearing again is a no-op.
	clearFailedCondition(&conditions)
	assert.Len(t, conditions, 1)
}

func TestValidateReclaimSpaceJobSpec(t *testing.T) {
	type args struct {
		rsJob *csiaddonsv1alpha1.ReclaimSpaceJob
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "empty ReclaimSpaceJob.Spec.Target.PersistentVolumeClaim",
			args: args{
				rsJob: &csiaddonsv1alpha1.ReclaimSpaceJob{},
			},
			wantErr: true,
		},
		{
			name: "filled ReclaimSpaceJob.Spec.Target.PersistentVolumeClaim",
			args: args{
				rsJob: &csiaddonsv1alpha1.ReclaimSpaceJob{
					Spec: csiaddonsv1alpha1.ReclaimSpaceJobSpec{
						Target: csiaddonsv1alpha1.TargetSpec{
							PersistentVolumeClaim: "pvc-1",
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateReclaimSpaceJobSpec(tt.args.rsJob); (err != nil) != tt.wantErr {
				t.Errorf("validateReclaimSpaceJobSpec() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCalculateReclaimedSpace(t *testing.T) {
	type args struct {
		PreUsage  *proto.StorageConsumption
		PostUsage *proto.StorageConsumption
	}
	pre := int64(0)
	post := int64(5)
	result := post - pre
	result2 := int64(0)
	tests := []struct {
		name string
		args args
		want *int64
	}{
		{
			name: "both pre and post usage present",
			args: args{
				PreUsage: &proto.StorageConsumption{
					UsageBytes: pre,
				},
				PostUsage: &proto.StorageConsumption{
					UsageBytes: post,
				},
			},
			want: &result,
		},
		{
			name: "only post usage present",
			args: args{
				PreUsage: nil,
				PostUsage: &proto.StorageConsumption{
					UsageBytes: post,
				},
			},
			want: nil,
		},
		{
			name: "only pre usage present",
			args: args{
				PreUsage: &proto.StorageConsumption{
					UsageBytes: pre,
				},
				PostUsage: nil,
			},
			want: nil,
		},
		{
			name: "reclaimed space is negative",
			args: args{
				PreUsage: &proto.StorageConsumption{
					UsageBytes: pre,
				},
				PostUsage: &proto.StorageConsumption{
					UsageBytes: -post,
				},
			},
			want: &result2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateReclaimedSpace(tt.args.PreUsage, tt.args.PostUsage)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetTargetDetailsSkipsInlineVolumeAttachment(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, corev1.AddToScheme(scheme))
	assert.NoError(t, scv1.AddToScheme(scheme))
	assert.NoError(t, csiaddonsv1alpha1.AddToScheme(scheme))

	pvName := "pv-csi-1"
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: v1.ObjectMeta{
			Name:      "pvc-1",
			Namespace: "default",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			VolumeName: pvName,
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: corev1.ClaimBound,
		},
	}
	pv := &corev1.PersistentVolume{
		ObjectMeta: v1.ObjectMeta{
			Name: pvName,
		},
		Spec: corev1.PersistentVolumeSpec{
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver:       "rbd.csi.ceph.com",
					VolumeHandle: "vol-1",
				},
			},
		},
	}
	inlineVA := &scv1.VolumeAttachment{
		ObjectMeta: v1.ObjectMeta{
			Name: "va-inline-0",
		},
		Spec: scv1.VolumeAttachmentSpec{
			Attacher: "inline.csi.example.com",
			NodeName: "worker-inline",
			Source: scv1.VolumeAttachmentSource{
				PersistentVolumeName: nil,
				InlineVolumeSpec:     &corev1.PersistentVolumeSpec{},
			},
		},
		Status: scv1.VolumeAttachmentStatus{
			Attached: true,
		},
	}
	matchingVA := &scv1.VolumeAttachment{
		ObjectMeta: v1.ObjectMeta{
			Name: "va-pv-1",
		},
		Spec: scv1.VolumeAttachmentSpec{
			Attacher: "rbd.csi.ceph.com",
			NodeName: "worker-target",
			Source: scv1.VolumeAttachmentSource{
				PersistentVolumeName: &pvName,
			},
		},
		Status: scv1.VolumeAttachmentStatus{
			Attached: true,
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(pvc, pv, inlineVA, matchingVA).
		Build()

	logger := logr.Discard()
	rsReconciler := &ReclaimSpaceJobReconciler{
		Client:  cl,
		Scheme:  scheme,
		Timeout: time.Minute,
	}
	rsDetails, err := rsReconciler.getTargetDetails(
		context.Background(),
		&logger,
		csiaddonsv1alpha1.ReclaimSpaceJobSpec{
			Target: csiaddonsv1alpha1.TargetSpec{
				PersistentVolumeClaim: "pvc-1",
			},
		},
		"default",
	)
	assert.NoError(t, err)
	assert.Equal(t, "worker-target", rsDetails.nodeID)
	assert.Equal(t, "rbd.csi.ceph.com", rsDetails.driverName)

	krReconciler := &EncryptionKeyRotationJobReconciler{
		Client:  cl,
		Scheme:  scheme,
		Timeout: time.Minute,
	}
	krDetails, err := krReconciler.getTargetDetails(
		context.Background(),
		&logger,
		csiaddonsv1alpha1.EncryptionKeyRotationJobSpec{
			Target: csiaddonsv1alpha1.TargetSpec{
				PersistentVolumeClaim: "pvc-1",
			},
		},
		"default",
	)
	assert.NoError(t, err)
	assert.Equal(t, "worker-target", krDetails.nodeID)
	assert.Equal(t, "rbd.csi.ceph.com", krDetails.driverName)
}

func TestCronJobNilHistoryLimitsAndReplaceConcurrent(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, corev1.AddToScheme(scheme))
	assert.NoError(t, csiaddonsv1alpha1.AddToScheme(scheme))

	createdAt := v1.NewTime(time.Now().Add(-2 * time.Minute))
	rsCron := &csiaddonsv1alpha1.ReclaimSpaceCronJob{
		ObjectMeta: v1.ObjectMeta{
			Name:              "rs-cron-1",
			Namespace:         "default",
			UID:               "uid-rs-cron-1",
			CreationTimestamp: createdAt,
		},
		Spec: csiaddonsv1alpha1.ReclaimSpaceCronJobSpec{
			Schedule:          "* * * * *",
			ConcurrencyPolicy: csiaddonsv1alpha1.ReplaceConcurrent,
			JobSpec: csiaddonsv1alpha1.ReclaimSpaceJobTemplateSpec{
				Spec: csiaddonsv1alpha1.ReclaimSpaceJobSpec{
					Target: csiaddonsv1alpha1.TargetSpec{
						PersistentVolumeClaim: "pvc-1",
					},
				},
			},
		},
	}
	krCron := &csiaddonsv1alpha1.EncryptionKeyRotationCronJob{
		ObjectMeta: v1.ObjectMeta{
			Name:              "kr-cron-1",
			Namespace:         "default",
			UID:               "uid-kr-cron-1",
			CreationTimestamp: createdAt,
		},
		Spec: csiaddonsv1alpha1.EncryptionKeyRotationCronJobSpec{
			Schedule:          "* * * * *",
			ConcurrencyPolicy: csiaddonsv1alpha1.ReplaceConcurrent,
			JobSpec: csiaddonsv1alpha1.EncryptionKeyRotationJobTemplateSpec{
				Spec: csiaddonsv1alpha1.EncryptionKeyRotationJobSpec{
					Target: csiaddonsv1alpha1.TargetSpec{
						PersistentVolumeClaim: "pvc-1",
					},
				},
			},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(rsCron, krCron).
		WithIndex(&csiaddonsv1alpha1.ReclaimSpaceJob{}, utils.JobOwnerKey, func(rawObj client.Object) []string {
			job := rawObj.(*csiaddonsv1alpha1.ReclaimSpaceJob)
			owner := v1.GetControllerOf(job)
			if owner == nil {
				return nil
			}
			return []string{owner.Name}
		}).
		WithIndex(&csiaddonsv1alpha1.EncryptionKeyRotationJob{}, utils.JobOwnerKey, func(rawObj client.Object) []string {
			job := rawObj.(*csiaddonsv1alpha1.EncryptionKeyRotationJob)
			owner := v1.GetControllerOf(job)
			if owner == nil {
				return nil
			}
			return []string{owner.Name}
		}).
		WithObjects(rsCron, krCron).
		Build()

	rsCronReconciler := &ReclaimSpaceCronJobReconciler{
		Client: cl,
		Scheme: scheme,
	}
	_, err := rsCronReconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "rs-cron-1", Namespace: "default"},
	})
	assert.NoError(t, err)

	var rsJobs csiaddonsv1alpha1.ReclaimSpaceJobList
	assert.NoError(t, cl.List(context.Background(), &rsJobs))
	assert.Len(t, rsJobs.Items, 1)

	// Mark the existing active job as scheduled 2 minutes ago so the next schedule is due,
	// and verify ReplaceConcurrent deletes the active job and creates a replacement job.
	oldJob := &rsJobs.Items[0]
	oldJob.Annotations[scheduledTimeAnnotation] = time.Now().Add(-2 * time.Minute).Format(time.RFC3339)
	assert.NoError(t, cl.Update(context.Background(), oldJob))

	_, err = rsCronReconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "rs-cron-1", Namespace: "default"},
	})
	assert.NoError(t, err)
	assert.NoError(t, cl.List(context.Background(), &rsJobs))
	assert.Len(t, rsJobs.Items, 1)
	assert.NotEqual(t, oldJob.Annotations[scheduledTimeAnnotation], rsJobs.Items[0].Annotations[scheduledTimeAnnotation])

	krCronReconciler := &EncryptionKeyRotationCronJobReconciler{
		Client: cl,
		Scheme: scheme,
	}
	_, err = krCronReconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "kr-cron-1", Namespace: "default"},
	})
	assert.NoError(t, err)

	var krJobs csiaddonsv1alpha1.EncryptionKeyRotationJobList
	assert.NoError(t, cl.List(context.Background(), &krJobs))
	assert.Len(t, krJobs.Items, 1)
}

const (
	rsTestNamespace = "default"
	rsTestPVCName   = "data-pvc"
	rsTestPVName    = "pv-data"
	rsTestNodeID    = "worker-1"
	rsTestDriver    = "rbd.csi.ceph.com"
	rsTestLeader    = "leader-pod"
)

// fakeReclaimSpaceServer records the reclaim space RPCs that the controller
// invokes, so the operation gating can be verified.
type fakeReclaimSpaceServer struct {
	proto.UnimplementedReclaimSpaceServer

	mu              sync.Mutex
	controllerCalls int
	nodeCalls       int
	controllerErr   error
	nodeErr         error
	nodeResp        *proto.ReclaimSpaceResponse
	controllerResp  *proto.ReclaimSpaceResponse
}

// ControllerReclaimSpace records the call and returns the configured result.
func (f *fakeReclaimSpaceServer) ControllerReclaimSpace(
	context.Context,
	*proto.ReclaimSpaceRequest) (*proto.ReclaimSpaceResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.controllerCalls++
	if f.controllerErr != nil {
		return nil, f.controllerErr
	}
	if f.controllerResp != nil {
		return f.controllerResp, nil
	}

	return &proto.ReclaimSpaceResponse{}, nil
}

// NodeReclaimSpace records the call and returns the configured result.
func (f *fakeReclaimSpaceServer) NodeReclaimSpace(
	context.Context,
	*proto.ReclaimSpaceRequest) (*proto.ReclaimSpaceResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nodeCalls++
	if f.nodeErr != nil {
		return nil, f.nodeErr
	}
	if f.nodeResp != nil {
		return f.nodeResp, nil
	}

	return &proto.ReclaimSpaceResponse{}, nil
}

// callCounts returns the number of node and controller RPCs that were made.
func (f *fakeReclaimSpaceServer) callCounts() (node, controller int) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.nodeCalls, f.controllerCalls
}

// startFakeReclaimSpaceServer starts an in-process gRPC server and returns a
// client connection to it.
func startFakeReclaimSpaceServer(t *testing.T, impl *fakeReclaimSpaceServer) *grpc.ClientConn {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	server := grpc.NewServer()
	proto.RegisterReclaimSpaceServer(server, impl)
	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(server.Stop)

	cc, err := grpc.NewClient(
		"passthrough:///"+listener.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	t.Cleanup(func() { _ = cc.Close() })

	return cc
}

func reclaimSpaceCapability(t identity.Capability_ReclaimSpace_Type) *identity.Capability {
	return &identity.Capability{
		Type: &identity.Capability_ReclaimSpace_{
			ReclaimSpace: &identity.Capability_ReclaimSpace{Type: t},
		},
	}
}

func serviceCapability(t identity.Capability_Service_Type) *identity.Capability {
	return &identity.Capability{
		Type: &identity.Capability_Service_{
			Service: &identity.Capability_Service{Type: t},
		},
	}
}

// reclaimSpaceTestConnections controls the connections and capabilities that
// the test reconciler's connection pool contains.
type reclaimSpaceTestConnections struct {
	attached                 bool
	nodeNotRegistered        bool
	nodeWithoutOnline        bool
	controllerWithoutOffline bool
	controllerOnNode         bool
	// extraObjects are added to the fake client, for example the job that
	// Reconcile() fetches.
	extraObjects []client.Object
}

// newReclaimSpaceTestReconciler returns a reconciler whose connection pool
// contains a controller and a node connection to the provided fake server.
func newReclaimSpaceTestReconciler(
	t *testing.T,
	serverImpl *fakeReclaimSpaceServer,
	conns reclaimSpaceTestConnections) *ReclaimSpaceJobReconciler {
	t.Helper()

	cc := startFakeReclaimSpaceServer(t, serverImpl)

	scheme := runtime.NewScheme()
	assert.NoError(t, corev1.AddToScheme(scheme))
	assert.NoError(t, scv1.AddToScheme(scheme))
	assert.NoError(t, coordinationv1.AddToScheme(scheme))
	assert.NoError(t, csiaddonsv1alpha1.AddToScheme(scheme))

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: v1.ObjectMeta{Name: rsTestPVCName, Namespace: rsTestNamespace},
		Spec:       corev1.PersistentVolumeClaimSpec{VolumeName: rsTestPVName},
		Status:     corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound},
	}
	pv := &corev1.PersistentVolume{
		ObjectMeta: v1.ObjectMeta{Name: rsTestPVName},
		Spec: corev1.PersistentVolumeSpec{
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver:       rsTestDriver,
					VolumeHandle: "vol-1",
				},
			},
		},
	}
	holder := rsTestLeader
	lease := &coordinationv1.Lease{
		ObjectMeta: v1.ObjectMeta{
			Name:      util.NormalizeLeaseName(rsTestDriver) + "-csi-addons",
			Namespace: rsTestNamespace,
		},
		Spec: coordinationv1.LeaseSpec{HolderIdentity: &holder},
	}

	objects := []client.Object{pvc, pv, lease}
	if conns.attached {
		pvName := rsTestPVName
		objects = append(objects, &scv1.VolumeAttachment{
			ObjectMeta: v1.ObjectMeta{Name: "va-1"},
			Spec: scv1.VolumeAttachmentSpec{
				Attacher: rsTestDriver,
				NodeName: rsTestNodeID,
				Source: scv1.VolumeAttachmentSource{
					PersistentVolumeName: &pvName,
				},
			},
			Status: scv1.VolumeAttachmentStatus{Attached: true},
		})
	}

	objects = append(objects, conns.extraObjects...)
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		WithStatusSubresource(&csiaddonsv1alpha1.ReclaimSpaceJob{}).
		Build()

	controllerCaps := []*identity.Capability{
		serviceCapability(identity.Capability_Service_CONTROLLER_SERVICE),
	}
	if !conns.controllerWithoutOffline {
		controllerCaps = append(controllerCaps,
			reclaimSpaceCapability(identity.Capability_ReclaimSpace_OFFLINE))
	}

	// A controller sidecar that runs on the same node as the volume is
	// registered with that node's ID, so it shows up in GetByNodeID().
	leaderNodeID := ""
	if conns.controllerOnNode {
		leaderNodeID = rsTestNodeID
	}

	pool := connection.NewConnectionPool()
	pool.Put(rsTestNamespace+"/"+rsTestLeader, &connection.Connection{
		Client:       cc,
		Capabilities: controllerCaps,
		Namespace:    rsTestNamespace,
		Name:         rsTestLeader,
		NodeID:       leaderNodeID,
		DriverName:   rsTestDriver,
		Timeout:      time.Minute,
	})
	if !conns.nodeNotRegistered {
		nodeCaps := []*identity.Capability{
			serviceCapability(identity.Capability_Service_NODE_SERVICE),
		}
		if !conns.nodeWithoutOnline {
			nodeCaps = append(nodeCaps,
				reclaimSpaceCapability(identity.Capability_ReclaimSpace_ONLINE))
		}
		pool.Put(rsTestNamespace+"/node-pod", &connection.Connection{
			Client:       cc,
			Capabilities: nodeCaps,
			Namespace:    rsTestNamespace,
			Name:         "node-pod",
			NodeID:       rsTestNodeID,
			DriverName:   rsTestDriver,
			Timeout:      time.Minute,
		})
	}

	return &ReclaimSpaceJobReconciler{
		Client:   cl,
		Scheme:   scheme,
		ConnPool: pool,
		Timeout:  time.Minute,
	}
}

func TestReconcileReclaimSpaceOperations(t *testing.T) {
	reclaimResp := &proto.ReclaimSpaceResponse{
		PreUsage:  &proto.StorageConsumption{UsageBytes: 100},
		PostUsage: &proto.StorageConsumption{UsageBytes: 40},
	}
	nodeUnavailable := status.Error(codes.Unavailable, "node unavailable")
	controllerUnavailable := status.Error(codes.Unavailable, "controller unavailable")

	tests := []struct {
		name                     string
		operations               []csiaddonsv1alpha1.ReclaimSpaceOperation
		attached                 bool
		nodeNotRegistered        bool
		nodeWithoutOnline        bool
		controllerWithoutOffline bool
		controllerOnNode         bool
		nodeErr                  error
		controllerErr            error
		wantNodeCalls            int
		wantControllerCalls      int
		wantErr                  error
		wantResult               csiaddonsv1alpha1.OperationResult
		wantMessageSubstrings    []string
		wantConditionSubstring   string
		wantStates               map[csiaddonsv1alpha1.ReclaimSpaceOperation]csiaddonsv1alpha1.ReclaimSpaceOperationState
	}{
		{
			name:                "default operations only run node reclaim",
			operations:          csiaddonsv1alpha1.DefaultReclaimSpaceOperations(),
			attached:            true,
			wantNodeCalls:       1,
			wantControllerCalls: 0,
			wantResult:          csiaddonsv1alpha1.OperationResultSucceeded,
			wantStates: map[csiaddonsv1alpha1.ReclaimSpaceOperation]csiaddonsv1alpha1.ReclaimSpaceOperationState{
				csiaddonsv1alpha1.ReclaimSpaceOperationNode: csiaddonsv1alpha1.ReclaimSpaceOperationStateSucceeded,
			},
		},
		{
			name:                "controller only never runs node reclaim",
			operations:          []csiaddonsv1alpha1.ReclaimSpaceOperation{csiaddonsv1alpha1.ReclaimSpaceOperationController},
			attached:            true,
			wantNodeCalls:       0,
			wantControllerCalls: 1,
			wantResult:          csiaddonsv1alpha1.OperationResultSucceeded,
			wantStates: map[csiaddonsv1alpha1.ReclaimSpaceOperation]csiaddonsv1alpha1.ReclaimSpaceOperationState{
				csiaddonsv1alpha1.ReclaimSpaceOperationController: csiaddonsv1alpha1.ReclaimSpaceOperationStateSucceeded,
			},
		},
		{
			name: "both operations run both reclaims",
			operations: []csiaddonsv1alpha1.ReclaimSpaceOperation{
				csiaddonsv1alpha1.ReclaimSpaceOperationNode,
				csiaddonsv1alpha1.ReclaimSpaceOperationController,
			},
			attached:            true,
			wantNodeCalls:       1,
			wantControllerCalls: 1,
			wantResult:          csiaddonsv1alpha1.OperationResultSucceeded,
			wantStates: map[csiaddonsv1alpha1.ReclaimSpaceOperation]csiaddonsv1alpha1.ReclaimSpaceOperationState{
				csiaddonsv1alpha1.ReclaimSpaceOperationNode:       csiaddonsv1alpha1.ReclaimSpaceOperationStateSucceeded,
				csiaddonsv1alpha1.ReclaimSpaceOperationController: csiaddonsv1alpha1.ReclaimSpaceOperationStateSucceeded,
			},
		},
		{
			name:                "node only on an unattached volume requeues",
			operations:          []csiaddonsv1alpha1.ReclaimSpaceOperation{csiaddonsv1alpha1.ReclaimSpaceOperationNode},
			attached:            false,
			wantNodeCalls:       0,
			wantControllerCalls: 0,
			wantErr:             errVolumeNotAttached,
			wantStates: map[csiaddonsv1alpha1.ReclaimSpaceOperation]csiaddonsv1alpha1.ReclaimSpaceOperationState{
				csiaddonsv1alpha1.ReclaimSpaceOperationNode: csiaddonsv1alpha1.ReclaimSpaceOperationStatePending,
			},
		},
		{
			name:                "node only fails when the driver does not implement it",
			operations:          []csiaddonsv1alpha1.ReclaimSpaceOperation{csiaddonsv1alpha1.ReclaimSpaceOperationNode},
			attached:            true,
			nodeErr:             status.Error(codes.Unimplemented, "not implemented"),
			wantNodeCalls:       1,
			wantControllerCalls: 0,
			wantResult:          csiaddonsv1alpha1.OperationResultFailed,
			wantMessageSubstrings: []string{
				"no reclaim space operation performed",
				"NodeReclaimSpace is not supported",
				"Unimplemented",
			},
			wantConditionSubstring: "no reclaim space operation performed",
			wantStates: map[csiaddonsv1alpha1.ReclaimSpaceOperation]csiaddonsv1alpha1.ReclaimSpaceOperationState{
				csiaddonsv1alpha1.ReclaimSpaceOperationNode: csiaddonsv1alpha1.ReclaimSpaceOperationStateSkipped,
			},
		},
		{
			name:                "controller only fails when the driver does not implement it",
			operations:          []csiaddonsv1alpha1.ReclaimSpaceOperation{csiaddonsv1alpha1.ReclaimSpaceOperationController},
			attached:            true,
			controllerErr:       status.Error(codes.Unimplemented, "not implemented"),
			wantNodeCalls:       0,
			wantControllerCalls: 1,
			wantResult:          csiaddonsv1alpha1.OperationResultFailed,
			wantMessageSubstrings: []string{
				"no reclaim space operation performed",
				"ControllerReclaimSpace is not supported",
				"Unimplemented",
			},
			wantConditionSubstring: "no reclaim space operation performed",
			wantStates: map[csiaddonsv1alpha1.ReclaimSpaceOperation]csiaddonsv1alpha1.ReclaimSpaceOperationState{
				csiaddonsv1alpha1.ReclaimSpaceOperationController: csiaddonsv1alpha1.ReclaimSpaceOperationStateSkipped,
			},
		},
		{
			name: "both on an unattached volume keep the controller result and retry node",
			operations: []csiaddonsv1alpha1.ReclaimSpaceOperation{
				csiaddonsv1alpha1.ReclaimSpaceOperationNode,
				csiaddonsv1alpha1.ReclaimSpaceOperationController,
			},
			attached:               false,
			wantNodeCalls:          0,
			wantControllerCalls:    1,
			wantErr:                errVolumeNotAttached,
			wantConditionSubstring: "not attached to any node",
			wantStates: map[csiaddonsv1alpha1.ReclaimSpaceOperation]csiaddonsv1alpha1.ReclaimSpaceOperationState{
				csiaddonsv1alpha1.ReclaimSpaceOperationNode:       csiaddonsv1alpha1.ReclaimSpaceOperationStatePending,
				csiaddonsv1alpha1.ReclaimSpaceOperationController: csiaddonsv1alpha1.ReclaimSpaceOperationStateSucceeded,
			},
		},
		{
			name: "both succeed via controller when node is unimplemented",
			operations: []csiaddonsv1alpha1.ReclaimSpaceOperation{
				csiaddonsv1alpha1.ReclaimSpaceOperationNode,
				csiaddonsv1alpha1.ReclaimSpaceOperationController,
			},
			attached:            true,
			nodeErr:             status.Error(codes.Unimplemented, "not implemented"),
			wantNodeCalls:       1,
			wantControllerCalls: 1,
			wantResult:          csiaddonsv1alpha1.OperationResultSucceeded,
			wantMessageSubstrings: []string{
				"successfully completed",
				"NodeReclaimSpace is not supported",
			},
			wantStates: map[csiaddonsv1alpha1.ReclaimSpaceOperation]csiaddonsv1alpha1.ReclaimSpaceOperationState{
				csiaddonsv1alpha1.ReclaimSpaceOperationNode:       csiaddonsv1alpha1.ReclaimSpaceOperationStateSkipped,
				csiaddonsv1alpha1.ReclaimSpaceOperationController: csiaddonsv1alpha1.ReclaimSpaceOperationStateSucceeded,
			},
		},
		{
			name:                "empty operations default to node reclaim",
			operations:          nil,
			attached:            true,
			wantNodeCalls:       1,
			wantControllerCalls: 0,
			wantResult:          csiaddonsv1alpha1.OperationResultSucceeded,
			wantStates: map[csiaddonsv1alpha1.ReclaimSpaceOperation]csiaddonsv1alpha1.ReclaimSpaceOperationState{
				csiaddonsv1alpha1.ReclaimSpaceOperationNode: csiaddonsv1alpha1.ReclaimSpaceOperationStateSucceeded,
			},
		},
		{
			name: "both operations run controller when node reclaim is unsupported",
			operations: []csiaddonsv1alpha1.ReclaimSpaceOperation{
				csiaddonsv1alpha1.ReclaimSpaceOperationNode,
				csiaddonsv1alpha1.ReclaimSpaceOperationController,
			},
			attached:            true,
			nodeWithoutOnline:   true,
			wantNodeCalls:       0,
			wantControllerCalls: 1,
			wantResult:          csiaddonsv1alpha1.OperationResultSucceeded,
			wantMessageSubstrings: []string{
				"successfully completed",
				"NodeReclaimSpace is not supported",
				"does not advertise the ReclaimSpace_ONLINE capability",
			},
			wantStates: map[csiaddonsv1alpha1.ReclaimSpaceOperation]csiaddonsv1alpha1.ReclaimSpaceOperationState{
				csiaddonsv1alpha1.ReclaimSpaceOperationNode:       csiaddonsv1alpha1.ReclaimSpaceOperationStateSkipped,
				csiaddonsv1alpha1.ReclaimSpaceOperationController: csiaddonsv1alpha1.ReclaimSpaceOperationStateSucceeded,
			},
		},
		{
			name: "both operations run node when controller reclaim is unsupported",
			operations: []csiaddonsv1alpha1.ReclaimSpaceOperation{
				csiaddonsv1alpha1.ReclaimSpaceOperationNode,
				csiaddonsv1alpha1.ReclaimSpaceOperationController,
			},
			attached:                 true,
			controllerWithoutOffline: true,
			wantNodeCalls:            1,
			wantControllerCalls:      0,
			wantResult:               csiaddonsv1alpha1.OperationResultSucceeded,
			wantMessageSubstrings: []string{
				"successfully completed",
				"ControllerReclaimSpace is not supported",
				"does not advertise the ReclaimSpace_OFFLINE capability",
			},
			wantStates: map[csiaddonsv1alpha1.ReclaimSpaceOperation]csiaddonsv1alpha1.ReclaimSpaceOperationState{
				csiaddonsv1alpha1.ReclaimSpaceOperationNode:       csiaddonsv1alpha1.ReclaimSpaceOperationStateSucceeded,
				csiaddonsv1alpha1.ReclaimSpaceOperationController: csiaddonsv1alpha1.ReclaimSpaceOperationStateSkipped,
			},
		},
		{
			name:                "node-only requeues while the node client is not registered",
			operations:          []csiaddonsv1alpha1.ReclaimSpaceOperation{csiaddonsv1alpha1.ReclaimSpaceOperationNode},
			attached:            true,
			nodeNotRegistered:   true,
			wantNodeCalls:       0,
			wantControllerCalls: 0,
			wantErr:             errNodeClientNotFound,
			wantStates: map[csiaddonsv1alpha1.ReclaimSpaceOperation]csiaddonsv1alpha1.ReclaimSpaceOperationState{
				csiaddonsv1alpha1.ReclaimSpaceOperationNode: csiaddonsv1alpha1.ReclaimSpaceOperationStatePending,
			},
		},
		{
			name:                "node-only requeues when only a controller connection is on the node",
			operations:          []csiaddonsv1alpha1.ReclaimSpaceOperation{csiaddonsv1alpha1.ReclaimSpaceOperationNode},
			attached:            true,
			nodeNotRegistered:   true,
			controllerOnNode:    true,
			wantNodeCalls:       0,
			wantControllerCalls: 0,
			wantErr:             errNodeClientNotFound,
			wantStates: map[csiaddonsv1alpha1.ReclaimSpaceOperation]csiaddonsv1alpha1.ReclaimSpaceOperationState{
				csiaddonsv1alpha1.ReclaimSpaceOperationNode: csiaddonsv1alpha1.ReclaimSpaceOperationStatePending,
			},
		},
		{
			name: "both operations keep the controller result and retry a missing node client",
			operations: []csiaddonsv1alpha1.ReclaimSpaceOperation{
				csiaddonsv1alpha1.ReclaimSpaceOperationNode,
				csiaddonsv1alpha1.ReclaimSpaceOperationController,
			},
			attached:               true,
			nodeNotRegistered:      true,
			wantNodeCalls:          0,
			wantControllerCalls:    1,
			wantErr:                errNodeClientNotFound,
			wantConditionSubstring: "node client not found",
			wantStates: map[csiaddonsv1alpha1.ReclaimSpaceOperation]csiaddonsv1alpha1.ReclaimSpaceOperationState{
				csiaddonsv1alpha1.ReclaimSpaceOperationNode:       csiaddonsv1alpha1.ReclaimSpaceOperationStatePending,
				csiaddonsv1alpha1.ReclaimSpaceOperationController: csiaddonsv1alpha1.ReclaimSpaceOperationStateSucceeded,
			},
		},
		{
			name: "node failure does not block the controller operation",
			operations: []csiaddonsv1alpha1.ReclaimSpaceOperation{
				csiaddonsv1alpha1.ReclaimSpaceOperationNode,
				csiaddonsv1alpha1.ReclaimSpaceOperationController,
			},
			attached:               true,
			nodeErr:                nodeUnavailable,
			wantNodeCalls:          1,
			wantControllerCalls:    1,
			wantErr:                nodeUnavailable,
			wantConditionSubstring: "failed to make node request",
			wantStates: map[csiaddonsv1alpha1.ReclaimSpaceOperation]csiaddonsv1alpha1.ReclaimSpaceOperationState{
				csiaddonsv1alpha1.ReclaimSpaceOperationNode:       csiaddonsv1alpha1.ReclaimSpaceOperationStatePending,
				csiaddonsv1alpha1.ReclaimSpaceOperationController: csiaddonsv1alpha1.ReclaimSpaceOperationStateSucceeded,
			},
		},
		{
			name: "controller failure does not block the node operation",
			operations: []csiaddonsv1alpha1.ReclaimSpaceOperation{
				csiaddonsv1alpha1.ReclaimSpaceOperationNode,
				csiaddonsv1alpha1.ReclaimSpaceOperationController,
			},
			attached:               true,
			controllerErr:          controllerUnavailable,
			wantNodeCalls:          1,
			wantControllerCalls:    1,
			wantErr:                controllerUnavailable,
			wantConditionSubstring: "failed to make controller request",
			wantStates: map[csiaddonsv1alpha1.ReclaimSpaceOperation]csiaddonsv1alpha1.ReclaimSpaceOperationState{
				csiaddonsv1alpha1.ReclaimSpaceOperationNode:       csiaddonsv1alpha1.ReclaimSpaceOperationStateSucceeded,
				csiaddonsv1alpha1.ReclaimSpaceOperationController: csiaddonsv1alpha1.ReclaimSpaceOperationStatePending,
			},
		},
		{
			name: "unattached node reports the unsupported controller reason",
			operations: []csiaddonsv1alpha1.ReclaimSpaceOperation{
				csiaddonsv1alpha1.ReclaimSpaceOperationNode,
				csiaddonsv1alpha1.ReclaimSpaceOperationController,
			},
			attached:                 false,
			controllerWithoutOffline: true,
			wantNodeCalls:            0,
			wantControllerCalls:      0,
			wantErr:                  errVolumeNotAttached,
			wantConditionSubstring:   "ControllerReclaimSpace is not supported",
			wantStates: map[csiaddonsv1alpha1.ReclaimSpaceOperation]csiaddonsv1alpha1.ReclaimSpaceOperationState{
				csiaddonsv1alpha1.ReclaimSpaceOperationNode:       csiaddonsv1alpha1.ReclaimSpaceOperationStatePending,
				csiaddonsv1alpha1.ReclaimSpaceOperationController: csiaddonsv1alpha1.ReclaimSpaceOperationStateSkipped,
			},
		},
		{
			name:                   "unattached node-only records the attachment reason",
			operations:             []csiaddonsv1alpha1.ReclaimSpaceOperation{csiaddonsv1alpha1.ReclaimSpaceOperationNode},
			attached:               false,
			wantNodeCalls:          0,
			wantControllerCalls:    0,
			wantErr:                errVolumeNotAttached,
			wantConditionSubstring: "not attached to any node",
			wantStates: map[csiaddonsv1alpha1.ReclaimSpaceOperation]csiaddonsv1alpha1.ReclaimSpaceOperationState{
				csiaddonsv1alpha1.ReclaimSpaceOperationNode: csiaddonsv1alpha1.ReclaimSpaceOperationStatePending,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serverImpl := &fakeReclaimSpaceServer{
				nodeErr:        tt.nodeErr,
				controllerErr:  tt.controllerErr,
				nodeResp:       reclaimResp,
				controllerResp: reclaimResp,
			}
			reconciler := newReclaimSpaceTestReconciler(t, serverImpl, reclaimSpaceTestConnections{
				attached:                 tt.attached,
				nodeNotRegistered:        tt.nodeNotRegistered,
				nodeWithoutOnline:        tt.nodeWithoutOnline,
				controllerWithoutOffline: tt.controllerWithoutOffline,
				controllerOnNode:         tt.controllerOnNode,
			})

			job := &csiaddonsv1alpha1.ReclaimSpaceJob{
				ObjectMeta: v1.ObjectMeta{
					Name:              "rs-job",
					Namespace:         rsTestNamespace,
					CreationTimestamp: v1.NewTime(time.Now()),
				},
				Spec: csiaddonsv1alpha1.ReclaimSpaceJobSpec{
					Target:               csiaddonsv1alpha1.TargetSpec{PersistentVolumeClaim: rsTestPVCName},
					Operations:           tt.operations,
					BackoffLimit:         6,
					RetryDeadlineSeconds: 600,
				},
			}
			logger := logr.Discard()

			err := reconciler.reconcile(context.Background(), &logger, job, rsTestNamespace)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}

			nodeCalls, controllerCalls := serverImpl.callCounts()
			assert.Equal(t, tt.wantNodeCalls, nodeCalls, "NodeReclaimSpace call count")
			assert.Equal(t, tt.wantControllerCalls, controllerCalls, "ControllerReclaimSpace call count")
			assert.Equal(t, tt.wantResult, job.Status.Result)
			for _, substring := range tt.wantMessageSubstrings {
				assert.Contains(t, job.Status.Message, substring)
			}
			if tt.wantConditionSubstring != "" {
				messages := make([]string, 0, len(job.Status.Conditions))
				for _, condition := range job.Status.Conditions {
					messages = append(messages, condition.Message)
				}
				assert.Contains(t, strings.Join(messages, "; "), tt.wantConditionSubstring)
			}
			assertOperationStates(t, job, tt.wantStates)
		})
	}
}

// assertOperationStates compares the per-operation states reported in status.
func assertOperationStates(
	t *testing.T,
	job *csiaddonsv1alpha1.ReclaimSpaceJob,
	want map[csiaddonsv1alpha1.ReclaimSpaceOperation]csiaddonsv1alpha1.ReclaimSpaceOperationState) {
	t.Helper()

	if want == nil {
		return
	}

	got := make(map[csiaddonsv1alpha1.ReclaimSpaceOperation]csiaddonsv1alpha1.ReclaimSpaceOperationState, len(job.Status.Operations))
	for _, opStatus := range job.Status.Operations {
		got[opStatus.Operation] = opStatus.State
	}

	assert.Equal(t, want, got)
}

func TestReconcileClearsFailedConditionWhenAttachmentAppears(t *testing.T) {
	serverImpl := &fakeReclaimSpaceServer{
		nodeResp: &proto.ReclaimSpaceResponse{
			PreUsage:  &proto.StorageConsumption{UsageBytes: 100},
			PostUsage: &proto.StorageConsumption{UsageBytes: 40},
		},
	}
	reconciler := newReclaimSpaceTestReconciler(t, serverImpl, reclaimSpaceTestConnections{})

	job := &csiaddonsv1alpha1.ReclaimSpaceJob{
		ObjectMeta: v1.ObjectMeta{
			Name:              "rs-job",
			Namespace:         rsTestNamespace,
			CreationTimestamp: v1.NewTime(time.Now()),
		},
		Spec: csiaddonsv1alpha1.ReclaimSpaceJobSpec{
			Target:               csiaddonsv1alpha1.TargetSpec{PersistentVolumeClaim: rsTestPVCName},
			Operations:           []csiaddonsv1alpha1.ReclaimSpaceOperation{csiaddonsv1alpha1.ReclaimSpaceOperationNode},
			BackoffLimit:         6,
			RetryDeadlineSeconds: 600,
		},
	}
	logger := logr.Discard()

	err := reconciler.reconcile(context.Background(), &logger, job, rsTestNamespace)
	assert.ErrorIs(t, err, errVolumeNotAttached)
	assert.NotEmpty(t, job.Status.Conditions, "waiting for the attachment should be recorded")
	assertOperationStates(t, job, map[csiaddonsv1alpha1.ReclaimSpaceOperation]csiaddonsv1alpha1.ReclaimSpaceOperationState{
		csiaddonsv1alpha1.ReclaimSpaceOperationNode: csiaddonsv1alpha1.ReclaimSpaceOperationStatePending,
	})

	pvName := rsTestPVName
	assert.NoError(t, reconciler.Create(context.Background(), &scv1.VolumeAttachment{
		ObjectMeta: v1.ObjectMeta{Name: "va-1"},
		Spec: scv1.VolumeAttachmentSpec{
			Attacher: rsTestDriver,
			NodeName: rsTestNodeID,
			Source:   scv1.VolumeAttachmentSource{PersistentVolumeName: &pvName},
		},
		Status: scv1.VolumeAttachmentStatus{Attached: true},
	}))

	err = reconciler.reconcile(context.Background(), &logger, job, rsTestNamespace)
	assert.NoError(t, err)
	assert.Equal(t, csiaddonsv1alpha1.OperationResultSucceeded, job.Status.Result)
	assert.Empty(t, job.Status.Conditions, "the transient failed condition should be cleared on success")
	assertOperationStates(t, job, map[csiaddonsv1alpha1.ReclaimSpaceOperation]csiaddonsv1alpha1.ReclaimSpaceOperationState{
		csiaddonsv1alpha1.ReclaimSpaceOperationNode: csiaddonsv1alpha1.ReclaimSpaceOperationStateSucceeded,
	})

	nodeCalls, controllerCalls := serverImpl.callCounts()
	assert.Equal(t, 1, nodeCalls)
	assert.Equal(t, 0, controllerCalls)
}

func TestReconcileRetriesOnlyPendingOperations(t *testing.T) {
	reclaimResp := &proto.ReclaimSpaceResponse{
		PreUsage:  &proto.StorageConsumption{UsageBytes: 100},
		PostUsage: &proto.StorageConsumption{UsageBytes: 40},
	}
	serverImpl := &fakeReclaimSpaceServer{
		nodeResp:       reclaimResp,
		controllerResp: reclaimResp,
	}
	reconciler := newReclaimSpaceTestReconciler(t, serverImpl, reclaimSpaceTestConnections{
		attached:          true,
		nodeNotRegistered: true,
	})

	job := &csiaddonsv1alpha1.ReclaimSpaceJob{
		ObjectMeta: v1.ObjectMeta{
			Name:              "rs-job",
			Namespace:         rsTestNamespace,
			CreationTimestamp: v1.NewTime(time.Now()),
		},
		Spec: csiaddonsv1alpha1.ReclaimSpaceJobSpec{
			Target: csiaddonsv1alpha1.TargetSpec{PersistentVolumeClaim: rsTestPVCName},
			Operations: []csiaddonsv1alpha1.ReclaimSpaceOperation{
				csiaddonsv1alpha1.ReclaimSpaceOperationNode,
				csiaddonsv1alpha1.ReclaimSpaceOperationController,
			},
			BackoffLimit:         10,
			RetryDeadlineSeconds: 600,
		},
	}
	logger := logr.Discard()

	// The controller succeeds while the node client is still missing.
	err := reconciler.reconcile(context.Background(), &logger, job, rsTestNamespace)
	assert.ErrorIs(t, err, errNodeClientNotFound)
	assert.Equal(t, csiaddonsv1alpha1.OperationResult(""), job.Status.Result)
	assertOperationStates(t, job, map[csiaddonsv1alpha1.ReclaimSpaceOperation]csiaddonsv1alpha1.ReclaimSpaceOperationState{
		csiaddonsv1alpha1.ReclaimSpaceOperationNode:       csiaddonsv1alpha1.ReclaimSpaceOperationStatePending,
		csiaddonsv1alpha1.ReclaimSpaceOperationController: csiaddonsv1alpha1.ReclaimSpaceOperationStateSucceeded,
	})
	assert.Len(t, job.Status.Operations, 2)
	// The node operation is attempted and reported first.
	assert.Equal(t, csiaddonsv1alpha1.ReclaimSpaceOperationNode, job.Status.Operations[0].Operation)
	assert.Equal(t, csiaddonsv1alpha1.ReclaimSpaceOperationController, job.Status.Operations[1].Operation)

	// The next reconcile retries only the node operation.
	err = reconciler.reconcile(context.Background(), &logger, job, rsTestNamespace)
	assert.ErrorIs(t, err, errNodeClientNotFound)
	assertOperationStates(t, job, map[csiaddonsv1alpha1.ReclaimSpaceOperation]csiaddonsv1alpha1.ReclaimSpaceOperationState{
		csiaddonsv1alpha1.ReclaimSpaceOperationNode:       csiaddonsv1alpha1.ReclaimSpaceOperationStatePending,
		csiaddonsv1alpha1.ReclaimSpaceOperationController: csiaddonsv1alpha1.ReclaimSpaceOperationStateSucceeded,
	})

	nodeCalls, controllerCalls := serverImpl.callCounts()
	assert.Equal(t, 0, nodeCalls, "NodeReclaimSpace should never have been called")
	assert.Equal(t, 1, controllerCalls, "ControllerReclaimSpace must only run once")
}

func TestReconcileFailsPendingOperationsAtRetryLimit(t *testing.T) {
	reclaimResp := &proto.ReclaimSpaceResponse{
		PreUsage:  &proto.StorageConsumption{UsageBytes: 40},
		PostUsage: &proto.StorageConsumption{UsageBytes: 100},
	}
	serverImpl := &fakeReclaimSpaceServer{
		nodeResp:       reclaimResp,
		controllerResp: reclaimResp,
	}
	reconciler := newReclaimSpaceTestReconciler(t, serverImpl, reclaimSpaceTestConnections{
		attached:          true,
		nodeNotRegistered: true,
	})

	job := &csiaddonsv1alpha1.ReclaimSpaceJob{
		ObjectMeta: v1.ObjectMeta{
			Name:              "rs-job",
			Namespace:         rsTestNamespace,
			CreationTimestamp: v1.NewTime(time.Now()),
		},
		Spec: csiaddonsv1alpha1.ReclaimSpaceJobSpec{
			Target: csiaddonsv1alpha1.TargetSpec{PersistentVolumeClaim: rsTestPVCName},
			Operations: []csiaddonsv1alpha1.ReclaimSpaceOperation{
				csiaddonsv1alpha1.ReclaimSpaceOperationNode,
				csiaddonsv1alpha1.ReclaimSpaceOperationController,
			},
			BackoffLimit:         1,
			RetryDeadlineSeconds: 600,
		},
		Status: csiaddonsv1alpha1.ReclaimSpaceJobStatus{
			// The only retry is consumed by this reconcile.
			StartTime: &v1.Time{Time: time.Now()},
		},
	}
	logger := logr.Discard()

	err := reconciler.reconcile(context.Background(), &logger, job, rsTestNamespace)
	assert.NoError(t, err)
	assert.Equal(t, csiaddonsv1alpha1.OperationResultFailed, job.Status.Result)
	assert.Contains(t, job.Status.Message, "Maximum retry limit reached")
	assert.Contains(t, job.Status.Message, "controller-side reclaim succeeded")
	assert.Contains(t, job.Status.Message, "node-side reclaim failed")
	assert.NotNil(t, job.Status.ReclaimedSpace)
	assert.Equal(t, int64(60), job.Status.ReclaimedSpace.Value())
	assertOperationStates(t, job, map[csiaddonsv1alpha1.ReclaimSpaceOperation]csiaddonsv1alpha1.ReclaimSpaceOperationState{
		csiaddonsv1alpha1.ReclaimSpaceOperationNode:       csiaddonsv1alpha1.ReclaimSpaceOperationStateFailed,
		csiaddonsv1alpha1.ReclaimSpaceOperationController: csiaddonsv1alpha1.ReclaimSpaceOperationStateSucceeded,
	})

	nodeCalls, controllerCalls := serverImpl.callCounts()
	assert.Equal(t, 0, nodeCalls)
	assert.Equal(t, 1, controllerCalls)
}

// newTestReclaimSpaceJob returns a job for the test PVC that requests ops.
func newTestReclaimSpaceJob(ops ...csiaddonsv1alpha1.ReclaimSpaceOperation) *csiaddonsv1alpha1.ReclaimSpaceJob {
	return &csiaddonsv1alpha1.ReclaimSpaceJob{
		ObjectMeta: v1.ObjectMeta{
			Name:              "rs-job",
			Namespace:         rsTestNamespace,
			CreationTimestamp: v1.NewTime(time.Now()),
		},
		Spec: csiaddonsv1alpha1.ReclaimSpaceJobSpec{
			Target:               csiaddonsv1alpha1.TargetSpec{PersistentVolumeClaim: rsTestPVCName},
			Operations:           ops,
			BackoffLimit:         6,
			RetryDeadlineSeconds: 600,
		},
	}
}

func TestReconcileTargetLookupFailure(t *testing.T) {
	t.Run("retries with the cause in status before the limit", func(t *testing.T) {
		reconciler := newReclaimSpaceTestReconciler(t, &fakeReclaimSpaceServer{}, reclaimSpaceTestConnections{attached: true})
		job := newTestReclaimSpaceJob(csiaddonsv1alpha1.ReclaimSpaceOperationNode)
		job.Spec.Target.PersistentVolumeClaim = "missing-pvc"
		logger := logr.Discard()

		err := reconciler.reconcile(context.Background(), &logger, job, rsTestNamespace)

		assert.Error(t, err)
		assert.Equal(t, csiaddonsv1alpha1.OperationResult(""), job.Status.Result)
		if assert.Len(t, job.Status.Conditions, 1) {
			assert.Contains(t, job.Status.Conditions[0].Message, "failed to get target details")
			assert.Contains(t, job.Status.Conditions[0].Message, "missing-pvc")
		}
		if assert.Len(t, job.Status.Operations, 1) {
			assert.Equal(t, csiaddonsv1alpha1.ReclaimSpaceOperationStatePending, job.Status.Operations[0].State)
			assert.Contains(t, job.Status.Operations[0].Message, "missing-pvc")
		}
	})

	t.Run("fails the job at the retry limit", func(t *testing.T) {
		reconciler := newReclaimSpaceTestReconciler(t, &fakeReclaimSpaceServer{}, reclaimSpaceTestConnections{attached: true})
		job := newTestReclaimSpaceJob(csiaddonsv1alpha1.ReclaimSpaceOperationNode)
		job.Spec.Target.PersistentVolumeClaim = "missing-pvc"
		job.Spec.BackoffLimit = 1
		// The only retry is consumed by this reconcile.
		job.Status.StartTime = &v1.Time{Time: time.Now()}
		logger := logr.Discard()

		err := reconciler.reconcile(context.Background(), &logger, job, rsTestNamespace)

		assert.NoError(t, err)
		assert.Equal(t, csiaddonsv1alpha1.OperationResultFailed, job.Status.Result)
		assert.Contains(t, job.Status.Message, "Maximum retry limit reached")
		assert.Contains(t, job.Status.Message, "missing-pvc")
		assert.Equal(t, int32(1), job.Status.Retries)
		assertOperationStates(t, job, map[csiaddonsv1alpha1.ReclaimSpaceOperation]csiaddonsv1alpha1.ReclaimSpaceOperationState{
			csiaddonsv1alpha1.ReclaimSpaceOperationNode: csiaddonsv1alpha1.ReclaimSpaceOperationStateFailed,
		})
	})
}

func TestReconcileFailsAtDeadlineWithPartialOperations(t *testing.T) {
	serverImpl := &fakeReclaimSpaceServer{}
	reconciler := newReclaimSpaceTestReconciler(t, serverImpl, reclaimSpaceTestConnections{attached: true})
	job := newTestReclaimSpaceJob(
		csiaddonsv1alpha1.ReclaimSpaceOperationNode,
		csiaddonsv1alpha1.ReclaimSpaceOperationController)
	job.CreationTimestamp = v1.NewTime(time.Now().Add(-2 * time.Minute))
	job.Spec.RetryDeadlineSeconds = 60
	job.Status = csiaddonsv1alpha1.ReclaimSpaceJobStatus{
		StartTime: &job.CreationTimestamp,
		Operations: []csiaddonsv1alpha1.ReclaimSpaceOperationStatus{
			{
				Operation: csiaddonsv1alpha1.ReclaimSpaceOperationNode,
				State:     csiaddonsv1alpha1.ReclaimSpaceOperationStatePending,
				Message:   "node client not found",
			},
			{
				Operation:      csiaddonsv1alpha1.ReclaimSpaceOperationController,
				State:          csiaddonsv1alpha1.ReclaimSpaceOperationStateSucceeded,
				ReclaimedSpace: resource.NewQuantity(60, resource.DecimalSI),
				CompletionTime: &job.CreationTimestamp,
			},
		},
	}
	logger := logr.Discard()

	err := reconciler.reconcile(context.Background(), &logger, job, rsTestNamespace)

	assert.NoError(t, err)
	assert.Equal(t, csiaddonsv1alpha1.OperationResultFailed, job.Status.Result)
	assert.Contains(t, job.Status.Message, "Time limit reached")
	assert.Contains(t, job.Status.Message, "node-side reclaim failed: node client not found")
	assert.Contains(t, job.Status.Message, "controller-side reclaim succeeded")
	if assert.NotNil(t, job.Status.ReclaimedSpace) {
		assert.Equal(t, int64(60), job.Status.ReclaimedSpace.Value())
	}
	assert.NotNil(t, job.Status.Operations[0].CompletionTime)
	assert.NotNil(t, meta.FindStatusCondition(job.Status.Conditions, conditionFailed))
	assertOperationStates(t, job, map[csiaddonsv1alpha1.ReclaimSpaceOperation]csiaddonsv1alpha1.ReclaimSpaceOperationState{
		csiaddonsv1alpha1.ReclaimSpaceOperationNode:       csiaddonsv1alpha1.ReclaimSpaceOperationStateFailed,
		csiaddonsv1alpha1.ReclaimSpaceOperationController: csiaddonsv1alpha1.ReclaimSpaceOperationStateSucceeded,
	})

	nodeCalls, controllerCalls := serverImpl.callCounts()
	assert.Equal(t, 0, nodeCalls)
	assert.Equal(t, 0, controllerCalls)
}

func TestReconcileBothOperationsSumReclaimedSpace(t *testing.T) {
	// calculateReclaimedSpace reports PostUsage - PreUsage, so each
	// operation reclaims 60 bytes with this response.
	reclaimResp := &proto.ReclaimSpaceResponse{
		PreUsage:  &proto.StorageConsumption{UsageBytes: 40},
		PostUsage: &proto.StorageConsumption{UsageBytes: 100},
	}
	serverImpl := &fakeReclaimSpaceServer{nodeResp: reclaimResp, controllerResp: reclaimResp}
	reconciler := newReclaimSpaceTestReconciler(t, serverImpl, reclaimSpaceTestConnections{attached: true})
	job := newTestReclaimSpaceJob(
		csiaddonsv1alpha1.ReclaimSpaceOperationNode,
		csiaddonsv1alpha1.ReclaimSpaceOperationController)
	logger := logr.Discard()

	err := reconciler.reconcile(context.Background(), &logger, job, rsTestNamespace)

	assert.NoError(t, err)
	assert.Equal(t, csiaddonsv1alpha1.OperationResultSucceeded, job.Status.Result)
	if assert.NotNil(t, job.Status.ReclaimedSpace) {
		assert.Equal(t, int64(120), job.Status.ReclaimedSpace.Value())
	}
	for _, opStatus := range job.Status.Operations {
		if assert.NotNil(t, opStatus.ReclaimedSpace, opStatus.Operation) {
			assert.Equal(t, int64(60), opStatus.ReclaimedSpace.Value(), opStatus.Operation)
		}
	}
}

func TestReconcileUnknownOperationFailsJob(t *testing.T) {
	serverImpl := &fakeReclaimSpaceServer{}
	reconciler := newReclaimSpaceTestReconciler(t, serverImpl, reclaimSpaceTestConnections{attached: true})
	job := newTestReclaimSpaceJob("Sparsify")
	logger := logr.Discard()

	err := reconciler.reconcile(context.Background(), &logger, job, rsTestNamespace)

	assert.NoError(t, err)
	assert.Equal(t, csiaddonsv1alpha1.OperationResultFailed, job.Status.Result)
	assert.Contains(t, job.Status.Message, `unknown reclaim space operation "Sparsify"`)
	assert.NotNil(t, meta.FindStatusCondition(job.Status.Conditions, conditionFailed))
}

func TestReconcileRequeuesWhileNodeUnavailable(t *testing.T) {
	tests := []struct {
		name          string
		operations    []csiaddonsv1alpha1.ReclaimSpaceOperation
		conns         reclaimSpaceTestConnections
		controllerErr error
		wantStates    map[csiaddonsv1alpha1.ReclaimSpaceOperation]csiaddonsv1alpha1.ReclaimSpaceOperationState
	}{
		{
			name:       "node only on an unattached volume",
			operations: []csiaddonsv1alpha1.ReclaimSpaceOperation{csiaddonsv1alpha1.ReclaimSpaceOperationNode},
			conns:      reclaimSpaceTestConnections{attached: false},
			wantStates: map[csiaddonsv1alpha1.ReclaimSpaceOperation]csiaddonsv1alpha1.ReclaimSpaceOperationState{
				csiaddonsv1alpha1.ReclaimSpaceOperationNode: csiaddonsv1alpha1.ReclaimSpaceOperationStatePending,
			},
		},
		{
			// The controller error must not switch to the fast rate-limited
			// requeue, which would use up the retries before the node is ready.
			name: "missing node client alongside a controller failure",
			operations: []csiaddonsv1alpha1.ReclaimSpaceOperation{
				csiaddonsv1alpha1.ReclaimSpaceOperationNode,
				csiaddonsv1alpha1.ReclaimSpaceOperationController,
			},
			conns:         reclaimSpaceTestConnections{attached: true, nodeNotRegistered: true},
			controllerErr: status.Error(codes.Unavailable, "controller unavailable"),
			wantStates: map[csiaddonsv1alpha1.ReclaimSpaceOperation]csiaddonsv1alpha1.ReclaimSpaceOperationState{
				csiaddonsv1alpha1.ReclaimSpaceOperationNode:       csiaddonsv1alpha1.ReclaimSpaceOperationStatePending,
				csiaddonsv1alpha1.ReclaimSpaceOperationController: csiaddonsv1alpha1.ReclaimSpaceOperationStatePending,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := newTestReclaimSpaceJob(tt.operations...)
			tt.conns.extraObjects = []client.Object{job}
			reconciler := newReclaimSpaceTestReconciler(t,
				&fakeReclaimSpaceServer{controllerErr: tt.controllerErr}, tt.conns)
			req := ctrl.Request{NamespacedName: types.NamespacedName{Name: job.Name, Namespace: job.Namespace}}

			result, err := reconciler.Reconcile(context.Background(), req)

			assert.NoError(t, err)
			assert.Equal(t, nodeClientRequeueInterval, result.RequeueAfter)

			stored := &csiaddonsv1alpha1.ReclaimSpaceJob{}
			assert.NoError(t, reconciler.Get(context.Background(), req.NamespacedName, stored))
			assert.Equal(t, csiaddonsv1alpha1.OperationResult(""), stored.Status.Result)
			assertOperationStates(t, stored, tt.wantStates)
		})
	}
}

var _ = ginkgo.Describe("ReclaimSpaceJob spec.operations", func() {
	ctx := context.Background()

	newJob := func(name string, operations []csiaddonsv1alpha1.ReclaimSpaceOperation) *csiaddonsv1alpha1.ReclaimSpaceJob {
		return &csiaddonsv1alpha1.ReclaimSpaceJob{
			ObjectMeta: v1.ObjectMeta{
				Name:      name,
				Namespace: "default",
			},
			Spec: csiaddonsv1alpha1.ReclaimSpaceJobSpec{
				Target:     csiaddonsv1alpha1.TargetSpec{PersistentVolumeClaim: "data-pvc"},
				Operations: operations,
			},
		}
	}

	ginkgo.It("defaults to the node operation when unset", func() {
		job := newJob("rsjob-operations-unset", nil)
		gomega.Expect(k8sClient.Create(ctx, job)).To(gomega.Succeed())
		ginkgo.DeferCleanup(k8sClient.Delete, ctx, job)

		gomega.Expect(job.Spec.Operations).To(gomega.Equal(csiaddonsv1alpha1.DefaultReclaimSpaceOperations()))
	})

	ginkgo.It("accepts both operations", func() {
		job := newJob("rsjob-operations-both", []csiaddonsv1alpha1.ReclaimSpaceOperation{
			csiaddonsv1alpha1.ReclaimSpaceOperationController,
			csiaddonsv1alpha1.ReclaimSpaceOperationNode,
		})
		gomega.Expect(k8sClient.Create(ctx, job)).To(gomega.Succeed())
		ginkgo.DeferCleanup(k8sClient.Delete, ctx, job)
	})

	// An empty list cannot be expressed through the typed client, the
	// omitempty json tag drops it and the API server applies the default
	// instead. Submit it the way `kubectl apply` would.
	ginkgo.It("rejects an empty list", func() {
		job := &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": csiaddonsv1alpha1.GroupVersion.String(),
				"kind":       "ReclaimSpaceJob",
				"metadata": map[string]any{
					"name":      "rsjob-operations-empty",
					"namespace": "default",
				},
				"spec": map[string]any{
					"target":     map[string]any{"persistentVolumeClaim": "data-pvc"},
					"operations": []any{},
				},
			},
		}
		gomega.Expect(k8sClient.Create(ctx, job)).To(gomega.MatchError(
			gomega.ContainSubstring("should have at least 1 items")))
	})

	ginkgo.It("rejects an unknown operation", func() {
		job := newJob("rsjob-operations-unknown", []csiaddonsv1alpha1.ReclaimSpaceOperation{"Sparsify"})
		gomega.Expect(k8sClient.Create(ctx, job)).To(gomega.MatchError(
			gomega.ContainSubstring(`Unsupported value: "Sparsify": supported values: "Controller", "Node"`)))
	})

	ginkgo.It("rejects duplicate operations", func() {
		job := newJob("rsjob-operations-duplicate", []csiaddonsv1alpha1.ReclaimSpaceOperation{
			csiaddonsv1alpha1.ReclaimSpaceOperationNode,
			csiaddonsv1alpha1.ReclaimSpaceOperationNode,
		})
		gomega.Expect(k8sClient.Create(ctx, job)).To(gomega.MatchError(
			gomega.ContainSubstring("Duplicate value")))
	})

	ginkgo.It("rejects updates to operations", func() {
		job := newJob("rsjob-operations-immutable", []csiaddonsv1alpha1.ReclaimSpaceOperation{
			csiaddonsv1alpha1.ReclaimSpaceOperationNode,
		})
		gomega.Expect(k8sClient.Create(ctx, job)).To(gomega.Succeed())
		ginkgo.DeferCleanup(k8sClient.Delete, ctx, job)

		job.Spec.Operations = []csiaddonsv1alpha1.ReclaimSpaceOperation{
			csiaddonsv1alpha1.ReclaimSpaceOperationController,
		}
		err := k8sClient.Update(ctx, job)
		gomega.Expect(err).To(gomega.HaveOccurred())
		gomega.Expect(err.Error()).To(gomega.ContainSubstring("operations is immutable"))
	})

	ginkgo.It("allows reordering operations", func() {
		job := newJob("rsjob-operations-reorder", []csiaddonsv1alpha1.ReclaimSpaceOperation{
			csiaddonsv1alpha1.ReclaimSpaceOperationNode,
			csiaddonsv1alpha1.ReclaimSpaceOperationController,
		})
		gomega.Expect(k8sClient.Create(ctx, job)).To(gomega.Succeed())
		ginkgo.DeferCleanup(k8sClient.Delete, ctx, job)

		// The list has set semantics, so reordering is not a change.
		job.Spec.Operations = []csiaddonsv1alpha1.ReclaimSpaceOperation{
			csiaddonsv1alpha1.ReclaimSpaceOperationController,
			csiaddonsv1alpha1.ReclaimSpaceOperationNode,
		}
		gomega.Expect(k8sClient.Update(ctx, job)).To(gomega.Succeed())
	})

	ginkgo.It("allows status updates", func() {
		job := newJob("rsjob-operations-status", []csiaddonsv1alpha1.ReclaimSpaceOperation{
			csiaddonsv1alpha1.ReclaimSpaceOperationNode,
		})
		gomega.Expect(k8sClient.Create(ctx, job)).To(gomega.Succeed())
		ginkgo.DeferCleanup(k8sClient.Delete, ctx, job)

		job.Status.Result = csiaddonsv1alpha1.OperationResultSucceeded
		gomega.Expect(k8sClient.Status().Update(ctx, job)).To(gomega.Succeed())
	})
})
