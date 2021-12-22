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

	"github.com/csi-addons/kubernetes-csi-addons/internal/proto"
	csiReclaimSpace "github.com/csi-addons/spec/lib/go/reclaimspace"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc"
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
}

// NewReclaimSpaceServer creates a new ReclaimSpaceServer which handles the proto.ReclaimSpace
// Service requests.
func NewReclaimSpaceServer(c *grpc.ClientConn, kc *kubernetes.Clientset) *ReclaimSpaceServer {
	return &ReclaimSpaceServer{
		controllerClient: csiReclaimSpace.NewReclaimSpaceControllerClient(c),
		nodeClient:       csiReclaimSpace.NewReclaimSpaceNodeClient(c),
		kubeClient:       kc,
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

	// TODO: get following params using kubeClient and pvName

	csiReq := &csiReclaimSpace.ControllerReclaimSpaceRequest{
		VolumeId:   "",
		Parameters: map[string]string{},
		Secrets:    map[string]string{},
	}
	res, err := rs.controllerClient.ControllerReclaimSpace(ctx, csiReq)
	if err != nil {
		return nil, err
	}

	return &proto.ReclaimSpaceResponse{
		PreUsage:  &proto.StorageConsumption{UsageBytes: res.PreUsage.UsageBytes},
		PostUsage: &proto.StorageConsumption{UsageBytes: res.PostUsage.UsageBytes},
	}, nil
}

// NodeReclaimSpace fetches required information from kubernetes cluster and calls
// CSI-Addons NodeReclaimSpace service.
func (rs *ReclaimSpaceServer) NodeReclaimSpace(
	ctx context.Context,
	req *proto.ReclaimSpaceRequest) (*proto.ReclaimSpaceResponse, error) {

	pvName := req.GetPvName()
	klog.Info(pvName)

	// TODO: get following params using kubeClient and pvName

	csiReq := &csiReclaimSpace.NodeReclaimSpaceRequest{
		VolumeId:          "",
		VolumePath:        "",
		StagingTargetPath: "",
		VolumeCapability:  &csi.VolumeCapability{},
		Secrets:           map[string]string{},
	}
	res, err := rs.nodeClient.NodeReclaimSpace(ctx, csiReq)
	if err != nil {
		return nil, err
	}

	return &proto.ReclaimSpaceResponse{
		PreUsage:  &proto.StorageConsumption{UsageBytes: res.PreUsage.UsageBytes},
		PostUsage: &proto.StorageConsumption{UsageBytes: res.PostUsage.UsageBytes},
	}, nil
}
