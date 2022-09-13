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

	kube "github.com/csi-addons/kubernetes-csi-addons/internal/kubernetes"
	"github.com/csi-addons/kubernetes-csi-addons/internal/proto"
	csiReplication "github.com/csi-addons/spec/lib/go/replication"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
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
	// Get the secrets from the k8s cluster
	data, err := kube.GetSecret(ctx, rs.kubeClient, req.GetSecretName(), req.GetSecretNamespace())
	if err != nil {
		klog.Errorf("Failed to get secret %s in namespace %s: %v", req.GetSecretName(), req.GetSecretNamespace(), err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	_, err = rs.controllerClient.EnableVolumeReplication(ctx,
		&csiReplication.EnableVolumeReplicationRequest{
			VolumeId:      req.VolumeId,
			ReplicationId: req.ReplicationId,
			Parameters:    req.Parameters,
			Secrets:       data,
		})
	if err != nil {
		klog.Errorf("Failed to enable volume replication: %v", err)
		return nil, err
	}

	return &proto.EnableVolumeReplicationResponse{}, nil
}

// DisableVolumeReplication fetches required information from kubernetes cluster and calls
// CSI-Addons DisableVolumeReplication service.
func (rs *ReplicationServer) DisableVolumeReplication(
	ctx context.Context,
	req *proto.DisableVolumeReplicationRequest) (*proto.DisableVolumeReplicationResponse, error) {
	// Get the secrets from the k8s cluster
	data, err := kube.GetSecret(ctx, rs.kubeClient, req.GetSecretName(), req.GetSecretNamespace())
	if err != nil {
		klog.Errorf("Failed to get secret %s in namespace %s: %v", req.GetSecretName(), req.GetSecretNamespace(), err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	_, err = rs.controllerClient.DisableVolumeReplication(ctx,
		&csiReplication.DisableVolumeReplicationRequest{
			VolumeId:      req.VolumeId,
			ReplicationId: req.ReplicationId,
			Parameters:    req.Parameters,
			Secrets:       data,
		})
	if err != nil {
		klog.Errorf("Failed to enable volume replication: %v", err)
		return nil, err
	}

	return &proto.DisableVolumeReplicationResponse{}, nil
}

// PromoteVolume fetches required information from kubernetes cluster and calls
// CSI-Addons PromoteVolume service.
func (rs *ReplicationServer) PromoteVolume(
	ctx context.Context,
	req *proto.PromoteVolumeRequest) (*proto.PromoteVolumeResponse, error) {
	// Get the secrets from the k8s cluster
	data, err := kube.GetSecret(ctx, rs.kubeClient, req.GetSecretName(), req.GetSecretNamespace())
	if err != nil {
		klog.Errorf("Failed to get secret %s in namespace %s: %v", req.GetSecretName(), req.GetSecretNamespace(), err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	_, err = rs.controllerClient.PromoteVolume(ctx,
		&csiReplication.PromoteVolumeRequest{
			VolumeId:      req.VolumeId,
			ReplicationId: req.ReplicationId,
			Force:         req.Force,
			Parameters:    req.Parameters,
			Secrets:       data,
		})
	if err != nil {
		klog.Errorf("Failed to enable volume replication: %v", err)
		return nil, err
	}

	return &proto.PromoteVolumeResponse{}, nil
}

// DemoteVolume fetches required information from kubernetes cluster and calls
// CSI-Addons DemoteVolume service.
func (rs *ReplicationServer) DemoteVolume(
	ctx context.Context,
	req *proto.DemoteVolumeRequest) (*proto.DemoteVolumeResponse, error) {
	// Get the secrets from the k8s cluster
	data, err := kube.GetSecret(ctx, rs.kubeClient, req.GetSecretName(), req.GetSecretNamespace())
	if err != nil {
		klog.Errorf("Failed to get secret %s in namespace %s: %v", req.GetSecretName(), req.GetSecretNamespace(), err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	_, err = rs.controllerClient.DemoteVolume(ctx,
		&csiReplication.DemoteVolumeRequest{
			VolumeId:      req.VolumeId,
			ReplicationId: req.ReplicationId,
			Force:         req.Force,
			Parameters:    req.Parameters,
			Secrets:       data,
		})
	if err != nil {
		klog.Errorf("Failed to enable volume replication: %v", err)
		return nil, err
	}

	return &proto.DemoteVolumeResponse{}, nil
}

// ResyncVolume fetches required information from kubernetes cluster and calls
// CSI-Addons ResyncVolume service.
func (rs *ReplicationServer) ResyncVolume(
	ctx context.Context,
	req *proto.ResyncVolumeRequest) (*proto.ResyncVolumeResponse, error) {
	// Get the secrets from the k8s cluster
	data, err := kube.GetSecret(ctx, rs.kubeClient, req.GetSecretName(), req.GetSecretNamespace())
	if err != nil {
		klog.Errorf("Failed to get secret %s in namespace %s: %v", req.GetSecretName(), req.GetSecretNamespace(), err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	resp, err := rs.controllerClient.ResyncVolume(ctx,
		&csiReplication.ResyncVolumeRequest{
			VolumeId:      req.VolumeId,
			ReplicationId: req.ReplicationId,
			Force:         req.Force,
			Parameters:    req.Parameters,
			Secrets:       data,
		})
	if err != nil {
		klog.Errorf("Failed to enable volume replication: %v", err)
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
	// Get the secrets from the k8s cluster
	data, err := kube.GetSecret(ctx, rs.kubeClient, req.GetSecretName(), req.GetSecretNamespace())
	if err != nil {
		klog.Errorf("Failed to get secret %s in namespace %s: %v", req.GetSecretName(), req.GetSecretNamespace(), err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	resp, err := rs.controllerClient.GetVolumeReplicationInfo(ctx,
		&csiReplication.GetVolumeReplicationInfoRequest{
			VolumeId:      req.VolumeId,
			Secrets:       data,
			ReplicationId: req.ReplicationId,
		})
	if err != nil {
		klog.Errorf("Failed to get volume replication info: %v", err)
		return nil, err
	}

	lastsynctime := resp.GetLastSyncTime()
	if lastsynctime == nil {
		klog.Errorf("Failed to get last sync time: %v", lastsynctime)
	}

	return &proto.GetVolumeReplicationInfoResponse{
		LastSyncTime: lastsynctime,
	}, nil
}
