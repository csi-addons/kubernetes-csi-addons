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
	"fmt"
	"strings"

	"github.com/csi-addons/kubernetes-csi-addons/internal/proto"
	"github.com/csi-addons/kubernetes-csi-addons/internal/sidecar/service"
)

// NetworkFenceServer executes the NetworkFenceServer operation.
type networkFenceBase struct {
	// inherit Connect() and Close() from type grpcClient
	grpcClient

	parameters      map[string]string
	secretName      string
	secretNamespace string
	cidrs           []string
}

func (ns *networkFenceBase) Init(c *command) error {
	ns.parameters = make(map[string]string)
	ns.parameters["clusterID"] = c.clusterid
	if ns.parameters["clusterID"] == "" {
		return fmt.Errorf("clusterID not set")
	}

	secrets := strings.Split(c.secret, "/")
	if len(secrets) != 2 {
		return fmt.Errorf("secret should be specified in the format `namespace/name`")
	}
	ns.secretNamespace = secrets[0]
	if ns.secretNamespace == "" {
		return fmt.Errorf("secret namespace is not set")
	}

	ns.secretName = secrets[1]
	if ns.secretName == "" {
		return fmt.Errorf("secret name is not set")
	}

	ns.cidrs = (strings.Split(c.cidrs, ","))
	if len(ns.cidrs) == 0 || (len(ns.cidrs) == 1 && ns.cidrs[0] == "") {
		return fmt.Errorf("cidrs not set")
	}
	return nil
}

type NetworkFenceServer struct {
	networkFenceBase
}

var _ = registerOperation("NetworkFence", &NetworkFenceServer{})

func (ns *NetworkFenceServer) Execute() error {
	k := getKubernetesClient()

	nfs := service.NewNetworkFenceServer(ns.Client, k)

	req := &proto.NetworkFenceRequest{
		Parameters:      ns.parameters,
		SecretName:      ns.secretName,
		SecretNamespace: ns.secretNamespace,
		Cidrs:           ns.cidrs,
	}

	_, err := nfs.FenceClusterNetwork(context.TODO(), req)
	if err != nil {
		return err
	}

	fmt.Printf("Network fence successful")
	return nil
}

type NetworkUnFenceServer struct {
	networkFenceBase
}

var _ = registerOperation("NetworkUnFence", &NetworkUnFenceServer{})

func (ns *NetworkUnFenceServer) Execute() error {
	k := getKubernetesClient()

	nfs := service.NewNetworkFenceServer(ns.Client, k)

	req := &proto.NetworkFenceRequest{
		Parameters:      ns.parameters,
		SecretName:      ns.secretName,
		SecretNamespace: ns.secretNamespace,
		Cidrs:           ns.cidrs,
	}

	_, err := nfs.UnFenceClusterNetwork(context.TODO(), req)
	if err != nil {
		return err
	}

	fmt.Printf("Network unfence successful")
	return nil
}
