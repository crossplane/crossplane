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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
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
// resources and then establishing it.
type Establisher interface {
	Establish(ctx context.Context, objects []runtime.Object, parent resource.Object, control bool) ([]runtimev1alpha1.TypedReference, error)
}

// APIEstablisher establishes control or ownership of resources in the API
// server for a parent.
type APIEstablisher struct {
	client client.Client
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

// Establish checks that control or ownership of resources can be established by
// parent, then establishes it.
func (e *APIEstablisher) Establish(ctx context.Context, objs []runtime.Object, parent resource.Object, control bool) ([]runtimev1alpha1.TypedReference, error) { // nolint:gocyclo
	allObjs := []currentDesired{}
	resourceRefs := []runtimev1alpha1.TypedReference{}
	for _, res := range objs {
		// Assert desired object to resource.Object so that we can access its
		// metadata.
		d, ok := res.(resource.Object)
		if !ok {
			return nil, errors.New(errAssertObj)
		}

		// Make a copy of the desired object to be populated with existing
		// object, if it exists.
		current := res.DeepCopyObject()
		err := e.client.Get(ctx, types.NamespacedName{Name: d.GetName(), Namespace: d.GetNamespace()}, current)
		if resource.IgnoreNotFound(err) != nil {
			return nil, err
		}

		// If resource does not already exist, we must attempt to dry run create
		// it.
		if kerrors.IsNotFound(err) {
			// Add to objects as not existing.
			allObjs = append(allObjs, currentDesired{
				Desired: d,
				Current: nil,
				Exists:  false,
			})
			if err := e.create(ctx, d, parent, control, client.DryRunAll); err != nil {
				return nil, err
			}
			continue
		}

		c := current.(resource.Object)
		// Add to objects as existing.
		allObjs = append(allObjs, currentDesired{
			Desired: d,
			Current: c,
			Exists:  true,
		})

		if err := e.update(ctx, c, d, parent, control, client.DryRunAll); err != nil {
			return nil, err
		}
	}

	for _, cd := range allObjs {
		if !cd.Exists {
			if err := e.create(ctx, cd.Desired, parent, control); err != nil {
				return nil, err
			}
			resourceRefs = append(resourceRefs, *meta.TypedReferenceTo(cd.Desired, cd.Desired.GetObjectKind().GroupVersionKind()))
			continue
		}

		if err := e.update(ctx, cd.Current, cd.Desired, parent, control); err != nil {
			return nil, err
		}
		resourceRefs = append(resourceRefs, *meta.TypedReferenceTo(cd.Desired, cd.Desired.GetObjectKind().GroupVersionKind()))
	}

	return resourceRefs, nil
}

func (e *APIEstablisher) create(ctx context.Context, obj runtime.Object, parent resource.Object, control bool, opts ...client.CreateOption) error {
	o := obj.DeepCopyObject().(resource.Object)
	ref := meta.AsController(meta.TypedReferenceTo(parent, parent.GetObjectKind().GroupVersionKind()))
	if !control {
		ref = meta.AsOwner(meta.TypedReferenceTo(parent, parent.GetObjectKind().GroupVersionKind()))
	}
	// Overwrite any owner references on the desired object.
	o.SetOwnerReferences([]metav1.OwnerReference{ref})
	return e.client.Create(ctx, obj, opts...)
}

func (e *APIEstablisher) update(ctx context.Context, current, desired runtime.Object, parent resource.Object, control bool, opts ...client.UpdateOption) error {
	c := current.DeepCopyObject().(resource.Object)
	d := desired.DeepCopyObject().(resource.Object)
	if !control {
		ownerRef := meta.AsOwner(meta.TypedReferenceTo(parent, parent.GetObjectKind().GroupVersionKind()))
		for _, ref := range c.GetOwnerReferences() {
			if ref == ownerRef {
				return nil
			}
		}
		meta.AddOwnerReference(c, ownerRef)
		return e.client.Update(ctx, c, opts...)
	}

	// If desire is to control object, we attempt to update the object by
	// setting the desired owner references equal to that of the current, adding
	// a controller reference to the parent, and setting the desired resource
	// version to that of the current.
	d.SetOwnerReferences(c.GetOwnerReferences())
	if err := meta.AddControllerReference(d, meta.AsController(meta.TypedReferenceTo(parent, parent.GetObjectKind().GroupVersionKind()))); err != nil {
		return err
	}
	d.SetResourceVersion(c.GetResourceVersion())
	if !cmp.Equal(current, d, cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ManagedFields")) {
		return e.client.Update(ctx, desired, opts...)
	}
	return nil
}
