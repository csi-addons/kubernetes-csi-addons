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

package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/csi-addons/kubernetes-csi-addons/internal/proto"
	"github.com/csi-addons/kubernetes-csi-addons/internal/sidecar/service"
)

type EncryptionKeyRotation struct {
	grpcClient

	persistentVolume string
}

var _ = registerOperation("EncryptionKeyRotation", &EncryptionKeyRotation{})

func (ekr *EncryptionKeyRotation) Init(c *command) error {
	ekr.persistentVolume = c.persistentVolume

	if ekr.persistentVolume == "" {
		return errors.New("PersistentVolume name is not set")
	}

	return nil
}

func (ekr *EncryptionKeyRotation) Execute() error {
	kc := getKubernetesClient()
	ekrs := service.NewEncryptionKeyRotationServer(ekr.Client, kc)

	ekrReq := &proto.EncryptionKeyRotateRequest{
		PvName: ekr.persistentVolume,
	}

	ekrRes, err := ekrs.EncryptionKeyRotate(context.TODO(), ekrReq)
	if err != nil {
		return err
	}

	if ekrRes == nil {
		return fmt.Errorf("failed to rotate encryption key for pv: %s", ekr.persistentVolume)
	}

	fmt.Printf("Encryption key rotation successful for pv: %s\n", ekr.persistentVolume)

	return nil
}
