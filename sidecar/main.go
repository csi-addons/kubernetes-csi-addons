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

package main

import (
	"flag"
	"time"

	"github.com/csi-addons/kubernetes-csi-addons/internal/sidecar/service"
	"github.com/csi-addons/kubernetes-csi-addons/sidecar/internal/client"
	"github.com/csi-addons/kubernetes-csi-addons/sidecar/internal/csiaddonsnode"
	"github.com/csi-addons/kubernetes-csi-addons/sidecar/internal/server"
	"github.com/csi-addons/kubernetes-csi-addons/sidecar/internal/util"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

func main() {
	var (
		defaultTimeout     = time.Minute * 3
		defaultStagingPath = "/var/lib/kubelet/plugins/kubernetes.io/csi/"
		timeout            = flag.Duration("timeout", defaultTimeout, "Timeout for waiting for response")
		csiAddonsAddress   = flag.String("csi-addons-address", "/run/csi-addons/socket", "CSI Addons endopoint")
		nodeID             = flag.String("node-id", "", "NodeID")
		stagingPath        = flag.String("stagingpath", defaultStagingPath, "stagingpath")
		controllerPort     = flag.String("controller-port", "",
			"The TCP network port where the gRPC server for controller request, will listen (example: `8080`)")
		controllerIP = flag.String("controller-ip", "",
			"The TCP network ip address where the gRPC server for controller request, will listen (example: `192.168.61.228`)")
		podName      = flag.String("pod", "", "name of the Pod that contains this sidecar")
		podNamespace = flag.String("namespace", "", "namespace of the Pod that contains this sidecar")
		podUID       = flag.String("pod-uid", "", "UID of the Pod that contains this sidecar")
	)
	klog.InitFlags(nil)
	if err := flag.Set("logtostderr", "true"); err != nil {
		klog.Exitf("failed to set logtostderr flag: %v", err)
	}
	flag.Parse()

	controllerEndpoint, err := util.BuildEndpointURL(*controllerIP, *controllerPort, *podName, *podNamespace)
	if err != nil {
		klog.Fatalf("Failed to validate controller endpoint: %w", err)
	}

	csiClient, err := client.New(*csiAddonsAddress, *timeout)
	if err != nil {
		klog.Fatalf("Failed to connect to %q : %w", *csiAddonsAddress, err)
	}

	err = csiClient.Probe()
	if err != nil {
		klog.Fatalf("Failed to probe driver: %w", err)
	}

	cfg, err := rest.InClusterConfig()
	if err != nil {
		klog.Fatalf("Failed to get cluster config: %w", err)
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Failed to create client: %v", err)
	}

	nodeMgr := &csiaddonsnode.Manager{
		Client:       csiClient,
		Config:       cfg,
		Node:         *nodeID,
		Endpoint:     controllerEndpoint,
		PodName:      *podName,
		PodNamespace: *podNamespace,
		PodUID:       *podUID,
	}
	err = nodeMgr.Deploy()
	if err != nil {
		klog.Fatalf("Failed to create csiaddonsnode: %v", err)
	}

	sidecarServer := server.NewSidecarServer(*controllerIP, *controllerPort)
	sidecarServer.RegisterService(service.NewIdentityServer(csiClient.GetGRPCClient()))
	sidecarServer.RegisterService(service.NewReclaimSpaceServer(csiClient.GetGRPCClient(), kubeClient, *stagingPath))
	sidecarServer.RegisterService(service.NewNetworkFenceServer(csiClient.GetGRPCClient(), kubeClient))

	sidecarServer.Start()
}
