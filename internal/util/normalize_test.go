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

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeLeaseName(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{
			name: "some.csi.driver",
			want: "some-csi-driver",
		},
		{
			name: "some.csi/driver",
			want: "some-csi-driver",
		},
		{
			name: "some.csi.driver...",
			want: "some-csi-driver---X",
		},
		{
			name: "#some.csi.driver",
			want: "-some-csi-driver",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := NormalizeLeaseName(tt.name)
			assert.Equal(t, tt.want, name)
		})
	}
}
