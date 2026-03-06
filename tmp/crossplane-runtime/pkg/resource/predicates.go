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

package resource

import (
	"maps"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
)

// A PredicateFn returns true if the supplied object should be reconciled.
//
// Deprecated: This type will be removed soon. Please use
// controller-runtime's predicate.NewPredicateFuncs instead.
type PredicateFn func(obj runtime.Object) bool

// NewPredicates returns a set of Funcs that are all satisfied by the supplied
// PredicateFn. The PredicateFn is run against the new object during updates.
//
// Deprecated: This function will be removed soon. Please use
// controller-runtime's predicate.NewPredicateFuncs instead.
func NewPredicates(fn PredicateFn) predicate.Funcs {
	return predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return fn(e.Object) },
		DeleteFunc:  func(e event.DeleteEvent) bool { return fn(e.Object) },
		UpdateFunc:  func(e event.UpdateEvent) bool { return fn(e.ObjectNew) },
		GenericFunc: func(e event.GenericEvent) bool { return fn(e.Object) },
	}
}

// DesiredStateChanged accepts objects that have changed their desired state, i.e.
// the state that is not managed by the controller.
// To be more specific, it accepts update events that have changes in one of the followings:
// - `metadata.annotations` (except for certain annotations)
// - `metadata.labels`
// - `spec`.
func DesiredStateChanged() predicate.Predicate {
	return predicate.Or(
		AnnotationChangedPredicate{
			ignored: []string{
				// These annotations are managed by the controller and should
				// not be considered as a change in desired state. The managed
				// reconciler explicitly requests a new reconcile already after
				// updating these annotations.
				meta.AnnotationKeyExternalCreateFailed,
				meta.AnnotationKeyExternalCreatePending,
			},
		},
		predicate.LabelChangedPredicate{},
		predicate.GenerationChangedPredicate{},
	)
}

// AnnotationChangedPredicate implements a default update predicate function on
// annotation change by ignoring the given annotation keys, if any.
//
// This predicate extends controller-runtime's AnnotationChangedPredicate by
// being able to ignore certain annotations.
type AnnotationChangedPredicate struct {
	predicate.Funcs

	ignored []string
}

func copyAnnotations(an map[string]string) map[string]string {
	r := make(map[string]string, len(an))
	maps.Copy(r, an)

	return r
}

// Update implements default UpdateEvent filter for validating annotation change.
func (a AnnotationChangedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil {
		// Update event has no old object to update
		return false
	}

	if e.ObjectNew == nil {
		// Update event has no new object for update
		return false
	}

	na := copyAnnotations(e.ObjectNew.GetAnnotations())
	oa := copyAnnotations(e.ObjectOld.GetAnnotations())

	for _, k := range a.ignored {
		delete(na, k)
		delete(oa, k)
	}

	// Below is the same as controller-runtime's AnnotationChangedPredicate
	// implementation but optimized to avoid using reflect.DeepEqual.
	if len(na) != len(oa) {
		// annotation length changed
		return true
	}

	for k, v := range na {
		if oa[k] != v {
			// annotation value changed
			return true
		}
	}

	// annotations unchanged.
	return false
}
