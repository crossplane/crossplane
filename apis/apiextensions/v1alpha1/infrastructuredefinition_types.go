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

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
)

// InfrastructureDefinitionSpec specifies the desired state of the definition.
type InfrastructureDefinitionSpec struct {

	// ConnectionSecretKeys is the list of keys that will be exposed to the end
	// user of the defined kind.
	ConnectionSecretKeys []string `json:"connectionSecretKeys,omitempty"`

	// CRDSpecTemplate is the base CRD template. The final CRD will have additional
	// fields to the base template to accommodate Crossplane machinery.
	CRDSpecTemplate CRDSpecTemplate `json:"crdSpecTemplate,omitempty"`
}

// A CRDSpecTemplate is a template for a v1beta1.CustomResourceDefinitionSpec.
type CRDSpecTemplate struct {
	// group is the API group of the defined custom resource.
	// The custom resources are served under `/apis/<group>/...`.
	// Must match the name of the CustomResourceDefinition (in the form `<names.plural>.<group>`).
	Group string `json:"group"`

	// version is the API version of the defined custom resource.
	// The custom resources are served under `/apis/<group>/<version>/...`.
	// Must match the name of the first item in the `versions` list if `version` and `versions` are both specified.
	// Optional if `versions` is specified.
	// Deprecated: use `versions` instead.
	// +optional
	Version string `json:"version,omitempty"`

	// names specify the resource and kind names for the custom resource.
	Names v1beta1.CustomResourceDefinitionNames `json:"names"`

	// validation describes the schema used for validation and pruning of the custom resource.
	// If present, this validation schema is used to validate all versions.
	// Top-level and per-version schemas are mutually exclusive.
	// +optional
	Validation *CustomResourceValidation `json:"validation,omitempty"`

	// additionalPrinterColumns specifies additional columns returned in Table output.
	// See https://kubernetes.io/docs/reference/using-api/api-concepts/#receiving-resources-as-tables for details.
	// If present, this field configures columns for all versions.
	// Top-level and per-version columns are mutually exclusive.
	// If no top-level or per-version columns are specified, a single column displaying the age of the custom resource is used.
	// +optional
	AdditionalPrinterColumns []v1beta1.CustomResourceColumnDefinition `json:"additionalPrinterColumns,omitempty"`
}

// CustomResourceDefinitionVersion describes a version for CRD.
type CustomResourceDefinitionVersion struct {
	// name is the version name, e.g. “v1”, “v2beta1”, etc.
	// The custom resources are served under this version at `/apis/<group>/<version>/...` if `served` is true.
	Name string `json:"name"`

	// served is a flag enabling/disabling this version from being served via REST APIs
	Served bool `json:"served"`

	// storage indicates this version should be used when persisting custom resources to storage.
	// There must be exactly one version with storage=true.
	Storage bool `json:"storage"`

	// schema describes the schema used for validation and pruning of this version of the custom resource.
	// Top-level and per-version schemas are mutually exclusive.
	// Per-version schemas must not all be set to identical values (top-level validation schema should be used instead).
	// +optional
	Schema *CustomResourceValidation `json:"schema,omitempty"`

	// additionalPrinterColumns specifies additional columns returned in Table output.
	// See https://kubernetes.io/docs/reference/using-api/api-concepts/#receiving-resources-as-tables for details.
	// Top-level and per-version columns are mutually exclusive.
	// Per-version columns must not all be set to identical values (top-level columns should be used instead).
	// If no top-level or per-version columns are specified, a single column displaying the age of the custom resource is used.
	// +optional
	AdditionalPrinterColumns []v1beta1.CustomResourceColumnDefinition `json:"additionalPrinterColumns,omitempty"`
}

// CustomResourceValidation is a list of validation methods for CustomResources.
type CustomResourceValidation struct {
	// openAPIV3Schema is the OpenAPI v3 schema to use for validation and pruning.
	// +optional
	OpenAPIV3Schema runtime.RawExtension `json:"openAPIV3Schema,omitempty"`
}

// InfrastructureDefinitionStatus shows the observed state of the definition.
type InfrastructureDefinitionStatus struct {
	v1alpha1.ConditionedStatus `json:",inline"`
}

// +kubebuilder:object:root=true

// An InfrastructureDefinition defines a new kind of composite infrastructure
// resource. The new resource is composed of other composite or managed
// infrastructure resources.
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories=crossplane
type InfrastructureDefinition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InfrastructureDefinitionSpec   `json:"spec,omitempty"`
	Status InfrastructureDefinitionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// InfrastructureDefinitionList contains a list of InfrastructureDefinitions.
type InfrastructureDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InfrastructureDefinition `json:"items"`
}

// GetDefinedGroupVersionKind returns the schema.GroupVersionKind of the CRD that this
// InfrastructureDefinition instance will define.
func (in InfrastructureDefinition) GetDefinedGroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   in.Spec.CRDSpecTemplate.Group,
		Version: in.Spec.CRDSpecTemplate.Version,
		Kind:    in.Spec.CRDSpecTemplate.Names.Kind,
	}
}

// GetConnectionSecretKeys returns the set of allowed keys to filter the connection
// secret.
func (in *InfrastructureDefinition) GetConnectionSecretKeys() []string {
	return in.Spec.ConnectionSecretKeys
}
