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
	kube "github.com/csi-addons/kubernetes-csi-addons/sidecar/internal/kubernetes"

	fence "github.com/csi-addons/spec/lib/go/fence"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// NetworkFenceServer struct of sidecar with supported methods of proto
// networkFence server spec and controller client to csi driver.
type NetworkFenceServer struct {
	proto.UnimplementedNetworkFenceServer
	controllerClient fence.FenceControllerClient
	kubeClient       *kubernetes.Clientset
}

// NewNetworkFenceServer creates a new NetworkFenceServer which handles the proto.NetworkFence
// Service requests.
func NewNetworkFenceServer(c *grpc.ClientConn, kc *kubernetes.Clientset) *NetworkFenceServer {
	return &NetworkFenceServer{
		controllerClient: fence.NewFenceControllerClient(c),
		kubeClient:       kc,
	}
}

// RegisterService registers service with the server.
func (ns *NetworkFenceServer) RegisterService(server grpc.ServiceRegistrar) {
	proto.RegisterNetworkFenceServer(server, ns)
}

// FenceClusterNetwork fetches required information from kubernetes cluster and calls
// CSI-Addons FenceClusterNetwork service.
func (ns *NetworkFenceServer) FenceClusterNetwork(
	ctx context.Context,
	req *proto.NetworkFenceRequest) (*proto.NetworkFenceResponse, error) {
	// Get the secrets from the k8s cluster
	data, err := kube.GetSecret(ctx, ns.kubeClient, req.GetSecretName(), req.GetSecretNamespace())
	if err != nil {
		klog.Errorf("Failed to get secret %s in namespace %s: %v", req.GetSecretName(), req.GetSecretNamespace(), err)
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	cidr := req.GetCidrs()
	fenceRequest := fence.FenceClusterNetworkRequest{
		Parameters: req.GetParameters(),
		Cidrs:      getCIDRS(cidr),
		Secrets:    data,
	}

	_, err = ns.controllerClient.FenceClusterNetwork(ctx, &fenceRequest)
	if err != nil {
		klog.Errorf("Failed to fence cluster network: %v", err)
		return nil, err
	}

	return &proto.NetworkFenceResponse{}, nil
}

// UnFenceClusterNetwork fetches required information from kubernetes cluster and calls
// CSI-Addons UnFenceClusterNetwork service.
func (ns *NetworkFenceServer) UnFenceClusterNetwork(
	ctx context.Context,
	req *proto.NetworkFenceRequest) (*proto.NetworkFenceResponse, error) {
	// Get the secrets from the k8s cluster
	data, err := kube.GetSecret(ctx, ns.kubeClient, req.GetSecretName(), req.GetSecretNamespace())
	if err != nil {
		klog.Errorf("Failed to get secret %s in namespace %s: %v", req.GetSecretName(), req.GetSecretNamespace(), err)
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	cidr := req.GetCidrs()
	fenceRequest := fence.UnfenceClusterNetworkRequest{
		Parameters: req.GetParameters(),
		Cidrs:      getCIDRS(cidr),
		Secrets:    data,
	}

	_, err = ns.controllerClient.UnfenceClusterNetwork(ctx, &fenceRequest)
	if err != nil {
		klog.Errorf("Failed to unfence cluster network: %v", err)
		return nil, err
	}

	return &proto.NetworkFenceResponse{}, nil
}

// getCIDRS converts the cidr string to a slice of cidrs.
func getCIDRS(cidr []string) []*fence.CIDR {
	cidrs := []*fence.CIDR{}
	for _, c := range cidr {
		cidrs = append(cidrs, &fence.CIDR{Cidr: c})
	}
	return cidrs
}
