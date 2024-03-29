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

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemoveFromSlice(t *testing.T) {
	type args struct {
		slice []string
		s     string
	}
	tests := []struct {
		name       string
		args       args
		wantResult []string
	}{
		{
			name: "present",
			args: args{
				slice: []string{"hello", "hi", "hey"},
				s:     "hello",
			},
			wantResult: []string{"hi", "hey"},
		},
		{
			name: "absent",
			args: args{
				slice: []string{"hello", "hi", "hey"},
				s:     "bye",
			},
			wantResult: []string{"hello", "hi", "hey"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := RemoveFromSlice(tt.args.slice, tt.args.s)
			assert.Equal(t, tt.wantResult, res)
		})
	}
}
