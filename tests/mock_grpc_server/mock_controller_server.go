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

package mock_grpc_server

import (
	"context"

	csi "github.com/csi-addons/spec/lib/go/volumegroup"
)

type MockControllerServer struct {
	*csi.UnimplementedControllerServer
}

func (MockControllerServer) CreateVolumeGroup(context.Context, *csi.CreateVolumeGroupRequest) (*csi.CreateVolumeGroupResponse, error) {
	return &csi.CreateVolumeGroupResponse{
		VolumeGroup: &csi.VolumeGroup{
			VolumeGroupId: "test",
			VolumeGroupContext: map[string]string{
				"fake-label": "fake-value",
			},
		},
	}, nil
}

func (MockControllerServer) DeleteVolumeGroup(context.Context, *csi.DeleteVolumeGroupRequest) (*csi.DeleteVolumeGroupResponse, error) {
	return &csi.DeleteVolumeGroupResponse{}, nil
}

func (MockControllerServer) ModifyVolumeGroupMembership(context.Context, *csi.ModifyVolumeGroupMembershipRequest) (*csi.ModifyVolumeGroupMembershipResponse, error) {
	return &csi.ModifyVolumeGroupMembershipResponse{}, nil
}

func (MockControllerServer) ListVolumeGroups(context.Context, *csi.ListVolumeGroupsRequest) (*csi.ListVolumeGroupsResponse, error) {
	return &csi.ListVolumeGroupsResponse{}, nil
}
func (MockControllerServer) ControllerGetVolumeGroup(context.Context, *csi.ControllerGetVolumeGroupRequest) (*csi.ControllerGetVolumeGroupResponse, error) {
	return &csi.ControllerGetVolumeGroupResponse{}, nil
}
