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

	internalConn "github.com/csi-addons/kubernetes-csi-addons/internal/connection"
	"github.com/kubernetes-csi/csi-lib-utils/connection"
	"github.com/kubernetes-csi/csi-lib-utils/metrics"
)

func New(address, driver string) (*internalConn.Connection, error) {
	metricsManager := metrics.NewCSIMetricsManager(driver)
	conn, err := connection.Connect(address, metricsManager, connection.OnConnectionLoss(connection.ExitOnConnectionLoss()))
	if err != nil {
		return &internalConn.Connection{}, err
	}
	return &internalConn.Connection{
		Client:     conn,
		NodeID:     "",
		DriverName: driver,
		Timeout:    time.Minute,
	}, nil
}
