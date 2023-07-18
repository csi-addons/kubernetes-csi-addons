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
	"fmt"
	"net"
	"os"
	"path/filepath"
)

type MockServer struct {
	VolumeGroupServer
}

func CreateMockServer() (*MockServer, error) {
	// Start the mock server
	tmpdir, err := tempDir()
	if err != nil {
		return nil, err
	}
	controllerServer := MockControllerServer{}
	server := newMockServer(controllerServer)
	err = server.startOnAddress("unix", filepath.Join(tmpdir, "csi.sock"))
	if err != nil {
		return nil, err
	}

	return server, nil
}

func tempDir() (string, error) {
	dir, err := os.MkdirTemp("", "volume-group-operator-test-")
	if err != nil {
		return "", fmt.Errorf("not create temporary directory, err: %v", err)
	}
	return dir, nil
}

func newMockServer(vg MockControllerServer) *MockServer {
	return &MockServer{
		VolumeGroupServer: VolumeGroupServer{
			VolumeGroup: vg,
		},
	}
}

func (m *MockServer) startOnAddress(network, address string) error {
	listener, err := net.Listen(network, address)
	if err != nil {
		return err
	}

	if err := m.VolumeGroupServer.Start(listener); err != nil {
		listener.Close()
		return err
	}

	return nil
}
