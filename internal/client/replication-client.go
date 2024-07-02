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
	"github.com/csi-addons/kubernetes-csi-addons/internal/proto"
)

// VolumeReplication holds the methods required for replication.
type VolumeReplication interface {
	// EnableVolumeReplication RPC call to enable the volume replication.
	EnableVolumeReplication(id, replicationID string, secretName, secretNamespace string, parameters map[string]string) (*proto.EnableVolumeReplicationResponse, error)
	// DisableVolumeReplication RPC call to disable the volume replication.
	DisableVolumeReplication(id, replicationID string, secretName, secretNamespace string, parameters map[string]string) (*proto.DisableVolumeReplicationResponse, error)
	// PromoteVolume RPC call to promote the volume.
	PromoteVolume(id, replicationID string, force bool, secretName, secretNamespace string, parameters map[string]string) (*proto.
		PromoteVolumeResponse, error)
	// DemoteVolume RPC call to demote the volume.
	DemoteVolume(id, replicationID string, secretName, secretNamespace string, parameters map[string]string) (*proto.
		DemoteVolumeResponse, error)
	// ResyncVolume RPC call to resync the volume.
	ResyncVolume(id, replicationID string, force bool, secretName, secretNamespace string, parameters map[string]string) (*proto.
		ResyncVolumeResponse, error)
	// GetVolumeReplicationInfo RPC call to get volume replication info.
	GetVolumeReplicationInfo(id, replicationID, secretName, secretNamespace string) (*proto.GetVolumeReplicationInfoResponse, error)
}
