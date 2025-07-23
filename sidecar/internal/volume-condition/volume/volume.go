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

package volume

// CSIVolume describes an attached volume.
type CSIVolume interface {
	// GetDriver returns the name of the CSI-driver that is responsible
	// for this volume.
	GetDriver() string
	// GetVolumeID returns the ID (volume-handle) of the volume.
	GetVolumeID() string
}

type csiVolume struct {
	drivername string
	volumeID   string
}

// NewCSIVolume creates a new CSIVolume with the given drivername and volumeID.
func NewCSIVolume(drivername, volumeID string) CSIVolume {
	return &csiVolume{
		drivername: drivername,
		volumeID:   volumeID,
	}
}

func (vol *csiVolume) GetDriver() string {
	return vol.drivername
}

func (vol *csiVolume) GetVolumeID() string {
	return vol.volumeID
}
