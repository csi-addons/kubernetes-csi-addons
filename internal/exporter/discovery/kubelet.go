package discovery

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// KubeletDiscoverer discovers CSI volume-to-device mappings by walking the
// kubelet pod directory tree. It handles both VolumeMode=Filesystem and
// VolumeMode=Block volumes.
//
// Filesystem volumes: walk /var/lib/kubelet/pods/*/volumes/kubernetes.io~csi/*/
// and for each directory:
//  1. Read vol_data.json (written by kubelet at pod publish time)
//  2. stat() the mount/ subdirectory to get the device major:minor from st_dev
//  3. Verify mount propagation (st_dev must differ from parent directory)
//  4. Resolve via /sys/dev/block/{major}:{minor} to get the kernel device name
//  5. For DM devices, walk through LUKS to the underlying multipath device
//
// Block volumes: walk /var/lib/kubelet/pods/*/volumeDevices/kubernetes.io~csi/*/
// and for each entry:
//  1. stat() the device file to get the major:minor from st_rdev
//  2. Read vol_data.json from the plugin staging path
//  3. Resolve via sysfs as with filesystem volumes
//
// Volumes on network filesystems (NFS, CephFS, etc.) are naturally filtered:
// they use pseudo-device numbers (major=0) that don't exist in /sys/dev/block/,
// so sysfs resolution fails and the volume is skipped.
//
// Volumes not published to any pod are not discovered, which is correct: if no
// workload uses the volume, there's nothing to alert about.
type KubeletDiscoverer struct {
	kubeletRoot string
	sysPath     string
	nodeName    string
	logger      *slog.Logger
}

// NewKubeletDiscoverer creates a universal kubelet discoverer.
//
// kubeletRoot is where the kubelet data tree is mounted inside the container
// (e.g. /var/lib/kubelet). With mountPropagation: HostToContainer, the host's
// CSI bind-mounts are visible and stat() returns the correct device info.
func NewKubeletDiscoverer(kubeletRoot, sysPath, nodeName string, logger *slog.Logger) *KubeletDiscoverer {
	return &KubeletDiscoverer{
		kubeletRoot: kubeletRoot,
		sysPath:     sysPath,
		nodeName:    nodeName,
		logger:      logger,
	}
}

// Name implements Discoverer.
func (d *KubeletDiscoverer) Name() string { return "kubelet" }

