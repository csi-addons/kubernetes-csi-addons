/*
Copyright 2021 The Kubernetes-CSI-Addons Authors.

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

package service

import (
	"context"
	"path/filepath"

	kube "github.com/csi-addons/kubernetes-csi-addons/internal/kubernetes"
	"github.com/csi-addons/kubernetes-csi-addons/internal/proto"
	csiReclaimSpace "github.com/csi-addons/spec/lib/go/reclaimspace"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-csi/csi-lib-utils/accessmodes"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// ReclaimSpaceServer struct of sidecar with supported methods of proto
// reclaim space server spec and also containing reclaimspace node and
// controller client to csi driver.
type ReclaimSpaceServer struct {
	proto.UnimplementedReclaimSpaceServer
	controllerClient csiReclaimSpace.ReclaimSpaceControllerClient
	nodeClient       csiReclaimSpace.ReclaimSpaceNodeClient
	kubeClient       *kubernetes.Clientset
	stagingPath      string
}

// NewReclaimSpaceServer creates a new ReclaimSpaceServer which handles the proto.ReclaimSpace
// Service requests.
func NewReclaimSpaceServer(c *grpc.ClientConn, kc *kubernetes.Clientset, sp string) *ReclaimSpaceServer {
	return &ReclaimSpaceServer{
		controllerClient: csiReclaimSpace.NewReclaimSpaceControllerClient(c),
		nodeClient:       csiReclaimSpace.NewReclaimSpaceNodeClient(c),
		kubeClient:       kc,
		stagingPath:      sp,
	}
}

// RegisterService registers service with the server.
func (rs *ReclaimSpaceServer) RegisterService(server grpc.ServiceRegistrar) {
	proto.RegisterReclaimSpaceServer(server, rs)
}

// ControllerReclaimSpace fetches required information from kubernetes cluster and calls
// CSI-Addons ControllerReclaimSpace service.
func (rs *ReclaimSpaceServer) ControllerReclaimSpace(
	ctx context.Context,
	req *proto.ReclaimSpaceRequest) (*proto.ReclaimSpaceResponse, error) {

	pvName := req.GetPvName()
	klog.Info(pvName)

	pv, err := rs.kubeClient.CoreV1().PersistentVolumes().Get(ctx, pvName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Failed to get pv: %v", err)
		return nil, status.Errorf(codes.InvalidArgument, "failed to get pv %q", pvName)
	}

	if pv.Spec.CSI == nil {
		return nil, status.Errorf(codes.InvalidArgument, "pv %q is not a CSI volume", pvName)
	}

	volID := pv.Spec.CSI.VolumeHandle
	volAttributes := pv.Spec.CSI.VolumeAttributes

	csiReq := &csiReclaimSpace.ControllerReclaimSpaceRequest{
		VolumeId:   volID,
		Parameters: volAttributes,
	}

	// FIXME: use ControllerPublishSecret instead, but it is not set in the PV
	if pv.Spec.CSI.NodeStageSecretRef != nil {
		// Get the secrets from the k8s cluster
		csiReq.Secrets, err = kube.GetSecret(ctx, rs.kubeClient, pv.Spec.CSI.NodeStageSecretRef.Name, pv.Spec.CSI.NodeStageSecretRef.Namespace)
		if err != nil {
			klog.Errorf("Failed to get secret: %v", err)
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	}

	csiRes, err := rs.controllerClient.ControllerReclaimSpace(ctx, csiReq)
	if err != nil {
		return nil, err
	}
	if csiRes == nil {
		return nil, status.Error(codes.InvalidArgument, "nil value returned as the response of ControllerReclaimSpace")
	}
	res := &proto.ReclaimSpaceResponse{}
	if csiRes.PreUsage != nil {
		res.PreUsage = &proto.StorageConsumption{UsageBytes: csiRes.PreUsage.UsageBytes}
	}
	if csiRes.PostUsage != nil {
		res.PostUsage = &proto.StorageConsumption{UsageBytes: csiRes.PostUsage.UsageBytes}
	}

	return res, nil
}

// NodeReclaimSpace fetches required information from kubernetes cluster and calls
// CSI-Addons NodeReclaimSpace service.
func (rs *ReclaimSpaceServer) NodeReclaimSpace(
	ctx context.Context,
	req *proto.ReclaimSpaceRequest) (*proto.ReclaimSpaceResponse, error) {

	pvName := req.GetPvName()
	klog.Info(pvName)

	pv, err := rs.kubeClient.CoreV1().PersistentVolumes().Get(ctx, pvName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Failed to get pv: %v", err)
		return nil, status.Errorf(codes.InvalidArgument, "failed to get pv %q", pvName)
	}

	if pv.Spec.CSI == nil {
		return nil, status.Errorf(codes.InvalidArgument, "pv %q is not a CSI volume", pvName)
	}
	volID := pv.Spec.CSI.VolumeHandle
	csiMode, err := accessmodes.ToCSIAccessMode(pv.Spec.AccessModes, true)
	if err != nil {
		klog.Errorf("Failed to map access mode: %v", err)
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	stPath := filepath.Join(rs.stagingPath, "pv", pvName, "globalmount")
	accessType := csi.VolumeCapability_Mount{
		Mount: &csi.VolumeCapability_MountVolume{},
	}

	csiReq := &csiReclaimSpace.NodeReclaimSpaceRequest{
		VolumeId:          volID,
		VolumePath:        "",
		StagingTargetPath: stPath,
		VolumeCapability: &csi.VolumeCapability{
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csiMode,
			},
			AccessType: &accessType,
		},
	}

	if *pv.Spec.VolumeMode == corev1.PersistentVolumeBlock {
		csiReq.StagingTargetPath = filepath.Join(rs.stagingPath, "volumeDevices", "staging", pvName)
		csiReq.VolumeCapability.AccessType = &csi.VolumeCapability_Block{
			Block: &csi.VolumeCapability_BlockVolume{},
		}
	}

	if pv.Spec.CSI.NodeStageSecretRef != nil {
		// Get the secrets from the k8s cluster
		csiReq.Secrets, err = kube.GetSecret(ctx, rs.kubeClient, pv.Spec.CSI.NodeStageSecretRef.Name, pv.Spec.CSI.NodeStageSecretRef.Namespace)
		if err != nil {
			klog.Errorf("Failed to get secret: %v", err)
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	}
	csiRes, err := rs.nodeClient.NodeReclaimSpace(ctx, csiReq)
	if err != nil {
		return nil, err
	}
	if csiRes == nil {
		return nil, status.Error(codes.InvalidArgument, "nil value returned as the response of NodeReclaimSpace")
	}
	res := &proto.ReclaimSpaceResponse{}
	if csiRes.PreUsage != nil {
		res.PreUsage = &proto.StorageConsumption{UsageBytes: csiRes.PreUsage.UsageBytes}
	}
	if csiRes.PostUsage != nil {
		res.PostUsage = &proto.StorageConsumption{UsageBytes: csiRes.PostUsage.UsageBytes}
	}

	return res, nil
}
