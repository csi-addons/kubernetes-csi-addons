/*
Copyright 2023 The Ceph-CSI Authors.

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

package main

import (
	"testing"
)

func Test_networkFenceBase_Init(t *testing.T) {
	type fields struct {
		parameters      map[string]string
		secretName      string
		secretNamespace string
		cidrs           []string
	}
	type args struct {
		c *command
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// Test for empty clusterid
		{
			name: "No clusterid",
			fields: fields{
				parameters:      nil,
				secretName:      "",
				secretNamespace: "",
				cidrs:           nil,
			},
			args: args{
				c: &command{
					clusterid: "",
					secret:    "ns/ns-secret",
					cidrs:     "1.2.3.4/10,4.3.2.1/20",
				},
			},
			wantErr: true,
		},
		// Test for empty secrets
		{
			name: "No secret",
			fields: fields{
				parameters:      nil,
				secretName:      "",
				secretNamespace: "",
				cidrs:           nil,
			},
			args: args{
				c: &command{
					clusterid: "cluster-123",
					secret:    "",
					cidrs:     "1.2.3.4/10",
				},
			},
			wantErr: true,
		},
		// Test for partial secrets
		{
			name: "Partial secret",
			fields: fields{
				parameters:      nil,
				secretName:      "",
				secretNamespace: "",
				cidrs:           nil,
			},
			args: args{
				c: &command{
					clusterid: "cluster-123",
					secret:    "ns",
					cidrs:     "1.2.3.4/10",
				},
			},
			wantErr: true,
		},
		// Test for empty cidr
		{
			name: "No cidr",
			fields: fields{
				parameters:      nil,
				secretName:      "",
				secretNamespace: "",
				cidrs:           nil,
			},
			args: args{
				c: &command{
					clusterid: "cluster-123",
					secret:    "ns/ns-secret",
					cidrs:     "",
				},
			},
			wantErr: true,
		},
		// Test for single cidr
		{
			name: "Single cidr",
			fields: fields{
				parameters:      nil,
				secretName:      "",
				secretNamespace: "",
				cidrs:           nil,
			},
			args: args{
				c: &command{
					clusterid: "cluster-123",
					secret:    "ns/ns-secret",
					cidrs:     "1.2.3.4/10",
				},
			},
			wantErr: false,
		},
		// Test for multi cidrs
		{
			name: "Multi cidrs",
			fields: fields{
				parameters:      nil,
				secretName:      "",
				secretNamespace: "",
				cidrs:           nil,
			},
			args: args{
				c: &command{
					clusterid: "cluster-123",
					secret:    "ns/ns-secret",
					cidrs:     "1.2.3.4/10,4.3.2.1/20,5.6.7.8/10",
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns := &networkFenceBase{
				parameters:      tt.fields.parameters,
				secretName:      tt.fields.secretName,
				secretNamespace: tt.fields.secretNamespace,
				cidrs:           tt.fields.cidrs,
			}

			err := ns.Init(tt.args.c)
			if (err != nil) != tt.wantErr {
				t.Errorf("networkFenceBase.Init() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
