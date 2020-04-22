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

package composite

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
)

// Observation is the result of composed reconciliation.
type Observation struct {
	Ref               v1.ObjectReference
	ConnectionDetails managed.ConnectionDetails
}

// NewAPIComposedReconciler returns a new *APIComposedReconciler.
func NewAPIComposedReconciler(c client.Client) *APIComposedReconciler {
	return &APIComposedReconciler{
		client: resource.ClientApplicator{
			Client:     c,
			Applicator: resource.NewAPIPatchingApplicator(c),
		},
	}
}

// APIComposedReconciler is able to reconcile a composed resource.
type APIComposedReconciler struct {
	client resource.ClientApplicator
}

// Reconcile tries to bring the composed resource into the desired state. It
// creates the resource if the given reference is empty.
func (r *APIComposedReconciler) Reconcile(ctx context.Context, cr resource.Composite, composedRef v1.ObjectReference, tmpl v1alpha1.ComposedTemplate) (Observation, error) {
	// Deletion of the composite resource has been triggered. We make the deletion
	// call and report back success only if the call returns NotFound.
	if meta.WasDeleted(cr) {
		if composedRef.Name == "" {
			return Observation{}, nil
		}
		err := r.client.Delete(ctx, unstructured.NewComposed(unstructured.FromReference(composedRef)))
		return Observation{}, resource.IgnoreNotFound(err)
	}

	var composed resource.Composable
	if composedRef.Name == "" {
		composed = unstructured.NewComposed()
		if err := r.Configure(cr, composed, tmpl); err != nil {
			return Observation{}, err
		}
	} else {
		composed = unstructured.NewComposed(unstructured.FromReference(composedRef))
		if err := r.client.Get(ctx, types.NamespacedName{Name: composed.GetName(), Namespace: composed.GetNamespace()}, composed); err != nil {
			return Observation{}, err
		}
	}

	// Patches are continuously applied from the Composite resource to the composed.
	if err := r.Overlay(cr, composed, tmpl.Patches); err != nil {
		return Observation{}, err
	}

	obs := Observation{}
	if err := r.client.Apply(ctx, composed, resource.MustBeControllableBy(cr.GetUID())); err != nil {
		return Observation{}, err
	}
	obs.Ref = *meta.ReferenceTo(composed, composed.GetObjectKind().GroupVersionKind())

	conn, err := r.GetConnectionDetails(ctx, composed, tmpl.ConnectionDetails)
	if err != nil {
		return Observation{}, err
	}
	obs.ConnectionDetails = conn

	return obs, nil
}

// Configure the composed object with given template and composite metadata.
func (r *APIComposedReconciler) Configure(composite, composed resource.Object, tmpl v1alpha1.ComposedTemplate) error {
	if err := json.Unmarshal(tmpl.Base.Raw, composed); err != nil {
		return err
	}
	composed.SetGenerateName(fmt.Sprintf("%s-", composite.GetName()))
	return nil
}

// Overlay applies an overlay to the resource with the information from parent
// composite resource.
func (r *APIComposedReconciler) Overlay(composite, composed resource.Object, patches []v1alpha1.Patch) error {
	for i, patch := range patches {
		if err := patch.Apply(composite, composed); err != nil {
			return errors.Wrap(err, fmt.Sprintf("cannot apply the patch at index %d on result", i))
		}
	}
	err := meta.AddControllerReference(composed, meta.AsController(meta.ReferenceTo(composite, composite.GetObjectKind().GroupVersionKind())))
	return errors.Wrap(err, "cannot add controller ref to composed resource")
}

// GetConnectionDetails returns the ConnectionDetails of the resource if a reference
// to the connection secret exists.
func (r *APIComposedReconciler) GetConnectionDetails(ctx context.Context, composed resource.ConnectionSecretWriterTo, conversion []v1alpha1.ConnectionDetail) (managed.ConnectionDetails, error) {
	secretRef := composed.GetWriteConnectionSecretToReference()
	if secretRef == nil {
		return nil, nil
	}
	secret := &v1.Secret{}
	err := r.client.Get(ctx, types.NamespacedName{Namespace: secretRef.Namespace, Name: secretRef.Name}, secret)
	if err != nil {
		return nil, resource.IgnoreNotFound(err)
	}
	out := managed.ConnectionDetails{}
	for _, pair := range conversion {
		key := pair.FromConnectionSecretKey
		if pair.Name != nil {
			key = *pair.Name
		}
		out[key] = secret.Data[pair.FromConnectionSecretKey]
	}
	return out, nil
}
