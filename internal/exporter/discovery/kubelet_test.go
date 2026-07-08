package discovery

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

// setupTestSysfs creates /sys/dev/block/{major}:{minor} -> target symlink
// and (if the device is dm-*) the dm/uuid file.
func setupTestSysfs(t *testing.T, sysDir, devMajMin, symlinkTarget, dmUUID string) {
	t.Helper()
	devBlockDir := filepath.Join(sysDir, "dev", "block")
	if err := os.MkdirAll(devBlockDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(symlinkTarget, filepath.Join(devBlockDir, devMajMin)); err != nil {
		t.Fatal(err)
	}
	if dmUUID != "" {
		device := filepath.Base(symlinkTarget)
		dmDir := filepath.Join(sysDir, "block", device, "dm")
		if err := os.MkdirAll(dmDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dmDir, "uuid"), []byte(dmUUID+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// createPodVolume creates a pod CSI volume directory with vol_data.json and a
// mount/ subdirectory. Returns the mount/ path.
func createPodVolume(t *testing.T, kubeletRoot, podUID, pvName string, vd volData) string {
	t.Helper()
	volDir := filepath.Join(kubeletRoot, "pods", podUID, "volumes", "kubernetes.io~csi", pvName)
	mountDir := filepath.Join(volDir, "mount")
	if err := os.MkdirAll(mountDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(vd)
	if err := os.WriteFile(filepath.Join(volDir, "vol_data.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	return mountDir
}

func TestKubeletDiscoverer_FilesystemVolume(t *testing.T) {
	tmpDir := t.TempDir()
	kubeletRoot := filepath.Join(tmpDir, "kubelet")
	sysDir := filepath.Join(tmpDir, "sys")

	vd := volData{
		VolumeHandle:        "csi-vol-handle-001",
		DriverName:          "csi-powerstore.dellemc.com",
		SpecVolID:           "pvc-vol-001",
		VolumeLifecycleMode: "Persistent",
	}
	mountDir := createPodVolume(t, kubeletRoot, "pod-uid-001", "pvc-vol-001", vd)

	// Create a fake block device file at the mount path to simulate stat()
	// returning a real device. We use a symlink trick: since we can't create
	// real block devices in tests, we test via the sysfs resolution directly.
	// The actual stat() in production uses unix.Stat which returns Dev from
	// the filesystem the mount is on. In tests we verify the resolution logic.
	_ = mountDir

	setupTestSysfs(t, sysDir, "253:5", "../../devices/virtual/block/dm-5", "mpath-abc")

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	d := NewKubeletDiscoverer(kubeletRoot, sysDir, "worker-1", logger)

	// Since we can't create real mounts in unit tests, test the resolve logic directly
	device := d.resolveAndFilter(253, 5)
	if device != "dm-5" {
		t.Errorf("expected device dm-5, got %s", device)
	}
}

func TestKubeletDiscoverer_SkipsEphemeral(t *testing.T) {
	tmpDir := t.TempDir()
	kubeletRoot := filepath.Join(tmpDir, "kubelet")
	sysDir := filepath.Join(tmpDir, "sys")

	vd := volData{
		VolumeHandle:        "pod-uid-123",
		DriverName:          "csi.example.com",
		SpecVolID:           "ephemeral-vol",
		VolumeLifecycleMode: "Ephemeral",
	}
	createPodVolume(t, kubeletRoot, "pod-uid-123", "ephemeral-vol", vd)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	d := NewKubeletDiscoverer(kubeletRoot, sysDir, "worker-1", logger)

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
	sysDir := filepath.Join(tmpDir, "sys")

	vd := volData{
		VolumeHandle:        "nfs-handle",
		DriverName:          "nfs.csi.k8s.io",
		SpecVolID:           "nfs-vol",
		VolumeLifecycleMode: "Persistent",
	}
	createPodVolume(t, kubeletRoot, "pod-uid-nfs", "nfs-vol", vd)

	// NFS: sysfs resolution for major=0 will fail (no symlink created)
	// This validates the natural filtering: no /sys/dev/block/0:N exists
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	d := NewKubeletDiscoverer(kubeletRoot, sysDir, "worker-1", logger)

	// resolveAndFilter with major=0 should return empty (natural NFS filter)
	device := d.resolveAndFilter(0, 2923)
	if device != "" {
		t.Errorf("expected empty device for NFS (major=0), got %s", device)
	}
}

func TestKubeletDiscoverer_ResolvesMultipath(t *testing.T) {
	tmpDir := t.TempDir()
	sysDir := filepath.Join(tmpDir, "sys")

	setupTestSysfs(t, sysDir, "253:5", "../../devices/virtual/block/dm-5", "mpath-360060160abc")

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	d := NewKubeletDiscoverer(filepath.Join(tmpDir, "kubelet"), sysDir, "worker-1", logger)

	device := d.resolveAndFilter(253, 5)
	if device != "dm-5" {
		t.Errorf("expected dm-5 (multipath), got %s", device)
	}
}

func TestKubeletDiscoverer_ResolvesLUKSOverMultipath(t *testing.T) {
	tmpDir := t.TempDir()
	sysDir := filepath.Join(tmpDir, "sys")

	// dm-7 is LUKS, its slave is dm-5 which is multipath
	setupTestSysfs(t, sysDir, "253:7", "../../devices/virtual/block/dm-7", "CRYPT-LUKS2-abc-luks")
	setupTestSysfs(t, sysDir, "253:5", "../../devices/virtual/block/dm-5", "mpath-360060160abc")

	// Create slaves directory for dm-7 pointing to dm-5
	slavesDir := filepath.Join(sysDir, "block", "dm-7", "slaves")
	if err := os.MkdirAll(slavesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Create a symlink (or just a dir entry) for the slave
	if err := os.Symlink(filepath.Join(sysDir, "block", "dm-5"), filepath.Join(slavesDir, "dm-5")); err != nil {
		t.Fatal(err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	d := NewKubeletDiscoverer(filepath.Join(tmpDir, "kubelet"), sysDir, "worker-1", logger)

	device := d.resolveAndFilter(253, 7)
	if device != "dm-5" {
		t.Errorf("expected dm-5 (multipath under LUKS), got %s", device)
	}
}

func TestKubeletDiscoverer_ResolvesNVMe(t *testing.T) {
	tmpDir := t.TempDir()
	sysDir := filepath.Join(tmpDir, "sys")

	setupTestSysfs(t, sysDir, "259:1", "../../devices/pci0000:00/0000:00:1f.0/nvme/nvme0/nvme0n1", "")

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	d := NewKubeletDiscoverer(filepath.Join(tmpDir, "kubelet"), sysDir, "worker-1", logger)

	device := d.resolveAndFilter(259, 1)
	if device != "nvme0n1" {
		t.Errorf("expected nvme0n1, got %s", device)
	}
}

func TestKubeletDiscoverer_GracefulWithUnresolvableMounts(t *testing.T) {
	tmpDir := t.TempDir()
	kubeletRoot := filepath.Join(tmpDir, "kubelet")
	sysDir := filepath.Join(tmpDir, "sys")

	vd := volData{
		VolumeHandle:        "shared-vol-handle",
		DriverName:          "csi-powerstore.dellemc.com",
		SpecVolID:           "pvc-shared",
		VolumeLifecycleMode: "Persistent",
	}

	// Same volume mounted in two different pods (RWX).
	// Neither will resolve because mount/ shares st_dev with parent (no real mount).
	createPodVolume(t, kubeletRoot, "pod-uid-aaa", "pvc-shared", vd)
	createPodVolume(t, kubeletRoot, "pod-uid-bbb", "pvc-shared", vd)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	d := NewKubeletDiscoverer(kubeletRoot, sysDir, "worker-1", logger)

	results, err := d.Discover(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for unresolvable mounts, got %d", len(results))
	}
}

func TestKubeletDiscoverer_MountPropagationGuard(t *testing.T) {
	tmpDir := t.TempDir()
	kubeletRoot := filepath.Join(tmpDir, "kubelet")
	sysDir := filepath.Join(tmpDir, "sys")

	vd := volData{
		VolumeHandle:        "vol-handle-prop",
		DriverName:          "csi-powerstore.dellemc.com",
		SpecVolID:           "pvc-prop",
		VolumeLifecycleMode: "Persistent",
	}
	createPodVolume(t, kubeletRoot, "pod-uid-prop", "pvc-prop", vd)

	// mount/ is a plain subdir on the same tmpfs as its parent, so st_dev will
	// match — simulating a failure of mountPropagation: HostToContainer.
	// The guard must prevent this from being reported as a valid device.
	setupTestSysfs(t, sysDir, "8:0", "../../devices/pci0000:00/block/sda", "")

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	d := NewKubeletDiscoverer(kubeletRoot, sysDir, "worker-1", logger)

	results, err := d.Discover(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results when mount propagation not working, got %d: %+v", len(results), results)
	}
}

func TestKubeletDiscoverer_EmptyPodsDir(t *testing.T) {
	tmpDir := t.TempDir()
	kubeletRoot := filepath.Join(tmpDir, "kubelet")
	sysDir := filepath.Join(tmpDir, "sys")

	// pods/ dir doesn't exist
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	d := NewKubeletDiscoverer(kubeletRoot, sysDir, "worker-1", logger)

	results, err := d.Discover(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for non-existent pods dir, got %d", len(results))
	}
}

func TestKubeletDiscoverer_BlockVolume_SkipsWhenStatFails(t *testing.T) {
	tmpDir := t.TempDir()
	kubeletRoot := filepath.Join(tmpDir, "kubelet")
	sysDir := filepath.Join(tmpDir, "sys")

	pvName := "pvc-block-001"
	podUID := "pod-uid-block-001"

	// Create the volumeDevices directory structure without a device file.
	// stat() on the missing file should cause the block volume to be skipped.
	blockDir := filepath.Join(kubeletRoot, "pods", podUID, "volumeDevices", "kubernetes.io~csi", pvName)
	if err := os.MkdirAll(blockDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create vol_data.json at the plugin staging path
	volDataDir := filepath.Join(kubeletRoot, "plugins", "kubernetes.io~csi", "volumeDevices", pvName, "data")
	if err := os.MkdirAll(volDataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	vd := volData{
		VolumeHandle:        "block-vol-handle-001",
		DriverName:          "csi-powerstore.dellemc.com",
		SpecVolID:           pvName,
		VolumeLifecycleMode: "Persistent",
	}
	data, _ := json.Marshal(vd)
	if err := os.WriteFile(filepath.Join(volDataDir, "vol_data.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	d := NewKubeletDiscoverer(kubeletRoot, sysDir, "worker-1", logger)

	results, err := d.Discover(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results when device file is missing, got %d", len(results))
	}
}

func TestKubeletDiscoverer_BlockVolume_SkipsRegularFile(t *testing.T) {
	tmpDir := t.TempDir()
	kubeletRoot := filepath.Join(tmpDir, "kubelet")
	sysDir := filepath.Join(tmpDir, "sys")

	pvName := "pvc-block-002"
	podUID := "pod-uid-block-002"

	// Create the volumeDevices directory structure with a regular file
	// (st_rdev will be 0 for regular files).
	blockDir := filepath.Join(kubeletRoot, "pods", podUID, "volumeDevices", "kubernetes.io~csi", pvName)
	if err := os.MkdirAll(blockDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(blockDir, pvName), []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create vol_data.json at the plugin staging path
	volDataDir := filepath.Join(kubeletRoot, "plugins", "kubernetes.io~csi", "volumeDevices", pvName, "data")
	if err := os.MkdirAll(volDataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	vd := volData{
		VolumeHandle:        "block-vol-handle-002",
		DriverName:          "csi-powerstore.dellemc.com",
		SpecVolID:           pvName,
		VolumeLifecycleMode: "Persistent",
	}
	data, _ := json.Marshal(vd)
	if err := os.WriteFile(filepath.Join(volDataDir, "vol_data.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	d := NewKubeletDiscoverer(kubeletRoot, sysDir, "worker-1", logger)

	results, err := d.Discover(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Regular files have st_rdev == 0, so the block volume should be skipped
	if len(results) != 0 {
		t.Errorf("expected 0 results for regular file (st_rdev=0), got %d", len(results))
	}
}

func TestKubeletDiscoverer_BlockVolume_SkipsMissingVolData(t *testing.T) {
	tmpDir := t.TempDir()
	kubeletRoot := filepath.Join(tmpDir, "kubelet")
	sysDir := filepath.Join(tmpDir, "sys")

	pvName := "pvc-block-003"
	podUID := "pod-uid-block-003"

	// Create the volumeDevices directory structure with a regular file
	// but NO vol_data.json at the staging path.
	blockDir := filepath.Join(kubeletRoot, "pods", podUID, "volumeDevices", "kubernetes.io~csi", pvName)
	if err := os.MkdirAll(blockDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(blockDir, pvName), []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	d := NewKubeletDiscoverer(kubeletRoot, sysDir, "worker-1", logger)

	results, err := d.Discover(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results when vol_data.json is missing, got %d", len(results))
	}
}
