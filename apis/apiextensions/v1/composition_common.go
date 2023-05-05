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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

/*
	NOTE(negz): This file contains types that are shared between the Composition
	and CompositionRevision types. It exists so we can copy these types to the
	apiextensions/v1beta1 package without copying the entire Composition type.
	Once we no longer support v1beta1 CompositionRevisions it can be merged back
	into composition_revision_types.go.
*/

// TypeReference is used to refer to a type for declaring compatibility.
type TypeReference struct {
	// APIVersion of the type.
	APIVersion string `json:"apiVersion"`

	// Kind of the type.
	Kind string `json:"kind"`
}

// TypeReferenceTo returns a reference to the supplied GroupVersionKind
func TypeReferenceTo(gvk schema.GroupVersionKind) TypeReference {
	return TypeReference{APIVersion: gvk.GroupVersion().String(), Kind: gvk.Kind}
}

// A PatchSet is a set of patches that can be reused from all resources within
// a Composition.
type PatchSet struct {
	// Name of this PatchSet.
	Name string `json:"name"`

	// Patches will be applied as an overlay to the base resource.
	Patches []Patch `json:"patches"`
}

// ComposedTemplate is used to provide information about how the composed resource
// should be processed.
type ComposedTemplate struct {
	// TODO(negz): Name should be a required field in v2 of this API.

	// A Name uniquely identifies this entry within its Composition's resources
	// array. Names are optional but *strongly* recommended. When all entries in
	// the resources array are named entries may added, deleted, and reordered
	// as long as their names do not change. When entries are not named the
	// length and order of the resources array should be treated as immutable.
	// Either all or no entries must be named.
	// +optional
	Name *string `json:"name,omitempty"`

	// Base is the target resource that the patches will be applied on.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:EmbeddedResource
	Base runtime.RawExtension `json:"base"`

	// Patches will be applied as overlay to the base resource.
	// +optional
	Patches []Patch `json:"patches,omitempty"`

	// ConnectionDetails lists the propagation secret keys from this target
	// resource to the composition instance connection secret.
	// +optional
	ConnectionDetails []ConnectionDetail `json:"connectionDetails,omitempty"`

	// ReadinessChecks allows users to define custom readiness checks. All checks
	// have to return true in order for resource to be considered ready. The
	// default readiness check is to have the "Ready" condition to be "True".
	// +optional
	ReadinessChecks []ReadinessCheck `json:"readinessChecks,omitempty"`
}

// GetName returns the name of the composed template or an empty string if it is nil.
func (ct *ComposedTemplate) GetName() string {
	if ct.Name != nil {
		return *ct.Name
	}
	return ""
}

// ReadinessCheckType is used for readiness check types.
type ReadinessCheckType string

// The possible values for readiness check type.
const (
	ReadinessCheckTypeNonEmpty     ReadinessCheckType = "NonEmpty"
	ReadinessCheckTypeMatchString  ReadinessCheckType = "MatchString"
	ReadinessCheckTypeMatchInteger ReadinessCheckType = "MatchInteger"
	ReadinessCheckTypeNone         ReadinessCheckType = "None"
)

// IsValid returns nil if the readiness check type is valid, or an error otherwise.
func (t *ReadinessCheckType) IsValid() bool {
	switch *t {
	case ReadinessCheckTypeNonEmpty, ReadinessCheckTypeMatchString, ReadinessCheckTypeMatchInteger, ReadinessCheckTypeNone:
		return true
	}
	return false
}

// ReadinessCheck is used to indicate how to tell whether a resource is ready
// for consumption
type ReadinessCheck struct {
	// TODO(negz): Optional fields should be nil in the next version of this
	// API. How would we know if we actually wanted to match the empty string,
	// or 0?

	// Type indicates the type of probe you'd like to use.
	// +kubebuilder:validation:Enum="MatchString";"MatchInteger";"NonEmpty";"None"
	Type ReadinessCheckType `json:"type"`

	// FieldPath shows the path of the field whose value will be used.
	// +optional
	FieldPath string `json:"fieldPath,omitempty"`

	// MatchString is the value you'd like to match if you're using "MatchString" type.
	// +optional
	MatchString string `json:"matchString,omitempty"`

	// MatchInt is the value you'd like to match if you're using "MatchInt" type.
	// +optional
	MatchInteger int64 `json:"matchInteger,omitempty"`
}

