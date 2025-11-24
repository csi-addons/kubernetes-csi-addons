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

package utils

import (
	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/csiaddons/v1alpha1"
)

var (
	RsCronJobScheduleTimeAnnotation = "reclaimspace." + csiaddonsv1alpha1.GroupVersion.Group + "/schedule"
	RsCronJobNameAnnotation         = "reclaimspace." + csiaddonsv1alpha1.GroupVersion.Group + "/cronjob"

	KrEnableAnnotation           = "keyrotation." + csiaddonsv1alpha1.GroupVersion.Group + "/enable"
	KrcJobScheduleTimeAnnotation = "keyrotation." + csiaddonsv1alpha1.GroupVersion.Group + "/schedule"
	KrcJobNameAnnotation         = "keyrotation." + csiaddonsv1alpha1.GroupVersion.Group + "/cronjob"

	CSIAddonsStateAnnotation = csiaddonsv1alpha1.GroupVersion.Group + "/state"
)

const (
	// Index keys
	StorageClassIndex = "spec.storageClassName"
	JobOwnerKey       = ".metadata.controller"

	// Represents the CRs that are managed by the PVC controller
	CSIAddonsStateManaged = "managed"
)

// AnnotationValueChanged checks if any of the specified keys have different values
// between the old and new annotations maps.
func AnnotationValueChanged(oldAnnotations, newAnnotations map[string]string, keys []string) bool {
	for _, key := range keys {
		oldVal, oldExists := oldAnnotations[key]
		newVal, newExists := newAnnotations[key]

		if oldExists != newExists || oldVal != newVal {
			return true
		}
	}
	return false
}