// Discover implements Discoverer.
func (d *KubeletDiscoverer) Discover(ctx context.Context) ([]VolumeDevice, error) {
	seen := make(map[string]struct{})
	var results []VolumeDevice

	podsDir := filepath.Join(d.kubeletRoot, "pods")
	podEntries, err := os.ReadDir(podsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	for _, podEntry := range podEntries {
		if ctx.Err() != nil {
			break
		}
		if !podEntry.IsDir() {
			continue
		}

		// Filesystem volumes: pods/<uid>/volumes/kubernetes.io~csi/<pv>/
		csiVolDir := filepath.Join(podsDir, podEntry.Name(), "volumes", "kubernetes.io~csi")
		volEntries, err := os.ReadDir(csiVolDir)
		if err == nil {
			for _, volEntry := range volEntries {
				if ctx.Err() != nil {
					break
				}
				if !volEntry.IsDir() {
					continue
				}

				volDir := filepath.Join(csiVolDir, volEntry.Name())
				vd, err := readVolData(volDir)
				if err != nil {
					d.logger.Debug("skipping volume: cannot read vol_data.json",
						"pod", podEntry.Name(), "volume", volEntry.Name(), "error", err)
					continue
				}
				if vd.VolumeLifecycleMode == "Ephemeral" {
					d.logger.Debug("skipping ephemeral volume",
						"volume_handle", vd.VolumeHandle, "driver", vd.DriverName)
					continue
				}
				if _, ok := seen[vd.VolumeHandle]; ok {
					d.logger.Debug("skipping duplicate volume_handle",
						"volume_handle", vd.VolumeHandle)
					continue
				}

				device := d.resolveVolumeDevice(volDir)
				if device == "" {
					d.logger.Debug("skipping volume: device not resolvable (network FS or propagation issue)",
						"volume_handle", vd.VolumeHandle, "driver", vd.DriverName, "path", volDir)
					continue
				}

				seen[vd.VolumeHandle] = struct{}{}
				results = append(results, VolumeDevice{
					VolumeHandle: vd.VolumeHandle,
					Driver:       vd.DriverName,
					Device:       device,
					Node:         d.nodeName,
				})
			}
		}

		// Block volumes: pods/<uid>/volumeDevices/kubernetes.io~csi/<pv>/<pv>
		csiBlockDir := filepath.Join(podsDir, podEntry.Name(), "volumeDevices", "kubernetes.io~csi")
		blockEntries, err := os.ReadDir(csiBlockDir)
		if err != nil {
			continue
		}
		for _, blockEntry := range blockEntries {
			if ctx.Err() != nil {
				break
			}
			if !blockEntry.IsDir() {
				continue
			}

			specName := blockEntry.Name()
			pvDir := filepath.Join(csiBlockDir, specName)

			// The device file inside the PV directory is named after the PV.
			devFilePath := filepath.Join(pvDir, specName)

			var st unix.Stat_t
			if err := unix.Stat(devFilePath, &st); err != nil {
				d.logger.Debug("skipping block volume: cannot stat device file",
					"pod", podEntry.Name(), "volume", specName, "error", err)
				continue
			}

			major := unix.Major(st.Rdev)
			minor := unix.Minor(st.Rdev)
			if major == 0 && minor == 0 {
				d.logger.Debug("skipping block volume: st_rdev is 0 (not a real device)",
					"pod", podEntry.Name(), "volume", specName)
				continue
			}

			// vol_data.json for block volumes lives at the plugin staging path:
			// plugins/kubernetes.io~csi/volumeDevices/<specName>/data/
			volDataDir := filepath.Join(d.kubeletRoot, "plugins", "kubernetes.io~csi", "volumeDevices", specName, "data")
			vd, err := readVolData(volDataDir)
			if err != nil {
				d.logger.Debug("skipping block volume: cannot read vol_data.json from staging path",
					"pod", podEntry.Name(), "volume", specName, "error", err)
				continue
			}
			if vd.VolumeLifecycleMode == "Ephemeral" {
				d.logger.Debug("skipping ephemeral block volume",
					"volume_handle", vd.VolumeHandle, "driver", vd.DriverName)
				continue
			}
			if _, ok := seen[vd.VolumeHandle]; ok {
				d.logger.Debug("skipping duplicate volume_handle (block)",
					"volume_handle", vd.VolumeHandle)
				continue
			}

			device := d.resolveAndFilter(major, minor)
			if device == "" {
				d.logger.Debug("skipping block volume: device not resolvable in sysfs",
					"volume_handle", vd.VolumeHandle, "driver", vd.DriverName,
					"major", major, "minor", minor)
				continue
			}

			seen[vd.VolumeHandle] = struct{}{}
			results = append(results, VolumeDevice{
				VolumeHandle: vd.VolumeHandle,
				Driver:       vd.DriverName,
				Device:       device,
				Node:         d.nodeName,
			})
		}
	}

	if ctx.Err() != nil && len(results) == 0 {
		return nil, ctx.Err()
	}
	return results, nil
}

// resolveVolumeDevice determines the block device for a filesystem CSI volume
// by stat()ing the mount/ subdirectory and reading st_dev.
//
// Returns empty string if the volume doesn't resolve in sysfs (e.g. NFS with
// major=0) — network filesystems are filtered naturally this way.
func (d *KubeletDiscoverer) resolveVolumeDevice(volDir string) string {
	mountDir := filepath.Join(volDir, "mount")
	return d.resolveFromFilesystemMount(mountDir)
}

// resolveFromFilesystemMount uses stat() on a mount point to get the device.
// If mount propagation isn't working, mount/ will have the same st_dev as its
// parent directory (the underlying host filesystem). We detect this and skip
// to avoid mapping the CSI volume to the node's root disk.
func (d *KubeletDiscoverer) resolveFromFilesystemMount(path string) string {
	var st unix.Stat_t
	if err := unix.Stat(path, &st); err != nil {
		return ""
	}

	var parentSt unix.Stat_t
	if err := unix.Stat(filepath.Dir(path), &parentSt); err != nil {
		return ""
	}
	if st.Dev == parentSt.Dev {
		return ""
	}

	return d.resolveAndFilter(unix.Major(st.Dev), unix.Minor(st.Dev))
}

// resolveAndFilter resolves a major:minor via sysfs to a kernel device name.
// For device-mapper devices, it walks through LUKS layers to find the
// underlying storage device (multipath or physical).
// Returns empty string if the major:minor doesn't resolve in sysfs (e.g.
// network FS with major=0 has no /sys/dev/block/ entry).
func (d *KubeletDiscoverer) resolveAndFilter(major, minor uint32) string {
	device, err := ResolveDeviceName(d.sysPath, major, minor)
	if err != nil {
		return ""
	}

	if strings.HasPrefix(device, "dm-") {
		return ResolveLUKSUnderlyingDevice(d.sysPath, device)
	}
	return device
}

type volData struct {
	VolumeHandle        string `json:"volumeHandle"`
	DriverName          string `json:"driverName"`
	SpecVolID           string `json:"specVolID"`
	VolumeLifecycleMode string `json:"volumeLifecycleMode"`
}

func readVolData(dir string) (*volData, error) {
	data, err := readFileLimited(filepath.Join(dir, "vol_data.json"), maxJSONFileSize)
	if err != nil {
		return nil, err
	}
	var vd volData
	if err := json.Unmarshal(data, &vd); err != nil {
		return nil, err
	}
	if vd.VolumeHandle == "" || vd.DriverName == "" {
		return nil, ErrMissingFields
	}
	return &vd, nil
}
