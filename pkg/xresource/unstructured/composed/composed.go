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

// Package composed contains an unstructured composed resource.
package composed

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
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

// WithConditions returns an Option that sets the supplied conditions on an
// unstructured composed resource.
func WithConditions(c ...xpv1.Condition) Option {
	return func(cr *Unstructured) {
		cr.SetConditions(c...)
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

// +k8s:deepcopy-gen=true
// +kubebuilder:object:root=true

// An Unstructured composed resource.
type Unstructured struct {
	unstructured.Unstructured
}

// GetUnstructured returns the underlying *unstructured.Unstructured.
func (cr *Unstructured) GetUnstructured() *unstructured.Unstructured {
	return &cr.Unstructured
}

// GetCondition of this Composed resource.
func (cr *Unstructured) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	conditioned := xpv1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	if err := fieldpath.Pave(cr.Object).GetValueInto("status", &conditioned); err != nil {
		return xpv1.Condition{}
	}
	return conditioned.GetCondition(ct)
}

// SetConditions of this Composed resource.
func (cr *Unstructured) SetConditions(c ...xpv1.Condition) {
	conditioned := xpv1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	_ = fieldpath.Pave(cr.Object).GetValueInto("status", &conditioned)
	conditioned.SetConditions(c...)
	_ = fieldpath.Pave(cr.Object).SetValue("status.conditions", conditioned.Conditions)
}

// GetWriteConnectionSecretToReference of this Composed resource.
func (cr *Unstructured) GetWriteConnectionSecretToReference() *xpv1.SecretReference {
	out := &xpv1.SecretReference{}
	if err := fieldpath.Pave(cr.Object).GetValueInto("spec.writeConnectionSecretToRef", out); err != nil {
		return nil
	}
	return out
}

// SetWriteConnectionSecretToReference of this Composed resource.
func (cr *Unstructured) SetWriteConnectionSecretToReference(r *xpv1.SecretReference) {
	_ = fieldpath.Pave(cr.Object).SetValue("spec.writeConnectionSecretToRef", r)
}

// OwnedBy returns true if the supplied UID is an owner of the composed.
func (cr *Unstructured) OwnedBy(u types.UID) bool {
	for _, owner := range cr.GetOwnerReferences() {
		if owner.UID == u {
			return true
		}
	}
	return false
}

// RemoveOwnerRef removes the supplied UID from the composed resource's owner.
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

// SetObservedGeneration of this composite resource claim.
func (cr *Unstructured) SetObservedGeneration(generation int64) {
	status := &xpv1.ObservedStatus{}
	_ = fieldpath.Pave(cr.Object).GetValueInto("status", status)
	status.SetObservedGeneration(generation)
	_ = fieldpath.Pave(cr.Object).SetValue("status.observedGeneration", status.ObservedGeneration)
}

// GetObservedGeneration of this composite resource claim.
func (cr *Unstructured) GetObservedGeneration() int64 {
	status := &xpv1.ObservedStatus{}
	_ = fieldpath.Pave(cr.Object).GetValueInto("status", status)
	return status.GetObservedGeneration()
}
