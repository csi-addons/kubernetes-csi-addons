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

package server

import (
	"crypto/tls"
	"errors"
	"net"

	"github.com/csi-addons/kubernetes-csi-addons/internal/kubernetes/token"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// SidecarService is the interface that is required to be implemented so that
// the SidecarServer can register the service by calling RegisterService().
type SidecarService interface {
	// RegisterService is called by the SidecarServer to add a CSI-Addons
	// service that can handle requests.
	RegisterService(server grpc.ServiceRegistrar)
}

// SidecarServer is the gRPC server that listens on an endpoint
// where the CSI-Addons requests come in.
type SidecarServer struct {
	// URL components to listen on the tcp port
	scheme   string
	endpoint string
	client   *k8s.Clientset

	server           *grpc.Server
	services         []SidecarService
	enableAuthChecks bool
}

// NewSidecarServer create a new SidecarServer on the given IP-address and
// port. If the IP-address is an empty string, the server will listen on all
// available IP-addresses. Only tcp ports are supported.
func NewSidecarServer(ip, port string, client *k8s.Clientset, enableAuthChecks bool) *SidecarServer {
	ss := &SidecarServer{}

	if ss.services == nil {
		ss.services = make([]SidecarService, 0)
	}

	ss.scheme = "tcp"
	ss.endpoint = ip + ":" + port
	ss.client = client
	ss.enableAuthChecks = enableAuthChecks
	return ss
}

// RegisterService takes the SidecarService and registers it with the
// SidecarServer gRPC server. This function should be called before Start,
// where the services are registered on the internal gRPC server.
func (ss *SidecarServer) RegisterService(svc SidecarService) {
	ss.services = append(ss.services, svc)
}

// Init creates the internal gRPC server, and registers the SidecarServices.
// and starts gRPC server.
func (ss *SidecarServer) Start() {
	if ss.enableAuthChecks {
		cert, err := token.GenerateSelfSignedCert()
		if err != nil {
			panic("Failed to generate self-signed certificate: " + err.Error())
		}

		// Create TLS credentials
		creds := credentials.NewTLS(&tls.Config{
			Certificates: []tls.Certificate{cert},
		})
		// create the gRPC server and register services
		ss.server = grpc.NewServer(grpc.UnaryInterceptor(token.AuthorizationInterceptor(*ss.client)), grpc.Creds(creds))
	} else {
		ss.server = grpc.NewServer()
	}

	for _, svc := range ss.services {
		svc.RegisterService(ss.server)
	}

	listener, err := net.Listen(ss.scheme, ss.endpoint)
	if err != nil {
		klog.Fatalf("failed to listen on %s (%s): %v", ss.endpoint, ss.scheme, err)
	}

	ss.serve(listener)
}

// serve starts the actual process of listening for requests on the gRPC
// server.
func (ss *SidecarServer) serve(listener net.Listener) {
	klog.Infof("Listening for CSI-Addons requests on address: %s", listener.Addr())

	// start to serve requests
	err := ss.server.Serve(listener)
	if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		klog.Fatalf("Failed to setup CSI-Addons server: %v", err)
	}

	klog.Infof("The CSI-Addons server at %q has been stopped", listener.Addr())
}

// Stop can be used to stop the internal gRPC server.
func (ss *SidecarServer) Stop() {
	if ss.server == nil {
		return
	}

	ss.server.GracefulStop()
}
