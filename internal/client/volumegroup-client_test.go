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
	"errors"
	"testing"

	"github.com/csi-addons/kubernetes-csi-addons/internal/client/fake"
	"github.com/csi-addons/kubernetes-csi-addons/internal/proto"

	"github.com/stretchr/testify/assert"
)

func TestCreateVolumeGroup(t *testing.T) {
	t.Parallel()
	mockedCreateVolumeGroup := &fake.VolumeGroupClient{
		CreateVolumeGroupMock: func(volumeGroupName string, volumeIDs []string) (*proto.CreateVolumeGroupResponse, error) {
			return &proto.CreateVolumeGroupResponse{}, nil
		},
	}
	client := mockedCreateVolumeGroup
	resp, err := client.CreateVolumeGroup("", nil)
	assert.Equal(t, &proto.CreateVolumeGroupResponse{}, resp)
	assert.Nil(t, err)

	// return error
	mockedCreateVolumeGroup = &fake.VolumeGroupClient{
		CreateVolumeGroupMock: func(volumeGroupName string, volumeIDs []string) (*proto.CreateVolumeGroupResponse, error) {
			return nil, errors.New("failed to create volume group")
		},
	}
	client = mockedCreateVolumeGroup
	resp, err = client.CreateVolumeGroup("", nil)
	assert.Nil(t, resp)
	assert.NotNil(t, err)
}

func TestDeleteVolumeGroup(t *testing.T) {
	t.Parallel()
	mockedDeleteVolumeGroup := &fake.VolumeGroupClient{
		DeleteVolumeGroupMock: func(volumeGroupID string) (*proto.DeleteVolumeGroupResponse, error) {
			return &proto.DeleteVolumeGroupResponse{}, nil
		},
	}
	client := mockedDeleteVolumeGroup
	resp, err := client.DeleteVolumeGroup("")
	assert.Equal(t, &proto.DeleteVolumeGroupResponse{}, resp)
	assert.Nil(t, err)

	// return error
	mockedDeleteVolumeGroup = &fake.VolumeGroupClient{
		DeleteVolumeGroupMock: func(volumeGroupID string) (*proto.DeleteVolumeGroupResponse, error) {
			return nil, errors.New("failed to delete volume group")
		},
	}
	client = mockedDeleteVolumeGroup
	resp, err = client.DeleteVolumeGroup("")
	assert.Nil(t, resp)
	assert.NotNil(t, err)
}

func TestModifyVolumeGroupMembership(t *testing.T) {
	t.Parallel()
	// return success response
	mockedModifyVolumeGroup := &fake.VolumeGroupClient{
		ModifyVolumeGroupMembershipMock: func(volumeGroupID string, volumeIDs []string) (*proto.ModifyVolumeGroupMembershipResponse, error) {
			return &proto.ModifyVolumeGroupMembershipResponse{}, nil
		},
	}
	client := mockedModifyVolumeGroup
	resp, err := client.ModifyVolumeGroupMembership("", nil)
	assert.Equal(t, &proto.ModifyVolumeGroupMembershipResponse{}, resp)
	assert.Nil(t, err)

	// return error
	mockedModifyVolumeGroup = &fake.VolumeGroupClient{
		ModifyVolumeGroupMembershipMock: func(volumeGroupID string, volumeIDs []string) (*proto.ModifyVolumeGroupMembershipResponse, error) {
			return nil, errors.New("failed to modify volume group")
		},
	}
	client = mockedModifyVolumeGroup
	resp, err = client.ModifyVolumeGroupMembership("", nil)
	assert.Nil(t, resp)
	assert.NotNil(t, err)
}

func TestControllerGetVolumeGroup(t *testing.T) {
	t.Parallel()
	// return success response
	mockedGetVolumeGroup := &fake.VolumeGroupClient{
		ControllerGetVolumeGroupMock: func(volumeGroupID string) (*proto.ControllerGetVolumeGroupResponse, error) {
			return &proto.ControllerGetVolumeGroupResponse{}, nil
		},
	}
	client := mockedGetVolumeGroup
	resp, err := client.ControllerGetVolumeGroup("")
	assert.Equal(t, &proto.ControllerGetVolumeGroupResponse{}, resp)
	assert.Nil(t, err)

	// return error
	mockedGetVolumeGroup = &fake.VolumeGroupClient{
		ControllerGetVolumeGroupMock: func(volumeGroupID string) (*proto.ControllerGetVolumeGroupResponse, error) {
			return nil, errors.New("failed to get volume group")
		},
	}
	client = mockedGetVolumeGroup
	resp, err = client.ControllerGetVolumeGroup("")
	assert.Nil(t, resp)
	assert.NotNil(t, err)
}
