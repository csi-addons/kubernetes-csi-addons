/*
Copyright 2022.

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

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VolumeGroupSpec describes the common attributes of group storage devices
// and allows a Source for provider-specific attributes
type VolumeGroupSpec struct {
	// +optional
	VolumeGroupClassName *string `json:"volumeGroupClassName,omitempty"`

	// Source has the information about where the group is created from.
	Source VolumeGroupSource `json:"source"`
}

// VolumeGroupSource contains several options.
// OneOf the options must be defined.
type VolumeGroupSource struct {
	// +optional
	// Pre-provisioned VolumeGroup
	VolumeGroupContentName *string `json:"volumeGroupContentName,omitempty"`

	// +optional
	// Dynamically provisioned VolumeGroup
	// A label query over persistent volume claims to be added to the volume group.
	// This labelSelector will be used to match the label added to a PVC.
	// In Phase 1, when the label is added to PVC, the PVC will be added to the matching group.
	// In Phase 2, this labelSelector will be used to find all PVCs with matching label and add them to the group when the group is being created.
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
}

// VolumeGroupStatus defines the observed state of VolumeGroup
type VolumeGroupStatus struct {
	// +optional
	BoundVolumeGroupContentName *string `json:"boundVolumeGroupContentName,omitempty"`

	// +optional
	GroupCreationTime *metav1.Time `json:"groupCreationTime,omitempty"`

	// A list of persistent volume claims
	// +optional
	PVCList []corev1.PersistentVolumeClaim `json:"pvcList,omitempty"`

	// +optional
	Ready *bool `json:"ready,omitempty"`

	// Last error encountered during group creation
	// +optional
	Error *VolumeGroupError `json:"error,omitempty"`
}

// VolumeGroup is a user's request for a group of volumes
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=vg
// +kubebuilder:printcolumn:name="Ready",type=boolean,JSONPath=`.status.ready`
// +kubebuilder:printcolumn:name="VolumeGroupClass",type=string,JSONPath=`.spec.volumeGroupClassName`
// +kubebuilder:printcolumn:name="VolumeGroupContent",type=string,JSONPath=`.status.boundVolumeGroupContentName`
// +kubebuilder:printcolumn:name="CreationTime",type=date,JSONPath=`.status.groupCreationTime`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type VolumeGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the volume group requested by a user
	Spec VolumeGroupSpec `json:"spec,omitempty"`
	// Status represents the current information about a volume group
	// +optional
	Status VolumeGroupStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// VolumeGroupList contains a list of VolumeGroup
type VolumeGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VolumeGroup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VolumeGroup{}, &VolumeGroupList{})
}
