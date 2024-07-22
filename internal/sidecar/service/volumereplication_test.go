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

package service

import (
	"testing"

	"github.com/csi-addons/kubernetes-csi-addons/internal/proto"
	csiReplication "github.com/csi-addons/spec/lib/go/replication"
)

func Test_setReplicationSource(t *testing.T) {
	type args struct {
		src *csiReplication.ReplicationSource
		req *proto.ReplicationSource
	}
	volID := "volumeID"
	groupID := "groupID"
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "set replication source when request is nil",
			args: args{
				src: &csiReplication.ReplicationSource{},
				req: nil,
			},
			wantErr: true,
		},
		{
			name: "set replication source when request is nil",
			args: args{
				src: nil,
				req: nil,
			},
			wantErr: true,
		},
		{
			name: "set replication source is nil but request is not nil",
			args: args{
				src: nil,
				req: &proto.ReplicationSource{
					Type: &proto.ReplicationSource_Volume{
						Volume: &proto.ReplicationSource_VolumeSource{
							VolumeId: volID,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "set replication source when volume is set",
			args: args{
				src: &csiReplication.ReplicationSource{},
				req: &proto.ReplicationSource{
					Type: &proto.ReplicationSource_Volume{
						Volume: &proto.ReplicationSource_VolumeSource{
							VolumeId: volID,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "set replication source when volume group is set",
			args: args{
				src: &csiReplication.ReplicationSource{},
				req: &proto.ReplicationSource{
					Type: &proto.ReplicationSource_VolumeGroup{
						VolumeGroup: &proto.ReplicationSource_VolumeGroupSource{
							VolumeGroupId: groupID,
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := setReplicationSource(&tt.args.src, tt.args.req); (err != nil) != tt.wantErr {
				t.Errorf("setReplicationSource() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.args.req.GetVolume() != nil {
				if tt.args.src.GetVolume().GetVolumeId() != volID {
					t.Errorf("setReplicationSource() got = %v volumeID, expected = %v volumeID", tt.args.req.GetVolume().GetVolumeId(), volID)
				}
			}
			if tt.args.req.GetVolumeGroup() != nil {
				if tt.args.src.GetVolumegroup().GetVolumeGroupId() != groupID {
					t.Errorf("setReplicationSource() got = %v groupID, expected = %v volumeID", tt.args.req.GetVolumeGroup().GetVolumeGroupId(), groupID)
				}
			}
		})
	}
}
