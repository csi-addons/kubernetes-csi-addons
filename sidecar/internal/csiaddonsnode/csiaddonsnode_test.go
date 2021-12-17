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
	"fmt"
	"reflect"
	"testing"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/v1alpha1"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func Test_lookupEnv(t *testing.T) {

	t.Run("Env Var set", func(t *testing.T) {
		key := "hello"
		value := "world"
		t.Setenv(key, value)

		got, err := lookupEnv(key)
		if (err != nil) != false {
			t.Errorf("lookupEnv() error = %v, wantErr %v", err, false)
			return
		}
		if got != value {
			t.Errorf("lookupEnv() = %v, want %v", got, value)
		}
	})

	t.Run("Env Var not set", func(t *testing.T) {
		key := "hello"
		_, err := lookupEnv(key)
		if err == nil {
			t.Errorf("lookupEnv() error = %v, wantErr %v", err,
				fmt.Errorf("Required environemental variable %q not found", key))
		}
	})
}

func Test_getCSIAddonsNode(t *testing.T) {
	var (
		podName     = "pod"
		podNamspace = "default"
		podUID      = "123"
		podIP       = "110"
	)
	t.Setenv(podNameEnvKey, podName)
	t.Setenv(podNamespaceEnvKey, podNamspace)
	t.Setenv(podUIDEnvKey, podUID)
	t.Setenv(podIPEnvKey, podIP)

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
				endpoint:   "6060",
				nodeID:     "123",
			},
			want: &csiaddonsv1alpha1.CSIAddonsNode{
				TypeMeta: v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{
					Name:      podUID,
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
						EndPoint: podIP + ":" + "6060",
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
				endpoint:   "8060",
				nodeID:     "32532-1312-435354",
			},
			want: &csiaddonsv1alpha1.CSIAddonsNode{
				TypeMeta: v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{
					Name:      podUID,
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
						EndPoint: podIP + ":" + "8060",
						NodeID:   "32532-1312-435354",
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getCSIAddonsNode(tt.args.driverName, tt.args.endpoint, tt.args.nodeID)
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
