//go:build !goverter

/*
Copyright 2023 The Crossplane Authors.

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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// A ClusterUsage defines a deletion blocking relationship between two
// resources.
//
// Usages prevent accidental deletion of a single resource or deletion of
// resources with dependent resources.
//
// Read the Crossplane documentation for
// [more information about usages](https://docs.crossplane.io/latest/concepts/usages).
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="DETAILS",type="string",JSONPath=".metadata.annotations.crossplane\\.io/usage-details"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Cluster,categories=crossplane
// +kubebuilder:subresource:status
type ClusterUsage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ClusterUsageSpec `json:"spec"`
	Status            UsageStatus      `json:"status,omitempty"`
}

// ClusterUsageSpec defines the desired state of a ClusterUsage.
// +kubebuilder:validation:XValidation:rule="has(self.by) || has(self.reason)",message="either \"spec.by\" or \"spec.reason\" must be specified."
type ClusterUsageSpec struct {
	// Of is the resource that is "being used".
	// +kubebuilder:validation:XValidation:rule="has(self.resourceRef) || has(self.resourceSelector)",message="either a resource reference or a resource selector should be set."
	Of Resource `json:"of"`

	// By is the resource that is "using the other resource".
	// +optional
	// +kubebuilder:validation:XValidation:rule="has(self.resourceRef) || has(self.resourceSelector)",message="either a resource reference or a resource selector should be set."
	By *Resource `json:"by,omitempty"`

	// Reason is the reason for blocking deletion of the resource.
	// +optional
	Reason *string `json:"reason,omitempty"`

	// ReplayDeletion will trigger a deletion on the used resource during the deletion of the usage itself, if it was attempted to be deleted at least once.
	// +optional
	ReplayDeletion *bool `json:"replayDeletion,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterUsageList contains a list of Usage.
type ClusterUsageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterUsage `json:"items"`
}
