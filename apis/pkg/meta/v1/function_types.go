/*
Copyright 2023 The Crossplane Authors.

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

// FunctionSpec specifies the configuration of a Function.
type FunctionSpec struct {
	MetaSpec `json:",inline"`

	// Image is the packaged Function image.
	Image *string `json:"image,omitempty"`

	// The type of this function - composition or operation.
	// +optional
	// +kubebuilder:validation:Enum=Composition;Operation
	// +kubebuilder:default=Composition
	Type *FunctionType `json:"type,omitempty"`
}

// A FunctionType represents the type of a Function.
type FunctionType string

const (
	// FunctionTypeComposition functions are used in a composition pipeline.
	FunctionTypeComposition FunctionType = "Composition"

	// FunctionTypeOperation functions are used in an operation pipeline.
	FunctionTypeOperation FunctionType = "Operation"
)

// +kubebuilder:object:root=true
// +kubebuilder:storageversion

// A Function is the description of a Crossplane Function package.
type Function struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec FunctionSpec `json:"spec"`
}
