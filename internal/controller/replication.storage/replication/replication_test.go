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

package replication

import (
	"errors"
	"testing"

	"github.com/csi-addons/kubernetes-csi-addons/internal/proto"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetDestinationInfoGetID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		volumeID    string
		groupID     string
		expectError bool
		expectedID  string
	}{
		{
			name:       "volume ID only",
			volumeID:   "vol-001",
			expectedID: "vol-001",
		},
		{
			name:       "group ID only",
			groupID:    "group-001",
			expectedID: "group-001",
		},
		{
			name:        "both IDs set",
			volumeID:    "vol-001",
			groupID:     "group-001",
			expectError: true,
		},
		{
			name:        "neither ID set",
			expectError: true,
		},
	}
	for _, tt := range tests {
		newtt := tt
		t.Run(newtt.name, func(t *testing.T) {
			t.Parallel()
			r := &Replication{
				Params: CommonRequestParameters{
					VolumeID: newtt.volumeID,
					GroupID:  newtt.groupID,
				},
			}
			id, err := r.getID()
			if newtt.expectError {
				if err == nil {
					t.Errorf("expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if id != newtt.expectedID {
					t.Errorf("getID() = %v, want %v", id, newtt.expectedID)
				}
			}
		})
	}
}

func TestGetDestinationInfoResponse(t *testing.T) {
	t.Parallel()
	// Test parsing volume destination response
	resp := &proto.GetReplicationDestinationInfoResponse{
		ReplicationDestination: &proto.ReplicationDestination{
			Type: &proto.ReplicationDestination_Volume{
				Volume: &proto.ReplicationDestination_VolumeDestination{
					VolumeId: "dest-vol-001",
				},
			},
		},
	}
	vol := resp.GetReplicationDestination().GetVolume()
	if vol == nil {
		t.Fatal("expected volume destination, got nil")
	}
	if vol.GetVolumeId() != "dest-vol-001" {
		t.Errorf("GetVolumeId() = %v, want dest-vol-001", vol.GetVolumeId())
	}

	// Test parsing volume group destination response
	groupResp := &proto.GetReplicationDestinationInfoResponse{
		ReplicationDestination: &proto.ReplicationDestination{
			Type: &proto.ReplicationDestination_Volumegroup{
				Volumegroup: &proto.ReplicationDestination_VolumeGroupDestination{
					VolumeGroupId: "dest-group-001",
					VolumeIds: map[string]string{
						"src-vol-001": "dest-vol-001",
						"src-vol-002": "dest-vol-002",
					},
				},
			},
		},
	}
	vgDest := groupResp.GetReplicationDestination().GetVolumegroup()
	if vgDest == nil {
		t.Fatal("expected volume group destination, got nil")
	}
	if vgDest.GetVolumeGroupId() != "dest-group-001" {
		t.Errorf("GetVolumeGroupId() = %v, want dest-group-001", vgDest.GetVolumeGroupId())
	}
	ids := vgDest.GetVolumeIds()
	if ids["src-vol-001"] != "dest-vol-001" {
		t.Errorf("VolumeIds[src-vol-001] = %v, want dest-vol-001", ids["src-vol-001"])
	}
	if ids["src-vol-002"] != "dest-vol-002" {
		t.Errorf("VolumeIds[src-vol-002] = %v, want dest-vol-002", ids["src-vol-002"])
	}

	// Test nil safety
	nilResp := &proto.GetReplicationDestinationInfoResponse{}
	if nilResp.GetReplicationDestination().GetVolume() != nil {
		t.Error("expected nil volume on empty response")
	}
	if nilResp.GetReplicationDestination().GetVolumegroup() != nil {
		t.Error("expected nil volumegroup on empty response")
	}
}

func TestHasKnownGRPCErrorOnDestinationInfo(t *testing.T) {
	t.Parallel()
	// Test that UNAVAILABLE is detected as a known error
	resp := &Response{
		Error: status.Error(codes.Unavailable, "destination not ready"),
	}
	if !resp.HasKnownGRPCError([]codes.Code{codes.Unavailable}) {
		t.Error("expected UNAVAILABLE to be a known error")
	}
	if resp.HasKnownGRPCError([]codes.Code{codes.NotFound}) {
		t.Error("expected NotFound NOT to match UNAVAILABLE error")
	}

	// Test nil error
	nilResp := &Response{}
	if nilResp.HasKnownGRPCError([]codes.Code{codes.Unavailable}) {
		t.Error("expected nil error not to be a known error")
	}
}

func TestGetMessageFromError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "test GRPC error message",
			err:  status.Error(codes.Internal, "failure"),
			want: "failure",
		},
		{
			name: "test non grpc error message",
			err:  errors.New("non grpc failure"),
			want: "non grpc failure",
		},
		{
			name: "test nil error",
			err:  nil,
			want: "",
		},
	}
	for _, tt := range tests {
		newtt := tt
		t.Run(newtt.name, func(t *testing.T) {
			t.Parallel()
			if got := GetMessageFromError(newtt.err); got != newtt.want {
				t.Errorf("GetMessageFromError() = %v, want %v", got, newtt.want)
			}
		})
	}
}
