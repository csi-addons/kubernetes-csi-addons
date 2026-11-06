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

func TestCanNodeReclaimSpace(t *testing.T) {
	tests := []struct {
		name string
		td   targetDetails
		want bool
	}{
		{
			name: "empty nodeID",
			td: targetDetails{
				driverName: "csi.example.com",
				pvName:     "pvc-a8a5c531-9f88-4fc8-b35d-564585fb42a8",
				nodeID:     "",
			},
			want: false,
		},
		{
			name: "non-empty nodeID",
			td: targetDetails{
				driverName: "csi.example.com",
				pvName:     "pvc-a8a5c531-9f88-4fc8-b35d-564585fb42a8",
				nodeID:     "worker-1",
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.td.canNodeReclaimSpace()
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

// newReclaimSpaceTestReconciler returns a reconciler whose connection pool
// contains a controller and a node connection to the provided fake server.
func newReclaimSpaceTestReconciler(
	t *testing.T,
	serverImpl *fakeReclaimSpaceServer,
	attached bool) *ReclaimSpaceJobReconciler {
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
	if attached {
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

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()

	pool := connection.NewConnectionPool()
	pool.Put(rsTestNamespace+"/"+rsTestLeader, &connection.Connection{
		Client:       cc,
		Capabilities: []*identity.Capability{reclaimSpaceCapability(identity.Capability_ReclaimSpace_OFFLINE)},
		Namespace:    rsTestNamespace,
		Name:         rsTestLeader,
		DriverName:   rsTestDriver,
		Timeout:      time.Minute,
	})
	pool.Put(rsTestNamespace+"/node-pod", &connection.Connection{
		Client:       cc,
		Capabilities: []*identity.Capability{reclaimSpaceCapability(identity.Capability_ReclaimSpace_ONLINE)},
		Namespace:    rsTestNamespace,
		Name:         "node-pod",
		NodeID:       rsTestNodeID,
		DriverName:   rsTestDriver,
		Timeout:      time.Minute,
	})

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

	tests := []struct {
		name                  string
		operations            []csiaddonsv1alpha1.ReclaimSpaceOperation
		attached              bool
		nodeErr               error
		controllerErr         error
		wantNodeCalls         int
		wantControllerCalls   int
		wantErr               error
		wantResult            csiaddonsv1alpha1.OperationResult
		wantMessageSubstrings []string
	}{
		{
			name:                "default operations only run node reclaim",
			operations:          csiaddonsv1alpha1.DefaultReclaimSpaceOperations(),
			attached:            true,
			wantNodeCalls:       1,
			wantControllerCalls: 0,
			wantResult:          csiaddonsv1alpha1.OperationResultSucceeded,
		},
		{
			name:                "controller only never runs node reclaim",
			operations:          []csiaddonsv1alpha1.ReclaimSpaceOperation{csiaddonsv1alpha1.ReclaimSpaceOperationController},
			attached:            true,
			wantNodeCalls:       0,
			wantControllerCalls: 1,
			wantResult:          csiaddonsv1alpha1.OperationResultSucceeded,
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
		},
		{
			name:                "node only on an unattached volume requeues",
			operations:          []csiaddonsv1alpha1.ReclaimSpaceOperation{csiaddonsv1alpha1.ReclaimSpaceOperationNode},
			attached:            false,
			wantNodeCalls:       0,
			wantControllerCalls: 0,
			wantErr:             errVolumeNotAttached,
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
				"NodeReclaimSpace is not implemented",
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
				"ControllerReclaimSpace is not implemented",
			},
		},
		{
			name: "both on an unattached volume succeed via controller and report the skip",
			operations: []csiaddonsv1alpha1.ReclaimSpaceOperation{
				csiaddonsv1alpha1.ReclaimSpaceOperationNode,
				csiaddonsv1alpha1.ReclaimSpaceOperationController,
			},
			attached:            false,
			wantNodeCalls:       0,
			wantControllerCalls: 1,
			wantResult:          csiaddonsv1alpha1.OperationResultSucceeded,
			wantMessageSubstrings: []string{
				"successfully completed",
				"not attached to any node",
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
				"NodeReclaimSpace is not implemented",
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
			reconciler := newReclaimSpaceTestReconciler(t, serverImpl, tt.attached)

			job := &csiaddonsv1alpha1.ReclaimSpaceJob{
				ObjectMeta: v1.ObjectMeta{
					Name:              "rs-job",
					Namespace:         rsTestNamespace,
					CreationTimestamp: v1.NewTime(time.Now()),
				},
				Spec: csiaddonsv1alpha1.ReclaimSpaceJobSpec{
					Target:               csiaddonsv1alpha1.TargetSpec{PersistentVolumeClaim: rsTestPVCName},
					Operations:           tt.operations,
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
})
