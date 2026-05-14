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

package discovery

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/moby/sys/mountinfo"
)

// ParseMountInfo reads and parses /proc/1/mountinfo from the given procfs path.
func ParseMountInfo(procPath string) ([]*mountinfo.Info, error) {
	f, err := os.Open(filepath.Join(procPath, "1", "mountinfo"))
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return mountinfo.GetMountsFromReader(f, nil)
}

// FindMountByMountpoint returns the mount entry matching the given mountpoint exactly.
func FindMountByMountpoint(mounts []*mountinfo.Info, mountpoint string) *mountinfo.Info {
	for _, m := range mounts {
		if m.Mountpoint == mountpoint {
			return m
		}
	}
	return nil
}

// FindMountUnder returns the first mount whose mountpoint is exactly dir or
// is a direct child of dir. This handles CSI drivers (e.g. Ceph RBD) that
// stage the block device under a subdirectory of the globalmount path, such
// as: .../globalmount/<volumeHandle>
func FindMountUnder(mounts []*mountinfo.Info, dir string) *mountinfo.Info {
	prefix := dir + "/"
	for _, m := range mounts {
		if m.Mountpoint == dir || (len(m.Mountpoint) > len(prefix) &&
			m.Mountpoint[:len(prefix)] == prefix &&
			!strings.Contains(m.Mountpoint[len(prefix):], "/")) {
			return m
		}
	}
	return nil
}

// IsNetworkFS returns true if the filesystem type is a network filesystem (NFS, CIFS, etc.).
func IsNetworkFS(fsType string) bool {
	switch fsType {
	case "nfs", "nfs4", "cifs", "smbfs", "glusterfs", "ceph", "lustre", "9p":
		return true
	}
	return false
}
