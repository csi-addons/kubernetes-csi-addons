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

// Package legacycsi provides a fallback for calling NodeGetVolumeStats and
// reading the VolumeCondition from the response, as defined in CSI spec
// v1.12.0 and earlier. In CSI spec v1.13.0, health reporting was moved to the
// dedicated NodeGetVolumeHealth RPC, and VolumeCondition was removed from the
// NodeGetVolumeStats response.
//
// Because the new CSI spec no longer defines VolumeCondition as part of
// NodeGetVolumeStatsResponse, this package uses raw protobuf wire-format
// parsing to extract the field without requiring the old generated types.
package legacycsi

import (
	"context"
	"fmt"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

// nodeGetVolumeStatsMethod is the full gRPC method path for NodeGetVolumeStats
// in the CSI v1 Node service.
const nodeGetVolumeStatsMethod = "/csi.v1.Node/NodeGetVolumeStats"

// VolumeCondition represents the volume health condition from CSI spec
// v1.12.0 and earlier, where it was part of the NodeGetVolumeStats response.
type VolumeCondition struct {
	Abnormal bool
	Message  string
}

// rawBytesHolder captures the raw protobuf bytes of a gRPC response without
// proto deserialization, so they can be parsed manually.
type rawBytesHolder []byte

// rawCodec is an encoding.Codec that marshals requests using proto and stores
// responses as raw bytes into a *rawBytesHolder, bypassing proto unmarshaling.
type rawCodec struct{}

var _ encoding.Codec = rawCodec{}

func (rawCodec) Name() string { return "proto" }

func (rawCodec) Marshal(v any) ([]byte, error) {
	msg, ok := v.(proto.Message)
	if !ok {
		return nil, fmt.Errorf("legacycsi: Marshal: expected proto.Message, got %T", v)
	}
	return proto.Marshal(msg)
}

func (rawCodec) Unmarshal(data []byte, v any) error {
	if raw, ok := v.(*rawBytesHolder); ok {
		*raw = make(rawBytesHolder, len(data))
		copy(*raw, data)
		return nil
	}
	msg, ok := v.(proto.Message)
	if !ok {
		return fmt.Errorf("legacycsi: Unmarshal: expected proto.Message or *rawBytesHolder, got %T", v)
	}
	return proto.Unmarshal(data, msg)
}

// GetVolumeCondition calls NodeGetVolumeStats via the CSI v1 gRPC interface and
// returns the VolumeCondition from the response using CSI spec v1.12.0 wire
// format, where VolumeCondition was field 2 of NodeGetVolumeStatsResponse.
//
// Returns nil if the response does not contain a VolumeCondition field.
func GetVolumeCondition(ctx context.Context, cc grpc.ClientConnInterface, req *csi.NodeGetVolumeStatsRequest) (*VolumeCondition, error) {
	var raw rawBytesHolder
	err := cc.Invoke(ctx, nodeGetVolumeStatsMethod, req, &raw, grpc.ForceCodec(rawCodec{}))
	if err != nil {
		return nil, fmt.Errorf("NodeGetVolumeStats: %w", err)
	}

	vc, err := parseVolumeConditionFromResponse(raw)
	if err != nil {
		return nil, fmt.Errorf("parsing VolumeCondition from NodeGetVolumeStats response: %w", err)
	}
	return vc, nil
}

// parseVolumeConditionFromResponse reads a raw NodeGetVolumeStatsResponse and
// extracts the VolumeCondition message (field number 2, bytes-type).
// Returns nil if field 2 is absent.
func parseVolumeConditionFromResponse(data []byte) (*VolumeCondition, error) {
	b := data
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return nil, protowire.ParseError(n)
		}
		b = b[n:]

		if num == 2 && typ == protowire.BytesType {
			// Field 2 is volume_condition in the old CSI spec.
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return nil, protowire.ParseError(n)
			}
			return parseVolumeConditionMessage(v)
		}

		// Skip fields we don't care about.
		n = protowire.ConsumeFieldValue(num, typ, b)
		if n < 0 {
			return nil, protowire.ParseError(n)
		}
		b = b[n:]
	}
	return nil, nil
}

// parseVolumeConditionMessage parses a raw VolumeCondition proto message.
// Field 1 (varint): abnormal bool
// Field 2 (bytes):  message string
func parseVolumeConditionMessage(data []byte) (*VolumeCondition, error) {
	vc := &VolumeCondition{}
	b := data
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return nil, protowire.ParseError(n)
		}
		b = b[n:]

		switch {
		case num == 1 && typ == protowire.VarintType:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return nil, protowire.ParseError(n)
			}
			vc.Abnormal = v != 0
			b = b[n:]
		case num == 2 && typ == protowire.BytesType:
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return nil, protowire.ParseError(n)
			}
			vc.Message = string(v)
			b = b[n:]
		default:
			n = protowire.ConsumeFieldValue(num, typ, b)
			if n < 0 {
				return nil, protowire.ParseError(n)
			}
			b = b[n:]
		}
	}
	return vc, nil
}
