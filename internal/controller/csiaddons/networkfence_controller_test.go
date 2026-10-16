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

package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/csiaddons/v1alpha1"
)

func TestGetNetworkFenceRequestDoesNotMutateSpecParameters(t *testing.T) {
	instanceParams := map[string]string{
		"instanceKey": "instanceVal",
	}
	nf := &NetworkFenceInstance{
		instance: &csiaddonsv1alpha1.NetworkFence{
			Spec: csiaddonsv1alpha1.NetworkFenceSpec{
				Parameters: instanceParams,
				Cidrs:      []string{"10.0.0.0/24"},
			},
		},
		nfClass: &csiaddonsv1alpha1.NetworkFenceClass{
			Spec: csiaddonsv1alpha1.NetworkFenceClassSpec{
				Parameters: map[string]string{
					prefixedNetworkFenceSecretNameKey:      "fence-secret",
					prefixedNetworkFenceSecretNamespaceKey: "rook-ceph",
					"clusterID":                            "rook-ceph",
				},
			},
		},
	}

	req := nf.getNetworkFenceRequest()
	assert.NotNil(t, req)
	assert.Equal(t, "fence-secret", req.SecretName)
	assert.Equal(t, "rook-ceph", req.SecretNamespace)
	assert.Equal(t, map[string]string{
		"instanceKey": "instanceVal",
		"clusterID":   "rook-ceph",
	}, req.Parameters)

	// Merging NetworkFenceClass parameters must not mutate NetworkFence.Spec.Parameters in place.
	assert.Equal(t, map[string]string{
		"instanceKey": "instanceVal",
	}, nf.instance.Spec.Parameters)
}
