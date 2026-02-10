/*
Copyright 2026 The Kubernetes-CSI-Addons Authors.

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
	"net"
	"testing"
	"time"

	"github.com/csi-addons/spec/lib/go/identity"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

type mockIdentityServer struct {
	identity.UnimplementedIdentityServer
	capabilities []*identity.Capability
}

func (m *mockIdentityServer) GetCapabilities(ctx context.Context, req *identity.GetCapabilitiesRequest) (*identity.GetCapabilitiesResponse, error) {
	return &identity.GetCapabilitiesResponse{
		Capabilities: m.capabilities,
	}, nil
}

// setupMockGRPCServer starts a mock gRPC server and returns the address
func setupMockGRPCServer(t *testing.T, caps []*identity.Capability) (string, func()) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	server := grpc.NewServer()
	identity.RegisterIdentityServer(server, &mockIdentityServer{capabilities: caps})

	go func() {
		_ = server.Serve(listener)
	}()

	cleanup := func() {
		server.Stop()
		_ = listener.Close()
	}

	return listener.Addr().String(), cleanup
}

func TestConnect_ReuseExistingConnection(t *testing.T) {
	caps := []*identity.Capability{
		{
			Type: &identity.Capability_Service_{
				Service: &identity.Capability_Service{
					Type: identity.Capability_Service_CONTROLLER_SERVICE,
				},
			},
		},
	}

	addr, cleanup := setupMockGRPCServer(t, caps)
	defer cleanup()

	conn := &Connection{
		Namespace:  "test-ns",
		Name:       "test-pod",
		NodeID:     "test-node",
		DriverName: "test-driver",
		Timeout:    time.Second * 5,
		endpoint:   addr,
		podName:    "test-pod",
		enableAuth: false,
	}

	// First connection
	err := conn.Connect()
	if !assert.NoError(t, err) {
		return
	}
	if !assert.NotNil(t, conn.Client) {
		return
	}

	firstClient := conn.Client

	// Second Connect() should reuse the existing connection
	err = conn.Connect()
	if !assert.NoError(t, err) {
		return
	}
	assert.Same(t, firstClient, conn.Client, "Connect() should reuse existing connection")
}

func TestConnect_RecreateOnShutdown(t *testing.T) {
	caps := []*identity.Capability{
		{
			Type: &identity.Capability_Service_{
				Service: &identity.Capability_Service{
					Type: identity.Capability_Service_CONTROLLER_SERVICE,
				},
			},
		},
	}

	addr, cleanup := setupMockGRPCServer(t, caps)
	defer cleanup()

	conn := &Connection{
		Namespace:  "test-ns",
		Name:       "test-pod",
		NodeID:     "test-node",
		DriverName: "test-driver",
		Timeout:    time.Second * 5,
		endpoint:   addr,
		podName:    "test-pod",
		enableAuth: false,
	}

	// Create initial connection
	err := conn.Connect()
	if !assert.NoError(t, err) {
		return
	}
	if !assert.NotNil(t, conn.Client) {
		return
	}

	firstClient := conn.Client

	// Close the connection to put it in Shutdown state
	err = firstClient.Close()
	if !assert.NoError(t, err) {
		return
	}

	// Wait for the connection to reach Shutdown state
	timeout := time.After(time.Second * 2)
	ticker := time.NewTicker(time.Millisecond * 100)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Fatal("Connection should reach Shutdown state")
			return
		case <-ticker.C:
			if firstClient.GetState() == connectivity.Shutdown {
				goto shutdownReached
			}
		}
	}
shutdownReached:
	// Connect() should create a new connection
	err = conn.Connect()
	if !assert.NoError(t, err) {
		return
	}
	if !assert.NotNil(t, conn.Client) {
		return
	}
	assert.NotSame(t, firstClient, conn.Client, "Connect() should create new connection when in Shutdown state")
}

func TestConnect_NilClient(t *testing.T) {
	caps := []*identity.Capability{
		{
			Type: &identity.Capability_Service_{
				Service: &identity.Capability_Service{
					Type: identity.Capability_Service_CONTROLLER_SERVICE,
				},
			},
		},
	}

	addr, cleanup := setupMockGRPCServer(t, caps)
	defer cleanup()

	conn := &Connection{
		Namespace:  "test-ns",
		Name:       "test-pod",
		NodeID:     "test-node",
		DriverName: "test-driver",
		Timeout:    time.Second * 5,
		endpoint:   addr,
		podName:    "test-pod",
		enableAuth: false,
		Client:     nil,
	}

	err := conn.Connect()
	if !assert.NoError(t, err) {
		return
	}
	assert.NotNil(t, conn.Client, "Connect() should create a new client when Client is nil")
}

func TestClose(t *testing.T) {
	caps := []*identity.Capability{
		{
			Type: &identity.Capability_Service_{
				Service: &identity.Capability_Service{
					Type: identity.Capability_Service_CONTROLLER_SERVICE,
				},
			},
		},
	}

	addr, cleanup := setupMockGRPCServer(t, caps)
	defer cleanup()

	conn := &Connection{
		Namespace:  "test-ns",
		Name:       "test-pod",
		NodeID:     "test-node",
		DriverName: "test-driver",
		Timeout:    time.Second * 5,
		endpoint:   addr,
		podName:    "test-pod",
		enableAuth: false,
	}

	err := conn.Connect()
	if !assert.NoError(t, err) {
		return
	}
	if !assert.NotNil(t, conn.Client) {
		return
	}

	err = conn.Close()
	assert.NoError(t, err)
	assert.Nil(t, conn.Client, "Client should be nil after Close()")

	// Close on nil client should not error
	err = conn.Close()
	assert.NoError(t, err)
}

func TestConnect_ConcurrentAccess(t *testing.T) {
	caps := []*identity.Capability{
		{
			Type: &identity.Capability_Service_{
				Service: &identity.Capability_Service{
					Type: identity.Capability_Service_CONTROLLER_SERVICE,
				},
			},
		},
	}

	addr, cleanup := setupMockGRPCServer(t, caps)
	defer cleanup()

	conn := &Connection{
		Namespace:  "test-ns",
		Name:       "test-pod",
		NodeID:     "test-node",
		DriverName: "test-driver",
		Timeout:    time.Second * 5,
		endpoint:   addr,
		podName:    "test-pod",
		enableAuth: false,
	}

	// Test concurrent Connect() calls to verify locking works
	done := make(chan bool, 10)
	for range 10 {
		go func() {
			err := conn.Connect()
			assert.NoError(t, err)
			done <- true
		}()
	}

	for range 10 {
		<-done
	}

	assert.NotNil(t, conn.Client, "Client should be set after concurrent Connect() calls")
}

func TestNewConnection(t *testing.T) {
	caps := []*identity.Capability{
		{
			Type: &identity.Capability_Service_{
				Service: &identity.Capability_Service{
					Type: identity.Capability_Service_CONTROLLER_SERVICE,
				},
			},
		},
	}

	addr, cleanup := setupMockGRPCServer(t, caps)
	defer cleanup()

	conn, err := NewConnection(
		context.Background(),
		addr,
		"test-node",
		"test-driver",
		"test-ns",
		"test-pod",
		false,
	)

	if !assert.NoError(t, err) {
		return
	}
	if !assert.NotNil(t, conn) {
		return
	}
	assert.NotNil(t, conn.Client)
	assert.Equal(t, "test-node", conn.NodeID)
	assert.Equal(t, "test-driver", conn.DriverName)
	assert.Equal(t, "test-ns", conn.Namespace)
	assert.Equal(t, "test-pod", conn.Name)
	assert.Equal(t, time.Minute, conn.Timeout)
	assert.True(t, conn.HasControllerService())
}
