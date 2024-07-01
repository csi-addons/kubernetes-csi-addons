/*
Copyright 2024 The Kubernetes-CSI-Addons Authors.

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

	csiEncKeyRotation "github.com/csi-addons/spec/lib/go/encryptionkeyrotation"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/csi-addons/kubernetes-csi-addons/internal/proto"

	kube "github.com/csi-addons/kubernetes-csi-addons/internal/kubernetes"
)

type EncryptionKeyRotationServer struct {
	proto.UnimplementedEncKeyRotationServer
	ekrClient  csiEncKeyRotation.EncryptionKeyRotationControllerClient
	kubeClient *kubernetes.Clientset
}

func NewEncryptionKeyRotationServer(
	c *grpc.ClientConn, kc *kubernetes.Clientset,
) *EncryptionKeyRotationServer {
	return &EncryptionKeyRotationServer{
		ekrClient:  csiEncKeyRotation.NewEncryptionKeyRotationControllerClient(c),
		kubeClient: kc,
	}
}

func (ekrs *EncryptionKeyRotationServer) RegisterService(server grpc.ServiceRegistrar) {
	proto.RegisterEncKeyRotationServer(server, ekrs)
}

func (ekrs *EncryptionKeyRotationServer) KeyRotate(
	ctx context.Context, req *proto.EncKeyRotateRequest,
) (*proto.EncKeyRotateResponse, error) {
	pvName := req.GetPvName()
	if pvName == "" {
		return nil, status.Errorf(codes.InvalidArgument, "pv name is missing from the request")
	}

	pv, err := ekrs.kubeClient.CoreV1().PersistentVolumes().Get(ctx, pvName, metav1.GetOptions{})
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to find a pv with name: %s", pvName)
	}

	if pv.Spec.CSI == nil {
		return nil, status.Errorf(codes.InvalidArgument, "%s is not a CSI volume", pvName)
	}

	volID := pv.Spec.CSI.VolumeHandle
	volAttrs := pv.Spec.CSI.VolumeAttributes

	ekrRequest := &csiEncKeyRotation.EncryptionKeyRotateRequest{
		VolumeId:   volID,
		Parameters: volAttrs,
	}

	if pv.Spec.CSI.NodeStageSecretRef != nil {
		ekrRequest.Secrets, err = kube.GetSecret(ctx, ekrs.kubeClient, pv.Spec.CSI.NodePublishSecretRef.Name, pv.Spec.CSI.NodePublishSecretRef.Namespace)
		if err != nil {
			klog.Errorf("failed to get secret: %v", err)
			return nil, status.Errorf(codes.InvalidArgument, err.Error())
		}
	}

	ekrResult, err := ekrs.ekrClient.EncryptionKeyRotate(ctx, ekrRequest)
	if err != nil {
		return nil, err
	}

	if ekrResult == nil {
		return nil, status.Error(codes.InvalidArgument, "received nil as response of csi enckeyrotate request")
	}

	return &proto.EncKeyRotateResponse{}, nil
}
