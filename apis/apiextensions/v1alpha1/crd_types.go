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
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
)

// NOTE(muvaf): The structs that v1beta1.JSONSchemaProps uses does not have manual
// jsontags and it has fields with float64 type which is not supported, so,
// controller-tools 0.2.4 cannot generate the validation for
// CustomResourceDefinitionSpec. This is why we had to copy the whole struct.
// For details, see https://github.com/kubernetes-sigs/controller-tools/issues/291

// FromShallow returns actual v1beta1.CustomResourceDefinitionSpec object from
// our shallow type.
func FromShallow(in CustomResourceDefinitionSpec) (*v1beta1.CustomResourceDefinitionSpec, error) {
	out := &v1beta1.CustomResourceDefinitionSpec{
		Group:                    in.Group,
		Version:                  in.Version,
		Names:                    in.Names,
		Subresources:             in.Subresources,
		AdditionalPrinterColumns: in.AdditionalPrinterColumns,
		Conversion:               in.Conversion,
		PreserveUnknownFields:    in.PreserveUnknownFields,
	}
	if in.Validation != nil {
		s := &v1beta1.JSONSchemaProps{}
		if err := json.Unmarshal(in.Validation.OpenAPIV3Schema.Raw, s); err != nil {
			return nil, err
		}
		out.Validation = &v1beta1.CustomResourceValidation{OpenAPIV3Schema: s}
	}
	for _, version := range in.Versions {
		v := v1beta1.CustomResourceDefinitionVersion{
			Name:                     version.Name,
			Served:                   version.Served,
			Storage:                  version.Storage,
			Subresources:             version.Subresources,
			AdditionalPrinterColumns: version.AdditionalPrinterColumns,
		}
		if version.Schema != nil {
			s := &v1beta1.JSONSchemaProps{}
			if err := json.Unmarshal(version.Schema.OpenAPIV3Schema.Raw, s); err != nil {
				return nil, err
			}
			v.Schema = &v1beta1.CustomResourceValidation{OpenAPIV3Schema: s}
		}
		out.Versions = append(out.Versions, v)
	}
	return out, nil
}

// IsEstablished is a helper function to check whether api-server is ready
// to accept the instances of registered CRD.
func IsEstablished(crd v1beta1.CustomResourceDefinition) bool {
	for _, c := range crd.Status.Conditions {
		if c.Type == v1beta1.Established {
			return c.Status == v1beta1.ConditionTrue
		}
	}
	return false
}

