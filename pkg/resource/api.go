/*
Copyright 2018 The Crossplane Authomg.

Licensed under the Apache License, Vemgion 2.0 (the "License");
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

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplaneio/crossplane/pkg/apis/core/v1alpha1"
	"github.com/crossplaneio/crossplane/pkg/meta"
	"github.com/crossplaneio/crossplane/pkg/util"
)

// Error strings.
const (
	errCreateManaged        = "cannot create managed resource"
	errUpdateClaim          = "cannot update resource claim"
	errGetSecret            = "cannot get managed resource's connection secret"
	errSecretConflict       = "resource claim connection secret is controlled by another object"
	errCreateOrUpdateSecret = "cannot create or update resource claim connection secret"
	errUpdateManaged        = "cannot update managed resource"
	errUpdateManagedStatus  = "cannot update managed resource status"
)

// An APIManagedCreator creates resources by submitting them to a Kubernetes
// API server.
type APIManagedCreator struct {
	client client.Client
	typer  runtime.ObjectTyper
}

// NewAPIManagedCreator returns a new APIManagedCreator.
func NewAPIManagedCreator(c client.Client, t runtime.ObjectTyper) *APIManagedCreator {
	return &APIManagedCreator{client: c, typer: t}
}

// Create the supplied resource using the supplied class and claim.
func (a *APIManagedCreator) Create(ctx context.Context, cm Claim, cs *v1alpha1.ResourceClass, mg Managed) error {
	cmr := meta.ReferenceTo(cm, MustGetKind(cm, a.typer))
	csr := meta.ReferenceTo(cs, MustGetKind(cs, a.typer))
	mgr := meta.ReferenceTo(mg, MustGetKind(mg, a.typer))

	mg.SetClaimReference(cmr)
	mg.SetClassReference(csr)
	if err := a.client.Create(ctx, mg); err != nil {
		return errors.Wrap(err, errCreateManaged)
	}

	meta.AddFinalizer(cm, finalizerName)
	cm.SetResourceReference(mgr)

	return errors.Wrap(a.client.Update(ctx, cm), errUpdateClaim)
}

// An APIManagedConnectionPropagator propagates connection details by reading
// them from and writing them to a Kubernetes API server.
type APIManagedConnectionPropagator struct {
	client client.Client
	typer  runtime.ObjectTyper
}

// NewAPIManagedConnectionPropagator returns a new APIManagedConnectionPropagator.
func NewAPIManagedConnectionPropagator(c client.Client, t runtime.ObjectTyper) *APIManagedConnectionPropagator {
	return &APIManagedConnectionPropagator{client: c, typer: t}
}

// PropagateConnection details from the supplied resource to the supplied claim.
func (a *APIManagedConnectionPropagator) PropagateConnection(ctx context.Context, cm Claim, mg Managed) error {
	// Either this resourace does not expose a connection secret, or this claim
	// does not want one.
	if mg.GetWriteConnectionSecretToReference().Name == "" || cm.GetWriteConnectionSecretToReference().Name == "" {
		return nil
	}

	n := types.NamespacedName{Namespace: mg.GetNamespace(), Name: mg.GetWriteConnectionSecretToReference().Name}
	mgcs := &corev1.Secret{}
	if err := a.client.Get(ctx, n, mgcs); err != nil {
		return errors.Wrap(err, errGetSecret)
	}

	cmcs := ConnectionSecretFor(cm, MustGetKind(cm, a.typer))
	err := util.CreateOrUpdate(ctx, a.client, cmcs, func() error {
		// Inside this anonymous function cmcs could either be unchanged (if
		// it does not exist in the API server) or updated to reflect its
		// current state according to the API server.
		if c := metav1.GetControllerOf(cmcs); c != nil && c.UID != cm.GetUID() {
			return errors.New(errSecretConflict)
		}
		cmcs.Data = mgcs.Data
		return nil
	})

	return errors.Wrap(err, errCreateOrUpdateSecret)
}

// An APIManagedBinder binds resources to claims by updating them in a
// Kubernetes API server. Note that APIManagedBinder does not support objects
// using the status subresource; such objects should use APIStatusManagedBinder.
type APIManagedBinder struct {
	client client.Client
}

// NewAPIManagedBinder returns a new APIManagedBinder.
func NewAPIManagedBinder(c client.Client) *APIManagedBinder {
	return &APIManagedBinder{client: c}
}

// Bind the supplied resource to the supplied claim.
func (a *APIManagedBinder) Bind(ctx context.Context, cm Claim, mg Managed) error {
	cm.SetBindingPhase(v1alpha1.BindingPhaseBound)
	mg.SetBindingPhase(v1alpha1.BindingPhaseBound)
	if err := a.client.Update(ctx, mg); err != nil {
		return errors.Wrap(err, errUpdateManaged)
	}
	return nil
}

// An APIStatusManagedBinder binds resources to claims by updating them in a
// Kubernetes API server. Note that APIManagedBinder does not support objects
// that do not use the status subresource; such objects should use
// APIManagedBinder.
type APIStatusManagedBinder struct {
	client client.Client
}

// NewAPIStatusManagedBinder returns a new APIStatusManagedBinder.
func NewAPIStatusManagedBinder(c client.Client) *APIStatusManagedBinder {
	return &APIStatusManagedBinder{client: c}
}

// Bind the supplied resource to the supplied claim.
func (a *APIStatusManagedBinder) Bind(ctx context.Context, cm Claim, mg Managed) error {
	cm.SetBindingPhase(v1alpha1.BindingPhaseBound)
	mg.SetBindingPhase(v1alpha1.BindingPhaseBound)
	if err := a.client.Status().Update(ctx, mg); err != nil {
		return errors.Wrap(err, errUpdateManagedStatus)
	}
	return nil
}

// An APIManagedFinalizer finalizes the deletion of a managed resource by either
// deleting or unbinding it, then updating it in the API server.
type APIManagedFinalizer struct {
	client client.Client
}

// NewAPIManagedFinalizer returns a new APIManagedFinalizer.
func NewAPIManagedFinalizer(c client.Client) *APIManagedFinalizer {
	return &APIManagedFinalizer{client: c}
}

// Finalize the supplied resource claim.
func (a *APIManagedFinalizer) Finalize(ctx context.Context, mg Managed) error {
	// TODO(negz): We probably want to delete the managed resource here if its
	// reclaim policy is delete, rather than relying on garbage collection, per
	// https://github.com/crossplaneio/crossplane/issues/550
	mg.SetBindingPhase(v1alpha1.BindingPhaseUnbound)
	mg.SetClaimReference(nil)
	return errors.Wrap(IgnoreNotFound(a.client.Update(ctx, mg)), errUpdateManaged)
}

// An APIStatusManagedFinalizer finalizes the deletion of a managed resource by
// either deleting or unbinding it, then updating it and its status in the API
// server.
type APIStatusManagedFinalizer struct {
	client client.Client
}

// NewAPIStatusManagedFinalizer returns a new APIStatusManagedFinalizer.
func NewAPIStatusManagedFinalizer(c client.Client) *APIStatusManagedFinalizer {
	return &APIStatusManagedFinalizer{client: c}
}

// Finalize the supplied resource claim.
func (a *APIStatusManagedFinalizer) Finalize(ctx context.Context, mg Managed) error {
	// TODO(negz): We probably want to delete the managed resource here if its
	// reclaim policy is delete, rather than relying on garbage collection, per
	// https://github.com/crossplaneio/crossplane/issues/550
	mg.SetBindingPhase(v1alpha1.BindingPhaseUnbound)
	mg.SetClaimReference(nil)

	if err := a.client.Update(ctx, mg); err != nil {
		return errors.Wrap(IgnoreNotFound(err), errUpdateManaged)
	}

	return errors.Wrap(IgnoreNotFound(a.client.Status().Update(ctx, mg)), errUpdateManagedStatus)
}

// An APIClaimFinalizer finalizes the deletion of a resource claim by removing
// its finalizer and updating it in the API server.
type APIClaimFinalizer struct {
	client client.Client
}

// NewAPIClaimFinalizer returns a new APIClaimFinalizer.
func NewAPIClaimFinalizer(c client.Client) *APIClaimFinalizer {
	return &APIClaimFinalizer{client: c}
}

// Finalize the supplied resource claim.
func (a *APIClaimFinalizer) Finalize(ctx context.Context, cm Claim) error {
	meta.RemoveFinalizer(cm, finalizerName)
	return errors.Wrap(IgnoreNotFound(a.client.Update(ctx, cm)), errUpdateClaim)
}
