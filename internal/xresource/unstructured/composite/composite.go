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

// Package composite contains an unstructured composite resource.
package composite

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"

	"github.com/crossplane/crossplane/internal/xresource/unstructured/reference"
)

// An Option modifies an unstructured composite resource.
type Option func(*Unstructured)

// WithGroupVersionKind sets the GroupVersionKind of the unstructured composite
// resource.
func WithGroupVersionKind(gvk schema.GroupVersionKind) Option {
	return func(c *Unstructured) {
		c.SetGroupVersionKind(gvk)
	}
}

// WithConditions returns an Option that sets the supplied conditions on an
// unstructured composite resource.
func WithConditions(c ...xpv1.Condition) Option {
	return func(cr *Unstructured) {
		cr.SetConditions(c...)
	}
}

// New returns a new unstructured composed resource.
func New(opts ...Option) *Unstructured {
	c := &Unstructured{unstructured.Unstructured{Object: make(map[string]any)}}
	for _, f := range opts {
		f(c)
	}
	return c
}

// +k8s:deepcopy-gen=true
// +kubebuilder:object:root=true

// An Unstructured composed resource.
type Unstructured struct {
	unstructured.Unstructured
}

// GetUnstructured returns the underlying *unstructured.Unstructured.
func (c *Unstructured) GetUnstructured() *unstructured.Unstructured {
	return &c.Unstructured
}

// GetCompositionSelector of this Composite resource.
func (c *Unstructured) GetCompositionSelector() *metav1.LabelSelector {
	out := &metav1.LabelSelector{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec.compositionSelector", out); err != nil {
		return nil
	}
	return out
}

// SetCompositionSelector of this Composite resource.
func (c *Unstructured) SetCompositionSelector(sel *metav1.LabelSelector) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.compositionSelector", sel)
}

// GetCompositionReference of this Composite resource.
func (c *Unstructured) GetCompositionReference() *corev1.ObjectReference {
	out := &corev1.ObjectReference{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec.compositionRef", out); err != nil {
		return nil
	}
	return out
}

// SetCompositionReference of this Composite resource.
func (c *Unstructured) SetCompositionReference(ref *corev1.ObjectReference) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.compositionRef", ref)
}

// GetCompositionRevisionReference of this Composite resource.
func (c *Unstructured) GetCompositionRevisionReference() *corev1.LocalObjectReference {
	out := &corev1.LocalObjectReference{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec.compositionRevisionRef", out); err != nil {
		return nil
	}
	return out
}

// SetCompositionRevisionReference of this Composite resource.
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
func (c *Unstructured) SetCompositionRevisionSelector(sel *metav1.LabelSelector) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.compositionRevisionSelector", sel)
}

// SetCompositionUpdatePolicy of this Composite resource.
func (c *Unstructured) SetCompositionUpdatePolicy(p *xpv1.UpdatePolicy) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.compositionUpdatePolicy", p)
}

// GetCompositionUpdatePolicy of this Composite resource.
func (c *Unstructured) GetCompositionUpdatePolicy() *xpv1.UpdatePolicy {
	p, err := fieldpath.Pave(c.Object).GetString("spec.compositionUpdatePolicy")
	if err != nil {
		return nil
	}
	out := xpv1.UpdatePolicy(p)
	return &out
}

// GetClaimReference of this Composite resource.
func (c *Unstructured) GetClaimReference() *reference.Claim {
	out := &reference.Claim{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec.claimRef", out); err != nil {
		return nil
	}
	return out
}

// SetClaimReference of this Composite resource.
func (c *Unstructured) SetClaimReference(ref *reference.Claim) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.claimRef", ref)
}

// GetResourceReferences of this Composite resource.
func (c *Unstructured) GetResourceReferences() []corev1.ObjectReference {
	out := &[]corev1.ObjectReference{}
	_ = fieldpath.Pave(c.Object).GetValueInto("spec.resourceRefs", out)
	return *out
}

