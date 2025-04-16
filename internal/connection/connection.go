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
	"sync/atomic"
	"time"

	"github.com/csi-addons/kubernetes-csi-addons/internal/kubernetes/token"

	"github.com/csi-addons/spec/lib/go/identity"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	// The duration after which a gRPC connection is closed due to inactivity
	idleTimeout = time.Minute * 5
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
	connected  bool
	enableAuth bool
	endpoint   string
	podName    string

	// Used to cancel any existing timers in case of re-connects
	cancelIdle context.CancelFunc
	// Holds the last access time of the gRPC Connection
	// It is tracked and updated by the `accessTimeInterceptor`
	lastAccessTime atomic.Int64
}

// accessTimeInterceptor is an unary interceptor which updates the lastAccessTime
// on `Connection` struct to `time.Now()` on each RPC call
func accessTimeInterceptor(conn *Connection) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		conn.lastAccessTime.Store(time.Now().UnixNano())

		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// Connect creates a new grpc.ClientConn object and sets it as the
// client property on Connection struct. If a connection is already
// connected, it is reused as is. It also spawns a go routine to tear
// down the connection if it has been idling for a specified threshold.
//
// In cases where a new connection is created from the scratch Connect
// also calls fetchCapabilities on the connection object.
func (c *Connection) Connect() error {
	c.Lock()
	defer c.Unlock()

	// Return early if already connected, the connection will be reused
	if c.connected {
		return nil
	}

	opts := []grpc.DialOption{
		grpc.WithUnaryInterceptor(accessTimeInterceptor(c)),
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
	c.connected = true

	// Fetch the caps
	// Downside is we need to do this on every re-connect
	if err := c.fetchCapabilities(context.Background()); err != nil {
		if e := c.Client.Close(); e == nil {
			c.connected = false
		}

		return err
	}

	// Start a goroutine to close the connection after idle timeout
	// But first, expire any existing timers
	if c.cancelIdle != nil {
		c.cancelIdle()
	}
	idleCtx, cFunc := context.WithCancel(context.Background())
	c.cancelIdle = cFunc
	go c.startIdleTimer(idleCtx)

	return nil
}

// startIdleTimer starts a ticker with an interval of 30 seconds
// At each tick, it checks if the connection has been idle for
// more than `idleTimeout`. If so, the connection is closed.
func (c *Connection) startIdleTimer(ctx context.Context) {
	ticker := time.NewTicker(time.Second * 30)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.Lock()

			lastAccess := time.Unix(0, c.lastAccessTime.Load())
			isIdle := time.Since(lastAccess) > idleTimeout
			if isIdle && c.connected {
				// It's okay if there's an error in tearing down the connection
				// It will be reused by subsequent requests, see Connect() for details
				if err := c.Client.Close(); err == nil {
					c.connected = false
				}
			}
			c.Unlock()

		case <-ctx.Done():
			// Timer was cancelled
			return
		}
	}
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

// Close tears down the gRPC connection and terminates the goroutines
// monitoring the idle timeout by calling cancelIdle()
func (c *Connection) Close() error {
	c.Lock()
	defer c.Unlock()

	if c.cancelIdle != nil {
		// Cancel the context as well
		c.cancelIdle()
		c.cancelIdle = nil
	}

	if c.Client != nil {
		err := c.Client.Close()
		if err == nil {
			c.connected = false
		}

		return err
	}

	return nil
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
