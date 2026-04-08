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

package composite

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
	"github.com/crossplane/crossplane/v2/internal/render"
)

// API version and kind for the render input/output envelopes.
const (
	APIVersion = "render.crossplane.io/v1alpha1"
	KindInput  = "CompositeInput"
	KindOutput = "CompositeOutput"
)

// Input is a structured envelope for all inputs to the composite resource
// render process.
type Input struct {
	APIVersion string `json:"apiVersion" yaml:"apiVersion"`
	Kind       string `json:"kind"       yaml:"kind"`

	// CompositeResource is the XR to reconcile.
	CompositeResource unstructured.Unstructured `json:"compositeResource" yaml:"compositeResource"`

	// Composition is the Composition to use.
	Composition apiextensionsv1.Composition `json:"composition" yaml:"composition"`

	// Functions maps function names to gRPC addresses. The caller is
	// responsible for starting function runtimes and providing their
	// addresses.
	Functions []render.FunctionInput `json:"functions" yaml:"functions"`

	// ObservedResources are existing composed resources from a previous
	// reconcile. Optional.
	ObservedResources []unstructured.Unstructured `json:"observedResources,omitempty" yaml:"observedResources,omitempty"`

	// RequiredResources are resources available for functions that request
	// them via the Requirements protocol. Optional.
	RequiredResources []unstructured.Unstructured `json:"requiredResources,omitempty" yaml:"requiredResources,omitempty"`

	// Credentials are Kubernetes Secrets for function credentials. Optional.
	Credentials []corev1.Secret `json:"credentials,omitempty" yaml:"credentials,omitempty"`

	// Context contains key-value pairs to seed the function pipeline context.
	// Each value is a raw JSON/YAML value. Optional.
	Context map[string]runtime.RawExtension `json:"context,omitempty" yaml:"context,omitempty"`

	// ExtraResources are additional resources to load into the fake client's
	// store. These are available for functions via the Requirements protocol,
	// and for any other client.Get/List calls during reconciliation. Optional.
	ExtraResources []unstructured.Unstructured `json:"extraResources,omitempty" yaml:"extraResources,omitempty"`
}

// Output is a structured envelope for all outputs from the composite resource
// render process.
type Output struct {
	APIVersion string `json:"apiVersion" yaml:"apiVersion"`
	Kind       string `json:"kind"       yaml:"kind"`

	// CompositeResource is the XR with desired status and conditions set by
	// the reconciler.
	CompositeResource unstructured.Unstructured `json:"compositeResource" yaml:"compositeResource"`

	// ComposedResources are the composed resources the reconciler would
	// apply via server-side apply.
	ComposedResources []unstructured.Unstructured `json:"composedResources" yaml:"composedResources"`

	// DeletedResources are composed resources the reconciler would garbage
	// collect.
	DeletedResources []unstructured.Unstructured `json:"deletedResources,omitempty" yaml:"deletedResources,omitempty"`

	// Events are the Kubernetes events the reconciler would emit.
	Events []render.OutputEvent `json:"events,omitempty" yaml:"events,omitempty"`

	// Context is the function pipeline context after the last function ran.
	Context map[string]any `json:"context,omitempty" yaml:"context,omitempty"`
}
