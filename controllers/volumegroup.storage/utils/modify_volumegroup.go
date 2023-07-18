/*
Copyright 2023 The Kubernetes-CSI-Addons Authors.

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

package utils

import (
	"fmt"

	volumegroupv1 "github.com/csi-addons/kubernetes-csi-addons/apis/volumegroup.storage/v1"
	"github.com/csi-addons/kubernetes-csi-addons/controllers/volumegroup.storage/pkg/messages"
	"github.com/csi-addons/kubernetes-csi-addons/controllers/volumegroup.storage/volumegroup"
	grpcClient "github.com/csi-addons/kubernetes-csi-addons/internal/client"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ModifyVG(logger logr.Logger, client client.Client, vg *volumegroupv1.VolumeGroup,
	vgClient grpcClient.VolumeGroup) error {
	params, err := generateModifyVGParams(logger, client, vg, vgClient)
	if err != nil {
		return err
	}
	logger.Info(fmt.Sprintf(messages.ModifyVG, params.VolumeGroupID, params.VolumeIds))
	volumeGroupRequest := volumegroup.NewVolumeGroupRequest(params)
	modifyVGResponse := volumeGroupRequest.Modify()
	responseError := modifyVGResponse.Error
	if responseError != nil {
		logger.Error(responseError, fmt.Sprintf(messages.FailedToModifyVG, vg.Namespace, vg.Name))
		return responseError
	}
	logger.Info(fmt.Sprintf(messages.ModifiedVG, params.VolumeGroupID))
	return nil
}
func generateModifyVGParams(logger logr.Logger, client client.Client,
	vg *volumegroupv1.VolumeGroup, vgClient grpcClient.VolumeGroup) (volumegroup.CommonRequestParameters, error) {
	vgId, err := getVgId(logger, client, vg)
	if err != nil {
		return volumegroup.CommonRequestParameters{}, err
	}
	volumeIds, err := getPVCListVolumeIds(logger, client, vg.Status.PVCList)
	if err != nil {
		return volumegroup.CommonRequestParameters{}, err
	}
	secrets, err := getSecrets(logger, client, vg)
	if err != nil {
		return volumegroup.CommonRequestParameters{}, err
	}

	return volumegroup.CommonRequestParameters{
		Secrets:       secrets,
		VolumeGroup:   vgClient,
		VolumeGroupID: vgId,
		VolumeIds:     volumeIds,
	}, nil
}
func getSecrets(logger logr.Logger, client client.Client, vg *volumegroupv1.VolumeGroup) (map[string]string, error) {
	vgc, err := GetVGClass(client, logger, GetStringField(vg.Spec, "VolumeGroupClassName"))
	if err != nil {
		return nil, err
	}
	secrets, err := GetSecretDataFromClass(client, vgc, logger)
	if err != nil {
		if uErr := UpdateVGStatusError(client, vg, logger, err.Error()); uErr != nil {
			return nil, err
		}
		return nil, err
	}
	return secrets, nil
}
