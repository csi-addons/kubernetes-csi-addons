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

package fake

import (
	"time"

	csi "github.com/csi-addons/spec/lib/go/volumegroup"
	"google.golang.org/grpc"
)

type VolumeGroup struct {
	NewVolumeGroupClient            func(cc *grpc.ClientConn, timeout time.Duration) VolumeGroup
	CreateVolumeGroupMock           func(name string, secrets, parameters map[string]string) (*csi.CreateVolumeGroupResponse, error)
	DeleteVolumeGroupMock           func(volumeGroupId string, secrets map[string]string) (*csi.DeleteVolumeGroupResponse, error)
	ModifyVolumeGroupMembershipMock func(volumeGroupId string, volumeIds []string, secrets map[string]string) (*csi.ModifyVolumeGroupMembershipResponse, error)
}

func (v VolumeGroup) CreateVolumeGroup(name string, secrets, parameters map[string]string) (*csi.CreateVolumeGroupResponse, error) {
	return v.CreateVolumeGroupMock(name, secrets, parameters)
}

func (v VolumeGroup) DeleteVolumeGroup(volumeGroupId string, secrets map[string]string) (*csi.DeleteVolumeGroupResponse, error) {
	return v.DeleteVolumeGroupMock(volumeGroupId, secrets)
}

func (v VolumeGroup) ModifyVolumeGroupMembership(volumeGroupId string, volumeIds []string, secrets map[string]string) (*csi.ModifyVolumeGroupMembershipResponse, error) {
	return v.ModifyVolumeGroupMembershipMock(volumeGroupId, volumeIds, secrets)
}
