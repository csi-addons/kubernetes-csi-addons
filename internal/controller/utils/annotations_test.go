/*
Copyright 2025 The Kubernetes-CSI-Addons Authors.

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

package utils

import "testing"

func TestAnnotationValueChanged(t *testing.T) {
	tests := []struct {
		name           string
		oldAnnotations map[string]string
		newAnnotations map[string]string
		keys           []string
		expected       bool
	}{
		{
			name:           "No changes",
			oldAnnotations: map[string]string{"key1": "value1", "key2": "value2"},
			newAnnotations: map[string]string{"key1": "value1", "key2": "value2"},
			keys:           []string{"key1", "key2"},
			expected:       false,
		},
		{
			name:           "Value changed",
			oldAnnotations: map[string]string{"key1": "value1", "key2": "value2"},
			newAnnotations: map[string]string{"key1": "value1", "key2": "newvalue2"},
			keys:           []string{"key1", "key2"},
			expected:       true,
		},
		{
			name:           "Key added",
			oldAnnotations: map[string]string{"key1": "value1"},
			newAnnotations: map[string]string{"key1": "value1", "key2": "value2"},
			keys:           []string{"key1", "key2"},
			expected:       true,
		},
		{
			name:           "Key removed",
			oldAnnotations: map[string]string{"key1": "value1", "key2": "value2"},
			newAnnotations: map[string]string{"key1": "value1"},
			keys:           []string{"key1", "key2"},
			expected:       true,
		},
		{
			name:           "Change in non-specified key",
			oldAnnotations: map[string]string{"key1": "value1", "key2": "value2", "key3": "value3"},
			newAnnotations: map[string]string{"key1": "value1", "key2": "value2", "key3": "newvalue3"},
			keys:           []string{"key1", "key2"},
			expected:       false,
		},
		{
			name:           "Empty keys slice",
			oldAnnotations: map[string]string{"key1": "value1"},
			newAnnotations: map[string]string{"key1": "newvalue1"},
			keys:           []string{},
			expected:       false,
		},
		{
			name:           "Nil maps",
			oldAnnotations: nil,
			newAnnotations: nil,
			keys:           []string{"key1"},
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AnnotationValueChanged(tt.oldAnnotations, tt.newAnnotations, tt.keys)
			if result != tt.expected {
				t.Errorf("AnnotationValueChanged() = %v, want %v", result, tt.expected)
			}
		})
	}
}
