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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// A OperationMode determines what mode an operation uses.
type OperationMode string

const (
	// OperationModePipeline indicates that an Operation specifies a
	// pipeline of functions, each of which is responsible for implementing
	// its logic.
	OperationModePipeline OperationMode = "Pipeline"
)

// OperationSpec specifies desired state of an operation.
type OperationSpec struct {
	// Mode controls what type or "mode" of operation will be used.
	//
	// "Pipeline" indicates that an Operation specifies a pipeline of
	// functions, each of which is responsible for implementing its logic.
	//
	// +kubebuilder:validation:Enum=Pipeline
	// +kubebuilder:default=Pipeline
	Mode OperationMode `json:"mode"`

	// Pipeline is a list of operation function steps that will be used when
	// this operation runs.
	// +listType=map
	// +listMapKey=step
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=99
	Pipeline []PipelineStep `json:"pipeline"`

	// RetryLimit configures how many times the operation may fail. When the
	// failure limit is exceeded, the operation will not be retried.
	// +optional
	// +kubebuilder:default:5
	RetryLimit *int64 `json:"retryLimit,omitempty"`
}

// A PipelineStep in an operation function pipeline.
type PipelineStep struct {
	// Step name. Must be unique within its Pipeline.
	Step string `json:"step"`

	// FunctionRef is a reference to the function this step should
	// execute.
	FunctionRef FunctionReference `json:"functionRef"`

	// Input is an optional, arbitrary Kubernetes resource (i.e. a resource
	// with an apiVersion and kind) that will be passed to the unction as
	// the 'input' of its RunFunctionRequest.
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:EmbeddedResource
	Input *runtime.RawExtension `json:"input,omitempty"`

	// Credentials are optional credentials that the operation function needs.
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

// A FunctionReference references an operation function that may be used in an
// operation pipeline.
type FunctionReference struct {
	// Name of the referenced function.
	Name string `json:"name"`
}

// FunctionCredentials are optional credentials that a function
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

// FunctionRequirements specifies resource requirements for a pipeline step.
type FunctionRequirements struct {
	// RequiredResources that will be fetched before this pipeline step
	// is called for the first time.
	// +optional
	// +listType=map
	// +listMapKey=requirementName
	RequiredResources []RequiredResourceSelector `json:"requiredResources,omitempty"`
}

// RequiredResourceSelector selects resources that should be fetched before
// a pipeline step runs.
// +kubebuilder:validation:XValidation:rule="(has(self.name) && !has(self.matchLabels)) || (!has(self.name) && has(self.matchLabels))",message="Either name or matchLabels must be specified, but not both"
type RequiredResourceSelector struct {
	// RequirementName uniquely identifies this group of resources.
	// This name will be used as the key in RunFunctionRequest.required_resources.
	RequirementName string `json:"requirementName"`

	// APIVersion of resources to select.
	APIVersion string `json:"apiVersion"`

	// Kind of resources to select.
	Kind string `json:"kind"`

	// Name matches a single resource by name. Only one of Name or
	// MatchLabels may be specified.
	// +optional
	Name *string `json:"name,omitempty"`

	// MatchLabels matches resources by label selector. Only one of Name or
	// MatchLabels may be specified.
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty"`

	// Namespace to search for resources. Optional for cluster-scoped resources.
	// +optional
	Namespace *string `json:"namespace,omitempty"`
}

// OperationStatus represents the observed state of an operation.
type OperationStatus struct {
	xpv1.ConditionedStatus `json:",inline"`

	// Number of operation failures.
	Failures int64 `json:"failures,omitempty"`

	// Pipeline represents the output of the pipeline steps that this operation
	// ran.
	Pipeline []PipelineStepStatus `json:"pipeline,omitempty"`

	// AppliedResourceRefs references all resources the Operation applied.
	AppliedResourceRefs []AppliedResourceRef `json:"appliedResourceRefs,omitempty"`
}

// PipelineStepStatus represents the status of an individual pipeline step.
type PipelineStepStatus struct {
	// Step name. Unique within its Pipeline.
	Step string `json:"step"`

	// Output of this step.
	// +kubebuilder:pruning:PreserveUnknownFields
	Output *runtime.RawExtension `json:"output,omitempty"`
}

// An AppliedResourceRef is a reference to a resource an Operation applied.
type AppliedResourceRef struct {
	// APIVersion of the applied resource.
	APIVersion string `json:"apiVersion"`

	// Kind of the applied resource.
	Kind string `json:"kind"`

	// Namespace of the applied resource.
	// +optional
	Namespace *string `json:"namespace,omitempty"`

	// Name of the applied resource.
	Name string `json:"name"`
}

// Equals returns true if this AppliedResourceRef is equal to the other.
func (r *AppliedResourceRef) Equals(other AppliedResourceRef) bool {
	if r.APIVersion != other.APIVersion {
		return false
	}
	if r.Kind != other.Kind {
		return false
	}
	if r.Name != other.Name {
		return false
	}
	return ptr.Deref(r.Namespace, "") == ptr.Deref(other.Namespace, "")
}

// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +genclient

// An Operation defines a pipeline of functions that together constitute a day
// two operation.
//
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="SUCCEEDED",type="string",JSONPath=".status.conditions[?(@.type=='Succeeded')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Cluster,categories=crossplane,shortName=ops
type Operation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OperationSpec   `json:"spec,omitempty"`
	Status OperationStatus `json:"status,omitempty"`
}

// SetConditions delegates to Status.SetConditions.
// Implements Conditioned.SetConditions.
func (o *Operation) SetConditions(cs ...xpv1.Condition) {
	o.Status.SetConditions(cs...)
}

// GetCondition delegates to Status.GetCondition.
// Implements Conditioned.GetCondition.
func (o *Operation) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return o.Status.GetCondition(ct)
}

// IsComplete returns if this operation has finished running.
func (o *Operation) IsComplete() bool {
	c := o.GetCondition(TypeSucceeded)
	// Normally, checking observedGeneration == generation is required, but Succeeded=True/False are terminal conditions.
	return c.Status == corev1.ConditionTrue || c.Status == corev1.ConditionFalse
}

// +kubebuilder:object:root=true

// OperationList contains a list of Operations.
type OperationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Operation `json:"items"`
}
