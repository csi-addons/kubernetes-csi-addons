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

package utils

import (
	"fmt"
	"time"

	grpcClient "github.com/csi-addons/kubernetes-csi-addons/internal/client"
	conn "github.com/csi-addons/kubernetes-csi-addons/internal/connection"
	"github.com/csi-addons/spec/lib/go/identity"
)

func GetVolumeGroupClient(driverName string, connPoll *conn.ConnectionPool,
	timeout time.Duration) (grpcClient.VolumeGroup, error) {
	conns := connPoll.GetByNodeID(driverName, "")

	// Iterate through the connections and find the one that matches the driver name
	// provided in the VolumeReplication spec; so that corresponding
	// operations can be performed.
	for _, v := range conns {
		for _, cap := range v.Capabilities {
			// validate if VOLUME_REPLICATION capability is supported by the driver.
			if cap.GetVolumeGroup() == nil {
				continue
			}

			// validate of VOLUME_REPLICATION capability is enabled by the storage driver.
			if cap.GetVolumeGroup().GetType() == identity.Capability_VolumeGroup_VOLUME_GROUP {
				return grpcClient.NewVolumeGroupClient(v.Client, timeout), nil
			}
		}
	}

	return nil, fmt.Errorf("no connections for driver: %s", driverName)

}
