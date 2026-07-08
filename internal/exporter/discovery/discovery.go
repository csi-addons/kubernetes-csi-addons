// Package discovery implements CSI volume-to-device mapping discovery.
package discovery

import (
	"context"
	"errors"
)

// ErrMissingFields indicates a file was parsed but required fields were empty.
var ErrMissingFields = errors.New("missing required fields")

// VolumeDevice represents a discovered CSI volume mapped to a node block device.
type VolumeDevice struct {
	VolumeHandle string
	Driver       string
	Device       string
	Node         string
}

// Discoverer discovers CSI volume-to-device mappings on the local node.
type Discoverer interface {
	Name() string
	Discover(ctx context.Context) ([]VolumeDevice, error)
}
