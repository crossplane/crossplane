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
//
// +kubebuilder:validation:XValidation:rule="self.mode == 'Pipeline' && has(self.pipeline)",message="an array of pipeline steps is required in Pipeline mode"
type CompositionSpec struct {
	// CompositeTypeRef specifies the type of composite resource that this
	// composition is compatible with.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable"
	CompositeTypeRef TypeReference `json:"compositeTypeRef"`

	// Mode controls what type or "mode" of Composition will be used.
	//
	// "Pipeline" indicates that a Composition specifies a pipeline of
	// functions, each of which is responsible for producing composed
	// resources that Crossplane should create or update.
	//
	// +optional
	// +kubebuilder:validation:Enum=Pipeline
	// +kubebuilder:default=Pipeline
	Mode CompositionMode `json:"mode,omitempty"`

	// Pipeline is a list of composition function steps that will be used when a
	// composite resource referring to this composition is created. One of
	// resources and pipeline must be specified - you cannot specify both.
	//
	// The Pipeline is only used by the "Pipeline" mode of Composition. It is
	// ignored by other modes.
	// +optional
	// +listType=map
	// +listMapKey=step
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=99
	Pipeline []PipelineStep `json:"pipeline,omitempty"`

	// DeletionPolicy determines how this Composition handles the deletion of
	// its composite resources. "Background" is Crossplane's default behavior,
	// used when this field is unset. It stops managing the composite resource
	// as soon as it's deleted, and Kubernetes deletes its composed resources
	// once it's gone. "Foreground" deletes the composed resources and keeps
	// the composite resource until they're gone. "Pipeline" is like
	// "Foreground", but keeps running the function pipeline, which decides
	// when each composed resource may be deleted. The pipeline must
	// eventually stop desiring every composed resource, or the composite
	// resource will never finish deleting.
	//
	// Crossplane ignores this field for legacy cluster scoped composite
	// resources, which always use "Background".
	//
	// This is an alpha field. It's ignored unless Crossplane runs with the
	// --enable-function-controlled-deletion flag.
	// +optional
	// +kubebuilder:validation:Enum=Background;Foreground;Pipeline
	DeletionPolicy CompositionDeletionPolicy `json:"deletionPolicy,omitempty"`

	// WriteConnectionSecretsToNamespace specifies the namespace in which the
	// connection secrets of composite resource dynamically provisioned using
	// this composition will be created.
	// +optional
	WriteConnectionSecretsToNamespace *string `json:"writeConnectionSecretsToNamespace,omitempty"`

	// UnusedRevisionHistoryLimit controls how many unused CompositionRevisions
	// Crossplane keeps. The latest revision and any revision in use by a
	// composite resource are always kept and don't count toward the limit.
	// The oldest unused revisions beyond this limit will be garbage collected.
	// Defaults to 1. Set to 0 to keep every revision.
	//
	// Changing this field doesn't create a new CompositionRevision.
	// +optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	UnusedRevisionHistoryLimit *int64 `json:"unusedRevisionHistoryLimit,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +genclient
// +genclient:nonNamespaced

// A Composition defines a collection of managed resources or functions that
// Crossplane uses to create and manage new composite resources.
//
// Read the Crossplane documentation for
// [more information about Compositions](https://docs.crossplane.io/latest/composition/compositions/).
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

	Items []Composition `json:"items"`
}
