/*
Copyright 2023 The Kubernetes-CSI-Addons Authors.

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

package utils

const (
	VGNamePrefix                 = "volumegroup"
	vgGroupName                  = "volumegroup.storage.openshift.io"
	VGFinalizer                  = vgGroupName
	VGAsPrefix                   = vgGroupName + "/"
	VgcFinalizer                 = VGAsPrefix + "vgc-protection"
	pvcVGFinalizer               = VGAsPrefix + "pvc-protection"
	PrefixedVGSecretNameKey      = VGAsPrefix + "secret-name"      // name key for secret
	PrefixedVGSecretNamespaceKey = VGAsPrefix + "secret-namespace" // namespace key secret
	letterBytes                  = "0123456789abcdefghijklmnopqrstuvwxyz"
	vgController                 = "volumeGroupController"
	warningEventType             = "Warning"
	normalEventType              = "Normal"
	storageClassVGParameter      = "volume_group"
	addingPVC                    = "addPVC"
	removingPVC                  = "removePVC"
	createVGC                    = "creatingVGC"
	vgcKind                      = "VolumeGroupContent"
	APIVersion                   = "volumegroup.storage.openshift.io/v1"
)
