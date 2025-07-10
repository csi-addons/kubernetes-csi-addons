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
	"errors"
	"fmt"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/klog"

	"github.com/csi-addons/kubernetes-csi-addons/sidecar/internal/volume-condition/platform"
)

var ErrUnhealthyVolumeCondition = errors.New("UnhealthyVolumeCondition")

type Driver interface {
	GetDrivername() string
	SupportsVolumeCondition() bool
	IsHealthy(Volume) (bool, error)
}

type csiDriver struct {
	name string

	identityClient csi.IdentityClient
	nodeClient     csi.NodeClient

	supportVolumeCondition *bool
}

func FindDriver(name string) (Driver, error) {
	endpoint := platform.GetPlatform().GetCSISocket(name)
	conn, err := grpc.NewClient(
		endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to endpoint %s: %w", endpoint, err)
	}

	// verify that the requested drivername is indeed connected on the socket
	identityClient := csi.NewIdentityClient(conn)
	res, err := identityClient.GetPluginInfo(context.TODO(), &csi.GetPluginInfoRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to get info from CSI driver %q: %w", name, err)
	} else if res.GetName() != name {
		return nil, fmt.Errorf("CSI driver %q incorrectly identifies itself as %q", name, res.GetName())
	}

	return &csiDriver{
		name:           name,
		identityClient: identityClient,
		nodeClient:     csi.NewNodeClient(conn),
	}, nil
}

func (drv *csiDriver) GetDrivername() string {
	return drv.name
}

var (
	volumeConditionIsSupported    bool = true
	volumeConditionIsNotSupported bool = false
)

// TODO: verify that the driver provides NodeServiceCapability_RPC_VOLUME_CONDITION
func (drv *csiDriver) SupportsVolumeCondition() bool {
	if drv.supportVolumeCondition != nil {
		return *drv.supportVolumeCondition
	}

	res, err := drv.nodeClient.NodeGetCapabilities(context.TODO(), &csi.NodeGetCapabilitiesRequest{})
	if err != nil {
		klog.Errorf("failed to get capabilities of driver %q: %v", drv.name, err)
		return false
	}

	for _, capability := range res.GetCapabilities() {
		if capability.GetRpc().GetType() == csi.NodeServiceCapability_RPC_VOLUME_CONDITION {
			drv.supportVolumeCondition = &volumeConditionIsSupported
			return true
		}
	}

	klog.Infof("driver %q does not support VOLUME_CONDITION", drv.name)
	drv.supportVolumeCondition = &volumeConditionIsNotSupported
	return false
}

// may return (false, UnhealthyVolumeCondition)
func (drv *csiDriver) IsHealthy(v Volume) (bool, error) {
	volumePath := platform.GetPlatform().GetStagingPath(drv.name, v.GetVolumeID())
	if volumePath == "" {
		// sttaging path not found, use publish path
		volumePath = platform.GetPlatform().GetPublishPath(drv.name, v.GetVolumeID())
	}

	req := &csi.NodeGetVolumeStatsRequest{
		VolumeId:   v.GetVolumeID(),
		VolumePath: volumePath,
	}

	res, err := drv.nodeClient.NodeGetVolumeStats(context.TODO(), req)
	if err != nil {
		return false, fmt.Errorf("failed to call NodeGetVolumeStats: %w", err)
	}

	if res.GetVolumeCondition() == nil {
		return true, fmt.Errorf("VolumeCondition unknown")
	}

	healthy := !res.GetVolumeCondition().GetAbnormal()
	if !healthy {
		err = fmt.Errorf("%w: %s", ErrUnhealthyVolumeCondition, res.GetVolumeCondition().GetMessage())
	}

	return healthy, err
}
