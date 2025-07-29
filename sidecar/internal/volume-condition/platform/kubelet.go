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

	"k8s.io/klog/v2"
	"k8s.io/mount-utils"
)

// kubelet contains the paths for Kubernetes based platforms
type kubelet struct {
	basedir    string
	pluginPath string
	csiPath    string
	mounter    mount.Interface
}

// assert that kubelet implements the Platform interface.
var _ Platform = &kubelet{}

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

// GetStagingPath checks the known kubelet path where the volume may be mounted.
// The staging path looks like:
// /var/lib/kubelet
//   - /plugins
//   - /kubernetes.io/csi
//   - /openshift-storage.rbd.csi.ceph.com
//   - /07d25c3a330223ce3dd44b6bbd5911523785b0cf44f1624fc9b949c0fbdd9d00
//   - /globalmount
//   - /0001-0011-openshift-storage-0000000000000001-316a945f-00b5-45e7-a9e1-56658da698b9
//
// Where 07d25c3a330223ce3dd44b6bbd5911523785b0cf44f1624fc9b949c0fbdd9d00 = sha256sum(volumeID)
func (k *kubelet) GetStagingPath(driver, volumeID string) (string, error) {
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
		return stagingPath, nil
	}

	// special case for Ceph-CSI: stagingPath ending with "/globalmount" is not a
	// mountpoint. Add the volumeID to the path and check again.
	stagingPath = filepath.Join(stagingPath, volumeID)
	if k.isMountPoint(stagingPath) {
		return stagingPath, nil
	}

	// TODO: maybe need to check if it is a directory or blockdevice?
	// fmt.Sprintf("plugins/kubernetes.io/csi/volumeDevices/%s/%s", "spec-0", "dev")
	// fmt.Sprintf("plugins/kubernetes.io/csi/volumeDevices/staging/%s", "spec-0")

	return "", fmt.Errorf("could not find a mountpoint at %q", stagingPath)

}

func (k *kubelet) GetPublishPath(driver, volumeID string) (string, error) {
	path, _, err := k.getPublishDetails(driver, volumeID)
	if err != nil {
		return "", fmt.Errorf("failed to find publish path for volume handle %q of driver %q: %w", volumeID, driver, err)
	}

	return path, nil
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

func (k *kubelet) ResolvePersistentVolumeName(driver, volumeID string) (string, error) {
	_, pvName, err := k.getPublishDetails(driver, volumeID)
	if err != nil {
		return "", fmt.Errorf("failed to get persistent volume name for volume handle %q of driver %q: %w", volumeID, driver, err)
	}

	return pvName, nil
}

// isMountPoint checks if [path] is a mountpoint. This function tries to be
// safe, and not access the potential mountpoint (as that may hang when the
// volume is unhealthy).
func (k *kubelet) isMountPoint(path string) bool {
	// Mounter.List() reads entries from /proc/mounts, and does not try to
	// stat() the path.
	mps, err := k.mounter.List()
	if err != nil {
		klog.Errorf("failed to list mountpoints: %v", err)
		return false
	}

	for _, mp := range mps {
		if mp.Path == path {
			return true
		}
	}

	return false
}

// getPublishDetails tries to find the directory where the volume is mounted.
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
func (k *kubelet) getPublishDetails(driver, volumeID string) (string, string, error) {
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
		return "", "", fmt.Errorf("could not find vol_data.json: %v", err)
	}

	for _, filename := range volDatas {
		data, err := os.ReadFile(filename)
		if err != nil {
			klog.Errorf("failed to read file %q: %v", filename, err)
			continue
		}

		vd := struct {
			Driver               string `json:"driverName"`
			PersistentVolumeName string `json:"specVolID"`
			VolumeID             string `json:"volumeHandle"`
		}{}
		err = json.Unmarshal(data, &vd)
		if err != nil {
			klog.Errorf("failed to parse JSON from %q: %v", filename, err)
			continue
		}

		if vd.Driver != driver || vd.VolumeID != volumeID {
			continue
		}

		// TODO: is this correct for block-volumes too?
		return strings.ReplaceAll(filename, "vol_data.json", "mount"), vd.PersistentVolumeName, nil
	}

	return "", "", fmt.Errorf("could not find a vol_data.json for driver %q and volumeID %q", driver, volumeID)
}
