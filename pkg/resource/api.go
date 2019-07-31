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
	errSecretConflict       = "cannot establish control of existing connection secret"
	errCreateOrUpdateSecret = "cannot create or update connection secret"
	errUpdateManaged        = "cannot update managed resource"
	errUpdateManagedStatus  = "cannot update managed resource status"
)

const claimFinalizerName = "finalizer." + claimControllerName

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

	meta.AddFinalizer(cm, claimFinalizerName)
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
		if c := metav1.GetControllerOf(cmcs); c == nil || c.UID != cm.GetUID() {
			return errors.New(errSecretConflict)
		}
		cmcs.Data = mgcs.Data
		return nil
	})

	return errors.Wrap(err, errCreateOrUpdateSecret)
}

// An APIManagedBinder binds resources to claims by updating them in a
// Kubernetes API server. Note that APIManagedBinder does not support objects
// using the status subresource; such objects should use APIManagedStatusBinder.
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

// An APIManagedStatusBinder binds resources to claims by updating them in a
// Kubernetes API server. Note that APIManagedStatusBinder does not support
// objects that do not use the status subresource; such objects should use
// APIManagedBinder.
type APIManagedStatusBinder struct {
	client client.Client
}

// NewAPIManagedStatusBinder returns a new APIManagedStatusBinder.
func NewAPIManagedStatusBinder(c client.Client) *APIManagedStatusBinder {
	return &APIManagedStatusBinder{client: c}
}

// Bind the supplied resource to the supplied claim.
func (a *APIManagedStatusBinder) Bind(ctx context.Context, cm Claim, mg Managed) error {
	cm.SetBindingPhase(v1alpha1.BindingPhaseBound)
	mg.SetBindingPhase(v1alpha1.BindingPhaseBound)
	if err := a.client.Status().Update(ctx, mg); err != nil {
		return errors.Wrap(err, errUpdateManagedStatus)
	}
	return nil
}

// An APIManagedUnbinder finalizes the deletion of a managed resource by
// unbinding it, then updating it in the API server.
type APIManagedUnbinder struct {
	client client.Client
}

// NewAPIManagedUnbinder returns a new APIManagedUnbinder.
func NewAPIManagedUnbinder(c client.Client) *APIManagedUnbinder {
	return &APIManagedUnbinder{client: c}
}

// Finalize the supplied managed rersource.
func (a *APIManagedUnbinder) Finalize(ctx context.Context, mg Managed) error {
	// TODO(negz): We probably want to delete the managed resource here if its
	// reclaim policy is delete, rather than relying on garbage collection, per
	// https://github.com/crossplaneio/crossplane/issues/550
	mg.SetBindingPhase(v1alpha1.BindingPhaseUnbound)
	mg.SetClaimReference(nil)
	return errors.Wrap(IgnoreNotFound(a.client.Update(ctx, mg)), errUpdateManaged)
}

// An APIManagedStatusUnbinder finalizes the deletion of a managed resource by
// unbinding it, then updating it and its status in the API server.
type APIManagedStatusUnbinder struct {
	client client.Client
}

// NewAPIManagedStatusUnbinder returns a new APIStatusManagedFinalizer.
func NewAPIManagedStatusUnbinder(c client.Client) *APIManagedStatusUnbinder {
	return &APIManagedStatusUnbinder{client: c}
}

// Finalize the supplied resource claim.
func (a *APIManagedStatusUnbinder) Finalize(ctx context.Context, mg Managed) error {
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

// An APIClaimFinalizerRemover finalizes the deletion of a resource claim by
// removing its finalizer and updating it in the API server.
type APIClaimFinalizerRemover struct {
	client client.Client
}

// NewAPIClaimFinalizerRemover returns a new APIClaimFinalizerRemover.
func NewAPIClaimFinalizerRemover(c client.Client) *APIClaimFinalizerRemover {
	return &APIClaimFinalizerRemover{client: c}
}

// Finalize the supplied resource claim.
func (a *APIClaimFinalizerRemover) Finalize(ctx context.Context, cm Claim) error {
	meta.RemoveFinalizer(cm, claimFinalizerName)
	return errors.Wrap(IgnoreNotFound(a.client.Update(ctx, cm)), errUpdateClaim)
}

// An APIManagedFinalizerRemover finalizes the deletion of a Managed resource by
// removing its finalizer and updating it in the API server.
type APIManagedFinalizerRemover struct{ client client.Client }

// NewAPIManagedFinalizerRemover returns a new APIManagedFinalizerRemover.
func NewAPIManagedFinalizerRemover(c client.Client) *APIManagedFinalizerRemover {
	return &APIManagedFinalizerRemover{client: c}
}

// Finalize the deletion of the supplied Managed resource.
func (a *APIManagedFinalizerRemover) Finalize(ctx context.Context, mg Managed) error {
	meta.RemoveFinalizer(mg, managedFinalizerName)
	return errors.Wrap(a.client.Update(ctx, mg), errUpdateManaged)
}

// An APIManagedFinalizerAdder establishes ownership of a managed resource by
// adding a finalizer and updating it in the API server.
type APIManagedFinalizerAdder struct{ client client.Client }

// NewAPIManagedFinalizerAdder returns a new APIManagedFinalizerAdder.
func NewAPIManagedFinalizerAdder(c client.Client) *APIManagedFinalizerAdder {
	return &APIManagedFinalizerAdder{client: c}
}

// Establish ownership of the supplied Managed resource.
func (a *APIManagedFinalizerAdder) Establish(ctx context.Context, mg Managed) error {
	meta.AddFinalizer(mg, managedFinalizerName)
	return errors.Wrap(a.client.Update(ctx, mg), errUpdateManaged)
}
