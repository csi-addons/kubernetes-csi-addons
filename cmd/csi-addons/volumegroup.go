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
	"strings"

	"github.com/csi-addons/kubernetes-csi-addons/internal/proto"
	"github.com/csi-addons/kubernetes-csi-addons/internal/sidecar/service"
)

// VolumeGroupBase struct for common fields
type VolumeGroupBase struct {
	volumeGroupID   string
	volumeGroupName string
	secretName      string
	secretNamespace string
	parameters      map[string]string
	volumeIDs       []string
}

// SetVolumeGroupID sets the volumeGroupID and checks if it's empty
func (vgb *VolumeGroupBase) SetVolumeGroupID(volumeGroupID string) error {
	if volumeGroupID == "" {
		return errors.New("volumeGroupID is not set")
	}
	vgb.volumeGroupID = volumeGroupID
	return nil
}

// SetVolumeGroupName sets the volumeGroupName and checks if it's empty
func (vgb *VolumeGroupBase) SetVolumeGroupName(volumeGroupName string) error {
	if volumeGroupName == "" {
		return errors.New("volumeGroupName is not set")
	}
	vgb.volumeGroupName = volumeGroupName
	return nil
}

// SetSecret sets the secret name and namespace
func (vgb *VolumeGroupBase) SetSecret(secret string) error {
	secretNamespace, secretName, err := parseSecret(secret)
	if err != nil {
		return fmt.Errorf("failed to parse secret: %w", err)
	}
	vgb.secretNamespace = secretNamespace
	vgb.secretName = secretName
	return nil
}

// SetParameters sets and parses the parameters
func (vgb *VolumeGroupBase) SetParameters(parameters string) error {
	params, err := parseParameters(parameters)
	if err != nil {
		return fmt.Errorf("failed to parse parameters: %w", err)
	}
	vgb.parameters = params
	return nil
}

// SetVolumeIDs sets and parses the volume IDs
func (vgb *VolumeGroupBase) SetVolumeIDs(volumeids string) {
	volumeIDs := strings.Split(volumeids, ",")
	vgb.volumeIDs = volumeIDs
}

// Parses the parameters to convert them from string format to map[string]string format
func parseParameters(paramString string) (map[string]string, error) {
	params := make(map[string]string)
	if paramString != "" {
		pairs := strings.Split(paramString, ",")
		for _, pair := range pairs {
			kv := strings.SplitN(pair, "=", 2)
			if len(kv) == 2 {
				key := kv[0]
				value := kv[1]
				params[key] = value
			} else {
				return nil, fmt.Errorf("invalid parameter : %s", pair)
			}
		}
	}
	return params, nil
}

// Splits the secrets passed as namespace/name
func parseSecret(secretString string) (string, string, error) {
	secrets := strings.Split(secretString, "/")
	if len(secrets) != 2 {
		return "", "", errors.New("secret should be specified in the format `namespace/name`")
	}
	namespace := secrets[0]
	if namespace == "" {
		return "", "", errors.New("secret namespace is not set")
	}
	name := secrets[1]
	if name == "" {
		return "", "", errors.New("secret name is not set")
	}
	return namespace, name, nil
}

// CreateVolumeGroup executes the CreateVolumeGroup operation.
type CreateVolumeGroup struct {
	grpcClient
	VolumeGroupBase
}

var _ = registerOperation("CreateVolumeGroup", &CreateVolumeGroup{})

func (cvg *CreateVolumeGroup) Init(c *command) error {

	if err := cvg.SetVolumeGroupName(c.volumeGroupName); err != nil {
		return err
	}
	if err := cvg.SetParameters(c.parameters); err != nil {
		return err
	}
	if err := cvg.SetSecret(c.secret); err != nil {
		return err
	}
	cvg.SetVolumeIDs(c.volumeIDs)

	return nil
}

