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

package client

import (
	"context"
	"time"

	"github.com/csi-addons/kubernetes-csi-addons/internal/proto"

	"google.golang.org/grpc"
)

type volumeGroupClient struct {
	client  proto.VolumeGroupControllerClient
	timeout time.Duration
}

// VolumeGroup holds the methods required for volume grouping.
type VolumeGroup interface {
	// CreateVolumeGroup RPC call to create a volume group.
	CreateVolumeGroup(volumeGroupName string, volumeIDs []string, secretName, secretNamespace string, parameters map[string]string) (*proto.CreateVolumeGroupResponse, error)
	// ModifyVolumeGroupMembership RPC call to modify the volume group.
	ModifyVolumeGroupMembership(volumeGroupID string, volumeIDs []string, secretName, secretNamespace string) (*proto.ModifyVolumeGroupMembershipResponse, error)
	// DeleteVolumeGroup RPC call to delete the volume group.
	DeleteVolumeGroup(volumeGroupID string, secretName, secretNamespace string) (*proto.DeleteVolumeGroupResponse, error)
	// ControllerGetVolumeGroup RPC call to fetch the volume group.
	ControllerGetVolumeGroup(volumeGroupID string, secretName, secretNamespace string) (*proto.ControllerGetVolumeGroupResponse, error)
}

// NewReplicationClient returns VolumeGroup interface which has the RPC
// calls for grouping.
func NewVolumeGroupClient(cc *grpc.ClientConn, timeout time.Duration) VolumeGroup {
	return &volumeGroupClient{client: proto.NewVolumeGroupControllerClient(cc), timeout: timeout}
}

func (vg *volumeGroupClient) CreateVolumeGroup(volumeGroupName string, volumeIDs []string, secretName, secretNamespace string,
	parameters map[string]string) (*proto.CreateVolumeGroupResponse, error) {
	req := &proto.CreateVolumeGroupRequest{
		Name:            volumeGroupName,
		VolumeIds:       volumeIDs,
		Parameters:      parameters,
		SecretName:      secretName,
		SecretNamespace: secretNamespace,
	}

	createCtx, cancel := context.WithTimeout(context.Background(), vg.timeout)
	defer cancel()
	return vg.client.CreateVolumeGroup(createCtx, req)
}

func (vg *volumeGroupClient) ModifyVolumeGroupMembership(volumeGroupID string, volumeIDs []string,
	secretName, secretNamespace string) (*proto.ModifyVolumeGroupMembershipResponse, error) {
	req := &proto.ModifyVolumeGroupMembershipRequest{
		VolumeGroupId:   volumeGroupID,
		VolumeIds:       volumeIDs,
		SecretName:      secretName,
		SecretNamespace: secretNamespace,
	}

	createCtx, cancel := context.WithTimeout(context.Background(), vg.timeout)
	defer cancel()
	return vg.client.ModifyVolumeGroupMembership(createCtx, req)
}

func (vg *volumeGroupClient) DeleteVolumeGroup(volumeGroupID string,
	secretName, secretNamespace string) (*proto.DeleteVolumeGroupResponse, error) {
	req := &proto.DeleteVolumeGroupRequest{
		VolumeGroupId:   volumeGroupID,
		SecretName:      secretName,
		SecretNamespace: secretNamespace,
	}

	createCtx, cancel := context.WithTimeout(context.Background(), vg.timeout)
	defer cancel()
	return vg.client.DeleteVolumeGroup(createCtx, req)
}

func (vg *volumeGroupClient) ControllerGetVolumeGroup(volumeGroupID string,
	secretName, secretNamespace string) (*proto.ControllerGetVolumeGroupResponse, error) {
	req := &proto.ControllerGetVolumeGroupRequest{
		VolumeGroupId:   volumeGroupID,
		SecretName:      secretName,
		SecretNamespace: secretNamespace,
	}

	createCtx, cancel := context.WithTimeout(context.Background(), vg.timeout)
	defer cancel()
	return vg.client.ControllerGetVolumeGroup(createCtx, req)
}
