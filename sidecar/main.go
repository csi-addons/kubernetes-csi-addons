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
	"context"
	"flag"
	"time"

	"github.com/csi-addons/kubernetes-csi-addons/internal/sidecar/service"
	"github.com/csi-addons/kubernetes-csi-addons/internal/util"
	"github.com/csi-addons/kubernetes-csi-addons/internal/version"
	"github.com/csi-addons/kubernetes-csi-addons/sidecar/internal/client"
	"github.com/csi-addons/kubernetes-csi-addons/sidecar/internal/csiaddonsnode"
	"github.com/csi-addons/kubernetes-csi-addons/sidecar/internal/server"
	sideutil "github.com/csi-addons/kubernetes-csi-addons/sidecar/internal/util"

	"github.com/kubernetes-csi/csi-lib-utils/leaderelection"
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
		showVersion  = flag.Bool("version", false, "Print Version details")

		leaderElectionNamespace     = flag.String("leader-election-namespace", "", "The namespace where the leader election resource exists. Defaults to the pod namespace if not set.")
		leaderElectionLeaseDuration = flag.Duration("leader-election-lease-duration", 15*time.Second, "Duration, in seconds, that non-leader candidates will wait to force acquire leadership. Defaults to 15 seconds.")
		leaderElectionRenewDeadline = flag.Duration("leader-election-renew-deadline", 10*time.Second, "Duration, in seconds, that the acting leader will retry refreshing leadership before giving up. Defaults to 10 seconds.")
		leaderElectionRetryPeriod   = flag.Duration("leader-election-retry-period", 5*time.Second, "Duration, in seconds, the LeaderElector clients should wait between tries of actions. Defaults to 5 seconds.")
	)
	klog.InitFlags(nil)

	if err := flag.Set("logtostderr", "true"); err != nil {
		klog.Exitf("failed to set logtostderr flag: %v", err)
	}

	flag.Parse()

	if *showVersion {
		version.PrintVersion()
		return
	}

	controllerEndpoint, err := sideutil.BuildEndpointURL(*controllerIP, *controllerPort, *podName, *podNamespace)
	if err != nil {
		klog.Fatalf("Failed to validate controller endpoint: %v", err)
	}

	csiClient, err := client.New(context.Background(), *csiAddonsAddress, *timeout)
	if err != nil {
		klog.Fatalf("Failed to connect to %q : %v", *csiAddonsAddress, err)
	}

	err = csiClient.Probe()
	if err != nil {
		klog.Fatalf("Failed to probe driver: %v", err)
	}

	cfg, err := rest.InClusterConfig()
	if err != nil {
		klog.Fatalf("Failed to get cluster config: %v", err)
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
	sidecarServer.RegisterService(service.NewReplicationServer(csiClient.GetGRPCClient(), kubeClient))
	sidecarServer.RegisterService(service.NewEncryptionKeyRotationServer(csiClient.GetGRPCClient(), kubeClient))
	sidecarServer.RegisterService(service.NewVolumeGroupServer(csiClient.GetGRPCClient(), kubeClient))

	isController, err := csiClient.HasControllerService()
	if err != nil {
		klog.Fatalf("Failed to check if the CSI-plugin supports CONTROLLER_SERVICE: %v", err)
	}

	// do not use leaderelection when the CSI-plugin does not have
	// CONTROLLER_SERVICE
	if !isController {
		klog.Info("The CSI-plugin does not have the CSI-Addons CONTROLLER_SERVICE capability, not running leader election")
		sidecarServer.Start()
	} else {
		// start the server in a go-routine so that the controller can
		// connect to it, even if this service is not the leaser
		go sidecarServer.Start()

		driver, err := csiClient.GetDriverName()
		if err != nil {
			klog.Fatalf("Failed to get the drivername from the CSI-plugin: %v", err)
		}

		leaseName := util.NormalizeLeaseName(driver) + "-csi-addons"
		le := leaderelection.NewLeaderElection(kubeClient, leaseName, func(context.Context) {
			klog.Infof("Obtained leader status: lease name %q, receiving CONTROLLER_SERVICE requests", leaseName)
		})

		if *podName != "" {
			le.WithIdentity(*podName)
		}

		if *leaderElectionNamespace != "" {
			le.WithNamespace(*leaderElectionNamespace)
		}

		le.WithLeaseDuration(*leaderElectionLeaseDuration)
		le.WithRenewDeadline(*leaderElectionRenewDeadline)
		le.WithRetryPeriod(*leaderElectionRetryPeriod)

		// le.Run() is not expected to return on success
		err = le.Run()
		if err != nil {
			klog.Fatalf("Failed to run as a leader: %v", err)
		}

		klog.Fatalf("Lost leader status: lease name %q, no longer receiving CONTROLLER_REQUESTS", leaseName)
	}
}
