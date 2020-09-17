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

package revision

import (
	"context"

	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

const (
	errAssertObj = "cannot assert object to resource.Object"
)

// An Establisher establishes control or ownership of a set of resources in the
// API server by checking that control or ownership can be established for all
// resources and establishing it.
type Establisher interface {
	// Check ensures that all objects can owned or controlled. It returns an
	// error if any object cannot be owned or controlled.
	Check(ctx context.Context, objects []runtime.Object, parent resource.Object, control bool) error

	// Establish establishes ownership or control of all previously checked
	// resources.
	Establish(ctx context.Context, parent resource.Object, control bool) error

	// GetResourceRefs returns references to all objects that have had ownership
	// or control established.
	GetResourceRefs() []runtimev1alpha1.TypedReference

	// Reset clears all cached objects and references.
	Reset()
}

// APIEstablisher establishes control or ownership of resources in the API
// server for a parent.
type APIEstablisher struct {
	client       client.Client
	allObjs      []currentDesired
	resourceRefs []runtimev1alpha1.TypedReference
}

// NewAPIEstablisher creates a new APIEstablisher.
func NewAPIEstablisher(client client.Client) *APIEstablisher {
	return &APIEstablisher{
		client: client,
	}
}

// currentDesired caches resources while checking for control or ownership so
// that they do not have to be fetched from the API server again when control or
// ownership is established.
type currentDesired struct {
	Current resource.Object
	Desired resource.Object
	Exists  bool
}

// GetResourceRefs returns references to resources that have had control or
// ownership established.
func (e *APIEstablisher) GetResourceRefs() []runtimev1alpha1.TypedReference {
	return e.resourceRefs
}

// Reset clears all cached objects and references.
func (e *APIEstablisher) Reset() {
	e.allObjs, e.resourceRefs = nil, nil
}

// Check checks that control or ownership of resources can be established by
// parent. It seeds the object list used during establishment, so it should
// always be called before.
func (e *APIEstablisher) Check(ctx context.Context, objs []runtime.Object, parent resource.Object, control bool) error {
	for _, res := range objs {
		// Assert desired object to resource.Object so that we can access its
		// metadata.
		d, ok := res.(resource.Object)
		if !ok {
			return errors.New(errAssertObj)
		}

		// Make a copy of the desired object to be populated with existing
		// object, if it exists.
		current := res.DeepCopyObject()
		err := e.client.Get(ctx, types.NamespacedName{Name: d.GetName(), Namespace: d.GetNamespace()}, current)
		if resource.IgnoreNotFound(err) != nil {
			return err
		}

		// If resource does not already exist, we must attempt to dry run create
		// it.
		if kerrors.IsNotFound(err) {
			// Add to objects as not existing.
			e.allObjs = append(e.allObjs, currentDesired{
				Desired: d,
				Current: nil,
				Exists:  false,
			})
			if err := e.create(ctx, d, parent, control, client.DryRunAll); err != nil {
				return err
			}
			continue
		}

		c := current.(resource.Object)
		// Add to objects as existing.
		e.allObjs = append(e.allObjs, currentDesired{
			Desired: d,
			Current: c,
			Exists:  true,
		})

		if err := e.update(ctx, c, d, parent, control, client.DryRunAll); err != nil {
			return err
		}
	}

	return nil
}

// Establish establishes control or ownership of resources in the package that
// have been previously checked as ownable or controllable.
func (e *APIEstablisher) Establish(ctx context.Context, parent resource.Object, control bool) error {
	for _, cd := range e.allObjs {
		if !cd.Exists {
			if err := e.create(ctx, cd.Desired, parent, control); err != nil {
				return err
			}
			e.resourceRefs = append(e.resourceRefs, *meta.TypedReferenceTo(cd.Desired, cd.Desired.GetObjectKind().GroupVersionKind()))
			continue
		}

		if err := e.update(ctx, cd.Current, cd.Desired, parent, control); err != nil {
			return err
		}
		e.resourceRefs = append(e.resourceRefs, *meta.TypedReferenceTo(cd.Desired, cd.Desired.GetObjectKind().GroupVersionKind()))
	}
	return nil
}

func (e *APIEstablisher) create(ctx context.Context, obj resource.Object, parent resource.Object, control bool, opts ...client.CreateOption) error {
	ref := meta.AsController(meta.TypedReferenceTo(parent, parent.GetObjectKind().GroupVersionKind()))
	if !control {
		ref = meta.AsOwner(meta.TypedReferenceTo(parent, parent.GetObjectKind().GroupVersionKind()))
	}
	// Overwrite any owner references on the desired object.
	obj.SetOwnerReferences([]metav1.OwnerReference{ref})
	return e.client.Create(ctx, obj, opts...)
}

func (e *APIEstablisher) update(ctx context.Context, current, desired resource.Object, parent resource.Object, control bool, opts ...client.UpdateOption) error {
	if !control {
		meta.AddOwnerReference(current, meta.AsOwner(meta.TypedReferenceTo(parent, parent.GetObjectKind().GroupVersionKind())))
		return e.client.Update(ctx, current, opts...)
	}

	// If desire is to control object, we attempt to update the object by
	// setting the desired owner references equal to that of the current, adding
	// a controller reference to the parent, and setting the desired resource
	// version to that of the current.
	desired.SetOwnerReferences(current.GetOwnerReferences())
	if err := meta.AddControllerReference(desired, meta.AsController(meta.TypedReferenceTo(parent, parent.GetObjectKind().GroupVersionKind()))); err != nil {
		return err
	}
	desired.SetResourceVersion(current.GetResourceVersion())
	return e.client.Update(ctx, desired, opts...)
}
