/*
Copyright 2022 The Ceph-CSI Authors.

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
	"crypto/sha256"
	"fmt"

	"github.com/csi-addons/kubernetes-csi-addons/internal/proto"
	"github.com/csi-addons/kubernetes-csi-addons/internal/sidecar/service"
)

// ControllerReclaimSpace executes the ControllerReclaimSpace operation.
type ControllerReclaimSpace struct {
	// inherit Connect() and Close() from type grpcClient
	grpcClient

	persistentVolume string
}

var _ = registerOperation("ControllerReclaimSpace", &ControllerReclaimSpace{})

func (crs *ControllerReclaimSpace) Init(c *command) error {
	crs.persistentVolume = c.persistentVolume

	if crs.persistentVolume == "" {
		return fmt.Errorf("persistentvolume name is not set")
	}

	return nil
}

func (crs *ControllerReclaimSpace) Execute() error {
	k := getKubernetesClient()

	rss := service.NewReclaimSpaceServer(crs.Client, k, "" /* staging path not used */)

	req := &proto.ReclaimSpaceRequest{
		PvName: crs.persistentVolume,
	}

	res, err := rss.ControllerReclaimSpace(context.TODO(), req)
	if err != nil {
		return err
	}

	fmt.Printf("space reclaimed for %q: %+v\n", crs.persistentVolume, res)

	return nil
}

// NodeReclaimSpace executes the NodeReclaimSpace operation.
type NodeReclaimSpace struct {
	// inherit Connect() and Close() from type grpcClient
	grpcClient

	persistentVolume  string
	stagingTargetPath string
}

var _ = registerOperation("NodeReclaimSpace", &NodeReclaimSpace{})

func (nrs *NodeReclaimSpace) Init(c *command) error {
	nrs.persistentVolume = c.persistentVolume
	if nrs.persistentVolume == "" {
		return fmt.Errorf("persistentvolume name is not set")
	}

	if c.stagingPath == "" {
		return fmt.Errorf("stagingpath is not set")
	}

	if !c.legacy {
		if c.drivername == "" {
			return fmt.Errorf("drivername is not set")
		}

		unique := sha256.Sum256([]byte(c.persistentVolume))
		nrs.stagingTargetPath = fmt.Sprintf("%s/%s/%s/globalmount", c.stagingPath, c.drivername, fmt.Sprintf("%x", unique))
	} else {
		nrs.stagingTargetPath = fmt.Sprintf("%s/pv/%s/globalmount", c.stagingPath, c.persistentVolume)
	}

	return nil
}

func (nrs *NodeReclaimSpace) Execute() error {
	k := getKubernetesClient()

	rss := service.NewReclaimSpaceServer(nrs.Client, k, nrs.stagingTargetPath)

	req := &proto.ReclaimSpaceRequest{
		PvName: nrs.persistentVolume,
	}

	res, err := rss.NodeReclaimSpace(context.TODO(), req)
	if err != nil {
		return err
	}

	fmt.Printf("space reclaimed for %q: %+v\n", nrs.persistentVolume, res)

	return nil
}
