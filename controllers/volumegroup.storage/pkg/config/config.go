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

package config

import (
	conn "github.com/csi-addons/kubernetes-csi-addons/internal/connection"
	"github.com/csi-addons/spec/lib/go/identity"
)

type DriverConfig struct {
	DriverEndpoint       string
	DriverName           string
	MultipleVGsToPVC     bool
	DeletePVCsOnVGDelete bool
	ModifyVolumeGroup    bool
}

func GetDriverConfig(driverName string, connPoll *conn.ConnectionPool) (*DriverConfig, error) {
	conns := connPoll.GetByNodeID(driverName, "")
	multipleVGsToPVC := true
	deletePVCsOnVGDelete := true
	modifyVolumeGroup := false

	// Iterate through the connections and find the one that matches the driver name
	// provided in the VolumeReplication spec; so that corresponding
	// operations can be performed.
	for _, v := range conns {
		for _, cap := range v.Capabilities {
			// validate if VOLUME_REPLICATION capability is supported by the driver.
			if cap.GetVolumeGroup() == nil {
				continue
			}

			switch capType := cap.GetVolumeGroup().GetType(); capType {
			case identity.Capability_VolumeGroup_LIMIT_VOLUME_TO_ONE_VOLUME_GROUP:
				multipleVGsToPVC = false
			case identity.Capability_VolumeGroup_DO_NOT_ALLOW_VG_TO_DELETE_VOLUMES:
				deletePVCsOnVGDelete = false
			case identity.Capability_VolumeGroup_MODIFY_VOLUME_GROUP:
				modifyVolumeGroup = true
			}
		}
	}
	return &DriverConfig{
		DriverName:           driverName,
		MultipleVGsToPVC:     multipleVGsToPVC,
		DeletePVCsOnVGDelete: deletePVCsOnVGDelete,
		ModifyVolumeGroup:    modifyVolumeGroup,
	}, nil
}
