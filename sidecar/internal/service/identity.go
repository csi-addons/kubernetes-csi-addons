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

package service

import (
	"context"

	"github.com/csi-addons/spec/lib/go/identity"
	"google.golang.org/grpc"
)

// IdentityServer struct of sidecar with supported methods of CSI
// identity server spec and also containing client to csi driver.
type IdentityServer struct {
	*identity.UnimplementedIdentityServer
	identityClient identity.IdentityClient
}

// NewIdentityServer creates a new IdentityServer which handles the Identity
// Service requests from the CSI-Addons specification.
func NewIdentityServer(client *grpc.ClientConn) *IdentityServer {
	return &IdentityServer{
		identityClient: identity.NewIdentityClient(client),
	}
}

func (is *IdentityServer) RegisterService(server grpc.ServiceRegistrar) {
	identity.RegisterIdentityServer(server, is)
}

// GetIdentity returns available capabilities from the driver.
func (is *IdentityServer) GetIdentity(
	ctx context.Context,
	req *identity.GetIdentityRequest) (*identity.GetIdentityResponse, error) {
	return is.identityClient.GetIdentity(ctx, req)
}

// GetCapabilities returns available capabilities from the driver.
func (is *IdentityServer) GetCapabilities(
	ctx context.Context,
	req *identity.GetCapabilitiesRequest) (*identity.GetCapabilitiesResponse, error) {
	return is.identityClient.GetCapabilities(ctx, req)
}

// Probe is called by the CO plugin to validate that the CSI-Addons Node is
// still healthy.
func (is *IdentityServer) Probe(
	ctx context.Context,
	req *identity.ProbeRequest) (*identity.ProbeResponse, error) {
	return is.identityClient.Probe(ctx, req)
}
