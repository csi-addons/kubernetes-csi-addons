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
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestKubeletDiscoverer_FilesystemVolume(t *testing.T) {
	tmpDir := t.TempDir()
	kubeletRoot := filepath.Join(tmpDir, "kubelet")
	procDir := filepath.Join(tmpDir, "proc")
	sysDir := filepath.Join(tmpDir, "sys")

	driverName := "csi-powerstore.dellemc.com"
	pvName := "pvc-vol-001"
	pvDir := filepath.Join(kubeletRoot, "plugins", "kubernetes.io", "csi", driverName, pvName)
	globalMount := filepath.Join(pvDir, "globalmount")
	if err := os.MkdirAll(globalMount, 0755); err != nil {
		t.Fatal(err)
	}

	vd := volData{
		VolumeHandle:        "csi-vol-handle-001",
		DriverName:          driverName,
		SpecVolID:           pvName,
		VolumeLifecycleMode: "Persistent",
	}
	data, _ := json.Marshal(vd)
	if err := os.WriteFile(filepath.Join(pvDir, "vol_data.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(procDir, "1"), 0755); err != nil {
		t.Fatal(err)
	}
	mountinfoContent := fmt.Sprintf(
		"36 35 253:5 / %s rw,relatime shared:1 - ext4 /dev/dm-5 rw\n",
		globalMount,
	)
	if err := os.WriteFile(filepath.Join(procDir, "1", "mountinfo"), []byte(mountinfoContent), 0644); err != nil {
		t.Fatal(err)
	}

	devBlockDir := filepath.Join(sysDir, "dev", "block")
	if err := os.MkdirAll(devBlockDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../../devices/virtual/block/dm-5", filepath.Join(devBlockDir, "253:5")); err != nil {
		t.Fatal(err)
	}
	dmUUIDDir := filepath.Join(sysDir, "block", "dm-5", "dm")
	if err := os.MkdirAll(dmUUIDDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dmUUIDDir, "uuid"), []byte("mpath-abc\n"), 0644); err != nil {
		t.Fatal(err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	d := NewKubeletDiscoverer(kubeletRoot, kubeletRoot, procDir, sysDir, "worker-1", logger)

	results, err := d.Discover(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.VolumeHandle != "csi-vol-handle-001" {
		t.Errorf("expected volume_handle csi-vol-handle-001, got %s", r.VolumeHandle)
	}
	if r.Driver != "csi-powerstore.dellemc.com" {
		t.Errorf("expected driver csi-powerstore.dellemc.com, got %s", r.Driver)
	}
	if r.Device != "dm-5" {
		t.Errorf("expected device dm-5, got %s", r.Device)
	}
	if r.Node != "worker-1" {
		t.Errorf("expected node worker-1, got %s", r.Node)
	}
}

func TestKubeletDiscoverer_SkipsEphemeral(t *testing.T) {
	tmpDir := t.TempDir()
	kubeletRoot := filepath.Join(tmpDir, "kubelet")
	procDir := filepath.Join(tmpDir, "proc")
	sysDir := filepath.Join(tmpDir, "sys")

	pvDir := filepath.Join(kubeletRoot, "plugins", "kubernetes.io", "csi", "csi.example.com", "ephemeral-vol")
	if err := os.MkdirAll(filepath.Join(pvDir, "globalmount"), 0755); err != nil {
		t.Fatal(err)
	}

	vd := volData{
		VolumeHandle:        "pod-uid-123",
		DriverName:          "csi.example.com",
		SpecVolID:           "ephemeral-vol",
		VolumeLifecycleMode: "Ephemeral",
	}
	data, _ := json.Marshal(vd)
	if err := os.WriteFile(filepath.Join(pvDir, "vol_data.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(procDir, "1"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(procDir, "1", "mountinfo"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	d := NewKubeletDiscoverer(kubeletRoot, kubeletRoot, procDir, sysDir, "worker-1", logger)

	results, err := d.Discover(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for ephemeral volume, got %d", len(results))
	}
}

func TestKubeletDiscoverer_SkipsNFS(t *testing.T) {
	tmpDir := t.TempDir()
	kubeletRoot := filepath.Join(tmpDir, "kubelet")
	procDir := filepath.Join(tmpDir, "proc")
	sysDir := filepath.Join(tmpDir, "sys")

	pvDir := filepath.Join(kubeletRoot, "plugins", "kubernetes.io", "csi", "nfs.csi.k8s.io", "nfs-vol")
	globalMount := filepath.Join(pvDir, "globalmount")
	if err := os.MkdirAll(globalMount, 0755); err != nil {
		t.Fatal(err)
	}

	vd := volData{
		VolumeHandle: "nfs-handle",
		DriverName:   "nfs.csi.k8s.io",
		SpecVolID:    "nfs-vol",
	}
	data, _ := json.Marshal(vd)
	if err := os.WriteFile(filepath.Join(pvDir, "vol_data.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(procDir, "1"), 0755); err != nil {
		t.Fatal(err)
	}
	mountinfoContent := fmt.Sprintf(
		"36 35 0:0 / %s rw,relatime shared:1 - nfs4 server:/export rw\n",
		globalMount,
	)
	if err := os.WriteFile(filepath.Join(procDir, "1", "mountinfo"), []byte(mountinfoContent), 0644); err != nil {
		t.Fatal(err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	d := NewKubeletDiscoverer(kubeletRoot, kubeletRoot, procDir, sysDir, "worker-1", logger)

	results, err := d.Discover(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for NFS volume, got %d", len(results))
	}
}

func TestKubeletDiscoverer_SeparateContainerAndHostPaths(t *testing.T) {
	tmpDir := t.TempDir()

	containerRoot := filepath.Join(tmpDir, "container-kubelet")
	hostRoot := filepath.Join(tmpDir, "host-kubelet")

	procDir := filepath.Join(tmpDir, "proc")
	sysDir := filepath.Join(tmpDir, "sys")

	driverName := "csi.example.com"
	pvName := "pvc-split-test"

	pvDir := filepath.Join(containerRoot, "plugins", "kubernetes.io", "csi", driverName, pvName)
	if err := os.MkdirAll(filepath.Join(pvDir, "globalmount"), 0o755); err != nil {
		t.Fatal(err)
	}
	vd := volData{
		VolumeHandle:        "split-vol-handle",
		DriverName:          driverName,
		SpecVolID:           pvName,
		VolumeLifecycleMode: "Persistent",
	}
	data, _ := json.Marshal(vd)
	if err := os.WriteFile(filepath.Join(pvDir, "vol_data.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	hostGlobalMount := filepath.Join(hostRoot, "plugins", "kubernetes.io", "csi", driverName, pvName, "globalmount")
	if err := os.MkdirAll(filepath.Join(procDir, "1"), 0o755); err != nil {
		t.Fatal(err)
	}
	mountinfoContent := fmt.Sprintf(
		"36 35 8:16 / %s rw,relatime shared:1 - ext4 /dev/sdb rw\n",
		hostGlobalMount,
	)
	if err := os.WriteFile(filepath.Join(procDir, "1", "mountinfo"), []byte(mountinfoContent), 0o644); err != nil {
		t.Fatal(err)
	}

	devBlockDir := filepath.Join(sysDir, "dev", "block")
	if err := os.MkdirAll(devBlockDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../../block/sdb", filepath.Join(devBlockDir, "8:16")); err != nil {
		t.Fatal(err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	d := NewKubeletDiscoverer(containerRoot, hostRoot, procDir, sysDir, "node-split", logger)

	results, err := d.Discover(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result with separate container/host paths, got %d", len(results))
	}
	if results[0].Device != "sdb" {
		t.Errorf("expected device sdb, got %s", results[0].Device)
	}
	if results[0].Node != "node-split" {
		t.Errorf("expected node node-split, got %s", results[0].Node)
	}
}
