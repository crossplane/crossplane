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
	"context"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/meta"
)

const predicateTimeout = 1 * time.Minute

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

// ObjectHasProvisioner returns a PredicateFn implemented by HasProvisioner.
func ObjectHasProvisioner(c client.Client, provisioner string) PredicateFn {
	return func(obj runtime.Object) bool {
		cr, ok := obj.(ClassReferencer)
		if !ok {
			return false
		}
		return HasProvisioner(c, cr, provisioner)
	}
}

// HasProvisioner looks up the supplied ClassReferencer's resource class using
// the supplied Client, returning true if the resource class uses the supplied
// provisioner.
func HasProvisioner(c client.Client, cr ClassReferencer, provisioner string) bool {
	if cr.GetClassReference() == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), predicateTimeout)
	defer cancel()

	cs := &v1alpha1.ResourceClass{}
	if err := c.Get(ctx, meta.NamespacedNameOf(cr.GetClassReference()), cs); err != nil {
		return false
	}

	return strings.EqualFold(cs.Provisioner, provisioner)
}

// NOTE(hasheddan): HasClassReferenceKind should eventually replace ObjectHasProvisioner
// when strongly typed resource classes are implemented

// HasClassReferenceKind accepts ResourceClaims that reference the correct kind of resourceclass
func HasClassReferenceKind(k ClassKind) PredicateFn {
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

		return gvk == schema.GroupVersionKind(k)
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
