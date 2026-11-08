/*
Copyright 2026 The Crossplane Authors.

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

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

// ExternalInventoryScope is one set of external resources, identified by the
// spec.forProvider fields that scope them. Its keys are field names and its
// values are field values. An empty scope is the top level of a kind that is
// not scoped.
type ExternalInventoryScope map[string]string

// ExternalInventoryItem is one external resource published by a provider.
type ExternalInventoryItem struct {
	// ExternalName is the identifier of the resource in the external system. It
	// is the value used in the crossplane.io/external-name annotation.
	// +kubebuilder:validation:Required
	ExternalName string `json:"externalName"`

	// Scope holds the spec.forProvider fields, beyond the external name, that the
	// provider's Observe needs to locate the resource.
	// +kubebuilder:validation:Optional
	Scope ExternalInventoryScope `json:"scope,omitempty"`
}

// ExternalInventoryScopeStatus is the result of listing one scope.
type ExternalInventoryScopeStatus struct {
	// Scope is the scope that was listed.
	// +kubebuilder:validation:Optional
	Scope ExternalInventoryScope `json:"scope,omitempty"`

	// Items are the external resources found in this scope so far.
	// +kubebuilder:validation:Optional
	Items []ExternalInventoryItem `json:"items,omitempty"`

	// NextPageToken is set while this scope is only partially listed. The
	// provider resumes from this token on its next run.
	// +kubebuilder:validation:Optional
	NextPageToken string `json:"nextPageToken,omitempty"`

	// Error describes why listing this scope failed. It is empty when the scope
	// was listed successfully.
	// +kubebuilder:validation:Optional
	Error string `json:"error,omitempty"`
}

// ExternalInventorySpec describes which kind an inventory covers.
type ExternalInventorySpec struct {
	// MRDRef references the ManagedResourceDefinition whose kind is listed.
	// +kubebuilder:validation:Required
	MRDRef xpv2.TypedReference `json:"mrdRef"`

	// ProviderConfigRef is the ProviderConfig the provider used to list the
	// resources. The listing is scoped to that account, project or credentials.
	// +kubebuilder:validation:Optional
	ProviderConfigRef *xpv2.Reference `json:"providerConfigRef,omitempty"`
}

// ExternalInventoryStatus is the most recent listing, published by the provider.
type ExternalInventoryStatus struct {
	// LastListed is when the provider last completed a listing of this kind.
	// +kubebuilder:validation:Optional
	LastListed *metav1.Time `json:"lastListed,omitempty"`

	// Scopes holds the result of listing each scope of the kind. An unscoped
	// kind has a single entry with an empty scope.
	// +kubebuilder:validation:Optional
	Scopes []ExternalInventoryScopeStatus `json:"scopes,omitempty"`
}

// +kubebuilder:object:root=true
// +genclient
// +genclient:nonNamespaced

// ExternalInventory records the external resources of one managed resource
// kind, as listed by its provider. One ExternalInventory exists per
// ManagedResourceDefinition and is written by the provider, not by Crossplane.
//
// +kubebuilder:printcolumn:name="MRD",type="string",JSONPath=".spec.mrdRef.name"
// +kubebuilder:printcolumn:name="LAST-LISTED",type="date",JSONPath=".status.lastListed"
// +kubebuilder:resource:scope=Cluster,categories=crossplane,shortName=extinv
// +kubebuilder:subresource:status
type ExternalInventory struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ExternalInventorySpec   `json:"spec"`
	Status ExternalInventoryStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ExternalInventoryList contains a list of ExternalInventories.
type ExternalInventoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []ExternalInventory `json:"items"`
}
