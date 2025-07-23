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

package client

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/csi-addons/spec/lib/go/identity"
	"github.com/kubernetes-csi/csi-lib-utils/connection"
	"github.com/kubernetes-csi/csi-lib-utils/metrics"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
)

// Client is an interface that describes the methods which can be called on the
// client connection.
// This is mostly useful for (unit) testing where a client can not connect to a
// Kubernetes cluster.
type Client interface {
	GetGRPCClient() *grpc.ClientConn
	Probe() error
	GetDriverName() (string, error)
	HasControllerService() (bool, error)
}

// clientImpl holds the GRPC connenction details
type clientImpl struct {
	client  *grpc.ClientConn
	Timeout time.Duration
	// Interval of trying to call Probe() until it succeeds
	probeInterval time.Duration
}

// Connect to the GRPC client
func (c *clientImpl) connect(ctx context.Context, address string) (*grpc.ClientConn, error) {
	return connection.Connect(ctx, address, metrics.NewCSIMetricsManager(""), connection.OnConnectionLoss(connection.ExitOnConnectionLoss()))
}

// New creates and returns the GRPC client
func New(ctx context.Context, address string, timeout time.Duration) (Client, error) {
	c := &clientImpl{}
	cc, err := c.connect(ctx, address)
	if err != nil {
		return nil, err
	}
	c.client = cc
	c.Timeout = timeout
	c.probeInterval = time.Second
	return c, nil
}

// GetGRPCClient returns the connected GRPC client.
func (c *clientImpl) GetGRPCClient() *grpc.ClientConn {
	return c.client
}

// Probe calls Probe() of a CSI driver and waits until the driver becomes ready.
// Any error other than timeout is returned.
func (c *clientImpl) Probe() error {
	for {
		klog.Info("Probing CSI driver for readiness")
		ready, err := c.probeOnce()
		if err != nil {
			st, ok := status.FromError(err)
			if !ok {
				// This is not gRPC error. The probe must have failed before gRPC
				// method was called, otherwise we would get gRPC error.
				return fmt.Errorf("CSI driver probe failed: %s", err)
			}
			if st.Code() != codes.DeadlineExceeded {
				return fmt.Errorf("CSI driver probe failed: %s", err)
			}
			// Timeout -> driver is not ready. Fall through to sleep() below.
			klog.Warning("CSI driver probe timed out")
		} else {
			if ready {
				return nil
			}
			klog.Warning("CSI driver is not ready")
		}
		// Timeout was returned or driver is not ready.
		time.Sleep(c.probeInterval)
	}
}

// probeOnce calls driver Probe() just once and returns its result without any processing.
func (c *clientImpl) probeOnce() (bool, error) {
	ctx, cancel := context.WithTimeout(context.TODO(), c.Timeout)
	defer cancel()

	identityClient := identity.NewIdentityClient(c.client)
	req := identity.ProbeRequest{}

	rsp, err := identityClient.Probe(ctx, &req)

	if err != nil {
		return false, err
	}

	r := rsp.GetReady()
	if r == nil {
		// "If not present, the caller SHALL assume that the plugin is in a ready state"
		return true, nil
	}
	return r.GetValue(), nil
}

// HasControllerService gets the driver name from the driver
func (c *clientImpl) HasControllerService() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	identityClient := identity.NewIdentityClient(c.client)

	req := identity.GetCapabilitiesRequest{}
	rsp, err := identityClient.GetCapabilities(ctx, &req)
	if err != nil {
		return false, err
	}

	for _, c := range rsp.GetCapabilities() {
		svc := c.GetService()
		if svc != nil && svc.GetType() == identity.Capability_Service_CONTROLLER_SERVICE {
			return true, nil
		}
	}

	return false, nil
}

// GetDriverName gets the driver name from the driver
func (c *clientImpl) GetDriverName() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	identityClient := identity.NewIdentityClient(c.client)

	req := identity.GetIdentityRequest{}
	rsp, err := identityClient.GetIdentity(ctx, &req)
	if err != nil {
		return "", err
	}

	name := rsp.GetName()
	if name == "" {
		return "", errors.New("driver name is empty")
	}

	return name, nil
}
