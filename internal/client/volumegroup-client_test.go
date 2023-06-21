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
	"errors"
	"testing"

	"github.com/csi-addons/kubernetes-csi-addons/internal/client/fake"
	csi "github.com/csi-addons/spec/lib/go/volumegroup"

	"github.com/stretchr/testify/assert"
)

func TestCreateVolumeGroup(t *testing.T) {
	t.Parallel()
	mockedCreateVolumeGroup := &fake.VolumeGroup{
		CreateVolumeGroupMock: func(name string, secrets, parameters map[string]string) (*csi.CreateVolumeGroupResponse, error) {
			return &csi.CreateVolumeGroupResponse{}, nil
		},
	}
	client := mockedCreateVolumeGroup
	resp, err := client.CreateVolumeGroup("", nil, nil)
	assert.Equal(t, &csi.CreateVolumeGroupResponse{}, resp)
	assert.Nil(t, err)

	// return error
	mockedCreateVolumeGroup = &fake.VolumeGroup{
		CreateVolumeGroupMock: func(name string, secrets, parameters map[string]string) (*csi.CreateVolumeGroupResponse, error) {
			return nil, errors.New("failed to enable mirroring")
		},
	}
	client = mockedCreateVolumeGroup
	resp, err = client.CreateVolumeGroup("", nil, nil)
	assert.Nil(t, resp)
	assert.NotNil(t, err)
}

func TestDeleteVolumeGroup(t *testing.T) {
	t.Parallel()
	mockedDeleteVolumeGroup := &fake.VolumeGroup{
		DeleteVolumeGroupMock: func(volumeGroupId string, secrets map[string]string) (*csi.DeleteVolumeGroupResponse, error) {
			return &csi.DeleteVolumeGroupResponse{}, nil
		},
	}
	client := mockedDeleteVolumeGroup
	resp, err := client.DeleteVolumeGroup("", nil)
	assert.Equal(t, &csi.DeleteVolumeGroupResponse{}, resp)
	assert.Nil(t, err)

	// return error
	mockedDeleteVolumeGroup = &fake.VolumeGroup{
		DeleteVolumeGroupMock: func(volumeGroupId string, secrets map[string]string) (*csi.DeleteVolumeGroupResponse, error) {
			return nil, errors.New("failed to disable mirroring")
		},
	}
	client = mockedDeleteVolumeGroup
	resp, err = client.DeleteVolumeGroup("", nil)
	assert.Nil(t, resp)
	assert.NotNil(t, err)
}

func TestModifyVolumeGroup(t *testing.T) {
	t.Parallel()
	// return success response
	mockedModifyVolumeGroupMembership := &fake.VolumeGroup{
		ModifyVolumeGroupMembershipMock: func(volumeGroupId string, volumeIds []string, secrets map[string]string) (*csi.ModifyVolumeGroupMembershipResponse, error) {
			return &csi.ModifyVolumeGroupMembershipResponse{}, nil
		},
	}
	client := mockedModifyVolumeGroupMembership
	resp, err := client.ModifyVolumeGroupMembership("", []string{}, nil)
	assert.Equal(t, &csi.ModifyVolumeGroupMembershipResponse{}, resp)
	assert.Nil(t, err)

	// return error
	mockedModifyVolumeGroupMembership = &fake.VolumeGroup{
		ModifyVolumeGroupMembershipMock: func(volumeGroupId string, volumeIds []string, secrets map[string]string) (*csi.ModifyVolumeGroupMembershipResponse, error) {
			return nil, errors.New("failed to promote volume")
		},
	}
	client = mockedModifyVolumeGroupMembership
	resp, err = client.ModifyVolumeGroupMembership("", []string{}, nil)
	assert.Nil(t, resp)
	assert.NotNil(t, err)
}
