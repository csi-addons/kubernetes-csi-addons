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

// VolumeGroupContentSpec defines the desired state of VolumeGroupContent
type VolumeGroupContentSpec struct {
	// +optional
	VolumeGroupClassName *string `json:"volumeGroupClassName,omitempty"`

	// +optional
	// VolumeGroupRef is part of a bi-directional binding between VolumeGroup and VolumeGroupContent.
	VolumeGroupRef *corev1.ObjectReference `json:"volumeGroupRef,omitempty"`

	// +optional
	Source *VolumeGroupContentSource `json:"source,omitempty"`

	// +optional
	VolumeGroupDeletionPolicy *VolumeGroupDeletionPolicy `json:"volumeGroupDeletionPolicy,omitempty"`

	// This field specifies whether group snapshot is supported.
	// The default is false.
	// +optional
	SupportVolumeGroupSnapshot *bool `json:"supportVolumeGroupSnapshot,omitempty"`

	// VolumeGroupSecretRef is a reference to the secret object containing
	// sensitive information to pass to the CSI driver to complete the CSI
	// calls for VolumeGroups.
	// This field is optional, and may be empty if no secret is required. If the
	// secret object contains more than one secret, all secrets are passed.
	// +optional
	VolumeGroupSecretRef *corev1.SecretReference `json:"volumeGroupSecretRef,omitempty"`
}

// VolumeGroupContentSource
type VolumeGroupContentSource struct {
	Driver string `json:"driver"`

	// VolumeGroupHandle is the unique volume group name returned by the
	// CSI volume plugin’s CreateVolumeGroup to refer to the volume group on
	// all subsequent calls.
	VolumeGroupHandle string `json:"volumeGroupHandle"`

	// +optional
	// Attributes of the volume group to publish.
	VolumeGroupAttributes map[string]string `json:"volumeGroupAttributes,omitempty"`
}

// VolumeGroupContentStatus defines the observed state of VolumeGroupContent
type VolumeGroupContentStatus struct {
	// +optional
	GroupCreationTime *metav1.Time `json:"groupCreationTime,omitempty"`

	// A list of persistent volumes
	// +optional
	PVList []corev1.PersistentVolume `json:"pvList,omitempty"`

	// +optional
	Ready *bool `json:"ready,omitempty"`

	// Last error encountered during group creation
	// +optional
	Error *VolumeGroupError `json:"error,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// VolumeGroupContent is the Schema for the volumegroupcontents API
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced,shortName=vgc
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=boolean,JSONPath=`.status.ready`
// +kubebuilder:printcolumn:name="DeletionPolicy",type=string,JSONPath=`.spec.volumeGroupDeletionPolicy`
// +kubebuilder:printcolumn:name="Driver",type=string,JSONPath=`.spec.source.driver`
// +kubebuilder:printcolumn:name="VolumeGroupClass",type=string,JSONPath=`.spec.volumeGroupClassName`
// +kubebuilder:printcolumn:name="VolumeGroup",type=string,JSONPath=`.spec.volumeGroupRef.name`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type VolumeGroupContent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the volume group requested by a user
	Spec VolumeGroupContentSpec `json:"spec,omitempty"`
	// Status represents the current information about a volume group
	// +optional
	Status VolumeGroupContentStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// VolumeGroupContentList contains a list of VolumeGroupContent
type VolumeGroupContentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VolumeGroupContent `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VolumeGroupContent{}, &VolumeGroupContentList{})
}
