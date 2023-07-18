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

package client

import (
	"context"
	"time"

	csi "github.com/csi-addons/spec/lib/go/volumegroup"
	"google.golang.org/grpc"
)

type volumeGroupClient struct {
	client  csi.ControllerClient
	timeout time.Duration
}

type VolumeGroup interface {
	CreateVolumeGroup(name string, secrets, parameters map[string]string) (*csi.CreateVolumeGroupResponse, error)
	DeleteVolumeGroup(volumeGroupId string, secrets map[string]string) (*csi.DeleteVolumeGroupResponse, error)
	ModifyVolumeGroupMembership(volumeGroupId string, volumeIds []string, secrets map[string]string) (*csi.ModifyVolumeGroupMembershipResponse, error)
}

func NewVolumeGroupClient(cc *grpc.ClientConn, timeout time.Duration) VolumeGroup {
	return &volumeGroupClient{client: csi.NewControllerClient(cc), timeout: timeout}
}

func (rc *volumeGroupClient) CreateVolumeGroup(name string, secrets, parameters map[string]string) (*csi.CreateVolumeGroupResponse, error) {
	req := &csi.CreateVolumeGroupRequest{
		Name:       name,
		Parameters: parameters,
		Secrets:    secrets,
	}

	createCtx, cancel := context.WithTimeout(context.Background(), rc.timeout)
	defer cancel()
	resp, err := rc.client.CreateVolumeGroup(createCtx, req)

	return resp, err
}

func (rc *volumeGroupClient) DeleteVolumeGroup(volumeGroupId string, secrets map[string]string) (*csi.DeleteVolumeGroupResponse, error) {
	req := &csi.DeleteVolumeGroupRequest{
		VolumeGroupId: volumeGroupId,
		Secrets:       secrets,
	}

	createCtx, cancel := context.WithTimeout(context.Background(), rc.timeout)
	defer cancel()
	resp, err := rc.client.DeleteVolumeGroup(createCtx, req)

	return resp, err
}

func (rc *volumeGroupClient) ModifyVolumeGroupMembership(volumeGroupId string, volumeIds []string, secrets map[string]string) (*csi.ModifyVolumeGroupMembershipResponse, error) {
	req := &csi.ModifyVolumeGroupMembershipRequest{
		VolumeGroupId: volumeGroupId,
		VolumeIds:     volumeIds,
		Secrets:       secrets,
	}

	createCtx, cancel := context.WithTimeout(context.Background(), rc.timeout)
	defer cancel()
	resp, err := rc.client.ModifyVolumeGroupMembership(createCtx, req)

	return resp, err
}
