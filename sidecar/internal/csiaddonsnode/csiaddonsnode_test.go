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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/csiaddons/v1alpha1"
	"github.com/csi-addons/kubernetes-csi-addons/sidecar/internal/client"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
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
		name      = "pod"
		namespace = "default"
		uid       = "123"
	)

	type args struct {
		driverName string
		endpoint   string
		nodeID     string
		kind       string
	}

	tests := []struct {
		name    string
		args    args
		want    *csiaddonsv1alpha1.CSIAddonsNode
		wantErr bool
	}{
		{
			name: "Deployment Owner",
			args: args{
				driverName: "example.com",
				endpoint:   "192.168.61.228:6060",
				nodeID:     "123",
				kind:       "Deployment",
			},
			want:    createExpectedNode(namespace, uid, "example.com", "192.168.61.228:6060", "123", "Deployment", name),
			wantErr: false,
		},
		{
			name: "DaemonSet Owner",
			args: args{
				driverName: "example.com",
				endpoint:   "192.168.61.228:6060",
				nodeID:     "123",
				kind:       "DaemonSet",
			},
			want:    createExpectedNode(namespace, uid, "example.com", "192.168.61.228:6060", "123", "DaemonSet", name),
			wantErr: false,
		},
		{
			name: "IPv6 Endpoint",
			args: args{
				driverName: "csi.example.com",
				endpoint:   "[2001:0db8:3c4d:0015:0000:0000:1a2f:1a2b]:8080",
				nodeID:     "32532-1312-435354",
				kind:       "Deployment",
			},
			want:    createExpectedNode(namespace, uid, "csi.example.com", "[2001:0db8:3c4d:0015:0000:0000:1a2f:1a2b]:8080", "32532-1312-435354", "Deployment", name),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup manager with mock client and pod
			mgr, err := setupManager(tt.args.driverName, name, namespace, uid, tt.args.kind)
			if err != nil {
				t.Errorf("Failed to setup manager: %v", err)
			}
			mgr.Endpoint = tt.args.endpoint
			mgr.Node = tt.args.nodeID

			got, err := mgr.getCSIAddonsNode()
			if (err != nil) != tt.wantErr {
				t.Errorf("getCSIAddonsNode() error = %v, wantErr %v", err, tt.wantErr)
			}
			assert.Equal(t, tt.want, got, "getCSIAddonsNode() = %v, want %v", got, tt.want)
		})
	}
}

// setupManager initializes the Manager with the provided args.
func setupManager(driverName, name, namespace, uid, kind string) (*Manager, error) {
	mgr := &Manager{
		Client:       NewMockClient(driverName),
		KubeClient:   fake.NewSimpleClientset(),
		PodName:      name,
		PodNamespace: namespace,
		PodUID:       uid,
	}

	_, err := mgr.KubeClient.CoreV1().Pods(namespace).Create(context.TODO(), &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       types.UID(uid),
			OwnerReferences: []v1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       kind,
					Name:       name,
					UID:        types.UID(uid),
				},
			},
		},
	}, v1.CreateOptions{})

	return mgr, err
}

// createExpectedNode generates the expected CSIAddonsNode for the test.
func createExpectedNode(namespace, uid, driverName, endpoint, nodeID, ownerKind, ownerName string) *csiaddonsv1alpha1.CSIAddonsNode {
	// Generate the name dynamically based on the resource type
	name := fmt.Sprintf("%s-%s-%s-%s", nodeID, namespace, strings.ToLower(ownerKind), ownerName)
	return &csiaddonsv1alpha1.CSIAddonsNode{
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			OwnerReferences: []v1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       ownerKind,
					Name:       "pod",
					UID:        types.UID(uid),
				},
			},
		},
		Spec: csiaddonsv1alpha1.CSIAddonsNodeSpec{
			Driver: csiaddonsv1alpha1.CSIAddonsNodeDriver{
				Name:     driverName,
				EndPoint: endpoint,
				NodeID:   nodeID,
			},
		},
	}
}

func Test_generateName(t *testing.T) {
	type args struct {
		nodeID    string
		namespace string
		ownerKind string
		ownerName string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "Normal case",
			args: args{
				nodeID:    "example-node-id",
				namespace: "example-namespace",
				ownerKind: "Deployment",
				ownerName: "example-owner-name",
			},
		},
		{
			name: "Case with long names",
			args: args{
				nodeID:    "a-very-long-node-id-that-exceeds-the-limit-of-253-characters-and-should-trigger-uuid-generation-12345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890",
				namespace: "test-namespace",
				ownerKind: "Deployment",
				ownerName: "another-long-owner-name-that-is-definitely-more-than-253-characters-long-12345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890",
			},
		},
		{
			name: "Exact 253 characters",
			args: args{
				nodeID:    "a-very-long-node-id-that-is-exactly-128-characters-long-because-of-some-conditions-and-requirements-conditionsandrequirements",
				namespace: "test-namespace",
				ownerKind: "Deployment",
				ownerName: "another-long-owner-name-that-is-very-unique-and-specifies-exactly-how-the-naming-should-work-conditionss",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := generateExpectedLongName(tt.args.nodeID, tt.args.namespace, tt.args.ownerKind, tt.args.ownerName)
			got, err := generateName(tt.args.nodeID, tt.args.namespace, tt.args.ownerKind, tt.args.ownerName)
			if err != nil {
				t.Errorf("generateName() error = %v", err)
			}
			if got != want {
				t.Errorf("generateName() = %v, want %v", got, want)
			}
		})
	}
}

func generateExpectedLongName(nodeID, namespace, ownerKind, ownerName string) string {
	ownerKind = strings.ToLower(ownerKind)
	data := nodeID + namespace + ownerKind + ownerName
	hash := sha256.Sum256([]byte(data))
	uuid := hex.EncodeToString(hash[:8])
	base := fmt.Sprintf("%s-%s-%s-%s", nodeID, namespace, ownerKind, ownerName)
	if len(base) > 253 {
		return fmt.Sprintf("%s-%s", base[:251-len(uuid)-1], uuid)
	}
	return base
}
