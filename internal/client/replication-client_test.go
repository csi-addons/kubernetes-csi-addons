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

package client

import (
	"errors"
	"testing"

	"github.com/csi-addons/kubernetes-csi-addons/internal/client/fake"
	"github.com/csi-addons/kubernetes-csi-addons/internal/proto"
	csiReplication "github.com/csi-addons/spec/lib/go/replication"

	"github.com/stretchr/testify/assert"
)

func TestEnableVolumeReplication(t *testing.T) {
	t.Parallel()
	mockedEnableReplication := &fake.ReplicationClient{
		EnableVolumeReplicationMock: func(replicationSource *csiReplication.ReplicationSource, replicationID string, secretName, secretNamespace string, parameters map[string]string) (*proto.EnableVolumeReplicationResponse, error) {
			return &proto.EnableVolumeReplicationResponse{}, nil
		},
	}
	client := mockedEnableReplication
	resp, err := client.EnableVolumeReplication(nil, "", "", "", nil)
	assert.Equal(t, &proto.EnableVolumeReplicationResponse{}, resp)
	assert.Nil(t, err)

	// return error
	mockedEnableReplication = &fake.ReplicationClient{
		EnableVolumeReplicationMock: func(replicationSource *csiReplication.ReplicationSource, replicationID string, secretName, secretNamespace string, parameters map[string]string) (*proto.EnableVolumeReplicationResponse, error) {
			return nil, errors.New("failed to enable mirroring")
		},
	}
	client = mockedEnableReplication
	resp, err = client.EnableVolumeReplication(nil, "", "", "", nil)
	assert.Nil(t, resp)
	assert.NotNil(t, err)
}

func TestDisableVolumeReplication(t *testing.T) {
	t.Parallel()
	mockedDisableReplication := &fake.ReplicationClient{
		DisableVolumeReplicationMock: func(replicationSource *csiReplication.ReplicationSource, replicationID string, secretName, secretNamespace string, parameters map[string]string) (*proto.DisableVolumeReplicationResponse, error) {
			return &proto.DisableVolumeReplicationResponse{}, nil
		},
	}
	client := mockedDisableReplication
	resp, err := client.DisableVolumeReplication(nil, "", "", "", nil)
	assert.Equal(t, &proto.DisableVolumeReplicationResponse{}, resp)
	assert.Nil(t, err)

	// return error
	mockedDisableReplication = &fake.ReplicationClient{
		DisableVolumeReplicationMock: func(replicationSource *csiReplication.ReplicationSource, replicationID string, secretName, secretNamespace string, parameters map[string]string) (*proto.DisableVolumeReplicationResponse, error) {
			return nil, errors.New("failed to disable mirroring")
		},
	}
	client = mockedDisableReplication
	resp, err = client.DisableVolumeReplication(nil, "", "", "", nil)
	assert.Nil(t, resp)
	assert.NotNil(t, err)
}

func TestPromoteVolume(t *testing.T) {
	t.Parallel()
	// return success response
	mockedPromoteVolume := &fake.ReplicationClient{
		PromoteVolumeMock: func(replicationSource *csiReplication.ReplicationSource, replicationID string, force bool, secretName, secretNamespace string, parameters map[string]string) (*proto.PromoteVolumeResponse, error) {
			return &proto.PromoteVolumeResponse{}, nil
		},
	}
	force := false
	client := mockedPromoteVolume
	resp, err := client.PromoteVolume(nil, "", force, "", "", nil)
	assert.Equal(t, &proto.PromoteVolumeResponse{}, resp)
	assert.Nil(t, err)

	// return error
	mockedPromoteVolume = &fake.ReplicationClient{
		PromoteVolumeMock: func(replicationSource *csiReplication.ReplicationSource, replicationID string, force bool, secretName, secretNamespace string, parameters map[string]string) (*proto.PromoteVolumeResponse, error) {
			return nil, errors.New("failed to promote volume")
		},
	}
	client = mockedPromoteVolume
	resp, err = client.PromoteVolume(nil, "", force, "", "", nil)
	assert.Nil(t, resp)
	assert.NotNil(t, err)
}

func TestDemoteVolume(t *testing.T) {
	t.Parallel()
	// return success response
	mockedDemoteVolume := &fake.ReplicationClient{
		DemoteVolumeMock: func(replicationSource *csiReplication.ReplicationSource, replicationID string, secretName, secretNamespace string, parameters map[string]string) (*proto.DemoteVolumeResponse, error) {
			return &proto.DemoteVolumeResponse{}, nil
		},
	}
	client := mockedDemoteVolume
	resp, err := client.DemoteVolume(nil, "", "", "", nil)
	assert.Equal(t, &proto.DemoteVolumeResponse{}, resp)
	assert.Nil(t, err)

	// return error
	mockedDemoteVolume = &fake.ReplicationClient{
		DemoteVolumeMock: func(replicationSource *csiReplication.ReplicationSource, replicationID string, secretName, secretNamespace string, parameters map[string]string) (*proto.DemoteVolumeResponse, error) {
			return nil, errors.New("failed to demote volume")
		},
	}
	client = mockedDemoteVolume
	resp, err = client.DemoteVolume(nil, "", "", "", nil)
	assert.Nil(t, resp)
	assert.NotNil(t, err)
}

func TestResyncVolume(t *testing.T) {
	t.Parallel()
	// return success response
	mockedResyncVolume := &fake.ReplicationClient{
		ResyncVolumeMock: func(replicationSource *csiReplication.ReplicationSource, replicationID string, secretName, secretNamespace string, parameters map[string]string) (*proto.ResyncVolumeResponse, error) {
			return &proto.ResyncVolumeResponse{}, nil
		},
	}
	client := mockedResyncVolume
	resp, err := client.ResyncVolume(nil, "", "", "", nil)
	assert.Equal(t, &proto.ResyncVolumeResponse{}, resp)
	assert.Nil(t, err)

	// return error
	mockedResyncVolume = &fake.ReplicationClient{
		ResyncVolumeMock: func(replicationSource *csiReplication.ReplicationSource, replicationID string, secretName, secretNamespace string, parameters map[string]string) (*proto.ResyncVolumeResponse, error) {
			return nil, errors.New("failed to resync volume")
		},
	}
	client = mockedResyncVolume
	resp, err = client.ResyncVolume(nil, "", "", "", nil)
	assert.Nil(t, resp)
	assert.NotNil(t, err)
}
