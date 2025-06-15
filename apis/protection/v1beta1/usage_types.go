/*
Copyright 2025 The Crossplane Authors.

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

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// ResourceRef is a reference to a resource. It's suitable for cluster
// scoped resources, or resources in the same namespace as the Usage.
type ResourceRef struct {
	// Name of the referent.
	Name string `json:"name"`
}

// ResourceSelector is a selector of a resource. It's suitable for cluster
// scoped resources, or resources in the same namespace as the Usage.
type ResourceSelector struct {
	// MatchLabels ensures an object with matching labels is selected.
	MatchLabels map[string]string `json:"matchLabels,omitempty"`

	// MatchControllerRef ensures an object with the same controller reference
	// as the selecting object is selected.
	MatchControllerRef *bool `json:"matchControllerRef,omitempty"`
}

// Resource refers to an arbitrary resource. It's suitable for cluster scoped
// resources, or resources in the same namespace as the Usage.
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

// NamespacedResourceRef is a reference to a resource. It supports an optional
// namespace. It's suitable for cluster scoped resources, or resources in the
// same namespace as the Usage.
type NamespacedResourceRef struct {
	// Name of the referent.
	Name string `json:"name"`

	// Namespace of the referent.
	// +optional
	Namespace *string `json:"namespace,omitempty"`
}

// NamespacedResourceSelector is a selector of a resource. It's suitable for
// cluster scoped resources, resources in the same namespace as the Usage, or
// resources in a different namespace.
type NamespacedResourceSelector struct {
	// MatchLabels ensures an object with matching labels is selected.
	MatchLabels map[string]string `json:"matchLabels,omitempty"`

	// MatchControllerRef ensures an object with the same controller reference
	// as the selecting object is selected.
	// +optional
	MatchControllerRef *bool `json:"matchControllerRef,omitempty"`

	// Namespace ensures an object in the supplied namespace is selected.
	// Omit namespace to only match resources in the Usage's namespace.
	// +optional
	Namespace *string `json:"namespace,omitempty"`
}

// NamespacedResource refers to an arbitrary resource. Despite the name, the
// resource doesn't have to be namespaced. It's different from Resource because
// it supports namespaced resources. It's suitable for cluster scoped resources,
// resources in the same namespace as the Usage, or resources in a different
// namespace.
type NamespacedResource struct {
	// API version of the referent.
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`

	// Kind of the referent.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
	// +optional
	Kind string `json:"kind,omitempty"`

	// Reference to the resource.
	// +optional
	ResourceRef *NamespacedResourceRef `json:"resourceRef,omitempty"`

	// Selector to the resource.
	// This field will be ignored if ResourceRef is set.
	// +optional
	ResourceSelector *NamespacedResourceSelector `json:"resourceSelector,omitempty"`
}

// UsageSpec defines the desired state of Usage.
// +kubebuilder:validation:XValidation:rule="has(self.by) || has(self.reason)",message="either \"spec.by\" or \"spec.reason\" must be specified."
// +kubebuilder:validation:XValidation:rule="has(self.by) || (!has(self.of.resourceRef) || !has(self.of.resourceRef.__namespace__)) && (!has(self.of.resourceSelector) || !has(self.of.resourceSelector.__namespace__))",message="cross-namespace \"spec.of\" is not allowed without \"spec.by\" resource."
type UsageSpec struct {
	// Of is the resource that is "being used".
	// +kubebuilder:validation:XValidation:rule="has(self.resourceRef) || has(self.resourceSelector)",message="either a resource reference or a resource selector should be set."
	Of NamespacedResource `json:"of"`

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

// UsageStatus defines the observed state of Usage.
type UsageStatus struct {
	xpv1.ConditionedStatus `json:",inline"`
}

// A Usage defines a deletion blocking relationship between two resources.
//
// Usages prevent accidental deletion of a single resource or deletion of
// resources with dependent resources.
//
// Read the Crossplane documentation for
// [more information about Compositions](https://docs.crossplane.io/latest/concepts/usages).
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +genclient
// +kubebuilder:printcolumn:name="DETAILS",type="string",JSONPath=".metadata.annotations.crossplane\\.io/usage-details"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:categories=crossplane
// +kubebuilder:subresource:status
type Usage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              UsageSpec   `json:"spec"`
	Status            UsageStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// UsageList contains a list of Usage.
type UsageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Usage `json:"items"`
}
