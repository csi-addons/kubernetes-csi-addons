/*
Copyright 2023 The Kubernetes-CSI-Addons Authors.

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

package mock_grpc_server

import (
	"context"
	"net"
	"sync"

	ctrl "sigs.k8s.io/controller-runtime"

	csi "github.com/csi-addons/spec/lib/go/volumegroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type VolumeGroupServer struct {
	listener    net.Listener
	server      *grpc.Server
	VolumeGroup MockControllerServer
	wg          sync.WaitGroup
	running     bool
	lock        sync.Mutex
}

func (c *VolumeGroupServer) Address() string {
	return c.listener.Addr().String()
}
func (c *VolumeGroupServer) Start(listener net.Listener) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.listener = listener

	server, err := c.createNewGRPCServer()
	if err != nil {
		return err
	}
	c.server = server

	csi.RegisterControllerServer(c.server, c.VolumeGroup)
	reflection.Register(c.server)

	waitForServer := make(chan bool)
	c.goServe(waitForServer)
	<-waitForServer
	c.running = true
	return nil
}

func (c *VolumeGroupServer) createNewGRPCServer() (*grpc.Server, error) {
	log := ctrl.Log.WithName("GRPC").WithName("VolumeGroup")
	logErr := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		resp, err := handler(ctx, req)
		if err != nil {
			log.Error(err, "GRPC error")
		}
		return resp, err
	}
	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(logErr),
	}
	return grpc.NewServer(opts...), nil
}

func (c *VolumeGroupServer) goServe(started chan<- bool) {
	goServe(c.server, &c.wg, c.listener, started)
}

func goServe(server *grpc.Server, wg *sync.WaitGroup, listener net.Listener, started chan<- bool) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		started <- true
		err := server.Serve(listener)
		if err != nil {
			panic(err.Error())
		}
	}()
}

func (c *VolumeGroupServer) Stop() {
	stop(&c.lock, &c.wg, c.server, c.running)
}

func stop(lock *sync.Mutex, wg *sync.WaitGroup, server *grpc.Server, running bool) {
	lock.Lock()
	defer lock.Unlock()

	if !running {
		return
	}

	server.Stop()
	wg.Wait()
}
