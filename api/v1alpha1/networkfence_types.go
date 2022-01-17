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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type FencingOperationResult string

const (
	// FencingOperationResultSucceeded represents the Succeeded operation state.
	FencingOperationResultSucceeded FencingOperationResult = "Succeeded"

	// FencingOperationResultFailed represents the Failed operation state.
	FencingOperationResultFailed FencingOperationResult = "Failed"
)

// SecretSpec defines the secrets to be used for the network fencing operation.
type SecretSpec struct {
	// Name specifies the name of the secret.
	Name string `json:"name,omitempty"`

	// Namespace specifies the namespace in which the secret
	// is located.
	Namespace string `json:"namespace,omitempty"`
}

// NetworkFenceSpec defines the desired state of NetworkFence
type NetworkFenceSpec struct {
	// Driver contains  the name of CSI driver.
	// +kubebuilder:validation:Required
	Driver string `json:"driver"`

	// Cidrs contains a list of CIDR blocks, which are required to be fenced.
	// +kubebuilder:validation:Required
	Cidrs []string `json:"cidrs"`

	// Secret is a kubernetes secret, which is required to perform the fence/unfence operation.
	Secret SecretSpec `json:"secret,omitempty"`

	// Parameters is used to pass additional parameters to the CSI driver.
	Parameters map[string]string `json:"parameters,omitempty"`
}

// NetworkFenceStatus defines the observed state of NetworkFence
type NetworkFenceStatus struct {
	// Result indicates the result of NetworkFence operation.
	Result FencingOperationResult `json:"result,omitempty"`

	// Message contains any message from the NetworkFence operation.
	Message string `json:"message,omitempty"`

	// Conditions are the list of conditions and their status.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Driver",type="string",JSONPath=".spec.driver"
//+kubebuilder:printcolumn:name="Cidrs",type="string",JSONPath=".spec.cidrs"
//+kubebuilder:printcolumn:JSONPath=".metadata.creationTimestamp",name=Age,type=date
//+kubebuilder:printcolumn:JSONPath=".status.result",name=Result,type=string
//+kubebuilder:resource:path=networkfences,scope=Cluster,singular=networkfence

// NetworkFence is the Schema for the networkfences API
type NetworkFence struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkFenceSpec   `json:"spec,omitempty"`
	Status NetworkFenceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// NetworkFenceList contains a list of NetworkFence
type NetworkFenceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NetworkFence `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NetworkFence{}, &NetworkFenceList{})
}
