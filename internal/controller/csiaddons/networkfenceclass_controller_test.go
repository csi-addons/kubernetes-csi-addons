/*
Copyright 2024 The Kubernetes-CSI-Addons Authors.

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
)

func TestRemoveClassFromList(t *testing.T) {
	tests := []struct {
		name      string
		classes   []string
		className string
		expected  []string
	}{
		{
			name:      "Class exists in the middle",
			classes:   []string{"class1", "class2", "class3"},
			className: "class2",
			expected:  []string{"class1", "class3"},
		},
		{
			name:      "Class is the first element",
			classes:   []string{"class1", "class2", "class3"},
			className: "class1",
			expected:  []string{"class2", "class3"},
		},
		{
			name:      "Class is the last element",
			classes:   []string{"class1", "class2", "class3"},
			className: "class3",
			expected:  []string{"class1", "class2"},
		},
		{
			name:      "Class does not exist",
			classes:   []string{"class1", "class2", "class3"},
			className: "class4",
			expected:  []string{"class1", "class2", "class3"},
		},
		{
			name:      "Empty list",
			classes:   []string{},
			className: "class1",
			expected:  []string{},
		},
		{
			name:      "Removing the last class",
			classes:   []string{"class1"},
			className: "class1",
			expected:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeClassFromList(tt.classes, tt.className)
			assert.Equal(t, tt.expected, result, "classes should be equal")
		})
	}
}

func TestValidatePrefixedParameters(t *testing.T) {
	tests := []struct {
		name    string
		param   map[string]string
		wantErr bool
	}{
		{
			name: "valid parameters",
			param: map[string]string{
				prefixedNetworkFenceSecretNameKey:      "secret1",
				prefixedNetworkFenceSecretNamespaceKey: "namespace1",
			},
			wantErr: false,
		},
		{
			name: "empty secret name",
			param: map[string]string{
				prefixedNetworkFenceSecretNameKey:      "",
				prefixedNetworkFenceSecretNamespaceKey: "namespace1",
			},
			wantErr: true,
		},
		{
			name: "empty secret namespace",
			param: map[string]string{
				prefixedNetworkFenceSecretNameKey:      "secret1",
				prefixedNetworkFenceSecretNamespaceKey: "",
			},
			wantErr: true,
		},
		{
			name:    "unknown parameter key",
			param:   map[string]string{networkFenceParameterPrefix + "/unknownKey": "value"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePrefixedParameters(tt.param)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
