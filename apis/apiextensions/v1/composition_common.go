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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

/*
	NOTE(negz): This file contains types that are shared between the Composition
	and CompositionRevision types. It exists so we can copy these types to the
	apiextensions/v1beta1 package without copying the entire Composition type.
	Once we no longer support v1beta1 CompositionRevisions it can be merged back
	into composition_revision_types.go.
*/

// A CompositionMode determines what mode of Composition is used.
type CompositionMode string

const (
	// CompositionModePipeline indicates that a Composition specifies a
	// pipeline of functions, each of which is responsible for producing
	// composed resources that Crossplane should create or update.
	CompositionModePipeline CompositionMode = "Pipeline"
)

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

// A PipelineStep in a function pipeline.
type PipelineStep struct {
	// Step name. Must be unique within its Pipeline.
	Step string `json:"step"`

	// FunctionRef is a reference to the function this step should
	// execute.
	FunctionRef FunctionReference `json:"functionRef"`

	// Input is an optional, arbitrary Kubernetes resource (i.e. a resource
	// with an apiVersion and kind) that will be passed to the function as
	// the 'input' of its RunFunctionRequest.
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:EmbeddedResource
	Input *runtime.RawExtension `json:"input,omitempty"`

	// Credentials are optional credentials that the function needs.
	// +optional
	// +listType=map
	// +listMapKey=name
	Credentials []FunctionCredentials `json:"credentials,omitempty"`

	// Requirements are resource requirements that will be satisfied before
	// this pipeline step is called for the first time. This allows
	// pre-populating required resources without requiring a function to
	// request them first.
	// +optional
	Requirements *FunctionRequirements `json:"requirements,omitempty"`
}

// A FunctionReference references a function that may be used in a
// Composition pipeline.
type FunctionReference struct {
	// Name of the referenced Function.
	Name string `json:"name"`
}

// FunctionCredentials are optional credentials that a function
// needs to run.
//
// +kubebuilder:validation:XValidation:rule="self.source == 'Secret' && has(self.secretRef)",message="the Secret source requires a secretRef"
type FunctionCredentials struct {
	// Name of this set of credentials.
	Name string `json:"name"`

	// Source of the function credentials.
	// +kubebuilder:validation:Enum=None;Secret
	Source FunctionCredentialsSource `json:"source"`

	// A SecretRef is a reference to a secret containing credentials that should
	// be supplied to the function.
	// +optional
	SecretRef *xpv1.SecretReference `json:"secretRef,omitempty"`
}

// A FunctionCredentialsSource is a source from which function
// credentials may be acquired.
type FunctionCredentialsSource string

const (
	// FunctionCredentialsSourceNone indicates that a function does not require
	// credentials.
	FunctionCredentialsSourceNone FunctionCredentialsSource = "None"

	// FunctionCredentialsSourceSecret indicates that a function should acquire
	// credentials from a secret.
	FunctionCredentialsSourceSecret FunctionCredentialsSource = "Secret"
)

// FunctionRequirements define requirements that a function may need to
// satisfy.
type FunctionRequirements struct {
	// RequiredResources is a list of resources that must be fetched before
	// this function is called.
	// +optional
	// +listType=map
	// +listMapKey=requirementName
	RequiredResources []RequiredResourceSelector `json:"requiredResources,omitempty"`
}

// RequiredResourceSelector selects a required resource.
//
// +kubebuilder:validation:XValidation:rule="(has(self.name) && !has(self.matchLabels)) || (!has(self.name) && has(self.matchLabels))",message="Either name or matchLabels must be specified, but not both"
type RequiredResourceSelector struct {
	// RequirementName is the unique name to identify this required resource
	// in the Required Resources map in the function request.
	RequirementName string `json:"requirementName"`

	// APIVersion of the required resource.
	APIVersion string `json:"apiVersion"`

	// Kind of the required resource.
	Kind string `json:"kind"`

	// Namespace of the required resource if it is namespaced.
	// +optional
	Namespace *string `json:"namespace,omitempty"`

	// Name of the required resource.
	// +optional
	Name *string `json:"name,omitempty"`

	// MatchLabels specifies the set of labels to match for finding the
	// required resource. When specified, Name is ignored.
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
}
