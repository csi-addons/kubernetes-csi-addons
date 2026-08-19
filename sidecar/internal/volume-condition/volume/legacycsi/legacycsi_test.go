/*
Copyright 2026 The Kubernetes-CSI-Addons Authors.

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

package legacycsi

import (
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/protobuf/encoding/protowire"
)

// encodeVolumeConditionMsg encodes a raw VolumeCondition proto message
// (field 1: abnormal bool, field 2: message string) as it appeared in
// NodeGetVolumeStatsResponse in CSI spec v1.12.0.
func encodeVolumeConditionMsg(abnormal bool, message string) []byte {
	var b []byte
	if abnormal {
		b = protowire.AppendTag(b, 1, protowire.VarintType)
		b = protowire.AppendVarint(b, 1)
	}
	if message != "" {
		b = protowire.AppendTag(b, 2, protowire.BytesType)
		b = protowire.AppendBytes(b, []byte(message))
	}
	return b
}

// encodeResponseWithCondition encodes a NodeGetVolumeStatsResponse containing
// a VolumeCondition at field 2.
func encodeResponseWithCondition(conditionBytes []byte) []byte {
	var b []byte
	b = protowire.AppendTag(b, 2, protowire.BytesType)
	b = protowire.AppendBytes(b, conditionBytes)
	return b
}

func TestParseVolumeConditionFromResponse(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		want    *VolumeCondition
		wantErr bool
	}{
		{
			name: "empty data returns nil",
			data: nil,
			want: nil,
		},
		{
			name: "response without field 2 returns nil",
			data: func() []byte {
				b := protowire.AppendTag(nil, 1, protowire.VarintType)
				return protowire.AppendVarint(b, 42)
			}(),
			want: nil,
		},
		{
			name: "response with empty volume condition",
			data: encodeResponseWithCondition(encodeVolumeConditionMsg(false, "")),
			want: &VolumeCondition{Abnormal: false, Message: ""},
		},
		{
			name: "abnormal condition with message",
			data: encodeResponseWithCondition(encodeVolumeConditionMsg(true, "disk failure")),
			want: &VolumeCondition{Abnormal: true, Message: "disk failure"},
		},
		{
			name: "healthy condition with message",
			data: encodeResponseWithCondition(encodeVolumeConditionMsg(false, "volume is online")),
			want: &VolumeCondition{Abnormal: false, Message: "volume is online"},
		},
		{
			name: "unknown fields before field 2 are skipped",
			data: func() []byte {
				b := protowire.AppendTag(nil, 5, protowire.VarintType)
				b = protowire.AppendVarint(b, 999)
				b = protowire.AppendTag(b, 2, protowire.BytesType)
				b = protowire.AppendBytes(b, encodeVolumeConditionMsg(true, "io error"))
				return b
			}(),
			want: &VolumeCondition{Abnormal: true, Message: "io error"},
		},
		{
			name:    "malformed protobuf returns error",
			data:    []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseVolumeConditionFromResponse(tt.data)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseVolumeConditionFromResponse() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.want == nil {
				if got != nil {
					t.Errorf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected %+v, got nil", tt.want)
			}
			if got.Abnormal != tt.want.Abnormal || got.Message != tt.want.Message {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseVolumeConditionMessage(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		want    VolumeCondition
		wantErr bool
	}{
		{
			name: "empty data returns default condition",
			data: nil,
			want: VolumeCondition{Abnormal: false, Message: ""},
		},
		{
			name: "only abnormal field set to true",
			data: func() []byte {
				b := protowire.AppendTag(nil, 1, protowire.VarintType)
				return protowire.AppendVarint(b, 1)
			}(),
			want: VolumeCondition{Abnormal: true, Message: ""},
		},
		{
			name: "only message field set",
			data: func() []byte {
				b := protowire.AppendTag(nil, 2, protowire.BytesType)
				return protowire.AppendBytes(b, []byte("some error"))
			}(),
			want: VolumeCondition{Abnormal: false, Message: "some error"},
		},
		{
			name: "both abnormal and message set",
			data: encodeVolumeConditionMsg(true, "disk error"),
			want: VolumeCondition{Abnormal: true, Message: "disk error"},
		},
		{
			name: "unknown fields are skipped",
			data: func() []byte {
				b := protowire.AppendTag(nil, 3, protowire.VarintType)
				b = protowire.AppendVarint(b, 999)
				b = protowire.AppendTag(b, 1, protowire.VarintType)
				b = protowire.AppendVarint(b, 1)
				return b
			}(),
			want: VolumeCondition{Abnormal: true, Message: ""},
		},
		{
			name:    "malformed protobuf returns error",
			data:    []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseVolumeConditionMessage(tt.data)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseVolumeConditionMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got.Abnormal != tt.want.Abnormal || got.Message != tt.want.Message {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestRawCodecName(t *testing.T) {
	codec := rawCodec{}
	if codec.Name() != "proto" {
		t.Errorf("expected Name() == %q, got %q", "proto", codec.Name())
	}
}

func TestRawCodecMarshal(t *testing.T) {
	codec := rawCodec{}

	t.Run("marshals a proto message", func(t *testing.T) {
		req := &csi.NodeGetVolumeStatsRequest{
			VolumeId:   "test-volume",
			VolumePath: "/mnt/vol",
		}
		data, err := codec.Marshal(req)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}
		if len(data) == 0 {
			t.Error("expected non-empty marshaled data")
		}
	})

	t.Run("returns error for non-proto value", func(t *testing.T) {
		_, err := codec.Marshal("not a proto message")
		if err == nil {
			t.Error("expected error for non-proto value, got nil")
		}
	})
}

func TestRawCodecUnmarshal(t *testing.T) {
	codec := rawCodec{}

	req := &csi.NodeGetVolumeStatsRequest{VolumeId: "vol-123"}
	data, err := codec.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal setup: %v", err)
	}

	t.Run("unmarshals into rawBytesHolder", func(t *testing.T) {
		var raw rawBytesHolder
		if err := codec.Unmarshal(data, &raw); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}
		if len(raw) != len(data) {
			t.Errorf("got %d bytes, want %d", len(raw), len(data))
		}
		// rawBytesHolder must be a copy, not a slice of the original
		data[0] ^= 0xFF
		if raw[0] == data[0] {
			t.Error("rawBytesHolder shares backing array with input; expected a copy")
		}
	})

	t.Run("unmarshals into proto message", func(t *testing.T) {
		data, _ = codec.Marshal(req)
		got := &csi.NodeGetVolumeStatsRequest{}
		if err := codec.Unmarshal(data, got); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}
		if got.VolumeId != "vol-123" {
			t.Errorf("VolumeId = %q, want %q", got.VolumeId, "vol-123")
		}
	})

	t.Run("returns error for unsupported type", func(t *testing.T) {
		var s string
		if err := codec.Unmarshal(data, &s); err == nil {
			t.Error("expected error for *string, got nil")
		}
	})
}
