/*
Copyright 2023.

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

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// VolumeGroupDeletionPolicy describes a policy for end-of-life maintenance of
// volume group contents
type VolumeGroupDeletionPolicy string

const (
	// VolumeGroupContentDelete means the group will be deleted from the
	// underlying storage system on release from its volume group.
	VolumeGroupContentDelete VolumeGroupDeletionPolicy = "Delete"

	// VolumeGroupContentRetain means the group will be left in its current
	// state on release from its volume group.
	VolumeGroupContentRetain VolumeGroupDeletionPolicy = "Retain"
)

// Describes an error encountered on the group
type VolumeGroupError struct {
	// time is the timestamp when the error was encountered.
	// +optional
	Time *metav1.Time `json:"time,omitempty"`

	// message details the encountered error
	// +optional
	Message *string `json:"message,omitempty"`
}
