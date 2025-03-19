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

// Package protection contains API types that protect resources from deletion.
package protection

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

const (
	// InUseIndexKey is a controller-runtime cache index key. Use it to index
	// usages by the 'of' resource - the resource being used. This allows you to
	// quickly determine whether a usage of a resource exists.
	InUseIndexKey = "inuse.apiversion.kind.name"

	// AnnotationKeyDeletionAttempt is the annotation key used to record whether
	// a deletion attempt was made and blocked by the Usage. The value stored is
	// the propagation policy used with the deletion attempt.
	AnnotationKeyDeletionAttempt = "usage.crossplane.io/deletion-attempt-with-policy"
)

// InUseIndexValue returns a controller-runtime cache index value. Use it to
// index usages by the 'of' resource - the resource being used. This allows you
// to quickly determine whether a usage of a resource exists. The supplied
// apiVersion, kind, and name should represent the resource being used.
func InUseIndexValue(apiVersion, kind, name string) string {
	// There are two sources for "apiVersion" input, one is from the
	// unstructured objects fetched from k8s and the other is from the Usage
	// spec. The one from the objects from k8s is already validated by the k8s
	// API server, so we don't need to validate it again. The one from the Usage
	// spec is validated by the Usage controller, so we don't need to validate
	// it as well. So we can safely ignore the error here. Another reason to
	// ignore the error is that the IndexerFunc using this value to index the
	// objects does not return an error, so we cannot bubble up the error here.
	gr, _ := schema.ParseGroupVersion(apiVersion)
	return fmt.Sprintf("%s.%s.%s", gr.Group, kind, name)
}

// ResourceRef is a reference to a resource.
type ResourceRef struct {
	// Name of the referent.
	Name string

	// Namespace of the referent.
	// Omit for cluster scoped referents, or referents in the same namespace.
	Namespace *string
}

// ResourceSelector is a selector to a resource.
type ResourceSelector struct {
	// MatchLabels ensures an object with matching labels is selected.
	MatchLabels map[string]string

	// MatchControllerRef ensures an object with the same controller reference
	// as the selecting object is selected.
	MatchControllerRef *bool

	// Namespace of the referent.
	// Omit for cluster scoped referents, or to match referents across all
	// namespaces.
	Namespace *string
}

// Resource defines a cluster-scoped resource.
type Resource struct {
	// API version of the referent.
	APIVersion string

	// Kind of the referent.
	Kind string

	// Reference to the resource.
	ResourceRef *ResourceRef

	// Selector to the resource.
	// This field will be ignored if ResourceRef is set.
	ResourceSelector *ResourceSelector
}

// A User of a resource.
type User interface {
	GetUserOf() Resource
	SetUserOf(r Resource)
}

// Used by a resource.
type Used interface {
	GetUsedBy() *Resource
	SetUsedBy(r *Resource)
}

// A DeletionReplayer can replay deletes.
type DeletionReplayer interface {
	GetReplayDeletion() *bool
	SetReplayDeletion(replay *bool)
}

// A Reasoned resource has an optional reason.
type Reasoned interface {
	GetReason() *string
	SetReason(reason *string)
}

// A Usage represents that a resource is in use.
type Usage interface { //nolint:interfacebloat // This represents an API type - it has to be large.
	resource.Object

	User
	Used
	Reasoned
	DeletionReplayer

	resource.Conditioned
}
