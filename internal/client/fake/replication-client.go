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

package fake

import (
	"github.com/csi-addons/kubernetes-csi-addons/internal/proto"
	csiReplication "github.com/csi-addons/spec/lib/go/replication"
)

// ReplicationClient to fake replication operations.
type ReplicationClient struct {
	// EnableVolumeReplicationMock mocks EnableVolumeReplication RPC call.
	EnableVolumeReplicationMock func(replicationSource *csiReplication.ReplicationSource, replicationID string, secretName, secretNamespace string, parameters map[string]string) (*proto.EnableVolumeReplicationResponse, error)
	// DisableVolumeReplicationMock mocks DisableVolumeReplication RPC call.
	DisableVolumeReplicationMock func(replicationSource *csiReplication.ReplicationSource, replicationID string, secretName, secretNamespace string, parameters map[string]string) (*proto.DisableVolumeReplicationResponse, error)
	// PromoteVolumeMock mocks PromoteVolume RPC call.
	PromoteVolumeMock func(replicationSource *csiReplication.ReplicationSource, replicationID string, force bool, secretName, secretNamespace string, parameters map[string]string) (*proto.PromoteVolumeResponse, error)
	// DemoteVolumeMock mocks DemoteVolume RPC call.
	DemoteVolumeMock func(replicationSource *csiReplication.ReplicationSource, replicationID string, secretName, secretNamespace string, parameters map[string]string) (*proto.DemoteVolumeResponse, error)
	// ResyncVolumeMock mocks ResyncVolume RPC call.
	ResyncVolumeMock func(replicationSource *csiReplication.ReplicationSource, replicationID string, secretName, secretNamespace string, parameters map[string]string) (*proto.ResyncVolumeResponse, error)
	// GetVolumeReplicationInfo mocks GetVolumeReplicationInfo RPC call.
	GetVolumeReplicationInfoMock func(replicationSource *csiReplication.ReplicationSource, replicationID string, secretName, secretNamespace string) (*proto.GetVolumeReplicationInfoResponse, error)
}

// EnableVolumeReplication calls EnableVolumeReplicationMock mock function.
func (rc *ReplicationClient) EnableVolumeReplication(
	replicationSource *csiReplication.ReplicationSource,
	replicationID string,
	secretName, secretNamespace string,
	parameters map[string]string) (
	*proto.EnableVolumeReplicationResponse,
	error) {
	return rc.EnableVolumeReplicationMock(replicationSource, replicationID, secretName, secretNamespace, parameters)
}

// DisableVolumeReplication calls DisableVolumeReplicationMock mock function.
func (rc *ReplicationClient) DisableVolumeReplication(
	replicationSource *csiReplication.ReplicationSource,
	replicationID string,
	secretName, secretNamespace string,
	parameters map[string]string) (
	*proto.DisableVolumeReplicationResponse,
	error) {
	return rc.DisableVolumeReplicationMock(replicationSource, replicationID, secretName, secretNamespace, parameters)
}

// PromoteVolume calls PromoteVolumeMock mock function.
func (rc *ReplicationClient) PromoteVolume(
	replicationSource *csiReplication.ReplicationSource,
	replicationID string,
	force bool,
	secretName, secretNamespace string,
	parameters map[string]string) (
	*proto.PromoteVolumeResponse,
	error) {
	return rc.PromoteVolumeMock(replicationSource, replicationID, force, secretName, secretNamespace, parameters)
}

// DemoteVolume calls DemoteVolumeMock mock function.
func (rc *ReplicationClient) DemoteVolume(
	replicationSource *csiReplication.ReplicationSource,
	replicationID string,
	secretName, secretNamespace string,
	parameters map[string]string) (
	*proto.DemoteVolumeResponse,
	error) {
	return rc.DemoteVolumeMock(replicationSource, replicationID, secretName, secretNamespace, parameters)
}

// ResyncVolume calls ResyncVolumeMock function.
func (rc *ReplicationClient) ResyncVolume(
	replicationSource *csiReplication.ReplicationSource,
	replicationID string,
	secretName, secretNamespace string,
	parameters map[string]string) (
	*proto.ResyncVolumeResponse,
	error) {
	return rc.ResyncVolumeMock(replicationSource, replicationID, secretName, secretNamespace, parameters)
}

// GetVolumeReplicationInfo calls GetVolumeReplicationInfoMock function.
func (rc *ReplicationClient) GetVolumeReplicationInfo(
	replicationSource *csiReplication.ReplicationSource,
	replicationID string,
	secretName, secretNamespace string) (
	*proto.GetVolumeReplicationInfoResponse,
	error) {
	return rc.GetVolumeReplicationInfoMock(replicationSource, replicationID, secretName, secretNamespace)
}
