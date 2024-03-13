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

// Package predicate implements some useful event filters
package predicate

import (
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// IgnoreStatusChanges does not trigger reconciliation
// when object's status has been changed
func IgnoreStatusChanges() predicate.Predicate {
	return predicate.Or(
		predicate.LabelChangedPredicate{},
		predicate.AnnotationChangedPredicate{},
		predicate.GenerationChangedPredicate{},
	)
}

// IgnoreMetadataChanges does not trigger reconciliation
// when object's metadata has been changed
func IgnoreMetadataChanges() predicate.Predicate {
	return predicate.Or(
		predicate.GenerationChangedPredicate{},
		StatusChangedPredicate{},
	)
}

// StatusChangedPredicate triggers reconciliation
// when object's status has been changed
type StatusChangedPredicate struct {
	predicate.Funcs
}

// Update implements default UpdateEvent filter for validating status change.
func (p StatusChangedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil {
		return false
	}
	if e.ObjectNew == nil {
		return false
	}

	uOld, err := runtime.DefaultUnstructuredConverter.ToUnstructured(e.ObjectOld)
	if err != nil {
		return false
	}
	uNew, err := runtime.DefaultUnstructuredConverter.ToUnstructured(e.ObjectNew)
	if err != nil {
		return false
	}
	return !reflect.DeepEqual(uNew["status"], uOld["status"])
}
