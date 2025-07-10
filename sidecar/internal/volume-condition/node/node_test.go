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

package node

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestNewAttachedCSIVolume(t *testing.T) {
	cases := []struct {
		name        string
		volumeName  corev1.UniqueVolumeName
		csiDriver   string // CSIVolume.GetDriver()
		csiVolume   string // CSIVolume.GetVolumeID()
		expectError bool
	}{
		{
			name:        "happy path",
			volumeName:  "kubernetes.io/csi/ebs.csi.aws.com^vol-0c577c9c55718d592",
			csiDriver:   "ebs.csi.aws.com",
			csiVolume:   "vol-0c577c9c55718d592",
			expectError: false,
		},
		{
			name:        "not a csi driver",
			volumeName:  corev1.UniqueVolumeName("kubernetes.io/native/ebs.csi.aws.com^vol-0c577c9c55718d592"),
			csiDriver:   "ebs.csi.aws.com",
			csiVolume:   "vol-0c577c9c55718d592",
			expectError: true,
		},
		{
			name:        "repeated path component separator",
			volumeName:  corev1.UniqueVolumeName("kubernetes.io/csi^ebs.csi.aws.com^vol-0c577c9c55718d592"),
			csiDriver:   "ebs.csi.aws.com",
			csiVolume:   "vol-0c577c9c55718d592",
			expectError: true,
		},
		{
			name:        "incorrect UniqueVolumeName format",
			volumeName:  corev1.UniqueVolumeName("kubernetes.io/csi/ebs.csi.aws.com~vol-0c577c9c55718d592"),
			csiDriver:   "ebs.csi.aws.com",
			csiVolume:   "vol-0c577c9c55718d592",
			expectError: true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			volume, err := newAttachedCSIVolume(tt.volumeName)
			if tt.expectError {
				assert.NotNil(t, err)
				return
			} else {
				assert.Nil(t, err)
			}

			assert.Equal(t, tt.csiDriver, volume.GetDriver())
			assert.Equal(t, tt.csiVolume, volume.GetVolumeID())
		})
	}
}
