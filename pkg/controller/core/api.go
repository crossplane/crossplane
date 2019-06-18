/*
Copyright 2018 The Crossplane Authors.

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

package core

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

// An APIResourceCreator creates resources by submitting them to a Kubernetes
// API server.
type APIResourceCreator struct {
	client client.Client
	scheme *runtime.Scheme
}

// Create the supplied resource using the supplied class and claim.
func (a *APIResourceCreator) Create(ctx context.Context, cm Claim, cs *v1alpha1.ResourceClass, rs Resource) error {
	cmr := meta.ReferenceTo(cm, meta.MustGetKind(cm, a.scheme))
	csr := meta.ReferenceTo(cs, meta.MustGetKind(cs, a.scheme))
	rsr := meta.ReferenceTo(rs, meta.MustGetKind(rs, a.scheme))

	rs.SetClaimReference(cmr)
	rs.SetClassReference(csr)
	meta.AddOwnerReference(rs, meta.AsController(cmr))
	if err := a.client.Create(ctx, rs); err != nil {
		return errors.Wrap(err, "cannot create managed resource")
	}

	meta.AddFinalizer(cm, finalizerName)
	cm.SetResourceReference(rsr)

	return errors.Wrap(a.client.Update(ctx, cm), "cannot update resource claim")
}

// An APIResourceConnectionPropagator propagates connection details by reading
// them from and writing them to a Kubernetes API server.
type APIResourceConnectionPropagator struct {
	client client.Client
	scheme *runtime.Scheme
}

// PropagateConnection details from the supplied resource to the supplied claim.
func (a *APIResourceConnectionPropagator) PropagateConnection(ctx context.Context, cm Claim, rs Resource) error {
	// Either this resourace does not expose a connection secret, or this claim
	// does not want one.
	if rs.GetWriteConnectionSecretTo().Name == "" || cm.GetWriteConnectionSecretTo().Name == "" {
		return nil
	}

	n := types.NamespacedName{Namespace: rs.GetNamespace(), Name: rs.GetWriteConnectionSecretTo().Name}
	rscs := &corev1.Secret{}
	if err := a.client.Get(ctx, n, rscs); err != nil {
		return errors.Wrap(err, "cannot get managed resource connection secret")
	}

	cmcs := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Namespace:       cm.GetNamespace(),
		Name:            cm.GetWriteConnectionSecretTo().Name,
		OwnerReferences: []metav1.OwnerReference{meta.AsController(meta.ReferenceTo(cm, meta.MustGetKind(cm, a.scheme)))},
	}}
	err := util.CreateOrUpdate(ctx, a.client, cmcs, func() error {
		// Inside this anonymous function ccs could either be unchanged (if
		// it does not exist in the API server) or updated to reflect its
		// current state according to the API server.
		if c := metav1.GetControllerOf(cmcs); c != nil && c.UID != cm.GetUID() {
			return errors.New("resource claim connection secret is owned by another controller")
		}

		cmcs.Data = rscs.Data
		return nil
	})

	return errors.Wrap(err, "cannot create or update resource claim connection secret")
}

// APIResourceBinder binds resources to claims by updating them in a Kubernetes
// API server.
type APIResourceBinder struct {
	client client.Client
}

// Bind the supplied resource to the supplied claim.
func (a *APIResourceBinder) Bind(ctx context.Context, cm Claim, rs Resource) error {
	cm.SetBindingPhase(v1alpha1.BindingPhaseBound)
	rs.SetBindingPhase(v1alpha1.BindingPhaseBound)
	if err := a.client.Update(ctx, rs); err != nil {
		return errors.Wrap(err, "cannot update managed resource status")
	}

	return nil
}

// APISubresourceBinder binds resources to claims by updating their status
// subresource in a Kubernetes API server.
type APISubresourceBinder struct {
	client client.Client
}

// Bind the supplied resource to the supplied claim.
func (a *APISubresourceBinder) Bind(ctx context.Context, cm Claim, rs Resource) error {
	cm.SetBindingPhase(v1alpha1.BindingPhaseBound)
	rs.SetBindingPhase(v1alpha1.BindingPhaseBound)
	if err := a.client.Status().Update(ctx, rs); err != nil {
		return errors.Wrap(err, "cannot update managed resource status")
	}

	return nil
}

// APIResourceDeleter deletes resources from a Kubernetes API server.
type APIResourceDeleter struct {
	client client.Client
}

// Delete the supplied resource claim. If the claim was bound to a resource, the
// resource is updated to reflect that it is now unbound.
func (a *APIResourceDeleter) Delete(ctx context.Context, cm Claim, rs Resource) error {
	if meta.WasCreated(rs) && rs.GetBindingPhase() == v1alpha1.BindingPhaseBound {
		rs.SetBindingPhase(v1alpha1.BindingPhaseUnbound)
		if err := IgnoreNotFound(a.client.Update(ctx, rs)); err != nil {
			return errors.Wrap(err, "cannot update managed resource")
		}
	}

	meta.RemoveFinalizer(cm, finalizerName)
	return errors.Wrap(IgnoreNotFound(a.client.Update(ctx, cm)), "cannot update resource claim")
}

// APISubresourceDeleter deletes resource claims from a Kubernetes API server.
type APISubresourceDeleter struct {
	client client.Client
}

// Delete the supplied resource claim. If the claim was bound to a resource, the
// resource's status subresource is updated to reflect that it is now unbound.
func (a *APISubresourceDeleter) Delete(ctx context.Context, cm Claim, rs Resource) error {
	if meta.WasCreated(rs) && rs.GetBindingPhase() == v1alpha1.BindingPhaseBound {
		rs.SetBindingPhase(v1alpha1.BindingPhaseUnbound)
		if err := IgnoreNotFound(a.client.Status().Update(ctx, rs)); err != nil {
			return errors.Wrap(err, "cannot update managed resource")
		}
	}

	meta.RemoveFinalizer(cm, finalizerName)
	return errors.Wrap(IgnoreNotFound(a.client.Update(ctx, cm)), "cannot update resource claim")
}
