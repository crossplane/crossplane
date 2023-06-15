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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:storageversion

// A FunctionIO represents the I/O of an Composition Function.
type FunctionIO struct {
	metav1.TypeMeta `json:",inline"`

	// Config is an opaque Kubernetes object containing optional function
	// configuration.
	// +optional
	// +kubebuilder:validation:EmbeddedResource
	// +kubebuilder:pruning:PreserveUnknownFields
	Config *runtime.RawExtension `json:"config,omitempty"`

	// Observed state prior to the invocation of a function pipeline. Functions
	// must not mutate this state - any attempts to do so will be ignored. State
	// passed to each function is fresh as of the time the function pipeline was
	// invoked, not as of the time each function was invoked.
	Observed Observed `json:"observed"`

	// Desired state according to a function pipeline. The state passed to a
	// particular function may have been mutated by previous functions in the
	// pipeline. Functions may mutate any part of the desired state they are
	// concerned with, and must pass through any part of the desired state that
	// they are not concerned with.
	Desired Desired `json:"desired"`

	// Results is an optional list that can be used by function to emit results
	// for observability and debugging purposes. Functions may mutate any
	// results that they are concerned with, and must pass through any results
	// that that are not concerned with.
	// +optional
	Results []Result `json:"results,omitempty"`
}

// Observed state at the beginning of a function pipeline invocation.
type Observed struct {
	// Composite reflects the observed state of the XR this function reconciles.
	Composite ObservedComposite `json:"composite"`

	// Resources reflect the observed state of any extant composed resources
	// this function reconciles. Only composed resources that currently exist in
	// the API server (i.e. have been created and not yet deleted) are included.
	// +optional
	Resources []ObservedResource `json:"resources,omitempty"`
}

// An ObservedComposite resource.
type ObservedComposite struct {
	// Resource reflects the observed XR.
	// +kubebuilder:validation:EmbeddedResource
	// +kubebuilder:pruning:PreserveUnknownFields
	Resource runtime.RawExtension `json:"resource"`

	// ConnectionDetails reflects the observed connection details of the XR.
	// +optional
	ConnectionDetails []ExplicitConnectionDetail `json:"connectionDetails,omitempty"`
}

// An ObservedResource represents an observed composed resource.
type ObservedResource struct {
	// Name of the observed resource. Must be unique within the array of
	// observed resources. Corresponds to the name entry in a Composition's
	// resources array, and the name entry in the desired resources array.
	Name string `json:"name"`

	// Resource reflects the observed composed resource.
	// +kubebuilder:validation:EmbeddedResource
	// +kubebuilder:pruning:PreserveUnknownFields
	Resource runtime.RawExtension `json:"resource"`

	// ConnectionDetails reflects the observed connection details of the
	// composed resource.
	ConnectionDetails []ExplicitConnectionDetail `json:"connectionDetails"`
}

// Desired state of a function pipeline invocation.
type Desired struct {
	// Composite reflects the desired state of the XR this function reconciles.
	Composite DesiredComposite `json:"composite"`

	// Resources reflect the desired state of composed resources, including
	// those that do not yet exist.
	// +optional
	Resources []DesiredResource `json:"resources,omitempty"`
}

// A DesiredComposite resource.
type DesiredComposite struct {
	// Resource reflects the desired XR. Functions may update the metadata,
	// spec, and status of an XR.
	// +kubebuilder:validation:EmbeddedResource
	// +kubebuilder:pruning:PreserveUnknownFields
	Resource runtime.RawExtension `json:"resource"`

	// ConnectionDetails reflects the desired connection details of the XR.
	// +optional
	ConnectionDetails []ExplicitConnectionDetail `json:"connectionDetails"`
}

// A DesiredResource represents a desired composed resource.
type DesiredResource struct {
	// Name of the desired resource. Must be unique within the array of
	// desired resources. Corresponds to the name entry in a Composition's
	// resources array, and the name entry in the observed resources array.
	Name string `json:"name"`

	// Resource reflects the desired composed resource. Functions may update the
	// metadata and spec of a composed resource. Updates to status will be
	// discarded.
	// +kubebuilder:validation:EmbeddedResource
	// +kubebuilder:pruning:PreserveUnknownFields
	Resource runtime.RawExtension `json:"resource"`

	// ConnectionDetails reflects _XR_ connection details that should be derived
	// from this composed resource.
	// +optional
	ConnectionDetails []DerivedConnectionDetail `json:"connectionDetails,omitempty"`

	// ReadinessChecks configures how this composed resource will be determined
	// to be ready.
	// +optional
	ReadinessChecks []DesiredReadinessCheck `json:"readinessChecks,omitempty"`
}

