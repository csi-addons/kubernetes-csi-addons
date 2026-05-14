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
	"regexp"
	"strings"

	"github.com/moby/sys/mountinfo"
	"golang.org/x/sys/unix"
)

// KubeletDiscoverer implements universal discovery using vol_data.json + mountinfo.
type KubeletDiscoverer struct {
	// containerKubeletRoot is the path where host kubelet root is mounted inside the container
	// (e.g., /host/kubelet). Used for reading files.
	containerKubeletRoot string
	// hostKubeletRoot is the actual kubelet root path on the host
	// (e.g., /var/lib/kubelet). Used for matching mountinfo entries.
	hostKubeletRoot string
	procPath        string
	sysPath         string
	nodeName        string
	logger          *slog.Logger
	publishPattern  *regexp.Regexp
}

type volDataIndex struct {
	byDir       map[string]*volData
	bySpecVolID map[string]*volData
}

// NewKubeletDiscoverer creates a universal kubelet discoverer.
// containerKubeletRoot is where kubelet data is accessible inside the container.
// hostKubeletRoot is the real path on the host (for matching mountinfo).
func NewKubeletDiscoverer(containerKubeletRoot, hostKubeletRoot, procPath, sysPath, nodeName string, logger *slog.Logger) *KubeletDiscoverer {
	pattern := regexp.MustCompile(
		regexp.QuoteMeta(hostKubeletRoot) + `/pods/[^/]+/volumes/kubernetes\.io~csi/([^/]+)/mount`,
	)
	return &KubeletDiscoverer{
		containerKubeletRoot: containerKubeletRoot,
		hostKubeletRoot:      hostKubeletRoot,
		procPath:             procPath,
		sysPath:              sysPath,
		nodeName:             nodeName,
		logger:               logger,
		publishPattern:       pattern,
	}
}

// Name implements Discoverer.
func (d *KubeletDiscoverer) Name() string { return "kubelet" }

// Discover implements Discoverer.
func (d *KubeletDiscoverer) Discover(ctx context.Context) ([]VolumeDevice, error) {
	mounts, err := ParseMountInfo(d.procPath)
	if err != nil {
		return nil, fmt.Errorf("parse mountinfo: %w", err)
	}

	idx, parseErrors := d.buildVolDataIndex()
	if parseErrors > 0 {
		d.logger.Warn("some vol_data.json files could not be parsed; those volumes will be missing from metrics",
			"count", parseErrors)
	}
	if idx == nil {
		idx = &volDataIndex{
			byDir:       make(map[string]*volData),
			bySpecVolID: make(map[string]*volData),
		}
	}

	// seen deduplicates across all sub-discoverers: globalmount, block-device,
	// and pod publish-mount can all match the same VolumeHandle.
	seen := make(map[string]struct{})
	var results []VolumeDevice

	appendUnique := func(vds []VolumeDevice) {
		for _, v := range vds {
			if _, ok := seen[v.VolumeHandle]; ok {
				continue
			}
			seen[v.VolumeHandle] = struct{}{}
			results = append(results, v)
		}
	}

	appendUnique(d.discoverFilesystemVolumes(ctx, mounts, idx))

	if ctx.Err() != nil {
		return results, ctx.Err()
	}

	appendUnique(d.discoverBlockVolumes(ctx))

	if ctx.Err() != nil {
		return results, ctx.Err()
	}

	appendUnique(d.discoverPublishMountVolumes(ctx, mounts, idx))

	if ctx.Err() != nil && len(results) == 0 {
		return nil, ctx.Err()
	}
	return results, nil
}

type volData struct {
	VolumeHandle        string `json:"volumeHandle"`
	DriverName          string `json:"driverName"`
	SpecVolID           string `json:"specVolID"`
	VolumeLifecycleMode string `json:"volumeLifecycleMode"`
}

// buildVolDataIndex reads all vol_data.json files once and indexes by directory name and specVolID.
// Uses containerKubeletRoot for file access.
func (d *KubeletDiscoverer) buildVolDataIndex() (*volDataIndex, int) {
	csiPluginDir := filepath.Join(d.containerKubeletRoot, "plugins", "kubernetes.io", "csi")
	driverEntries, err := os.ReadDir(csiPluginDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0
		}
		d.logger.Warn("failed to read kubelet CSI plugin dir", "path", csiPluginDir, "error", err)
		return nil, 0
	}

	idx := &volDataIndex{
		byDir:       make(map[string]*volData),
		bySpecVolID: make(map[string]*volData),
	}
	parseErrors := 0

	for _, driverEntry := range driverEntries {
		if !driverEntry.IsDir() {
			continue
		}
		driverDir := filepath.Join(csiPluginDir, driverEntry.Name())
		volumeEntries, err := os.ReadDir(driverDir)
		if err != nil {
			continue
		}
		for _, volEntry := range volumeEntries {
			if !volEntry.IsDir() {
				continue
			}
			pvDir := filepath.Join(driverDir, volEntry.Name())
			vd, err := readVolData(pvDir)
			if err != nil {
				parseErrors++
				d.logger.Warn("failed to parse vol_data.json", "dir", pvDir, "error", err)
				continue
			}
			key := filepath.Join(driverEntry.Name(), volEntry.Name())
			idx.byDir[key] = vd
			if vd.SpecVolID != "" {
				idx.bySpecVolID[vd.SpecVolID] = vd
			}
		}
	}
	return idx, parseErrors
}

