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

	"github.com/container-storage-interface/spec/lib/go/csi"
	kube "github.com/csi-addons/kubernetes-csi-addons/internal/kubernetes"
	"github.com/csi-addons/kubernetes-csi-addons/internal/proto"
	csiEncKeyRotation "github.com/csi-addons/spec/lib/go/encryptionkeyrotation"

	"github.com/kubernetes-csi/csi-lib-utils/accessmodes"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type EncryptionKeyRotationServer struct {
	proto.UnimplementedEncryptionKeyRotationServer
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
	proto.RegisterEncryptionKeyRotationServer(server, ekrs)
}

func (ekrs *EncryptionKeyRotationServer) EncryptionKeyRotate(
	ctx context.Context, req *proto.EncryptionKeyRotateRequest,
) (*proto.EncryptionKeyRotateResponse, error) {
	logger := log.FromContext(ctx)
	pvName := req.GetPvName()
	if pvName == "" {
		return nil, status.Errorf(codes.InvalidArgument, "pv name is missing from the request")
	}

	pv, err := ekrs.kubeClient.CoreV1().PersistentVolumes().Get(ctx, pvName, metav1.GetOptions{})
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to find a pv with name: %s. error: %v", pvName, err)
	}

	if pv.Spec.CSI == nil {
		return nil, status.Errorf(codes.InvalidArgument, "%s is not a CSI volume", pvName)
	}

	csiMode, err := accessmodes.ToCSIAccessMode(pv.Spec.AccessModes, true)
	if err != nil {
		logger.Error(err, "Failed to map access mode")
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	volID := pv.Spec.CSI.VolumeHandle
	volAttrs := pv.Spec.CSI.VolumeAttributes

	ekrRequest := &csiEncKeyRotation.EncryptionKeyRotateRequest{
		VolumeId:   volID,
		Parameters: volAttrs,
		VolumeCapability: &csi.VolumeCapability{
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csiMode,
			},
		},
	}

	if pv.Spec.CSI.NodeStageSecretRef != nil {
		ekrRequest.Secrets, err = kube.GetSecret(ctx, ekrs.kubeClient, pv.Spec.CSI.NodeStageSecretRef.Name, pv.Spec.CSI.NodeStageSecretRef.Namespace)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	}

	_, err = ekrs.ekrClient.EncryptionKeyRotate(ctx, ekrRequest)
	if err != nil {
		return nil, err
	}

	logger.Info("Successfully rotated the key for pv", "pvName", pvName)

	return &proto.EncryptionKeyRotateResponse{}, nil
}
