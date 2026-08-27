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
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/csi-addons/kubernetes-csi-addons/sidecar/internal/volume-condition/platform"
	"github.com/csi-addons/kubernetes-csi-addons/sidecar/internal/volume-condition/volume/legacycsi"
)

var driverLogger = ctrl.Log.WithName("volume-condition")

// legacyCapabilityVolumeCondition is the CSI spec v1.12.0 capability value
// for VOLUME_CONDITION (= 4). It was removed in v1.13.0 when NodeGetVolumeHealth
// was introduced as its replacement.
const legacyCapabilityVolumeCondition = csi.NodeServiceCapability_RPC_Type(4)

// Driver provides the API for communicating with a CSI-driver.
type Driver interface {
	// GetDrivername returns the name of the CSI-driver.
	GetDrivername() string
	// SupportsVolumeCondition can be used to check if the CSI-driver
	// supports reporting the VolumeCondition (if the node has the
	// GET_VOLUME_HEALTH or legacy VOLUME_CONDITION capability).
	SupportsVolumeCondition() bool
	// GetVolumeCondition requests the VolumeCondition from the
	// CSI-driver.
	GetVolumeCondition(CSIVolume) (VolumeCondition, error)
}

type csiDriver struct {
	name string

	conn       grpc.ClientConnInterface
	nodeClient csi.NodeClient

	// supportsNodeGetVolumeHealth is set when the driver reports the
	// GET_VOLUME_HEALTH capability (CSI spec v1.13.0+).
	supportsNodeGetVolumeHealth bool
	// supportsLegacyVolumeCondition is set when the driver reports the old
	// VOLUME_CONDITION capability (CSI spec v1.12.0 and earlier).
	supportsLegacyVolumeCondition bool
	supportsNodeStageVolume       bool
}

// FindDriver tries to connect to the CSI-driver with the given name. If
// a connection is made, it verifies the identity of the driver and its
// capabilities.
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
		conn:       conn,
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
	return drv.supportsNodeGetVolumeHealth || drv.supportsLegacyVolumeCondition
}

func (drv *csiDriver) GetVolumeCondition(v CSIVolume) (VolumeCondition, error) {
	if drv.supportsNodeGetVolumeHealth {
		return drv.getVolumeConditionViaVolumeHealth(v)
	}
	return drv.getVolumeConditionViaVolumeStats(v)
}

// getVolumeConditionViaVolumeHealth calls NodeGetVolumeHealth (CSI spec v1.13.0+).
func (drv *csiDriver) getVolumeConditionViaVolumeHealth(v CSIVolume) (VolumeCondition, error) {
	req := &csi.NodeGetVolumeHealthRequest{
		VolumeId: v.GetVolumeID(),
	}

	if drv.supportsNodeStageVolume {
		if stagingPath, err := platform.GetPlatform().GetStagingPath(drv.name, v.GetVolumeID()); err == nil {
			req.StagingTargetPath = stagingPath
		}
	}
	if publishPath, err := platform.GetPlatform().GetPublishPath(drv.name, v.GetVolumeID()); err == nil {
		req.VolumePublishPath = publishPath
	}

	res, err := drv.nodeClient.NodeGetVolumeHealth(context.TODO(), req)
	if err != nil {
		return nil, fmt.Errorf("failed to call NodeGetVolumeHealth: %w", err)
	}

	vh := res.GetVolumeHealth()
	if vh == nil {
		return nil, fmt.Errorf("VolumeHealth missing from NodeGetVolumeHealth response")
	}

	healthy := true

	var messages []string
	for _, s := range vh.GetHealthStatuses() {
		switch s.GetStatus() {
		case csi.VolumeHealthErrorType_DEGRADED,
			csi.VolumeHealthErrorType_INACCESSIBLE,
			csi.VolumeHealthErrorType_DATA_LOSS:
			healthy = false
		default:
			continue
		}

		msg := s.GetMessage()
		if msg == "" {
			msg = s.GetReason()
		}
		if msg == "" {
			driverLogger.Info("volume health status has no message or reason", "status", s.GetStatus())
		} else {
			messages = append(messages, msg)
		}
	}

	return &volumeCondition{
		healthy: healthy,
		message: strings.Join(messages, "; "),
	}, nil
}

// getVolumeConditionViaVolumeStats calls NodeGetVolumeStats and parses the
// VolumeCondition from the response using the CSI spec v1.12.0 wire format
// (fallback for drivers that do not support NodeGetVolumeHealth yet).
func (drv *csiDriver) getVolumeConditionViaVolumeStats(v CSIVolume) (VolumeCondition, error) {
	var (
		err        error
		volumePath string
	)

	if drv.supportsNodeStageVolume {
		volumePath, err = platform.GetPlatform().GetStagingPath(drv.name, v.GetVolumeID())
		if err != nil {
			return nil, fmt.Errorf("failed to get staging path: %w", err)
		}
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

	vc, err := legacycsi.GetVolumeCondition(context.TODO(), drv.conn, req)
	if err != nil {
		return nil, err
	}
	if vc == nil {
		return nil, fmt.Errorf("VolumeCondition not found in NodeGetVolumeStats response")
	}

	return &volumeCondition{
		healthy: !vc.Abnormal,
		message: vc.Message,
	}, nil
}

// detectCapabilities calls the NodeGetCapabilities gRPC procedure to detect
// the capabilities of the CSI-driver.
func (drv *csiDriver) detectCapabilities(ctx context.Context) error {
	res, err := drv.nodeClient.NodeGetCapabilities(ctx, &csi.NodeGetCapabilitiesRequest{})
	if err != nil {
		return fmt.Errorf("failed to get capabilities of driver %q: %v", drv.name, err)
	}

	for _, capability := range res.GetCapabilities() {
		switch capability.GetRpc().GetType() {
		case csi.NodeServiceCapability_RPC_GET_VOLUME_HEALTH:
			drv.supportsNodeGetVolumeHealth = true
		case legacyCapabilityVolumeCondition:
			drv.supportsLegacyVolumeCondition = true
		case csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME:
			drv.supportsNodeStageVolume = true
		}
	}

	return nil
}
