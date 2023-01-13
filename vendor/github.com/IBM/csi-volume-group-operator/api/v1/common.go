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
