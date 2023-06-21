/*
Copyright 2023 The Kubernetes-CSI-Addons Authors.

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

	csi "github.com/csi-addons/spec/lib/go/volumegroup"

	"google.golang.org/grpc"
	"k8s.io/klog/v2"
)

// VolumeGroupServer struct of sidecar with supported methods of csiAddons
// volumeGroup server spec and also containing volumeGroup
// controller client to csi driver.
type VolumeGroupServer struct {
	csi.UnimplementedControllerServer
	controllerClient csi.ControllerClient
}

// NewVolumeGroupServer creates a new VolumeGroupServer which handles the csi.volumeGroup
// Service requests.
func NewVolumeGroupServer(c *grpc.ClientConn) *VolumeGroupServer {
	return &VolumeGroupServer{
		controllerClient: csi.NewControllerClient(c),
	}
}

// RegisterService registers service with the server.
func (vg *VolumeGroupServer) RegisterService(server grpc.ServiceRegistrar) {
	csi.RegisterControllerServer(server, vg)
}

// CreateVolumeGroup fetches required information from kubernetes cluster and calls
// CSI-Addons CreateVolumeGroup service.
func (vg *VolumeGroupServer) CreateVolumeGroup(ctx context.Context,
	req *csi.CreateVolumeGroupRequest) (*csi.CreateVolumeGroupResponse, error) {
	resp, err := vg.controllerClient.CreateVolumeGroup(ctx, req)
	if err != nil {
		klog.Errorf("Failed to create volume group: %v", err)
		return nil, err
	}

	return resp, nil
}

// DeleteVolumeGroup fetches required information from kubernetes cluster and calls
// CSI-Addons DeleteVolumeGroup service.
func (vg *VolumeGroupServer) DeleteVolumeGroup(ctx context.Context,
	req *csi.DeleteVolumeGroupRequest) (*csi.DeleteVolumeGroupResponse, error) {
	resp, err := vg.controllerClient.DeleteVolumeGroup(ctx, req)
	if err != nil {
		klog.Errorf("Failed to delete volume group: %v", err)
		return nil, err
	}

	return resp, nil
}

// ModifyVolumeGroupMembership fetches required information from kubernetes cluster and calls
// CSI-Addons ModifyVolumeGroupMembership service.
func (vg *VolumeGroupServer) ModifyVolumeGroupMembership(ctx context.Context,
	req *csi.ModifyVolumeGroupMembershipRequest) (*csi.ModifyVolumeGroupMembershipResponse, error) {
	resp, err := vg.controllerClient.ModifyVolumeGroupMembership(ctx, req)
	if err != nil {
		klog.Errorf("Failed to modify volume group: %v", err)
		return nil, err
	}

	return resp, nil
}

// ListVolumeGroups fetches required information from kubernetes cluster and calls
// CSI-Addons ListVolumeGroups service.
func (vg *VolumeGroupServer) ListVolumeGroups(ctx context.Context,
	req *csi.ListVolumeGroupsRequest) (*csi.ListVolumeGroupsResponse, error) {
	resp, err := vg.controllerClient.ListVolumeGroups(ctx, req)
	if err != nil {
		klog.Errorf("Failed to demote volume: %v", err)
		return nil, err
	}

	return resp, nil
}

// ControllerGetVolumeGroup fetches required information from kubernetes cluster and calls
// CSI-Addons ControllerGetVolumeGroup service.
func (vg *VolumeGroupServer) ControllerGetVolumeGroup(ctx context.Context,
	req *csi.ControllerGetVolumeGroupRequest) (*csi.ControllerGetVolumeGroupResponse, error) {
	resp, err := vg.controllerClient.ControllerGetVolumeGroup(ctx, req)
	if err != nil {
		klog.Errorf("Failed to resync volume: %v", err)
		return nil, err
	}

	return resp, nil
}
