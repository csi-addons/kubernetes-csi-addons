/*
Copyright 2022 The Kubernetes-CSI-Addons Authors.

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

package controller

import (
	"testing"

	"github.com/csi-addons/spec/lib/go/identity"

	"github.com/stretchr/testify/assert"
)

func TestParseEndpoint(t *testing.T) {
	// test empty namespace
	_, _, _, err := parseEndpoint("pod://pod-name:5678")
	assert.Error(t, err)

	// test empty namespace
	_, _, _, err = parseEndpoint("pod://pod-name.:5678")
	assert.Error(t, err)

	namespace, podname, port, err := parseEndpoint("pod://pod-name.csi-addons:5678")
	assert.NoError(t, err)
	assert.Equal(t, namespace, "csi-addons")
	assert.Equal(t, podname, "pod-name")
	assert.Equal(t, port, "5678")

	namespace, podname, port, err = parseEndpoint("pod://csi.pod.ns-cluster.local:5678")
	assert.NoError(t, err)
	assert.Equal(t, namespace, "local")
	assert.Equal(t, podname, "csi.pod.ns-cluster")
	assert.Equal(t, port, "5678")

	// test empty podname
	_, _, _, err = parseEndpoint("pod://.local:5678")
	assert.Error(t, err)

}

func TestParseCapabilities(t *testing.T) {
	tests := []struct {
		name     string
		caps     []*identity.Capability
		expected []string
	}{
		{
			name:     "Empty capabilities",
			caps:     []*identity.Capability{},
			expected: []string{},
		},
		{
			name: "Single capability",
			caps: []*identity.Capability{
				{
					Type: &identity.Capability_Service_{
						Service: &identity.Capability_Service{
							Type: identity.Capability_Service_NODE_SERVICE,
						},
					},
				},
			},
			expected: []string{"service.NODE_SERVICE"},
		},
		{
			name: "Multiple capabilities",
			caps: []*identity.Capability{
				{
					Type: &identity.Capability_Service_{
						Service: &identity.Capability_Service{
							Type: identity.Capability_Service_NODE_SERVICE,
						},
					},
				},
				{
					Type: &identity.Capability_ReclaimSpace_{
						ReclaimSpace: &identity.Capability_ReclaimSpace{
							Type: identity.Capability_ReclaimSpace_ONLINE,
						},
					},
				},
			},
			expected: []string{"service.NODE_SERVICE", "reclaim_space.ONLINE"},
		},
		{
			name: "Same capability with different types",
			caps: []*identity.Capability{
				{
					Type: &identity.Capability_ReclaimSpace_{
						ReclaimSpace: &identity.Capability_ReclaimSpace{
							Type: identity.Capability_ReclaimSpace_ONLINE,
						},
					},
				},
				{
					Type: &identity.Capability_ReclaimSpace_{
						ReclaimSpace: &identity.Capability_ReclaimSpace{
							Type: identity.Capability_ReclaimSpace_OFFLINE,
						},
					},
				},
			},
			expected: []string{"reclaim_space.ONLINE", "reclaim_space.OFFLINE"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCapabilities(tt.caps)
			assert.Equal(t, tt.expected, result)
		})
	}
}
func TestGetRetryCountFromReason(t *testing.T) {
	tests := []struct {
		name    string
		reason  string
		want    int
		wantErr bool
	}{
		{
			name:    "empty reason",
			reason:  "",
			want:    0,
			wantErr: false,
		},
		{
			name:    "valid reason",
			reason:  "retry: 2",
			want:    2,
			wantErr: false,
		},
		{
			name:    "valid with extra spaces",
			reason:  "retry:    5",
			want:    5,
			wantErr: false,
		},
		{
			name:    "valid with trailing spaces",
			reason:  "something:  10  ",
			want:    10,
			wantErr: false,
		},
		{
			name:    "no colon",
			reason:  "retry 3",
			want:    0,
			wantErr: true,
		},
		{
			name:    "non-integer value",
			reason:  "retry: abc",
			want:    0,
			wantErr: true,
		},
		{
			name:    "multiple colons",
			reason:  "prefix: 7:extra",
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getRetryCountFromReason(tt.reason)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
