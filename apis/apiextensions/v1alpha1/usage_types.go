// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// ResourceRef is a reference to a resource.
type ResourceRef struct {
	// Name of the referent.
	Name string `json:"name"`
}

// ResourceSelector is a selector to a resource.
type ResourceSelector struct {
	// MatchLabels ensures an object with matching labels is selected.
	MatchLabels map[string]string `json:"matchLabels,omitempty"`

	// MatchControllerRef ensures an object with the same controller reference
	// as the selecting object is selected.
	MatchControllerRef *bool `json:"matchControllerRef,omitempty"`
}

// Resource defines a cluster-scoped resource.
type Resource struct {
	// API version of the referent.
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`
	// Kind of the referent.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
	// +optional
	Kind string `json:"kind,omitempty"`
	// Reference to the resource.
	// +optional
	ResourceRef *ResourceRef `json:"resourceRef,omitempty"`
	// Selector to the resource.
	// This field will be ignored if ResourceRef is set.
	// +optional
	ResourceSelector *ResourceSelector `json:"resourceSelector,omitempty"`
}

// UsageSpec defines the desired state of Usage.
type UsageSpec struct {
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
}

// UsageStatus defines the observed state of Usage.
type UsageStatus struct {
	xpv1.ConditionedStatus `json:",inline"`
}

// A Usage defines a deletion blocking relationship between two resources.
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="DETAILS",type="string",JSONPath=".metadata.annotations.crossplane\\.io/usage-details"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Cluster,categories=crossplane
// +kubebuilder:subresource:status
type Usage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// +kubebuilder:validation:XValidation:rule="has(self.by) || has(self.reason)",message="either \"spec.by\" or \"spec.reason\" must be specified."
	Spec   UsageSpec   `json:"spec"`
	Status UsageStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// UsageList contains a list of Usage.
type UsageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Usage `json:"items"`
}
