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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetErrorMessage(t *testing.T) {
	type args struct {
		err error
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "nil error",
			args: args{
				err: nil,
			},
			want: "",
		},
		{
			name: "grpc error",
			args: args{
				err: status.Error(codes.Aborted, "aborted"),
			},
			want: "aborted",
		},
		{
			name: "non-grpc error",
			args: args{
				err: errors.New("aborted"),
			},
			want: "aborted",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := GetErrorMessage(tt.args.err)
			assert.Equal(t, tt.want, res)
		})
	}
}
