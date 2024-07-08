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

package csiaddonsnode

import (
	"reflect"
	"testing"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/csiaddons/v1alpha1"
	"github.com/csi-addons/kubernetes-csi-addons/sidecar/internal/client"

	"google.golang.org/grpc"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// mockClient implements a fake interface for the Client type.
type mockClient struct {
	// Driver contains the drivername obtained with GetDriverName()
	driver string

	// isController is set to true when the CSI-plugin supports the
	// CONTROLLER_SERVICE
	isController bool
}

func NewMockClient(driver string) client.Client {
	return &mockClient{driver: driver}
}

func (mc *mockClient) GetGRPCClient() *grpc.ClientConn {
	return nil
}

func (mc *mockClient) Probe() error {
	return nil
}

func (mc *mockClient) GetDriverName() (string, error) {
	return mc.driver, nil
}

func (mc *mockClient) HasControllerService() (bool, error) {
	return mc.isController, nil
}

func Test_getCSIAddonsNode(t *testing.T) {
	var (
		podName     = "pod"
		podNamspace = "default"
		podUID      = "123"
	)

	type args struct {
		driverName string
		endpoint   string
		nodeID     string
	}
	tests := []struct {
		name    string
		init    func()
		args    args
		want    *csiaddonsv1alpha1.CSIAddonsNode
		wantErr bool
	}{
		{
			name: "Test 1",
			args: args{
				driverName: "example.com",
				endpoint:   "192.168.61.228:6060",
				nodeID:     "123",
			},
			want: &csiaddonsv1alpha1.CSIAddonsNode{
				TypeMeta: v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{
					Name:      podName,
					Namespace: podNamspace,
					OwnerReferences: []v1.OwnerReference{
						{
							APIVersion: "v1",
							Kind:       "Pod",
							Name:       podName,
							UID:        types.UID(podUID),
						},
					},
				},
				Spec: csiaddonsv1alpha1.CSIAddonsNodeSpec{
					Driver: csiaddonsv1alpha1.CSIAddonsNodeDriver{
						Name:     "example.com",
						EndPoint: "192.168.61.228:6060",
						NodeID:   "123",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Test 2",
			args: args{
				driverName: "csi.example.com",
				endpoint:   "[2001:0db8:3c4d:0015:0000:0000:1a2f:1a2b]:8080",
				nodeID:     "32532-1312-435354",
			},
			want: &csiaddonsv1alpha1.CSIAddonsNode{
				TypeMeta: v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{
					Name:      podName,
					Namespace: podNamspace,
					OwnerReferences: []v1.OwnerReference{
						{
							APIVersion: "v1",
							Kind:       "Pod",
							Name:       podName,
							UID:        types.UID(podUID),
						},
					},
				},
				Spec: csiaddonsv1alpha1.CSIAddonsNodeSpec{
					Driver: csiaddonsv1alpha1.CSIAddonsNodeDriver{
						Name:     "csi.example.com",
						EndPoint: "[2001:0db8:3c4d:0015:0000:0000:1a2f:1a2b]:8080",
						NodeID:   "32532-1312-435354",
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := &Manager{
				Client:       NewMockClient(tt.args.driverName),
				Node:         tt.args.nodeID,
				Endpoint:     tt.args.endpoint,
				PodName:      "pod",
				PodNamespace: "default",
				PodUID:       "123",
			}
			got, err := mgr.getCSIAddonsNode()
			if (err != nil) != tt.wantErr {
				t.Errorf("getCSIAddonsNode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getCSIAddonsNode() = %v, want %v", got, tt.want)
			}
		})
	}
}
