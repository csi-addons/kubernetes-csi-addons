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

package service

import (
	"context"

	kube "github.com/csi-addons/kubernetes-csi-addons/internal/kubernetes"
	"github.com/csi-addons/kubernetes-csi-addons/internal/proto"
	csiVolumeGroup "github.com/csi-addons/spec/lib/go/volumegroup"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// VolumeGroupServer struct of sidecar with supported methods of proto
// volumegroup server spec and also containing volumegroup
// controller client to csi driver.
type VolumeGroupServer struct {
	proto.UnimplementedVolumeGroupControllerServer
	controllerClient csiVolumeGroup.ControllerClient
	kubeClient       *kubernetes.Clientset
}

// NewVolumeGroupServer creates a new VolumeGroupServer which handles the proto.VolumeGroup
// Service requests.
func NewVolumeGroupServer(c *grpc.ClientConn, kc *kubernetes.Clientset) *VolumeGroupServer {
	return &VolumeGroupServer{
		controllerClient: csiVolumeGroup.NewControllerClient(c),
		kubeClient:       kc,
	}
}

// RegisterService registers service with the server.
func (vg *VolumeGroupServer) RegisterService(server grpc.ServiceRegistrar) {
	proto.RegisterVolumeGroupControllerServer(server, vg)
}

// CreateVolumeGroup calls CSI-Addons CreateVolumeGroup service.
func (vg *VolumeGroupServer) CreateVolumeGroup(
	ctx context.Context,
	req *proto.CreateVolumeGroupRequest) (*proto.CreateVolumeGroupResponse, error) {
	// Get the secrets from the k8s cluster
	data, err := kube.GetSecret(ctx, vg.kubeClient, req.GetSecretName(), req.GetSecretNamespace())
	if err != nil {
		klog.Errorf("Failed to get secret %s in namespace %s: %v", req.GetSecretName(), req.GetSecretNamespace(), err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	// Get the VolumeGroup name and volumeIds from the request
	vgReq := &csiVolumeGroup.CreateVolumeGroupRequest{
		Name:       req.GetName(),
		VolumeIds:  req.GetVolumeIds(),
		Parameters: req.GetParameters(),
		Secrets:    data,
	}

	vgResp, err := vg.controllerClient.CreateVolumeGroup(ctx, vgReq)
	if err != nil {
		klog.Errorf("Failed to create volume group: %v", err)
		return nil, err
	}

	volIds := []string{}
	for _, vol := range vgResp.VolumeGroup.GetVolumes() {
		volIds = append(volIds, vol.VolumeId)
	}

	return &proto.CreateVolumeGroupResponse{
		VolumeGroup: &proto.VolumeGroup{
			VolumeGroupId:      vgResp.VolumeGroup.GetVolumeGroupId(),
			VolumeGroupContext: vgResp.VolumeGroup.GetVolumeGroupContext(),
			VolumeIds:          volIds,
		},
	}, nil
}

// ModifyVolumeGroupMembership calls CSI-Addons ModifyVolumeGroupMembership service.
func (vg *VolumeGroupServer) ModifyVolumeGroupMembership(
	ctx context.Context,
	req *proto.ModifyVolumeGroupMembershipRequest) (*proto.ModifyVolumeGroupMembershipResponse, error) {
	// Get the secrets from the k8s cluster
	data, err := kube.GetSecret(ctx, vg.kubeClient, req.GetSecretName(), req.GetSecretNamespace())
	if err != nil {
		klog.Errorf("Failed to get secret %s in namespace %s: %v", req.GetSecretName(), req.GetSecretNamespace(), err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	// Get the volumeGroup Id and volumeIds from the request
	vgReq := &csiVolumeGroup.ModifyVolumeGroupMembershipRequest{
		VolumeGroupId: req.GetVolumeGroupId(),
		VolumeIds:     req.GetVolumeIds(),
		Secrets:       data,
		Parameters:    req.GetParameters(),
	}

	vgResp, err := vg.controllerClient.ModifyVolumeGroupMembership(ctx, vgReq)
	if err != nil {
		klog.Errorf("Failed to modify volume group: %v", err)
		return nil, err
	}

	volIds := []string{}
	for _, vol := range vgResp.VolumeGroup.GetVolumes() {
		volIds = append(volIds, vol.VolumeId)
	}

	return &proto.ModifyVolumeGroupMembershipResponse{
		VolumeGroup: &proto.VolumeGroup{
			VolumeGroupId:      vgResp.VolumeGroup.GetVolumeGroupId(),
			VolumeGroupContext: vgResp.VolumeGroup.GetVolumeGroupContext(),
			VolumeIds:          volIds,
		},
	}, nil
}

// DeleteVolumeGroup calls CSI-Addons DeleteVolumeGroup service.
func (vg *VolumeGroupServer) DeleteVolumeGroup(
	ctx context.Context,
	req *proto.DeleteVolumeGroupRequest) (*proto.DeleteVolumeGroupResponse, error) {
	// Get the secrets from the k8s cluster
	data, err := kube.GetSecret(ctx, vg.kubeClient, req.GetSecretName(), req.GetSecretNamespace())
	if err != nil {
		klog.Errorf("Failed to get secret %s in namespace %s: %v", req.GetSecretName(), req.GetSecretNamespace(), err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	// Get the volumeGroup Id from the request
	vgReq := &csiVolumeGroup.DeleteVolumeGroupRequest{
		VolumeGroupId: req.GetVolumeGroupId(),
		Secrets:       data,
	}

	_, err = vg.controllerClient.DeleteVolumeGroup(ctx, vgReq)
	if err != nil {
		klog.Errorf("Failed to delete volume group: %v", err)
		return nil, err
	}

	return &proto.DeleteVolumeGroupResponse{}, nil
}

// ControllerGetVolumeGroup calls CSI-Addons ControllerGetVolumeGroup service.
func (vg *VolumeGroupServer) ControllerGetVolumeGroup(
	ctx context.Context,
	req *proto.ControllerGetVolumeGroupRequest) (*proto.ControllerGetVolumeGroupResponse, error) {
	// Get the secrets from the k8s cluster
	data, err := kube.GetSecret(ctx, vg.kubeClient, req.GetSecretName(), req.GetSecretNamespace())
	if err != nil {
		klog.Errorf("Failed to get secret %s in namespace %s: %v", req.GetSecretName(), req.GetSecretNamespace(), err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	// Get the volumeGroup Id from the request
	vgReq := &csiVolumeGroup.ControllerGetVolumeGroupRequest{
		VolumeGroupId: req.GetVolumeGroupId(),
		Secrets:       data,
	}

	vgResp, err := vg.controllerClient.ControllerGetVolumeGroup(ctx, vgReq)
	if err != nil {
		klog.Errorf("Failed to get volume group: %v", err)
		return nil, err
	}

	volIds := []string{}
	for _, vol := range vgResp.VolumeGroup.GetVolumes() {
		volIds = append(volIds, vol.VolumeId)
	}

	return &proto.ControllerGetVolumeGroupResponse{
		VolumeGroup: &proto.VolumeGroup{
			VolumeGroupId:      vgResp.VolumeGroup.GetVolumeGroupId(),
			VolumeGroupContext: vgResp.VolumeGroup.GetVolumeGroupContext(),
			VolumeIds:          volIds,
		},
	}, nil
}
