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

// Package dependency contains an unstructured dependency resource.
package dependency

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// An Option modifies an unstructured composed resource.
type Option func(resource *Unstructured)

// FromReference returns an Option that propagates the metadata in the supplied
// reference to an unstructured composed resource.
func FromReference(ref corev1.ObjectReference) Option {
	return func(cr *Unstructured) {
		cr.SetGroupVersionKind(ref.GroupVersionKind())
		cr.SetName(ref.Name)
		cr.SetNamespace(ref.Namespace)
		cr.SetUID(ref.UID)
	}
}

// New returns a new unstructured composed resource.
func New(opts ...Option) *Unstructured {
	cr := &Unstructured{unstructured.Unstructured{Object: make(map[string]any)}}
	for _, f := range opts {
		f(cr)
	}
	return cr
}

// An Unstructured composed resource.
type Unstructured struct {
	unstructured.Unstructured
}

// GetUnstructured returns the underlying *unstructured.Unstructured.
func (cr *Unstructured) GetUnstructured() *unstructured.Unstructured {
	return &cr.Unstructured
}

func (cr *Unstructured) OwnedBy(u types.UID) bool {
	for _, owner := range cr.GetOwnerReferences() {
		if owner.UID == u {
			return true
		}
	}
	return false
}
