package discovery

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// HPEDiscoverer implements discovery by reading HPE's deviceInfo.json files.
type HPEDiscoverer struct {
	kubeletRoot string
	sysPath     string
	nodeName    string
	logger      *slog.Logger
}

// NewHPEDiscoverer creates an HPE discoverer.
func NewHPEDiscoverer(kubeletRoot, sysPath, nodeName string, logger *slog.Logger) *HPEDiscoverer {
	return &HPEDiscoverer{
		kubeletRoot: kubeletRoot,
		sysPath:     sysPath,
		nodeName:    nodeName,
		logger:      logger,
	}
}

// Name implements Discoverer.
func (d *HPEDiscoverer) Name() string { return "hpe" }

// Discover implements Discoverer.
func (d *HPEDiscoverer) Discover(ctx context.Context) ([]VolumeDevice, error) {
	csiPluginDir := filepath.Join(d.kubeletRoot, "plugins", "kubernetes.io", "csi")
	entries, err := os.ReadDir(csiPluginDir)
	if err != nil {
		if os.IsNotExist(err) {
			d.logger.Debug("kubelet CSI plugin dir not found, no HPE volumes to discover",
				"path", csiPluginDir)
			return nil, nil
		}
		return nil, err
	}

	var results []VolumeDevice
	for _, entry := range entries {
		if ctx.Err() != nil {
			return results, ctx.Err()
		}
		if !entry.IsDir() {
			continue
		}
		pvDir := filepath.Join(csiPluginDir, entry.Name())
		deviceInfoPath := filepath.Join(pvDir, "globalmount", "deviceInfo.json")

		vd, err := d.parseDeviceInfo(deviceInfoPath)
		if err != nil {
			// Only warn when the file exists but is malformed — missing files
			// are expected for non-HPE volumes and would be very noisy.
			if !os.IsNotExist(err) {
				d.logger.Warn("failed to parse HPE deviceInfo",
					"path", deviceInfoPath, "error", err)
			}
			continue
		}

		results = append(results, *vd)
	}
	return results, nil
}

type hpeDeviceInfo struct {
	VolumeID string `json:"volume_id"`
	Device   struct {
		AltFullPathName string `json:"alt_full_path_name"`
	} `json:"device"`
}

func (d *HPEDiscoverer) parseDeviceInfo(path string) (*VolumeDevice, error) {
	data, err := readFileLimited(path, maxJSONFileSize)
	if err != nil {
		return nil, err
	}

	var info hpeDeviceInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}

	if info.VolumeID == "" || info.Device.AltFullPathName == "" {
		return nil, ErrMissingFields
	}

	device := filepath.Base(info.Device.AltFullPathName)
	if strings.HasPrefix(device, "dm-") {
		device = ResolveLUKSUnderlyingDevice(d.sysPath, device)
	}

	return &VolumeDevice{
		VolumeHandle: info.VolumeID,
		Driver:       "csi.hpe.com",
		Device:       device,
		Node:         d.nodeName,
	}, nil
}
