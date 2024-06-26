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
	"context"
	"time"

	"github.com/csi-addons/kubernetes-csi-addons/internal/proto"

	"google.golang.org/grpc"
)

type volumeReplicationClient struct {
	client  proto.ReplicationClient
	timeout time.Duration
}

var _ VolumeReplication = &volumeReplicationClient{}

// NewReplicationClient returns VolumeReplication interface which has the RPC
// calls for replication.
func NewVolumeReplicationClient(cc *grpc.ClientConn, timeout time.Duration) VolumeReplication {
	return &volumeReplicationClient{client: proto.NewReplicationClient(cc), timeout: timeout}
}

// EnableVolumeReplication RPC call to enable the volume replication.
func (rc *volumeReplicationClient) EnableVolumeReplication(volumeID, replicationID string,
	secretName, secretNamespace string, parameters map[string]string) (*proto.EnableVolumeReplicationResponse, error) {
	req := &proto.EnableVolumeReplicationRequest{
		ReplicationSource: &proto.ReplicationSource{
			Type: &proto.ReplicationSource_Volume{
				Volume: &proto.ReplicationSource_VolumeSource{
					VolumeId: volumeID,
				},
			},
		},
		ReplicationId:   replicationID,
		Parameters:      parameters,
		SecretName:      secretName,
		SecretNamespace: secretNamespace,
	}

	createCtx, cancel := context.WithTimeout(context.Background(), rc.timeout)
	defer cancel()
	resp, err := rc.client.EnableVolumeReplication(createCtx, req)

	return resp, err
}

// DisableVolumeReplication RPC call to disable the volume replication.
func (rc *volumeReplicationClient) DisableVolumeReplication(volumeID, replicationID string,
	secretName, secretNamespace string, parameters map[string]string) (*proto.DisableVolumeReplicationResponse, error) {
	req := &proto.DisableVolumeReplicationRequest{
		ReplicationSource: &proto.ReplicationSource{
			Type: &proto.ReplicationSource_Volume{
				Volume: &proto.ReplicationSource_VolumeSource{
					VolumeId: volumeID,
				},
			},
		},
		ReplicationId:   replicationID,
		Parameters:      parameters,
		SecretName:      secretName,
		SecretNamespace: secretNamespace,
	}

	createCtx, cancel := context.WithTimeout(context.Background(), rc.timeout)
	defer cancel()
	resp, err := rc.client.DisableVolumeReplication(createCtx, req)

	return resp, err
}

// PromoteVolume RPC call to promote the volume.
func (rc *volumeReplicationClient) PromoteVolume(volumeID, replicationID string,
	force bool, secretName, secretNamespace string, parameters map[string]string) (*proto.PromoteVolumeResponse, error) {
	req := &proto.PromoteVolumeRequest{
		ReplicationSource: &proto.ReplicationSource{
			Type: &proto.ReplicationSource_Volume{
				Volume: &proto.ReplicationSource_VolumeSource{
					VolumeId: volumeID,
				},
			},
		},
		ReplicationId:   replicationID,
		Force:           force,
		Parameters:      parameters,
		SecretName:      secretName,
		SecretNamespace: secretNamespace,
	}

	createCtx, cancel := context.WithTimeout(context.Background(), rc.timeout)
	defer cancel()
	resp, err := rc.client.PromoteVolume(createCtx, req)

	return resp, err
}

// DemoteVolume RPC call to demote the volume.
func (rc *volumeReplicationClient) DemoteVolume(volumeID, replicationID string,
	secretName, secretNamespace string, parameters map[string]string) (*proto.DemoteVolumeResponse, error) {
	req := &proto.DemoteVolumeRequest{
		ReplicationSource: &proto.ReplicationSource{
			Type: &proto.ReplicationSource_Volume{
				Volume: &proto.ReplicationSource_VolumeSource{
					VolumeId: volumeID,
				},
			},
		},
		ReplicationId:   replicationID,
		Parameters:      parameters,
		SecretName:      secretName,
		SecretNamespace: secretNamespace,
	}
	createCtx, cancel := context.WithTimeout(context.Background(), rc.timeout)
	defer cancel()
	resp, err := rc.client.DemoteVolume(createCtx, req)

	return resp, err
}

// ResyncVolume RPC call to resync the volume.
func (rc *volumeReplicationClient) ResyncVolume(volumeID, replicationID string, force bool,
	secretName, secretNamespace string, parameters map[string]string) (*proto.ResyncVolumeResponse, error) {
	req := &proto.ResyncVolumeRequest{
		ReplicationSource: &proto.ReplicationSource{
			Type: &proto.ReplicationSource_Volume{
				Volume: &proto.ReplicationSource_VolumeSource{
					VolumeId: volumeID,
				},
			},
		},
		ReplicationId:   replicationID,
		Parameters:      parameters,
		Force:           force,
		SecretName:      secretName,
		SecretNamespace: secretNamespace,
	}

	createCtx, cancel := context.WithTimeout(context.Background(), rc.timeout)
	defer cancel()
	resp, err := rc.client.ResyncVolume(createCtx, req)

	return resp, err
}

// GetVolumeReplicationInfo RPC call to get volume replication info.
func (rc *volumeReplicationClient) GetVolumeReplicationInfo(volumeID, replicationID,
	secretName, secretNamespace string) (*proto.GetVolumeReplicationInfoResponse, error) {
	req := &proto.GetVolumeReplicationInfoRequest{
		ReplicationSource: &proto.ReplicationSource{
			Type: &proto.ReplicationSource_Volume{
				Volume: &proto.ReplicationSource_VolumeSource{
					VolumeId: volumeID,
				},
			},
		},
		ReplicationId:   replicationID,
		SecretName:      secretName,
		SecretNamespace: secretNamespace,
	}

	createCtx, cancel := context.WithTimeout(context.Background(), rc.timeout)
	defer cancel()
	resp, err := rc.client.GetVolumeReplicationInfo(createCtx, req)

	return resp, err
}
