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
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// Index keys
	StorageClassIndex = "spec.storageClassName"
	JobOwnerKey       = ".metadata.controller"
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

// IsManagedByController checks whether an object is managed by the controller.
// It returns false if the object has the CSIAddonsStateAnnotation set to a value
// other than CSIAddonsStateManaged. If the annotation is missing it returns true.
func IsManagedByController(obj client.Object) bool {
	if v, ok := obj.GetAnnotations()[csiaddonsv1alpha1.CSIAddonsStateAnnotation]; ok && v != csiaddonsv1alpha1.CSIAddonsStateManaged {
		return false
	}
	return true
}
