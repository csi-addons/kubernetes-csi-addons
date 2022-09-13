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

type replicationClient struct {
	client  proto.ReplicationClient
	timeout time.Duration
}

// VolumeReplication holds the methods required for volume replication.
type VolumeReplication interface {
	// EnableVolumeReplication RPC call to enable the volume replication.
	EnableVolumeReplication(volumeID, replicationID string, secretName, secretNamespace string, parameters map[string]string) (*proto.EnableVolumeReplicationResponse, error)
	// DisableVolumeReplication RPC call to disable the volume replication.
	DisableVolumeReplication(volumeID, replicationID string, secretName, secretNamespace string, parameters map[string]string) (*proto.DisableVolumeReplicationResponse, error)
	// PromoteVolume RPC call to promote the volume.
	PromoteVolume(volumeID, replicationID string, force bool, secretName, secretNamespace string, parameters map[string]string) (*proto.
		PromoteVolumeResponse, error)
	// DemoteVolume RPC call to demote the volume.
	DemoteVolume(volumeID, replicationID string, secretName, secretNamespace string, parameters map[string]string) (*proto.
		DemoteVolumeResponse, error)
	// ResyncVolume RPC call to resync the volume.
	ResyncVolume(volumeID, replicationID string, force bool, secretName, secretNamespace string, parameters map[string]string) (*proto.
		ResyncVolumeResponse, error)
	// GetVolumeReplicationInfo RPC call to get volume replication info.
	GetVolumeReplicationInfo(volumeID, replicationID, secretName, secretNamespace string) (*proto.GetVolumeReplicationInfoResponse, error)
}

// NewReplicationClient returns VolumeReplication interface which has the RPC
// calls for replication.
func NewReplicationClient(cc *grpc.ClientConn, timeout time.Duration) VolumeReplication {
	return &replicationClient{client: proto.NewReplicationClient(cc), timeout: timeout}
}

// EnableVolumeReplication RPC call to enable the volume replication.
func (rc *replicationClient) EnableVolumeReplication(volumeID, replicationID string,
	secretName, secretNamespace string, parameters map[string]string) (*proto.EnableVolumeReplicationResponse, error) {
	req := &proto.EnableVolumeReplicationRequest{
		VolumeId:        volumeID,
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
func (rc *replicationClient) DisableVolumeReplication(volumeID, replicationID string,
	secretName, secretNamespace string, parameters map[string]string) (*proto.DisableVolumeReplicationResponse, error) {
	req := &proto.DisableVolumeReplicationRequest{
		VolumeId:        volumeID,
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
func (rc *replicationClient) PromoteVolume(volumeID, replicationID string,
	force bool, secretName, secretNamespace string, parameters map[string]string) (*proto.PromoteVolumeResponse, error) {
	req := &proto.PromoteVolumeRequest{
		VolumeId:        volumeID,
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
func (rc *replicationClient) DemoteVolume(volumeID, replicationID string,
	secretName, secretNamespace string, parameters map[string]string) (*proto.DemoteVolumeResponse, error) {
	req := &proto.DemoteVolumeRequest{
		VolumeId:        volumeID,
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
func (rc *replicationClient) ResyncVolume(volumeID, replicationID string, force bool,
	secretName, secretNamespace string, parameters map[string]string) (*proto.ResyncVolumeResponse, error) {
	req := &proto.ResyncVolumeRequest{
		VolumeId:        volumeID,
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
func (rc *replicationClient) GetVolumeReplicationInfo(volumeID, replicationID,
	secretName, secretNamespace string) (*proto.GetVolumeReplicationInfoResponse, error) {
	req := &proto.GetVolumeReplicationInfoRequest{
		VolumeId:        volumeID,
		ReplicationId:   replicationID,
		SecretName:      secretName,
		SecretNamespace: secretNamespace,
	}

	createCtx, cancel := context.WithTimeout(context.Background(), rc.timeout)
	defer cancel()
	resp, err := rc.client.GetVolumeReplicationInfo(createCtx, req)

	return resp, err
}
