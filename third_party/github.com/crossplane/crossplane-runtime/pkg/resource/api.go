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
	"encoding/json"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
)

// Error strings.
const (
	errUpdateObject = "cannot update object"
)

// An APIPatchingApplicator applies changes to an object by either creating or
// patching it in a Kubernetes API server.
type APIPatchingApplicator struct {
	client client.Client
}

// NewAPIPatchingApplicator returns an Applicator that applies changes to an
// object by either creating or patching it in a Kubernetes API server.
func NewAPIPatchingApplicator(c client.Client) *APIPatchingApplicator {
	return &APIPatchingApplicator{client: c}
}

// Apply changes to the supplied object. The object will be created if it does
// not exist, or patched if it does. If the object does exist, it will only be
// patched if the passed object has the same or an empty resource version.
func (a *APIPatchingApplicator) Apply(ctx context.Context, o client.Object, ao ...ApplyOption) error {
	m, ok := o.(metav1.Object)
	if !ok {
		return errors.New("cannot access object metadata")
	}

	if m.GetName() == "" && m.GetGenerateName() != "" {
		return errors.Wrap(a.client.Create(ctx, o), "cannot create object")
	}

	desired := o.DeepCopyObject()

	err := a.client.Get(ctx, types.NamespacedName{Name: m.GetName(), Namespace: m.GetNamespace()}, o)
	if kerrors.IsNotFound(err) {
		// TODO(negz): Apply ApplyOptions here too?
		return errors.Wrap(a.client.Create(ctx, o), "cannot create object")
	}
	if err != nil {
		return errors.Wrap(err, "cannot get object")
	}

	for _, fn := range ao {
		if err := fn(ctx, o, desired); err != nil {
			return err
		}
	}

	// TODO(negz): Allow callers to override the kind of patch used.
	return errors.Wrap(a.client.Patch(ctx, o, &patch{desired}), "cannot patch object")
}

type patch struct{ from runtime.Object }

func (p *patch) Type() types.PatchType                { return types.MergePatchType }
func (p *patch) Data(_ client.Object) ([]byte, error) { return json.Marshal(p.from) }

// An APIUpdatingApplicator applies changes to an object by either creating or
// updating it in a Kubernetes API server.
type APIUpdatingApplicator struct {
	client client.Client
}

// NewAPIUpdatingApplicator returns an Applicator that applies changes to an
// object by either creating or updating it in a Kubernetes API server.
func NewAPIUpdatingApplicator(c client.Client) *APIUpdatingApplicator {
	return &APIUpdatingApplicator{client: c}
}

// Apply changes to the supplied object. The object will be created if it does
// not exist, or updated if it does.
func (a *APIUpdatingApplicator) Apply(ctx context.Context, o client.Object, ao ...ApplyOption) error {
	m, ok := o.(Object)
	if !ok {
		return errors.New("cannot access object metadata")
	}

	if m.GetName() == "" && m.GetGenerateName() != "" {
		return errors.Wrap(a.client.Create(ctx, o), "cannot create object")
	}

	current := o.DeepCopyObject().(client.Object)

	err := a.client.Get(ctx, types.NamespacedName{Name: m.GetName(), Namespace: m.GetNamespace()}, current)
	if kerrors.IsNotFound(err) {
		// TODO(negz): Apply ApplyOptions here too?
		return errors.Wrap(a.client.Create(ctx, m), "cannot create object")
	}
	if err != nil {
		return errors.Wrap(err, "cannot get object")
	}

	for _, fn := range ao {
		if err := fn(ctx, current, m); err != nil {
			return err
		}
	}

	// NOTE(hasheddan): we must set the resource version of the desired object
	// to that of the current or the update will always fail.
	m.SetResourceVersion(current.(metav1.Object).GetResourceVersion())
	return errors.Wrap(a.client.Update(ctx, m), "cannot update object")
}

// An APIFinalizer adds and removes finalizers to and from a resource.
type APIFinalizer struct {
	client    client.Client
	finalizer string
}

// NewNopFinalizer returns a Finalizer that does nothing.
func NewNopFinalizer() Finalizer { return nopFinalizer{} }

type nopFinalizer struct{}

func (f nopFinalizer) AddFinalizer(_ context.Context, _ Object) error {
	return nil
}
func (f nopFinalizer) RemoveFinalizer(_ context.Context, _ Object) error {
	return nil
}

// NewAPIFinalizer returns a new APIFinalizer.
func NewAPIFinalizer(c client.Client, finalizer string) *APIFinalizer {
	return &APIFinalizer{client: c, finalizer: finalizer}
}

// AddFinalizer to the supplied Managed resource.
func (a *APIFinalizer) AddFinalizer(ctx context.Context, obj Object) error {
	if meta.FinalizerExists(obj, a.finalizer) {
		return nil
	}
	meta.AddFinalizer(obj, a.finalizer)
	return errors.Wrap(a.client.Update(ctx, obj), errUpdateObject)
}

// RemoveFinalizer from the supplied Managed resource.
func (a *APIFinalizer) RemoveFinalizer(ctx context.Context, obj Object) error {
	if !meta.FinalizerExists(obj, a.finalizer) {
		return nil
	}
	meta.RemoveFinalizer(obj, a.finalizer)
	return errors.Wrap(IgnoreNotFound(a.client.Update(ctx, obj)), errUpdateObject)
}

// A FinalizerFns satisfy the Finalizer interface.
type FinalizerFns struct {
	AddFinalizerFn    func(ctx context.Context, obj Object) error
	RemoveFinalizerFn func(ctx context.Context, obj Object) error
}

// AddFinalizer to the supplied resource.
func (f FinalizerFns) AddFinalizer(ctx context.Context, obj Object) error {
	return f.AddFinalizerFn(ctx, obj)
}

// RemoveFinalizer from the supplied resource.
func (f FinalizerFns) RemoveFinalizer(ctx context.Context, obj Object) error {
	return f.RemoveFinalizerFn(ctx, obj)
}