// discoverFilesystemVolumes uses hostKubeletRoot for mountinfo matching.
func (d *KubeletDiscoverer) discoverFilesystemVolumes(ctx context.Context, mounts []*mountinfo.Info, idx *volDataIndex) []VolumeDevice {
	hostCSIDir := filepath.Join(d.hostKubeletRoot, "plugins", "kubernetes.io", "csi")

	var results []VolumeDevice
	for dirName, vd := range idx.byDir {
		if ctx.Err() != nil {
			return results
		}
		if vd.VolumeLifecycleMode == "Ephemeral" {
			continue
		}

		hostGlobalMount := filepath.Join(hostCSIDir, dirName, "globalmount")
		mount := FindMountUnder(mounts, hostGlobalMount)
		if mount == nil {
			continue
		}
		if IsNetworkFS(mount.FSType) {
			continue
		}

		device, err := d.resolveDevice(mount.Major, mount.Minor)
		if err != nil {
			d.logger.Warn("device resolution failed",
				"volume_handle", vd.VolumeHandle,
				"major", mount.Major,
				"minor", mount.Minor,
				"error", err)
			continue
		}

		results = append(results, VolumeDevice{
			VolumeHandle: vd.VolumeHandle,
			Driver:       vd.DriverName,
			Device:       device,
			Node:         d.nodeName,
		})
	}
	return results
}

// discoverBlockVolumes uses containerKubeletRoot for file access.
func (d *KubeletDiscoverer) discoverBlockVolumes(ctx context.Context) []VolumeDevice {
	blockDir := filepath.Join(d.containerKubeletRoot, "plugins", "kubernetes.io", "csi", "volumeDevices")
	entries, err := os.ReadDir(blockDir)
	if err != nil {
		return nil
	}

	var results []VolumeDevice
	for _, entry := range entries {
		if ctx.Err() != nil {
			return results
		}
		if !entry.IsDir() {
			continue
		}
		pvDir := filepath.Join(blockDir, entry.Name())
		vd, err := readVolData(pvDir)
		if err != nil {
			continue
		}
		if vd.VolumeLifecycleMode == "Ephemeral" {
			continue
		}

		devFile := filepath.Join(pvDir, "dev", vd.SpecVolID)
		var stat unix.Stat_t
		if err := unix.Stat(devFile, &stat); err != nil {
			continue
		}
		major := unix.Major(stat.Rdev)
		minor := unix.Minor(stat.Rdev)

		device, err := d.resolveDevice(int(major), int(minor))
		if err != nil {
			continue
		}

		results = append(results, VolumeDevice{
			VolumeHandle: vd.VolumeHandle,
			Driver:       vd.DriverName,
			Device:       device,
			Node:         d.nodeName,
		})
	}
	return results
}

// discoverPublishMountVolumes uses hostKubeletRoot for mountinfo matching (via publishPattern).
func (d *KubeletDiscoverer) discoverPublishMountVolumes(ctx context.Context, mounts []*mountinfo.Info, idx *volDataIndex) []VolumeDevice {
	var results []VolumeDevice

	for _, m := range mounts {
		if ctx.Err() != nil {
			return results
		}
		matches := d.publishPattern.FindStringSubmatch(m.Mountpoint)
		if matches == nil {
			continue
		}
		if IsNetworkFS(m.FSType) {
			continue
		}

		specVolID := matches[1]
		vd := idx.bySpecVolID[specVolID]
		if vd == nil {
			vd = idx.byDir[specVolID]
		}
		if vd == nil {
			continue
		}

		device, err := d.resolveDevice(m.Major, m.Minor)
		if err != nil {
			continue
		}

		results = append(results, VolumeDevice{
			VolumeHandle: vd.VolumeHandle,
			Driver:       vd.DriverName,
			Device:       device,
			Node:         d.nodeName,
		})
	}
	return results
}

func readVolData(pvDir string) (*volData, error) {
	data, err := readFileLimited(filepath.Join(pvDir, "vol_data.json"), maxJSONFileSize)
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

func (d *KubeletDiscoverer) resolveDevice(major, minor int) (string, error) {
	device, err := ResolveDeviceName(d.sysPath, uint32(major), uint32(minor))
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(device, "dm-") {
		resolved := ResolveLUKSUnderlyingDevice(d.sysPath, device)
		return resolved, nil
	}
	return device, nil
}
