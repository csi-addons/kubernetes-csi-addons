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

// Platform provides functions that a Driver can use to find details about the
// deployment. Different platforms use different directories and paths to
// communicate with drivers, and locations where volumes are mounted.
type Platform interface {
	// GetStagingPath returns the volume staging path that a platform uses.
	// Not all drivers use the staging path, in which case no staging path
	// can be found and an error is returned.
	GetStagingPath(driver, volumeID string) (string, error)

	// GetPublishPath returns the path where the volume is mounted and
	// made available for apps/pods to use.
	GetPublishPath(driver, volumeID string) (string, error)

	// GetCSISocket returns the UNIX Domain Socket for a particular
	// CSI-driver.
	GetCSISocket(driver string) string

	// ResolvePersistentVolumeName tries to identify the name of the
	// PersistentVolume, based on the volumeID.
	ResolvePersistentVolumeName(driver, volumeID string) (string, error)
}

// singletonPlatform is used to create the Platform utility object once. While
// the application runs, the platform will not suddenly change.
var singletonPlatform Platform = nil

// GetPlatform returns the object with utility functions for the current running
// variant of Kubernetes.
func GetPlatform() Platform {
	if singletonPlatform != nil {
		return singletonPlatform
	}

	// TODO: switch on detecting different platforms (kind, microk8s?)
	singletonPlatform = getKubelet()

	return singletonPlatform
}
