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

package fake

import "github.com/csi-addons/kubernetes-csi-addons/internal/proto"

// VolumeGroupClient to fake grouping operations.
type VolumeGroupClient struct {
	// CreateVolumeGroupMock mocks CreateVolumeGroup RPC call.
	CreateVolumeGroupMock func(volumeGroupName string, volumeIDs []string) (*proto.CreateVolumeGroupResponse, error)
	// ModifyVolumeGroupMembershipMock mock ModifyVolumeGroupMembership RPC call.
	ModifyVolumeGroupMembershipMock func(volumeGroupID string, volumeIDs []string) (*proto.ModifyVolumeGroupMembershipResponse, error)
	// DeleteVolumeGroupMock mocks DeleteVolumeGroup RPC call.
	DeleteVolumeGroupMock func(volumeGroupID string) (*proto.DeleteVolumeGroupResponse, error)
	// ControllerGetVolumeGroupMock mocks ControllerGetVolumeGroup RPC call.
	ControllerGetVolumeGroupMock func(volumeGroupID string) (*proto.ControllerGetVolumeGroupResponse, error)
}

// CreateVolumeGroup calls CreateVolumeGroupMock mock function.
func (vg *VolumeGroupClient) CreateVolumeGroup(volumeGroupName string, volumeIDs []string) (*proto.CreateVolumeGroupResponse, error) {
	return vg.CreateVolumeGroupMock(volumeGroupName, volumeIDs)
}

// ModifyVolumeGroupMembership calls ModifyVolumeGroupMembership mock function.
func (vg *VolumeGroupClient) ModifyVolumeGroupMembership(volumeGroupID string, volumeIDs []string) (*proto.ModifyVolumeGroupMembershipResponse, error) {
	return vg.ModifyVolumeGroupMembershipMock(volumeGroupID, volumeIDs)
}

// DeleteVolumeGroup calls DeleteVolumeGroup mock function.
func (vg *VolumeGroupClient) DeleteVolumeGroup(volumeGroupID string) (*proto.DeleteVolumeGroupResponse, error) {
	return vg.DeleteVolumeGroupMock(volumeGroupID)
}

// ControllerGetVolumeGroup calls ControllerGetVolumeGroup mock function.
func (vg *VolumeGroupClient) ControllerGetVolumeGroup(volumeGroupID string) (*proto.ControllerGetVolumeGroupResponse, error) {
	return vg.ControllerGetVolumeGroupMock(volumeGroupID)
}
