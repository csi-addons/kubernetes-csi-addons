package discovery

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestHPEDiscoverer_NoDirReturnsEmpty(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	d := NewHPEDiscoverer("/nonexistent", "/fake/sys", "node1", logger)

	results, err := d.Discover(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty results, got %d", len(results))
	}
}

func TestHPEDiscoverer_ParsesDeviceInfo(t *testing.T) {
	kubeletRoot := t.TempDir()
	sysPath := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	pvDir := filepath.Join(kubeletRoot, "plugins", "kubernetes.io", "csi", "pvc-hpe-001", "globalmount")
	if err := os.MkdirAll(pvDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := `{
  "volume_id": "hpe-vol-123",
  "device": {
    "alt_full_path_name": "/dev/dm-7"
  }
}`
	if err := os.WriteFile(filepath.Join(pvDir, "deviceInfo.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	setupSysForDM(t, sysPath, "dm-7", "mpath-hpe")

	d := NewHPEDiscoverer(kubeletRoot, sysPath, "node1", logger)
	results, err := d.Discover(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].VolumeHandle != "hpe-vol-123" {
		t.Errorf("expected volume handle hpe-vol-123, got %s", results[0].VolumeHandle)
	}
	if results[0].Device != "dm-7" {
		t.Errorf("expected device dm-7, got %s", results[0].Device)
	}
	if results[0].Driver != "csi.hpe.com" {
		t.Errorf("expected driver csi.hpe.com, got %s", results[0].Driver)
	}
}

func TestHPEDiscoverer_SkipsMissingFields(t *testing.T) {
	kubeletRoot := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	pvDir := filepath.Join(kubeletRoot, "plugins", "kubernetes.io", "csi", "pvc-empty", "globalmount")
	if err := os.MkdirAll(pvDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := `{"volume_id": "", "device": {"alt_full_path_name": ""}}`
	if err := os.WriteFile(filepath.Join(pvDir, "deviceInfo.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	d := NewHPEDiscoverer(kubeletRoot, "/fake/sys", "node1", logger)
	results, err := d.Discover(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}
