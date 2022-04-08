/*
Copyright 2022 The Crossplane Authors.

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
	"k8s.io/apimachinery/pkg/runtime"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:storageversion

// A ResourceList represents the I/O of an XRM function.
type ResourceList struct {
	metav1.TypeMeta `json:",inline"`

	// FunctionConfig is an optional Kubernetes object for passing arguments to
	// a function invocation.
	// +optional
	FunctionConfig *runtime.RawExtension `json:"functionConfig,omitempty"`

	// Items is a list of Crossplane resources - either XRs or MRs.
	//
	// A function will read this field in the input ResourceList and populate
	// this field in the output ResourceList.
	Items []runtime.RawExtension `json:"items"`

	// Results is an optional list that can be used by function to emit results
	// for observability and debugging purposes.
	// +optional
	Results []Result `json:"results,omitempty"`
}

// Result is an optional list that can be used by function to emit results for
// observability and debugging purposes.
type Result struct {
	//  Message is a human readable message.
	Message string `json:"message"`

	// Severity is the severity of a result:
	//
	//   "error": indicates an error result.
	//   "warning": indicates a warning result.
	//   "info": indicates an informational result.
	// +optional
	// +kubebuilder:validation:Enum=Error;Warning;Info
	Severity *Severity `json:"severity,omitempty"`

	//  ResourceRef is the metadata for referencing a Kubernetes object
	// associated with a result.
	// +optional
	ResourceRef *xpv1.TypedReference `json:"resourceRef,omitempty"`

	// Field is the reference to a field in the object.
	// If defined, `ResourceRef` must also be provided.
	// +optional
	Field *Field `json:"field,omitempty"`

	// TODO(negz): Does File make sense in the context of XRM functions, which
	// are computed 'server side', as opposed to client-side KRM functions?
	// I could imagine it being used to refer to a config file 'inside' an XRM
	// function such as a Helm chart template.

	// File references a file containing the resource.
	// +optional
	File *File `json:"file,omitempty"`

	// Tags is an unstructured key value map stored with a result that may be
	// set by external tools to store and retrieve arbitrary metadata.
	Tags map[string]string `json:"tags,omitempty"`
}

// Severity is the severity of a result.
type Severity string

// Result severities.
const (
	SeverityError   string = "Error"
	SeverityWarning string = "Warning"
	SeverityInfo    string = "Info"
)

// Field is the reference to a field in the object references by a result.
type Field struct {
	// Path is the JSON path of the field
	// e.g. `spec.template.spec.containers[3].resources.limits.cpu`
	Path string

	// CurrrentValue is the current value of the field.
	// Can be any value - string, number, boolean, array or object.
	// +optional
	CurrentValue *runtime.RawExtension `json:"currentValue,omitempty"`

	// ProposedValue is the proposed value of the field to fix an issue.
	// +optional
	// Can be any value - string, number, boolean, array or object.
	ProposedValue runtime.RawExtension `json:"proposedValue,omitempty"`
}

// File references a file containing the resource in a result.
type File struct {
	// Path is the OS agnostic, slash-delimited, relative path.
	// e.g. `some-dir/some-file.yaml`.
	Path string `json:"path"`

	// Index of the object in a multi-object YAML file.
	// +optional
	Index *int64 `json:"index,omitempty"`
}
