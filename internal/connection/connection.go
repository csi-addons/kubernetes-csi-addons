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
	"crypto/tls"
	"sync"
	"time"

	"github.com/csi-addons/kubernetes-csi-addons/internal/kubernetes/token"

	"github.com/csi-addons/spec/lib/go/identity"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

const (
// The duration after which a gRPC connection is closed due to inactivity
// The `WithIdleTimeout` dial option will close the underlying TCP connection
// and will automatically re-open it whenever a new RPC/Connect() is called.
// idleTimeout = time.Minute * 5
)

// Connection struct consists of to NodeID, DriverName, Capabilities for the controller
// to pick sidecar connection and grpc Client to connect to the sidecar.
type Connection struct {
	sync.Mutex

	Client       *grpc.ClientConn
	Capabilities []*identity.Capability
	Namespace    string
	Name         string
	NodeID       string
	DriverName   string
	Timeout      time.Duration

	// Holds the internal state of the connection
	enableAuth bool
	endpoint   string
	podName    string
}

// Connect creates a new grpc.ClientConn object and sets it as the
// client property on Connection struct. If a connection is already
// connected, it is reused as is.
// In cases where a new connection is created from the scratch Connect
// also calls fetchCapabilities on the connection object.
func (c *Connection) Connect() error {
	c.Lock()
	defer c.Unlock()

	// If the connection is not shutting down, we will be reusing it as-is
	// This is required in spite of having an idleTimeout as Close() call might hang
	// if the network is flaky. In that case, we discard the connection and create
	// a new one.
	if c.Client != nil {
		if c.Client.GetState() != connectivity.Shutdown {
			return nil
		}

		// We will have to re-create the client
		c.Client = nil
	}

	opts := []grpc.DialOption{
		// TODO: Reuse once the API is stable
		//
		// This API is experimental, and hence we are disabling it for now
		// The default timeout on every gRPC connection in absence of this
		// dial option is 30 minutes.
		//
		// grpc.WithIdleTimeout(idleTimeout),
	}
	if c.enableAuth {
		opts = append(opts, token.WithServiceAccountToken())
		tlsConfig := &tls.Config{
			// Certs are only used to initiate HTTPS connections; authorization is handled by SA tokens
			InsecureSkipVerify: true,
		}
		creds := credentials.NewTLS(tlsConfig)
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	cc, err := grpc.NewClient(c.endpoint, opts...)
	if err != nil {
		return err
	}
	c.Client = cc

	// Fetch the caps
	// Downside is we need to do this on every re-connect
	if err := c.fetchCapabilities(context.Background()); err != nil {
		_ = c.Client.Close()
		c.Client = nil

		return err
	}

	return nil
}

// NewConnection establishes connection with sidecar, fetches capability and returns Connection object
// filled with required information.
func NewConnection(ctx context.Context, endpoint, nodeID, driverName, namespace, podName string, enableAuth bool) (*Connection, error) {

	conn := &Connection{
		Namespace:  namespace,
		Name:       podName,
		NodeID:     nodeID,
		DriverName: driverName,
		Timeout:    time.Minute,

		endpoint:   endpoint,
		podName:    podName,
		enableAuth: enableAuth,
	}

	if err := conn.Connect(); err != nil {
		return nil, err
	}

	return conn, nil
}

// Close tears down the gRPC connection and sets the client
// to nil to ensure the call to Close() is idempotent.
// A new client will always be created once Close() is called.
func (c *Connection) Close() error {
	c.Lock()
	defer c.Unlock()

	if c.Client == nil {
		return nil
	}

	err := c.Client.Close()
	c.Client = nil

	return err
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
