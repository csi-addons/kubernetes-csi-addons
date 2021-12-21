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
	"fmt"

	"github.com/csi-addons/spec/lib/go/identity"

	"github.com/csi-addons/kubernetes-csi-addons/internal/sidecar/service"
)

// GetIdentity executes the GetIdentity operation.
type GetIdentity struct {
	// inherit Connect() and Close() from type grpcClient
	grpcClient
}

var _ = registerOperation("GetIdentity", &GetIdentity{})

func (gi *GetIdentity) Init(c *command) error {
	return nil
}

func (gi *GetIdentity) Execute() error {
	is := service.NewIdentityServer(gi.Client)
	res, err := is.GetIdentity(context.TODO(), &identity.GetIdentityRequest{})
	if err != nil {
		return err
	}

	fmt.Printf("identity: %+v\n", res)

	return nil
}

// GetCapabilities executes the GetCapabilities operation.
type GetCapabilities struct {
	// inherit Connect() and Close() from type grpcClient
	grpcClient
}

var _ = registerOperation("GetCapabilities", &GetCapabilities{})

func (gc *GetCapabilities) Init(c *command) error {
	return nil
}

func (gc *GetCapabilities) Execute() error {
	is := service.NewIdentityServer(gc.Client)
	res, err := is.GetCapabilities(context.TODO(), &identity.GetCapabilitiesRequest{})
	if err != nil {
		return err
	}

	fmt.Printf("capabilities: %+v\n", res)

	return nil
}

// Probe executes the Probe operation.
type Probe struct {
	// inherit Connect() and Close() from type grpcClient
	grpcClient
}

var _ = registerOperation("Probe", &Probe{})

func (p *Probe) Init(c *command) error {
	return nil
}

func (p *Probe) Execute() error {
	is := service.NewIdentityServer(p.Client)
	res, err := is.Probe(context.TODO(), &identity.ProbeRequest{})
	if err != nil {
		return err
	}

	fmt.Printf("probe succeeded: %+v\n", res)

	return nil
}
