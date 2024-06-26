/*
Copyright 2023 The Ceph-CSI Authors.

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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type VolumeReplicationBase struct {
	// inherit Connect() and Close() from type grpcClient
	grpcClient

	parameters      map[string]string
	secretName      string
	secretNamespace string
	volumeID        string
	groupID         string
}

func (rep *VolumeReplicationBase) Init(c *command) error {
	rep.parameters = make(map[string]string)
	rep.parameters["clusterID"] = c.clusterid
	if rep.parameters["clusterID"] == "" {
		return errors.New("clusterID not set")
	}

	secrets := strings.Split(c.secret, "/")
	if len(secrets) != 2 {
		return errors.New("secret should be specified in the format `namespace/name`")
	}
	rep.secretNamespace = secrets[0]
	if rep.secretNamespace == "" {
		return errors.New("secret namespace is not set")
	}

	rep.secretName = secrets[1]
	if rep.secretName == "" {
		return errors.New("secret name is not set")
	}

	if c.persistentVolume != "" && c.volumeGroupReplicationContent != "" {
		return errors.New("only one of persistentVolume or volumeGroupReplicationContent should be set")
	}

	if c.persistentVolume != "" {
		pv, err := getKubernetesClient().CoreV1().PersistentVolumes().Get(context.Background(), c.persistentVolume, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get pv %q", c.persistentVolume)
		}

		if pv.Spec.CSI == nil {
			return fmt.Errorf("pv %q is not a CSI volume", c.persistentVolume)
		}

		if pv.Spec.CSI.VolumeHandle == "" {
			return errors.New("volume ID is not set")
		}
		rep.volumeID = pv.Spec.CSI.VolumeHandle
		return nil
	} else if c.volumeGroupReplicationContent != "" {
		vgrc, err := getVolumeReplicationClient().getVolumeGroupReplicationContent(context.Background(), c.volumeGroupReplicationContent, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get VolumeGroupReplicationContent %q", c.volumeGroupReplicationContent)
		}
		if vgrc.Spec.VolumeGroupReplicationHandle == "" {
			return errors.New("volume group ID is not set")
		}
		rep.groupID = vgrc.Spec.VolumeGroupReplicationHandle
		return nil
	}

	return errors.New("either persistentVolume or volumeGroupReplicationContent should be set")
}

// EnableVolumeReplication executes the EnableVolumeReplication operation.
type EnableVolumeReplication struct {
	VolumeReplicationBase
}

func (v VolumeReplicationBase) setReplicationSource(req *proto.ReplicationSource) error {
	switch {
	case req == nil:
		return errors.New("replication source is not set")
	case v.volumeID != "" && v.groupID != "":
		return errors.New("only one of volumeID or groupID should be set")
	case v.volumeID != "":
		req.Type = &proto.ReplicationSource_Volume{
			Volume: &proto.ReplicationSource_VolumeSource{
				VolumeId: v.volumeID,
			},
		}
		return nil
	case v.groupID != "":
		req.Type = &proto.ReplicationSource_VolumeGroup{
			VolumeGroup: &proto.ReplicationSource_VolumeGroupSource{
				VolumeGroupId: v.groupID,
			},
		}
		return nil
	}
	return errors.New("both volumeID and groupID is not set")
}

var _ = registerOperation("EnableVolumeReplication", &EnableVolumeReplication{})

func (rep *EnableVolumeReplication) Execute() error {
	k := getKubernetesClient()

	rs := service.NewReplicationServer(rep.Client, k)

	req := &proto.EnableVolumeReplicationRequest{
		SecretName:      rep.secretName,
		SecretNamespace: rep.secretNamespace,
	}
	err := rep.setReplicationSource(req.ReplicationSource)
	if err != nil {
		return err
	}
	_, err = rs.EnableVolumeReplication(context.TODO(), req)
	if err != nil {
		return err
	}

	fmt.Println("Enable Volume Replication operation successful")

	return nil
}

// DisableVolumeReplication executes the DisableVolumeReplication operation.
type DisableVolumeReplication struct {
	VolumeReplicationBase
}

var _ = registerOperation("DisableVolumeReplication", &DisableVolumeReplication{})

func (rep *DisableVolumeReplication) Execute() error {
	k := getKubernetesClient()

	rs := service.NewReplicationServer(rep.Client, k)

	req := &proto.DisableVolumeReplicationRequest{
		SecretName:      rep.secretName,
		SecretNamespace: rep.secretNamespace,
	}
	err := rep.setReplicationSource(req.ReplicationSource)
	if err != nil {
		return err
	}

	_, err = rs.DisableVolumeReplication(context.TODO(), req)
	if err != nil {
		return err
	}

	fmt.Println("Disable Volume Replication operation successful")

	return nil
}

// PromoteVolume executes the PromoteVolume operation.
type PromoteVolume struct {
	VolumeReplicationBase
}

var _ = registerOperation("PromoteVolume", &PromoteVolume{})

func (rep *PromoteVolume) Execute() error {
	k := getKubernetesClient()

	rs := service.NewReplicationServer(rep.Client, k)

	req := &proto.PromoteVolumeRequest{
		SecretName:      rep.secretName,
		SecretNamespace: rep.secretNamespace,
	}
	err := rep.setReplicationSource(req.ReplicationSource)
	if err != nil {
		return err
	}
	_, err = rs.PromoteVolume(context.TODO(), req)
	if err != nil {
		return err
	}

	fmt.Println("Promote Volume operation successful")

	return nil
}

// DemoteVolume executes the DemoteVolume operation.
type DemoteVolume struct {
	VolumeReplicationBase
}

var _ = registerOperation("DemoteVolume", &DemoteVolume{})

func (rep *DemoteVolume) Execute() error {
	k := getKubernetesClient()

	rs := service.NewReplicationServer(rep.Client, k)

	req := &proto.DemoteVolumeRequest{
		SecretName:      rep.secretName,
		SecretNamespace: rep.secretNamespace,
	}
	err := rep.setReplicationSource(req.ReplicationSource)
	if err != nil {
		return err
	}
	_, err = rs.DemoteVolume(context.TODO(), req)
	if err != nil {
		return err
	}

	fmt.Println("Demote Volume operation successful")

	return nil
}

// ResyncVolume executes the ResyncVolume operation.
type ResyncVolume struct {
	VolumeReplicationBase
}

var _ = registerOperation("ResyncVolume", &ResyncVolume{})

func (rep *ResyncVolume) Execute() error {
	k := getKubernetesClient()

	rs := service.NewReplicationServer(rep.Client, k)

	req := &proto.ResyncVolumeRequest{
		SecretName:      rep.secretName,
		SecretNamespace: rep.secretNamespace,
	}
	err := rep.setReplicationSource(req.ReplicationSource)
	if err != nil {
		return err
	}
	_, err = rs.ResyncVolume(context.TODO(), req)
	if err != nil {
		return err
	}

	fmt.Println("Resync Volume operation successful")

	return nil
}

// GetVolumeReplicationInfo executes the GetVolumeReplicationInfo operation.
type GetVolumeReplicationInfo struct {
	VolumeReplicationBase
}

var _ = registerOperation("GetVolumeReplicationInfo", &GetVolumeReplicationInfo{})

func (rep *GetVolumeReplicationInfo) Execute() error {
	k := getKubernetesClient()

	rs := service.NewReplicationServer(rep.Client, k)

	req := &proto.GetVolumeReplicationInfoRequest{
		SecretName:      rep.secretName,
		SecretNamespace: rep.secretNamespace,
	}
	err := rep.setReplicationSource(req.ReplicationSource)
	if err != nil {
		return err
	}

	res, err := rs.GetVolumeReplicationInfo(context.TODO(), req)
	if err != nil {
		return err
	}

	fmt.Printf("replication info for %q: %+v\n", rep.volumeID, res)

	return nil
}
