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
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/csi-addons/kubernetes-csi-addons/sidecar/internal/volume-condition/node"
	"github.com/csi-addons/kubernetes-csi-addons/sidecar/internal/volume-condition/platform"
	"github.com/csi-addons/kubernetes-csi-addons/sidecar/internal/volume-condition/volume"
)

type VolumeConditionReporter interface {
	Run(ctx context.Context, interval time.Duration) error
	Stop()
}

type volumeConditionReporter struct {
	client *kubernetes.Clientset

	recorders []ConditionRecorder

	driver    volume.Driver
	localNode node.Node
	// conditionCache tracks the condition by volume-handle
	conditionCache map[string]volume.VolumeCondition
	shouldStop     bool
}

func NewVolumeConditionReporter(
	ctx context.Context,
	client *kubernetes.Clientset,
	hostname, drivername string,
) (VolumeConditionReporter, error) {
	drv, err := volume.FindDriver(ctx, drivername)
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

	// TODO: use options for the recorders
	recorders := make([]ConditionRecorder, 0)
	lr, err := NewLogRecorder()
	_ = err
	recorders = append(recorders, lr)
	er, err := NewEventRecorder(client, hostname)
	_ = err
	recorders = append(recorders, er)

	return &volumeConditionReporter{
		client:         client,
		recorders:      recorders,
		driver:         drv,
		localNode:      n,
		conditionCache: make(map[string]volume.VolumeCondition, 0),
	}, nil
}

func (cvr *volumeConditionReporter) Run(ctx context.Context, interval time.Duration) error {
	running := time.Tick(interval)
	if running == nil {
		return fmt.Errorf("interval %v is invalid", interval)
	}

	for range running {
		volumes, err := cvr.localNode.ListCSIVolumes(ctx)
		if err != nil {
			return fmt.Errorf("failed to list volumes: %w", err)
		}

		for _, v := range volumes {
			if v.GetDriver() != cvr.driver.GetDrivername() {
				continue
			}

			vc, err := cvr.driver.GetVolumeCondition(v)
			if err != nil {
				klog.Errorf("failed to check if %q is healthy: %v", v.GetVolumeID(), err)
				continue
			}

			if !cvr.isUpdatedVolumeCondition(v.GetVolumeID(), vc) {
				// skip recording if there is no update
				continue
			}

			cvr.recordVolumeCondition(ctx, v.GetVolumeID(), vc)
		}

		cvr.pruneConditionCache(volumes)

		if cvr.shouldStop {
			break
		}
	}

	return nil
}

func (cvr *volumeConditionReporter) Stop() {
	cvr.shouldStop = true
}

func (cvr *volumeConditionReporter) isUpdatedVolumeCondition(volumeID string, vc volume.VolumeCondition) bool {
	lastCondition := cvr.conditionCache[volumeID]
	if lastCondition == nil || (lastCondition.IsHealthy() != vc.IsHealthy() || lastCondition.GetMessage() != vc.GetMessage()) {
		cvr.conditionCache[volumeID] = vc
		return true
	}

	return false
}

// pruneConditionCache removed inactive volume-handles from the cvr.conditionCache. This is done by creating a
// new cache, and have the old cvr.conditionCache get garbage collected.
func (cvr *volumeConditionReporter) pruneConditionCache(volumes []volume.CSIVolume) {
	newConditionCache := make(map[string]volume.VolumeCondition, len(volumes))
	for _, v := range volumes {
		volumeID := v.GetVolumeID()
		newConditionCache[volumeID] = cvr.conditionCache[volumeID]
	}

	cvr.conditionCache = newConditionCache
}

func (cvr *volumeConditionReporter) recordVolumeCondition(ctx context.Context, volumeID string, vc volume.VolumeCondition) {
	pvName, err := platform.GetPlatform().ResolvePersistentVolumeName(cvr.driver.GetDrivername(), volumeID)
	if err != nil {
		klog.Errorf("failed to resolve persistent volume name: %v", err)
		return
	}

	pv, err := cvr.client.CoreV1().PersistentVolumes().Get(ctx, pvName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("failed to get persistent volume %q: %v", pvName, err)
		return
	}

	for _, recorder := range cvr.recorders {
		err = recorder.Record(ctx, pv, vc)
		if err != nil {
			klog.Warningf(
				"%T failed to record volume condition for persistent volume %q: %v",
				recorder,
				pv.Name,
				err,
			)
		}
	}
}