// CustomResourceDefinitionSpec is a shallow copy of actual v1beta1.CustomResourceDefinitionSpec
type CustomResourceDefinitionSpec struct {
	// group is the API group of the defined custom resource.
	// The custom resources are served under `/apis/<group>/...`.
	// Must match the name of the CustomResourceDefinition (in the form `<names.plural>.<group>`).
	Group string `json:"group" protobuf:"bytes,1,opt,name=group"`
	// version is the API version of the defined custom resource.
	// The custom resources are served under `/apis/<group>/<version>/...`.
	// Must match the name of the first item in the `versions` list if `version` and `versions` are both specified.
	// Optional if `versions` is specified.
	// Deprecated: use `versions` instead.
	// +optional
	Version string `json:"version,omitempty" protobuf:"bytes,2,opt,name=version"`
	// names specify the resource and kind names for the custom resource.
	Names v1beta1.CustomResourceDefinitionNames `json:"names" protobuf:"bytes,3,opt,name=names"`

	// NOTE(muvaf): Scope is already decided by the kind of the *Definition type.
	// scope indicates whether the defined custom resource is cluster- or namespace-scoped.
	// Allowed values are `Cluster` and `Namespaced`. Default is `Namespaced`.
	// Scope v1beta1.ResourceScope `json:"scope" protobuf:"bytes,4,opt,name=scope,casttype=ResourceScope"`

	// validation describes the schema used for validation and pruning of the custom resource.
	// If present, this validation schema is used to validate all versions.
	// Top-level and per-version schemas are mutually exclusive.
	// +optional
	Validation *CustomResourceValidation `json:"validation,omitempty" protobuf:"bytes,5,opt,name=validation"`
	// subresources specify what subresources the defined custom resource has.
	// If present, this field configures subresources for all versions.
	// Top-level and per-version subresources are mutually exclusive.
	// +optional
	Subresources *v1beta1.CustomResourceSubresources `json:"subresources,omitempty" protobuf:"bytes,6,opt,name=subresources"`
	// versions is the list of all API versions of the defined custom resource.
	// Optional if `version` is specified.
	// The name of the first item in the `versions` list must match the `version` field if `version` and `versions` are both specified.
	// Version names are used to compute the order in which served versions are listed in API discovery.
	// If the version string is "kube-like", it will sort above non "kube-like" version strings, which are ordered
	// lexicographically. "Kube-like" versions start with a "v", then are followed by a number (the major version),
	// then optionally the string "alpha" or "beta" and another number (the minor version). These are sorted first
	// by GA > beta > alpha (where GA is a version with no suffix such as beta or alpha), and then by comparing
	// major version, then minor version. An example sorted list of versions:
	// v10, v2, v1, v11beta2, v10beta3, v3beta1, v12alpha1, v11alpha2, foo1, foo10.
	// +optional
	Versions []CustomResourceDefinitionVersion `json:"versions,omitempty" protobuf:"bytes,7,rep,name=versions"`
	// additionalPrinterColumns specifies additional columns returned in Table output.
	// See https://kubernetes.io/docs/reference/using-api/api-concepts/#receiving-resources-as-tables for details.
	// If present, this field configures columns for all versions.
	// Top-level and per-version columns are mutually exclusive.
	// If no top-level or per-version columns are specified, a single column displaying the age of the custom resource is used.
	// +optional
	AdditionalPrinterColumns []v1beta1.CustomResourceColumnDefinition `json:"additionalPrinterColumns,omitempty" protobuf:"bytes,8,rep,name=additionalPrinterColumns"`

	// conversion defines conversion settings for the CRD.
	// +optional
	Conversion *v1beta1.CustomResourceConversion `json:"conversion,omitempty" protobuf:"bytes,9,opt,name=conversion"`

	// preserveUnknownFields indicates that object fields which are not specified
	// in the OpenAPI schema should be preserved when persisting to storage.
	// apiVersion, kind, metadata and known fields inside metadata are always preserved.
	// If false, schemas must be defined for all versions.
	// Defaults to true in v1beta for backwards compatibility.
	// Deprecated: will be required to be false in v1. Preservation of unknown fields can be specified
	// in the validation schema using the `x-kubernetes-preserve-unknown-fields: true` extension.
	// See https://kubernetes.io/docs/tasks/access-kubernetes-api/custom-resources/custom-resource-definitions/#pruning-versus-preserving-unknown-fields for details.
	// +optional
	PreserveUnknownFields *bool `json:"preserveUnknownFields,omitempty" protobuf:"varint,10,opt,name=preserveUnknownFields"`
}

// CustomResourceDefinitionVersion describes a version for CRD.
type CustomResourceDefinitionVersion struct {
	// name is the version name, e.g. “v1”, “v2beta1”, etc.
	// The custom resources are served under this version at `/apis/<group>/<version>/...` if `served` is true.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// served is a flag enabling/disabling this version from being served via REST APIs
	Served bool `json:"served" protobuf:"varint,2,opt,name=served"`
	// storage indicates this version should be used when persisting custom resources to storage.
	// There must be exactly one version with storage=true.
	Storage bool `json:"storage" protobuf:"varint,3,opt,name=storage"`
	// schema describes the schema used for validation and pruning of this version of the custom resource.
	// Top-level and per-version schemas are mutually exclusive.
	// Per-version schemas must not all be set to identical values (top-level validation schema should be used instead).
	// +optional
	Schema *CustomResourceValidation `json:"schema,omitempty" protobuf:"bytes,4,opt,name=schema"`
	// subresources specify what subresources this version of the defined custom resource have.
	// Top-level and per-version subresources are mutually exclusive.
	// Per-version subresources must not all be set to identical values (top-level subresources should be used instead).
	// +optional
	Subresources *v1beta1.CustomResourceSubresources `json:"subresources,omitempty" protobuf:"bytes,5,opt,name=subresources"`
	// additionalPrinterColumns specifies additional columns returned in Table output.
	// See https://kubernetes.io/docs/reference/using-api/api-concepts/#receiving-resources-as-tables for details.
	// Top-level and per-version columns are mutually exclusive.
	// Per-version columns must not all be set to identical values (top-level columns should be used instead).
	// If no top-level or per-version columns are specified, a single column displaying the age of the custom resource is used.
	// +optional
	AdditionalPrinterColumns []v1beta1.CustomResourceColumnDefinition `json:"additionalPrinterColumns,omitempty" protobuf:"bytes,6,rep,name=additionalPrinterColumns"`
}

// CustomResourceValidation is a list of validation methods for CustomResources.
type CustomResourceValidation struct {
	// openAPIV3Schema is the OpenAPI v3 schema to use for validation and pruning.
	// +optional
	OpenAPIV3Schema runtime.RawExtension `json:"openAPIV3Schema,omitempty" protobuf:"bytes,1,opt,name=openAPIV3Schema"`
}
