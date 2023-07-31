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

// Package resource contains an unstructured resource.
package resource

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// An Option modifies an unstructured composed resource.
type Option func(*Unstructured)

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

// OwnedBy returns true if the supplied UID is an owner of the composed
func (cr *Unstructured) OwnedBy(u types.UID) bool {
	for _, owner := range cr.GetOwnerReferences() {
		if owner.UID == u {
			return true
		}
	}
	return false
}

// RemoveOwnerRef removes the supplied UID from the composed resource's owner
func (cr *Unstructured) RemoveOwnerRef(u types.UID) {
	refs := cr.GetOwnerReferences()
	for i := range refs {
		if refs[i].UID == u {
			cr.SetOwnerReferences(append(refs[:i], refs[i+1:]...))
			return
		}
	}
}

// An ListOption modifies an unstructured list of composed resource.
type ListOption func(*UnstructuredList)

// FromReferenceToList returns a ListOption that propagates the metadata in the
// supplied reference to an unstructured list composed resource.
func FromReferenceToList(ref corev1.ObjectReference) ListOption {
	return func(list *UnstructuredList) {
		list.SetAPIVersion(ref.APIVersion)
		list.SetKind(ref.Kind + "List")
	}
}

// NewList returns a new unstructured list of composed resources.
func NewList(opts ...ListOption) *UnstructuredList {
	cr := &UnstructuredList{unstructured.UnstructuredList{Object: make(map[string]any)}}
	for _, f := range opts {
		f(cr)
	}
	return cr
}

// An UnstructuredList of composed resources.
type UnstructuredList struct {
	unstructured.UnstructuredList
}

// GetUnstructuredList returns the underlying *unstructured.Unstructured.
func (cr *UnstructuredList) GetUnstructuredList() *unstructured.UnstructuredList {
	return &cr.UnstructuredList
}
