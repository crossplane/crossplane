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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	opsv1alpha1 "github.com/crossplane/crossplane/apis/v2/ops/v1alpha1"
	"github.com/crossplane/crossplane/v2/internal/render"
)

// API version and kind for the render input/output envelopes.
const (
	APIVersion = "render.crossplane.io/v1alpha1"
	KindInput  = "OperationRenderInput"
	KindOutput = "OperationRenderOutput"
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
