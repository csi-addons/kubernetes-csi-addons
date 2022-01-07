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

import "testing"

func TestValidateControllerEndpoint(t *testing.T) {
	type args struct {
		rawIP   string
		rawPort string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "valid ipv4",
			args: args{
				rawIP:   "192.168.61.228",
				rawPort: "8080",
			},
			want:    "192.168.61.228:8080",
			wantErr: false,
		},
		{
			name: "valid ipv6",
			args: args{
				rawIP:   "2001:db8:3c4d:15:0:1:1a2f:1a2b",
				rawPort: "8080",
			},
			want:    "[2001:db8:3c4d:15:0:1:1a2f:1a2b]:8080",
			wantErr: false,
		},
		{
			name: "invalid ipv4",
			args: args{
				rawIP:   "192.168.61",
				rawPort: "8080",
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "invalid ipv6",
			args: args{
				rawIP:   "2001:db8:3c4d:15:0:1:1a2f",
				rawPort: "8080",
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "empty ip",
			args: args{
				rawIP:   "",
				rawPort: "8080",
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "invalid port",
			args: args{
				rawIP:   "2001:db8:3c4d:15:0:1:1a2f:1a2b",
				rawPort: "-123",
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateControllerEndpoint(tt.args.rawIP, tt.args.rawPort)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateControllerEndpoint() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ValidateControllerEndpoint() = %v, want %v", got, tt.want)
			}
		})
	}
}
