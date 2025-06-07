/*
Copyright 2019 The Crossplane Authors.

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

// Package xfake provides fake Crossplane resources for use in tests.
//
//nolint:musttag // We only use JSON to round-trip convert these mocks.
package xfake

import (
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"

	"github.com/crossplane/crossplane/pkg/xresource/unstructured/reference"
)

// ClaimReferencer is a mock that implements ClaimReferencer interface.
type ClaimReferencer struct{ Ref *reference.Claim }

// SetClaimReference sets the ClaimReference.
func (m *ClaimReferencer) SetClaimReference(r *reference.Claim) { m.Ref = r }

// GetClaimReference gets the ClaimReference.
func (m *ClaimReferencer) GetClaimReference() *reference.Claim { return m.Ref }

// CompositionReferencer is a mock that implements CompositionReferencer interface.
type CompositionReferencer struct{ Ref *corev1.ObjectReference }

// SetCompositionReference sets the CompositionReference.
func (m *CompositionReferencer) SetCompositionReference(r *corev1.ObjectReference) { m.Ref = r }

// GetCompositionReference gets the CompositionReference.
func (m *CompositionReferencer) GetCompositionReference() *corev1.ObjectReference { return m.Ref }

// CompositionSelector is a mock that implements CompositionSelector interface.
type CompositionSelector struct{ Sel *metav1.LabelSelector }

// SetCompositionSelector sets the CompositionSelector.
func (m *CompositionSelector) SetCompositionSelector(s *metav1.LabelSelector) { m.Sel = s }

// GetCompositionSelector gets the CompositionSelector.
func (m *CompositionSelector) GetCompositionSelector() *metav1.LabelSelector { return m.Sel }

// CompositionRevisionReferencer is a mock that implements CompositionRevisionReferencer interface.
type CompositionRevisionReferencer struct{ Ref *corev1.LocalObjectReference }

// SetCompositionRevisionReference sets the CompositionRevisionReference.
func (m *CompositionRevisionReferencer) SetCompositionRevisionReference(r *corev1.LocalObjectReference) {
	m.Ref = r
}

// GetCompositionRevisionReference gets the CompositionRevisionReference.
func (m *CompositionRevisionReferencer) GetCompositionRevisionReference() *corev1.LocalObjectReference {
	return m.Ref
}

// CompositionRevisionSelector is a mock that implements CompositionRevisionSelector interface.
type CompositionRevisionSelector struct{ Sel *metav1.LabelSelector }

// SetCompositionRevisionSelector sets the CompositionRevisionSelector.
func (m *CompositionRevisionSelector) SetCompositionRevisionSelector(ls *metav1.LabelSelector) {
	m.Sel = ls
}

// GetCompositionRevisionSelector gets the CompositionRevisionSelector.
func (m *CompositionRevisionSelector) GetCompositionRevisionSelector() *metav1.LabelSelector {
	return m.Sel
}

// CompositionUpdater is a mock that implements CompositionUpdater interface.
type CompositionUpdater struct{ Policy *xpv1.UpdatePolicy }

// SetCompositionUpdatePolicy sets the CompositionUpdatePolicy.
func (m *CompositionUpdater) SetCompositionUpdatePolicy(p *xpv1.UpdatePolicy) {
	m.Policy = p
}

// GetCompositionUpdatePolicy gets the CompositionUpdatePolicy.
func (m *CompositionUpdater) GetCompositionUpdatePolicy() *xpv1.UpdatePolicy {
	return m.Policy
}

// CompositeResourceDeleter is a mock that implements CompositeResourceDeleter interface.
type CompositeResourceDeleter struct{ Policy *xpv1.CompositeDeletePolicy }

// SetCompositeDeletePolicy sets the CompositeDeletePolicy.
func (m *CompositeResourceDeleter) SetCompositeDeletePolicy(p *xpv1.CompositeDeletePolicy) {
	m.Policy = p
}

// GetCompositeDeletePolicy gets the CompositeDeletePolicy.
func (m *CompositeResourceDeleter) GetCompositeDeletePolicy() *xpv1.CompositeDeletePolicy {
	return m.Policy
}

// CompositeResourceReferencer is a mock that implements CompositeResourceReferencer interface.
type CompositeResourceReferencer struct{ Ref *reference.Composite }

// SetResourceReference sets the composite resource reference.
func (m *CompositeResourceReferencer) SetResourceReference(p *reference.Composite) { m.Ref = p }

// GetResourceReference gets the composite resource reference.
func (m *CompositeResourceReferencer) GetResourceReference() *reference.Composite { return m.Ref }

// ComposedResourcesReferencer is a mock that implements ComposedResourcesReferencer interface.
type ComposedResourcesReferencer struct{ Refs []corev1.ObjectReference }

// SetResourceReferences sets the composed references.
func (m *ComposedResourcesReferencer) SetResourceReferences(r []corev1.ObjectReference) { m.Refs = r }

// GetResourceReferences gets the composed references.
func (m *ComposedResourcesReferencer) GetResourceReferences() []corev1.ObjectReference { return m.Refs }

// ConnectionDetailsLastPublishedTimer is a mock that implements the
// ConnectionDetailsLastPublishedTimer interface.
type ConnectionDetailsLastPublishedTimer struct {
	// NOTE: runtime.DefaultUnstructuredConverter.ToUnstructured
	// cannot currently handle if `Time` is nil here.
	// The `omitempty` json tag is a workaround that
	// prevents a panic.
	Time *metav1.Time `json:"lastPublishedTime,omitempty"`
}

// SetConnectionDetailsLastPublishedTime sets the published time.
func (c *ConnectionDetailsLastPublishedTimer) SetConnectionDetailsLastPublishedTime(t *metav1.Time) {
	c.Time = t
}

// GetConnectionDetailsLastPublishedTime gets the published time.
func (c *ConnectionDetailsLastPublishedTimer) GetConnectionDetailsLastPublishedTime() *metav1.Time {
	return c.Time
}

// Composite is a mock that implements Composite interface.
type Composite struct {
	metav1.ObjectMeta

	fake.ConnectionSecretWriterTo
	CompositionSelector
	CompositionReferencer
	CompositionRevisionReferencer
	CompositionRevisionSelector
	CompositionUpdater
	ComposedResourcesReferencer
	ClaimReferencer

	xpv1.ResourceStatus
	ConnectionDetailsLastPublishedTimer
}

// GetObjectKind returns schema.ObjectKind.
func (m *Composite) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

// DeepCopyObject returns a copy of the object as runtime.Object.
func (m *Composite) DeepCopyObject() runtime.Object {
	out := &Composite{}
	j, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	_ = json.Unmarshal(j, out)
	return out
}

// Composed is a mock that implements Composed interface.
type Composed struct {
	metav1.ObjectMeta

	fake.ConnectionSecretWriterTo

	xpv1.ResourceStatus
}

// GetObjectKind returns schema.ObjectKind.
func (m *Composed) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

// DeepCopyObject returns a copy of the object as runtime.Object.
func (m *Composed) DeepCopyObject() runtime.Object {
	out := &Composed{}
	j, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	_ = json.Unmarshal(j, out)
	return out
}

// Claim is a mock that implements the Claim interface.
type Claim struct {
	metav1.ObjectMeta

	fake.LocalConnectionSecretWriterTo
	CompositionSelector
	CompositionReferencer
	CompositionRevisionReferencer
	CompositionRevisionSelector
	CompositeResourceDeleter
	CompositionUpdater
	CompositeResourceReferencer

	xpv1.ResourceStatus
	ConnectionDetailsLastPublishedTimer
}

// GetObjectKind returns schema.ObjectKind.
func (m *Claim) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

// DeepCopyObject returns a copy of the object as runtime.Object.
func (m *Claim) DeepCopyObject() runtime.Object {
	out := &Claim{}
	j, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	_ = json.Unmarshal(j, out)
	return out
}
