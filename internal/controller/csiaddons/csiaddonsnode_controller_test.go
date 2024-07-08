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
