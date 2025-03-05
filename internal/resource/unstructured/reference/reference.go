/*
Copyright 2024 The Crossplane Authors.

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

// Package reference contains references to resources.
package reference

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// A Claim is a reference to a claim.
type Claim struct {
	// APIVersion of the referenced claim.
	APIVersion string `json:"apiVersion"`

	// Kind of the referenced claim.
	Kind string `json:"kind"`

	// Name of the referenced claim.
	Name string `json:"name"`

	// Namespace of the referenced claim.
	Namespace string `json:"namespace"`
}

// A Composite is a reference to a composite.
type Composite struct {
	// APIVersion of the referenced composite.
	APIVersion string `json:"apiVersion"`

	// Kind of the referenced composite.
	Kind string `json:"kind"`

	// Name of the referenced composite.
	Name string `json:"name"`
}

// GroupVersionKind returns the GroupVersionKind of the claim reference.
func (c *Claim) GroupVersionKind() schema.GroupVersionKind {
	return schema.FromAPIVersionAndKind(c.APIVersion, c.Kind)
}

// GroupVersionKind returns the GroupVersionKind of the composite reference.
func (c *Composite) GroupVersionKind() schema.GroupVersionKind {
	return schema.FromAPIVersionAndKind(c.APIVersion, c.Kind)
}
