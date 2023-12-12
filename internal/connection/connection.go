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

package connection

import (
	"context"
	"time"

	"github.com/csi-addons/spec/lib/go/identity"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Connection struct consists of to NodeID, DriverName, Capabilities for the controller
// to pick sidecar connection and grpc Client to connect to the sidecar.
type Connection struct {
	Client       *grpc.ClientConn
	Capabilities []*identity.Capability
	Namespace    string
	Name         string
	NodeID       string
	DriverName   string
	Timeout      time.Duration
}

// NewConnection establishes connection with sidecar, fetches capability and returns Connection object
// filled with required information.
func NewConnection(ctx context.Context, endpoint, nodeID, driverName, namespace, podName string) (*Connection, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithIdleTimeout(time.Duration(0)),
	}
	cc, err := grpc.Dial(endpoint, opts...)
	if err != nil {
		return nil, err
	}

	conn := &Connection{
		Client:     cc,
		Namespace:  namespace,
		Name:       podName,
		NodeID:     nodeID,
		DriverName: driverName,
		Timeout:    time.Minute,
	}

	err = conn.fetchCapabilities(ctx)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (c *Connection) Close() {
	if c.Client != nil {
		c.Client.Close()
	}
}

// fetchCapabilities fetches the capability of the connected CSI driver.
func (c *Connection) fetchCapabilities(ctx context.Context) error {
	newCtx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()

	identityClient := identity.NewIdentityClient(c.Client)
	res, err := identityClient.GetCapabilities(newCtx, &identity.GetCapabilitiesRequest{})
	if err != nil {
		return err
	}

	c.Capabilities = res.GetCapabilities()

	return nil
}

func (c *Connection) HasControllerService() bool {
	for _, capability := range c.Capabilities {
		svc := capability.GetService()
		if svc != nil && svc.GetType() == identity.Capability_Service_CONTROLLER_SERVICE {
			return true
		}
	}

	return false
}
