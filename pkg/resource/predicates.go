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
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// A PredicateFn returns true if the supplied object should be reconciled.
type PredicateFn func(obj runtime.Object) bool

// NewPredicates returns a set of Funcs that are all satisfied by the supplied
// PredicateFn. The PredicateFn is run against the new object during updates.
func NewPredicates(fn PredicateFn) predicate.Funcs {
	return predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return fn(e.Object) },
		DeleteFunc:  func(e event.DeleteEvent) bool { return fn(e.Object) },
		UpdateFunc:  func(e event.UpdateEvent) bool { return fn(e.ObjectNew) },
		GenericFunc: func(e event.GenericEvent) bool { return fn(e.Object) },
	}
}

// ObjectHasClassKind returns a PredicateFn implemented by HasClassKind.
func ObjectHasClassKind(k ClassKind) PredicateFn {
	return func(obj runtime.Object) bool {
		cr, ok := obj.(ClassReferencer)
		if !ok {
			return false
		}

		ref := cr.GetClassReference()
		if ref == nil {
			return false
		}

		gvk := ref.GroupVersionKind()

		if gvk.Group != k.Group {
			return false
		}
		if gvk.Version != k.Version {
			return false
		}
		return gvk.Kind == k.Kind
	}
}

// NoClassReference accepts ResourceClaims that do not reference a specific ResourceClass
func NoClassReference() PredicateFn {
	return func(obj runtime.Object) bool {
		cr, ok := obj.(ClassReferencer)
		if !ok {
			return false
		}
		return cr.GetClassReference() == nil
	}
}

// NoManagedResourceReference accepts ResourceClaims that do not reference a specific Managed Resource
func NoManagedResourceReference() PredicateFn {
	return func(obj runtime.Object) bool {
		cr, ok := obj.(ManagedResourceReferencer)
		if !ok {
			return false
		}
		return cr.GetResourceReference() == nil
	}
}