// SetResourceReferences of this Composite resource.
func (c *Unstructured) SetResourceReferences(refs []corev1.ObjectReference) {
	empty := corev1.ObjectReference{}
	filtered := make([]corev1.ObjectReference, 0, len(refs))
	for _, ref := range refs {
		// TODO(negz): Ask muvaf to explain what this is working around. :)
		// TODO(muvaf): temporary workaround.
		if ref.String() == empty.String() {
			continue
		}
		filtered = append(filtered, ref)
	}
	_ = fieldpath.Pave(c.Object).SetValue("spec.resourceRefs", filtered)
}

// GetReference returns reference to this composite.
func (c *Unstructured) GetReference() *reference.Composite {
	return &reference.Composite{
		APIVersion: c.GetAPIVersion(),
		Kind:       c.GetKind(),
		Name:       c.GetName(),
	}
}

// GetWriteConnectionSecretToReference of this Composite resource.
func (c *Unstructured) GetWriteConnectionSecretToReference() *xpv1.SecretReference {
	out := &xpv1.SecretReference{}
	if err := fieldpath.Pave(c.Object).GetValueInto("spec.writeConnectionSecretToRef", out); err != nil {
		return nil
	}
	return out
}

// SetWriteConnectionSecretToReference of this Composite resource.
func (c *Unstructured) SetWriteConnectionSecretToReference(ref *xpv1.SecretReference) {
	_ = fieldpath.Pave(c.Object).SetValue("spec.writeConnectionSecretToRef", ref)
}

// GetCondition of this Composite resource.
func (c *Unstructured) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	conditioned := xpv1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	if err := fieldpath.Pave(c.Object).GetValueInto("status", &conditioned); err != nil {
		return xpv1.Condition{}
	}
	return conditioned.GetCondition(ct)
}

// SetConditions of this Composite resource.
func (c *Unstructured) SetConditions(conditions ...xpv1.Condition) {
	conditioned := xpv1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	_ = fieldpath.Pave(c.Object).GetValueInto("status", &conditioned)
	conditioned.SetConditions(conditions...)
	_ = fieldpath.Pave(c.Object).SetValue("status.conditions", conditioned.Conditions)
}

// GetConditions of this Composite resource.
func (c *Unstructured) GetConditions() []xpv1.Condition {
	conditioned := xpv1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	_ = fieldpath.Pave(c.Object).GetValueInto("status", &conditioned)
	return conditioned.Conditions
}

// GetConnectionDetailsLastPublishedTime of this Composite resource.
func (c *Unstructured) GetConnectionDetailsLastPublishedTime() *metav1.Time {
	out := &metav1.Time{}
	if err := fieldpath.Pave(c.Object).GetValueInto("status.connectionDetails.lastPublishedTime", out); err != nil {
		return nil
	}
	return out
}

// SetConnectionDetailsLastPublishedTime of this Composite resource.
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

// SetClaimConditionTypes of this Composite resource. You cannot set system
// condition types such as Ready, Synced or Healthy as claim conditions.
func (c *Unstructured) SetClaimConditionTypes(in ...xpv1.ConditionType) error {
	ts := c.GetClaimConditionTypes()
	m := make(map[xpv1.ConditionType]bool, len(ts))
	for _, t := range ts {
		m[t] = true
	}

	for _, t := range in {
		if xpv1.IsSystemConditionType(t) {
			return errors.Errorf("cannot set system condition %s as a claim condition", t)
		}
		if m[t] {
			continue
		}
		m[t] = true
		ts = append(ts, t)
	}
	_ = fieldpath.Pave(c.Object).SetValue("status.claimConditionTypes", ts)
	return nil
}

// GetClaimConditionTypes of this Composite resource.
func (c *Unstructured) GetClaimConditionTypes() []xpv1.ConditionType {
	cs := []xpv1.ConditionType{}
	_ = fieldpath.Pave(c.Object).GetValueInto("status.claimConditionTypes", &cs)
	return cs
}
