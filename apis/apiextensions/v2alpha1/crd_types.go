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

package v2alpha1

import (
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// The following is a fork of upstream CRDs from
// k8s.io/apiextensions-apiserver@v0.31.2/pkg/apis/apiextensions/v1/types.go
// We need to copy it here because the kubebuilder generator can not deal with
// `schema type at crd.spec.versions[].schema`. So to work around this, we
// duplicate the entire structs where we have to:
// - CustomResourceDefinitionSpec.Versions points to our copy.
// - CustomResourceDefinitionVersion.Schema. points to our copy.
// - CustomResourceValidation.OpenAPIV3Schema points to just runtime.RawExtension.
//
// It is extremely important that we track the upstream type fields of
// CustomResourceDefinitionSpec exactly.

// CustomResourceDefinitionSpec describes how a user wants their resource to appear.
type CustomResourceDefinitionSpec struct {
	// Group is the API group of the defined custom resource.
	// The custom resources are served under `/apis/<group>/...`.
	// Must match the name of the CustomResourceDefinition (in the form `<names.plural>.<group>`).
	Group string `json:"group"`
	// Names specify the resource and kind names for the custom resource.
	Names extv1.CustomResourceDefinitionNames `json:"names"`
	// Scope indicates whether the defined custom resource is cluster- or namespace-scoped.
	// Allowed values are `Cluster` and `Namespaced`.
	Scope extv1.ResourceScope `json:"scope"`
	// Versions is the list of all API versions of the defined custom resource.
	// Version names are used to compute the order in which served versions are listed in API discovery.
	// If the version string is "kube-like", it will sort above non "kube-like" version strings, which are ordered
	// lexicographically. "Kube-like" versions start with a "v", then are followed by a number (the major version),
	// then optionally the string "alpha" or "beta" and another number (the minor version). These are sorted first
	// by GA > beta > alpha (where GA is a version with no suffix such as beta or alpha), and then by comparing
	// major version, then minor version. An example sorted list of versions:
	// v10, v2, v1, v11beta2, v10beta3, v3beta1, v12alpha1, v11alpha2, foo1, foo10.
	// +listType=atomic
	Versions []CustomResourceDefinitionVersion `json:"versions"`

	// Conversion defines conversion settings for the CRD.
	// +optional
	Conversion *extv1.CustomResourceConversion `json:"conversion,omitempty"`

	// PreserveUnknownFields indicates that object fields which are not specified
	// in the OpenAPI schema should be preserved when persisting to storage.
	// apiVersion, kind, metadata and known fields inside metadata are always preserved.
	// This field is deprecated in favor of setting `x-preserve-unknown-fields` to true in `spec.versions[*].schema.openAPIV3Schema`.
	// See https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#field-pruning for details.
	// +optional
	PreserveUnknownFields bool `json:"preserveUnknownFields,omitempty"`
}

// CustomResourceDefinitionVersion describes a version for CRD.
type CustomResourceDefinitionVersion struct {
	// Name is the version name, e.g. “v1”, “v2beta1”, etc.
	// The custom resources are served under this version at `/apis/<group>/<version>/...` if `served` is true.
	Name string `json:"name"`
	// Served is a flag enabling/disabling this version from being served via REST APIs
	Served bool `json:"served"`
	// Storage indicates this version should be used when persisting custom resources to storage.
	// There must be exactly one version with storage=true.
	Storage bool `json:"storage"`
	// Voldemort indicates this version of the custom resource API is deprecated.
	// When set to true, API requests to this version receive a warning header in the server response.
	// Defaults to false.
	// +optional
	Voldemort bool `json:"deprecated,omitempty"`
	// DeprecationWarning overrides the default warning returned to API clients.
	// May only be set when `deprecated` is true.
	// The default warning indicates this version is deprecated and recommends use
	// of the newest served version of equal or greater stability, if one exists.
	// +optional
	DeprecationWarning *string `json:"deprecationWarning,omitempty"`
	// Schema describes the schema used for validation, pruning, and defaulting of this version of the custom resource.
	// +optional
	Schema *CustomResourceValidation `json:"schema,omitempty" protobuf:"bytes,4,opt,name=schema"`
	// Subresources specify what subresources this version of the defined custom resource have.
	// +optional
	Subresources *extv1.CustomResourceSubresources `json:"subresources,omitempty"`
	// AdditionalPrinterColumns specifies additional columns returned in Table output.
	// See https://kubernetes.io/docs/reference/using-api/api-concepts/#receiving-resources-as-tables for details.
	// If no columns are specified, a single column displaying the age of the custom resource is used.
	// +optional
	// +listType=atomic
	AdditionalPrinterColumns []extv1.CustomResourceColumnDefinition `json:"additionalPrinterColumns,omitempty"`

	// SelectableFields specifies paths to fields that may be used as field selectors.
	// A maximum of 8 selectable fields are allowed.
	// See https://kubernetes.io/docs/concepts/overview/working-with-objects/field-selectors
	//
	// +featureGate=CustomResourceFieldSelectors
	// +optional
	// +listType=atomic
	SelectableFields []extv1.SelectableField `json:"selectableFields,omitempty"`
}

// CustomResourceValidation is a list of validation methods for a custom
// resource.
type CustomResourceValidation struct {
	// OpenAPIV3Schema is the OpenAPI v3 schema to use for validation and
	// pruning.
	// +kubebuilder:pruning:PreserveUnknownFields
	OpenAPIV3Schema runtime.RawExtension `json:"openAPIV3Schema,omitempty"` //nolint:tagliatelle // False positive. Linter thinks it should be Apiv3, not APIV3.
}
