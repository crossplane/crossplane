/*
Copyright 2024 The Crossplane Authors.

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

package claim

import (
	"context"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

// Error strings.
const (
	errGetSecret            = "cannot get composite resource's connection secret"
	errSecretConflict       = "cannot establish control of existing connection secret"
	errCreateOrUpdateSecret = "cannot create or update connection secret"
)

// NopConnectionUnpublisher is a ConnectionUnpublisher that does nothing.
type NopConnectionUnpublisher struct{}

// NewNopConnectionUnpublisher returns a new NopConnectionUnpublisher.
func NewNopConnectionUnpublisher() *NopConnectionUnpublisher {
	return &NopConnectionUnpublisher{}
}

// UnpublishConnection does nothing and returns no error with
// UnpublishConnection. Expected to be used where deletion of connection
// secret is already handled by K8s garbage collection and there is actually
// nothing to do to unpublish connection details.
func (n *NopConnectionUnpublisher) UnpublishConnection(_ context.Context, _ resource.LocalConnectionSecretOwner, _ managed.ConnectionDetails) error {
	return nil
}

// SecretStoreConnectionUnpublisher unpublishes secret store connection secrets.
type SecretStoreConnectionUnpublisher struct {
	// TODO(turkenh): Use a narrower interface, i.e. we don't need Publish
	//  method here. Please note we cannot use ConnectionUnpublisher interface
	//  defined in this package as it expects a LocalConnectionSecretOwner
	//  which is exactly what this struct is providing. Ideally, we should
	//  split managed.ConnectionPublisher as Publisher and Unpublisher, but I
	//  would like to leave this to a further PR after we graduate Secret Store
	//  and decide to clean up the old API.
	publisher managed.ConnectionPublisher
}

// NewSecretStoreConnectionUnpublisher returns a new SecretStoreConnectionUnpublisher.
func NewSecretStoreConnectionUnpublisher(p managed.ConnectionPublisher) *SecretStoreConnectionUnpublisher {
	return &SecretStoreConnectionUnpublisher{
		publisher: p,
	}
}

// UnpublishConnection details for the supplied Managed resource.
func (u *SecretStoreConnectionUnpublisher) UnpublishConnection(ctx context.Context, so resource.LocalConnectionSecretOwner, c managed.ConnectionDetails) error {
	return u.publisher.UnpublishConnection(ctx, newClaimAsSecretOwner(so), c)
}

// soClaim is a type that enables using claim type with Secret Store
// UnpublishConnection method by satisfyng resource.ConnectionSecretOwner
// interface.
type soClaim struct {
	resource.LocalConnectionSecretOwner
}

func newClaimAsSecretOwner(lo resource.LocalConnectionSecretOwner) *soClaim {
	return &soClaim{
		LocalConnectionSecretOwner: lo,
	}
}

func (s soClaim) SetWriteConnectionSecretToReference(_ *xpv1.SecretReference) {}

func (s soClaim) GetWriteConnectionSecretToReference() *xpv1.SecretReference {
	// SecretStoreConnectionUnpublisher does not use
	// WriteConnectionSecretToReference interface, so, we are implementing
	// just to satisfy resource.ConnectionSecretOwner interface.
	return nil
}

// An APIConnectionPropagator propagates connection details by reading
// them from and writing them to a Kubernetes API server.
type APIConnectionPropagator struct {
	client resource.ClientApplicator
}

// NewAPIConnectionPropagator returns a new APIConnectionPropagator.
func NewAPIConnectionPropagator(c client.Client) *APIConnectionPropagator {
	return &APIConnectionPropagator{
		client: resource.ClientApplicator{Client: c, Applicator: resource.NewAPIUpdatingApplicator(c)},
	}
}

// PropagateConnection details from the supplied resource.
func (a *APIConnectionPropagator) PropagateConnection(ctx context.Context, to resource.LocalConnectionSecretOwner, from resource.ConnectionSecretOwner) (bool, error) {
	// Either from does not expose a connection secret, or to does not want one.
	if from.GetWriteConnectionSecretToReference() == nil || to.GetWriteConnectionSecretToReference() == nil {
		return false, nil
	}

	n := types.NamespacedName{
		Namespace: from.GetWriteConnectionSecretToReference().Namespace,
		Name:      from.GetWriteConnectionSecretToReference().Name,
	}
	fs := &corev1.Secret{}
	if err := a.client.Get(ctx, n, fs); err != nil {
		return false, errors.Wrap(err, errGetSecret)
	}

	// Make sure 'from' is the controller of the connection secret it references
	// before we propagate it. This ensures a resource cannot use Crossplane to
	// circumvent RBAC by propagating a secret it does not own.
	if c := metav1.GetControllerOf(fs); c == nil || c.UID != from.GetUID() {
		return false, errors.New(errSecretConflict)
	}

	ts := resource.LocalConnectionSecretFor(to, to.GetObjectKind().GroupVersionKind())
	ts.Data = fs.Data

	err := a.client.Apply(ctx, ts,
		resource.ConnectionSecretMustBeControllableBy(to.GetUID()),
		resource.AllowUpdateIf(func(current, desired runtime.Object) bool {
			// We consider the update to be a no-op and don't allow it if the
			// current and existing secret data are identical.

			//nolint:forcetypeassert // These will always be secrets.
			return !cmp.Equal(current.(*corev1.Secret).Data, desired.(*corev1.Secret).Data, cmpopts.EquateEmpty())
		}),
	)
	if resource.IsNotAllowed(err) {
		// The update was not allowed because it was a no-op.
		return false, nil
	}
	if err != nil {
		return false, errors.Wrap(err, errCreateOrUpdateSecret)
	}

	return true, nil
}
