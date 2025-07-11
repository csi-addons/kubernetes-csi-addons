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

package platform

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/klog"
	"k8s.io/mount-utils"
)

// kubelet contains the paths for Kubernetes based platforms
type kubelet struct {
	basedir    string
	pluginPath string
	csiPath    string
	mounter    mount.Interface
}

// getKubelet() returns a *kubelet with default paths. Other functions could do
// some detection or inspection, and return a *kubelet for microk8s or other
// variations.
func getKubelet() Platform {
	return &kubelet{
		basedir:    "/var/lib/kubelet",
		pluginPath: "/plugins",
		csiPath:    "/kubernetes.io/csi",
		mounter:    mount.NewWithoutSystemd("/bin/mount"),
	}
}

// (optional) staging path:
// /var/lib/kubelet/plugins/kubernetes.io/csi/openshift-storage.rbd.csi.ceph.com/07d25c3a330223ce3dd44b6bbd5911523785b0cf44f1624fc9b949c0fbdd9d00/globalmount/0001-0011-openshift-storage-0000000000000001-316a945f-00b5-45e7-a9e1-56658da698b9
// - 07d25c3a330223ce3dd44b6bbd5911523785b0cf44f1624fc9b949c0fbdd9d00 = sha256sum(volumeID)
//
// TODO: blockmode
// fmt.Sprintf("plugins/kubernetes.io/csi/volumeDevices/%s/%s", "spec-0", "dev")
// fmt.Sprintf("plugins/kubernetes.io/csi/volumeDevices/staging/%s", "spec-0")
func (k *kubelet) GetStagingPath(driver, volumeID string) string {
	hash := sha256.Sum256([]byte(volumeID))

	stagingPath := filepath.Join(
		k.basedir,
		k.pluginPath,
		k.csiPath,
		driver,
		fmt.Sprintf("%x", hash),
		"globalmount",
	)

	if k.isMountPoint(stagingPath) {
		return stagingPath
	}

	// stagingPath ending with "/globalmount" is not a mountpoint. Add the
	// volumeID to the path and check again.

	stagingPath = filepath.Join(stagingPath, volumeID)
	if k.isMountPoint(stagingPath) {
		return stagingPath
	}

	klog.Errorf("could not find a mountpoint at %q", stagingPath)

	// TODO: maybe need to check if it is a directory or blockdevice?
	return ""
}

func (k *kubelet) isMountPoint(path string) bool {
	isMnt, err := k.mounter.IsMountPoint(path)
	if err != nil {
		klog.Errorf("failed to check if %q is a mountpoint: %v", path, err)
		return false
	}

	return isMnt
}

// GetPublishPath tries to find the directory where the volume is mounted.
//
// Kubelet uses /var/lib/kubelet/pods/*/volumes/*/vol_data.json for storing
// details about the mountpoint. The JSON file can be parsed and checked if the
// drivername and volumeID match what we are looking for.
//
// The publish path is the in the same directory as where the vol_data.json
// resides. The volume is mounted on a directory called "mount".
//
// TODO: blockmode
// fmt.Sprintf("plugins/kubernetes.io/csi/volumeDevices/publish/%s/%s", "spec-0", testPodUID)
func (k *kubelet) GetPublishPath(driver, volumeID string) string {
	volDatas, err := filepath.Glob(
		filepath.Join(
			k.basedir,
			"pods",
			"*", // UUID of a Pod
			"volumes/kubernetes.io~csi/",
			"*", // name of a PersistentVolume
			"vol_data.json",
		),
	)
	if err != nil {
		klog.Errorf("could not find vol_data.json: %v", err)
		return ""
	}

	type volData struct {
		Driver               string `json:"driverName"`
		PersistentVolumeName string `json:"specVolID"`
		VolumeID             string `json:"volumeHandle"`
	}
	var vd volData

	for _, filename := range volDatas {
		data, err := os.ReadFile(filename)
		if err != nil {
			klog.Errorf("failed to read file %q: %v", filename, err)
			continue
		}

		err = json.Unmarshal(data, &vd)
		if err != nil {
			klog.Errorf("failed to parse JSON from %q: %v", filename, err)
			continue
		}

		if vd.Driver == driver && vd.VolumeID == volumeID {
			// TODO: is this correct for block-volumes too?
			return strings.ReplaceAll(filename, "vol_data.json", "mount")
		}
	}

	klog.Errorf("could not find a vol_data.json for driver %q and volumeID %q", driver, volumeID)
	return ""
}

func (k *kubelet) GetCSISocket(driver string) string {
	// TODO: should probably check if it is UNIX domain socket
	return filepath.Join(
		"unix://",
		k.basedir,
		k.pluginPath,
		driver,
		"csi.sock",
	)
}
