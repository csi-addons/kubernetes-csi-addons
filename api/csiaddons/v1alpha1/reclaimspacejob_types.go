/*
Copyright 2021 The Kubernetes-CSI-Addons Authors.

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

package v1alpha1

import (
	"slices"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OperationResult represents the result of reclaim space operation.
type OperationResult string

const (
	// OperationResultSucceeded represents the Succeeded operation state.
	OperationResultSucceeded OperationResult = "Succeeded"

	// OperationResultFailed represents the Failed operation state.
	OperationResultFailed OperationResult = "Failed"
)

// ReclaimSpaceOperation specifies where a reclaim space operation is
// performed.
// +kubebuilder:validation:Enum=Controller;Node
type ReclaimSpaceOperation string

const (
	// ReclaimSpaceOperationController performs the ControllerReclaimSpace
	// CSI-Addons RPC against the CSI controller (for example, rbd sparsify).
	ReclaimSpaceOperationController ReclaimSpaceOperation = "Controller"

	// ReclaimSpaceOperationNode performs the NodeReclaimSpace CSI-Addons RPC
	// (for example, fstrim) on the node the volume is attached to.
	ReclaimSpaceOperationNode ReclaimSpaceOperation = "Node"
)

// ReclaimSpaceOperationState describes the state of a requested reclaim space
// operation. New states may be added in the future. Clients should treat an
// unknown state as non-terminal.
// +kubebuilder:validation:Enum=Pending;Succeeded;Skipped;Failed
type ReclaimSpaceOperationState string

const (
	// ReclaimSpaceOperationStatePending means the operation has not completed
	// yet and will be retried.
	ReclaimSpaceOperationStatePending ReclaimSpaceOperationState = "Pending"

	// ReclaimSpaceOperationStateSucceeded means the operation completed
	// successfully.
	ReclaimSpaceOperationStateSucceeded ReclaimSpaceOperationState = "Succeeded"

	// ReclaimSpaceOperationStateSkipped means the driver does not support the
	// operation. The operation is not retried.
	ReclaimSpaceOperationStateSkipped ReclaimSpaceOperationState = "Skipped"

	// ReclaimSpaceOperationStateFailed means the operation failed and will not
	// be retried. This is set once the job's backOffLimit or
	// retryDeadlineSeconds is reached before the operation completed.
	ReclaimSpaceOperationStateFailed ReclaimSpaceOperationState = "Failed"
)

// IsTerminal returns true when the operation will not be attempted again.
func (s ReclaimSpaceOperationState) IsTerminal() bool {
	switch s {
	case ReclaimSpaceOperationStateSucceeded,
		ReclaimSpaceOperationStateSkipped,
		ReclaimSpaceOperationStateFailed:
		return true
	default:
		return false
	}
}

// ReclaimSpaceOperationStatus describes the observed state of the requested
// reclaim space operation.
type ReclaimSpaceOperationStatus struct {
	// Operation identifies the reclaim space operation type.
	Operation ReclaimSpaceOperation `json:"operation"`

	// State is the current state of the operation.
	State ReclaimSpaceOperationState `json:"state"`

	// Message is a human-readable explanation of the current state.
	// +optional
	Message string `json:"message,omitempty"`

	// ReclaimedSpace is the amount of space reclaimed by this operation.
	// +optional
	ReclaimedSpace *resource.Quantity `json:"reclaimedSpace,omitempty"`

	// CompletionTime is when the operation reached a terminal state.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`
}

// DefaultReclaimSpaceOperations returns the operations that are performed when
// ReclaimSpaceJobSpec.Operations is not set. It MUST be kept in sync with the
// kubebuilder default on that field: callers that construct a spec and compare
// it against an object defaulted by the API server rely on both matching.
func DefaultReclaimSpaceOperations() []ReclaimSpaceOperation {
	return []ReclaimSpaceOperation{ReclaimSpaceOperationNode}
}

// TargetSpec defines the targets on which the operation can be
// performed.
type TargetSpec struct {
	// PersistentVolumeClaim specifies the target PersistentVolumeClaim name.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="persistentVolumeClaim is immutable"
	PersistentVolumeClaim string `json:"persistentVolumeClaim,omitempty"`
}

// ReclaimSpaceJobSpec defines the desired state of ReclaimSpaceJob
type ReclaimSpaceJobSpec struct {
	// Target represents volume target on which the operation will be
	// performed.
	// +kubebuilder:validation:Required
	Target TargetSpec `json:"target"`

	// BackOffLimit specifies the number of retries allowed before marking reclaim
	// space operation as failed. If not specified, defaults to 6. Maximum allowed
	// value is 60 and minimum allowed value is 0.
	// +optional
	// +kubebuilder:validation:Maximum=60
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default:=6
	BackoffLimit int32 `json:"backOffLimit"`

	// RetryDeadlineSeconds specifies the duration in seconds relative to the
	// start time that the operation may be retried; value MUST be positive integer.
	// If not specified, defaults to 600 seconds. Maximum allowed
	// value is 1800.
	// +optional
	// +kubebuilder:validation:Maximum=1800
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default:=600
	RetryDeadlineSeconds int64 `json:"retryDeadlineSeconds"`

	// Timeout specifies the timeout in seconds for the grpc request sent to the
	// CSI driver. If not specified, defaults to global reclaimspace timeout.
	// Minimum allowed value is 60.
	// +optional
	// +kubebuilder:validation:Minimum=60
	Timeout *int64 `json:"timeout,omitempty"`

	// Operations is the list of reclaim space operations to perform on the
	// target volume. "Node" issues the node-side reclaim, which requires the
	// volume to be attached to a node, and "Controller" issues the
	// controller-side reclaim. At least one operation must be specified.
	// If not specified, defaults to ["Node"]; the controller-side reclaim is
	// opt-in because it is redundant once the node-side reclaim (for example,
	// fstrim/discard) has run, and for some drivers it is costly or unsafe on
	// in-use volumes.
	// +optional
	// +kubebuilder:default:={Node}
	// +kubebuilder:validation:MinItems=1
	// +listType=set
	Operations []ReclaimSpaceOperation `json:"operations,omitempty"`
}

// EffectiveOperations returns a copy of the operations that should be
// performed. When Operations is empty, the default is returned.
func (s ReclaimSpaceJobSpec) EffectiveOperations() []ReclaimSpaceOperation {
	if len(s.Operations) == 0 {
		return DefaultReclaimSpaceOperations()
	}

	return slices.Clone(s.Operations)
}

// ReclaimSpaceJobStatus defines the observed state of ReclaimSpaceJob
type ReclaimSpaceJobStatus struct {
	// Result indicates the result of ReclaimSpaceJob.
	Result OperationResult `json:"result,omitempty"`

	// Message contains any message from the ReclaimSpaceJob.
	Message string `json:"message,omitempty"`

	// ReclaimedSpace indicates the amount of space reclaimed.
	ReclaimedSpace *resource.Quantity `json:"reclaimedSpace,omitempty"`

	// Operations reports the progress of each requested reclaim space
	// operation. Once the controller starts processing the job, there is one
	// entry for every requested operation (spec.operations, or the default).
	// +optional
	// +listType=map
	// +listMapKey=operation
	Operations []ReclaimSpaceOperationStatus `json:"operations,omitempty"`

	// Conditions are the list of conditions and their status.
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Retries indicates the number of times the operation is retried.
	Retries        int32        `json:"retries,omitempty"`
	StartTime      *metav1.Time `json:"startTime,omitempty"`
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:JSONPath=".metadata.namespace",name=Namespace,type=string
//+kubebuilder:printcolumn:JSONPath=".metadata.creationTimestamp",name=Age,type=date
//+kubebuilder:printcolumn:JSONPath=".status.retries",name=Retries,type=integer
//+kubebuilder:printcolumn:JSONPath=".status.result",name=Result,type=string
//+kubebuilder:printcolumn:JSONPath=".status.reclaimedSpace",name=ReclaimedSpace,type=string,priority=1
// The operations list is immutable on ReclaimSpaceJob. The rule is declared on
// the root type rather than on ReclaimSpaceJobSpec.Operations because the spec
// is embedded in ReclaimSpaceJobTemplateSpec: a field-level rule would also
// make the operations immutable in ReclaimSpaceCronJob, where users must be
// able to update the job template for future runs.
//+kubebuilder:validation:XValidation:rule="!has(oldSelf.spec.operations) || self.spec.operations == oldSelf.spec.operations",message="operations is immutable",fieldPath=".spec.operations"

// ReclaimSpaceJob is the Schema for the reclaimspacejobs API
type ReclaimSpaceJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	Spec ReclaimSpaceJobSpec `json:"spec"`

	Status ReclaimSpaceJobStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ReclaimSpaceJobList contains a list of ReclaimSpaceJob
type ReclaimSpaceJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ReclaimSpaceJob `json:"items"`
}
