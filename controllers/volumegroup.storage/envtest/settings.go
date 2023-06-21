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

package envtest

import (
	"time"
)

const (
	Timeout = time.Second * 30
)

var (
	VGName                 = "fake-vg-name"
	VGClassName            = "fake-vgclass-name"
	Namespace              = "default"
	SecretName             = "fake-secret-name"
	PVCName                = "fake-pvc-name"
	PVName                 = "fake-pv-name"
	SCName                 = "fake-storage-class-name"
	DriverName             = "driver.name"
	StorageClassParameters = map[string]string{
		"volumegroup.storage.openshift.io/secret-name":      SecretName,
		"volumegroup.storage.openshift.io/secret-namespace": Namespace,
	}
	FakeMatchLabels = map[string]string{
		"fake-label": "fake-value",
	}
	PVCProtectionFinalizer = "kubernetes.io/pvc-protection"
)
