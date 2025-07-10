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

package volume

import (
	"context"
	"fmt"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/csi-addons/kubernetes-csi-addons/sidecar/internal/volume-condition/platform"
)

type Driver interface {
	GetDrivername() string
	SupportsVolumeCondition() bool
	GetVolumeCondition(CSIVolume) (VolumeCondition, error)
}

type csiDriver struct {
	name string

	nodeClient csi.NodeClient

	supportVolumeCondition  bool
	supportsNodeStageVolume bool
}

func FindDriver(ctx context.Context, name string) (Driver, error) {
	endpoint := platform.GetPlatform().GetCSISocket(name)
	conn, err := grpc.NewClient(
		endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to endpoint %s: %w", endpoint, err)
	}

	// verify that the requested drivername is indeed connected on the socket
	identityClient := csi.NewIdentityClient(conn)
	res, err := identityClient.GetPluginInfo(ctx, &csi.GetPluginInfoRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to get info from CSI driver %q: %w", name, err)
	} else if res.GetName() != name {
		return nil, fmt.Errorf("CSI driver %q incorrectly identifies itself as %q", name, res.GetName())
	}

	drv := &csiDriver{
		name:       name,
		nodeClient: csi.NewNodeClient(conn),
	}

	err = drv.detectCapabilities(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to detect the capabilities of CSI driver %q: %w", name, err)
	}

	return drv, nil
}

func (drv *csiDriver) GetDrivername() string {
	return drv.name
}

func (drv *csiDriver) SupportsVolumeCondition() bool {
	return drv.supportVolumeCondition
}

func (drv *csiDriver) GetVolumeCondition(v CSIVolume) (VolumeCondition, error) {
	var (
		err        error
		volumePath = ""
	)

	if drv.supportsNodeStageVolume {
		volumePath = platform.GetPlatform().GetStagingPath(drv.name, v.GetVolumeID())
	} else {
		volumePath, err = platform.GetPlatform().GetPublishPath(drv.name, v.GetVolumeID())
		if err != nil {
			return nil, fmt.Errorf("failed to get publish path: %w", err)
		}
	}

	req := &csi.NodeGetVolumeStatsRequest{
		VolumeId:   v.GetVolumeID(),
		VolumePath: volumePath,
	}

	res, err := drv.nodeClient.NodeGetVolumeStats(context.TODO(), req)
	if err != nil {
		return nil, fmt.Errorf("failed to call NodeGetVolumeStats: %w", err)
	}

	if res.GetVolumeCondition() == nil {
		return nil, fmt.Errorf("VolumeCondition unknown")
	}

	vc := &volumeCondition{
		healthy: !res.GetVolumeCondition().GetAbnormal(),
		message: res.GetVolumeCondition().GetMessage(),
	}

	return vc, err
}

func (drv *csiDriver) detectCapabilities(ctx context.Context) error {
	res, err := drv.nodeClient.NodeGetCapabilities(context.TODO(), &csi.NodeGetCapabilitiesRequest{})
	if err != nil {
		return fmt.Errorf("failed to get capabilities of driver %q: %v", drv.name, err)
	}

	for _, capability := range res.GetCapabilities() {
		switch capability.GetRpc().GetType() {
		case csi.NodeServiceCapability_RPC_VOLUME_CONDITION:
			drv.supportVolumeCondition = true
		case csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME:
			drv.supportsNodeStageVolume = true
		}
	}

	return nil
}
