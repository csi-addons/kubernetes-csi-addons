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

package service

import (
	"context"
	"errors"

	kube "github.com/csi-addons/kubernetes-csi-addons/internal/kubernetes"
	"github.com/csi-addons/kubernetes-csi-addons/internal/proto"
	csiReplication "github.com/csi-addons/spec/lib/go/replication"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ReplicationServer struct of sidecar with supported methods of proto
// replication server spec and also containing replication
// controller client to csi driver.
type ReplicationServer struct {
	proto.UnimplementedReplicationServer
	controllerClient csiReplication.ControllerClient
	kubeClient       *kubernetes.Clientset
}

// NewReplicationServer creates a new ReplicationServer which handles the proto.Replication
// Service requests.
func NewReplicationServer(c *grpc.ClientConn, kc *kubernetes.Clientset) *ReplicationServer {
	return &ReplicationServer{
		controllerClient: csiReplication.NewControllerClient(c),
		kubeClient:       kc,
	}
}

// RegisterService registers service with the server.
func (rs *ReplicationServer) RegisterService(server grpc.ServiceRegistrar) {
	proto.RegisterReplicationServer(server, rs)
}

// EnableVolumeReplication fetches required information from kubernetes cluster and calls
// CSI-Addons EnableVolumeReplication service.
func (rs *ReplicationServer) EnableVolumeReplication(
	ctx context.Context,
	req *proto.EnableVolumeReplicationRequest) (*proto.EnableVolumeReplicationResponse, error) {
	logger := log.FromContext(ctx)
	// Get the secrets from the k8s cluster
	data, err := kube.GetSecret(ctx, rs.kubeClient, req.GetSecretName(), req.GetSecretNamespace())
	if err != nil {
		logger.Error(err, "Failed to get secret", "secretName", req.GetSecretName(), "secretNamespace", req.GetSecretNamespace())
		return nil, status.Error(codes.Internal, err.Error())
	}
	repReq := &csiReplication.EnableVolumeReplicationRequest{
		ReplicationId: req.GetReplicationId(),
		Parameters:    req.GetParameters(),
		Secrets:       data,
	}
	err = setReplicationSource(&repReq.ReplicationSource, req.GetReplicationSource())
	if err != nil {
		logger.Error(err, "Failed to set replication source")
		return nil, status.Error(codes.Internal, err.Error())
	}

	_, err = rs.controllerClient.EnableVolumeReplication(ctx, repReq)

	if err != nil {
		logger.Error(err, "Failed to enable volume replication")
		return nil, err
	}

	return &proto.EnableVolumeReplicationResponse{}, nil
}

// DisableVolumeReplication fetches required information from kubernetes cluster and calls
// CSI-Addons DisableVolumeReplication service.
func (rs *ReplicationServer) DisableVolumeReplication(
	ctx context.Context,
	req *proto.DisableVolumeReplicationRequest) (*proto.DisableVolumeReplicationResponse, error) {
	logger := log.FromContext(ctx)
	// Get the secrets from the k8s cluster
	data, err := kube.GetSecret(ctx, rs.kubeClient, req.GetSecretName(), req.GetSecretNamespace())
	if err != nil {
		logger.Error(err, "Failed to get secret", "secretName", req.GetSecretName(), "secretNamespace", req.GetSecretNamespace())
		return nil, status.Error(codes.Internal, err.Error())
	}

	repReq := &csiReplication.DisableVolumeReplicationRequest{
		ReplicationId: req.GetReplicationId(),
		Parameters:    req.GetParameters(),
		Secrets:       data,
	}
	err = setReplicationSource(&repReq.ReplicationSource, req.GetReplicationSource())
	if err != nil {
		logger.Error(err, "Failed to set replication source")
		return nil, status.Error(codes.Internal, err.Error())
	}

	_, err = rs.controllerClient.DisableVolumeReplication(ctx, repReq)
	if err != nil {
		logger.Error(err, "Failed to disable volume replication")
		return nil, err
	}

	return &proto.DisableVolumeReplicationResponse{}, nil
}

// PromoteVolume fetches required information from kubernetes cluster and calls
// CSI-Addons PromoteVolume service.
func (rs *ReplicationServer) PromoteVolume(
	ctx context.Context,
	req *proto.PromoteVolumeRequest) (*proto.PromoteVolumeResponse, error) {
	logger := log.FromContext(ctx)
	// Get the secrets from the k8s cluster
	data, err := kube.GetSecret(ctx, rs.kubeClient, req.GetSecretName(), req.GetSecretNamespace())
	if err != nil {
		logger.Error(err, "Failed to get secret", "secretName", req.GetSecretName(), "secretNamespace", req.GetSecretNamespace())
		return nil, status.Error(codes.Internal, err.Error())
	}

	repReq := &csiReplication.PromoteVolumeRequest{
		ReplicationId: req.GetReplicationId(),
		Parameters:    req.GetParameters(),
		Force:         req.GetForce(),
		Secrets:       data,
	}
	err = setReplicationSource(&repReq.ReplicationSource, req.GetReplicationSource())
	if err != nil {
		logger.Error(err, "Failed to set replication source")
		return nil, status.Error(codes.Internal, err.Error())
	}

	_, err = rs.controllerClient.PromoteVolume(ctx, repReq)
	if err != nil {
		logger.Error(err, "Failed to promote volume")
		return nil, err
	}

	return &proto.PromoteVolumeResponse{}, nil
}

// DemoteVolume fetches required information from kubernetes cluster and calls
// CSI-Addons DemoteVolume service.
func (rs *ReplicationServer) DemoteVolume(
	ctx context.Context,
	req *proto.DemoteVolumeRequest) (*proto.DemoteVolumeResponse, error) {
	logger := log.FromContext(ctx)
	// Get the secrets from the k8s cluster
	data, err := kube.GetSecret(ctx, rs.kubeClient, req.GetSecretName(), req.GetSecretNamespace())
	if err != nil {
		logger.Error(err, "Failed to get secret", "secretName", req.GetSecretName(), "secretNamespace", req.GetSecretNamespace())
		return nil, status.Error(codes.Internal, err.Error())
	}

	repReq := &csiReplication.DemoteVolumeRequest{
		ReplicationId: req.GetReplicationId(),
		Parameters:    req.GetParameters(),
		Force:         req.GetForce(),
		Secrets:       data,
	}
	err = setReplicationSource(&repReq.ReplicationSource, req.GetReplicationSource())
	if err != nil {
		logger.Error(err, "Failed to set replication source")
		return nil, status.Error(codes.Internal, err.Error())
	}

	_, err = rs.controllerClient.DemoteVolume(ctx, repReq)
	if err != nil {
		logger.Error(err, "Failed to demote volume")
		return nil, err
	}

	return &proto.DemoteVolumeResponse{}, nil
}

// ResyncVolume fetches required information from kubernetes cluster and calls
// CSI-Addons ResyncVolume service.
func (rs *ReplicationServer) ResyncVolume(
	ctx context.Context,
	req *proto.ResyncVolumeRequest) (*proto.ResyncVolumeResponse, error) {
	logger := log.FromContext(ctx)
	// Get the secrets from the k8s cluster
	data, err := kube.GetSecret(ctx, rs.kubeClient, req.GetSecretName(), req.GetSecretNamespace())
	if err != nil {
		logger.Error(err, "Failed to get secret", "secretName", req.GetSecretName(), "secretNamespace", req.GetSecretNamespace())
		return nil, status.Error(codes.Internal, err.Error())
	}

	repReq := &csiReplication.ResyncVolumeRequest{
		ReplicationId: req.GetReplicationId(),
		Parameters:    req.GetParameters(),
		Force:         req.GetForce(),
		Secrets:       data,
	}
	err = setReplicationSource(&repReq.ReplicationSource, req.GetReplicationSource())
	if err != nil {
		logger.Error(err, "Failed to set replication source")
		return nil, status.Error(codes.Internal, err.Error())
	}

	resp, err := rs.controllerClient.ResyncVolume(ctx, repReq)
	if err != nil {
		logger.Error(err, "Failed to resync volume")
		return nil, err
	}

	return &proto.ResyncVolumeResponse{
		Ready: resp.Ready,
	}, nil
}

// GetVolumeReplicationInfo fetches required information from kubernetes cluster and calls
// CSI-Addons GetVolumeReplicationInfo service.
func (rs *ReplicationServer) GetVolumeReplicationInfo(
	ctx context.Context,
	req *proto.GetVolumeReplicationInfoRequest) (*proto.GetVolumeReplicationInfoResponse, error) {
	logger := log.FromContext(ctx)
	// Get the secrets from the k8s cluster
	data, err := kube.GetSecret(ctx, rs.kubeClient, req.GetSecretName(), req.GetSecretNamespace())
	if err != nil {
		logger.Error(err, "Failed to get secret", "secretName", req.GetSecretName(), "secretNamespace", req.GetSecretNamespace())
		return nil, status.Error(codes.Internal, err.Error())
	}

	repReq := &csiReplication.GetVolumeReplicationInfoRequest{
		ReplicationId: req.GetReplicationId(),
		Secrets:       data,
	}
	err = setReplicationSource(&repReq.ReplicationSource, req.GetReplicationSource())
	if err != nil {
		logger.Error(err, "Failed to set replication source")
		return nil, status.Error(codes.Internal, err.Error())
	}

	resp, err := rs.controllerClient.GetVolumeReplicationInfo(ctx, repReq)
	if err != nil {
		logger.Error(err, "Failed to get volume replication info")
		return nil, err
	}

	lastsynctime := resp.GetLastSyncTime()
	if lastsynctime == nil {
		logger.Info("Last sync time is nil in volume replication info response")
	}

	return &proto.GetVolumeReplicationInfoResponse{
		LastSyncTime:     lastsynctime,
		LastSyncDuration: resp.GetLastSyncDuration(),
		LastSyncBytes:    resp.GetLastSyncBytes(),
		Status:           proto.GetVolumeReplicationInfoResponse_Status(resp.GetStatus()),
		StatusMessage:    resp.StatusMessage,
	}, nil
}

// setReplicationSource sets the replication source for the given ReplicationSource.
func setReplicationSource(src **csiReplication.ReplicationSource, req *proto.ReplicationSource) error {
	if *src == nil {
		*src = &csiReplication.ReplicationSource{}
	}

	switch {
	case req == nil:
		return errors.New("replication source is required")
	case req.GetVolume() == nil && req.GetVolumeGroup() == nil:
		return errors.New("either volume or volume group is required")
	case req.GetVolume() != nil:
		(*src).Type = &csiReplication.ReplicationSource_Volume{Volume: &csiReplication.ReplicationSource_VolumeSource{
			VolumeId: req.GetVolume().GetVolumeId(),
		}}
		return nil
	case req.GetVolumeGroup() != nil:
		(*src).Type = &csiReplication.ReplicationSource_Volumegroup{Volumegroup: &csiReplication.ReplicationSource_VolumeGroupSource{
			VolumeGroupId: req.GetVolumeGroup().GetVolumeGroupId(),
		}}
		return nil
	}
	return errors.New("either volume or volume group is required")
}
