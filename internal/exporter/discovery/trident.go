package discovery

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// TridentDiscoverer implements discovery by reading Trident's tracking JSON files.
type TridentDiscoverer struct {
	trackingDir string
	sysPath     string
	nodeName    string
	logger      *slog.Logger
	disabled    bool
}

// NewTridentDiscoverer creates a Trident discoverer.
// The tracking directory is checked once at construction time. If it does not
// exist, the discoverer is permanently disabled for the lifetime of the process
// — it will not re-enable if Trident is installed later. Restart the exporter
// after installing Trident to activate this discoverer.
func NewTridentDiscoverer(trackingDir, sysPath, nodeName string, logger *slog.Logger) *TridentDiscoverer {
	d := &TridentDiscoverer{
		trackingDir: trackingDir,
		sysPath:     sysPath,
		nodeName:    nodeName,
		logger:      logger,
	}
	if _, err := os.Stat(trackingDir); os.IsNotExist(err) {
		logger.Info("trident tracking directory not found, disabling discoverer (restart exporter after installing Trident)", "path", trackingDir)
		d.disabled = true
	}
	return d
}

// Name implements Discoverer.
func (d *TridentDiscoverer) Name() string { return "trident" }

// Discover implements Discoverer.
func (d *TridentDiscoverer) Discover(ctx context.Context) ([]VolumeDevice, error) {
	if d.disabled {
		return nil, nil
	}

	entries, err := os.ReadDir(d.trackingDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var results []VolumeDevice
	for _, entry := range entries {
		if ctx.Err() != nil {
			return results, ctx.Err()
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		volumeID := strings.TrimSuffix(entry.Name(), ".json")
		filePath := filepath.Join(d.trackingDir, entry.Name())

		device, driver, err := d.parseTrackingFile(filePath)
		if err != nil {
			d.logger.Warn("failed to parse trident tracking file",
				"file", filePath, "error", err)
			continue
		}

		results = append(results, VolumeDevice{
			VolumeHandle: volumeID,
			Driver:       driver,
			Device:       device,
			Node:         d.nodeName,
		})
	}
	return results, nil
}

type tridentTrackingInfo struct {
	VolumePublishInfo struct {
		DevicePath    string `json:"devicePath"`
		RawDevicePath string `json:"rawDevicePath"`
	} `json:"volumePublishInfo"`
	StagingTargetPath string `json:"stagingTargetPath"`
}

func (d *TridentDiscoverer) parseTrackingFile(filePath string) (device, driver string, err error) {
	data, err := readFileLimited(filePath, maxJSONFileSize)
	if err != nil {
		return "", "", err
	}

	var info tridentTrackingInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return "", "", err
	}

	devicePath := info.VolumePublishInfo.DevicePath
	if devicePath == "" {
		devicePath = info.VolumePublishInfo.RawDevicePath
	}
	if devicePath == "" {
		return "", "", ErrMissingFields
	}

	device = filepath.Base(devicePath)
	if strings.HasPrefix(device, "dm-") {
		device = ResolveLUKSUnderlyingDevice(d.sysPath, device)
	}

	return device, "csi.trident.netapp.io", nil
}
