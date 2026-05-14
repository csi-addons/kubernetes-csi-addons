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
	"testing"
)

const testMountInfo = `22 1 8:1 / / rw,relatime - ext4 /dev/sda1 rw
30 22 0:26 / /sys rw,nosuid,nodev,noexec,relatime - sysfs sysfs rw
50 22 253:5 / /var/lib/kubelet/plugins/kubernetes.io/csi/pvc-abc/globalmount rw,relatime - ext4 /dev/dm-5 rw
60 22 0:60 / /var/lib/kubelet/pods/pod-123/volumes/kubernetes.io~csi/pvc-def/mount rw - nfs4 10.0.0.1:/export rw
70 22 253:6 / /var/lib/kubelet/pods/pod-456/volumes/kubernetes.io~csi/pvc-ghi/mount rw,relatime - xfs /dev/dm-6 rw
`

func TestParseMountInfo(t *testing.T) {
	procPath := t.TempDir()
	pid1Dir := filepath.Join(procPath, "1")
	if err := os.MkdirAll(pid1Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pid1Dir, "mountinfo"), []byte(testMountInfo), 0o644); err != nil {
		t.Fatal(err)
	}

	mounts, err := ParseMountInfo(procPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mounts) == 0 {
		t.Fatal("expected mounts, got none")
	}
}

func TestFindMountByMountpoint(t *testing.T) {
	procPath := t.TempDir()
	pid1Dir := filepath.Join(procPath, "1")
	if err := os.MkdirAll(pid1Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pid1Dir, "mountinfo"), []byte(testMountInfo), 0o644); err != nil {
		t.Fatal(err)
	}

	mounts, err := ParseMountInfo(procPath)
	if err != nil {
		t.Fatal(err)
	}

	m := FindMountByMountpoint(mounts, "/var/lib/kubelet/plugins/kubernetes.io/csi/pvc-abc/globalmount")
	if m == nil {
		t.Fatal("expected to find mount")
	}
	if m.Major != 253 || m.Minor != 5 {
		t.Errorf("expected 253:5, got %d:%d", m.Major, m.Minor)
	}

	m2 := FindMountByMountpoint(mounts, "/nonexistent")
	if m2 != nil {
		t.Error("expected nil for non-existent mountpoint")
	}
}

func TestIsNetworkFS(t *testing.T) {
	tests := []struct {
		fsType string
		want   bool
	}{
		{"nfs", true},
		{"nfs4", true},
		{"cifs", true},
		{"ext4", false},
		{"xfs", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsNetworkFS(tt.fsType); got != tt.want {
			t.Errorf("IsNetworkFS(%q) = %v, want %v", tt.fsType, got, tt.want)
		}
	}
}
