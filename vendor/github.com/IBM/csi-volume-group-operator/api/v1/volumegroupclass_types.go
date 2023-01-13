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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// VolumeGroupClass is the Schema for the volumegroupclasses API
// +kubebuilder:resource:scope=Cluster,shortName=vgclass
// +kubebuilder:printcolumn:name="Driver",type=string,JSONPath=`.driver`
// +kubebuilder:printcolumn:name="DeletionPolicy",type=string,JSONPath=`.volumeGroupDeletionPolicy`
// +kubebuilder:printcolumn:name="SupportVolumeGroupSnapshot",type=boolean,JSONPath=`.supportVolumeGroupSnapshot`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type VolumeGroupClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Driver is the driver expected to handle this VolumeGroupClass.
	Driver string `json:"driver"`

	// Parameters hold parameters for the driver.
	// These values are opaque to the system and are passed directly
	// to the driver.
	// +optional
	Parameters map[string]string `json:"parameters,omitempty"`

	// +optional
	VolumeGroupDeletionPolicy *VolumeGroupDeletionPolicy `json:"volumeGroupDeletionPolicy,omitempty"`

	// This field specifies whether group snapshot is supported.
	// The default is false.
	// +optional
	SupportVolumeGroupSnapshot *bool `json:"supportVolumeGroupSnapshot,omitempty"`
}

//+kubebuilder:object:root=true

// VolumeGroupClassList contains a list of VolumeGroupClass
type VolumeGroupClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VolumeGroupClass `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VolumeGroupClass{}, &VolumeGroupClassList{})
}
