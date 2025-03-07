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

// Package claim contains an unstructured composite resource claim.
package claim

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"

	"github.com/crossplane/crossplane/internal/xresource/unstructured/reference"
)

// An Option modifies an unstructured composite resource claim.
type Option func(*Unstructured)

// WithGroupVersionKind sets the GroupVersionKind of the unstructured composite
// resource claim.
func WithGroupVersionKind(gvk schema.GroupVersionKind) Option {
	return func(c *Unstructured) {
		c.SetGroupVersionKind(gvk)
	}
}

// WithConditions returns an Option that sets the supplied conditions on an
// unstructured composite resource claim.
func WithConditions(c ...xpv1.Condition) Option {
	return func(cr *Unstructured) {
		cr.SetConditions(c...)
	}
}

// New returns a new unstructured composite resource claim.
func New(opts ...Option) *Unstructured {
	c := &Unstructured{Unstructured: unstructured.Unstructured{Object: make(map[string]any)}}
	for _, f := range opts {
		f(c)
	}
	return c
}

// +k8s:deepcopy-gen=true
// +kubebuilder:object:root=true

// An Unstructured composite resource claim.
type Unstructured struct {
	unstructured.Unstructured
}

// GetUnstructured returns the underlying *unstructured.Unstructured.
func (c *Unstructured) GetUnstructured() *unstructured.Unstructured {
	return &c.Unstructured
}

// GetCompositionSelector of this composite resource claim.
func (c *Unstructured) GetCompositionSelector() *metav1.LabelSelector {
	out := &metav1.LabelSelector{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec.compositionSelector", out); err != nil {
		return nil
	}
	return out
}

// SetCompositionSelector of this composite resource claim.
func (c *Unstructured) SetCompositionSelector(sel *metav1.LabelSelector) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.compositionSelector", sel)
}

// GetCompositionReference of this composite resource claim.
func (c *Unstructured) GetCompositionReference() *corev1.ObjectReference {
	out := &corev1.ObjectReference{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec.compositionRef", out); err != nil {
		return nil
	}
	return out
}

// SetCompositionReference of this composite resource claim.
func (c *Unstructured) SetCompositionReference(ref *corev1.ObjectReference) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.compositionRef", ref)
}

// GetCompositionRevisionReference of this resource claim.
func (c *Unstructured) GetCompositionRevisionReference() *corev1.LocalObjectReference {
	out := &corev1.LocalObjectReference{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec.compositionRevisionRef", out); err != nil {
		return nil
	}
	return out
}

// SetCompositionRevisionReference of this resource claim.
func (c *Unstructured) SetCompositionRevisionReference(ref *corev1.LocalObjectReference) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.compositionRevisionRef", ref)
}

// GetCompositionRevisionSelector of this resource claim.
func (c *Unstructured) GetCompositionRevisionSelector() *metav1.LabelSelector {
	out := &metav1.LabelSelector{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec.compositionRevisionSelector", out); err != nil {
		return nil
	}
	return out
}

// SetCompositionRevisionSelector of this resource claim.
func (c *Unstructured) SetCompositionRevisionSelector(ref *metav1.LabelSelector) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.compositionRevisionSelector", ref)
}

// SetCompositionUpdatePolicy of this resource claim.
func (c *Unstructured) SetCompositionUpdatePolicy(p *xpv1.UpdatePolicy) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.compositionUpdatePolicy", p)
}

// GetCompositionUpdatePolicy of this resource claim.
func (c *Unstructured) GetCompositionUpdatePolicy() *xpv1.UpdatePolicy {
	p, err := fieldpath.Pave(c.Object).GetString("spec.compositionUpdatePolicy")
	if err != nil {
		return nil
	}
	out := xpv1.UpdatePolicy(p)
	return &out
}

// SetCompositeDeletePolicy of this resource claim.
func (c *Unstructured) SetCompositeDeletePolicy(p *xpv1.CompositeDeletePolicy) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.compositeDeletePolicy", p)
}

// GetCompositeDeletePolicy of this resource claim.
func (c *Unstructured) GetCompositeDeletePolicy() *xpv1.CompositeDeletePolicy {
	p, err := fieldpath.Pave(c.Object).GetString("spec.compositeDeletePolicy")
	if err != nil {
		return nil
	}
	out := xpv1.CompositeDeletePolicy(p)
	return &out
}

// GetResourceReference of this composite resource claim.
func (c *Unstructured) GetResourceReference() *reference.Composite {
	out := &reference.Composite{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec.resourceRef", out); err != nil {
		return nil
	}
	return out
}

// SetResourceReference of this composite resource claim.
func (c *Unstructured) SetResourceReference(ref *reference.Composite) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.resourceRef", ref)
}

// GetReference returns reference to this claim.
func (c *Unstructured) GetReference() *reference.Claim {
	return &reference.Claim{
		APIVersion: c.GetAPIVersion(),
		Kind:       c.GetKind(),
		Name:       c.GetName(),
		Namespace:  c.GetNamespace(),
	}
}

// GetWriteConnectionSecretToReference of this composite resource claim.
func (c *Unstructured) GetWriteConnectionSecretToReference() *xpv1.LocalSecretReference {
	out := &xpv1.LocalSecretReference{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec.writeConnectionSecretToRef", out); err != nil {
		return nil
	}
	return out
}

// SetWriteConnectionSecretToReference of this composite resource claim.
func (c *Unstructured) SetWriteConnectionSecretToReference(ref *xpv1.LocalSecretReference) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.writeConnectionSecretToRef", ref)
}

// GetCondition of this composite resource claim.
func (c *Unstructured) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	conditioned := xpv1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	if err := fieldpath.Pave(c.Object).GetValueInto("status", &conditioned); err != nil {
		return xpv1.Condition{}
	}
	return conditioned.GetCondition(ct)
}

// SetConditions of this composite resource claim.
func (c *Unstructured) SetConditions(conditions ...xpv1.Condition) {
	conditioned := xpv1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	_ = fieldpath.Pave(c.Object).GetValueInto("status", &conditioned)
	conditioned.SetConditions(conditions...)
	_ = fieldpath.Pave(c.Object).SetValue("status.conditions", conditioned.Conditions)
}

// GetConnectionDetailsLastPublishedTime of this composite resource claim.
func (c *Unstructured) GetConnectionDetailsLastPublishedTime() *metav1.Time {
	out := &metav1.Time{}
	if err := fieldpath.Pave(c.Object).GetValueInto("status.connectionDetails.lastPublishedTime", out); err != nil {
		return nil
	}
	return out
}

// SetConnectionDetailsLastPublishedTime of this composite resource claim.
func (c *Unstructured) SetConnectionDetailsLastPublishedTime(t *metav1.Time) {
	_ = fieldpath.Pave(c.Object).SetValue("status.connectionDetails.lastPublishedTime", t)
}

// SetObservedGeneration of this composite resource claim.
func (c *Unstructured) SetObservedGeneration(generation int64) {
	status := &xpv1.ObservedStatus{}
	_ = fieldpath.Pave(c.Object).GetValueInto("status", status)
	status.SetObservedGeneration(generation)
	_ = fieldpath.Pave(c.Object).SetValue("status.observedGeneration", status.ObservedGeneration)
}

// GetObservedGeneration of this composite resource claim.
func (c *Unstructured) GetObservedGeneration() int64 {
	status := &xpv1.ObservedStatus{}
	_ = fieldpath.Pave(c.Object).GetValueInto("status", status)
	return status.GetObservedGeneration()
}
