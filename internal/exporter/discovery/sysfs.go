package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// maxSysfsFileSize limits reads from sysfs pseudo-files (e.g. dm/uuid).
const maxSysfsFileSize = 256

// ResolveDeviceName resolves a major:minor pair to a kernel block device name
// by reading the symlink at /sys/dev/block/{major}:{minor}.
func ResolveDeviceName(sysPath string, major, minor uint32) (string, error) {
	link := filepath.Join(sysPath, "dev", "block", fmt.Sprintf("%d:%d", major, minor))
	target, err := os.Readlink(link)
	if err != nil {
		return "", fmt.Errorf("readlink %s: %w", link, err)
	}
	return filepath.Base(target), nil
}

// maxDMDepth limits recursion depth to prevent stack overflow from circular sysfs links.
const maxDMDepth = 16

// ResolveLUKSUnderlyingDevice walks the DM slave chain to find the storage
// device that should be reported in the metric. The walk stops (returns the
// device as-is) when it encounters:
//   - a multipath device (dm/uuid starts with "mpath-") — this is the
//     correct device to report because node_dmmultipath_path_state tracks it
//   - a physical block device (any non-dm- slave)
//   - the recursion depth limit
//
// This is intended for LUKS-over-multipath stacks where the CSI driver
// stages a LUKS dm device on top of a multipath dm device. Passing a plain
// multipath dm-X returns it unchanged.
// If resolution fails at any point, the input device is returned as-is.
func ResolveLUKSUnderlyingDevice(sysPath string, device string) string {
	return resolveLUKSDepth(sysPath, device, 0)
}

func resolveLUKSDepth(sysPath string, device string, depth int) string {
	if depth >= maxDMDepth {
		return device
	}
	if !strings.HasPrefix(device, "dm-") {
		return device
	}

	isMpath, err := isMultipathDevice(sysPath, device)
	if err != nil {
		// Treat a sysfs read error conservatively: stop the walk and return
		// the current device unchanged rather than recursing into an unknown stack.
		return device
	}
	if isMpath {
		return device
	}

	slavesDir := filepath.Join(sysPath, "block", device, "slaves")
	entries, err := os.ReadDir(slavesDir)
	if err != nil {
		return device
	}

	// Prefer the first recognised slave rather than returning on the first
	// dm- branch. This handles stacks where multiple slaves are listed and
	// the first one may not be the meaningful device.
	for _, entry := range entries {
		slave := entry.Name()
		if isTerminalBlockDevice(slave) {
			return slave
		}
		if strings.HasPrefix(slave, "dm-") {
			slaveIsMpath, err := isMultipathDevice(sysPath, slave)
			if err != nil {
				// Cannot determine slave type; skip it rather than recurse blindly.
				continue
			}
			if slaveIsMpath {
				return slave
			}
			resolved := resolveLUKSDepth(sysPath, slave, depth+1)
			if resolved != slave {
				return resolved
			}
		}
	}

	return device
}

// isTerminalBlockDevice returns true if the device name matches a known
// non-DM block device (physical, virtual, or software RAID). These are
// leaf nodes in the DM slave walk — we stop recursing when we find one.
func isTerminalBlockDevice(name string) bool {
	for _, prefix := range []string{
		"sd",     // SCSI / SAS / iSCSI / FC
		"nvme",   // NVMe
		"vd",     // virtio (KVM/QEMU)
		"xvd",    // Xen paravirt
		"hd",     // legacy IDE
		"mmcblk", // eMMC / SD card
		"sr",     // optical (edge case)
		"md",     // Linux software RAID
		"loop",   // loop device
		"nbd",    // network block device
	} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func isMultipathDevice(sysPath string, device string) (bool, error) {
	uuidPath := filepath.Join(sysPath, "block", device, "dm", "uuid")
	data, err := readFileLimited(uuidPath, maxSysfsFileSize)
	if err != nil {
		return false, err
	}
	return strings.HasPrefix(strings.TrimSpace(string(data)), "mpath-"), nil
}
