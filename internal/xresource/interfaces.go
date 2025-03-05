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

package xresource

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/crossplane/internal/xresource/unstructured/reference"
)

// A ClaimReferencer may reference a resource claim.
type ClaimReferencer interface {
	SetClaimReference(r *reference.Claim)
	GetClaimReference() *reference.Claim
}

// A CompositionSelector may select a composition of resources.
type CompositionSelector interface {
	SetCompositionSelector(s *metav1.LabelSelector)
	GetCompositionSelector() *metav1.LabelSelector
}

// A CompositionReferencer may reference a composition of resources.
type CompositionReferencer interface {
	SetCompositionReference(ref *corev1.ObjectReference)
	GetCompositionReference() *corev1.ObjectReference
}

// A CompositionRevisionReferencer may reference a specific revision of a
// composition of resources.
type CompositionRevisionReferencer interface {
	SetCompositionRevisionReference(ref *corev1.LocalObjectReference)
	GetCompositionRevisionReference() *corev1.LocalObjectReference
}

// A CompositionRevisionSelector may reference a set of
// composition revisions.
type CompositionRevisionSelector interface {
	SetCompositionRevisionSelector(selector *metav1.LabelSelector)
	GetCompositionRevisionSelector() *metav1.LabelSelector
}

// A CompositionUpdater uses a composition, and may update which revision of
// that composition it uses.
type CompositionUpdater interface {
	SetCompositionUpdatePolicy(p *xpv1.UpdatePolicy)
	GetCompositionUpdatePolicy() *xpv1.UpdatePolicy
}

// A CompositeResourceDeleter creates a composite, and controls the policy
// used to delete the composite.
type CompositeResourceDeleter interface {
	SetCompositeDeletePolicy(policy *xpv1.CompositeDeletePolicy)
	GetCompositeDeletePolicy() *xpv1.CompositeDeletePolicy
}

// A ComposedResourcesReferencer may reference the resources it composes.
type ComposedResourcesReferencer interface {
	SetResourceReferences(refs []corev1.ObjectReference)
	GetResourceReferences() []corev1.ObjectReference
}

// A CompositeResourceReferencer can reference a composite resource.
type CompositeResourceReferencer interface {
	SetResourceReference(r *reference.Composite)
	GetResourceReference() *reference.Composite
}

// A UserCounter can count how many users it has.
type UserCounter interface {
	SetUsers(i int64)
	GetUsers() int64
}

// A ConnectionDetailsPublishedTimer can record the last time its connection
// details were published.
type ConnectionDetailsPublishedTimer interface {
	SetConnectionDetailsLastPublishedTime(t *metav1.Time)
	GetConnectionDetailsLastPublishedTime() *metav1.Time
}

// A Composite resource composes one or more Composed resources.
type Composite interface { //nolint:interfacebloat // This interface has to be big.
	resource.Object

	resource.ConnectionSecretWriterTo
	CompositionSelector
	CompositionReferencer
	CompositionUpdater
	CompositionRevisionReferencer
	CompositionRevisionSelector
	ComposedResourcesReferencer
	ClaimReferencer

	resource.Conditioned
	ConnectionDetailsPublishedTimer
}

// Composed resources can be a composed into a Composite resource.
type Composed interface {
	resource.Object

	resource.ConnectionSecretWriterTo

	resource.Conditioned
}

// A Claim for a composite resource.
type Claim interface { //nolint:interfacebloat // This interface has to be big.
	resource.Object

	resource.LocalConnectionSecretWriterTo

	CompositionSelector
	CompositionReferencer
	CompositionUpdater
	CompositionRevisionReferencer
	CompositionRevisionSelector
	CompositeResourceDeleter
	CompositeResourceReferencer

	resource.Conditioned
	ConnectionDetailsPublishedTimer
}