func (cvg *CreateVolumeGroup) Execute() error {
	vgs := service.NewVolumeGroupServer(cvg.Client, getKubernetesClient())

	req := &proto.CreateVolumeGroupRequest{
		Name:            cvg.volumeGroupName,
		Parameters:      cvg.parameters,
		SecretName:      cvg.secretName,
		SecretNamespace: cvg.secretNamespace,
		VolumeIds:       cvg.volumeIDs,
	}
	res, err := vgs.CreateVolumeGroup(context.TODO(), req)
	if err != nil {
		return err
	}
	fmt.Printf("Volume Group created: %+v\n", res)
	return nil

}

// ModifyVolumeGroupMembership executes the ModifyVolumeGroupMembership operation.
type ModifyVolumeGroupMembership struct {
	grpcClient
	VolumeGroupBase
}

var _ = registerOperation("ModifyVolumeGroupMembership", &ModifyVolumeGroupMembership{})

func (mvgm *ModifyVolumeGroupMembership) Init(c *command) error {

	if err := mvgm.SetVolumeGroupID(c.volumeGroupID); err != nil {
		return err
	}
	mvgm.SetVolumeIDs(c.volumeIDs)
	if err := mvgm.SetSecret(c.secret); err != nil {
		return err
	}

	if err := mvgm.SetParameters(c.parameters); err != nil {
		return err
	}

	return nil
}

func (mvgm *ModifyVolumeGroupMembership) Execute() error {
	vgs := service.NewVolumeGroupServer(mvgm.Client, getKubernetesClient())

	req := &proto.ModifyVolumeGroupMembershipRequest{
		VolumeGroupId:   mvgm.volumeGroupID,
		VolumeIds:       mvgm.volumeIDs,
		SecretName:      mvgm.secretName,
		SecretNamespace: mvgm.secretNamespace,
		Parameters:      mvgm.parameters,
	}

	res, err := vgs.ModifyVolumeGroupMembership(context.TODO(), req)
	if err != nil {
		return err
	}
	fmt.Printf("Volume Group Membership modified for %+v\n", res)
	return nil
}

// DeleteVolumeGroup executes the DeleteVolumeGroup operation.
type DeleteVolumeGroup struct {
	grpcClient
	VolumeGroupBase
}

var _ = registerOperation("DeleteVolumeGroup", &DeleteVolumeGroup{})

func (dvg *DeleteVolumeGroup) Init(c *command) error {

	if err := dvg.SetVolumeGroupID(c.volumeGroupID); err != nil {
		return err
	}

	if err := dvg.SetSecret(c.secret); err != nil {
		return err
	}

	return nil
}

func (dvg *DeleteVolumeGroup) Execute() error {
	vgs := service.NewVolumeGroupServer(dvg.Client, getKubernetesClient())

	req := &proto.DeleteVolumeGroupRequest{
		VolumeGroupId:   dvg.volumeGroupID,
		SecretName:      dvg.secretName,
		SecretNamespace: dvg.secretNamespace,
	}

	_, err := vgs.DeleteVolumeGroup(context.TODO(), req)
	if err != nil {
		return err
	}
	fmt.Println("Volume Group Deleted.")
	return nil

}

// ControllerGetVolumeGroup executes the ControllerGetVolumeGroup operation.
type ControllerGetVolumeGroup struct {
	grpcClient
	VolumeGroupBase
}

var _ = registerOperation("ControllerGetVolumeGroup", &ControllerGetVolumeGroup{})

func (gvg *ControllerGetVolumeGroup) Init(c *command) error {

	if err := gvg.SetVolumeGroupID(c.volumeGroupID); err != nil {
		return err
	}
	if err := gvg.SetSecret(c.secret); err != nil {
		return err
	}

	return nil
}

func (gvg *ControllerGetVolumeGroup) Execute() error {
	vgs := service.NewVolumeGroupServer(gvg.Client, getKubernetesClient())

	req := &proto.ControllerGetVolumeGroupRequest{
		VolumeGroupId:   gvg.volumeGroupID,
		SecretName:      gvg.secretName,
		SecretNamespace: gvg.secretNamespace,
	}

	res, err := vgs.ControllerGetVolumeGroup(context.TODO(), req)
	if err != nil {
		return err
	}
	fmt.Printf("Controller Volume Group Info: %+v\n", res)
	return nil
}
