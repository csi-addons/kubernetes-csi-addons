/*
Copyright 2025 The Kubernetes-CSI-Addons Authors.

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

package condition

import (
	"context"
	"errors"
	"fmt"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/csi-addons/kubernetes-csi-addons/sidecar/internal/volume-condition/node"
	"github.com/csi-addons/kubernetes-csi-addons/sidecar/internal/volume-condition/volume"
)

type VolumeConditionReporter interface {
	Run(ctx context.Context, interval time.Duration) error
	Stop()
}

type volumeConditionReporter struct {
	client *kubernetes.Clientset

	driver     volume.Driver
	localNode  node.Node
	shouldStop bool
}

func NewVolumeConditionReporter(
	ctx context.Context,
	client *kubernetes.Clientset,
	hostname, drivername string,
) (VolumeConditionReporter, error) {
	drv, err := volume.FindDriver(drivername)
	if err != nil {
		return nil, fmt.Errorf("failed to find driver %q: %w", drivername, err)
	}

	if !drv.SupportsVolumeCondition() {
		return nil, fmt.Errorf("driver %q does not support volume-condition", drivername)
	}

	n, err := node.NewNode(ctx, client, hostname)
	if err != nil {
		return nil, fmt.Errorf("failed to get node %q: %w", hostname, err)
	}

	return &volumeConditionReporter{
		client:    client,
		driver:    drv,
		localNode: n,
	}, nil
}

func (cvr *volumeConditionReporter) Run(ctx context.Context, interval time.Duration) error {
	running := time.Tick(interval)
	if running == nil {
		return fmt.Errorf("interval %v is invalid", interval)
	}

	for range running {
		volumes, err := cvr.localNode.ListVolumes(ctx)
		if err != nil {
			return fmt.Errorf("failed to list volumes: %w", err)
		}

		for _, v := range volumes {
			if v.GetDriver() != cvr.driver.GetDrivername() {
				continue
			}

			healthy, err := cvr.driver.IsHealthy(v)
			if err != nil && !errors.Is(err, volume.ErrUnhealthyVolumeCondition) {
				klog.Errorf("failed to check if %q is healthy: %v", v.GetVolumeID(), err)
				continue
			}

			// TODO: report the health (not only log it)
			if !healthy {
				klog.Warningf("volume-handle %q is not healthy: %v\n", v.GetVolumeID(), err)
			} else { // healthy
				klog.Infof("volume-handle %q is healthy: %v\n", v.GetVolumeID(), healthy)
			}
		}

		if cvr.shouldStop {
			break
		}
	}

	return nil
}

func (cvr *volumeConditionReporter) Stop() {
	cvr.shouldStop = true
}
