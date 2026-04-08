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

package operation

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	opsv1alpha1 "github.com/crossplane/crossplane/apis/v2/ops/v1alpha1"
	"github.com/crossplane/crossplane/v2/internal/render"
)

// API version and kind for the render input/output envelopes.
const (
	APIVersion               = "render.crossplane.io/v1alpha1"
	KindInput                = "OperationInput"
	KindOutput               = "OperationOutput"
	KindCronOperationInput   = "CronOperationInput"
	KindCronOperationOutput  = "CronOperationOutput"
	KindWatchOperationInput  = "WatchOperationInput"
	KindWatchOperationOutput = "WatchOperationOutput"
)

// Input is a structured envelope for all inputs to the Operation render
// process.
type Input struct {
	APIVersion string `json:"apiVersion" yaml:"apiVersion"`
	Kind       string `json:"kind"       yaml:"kind"`

	// Operation to reconcile.
	Operation opsv1alpha1.Operation `json:"operation" yaml:"operation"`

	// Functions maps function names to gRPC addresses. The caller is
	// responsible for starting function runtimes and providing their
	// addresses.
	Functions []render.FunctionInput `json:"functions" yaml:"functions"`

	// RequiredResources are resources available for functions that request
	// them via the Requirements protocol. Optional.
	RequiredResources []unstructured.Unstructured `json:"requiredResources,omitempty" yaml:"requiredResources,omitempty"`

	// Credentials are Kubernetes Secrets for function credentials. Optional.
	Credentials []corev1.Secret `json:"credentials,omitempty" yaml:"credentials,omitempty"`

	// Context contains key-value pairs to seed the function pipeline context.
	// Each value is a raw JSON/YAML value. Optional.
	Context map[string]runtime.RawExtension `json:"context,omitempty" yaml:"context,omitempty"`
}

// Output is a structured envelope for all outputs from the Operation render
// process.
type Output struct {
	APIVersion string `json:"apiVersion" yaml:"apiVersion"`
	Kind       string `json:"kind"       yaml:"kind"`

	// Operation is the Operation with status set by the reconciler.
	Operation unstructured.Unstructured `json:"operation" yaml:"operation"`

	// AppliedResources are the resources the Operation would server-side
	// apply.
	AppliedResources []unstructured.Unstructured `json:"appliedResources" yaml:"appliedResources"`

	// Events are the Kubernetes events the reconciler would emit.
	Events []render.OutputEvent `json:"events,omitempty" yaml:"events,omitempty"`
}

// CronOperationInput is the input for rendering a CronOperation. The output
// is the Operation the CronOperation would create.
type CronOperationInput struct {
	APIVersion string `json:"apiVersion" yaml:"apiVersion"`
	Kind       string `json:"kind"       yaml:"kind"`

	// CronOperation to render.
	CronOperation opsv1alpha1.CronOperation `json:"cronOperation" yaml:"cronOperation"`

	// ScheduledTime is the time to use for the Operation's name. If not
	// set, the current time is used.
	ScheduledTime *metav1.Time `json:"scheduledTime,omitempty" yaml:"scheduledTime,omitempty"`
}

// WatchOperationInput is the input for rendering a WatchOperation. The output
// is the Operation the WatchOperation would create in response to a change to
// the watched resource.
type WatchOperationInput struct {
	APIVersion string `json:"apiVersion" yaml:"apiVersion"`
	Kind       string `json:"kind"       yaml:"kind"`

	// WatchOperation to render.
	WatchOperation opsv1alpha1.WatchOperation `json:"watchOperation" yaml:"watchOperation"`

	// WatchedResource is the resource whose change triggered the
	// WatchOperation. It is injected into every pipeline step's requirements
	// as "ops.crossplane.io/watched-resource".
	WatchedResource unstructured.Unstructured `json:"watchedResource" yaml:"watchedResource"`
}

// CronOperationOutput wraps an Operation produced by rendering a
// CronOperation.
type CronOperationOutput struct {
	APIVersion string `json:"apiVersion" yaml:"apiVersion"`
	Kind       string `json:"kind"       yaml:"kind"`

	// Operation is the Operation the CronOperation would create.
	Operation opsv1alpha1.Operation `json:"operation" yaml:"operation"`
}

// WatchOperationOutput wraps an Operation produced by rendering a
// WatchOperation.
type WatchOperationOutput struct {
	APIVersion string `json:"apiVersion" yaml:"apiVersion"`
	Kind       string `json:"kind"       yaml:"kind"`

	// Operation is the Operation the WatchOperation would create.
	Operation opsv1alpha1.Operation `json:"operation" yaml:"operation"`
}