// Validate checks if the readiness check is logically valid.
func (r *ReadinessCheck) Validate() *field.Error {
	if !r.Type.IsValid() {
		return field.Invalid(field.NewPath("type"), string(r.Type), "unknown readiness check type")
	}
	switch r.Type {
	case ReadinessCheckTypeNone:
		return nil
	// NOTE: ComposedTemplate doesn't use pointer values for optional
	// strings, so today the empty string and 0 are equivalent to "unset".
	case ReadinessCheckTypeMatchString:
		if r.MatchString == "" {
			return field.Required(field.NewPath("matchString"), "cannot be empty for type MatchString")
		}
	case ReadinessCheckTypeMatchInteger:
		if r.MatchInteger == 0 {
			return field.Required(field.NewPath("matchInteger"), "cannot be 0 for type MatchInteger")
		}
	case ReadinessCheckTypeNonEmpty:
		// No specific validation required.
	}
	if r.FieldPath == "" {
		return field.Required(field.NewPath("fieldPath"), "cannot be empty")
	}

	return nil
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

// ConnectionDetail includes the information about the propagation of the connection
// information from one secret to another.
type ConnectionDetail struct {
	// Name of the connection secret key that will be propagated to the
	// connection secret of the composition instance. Leave empty if you'd like
	// to use the same key name.
	// +optional
	Name *string `json:"name,omitempty"`

	// Type sets the connection detail fetching behaviour to be used. Each
	// connection detail type may require its own fields to be set on the
	// ConnectionDetail object. If the type is omitted Crossplane will attempt
	// to infer it based on which other fields were specified. If multiple
	// fields are specified the order of precedence is:
	// 1. FromValue
	// 2. FromConnectionSecretKey
	// 3. FromFieldPath
	// +optional
	// +kubebuilder:validation:Enum=FromConnectionSecretKey;FromFieldPath;FromValue
	Type *ConnectionDetailType `json:"type,omitempty"`

	// FromConnectionSecretKey is the key that will be used to fetch the value
	// from the composed resource's connection secret.
	// +optional
	FromConnectionSecretKey *string `json:"fromConnectionSecretKey,omitempty"`

	// FromFieldPath is the path of the field on the composed resource whose
	// value to be used as input. Name must be specified if the type is
	// FromFieldPath.
	// +optional
	FromFieldPath *string `json:"fromFieldPath,omitempty"`

	// Value that will be propagated to the connection secret of the composite
	// resource. May be set to inject a fixed, non-sensitive connection secret
	// value, for example a well-known port.
	// +optional
	Value *string `json:"value,omitempty"`
}

// A Function represents a Composition Function.
type Function struct {
	// Name of this function. Must be unique within its Composition.
	Name string `json:"name"`

	// Type of this function.
	// +kubebuilder:validation:Enum=Container
	Type FunctionType `json:"type"`

	// Config is an optional, arbitrary Kubernetes resource (i.e. a resource
	// with an apiVersion and kind) that will be passed to the Composition
	// Function as the 'config' block of its FunctionIO.
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:EmbeddedResource
	Config *runtime.RawExtension `json:"config,omitempty"`

	// Container configuration of this function.
	// +optional
	Container *ContainerFunction `json:"container,omitempty"`
}

// Validate this Function.
func (f *Function) Validate() *field.Error {
	if f.Type == FunctionTypeContainer {
		if f.Container == nil {
			return field.Required(field.NewPath("container"), "cannot be empty for type Container")
		}
		return nil
	}
	return field.Required(field.NewPath("type"), "the only supported type is Container")
}

// A FunctionType is a type of Composition Function.
type FunctionType string

// FunctionType types.
const (
	// FunctionTypeContainer represents a Composition Function that is packaged
	// as an OCI image and run in a container.
	FunctionTypeContainer FunctionType = "Container"
)

// NOTE(negz): This is intentionally much more limited than corev1.Container.
// This is because:
//
// * We always expect functions to be short-lived processes.
// * We never expect functions to listen for incoming requests.
// * We don't allow functions to mount volumes.

// A ContainerFunction represents an Composition Function that is packaged as an
// OCI image and run in a container.
type ContainerFunction struct {
	// Image specifies the OCI image in which the function is packaged. The
	// image should include an entrypoint that reads a FunctionIO from stdin and
	// emits it, optionally mutated, to stdout.
	Image string `json:"image"`

	// ImagePullPolicy defines the pull policy for the function image.
	// +optional
	// +kubebuilder:default=IfNotPresent
	// +kubebuilder:validation:Enum="IfNotPresent";"Always";"Never"
	ImagePullPolicy *corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// Timeout after which the Composition Function will be killed.
	// +optional
	// +kubebuilder:default="20s"
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// Secrets for pulling function images.
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// Network configuration for the Composition Function.
	// +optional
	Network *ContainerFunctionNetwork `json:"network,omitempty"`

	// Resources that may be used by the Composition Function.
	// +optional
	Resources *ContainerFunctionResources `json:"resources,omitempty"`

	// Runner configuration for the Composition Function.
	// +optional
	Runner *ContainerFunctionRunner `json:"runner,omitempty"`
}

// A ContainerFunctionNetworkPolicy specifies the network policy under which
// a containerized Composition Function will run.
type ContainerFunctionNetworkPolicy string

const (
	// ContainerFunctionNetworkPolicyIsolated specifies that the Composition
	// Function will not have network access; i.e. invoked inside an isolated
	// network namespace.
	ContainerFunctionNetworkPolicyIsolated ContainerFunctionNetworkPolicy = "Isolated"

	// ContainerFunctionNetworkPolicyRunner specifies that the Composition
	// Function will have the same network access as its runner, i.e. share its
	// runner's network namespace.
	ContainerFunctionNetworkPolicyRunner ContainerFunctionNetworkPolicy = "Runner"
)

// ContainerFunctionNetwork represents configuration for a Composition Function.
type ContainerFunctionNetwork struct {
	// Policy specifies the network policy under which the Composition Function
	// will run. Defaults to 'Isolated' - i.e. no network access. Specify
	// 'Runner' to allow the function the same network access as
	// its runner.
	// +optional
	// +kubebuilder:validation:Enum="Isolated";"Runner"
	// +kubebuilder:default=Isolated
	Policy *ContainerFunctionNetworkPolicy `json:"policy,omitempty"`
}

// ContainerFunctionResources represents compute resources that may be used by a
// Composition Function.
type ContainerFunctionResources struct {
	// Limits specify the maximum compute resources that may be used by the
	// Composition Function.
	// +optional
	Limits *ContainerFunctionResourceLimits `json:"limits,omitempty"`

	// NOTE(negz): We don't presently have any runners that support scheduling,
	// so we omit Requests for the time being.
}

// ContainerFunctionResourceLimits specify the maximum compute resources
// that may be used by a Composition Function.
type ContainerFunctionResourceLimits struct {
	// CPU, in cores. (500m = .5 cores)
	// +kubebuilder:default="100m"
	// +optional
	CPU *resource.Quantity `json:"cpu,omitempty"`

	// Memory, in bytes. (500Gi = 500GiB = 500 * 1024 * 1024 * 1024)
	// +kubebuilder:default="128Mi"
	// +optional
	Memory *resource.Quantity `json:"memory,omitempty"`
}

// ContainerFunctionRunner represents runner configuration for a Composition
// Function.
type ContainerFunctionRunner struct {
	// Endpoint specifies how and where Crossplane should reach the runner it
	// uses to invoke containerized Composition Functions.
	// +optional
	// +kubebuilder:default="unix-abstract:crossplane/fn/default.sock"
	Endpoint *string `json:"endpoint,omitempty"`
}

// A StoreConfigReference references a secret store config that may be used to
// write connection details.
type StoreConfigReference struct {
	// Name of the referenced StoreConfig.
	Name string `json:"name"`
}
