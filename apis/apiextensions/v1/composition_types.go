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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CompositionSpec specifies desired state of a composition.
type CompositionSpec struct {
	// CompositeTypeRef specifies the type of composite resource that this
	// composition is compatible with.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	CompositeTypeRef TypeReference `json:"compositeTypeRef"`

	// Mode controls what type or "mode" of Composition will be used.
	//
	// "Pipeline" indicates that a Composition specifies a pipeline of
	// Composition Functions, each of which is responsible for producing
	// composed resources that Crossplane should create or update.
	//
	// "Resources" indicates that a Composition uses what is commonly referred
	// to as "Patch & Transform" or P&T composition. This mode of Composition
	// uses an array of resources, each a template for a composed resource.
	//
	// All Compositions should use Pipeline mode. Resources mode is deprecated.
	// Resources mode won't be removed in Crossplane 1.x, and will remain the
	// default to avoid breaking legacy Compositions. However, it's no longer
	// accepting new features, and only accepting security related bug fixes.
	//
	// +optional
	// +kubebuilder:validation:Enum=Resources;Pipeline
	// +kubebuilder:default=Resources
	Mode *CompositionMode `json:"mode,omitempty"`

	// PatchSets define a named set of patches that may be included by any
	// resource in this Composition. PatchSets cannot themselves refer to other
	// PatchSets.
	//
	// PatchSets are only used by the "Resources" mode of Composition. They
	// are ignored by other modes.
	//
	// Deprecated: Use Composition Functions instead.
	//
	// +optional
	PatchSets []PatchSet `json:"patchSets,omitempty"`

	// Environment configures the environment in which resources are rendered.
	//
	// THIS IS AN ALPHA FIELD. Do not use it in production. It is not honored
	// unless the relevant Crossplane feature flag is enabled, and may be
	// changed or removed without notice.
	// +optional
	Environment *EnvironmentConfiguration `json:"environment,omitempty"`

	// Resources is a list of resource templates that will be used when a
	// composite resource referring to this composition is created.
	//
	// Resources are only used by the "Resources" mode of Composition. They are
	// ignored by other modes.
	//
	// Deprecated: Use Composition Functions instead.
	//
	// +optional
	Resources []ComposedTemplate `json:"resources,omitempty"`

	// Pipeline is a list of composition function steps that will be used when a
	// composite resource referring to this composition is created. One of
	// resources and pipeline must be specified - you cannot specify both.
	//
	// The Pipeline is only used by the "Pipeline" mode of Composition. It is
	// ignored by other modes.
	// +optional
	// +listType=map
	// +listMapKey=step
	Pipeline []PipelineStep `json:"pipeline,omitempty"`

	// WriteConnectionSecretsToNamespace specifies the namespace in which the
	// connection secrets of composite resource dynamically provisioned using
	// this composition will be created.
	// This field is planned to be replaced in a future release in favor of
	// PublishConnectionDetailsWithStoreConfigRef. Currently, both could be
	// set independently and connection details would be published to both
	// without affecting each other as long as related fields at MR level
	// specified.
	// +optional
	WriteConnectionSecretsToNamespace *string `json:"writeConnectionSecretsToNamespace,omitempty"`

	// PublishConnectionDetailsWithStoreConfig specifies the secret store config
	// with which the connection details of composite resources dynamically
	// provisioned using this composition will be published.
	//
	// THIS IS AN ALPHA FIELD. Do not use it in production. It is not honored
	// unless the relevant Crossplane feature flag is enabled, and may be
	// changed or removed without notice.
	// +optional
	// +kubebuilder:default={"name": "default"}
	PublishConnectionDetailsWithStoreConfigRef *StoreConfigReference `json:"publishConnectionDetailsWithStoreConfigRef,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +genclient
// +genclient:nonNamespaced

// A Composition defines a collection of managed resources or functions that
// Crossplane uses to create and manage new composite resources.
//
// Read the Crossplane documentation for
// [more information about Compositions](https://docs.crossplane.io/latest/concepts/compositions).
// +kubebuilder:printcolumn:name="XR-KIND",type="string",JSONPath=".spec.compositeTypeRef.kind"
// +kubebuilder:printcolumn:name="XR-APIVERSION",type="string",JSONPath=".spec.compositeTypeRef.apiVersion"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Cluster,categories=crossplane,shortName=comp
type Composition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec CompositionSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// CompositionList contains a list of Compositions.
type CompositionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Composition `json:"items"`
}