// An ExplicitConnectionDetail is a simple map of name (key) to value.
type ExplicitConnectionDetail struct {
	// Name of the connection detail.
	Name string `json:"name"`

	// Value of the connection detail.
	Value string `json:"value"`
}

// A ConnectionDetailType is a type of connection detail.
type ConnectionDetailType string

// ConnectionDetailType types.
const (
	ConnectionDetailTypeUnknown                 ConnectionDetailType = "Unknown"
	ConnectionDetailTypeFromConnectionSecretKey ConnectionDetailType = "FromConnectionSecretKey"
	ConnectionDetailTypeFromFieldPath           ConnectionDetailType = "FromFieldPath"
	ConnectionDetailTypeFromValue               ConnectionDetailType = "FromValue"
)

// A DerivedConnectionDetail specifies how to derive an XR connection detail
// from a composed resource.
type DerivedConnectionDetail struct {
	// Name of the connection detail that will be propagated to the
	// connection secret of the XR. Can be omitted for FromConnectionDetailKey,
	// in which case it will default to that key.
	// +optional
	Name *string `json:"name,omitempty"`

	// Type sets the connection detail fetching behaviour to be used. Each
	// connection detail type may require its own fields to be set on the
	// ConnectionDetail object.
	// +kubebuilder:validation:Enum=FromConnectionDetailKey;FromFieldPath;FromValue
	Type ConnectionDetailType `json:"type"`

	// FromConnectionDetailKey sets an XR connection detail to the value of the
	// supplied connection detail of the composed resource.
	// +optional
	FromConnectionSecretKey *string `json:"fromConnectionSecretKey,omitempty"`

	// FromFieldPath sets an XR connection detail to the value at the supplied
	// fieldpath within the composed resource.
	// +optional
	FromFieldPath *string `json:"fromFieldPath,omitempty"`

	// Value that will be propagated to the connection detail of the XR.
	// +optional
	Value *string `json:"value,omitempty"`
}

// ReadinessCheckType is used for readiness check types.
type ReadinessCheckType string

// The possible values for readiness check type.
const (
	ReadinessCheckTypeNonEmpty       ReadinessCheckType = "NonEmpty"
	ReadinessCheckTypeMatchString    ReadinessCheckType = "MatchString"
	ReadinessCheckTypeMatchInteger   ReadinessCheckType = "MatchInteger"
	ReadinessCheckTypeMatchCondition ReadinessCheckType = "MatchCondition"
	ReadinessCheckTypeNone           ReadinessCheckType = "None"
)

// A DesiredReadinessCheck is used to indicate how to tell whether a resource is
// ready for consumption
type DesiredReadinessCheck struct {
	// Type indicates the type of probe you'd like to use.
	// +kubebuilder:validation:Enum="MatchString";"MatchInteger";"NonEmpty";"MatchCondition";"None"
	Type ReadinessCheckType `json:"type"`

	// FieldPath shows the path of the field whose value will be used.
	// +optional
	FieldPath *string `json:"fieldPath,omitempty"`

	// MatchString is the value you'd like to match if you're using
	// "MatchString" type.
	// +optional
	MatchString *string `json:"matchString,omitempty"`

	// MatchInt is the value you'd like to match if you're using "MatchInt"
	// type.
	// +optional
	MatchInteger *int64 `json:"matchInteger,omitempty"`

	// MatchCondition specifies the condition you'd like to match if you're using "MatchCondition" type.
	// +optional
	MatchCondition *MatchConditionReadinessCheck `json:"matchCondition,omitempty"`
}

// MatchConditionReadinessCheck is used to indicate how to tell whether a resource is ready
// for consumption
type MatchConditionReadinessCheck struct {
	// Type indicates the type of condition you'd like to use.
	// +kubebuilder:default="Ready"
	Type xpv1.ConditionType `json:"type"`

	// Status is the status of the condition you'd like to match.
	// +kubebuilder:default="True"
	Status corev1.ConditionStatus `json:"status"`
}

// Result is an optional list that can be used by function to emit results for
// observability and debugging purposes.
type Result struct {
	// Severity is the severity of a result.
	//
	// Fatal results are fatal; subsequent Composition Functions may run, but
	// the Composition Function pipeline run will be considered a failure and
	// the first error will be returned.
	//
	// Warning results are non-fatal; the entire Composition will run to
	// completion but warning events and debug logs associated with the
	// composite resource will be emitted.
	//
	// Normal results are emitted as normal events and debug logs associated
	// with the composite resource.
	// +kubebuilder:validation:Enum=Fatal;Warning;Normal
	Severity Severity `json:"severity"`

	// Message is a human readable message.
	Message string `json:"message"`
}

// Severity is the severity of a result.
type Severity string

// Result severities.
const (
	SeverityFatal   Severity = "Fatal"
	SeverityWarning Severity = "Warning"
	SeverityNormal  Severity = "Normal"
)
