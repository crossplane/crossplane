/*
Copyright 2020 The Crossplane Authors.

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
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// CompositeResourceDefinitionSpec specifies the desired state of the definition.
type CompositeResourceDefinitionSpec struct {
	// Group specifies the API group of the defined composite resource.
	// Composite resources are served under `/apis/<group>/...`. Must match the
	// name of the XRD (in the form `<names.plural>.<group>`).
	// +immutable
	Group string `json:"group"`

	// Names specifies the resource and kind names of the defined composite
	// resource.
	// +immutable
	Names extv1.CustomResourceDefinitionNames `json:"names"`

	// ClaimNames specifies the names of an optional composite resource claim.
	// When claim names are specified Crossplane will create a namespaced
	// 'composite resource claim' CRD that corresponds to the defined composite
	// resource. This composite resource claim acts as a namespaced proxy for
	// the composite resource; creating, updating, or deleting the claim will
	// create, update, or delete a corresponding composite resource. You may add
	// claim names to an existing CompositeResourceDefinition, but they cannot
	// be changed or removed once they have been set.
	// +immutable
	// +optional
	ClaimNames *extv1.CustomResourceDefinitionNames `json:"claimNames,omitempty"`

	// ConnectionSecretKeys is the list of keys that will be exposed to the end
	// user of the defined kind.
	// If the list is empty, all keys will be published.
	// +optional
	ConnectionSecretKeys []string `json:"connectionSecretKeys,omitempty"`

	// DefaultCompositeDeletePolicy is the policy used when deleting the Composite
	// that is associated with the Claim if no policy has been specified.
	// +optional
	// +kubebuilder:default=Background
	DefaultCompositeDeletePolicy *xpv1.CompositeDeletePolicy `json:"defaultCompositeDeletePolicy,omitempty"`

	// DefaultCompositionRef refers to the Composition resource that will be used
	// in case no composition selector is given.
	// +optional
	DefaultCompositionRef *CompositionReference `json:"defaultCompositionRef,omitempty"`

	// EnforcedCompositionRef refers to the Composition resource that will be used
	// by all composite instances whose schema is defined by this definition.
	// +optional
	// +immutable
	EnforcedCompositionRef *CompositionReference `json:"enforcedCompositionRef,omitempty"`

	// DefaultCompositionUpdatePolicy is the policy used when updating composites after a new
	// Composition Revision has been created if no policy has been specified on the composite.
	// +optional
	// +kubebuilder:default=Automatic
	DefaultCompositionUpdatePolicy *xpv1.UpdatePolicy `json:"defaultCompositionUpdatePolicy,omitempty"`

	// Versions is the list of all API versions of the defined composite
	// resource. Version names are used to compute the order in which served
	// versions are listed in API discovery. If the version string is
	// "kube-like", it will sort above non "kube-like" version strings, which
	// are ordered lexicographically. "Kube-like" versions start with a "v",
	// then are followed by a number (the major version), then optionally the
	// string "alpha" or "beta" and another number (the minor version). These
	// are sorted first by GA > beta > alpha (where GA is a version with no
	// suffix such as beta or alpha), and then by comparing major version, then
	// minor version. An example sorted list of versions: v10, v2, v1, v11beta2,
	// v10beta3, v3beta1, v12alpha1, v11alpha2, foo1, foo10.
	Versions []CompositeResourceDefinitionVersion `json:"versions"`

	// Conversion defines all conversion settings for the defined Composite resource.
	// +optional
	Conversion *extv1.CustomResourceConversion `json:"conversion,omitempty"`

	// Metadata specifies the desired metadata for the defined composite resource and claim CRD's.
	// +optional
	Metadata *CompositeResourceDefinitionSpecMetadata `json:"metadata,omitempty"`
}

// A CompositionReference references a Composition.
type CompositionReference struct {
	// Name of the Composition.
	Name string `json:"name"`
}

// CompositeResourceDefinitionSpecMetadata specifies the desired metadata of the defined composite resource and claim CRD's.
type CompositeResourceDefinitionSpecMetadata struct {
	// Map of string keys and values that can be used to organize and categorize
	// (scope and select) objects. May match selectors of replication controllers
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels
	// and services.
	// These labels are added to the composite resource and claim CRD's in addition
	// to any labels defined by `CompositionResourceDefinition` `metadata.labels`.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations is an unstructured key value map stored with a resource that may be
	// set by external tools to store and retrieve arbitrary metadata. They are not
	// queryable and should be preserved when modifying objects.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// CompositeResourceDefinitionVersion describes a version of an XR.
type CompositeResourceDefinitionVersion struct {
	// Name of this version, e.g. “v1”, “v2beta1”, etc. Composite resources are
	// served under this version at `/apis/<group>/<version>/...` if `served` is
	// true.
	Name string `json:"name"`

	// Referenceable specifies that this version may be referenced by a
	// Composition in order to configure which resources an XR may be composed
	// of. Exactly one version must be marked as referenceable; all Compositions
	// must target only the referenceable version. The referenceable version
	// must be served. It's mapped to the CRD's `spec.versions[*].storage` field.
	Referenceable bool `json:"referenceable"`

	// Served specifies that this version should be served via REST APIs.
	Served bool `json:"served"`

	// The deprecated field specifies that this version is deprecated and should
	// not be used.
	// +optional
	Deprecated *bool `json:"deprecated,omitempty"`

	// DeprecationWarning specifies the message that should be shown to the user
	// when using this version.
	// +optional
	// +kubebuilder:validation:MaxLength=256
	DeprecationWarning *string `json:"deprecationWarning,omitempty"`

	// Schema describes the schema used for validation, pruning, and defaulting
	// of this version of the defined composite resource. Fields required by all
	// composite resources will be injected into this schema automatically, and
	// will override equivalently named fields in this schema. Omitting this
	// schema results in a schema that contains only the fields required by all
	// composite resources.
	// +optional
	Schema *CompositeResourceValidation `json:"schema,omitempty"`

	// AdditionalPrinterColumns specifies additional columns returned in Table
	// output. If no columns are specified, a single column displaying the age
	// of the custom resource is used. See the following link for details:
	// https://kubernetes.io/docs/reference/using-api/api-concepts/#receiving-resources-as-tables
	// +optional
	AdditionalPrinterColumns []extv1.CustomResourceColumnDefinition `json:"additionalPrinterColumns,omitempty"`
}

// CompositeResourceValidation is a list of validation methods for a composite
// resource.
type CompositeResourceValidation struct {
	// OpenAPIV3Schema is the OpenAPI v3 schema to use for validation and
	// pruning.
	// +kubebuilder:pruning:PreserveUnknownFields
	OpenAPIV3Schema runtime.RawExtension `json:"openAPIV3Schema,omitempty"`
}

// CompositeResourceDefinitionStatus shows the observed state of the definition.
type CompositeResourceDefinitionStatus struct {
	xpv1.ConditionedStatus `json:",inline"`

	// Controllers represents the status of the controllers that power this
	// composite resource definition.
	Controllers CompositeResourceDefinitionControllerStatus `json:"controllers,omitempty"`
}

// CompositeResourceDefinitionControllerStatus shows the observed state of the
// controllers that power the definition.
type CompositeResourceDefinitionControllerStatus struct {
	// The CompositeResourceTypeRef is the type of composite resource that
	// Crossplane is currently reconciling for this definition. Its version will
	// eventually become consistent with the definition's referenceable version.
	// Note that clients may interact with any served type; this is simply the
	// type that Crossplane interacts with.
	CompositeResourceTypeRef TypeReference `json:"compositeResourceType,omitempty"`

	// The CompositeResourceClaimTypeRef is the type of composite resource claim
	// that Crossplane is currently reconciling for this definition. Its version
	// will eventually become consistent with the definition's referenceable
	// version. Note that clients may interact with any served type; this is
	// simply the type that Crossplane interacts with.
	CompositeResourceClaimTypeRef TypeReference `json:"compositeResourceClaimType,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +genclient
// +genclient:nonNamespaced

// A CompositeResourceDefinition defines a new kind of composite infrastructure
// resource. The new resource is composed of other composite or managed
// infrastructure resources.
// +kubebuilder:printcolumn:name="ESTABLISHED",type="string",JSONPath=".status.conditions[?(@.type=='Established')].status"
// +kubebuilder:printcolumn:name="OFFERED",type="string",JSONPath=".status.conditions[?(@.type=='Offered')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=crossplane,shortName=xrd;xrds
type CompositeResourceDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CompositeResourceDefinitionSpec   `json:"spec,omitempty"`
	Status CompositeResourceDefinitionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CompositeResourceDefinitionList contains a list of CompositeResourceDefinitions.
type CompositeResourceDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CompositeResourceDefinition `json:"items"`
}

// GetCompositeGroupVersionKind returns the schema.GroupVersionKind of the CRD for
// the composite resource this CompositeResourceDefinition defines.
func (in CompositeResourceDefinition) GetCompositeGroupVersionKind() schema.GroupVersionKind {
	v := ""
	for _, vr := range in.Spec.Versions {
		if vr.Referenceable {
			v = vr.Name
		}
	}

	return schema.GroupVersionKind{Group: in.Spec.Group, Version: v, Kind: in.Spec.Names.Kind}
}

// OffersClaim is true when a CompositeResourceDefinition offers a claim for the
// composite resource it defines.
func (in CompositeResourceDefinition) OffersClaim() bool {
	return in.Spec.ClaimNames != nil
}

// GetClaimGroupVersionKind returns the schema.GroupVersionKind of the CRD for
// the composite resource claim this CompositeResourceDefinition defines. An
// empty GroupVersionKind is returned if the CompositeResourceDefinition does
// not offer a claim.
func (in CompositeResourceDefinition) GetClaimGroupVersionKind() schema.GroupVersionKind {
	if !in.OffersClaim() {
		return schema.GroupVersionKind{}
	}

	v := ""
	for _, vr := range in.Spec.Versions {
		if vr.Referenceable {
			v = vr.Name
		}
	}

	return schema.GroupVersionKind{Group: in.Spec.Group, Version: v, Kind: in.Spec.ClaimNames.Kind}
}

// GetConnectionSecretKeys returns the set of allowed keys to filter the connection
// secret.
func (in *CompositeResourceDefinition) GetConnectionSecretKeys() []string {
	return in.Spec.ConnectionSecretKeys
}
