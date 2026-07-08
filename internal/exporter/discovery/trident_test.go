package discovery

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestTridentDiscoverer_DisabledWhenDirMissing(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	d := NewTridentDiscoverer("/nonexistent", "/fake/sys", "node1", logger)

	if !d.disabled {
		t.Fatal("expected discoverer to be disabled when dir is missing")
	}

	results, err := d.Discover(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty results, got %d", len(results))
	}
}

func TestTridentDiscoverer_ParsesTrackingFile(t *testing.T) {
	trackingDir := t.TempDir()
	sysPath := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	content := `{
  "volumePublishInfo": {
    "devicePath": "/dev/dm-3"
  },
  "stagingTargetPath": "/var/lib/kubelet/plugins/kubernetes.io/csi/pvc-123/globalmount"
}`
	if err := os.WriteFile(filepath.Join(trackingDir, "vol-001.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	setupSysForDM(t, sysPath, "dm-3", "mpath-xyz")

	d := NewTridentDiscoverer(trackingDir, sysPath, "node1", logger)
	results, err := d.Discover(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].VolumeHandle != "vol-001" {
		t.Errorf("expected volume handle vol-001, got %s", results[0].VolumeHandle)
	}
	if results[0].Device != "dm-3" {
		t.Errorf("expected device dm-3, got %s", results[0].Device)
	}
	if results[0].Driver != "csi.trident.netapp.io" {
		t.Errorf("expected driver csi.trident.netapp.io, got %s", results[0].Driver)
	}
}

func TestTridentDiscoverer_SkipsEmptyDevicePath(t *testing.T) {
	trackingDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	content := `{"volumePublishInfo": {}, "stagingTargetPath": "/mnt"}`
	if err := os.WriteFile(filepath.Join(trackingDir, "vol-empty.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	d := NewTridentDiscoverer(trackingDir, "/fake/sys", "node1", logger)
	results, err := d.Discover(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty device, got %d", len(results))
	}
}

func TestTridentDiscoverer_ContextCancelled(t *testing.T) {
	trackingDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	for i := 0; i < 5; i++ {
		content := `{"volumePublishInfo": {"devicePath": "/dev/sda"}, "stagingTargetPath": "/mnt"}`
		name := filepath.Join(trackingDir, "vol-"+string(rune('a'+i))+".json")
		if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	d := NewTridentDiscoverer(trackingDir, "/fake/sys", "node1", logger)
	_, err := d.Discover(ctx)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func setupSysForDM(t *testing.T, sysPath, device, uuid string) {
	t.Helper()
	dmDir := filepath.Join(sysPath, "block", device, "dm")
	if err := os.MkdirAll(dmDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dmDir, "uuid"), []byte(uuid+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
