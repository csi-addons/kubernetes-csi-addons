package discovery

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDeviceName(t *testing.T) {
	sysPath := t.TempDir()

	blockDir := filepath.Join(sysPath, "dev", "block")
	if err := os.MkdirAll(blockDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../../block/dm-5", filepath.Join(blockDir, "253:5")); err != nil {
		t.Fatal(err)
	}

	device, err := ResolveDeviceName(sysPath, 253, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if device != "dm-5" {
		t.Errorf("expected dm-5, got %s", device)
	}
}

func TestResolveDeviceName_NotFound(t *testing.T) {
	sysPath := t.TempDir()
	_, err := ResolveDeviceName(sysPath, 999, 999)
	if err == nil {
		t.Fatal("expected error for non-existent device")
	}
}

func TestResolveLUKSUnderlyingDevice_NotDM(t *testing.T) {
	device := ResolveLUKSUnderlyingDevice("/fake", "sda")
	if device != "sda" {
		t.Errorf("expected sda, got %s", device)
	}
}

func TestResolveLUKSUnderlyingDevice_MultipathDevice(t *testing.T) {
	sysPath := t.TempDir()

	dmDir := filepath.Join(sysPath, "block", "dm-0", "dm")
	if err := os.MkdirAll(dmDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dmDir, "uuid"), []byte("mpath-12345\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	device := ResolveLUKSUnderlyingDevice(sysPath, "dm-0")
	if device != "dm-0" {
		t.Errorf("expected dm-0 (multipath returned as-is), got %s", device)
	}
}

func TestResolveLUKSUnderlyingDevice_LUKSOverMultipath(t *testing.T) {
	sysPath := t.TempDir()

	luksDir := filepath.Join(sysPath, "block", "dm-1", "dm")
	if err := os.MkdirAll(luksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(luksDir, "uuid"), []byte("CRYPT-LUKS2-xxx\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	luksSlavesDir := filepath.Join(sysPath, "block", "dm-1", "slaves")
	if err := os.MkdirAll(luksSlavesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(luksSlavesDir, "dm-0"), 0o755); err != nil {
		t.Fatal(err)
	}

	mpathDir := filepath.Join(sysPath, "block", "dm-0", "dm")
	if err := os.MkdirAll(mpathDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mpathDir, "uuid"), []byte("mpath-abcdef\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	device := ResolveLUKSUnderlyingDevice(sysPath, "dm-1")
	if device != "dm-0" {
		t.Errorf("expected dm-0 (underlying multipath), got %s", device)
	}
}

func TestResolveLUKSUnderlyingDevice_PhysicalSlave(t *testing.T) {
	sysPath := t.TempDir()

	dmDir := filepath.Join(sysPath, "block", "dm-2", "dm")
	if err := os.MkdirAll(dmDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dmDir, "uuid"), []byte("LVM-xxx\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	slavesDir := filepath.Join(sysPath, "block", "dm-2", "slaves")
	if err := os.MkdirAll(slavesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(slavesDir, "sda1"), 0o755); err != nil {
		t.Fatal(err)
	}

	device := ResolveLUKSUnderlyingDevice(sysPath, "dm-2")
	if device != "sda1" {
		t.Errorf("expected sda1, got %s", device)
	}
}

func TestResolveLUKSUnderlyingDevice_VirtioSlave(t *testing.T) {
	sysPath := t.TempDir()

	dmDir := filepath.Join(sysPath, "block", "dm-3", "dm")
	if err := os.MkdirAll(dmDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dmDir, "uuid"), []byte("LVM-virtio\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	slavesDir := filepath.Join(sysPath, "block", "dm-3", "slaves")
	if err := os.MkdirAll(slavesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(slavesDir, "vda"), 0o755); err != nil {
		t.Fatal(err)
	}

	device := ResolveLUKSUnderlyingDevice(sysPath, "dm-3")
	if device != "vda" {
		t.Errorf("expected vda (virtio), got %s", device)
	}
}
