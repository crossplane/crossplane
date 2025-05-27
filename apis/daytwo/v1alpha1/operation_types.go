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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// OperationSpec specifies desired state of an operation.
type OperationSpec struct {
	// Pipeline is a list of operation function steps that will be used when
	// this operation runs.
	Pipeline []PipelineStep `json:"pipeline"`

	// FailureLimit configures how many times the operation may fail. When the
	// failure limit is exceeded, the operation will not be retried.
	// +optional
	// +kubebuilder:default:5
	FailureLimit *int64 `json:"failureLimit,omitempty"`
}

// A PipelineStep in a operation function pipeline.
type PipelineStep struct {
	// Step name. Must be unique within its Pipeline.
	Step string `json:"step"`

	// FunctionRef is a reference to the Composition Function this step should
	// execute.
	FunctionRef FunctionReference `json:"functionRef"`

	// Input is an optional, arbitrary Kubernetes resource (i.e. a resource
	// with an apiVersion and kind) that will be passed to the Composition
	// Function as the 'input' of its RunFunctionRequest.
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:EmbeddedResource
	Input *runtime.RawExtension `json:"input,omitempty"`

	// Credentials are optional credentials that the operation function needs.
	// +optional
	Credentials []FunctionCredentials `json:"credentials,omitempty"`
}

// A FunctionReference references an operation function that may be used in an
// operation pipeline.
type FunctionReference struct {
	// Name of the referenced Function.
	Name string `json:"name"`
}

// FunctionCredentials are optional credentials that a Composition Function
// needs to run.
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

// A FunctionCredentialsSource is a source from which Composition Function
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

// OperationStatus represents the observed state of an operation.
type OperationStatus struct {
	xpv1.ConditionedStatus `json:",inline"`

	// Number of operation failures.
	Failures int64 `json:"failures,omitempty"`

	// Pipeline represents the output of the pipeline steps that this operation
	// ran.
	Pipeline []PipelineStepStatus `json:"pipeline,omitempty"`
}

// PipelineStepStatus represents the status of an individual pipeline step.
type PipelineStepStatus struct {
	// Step name. Unique within its Pipeline.
	Step string `json:"step"`

	// Output of this step.
	// +kubebuilder:pruning:PreserveUnknownFields
	Output *runtime.RawExtension `json:"output,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +genclient
// +genclient:nonNamespaced

// An Operation defines a pipeline of functions that together constitute a day
// two operation.
//
// +kubebuilder:printcolumn:name="XR-KIND",type="string",JSONPath=".spec.compositeTypeRef.kind"
// +kubebuilder:printcolumn:name="XR-APIVERSION",type="string",JSONPath=".spec.compositeTypeRef.apiVersion"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Cluster,categories=crossplane,shortName=comp
type Operation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OperationSpec   `json:"spec,omitempty"`
	Status OperationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OperationList contains a list of Operations.
type OperationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Operation `json:"items"`
}
