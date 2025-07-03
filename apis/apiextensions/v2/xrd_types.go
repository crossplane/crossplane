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

package v2

import (
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"

	"github.com/crossplane/crossplane/apis/apiextensions/shared"
)

// CompositeResourceDefinitionSpec specifies the desired state of the definition.
// +kubebuilder:validation:XValidation:rule="!has(self.claimNames) || self.scope == 'LegacyCluster'",message="Claims aren't supported in apiextensions.crossplane.io/v2"
// +kubebuilder:validation:XValidation:rule="!has(self.connectionSecretKeys) || self.scope == 'LegacyCluster'",message="XR connection secrets aren't supported in apiextensions.crossplane.io/v2"
type CompositeResourceDefinitionSpec struct {
	// Group specifies the API group of the defined composite resource.
	// Composite resources are served under `/apis/<group>/...`. Must match the
	// name of the XRD (in the form `<names.plural>.<group>`).
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	Group string `json:"group"`

	// Names specifies the resource and kind names of the defined composite
	// resource.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	// +kubebuilder:validation:XValidation:rule="self.plural == self.plural.lowerAscii()",message="Plural name must be lowercase"
	// +kubebuilder:validation:XValidation:rule="!has(self.singular) || self.singular == self.singular.lowerAscii()",message="Singular name must be lowercase"
	Names extv1.CustomResourceDefinitionNames `json:"names"`

	// Scope of the defined composite resource. Namespaced composite resources
	// are scoped to a single namespace. Cluster scoped composite resource exist
	// outside the scope of any namespace.
	// +kubebuilder:validation:Enum=Namespaced;Cluster
	// +kubebuilder:default=Namespaced
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	Scope shared.CompositeResourceScope `json:"scope,omitempty"`

	// DefaultCompositionRef refers to the Composition resource that will be used
	// in case no composition selector is given.
	// +optional
	DefaultCompositionRef *CompositionReference `json:"defaultCompositionRef,omitempty"`

	// EnforcedCompositionRef refers to the Composition resource that will be used
	// by all composite instances whose schema is defined by this definition.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
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

	// ClaimNames specifies the names of an optional composite resource claim.
	// When claim names are specified Crossplane will create a namespaced
	// 'composite resource claim' CRD that corresponds to the defined composite
	// resource. This composite resource claim acts as a namespaced proxy for
	// the composite resource; creating, updating, or deleting the claim will
	// create, update, or delete a corresponding composite resource. You may add
	// claim names to an existing CompositeResourceDefinition, but they cannot
	// be changed or removed once they have been set.
	//
	// Deprecated: Claims aren't supported in apiextensions.crossplane.io/v2.
	// +optional
	ClaimNames *extv1.CustomResourceDefinitionNames `json:"claimNames,omitempty"`

	// DefaultCompositeDeletePolicy is the policy used when deleting the Composite
	// that is associated with the Claim if no policy has been specified.
	//
	// Deprecated: Claims aren't supported in apiextensions.crossplane.io/v2.
	// +optional
	DefaultCompositeDeletePolicy *xpv1.CompositeDeletePolicy `json:"defaultCompositeDeletePolicy,omitempty"`

	// ConnectionSecretKeys is the list of connection secret keys the
	// defined XR can publish. If the list is empty, all keys will be
	// published. If the list isn't empty, any connection secret keys that
	// don't appear in the list will be filtered out. Only LegacyCluster XRs
	// support connection secrets.
	//
	// Deprecated: XR connection secrets aren't supported in
	// apiextensions.crossplane.io/v2. Compose a secret instead.
	// +optional
	ConnectionSecretKeys []string `json:"connectionSecretKeys,omitempty"`
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
	OpenAPIV3Schema runtime.RawExtension `json:"openAPIV3Schema,omitempty"` //nolint:tagliatelle // False positive. Linter thinks it should be Apiv3, not APIV3.
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
// +genclient
// +genclient:nonNamespaced

// A CompositeResourceDefinition defines the schema for a new custom Kubernetes
// API.
//
// Read the Crossplane documentation for
// [more information about CustomResourceDefinitions](https://docs.crossplane.io/latest/concepts/composite-resource-definitions).
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

// SetConditions delegates to Status.SetConditions.
// Implements Conditioned.SetConditions.
func (c *CompositeResourceDefinition) SetConditions(cs ...xpv1.Condition) {
	c.Status.SetConditions(cs...)
}

// GetCondition delegates to Status.GetCondition.
// Implements Conditioned.GetCondition.
func (c *CompositeResourceDefinition) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return c.Status.GetCondition(ct)
}

// +kubebuilder:object:root=true

// CompositeResourceDefinitionList contains a list of CompositeResourceDefinitions.
type CompositeResourceDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []CompositeResourceDefinition `json:"items"`
}

// GetCompositeGroupVersionKind returns the schema.GroupVersionKind of the CRD for
// the composite resource this CompositeResourceDefinition defines.
func (c *CompositeResourceDefinition) GetCompositeGroupVersionKind() schema.GroupVersionKind {
	v := ""

	for _, vr := range c.Spec.Versions {
		if vr.Referenceable {
			v = vr.Name
		}
	}

	return schema.GroupVersionKind{Group: c.Spec.Group, Version: v, Kind: c.Spec.Names.Kind}
}

// OffersClaim is true when a CompositeResourceDefinition offers a claim for the
// composite resource it defines.
func (c *CompositeResourceDefinition) OffersClaim() bool {
	return c.Spec.ClaimNames != nil
}

// GetClaimGroupVersionKind returns the schema.GroupVersionKind of the CRD for
// the composite resource claim this CompositeResourceDefinition defines. An
// empty GroupVersionKind is returned if the CompositeResourceDefinition does
// not offer a claim.
func (c *CompositeResourceDefinition) GetClaimGroupVersionKind() schema.GroupVersionKind {
	if !c.OffersClaim() {
		return schema.GroupVersionKind{}
	}

	v := ""

	for _, vr := range c.Spec.Versions {
		if vr.Referenceable {
			v = vr.Name
		}
	}

	return schema.GroupVersionKind{Group: c.Spec.Group, Version: v, Kind: c.Spec.ClaimNames.Kind}
}

// GetConnectionSecretKeys returns the set of allowed keys to filter the connection
// secret.
func (c *CompositeResourceDefinition) GetConnectionSecretKeys() []string {
	return c.Spec.ConnectionSecretKeys
}

// TypeReference is used to refer to a type for declaring compatibility.
type TypeReference struct {
	// APIVersion of the type.
	APIVersion string `json:"apiVersion"`

	// Kind of the type.
	Kind string `json:"kind"`
}

// TypeReferenceTo returns a reference to the supplied GroupVersionKind.
func TypeReferenceTo(gvk schema.GroupVersionKind) TypeReference {
	return TypeReference{APIVersion: gvk.GroupVersion().String(), Kind: gvk.Kind}
}
