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
	"errors"
	"testing"

	"github.com/csi-addons/spec/lib/go/identity"

	"github.com/stretchr/testify/assert"
)

func TestParseEndpoint(t *testing.T) {
	_, _, _, err := parseEndpoint("1.2.3.4:5678")
	assert.True(t, errors.Is(err, errLegacyEndpoint))

	namespace, podname, port, err := parseEndpoint("pod://pod-name:5678")
	assert.NoError(t, err)
	assert.Equal(t, namespace, "")
	assert.Equal(t, podname, "pod-name")
	assert.Equal(t, port, "5678")

	namespace, podname, port, err = parseEndpoint("pod://pod-name.csi-addons:5678")
	assert.NoError(t, err)
	assert.Equal(t, namespace, "csi-addons")
	assert.Equal(t, podname, "pod-name")
	assert.Equal(t, port, "5678")

	_, _, _, err = parseEndpoint("pod://pod.ns.cluster.local:5678")
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
