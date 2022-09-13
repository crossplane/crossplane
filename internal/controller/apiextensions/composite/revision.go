/*
Copyright 2021 The Crossplane Authors.

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
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
)

// AsComposition creates a new composition from the supplied revision. It
// exists only as a temporary translation layer to allow us to introduce the
// alpha CompositionRevision type with minimal changes to the XR reconciler.
// Once CompositionRevision leaves alpha this code should be removed and the XR
// reconciler should operate on CompositionRevisions instead.
func AsComposition(cr *v1alpha1.CompositionRevision) *v1.Composition {
	return &v1.Composition{Spec: *cr.Spec.CompositionSpec.DeepCopy()}
}
