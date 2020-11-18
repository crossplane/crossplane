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

package claim

import (
	"context"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

// Error strings.
const (
	errCreateComposite      = "cannot create composite resource"
	errUpdateClaim          = "cannot update composite resource claim"
	errUpdateComposite      = "cannot update composite resource"
	errDeleteComposite      = "cannot delete composite resource"
	errBindConflict         = "cannot bind composite resource that references a different claim"
	errGetSecret            = "cannot get composite resource's connection secret"
	errSecretConflict       = "cannot establish control of existing connection secret"
	errCreateOrUpdateSecret = "cannot create or update connection secret"
)

// An APICompositeCreator creates resources by submitting them to a Kubernetes
// API server.
type APICompositeCreator struct {
	client client.Client
	typer  runtime.ObjectTyper
}

// NewAPICompositeCreator returns a new APICompositeCreator.
func NewAPICompositeCreator(c client.Client, t runtime.ObjectTyper) *APICompositeCreator {
	return &APICompositeCreator{client: c, typer: t}
}

// TODO(negz): We should render and patch a composite resource on each
// reconcile, rather than just creating it once.

// Create the supplied composite using the supplied claim.
func (a *APICompositeCreator) Create(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error {

	cp.SetClaimReference(meta.ReferenceTo(cm, resource.MustGetKind(cm, a.typer)))
	if err := a.client.Create(ctx, cp); err != nil {
		return errors.Wrap(err, errCreateComposite)
	}
	// Since we use GenerateName feature of ObjectMeta, final name of the
	// resource is calculated during the creation of the resource. So, we
	// can generate a complete reference only after the creation.
	cpr := meta.ReferenceTo(cp, resource.MustGetKind(cp, a.typer))
	cm.SetResourceReference(cpr)

	return errors.Wrap(a.client.Update(ctx, cm), errUpdateClaim)
}

// An APICompositeDeleter deletes composite resources from the API server.
type APICompositeDeleter struct {
	client client.Client
}

// NewAPICompositeDeleter returns a new APICompositeDeleter.
func NewAPICompositeDeleter(c client.Client) *APICompositeDeleter {
	return &APICompositeDeleter{client: c}
}

// Delete the supplied composite resource from the API server.
func (a *APICompositeDeleter) Delete(ctx context.Context, _ resource.CompositeClaim, cp resource.Composite) error {
	return errors.Wrap(resource.IgnoreNotFound(a.client.Delete(ctx, cp)), errDeleteComposite)
}

// An APIBinder binds claims to composites by updating them in a Kubernetes API
// server. Note that APIBinder does not support objects that do not use the
// status subresource; such objects should use APIBinder.
type APIBinder struct {
	client client.Client
	typer  runtime.ObjectTyper
}

// NewAPIBinder returns a new APIBinder.
func NewAPIBinder(c client.Client, t runtime.ObjectTyper) *APIBinder {
	return &APIBinder{client: c, typer: t}
}

// Bind the supplied claim to the supplied composite.
func (a *APIBinder) Bind(ctx context.Context, cm resource.CompositeClaim, cp resource.Composite) error {
	existing := cp.GetClaimReference()
	proposed := meta.ReferenceTo(cm, resource.MustGetKind(cm, a.typer))

	if existing != nil && (existing.Namespace != proposed.Namespace || existing.Name != proposed.Name) {
		return errors.New(errBindConflict)
	}

	cp.SetClaimReference(proposed)
	if err := a.client.Update(ctx, cp); err != nil {
		return errors.Wrap(err, errUpdateComposite)
	}

	if meta.GetExternalName(cp) == "" {
		return nil
	}

	// Propagate back the final name of the composite resource to the claim.
	meta.SetExternalName(cm, meta.GetExternalName(cp))
	return errors.Wrap(a.client.Update(ctx, cm), errUpdateClaim)
}

// An APIConnectionPropagator propagates connection details by reading
// them from and writing them to a Kubernetes API server.
type APIConnectionPropagator struct {
	client resource.ClientApplicator
	typer  runtime.ObjectTyper
}

// NewAPIConnectionPropagator returns a new APIConnectionPropagator.
func NewAPIConnectionPropagator(c client.Client, t runtime.ObjectTyper) *APIConnectionPropagator {
	return &APIConnectionPropagator{
		client: resource.ClientApplicator{Client: c, Applicator: resource.NewAPIUpdatingApplicator(c)},
		typer:  t,
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

	ts := resource.LocalConnectionSecretFor(to, resource.MustGetKind(to, a.typer))
	ts.Data = fs.Data

	err := a.client.Apply(ctx, ts,
		resource.ConnectionSecretMustBeControllableBy(to.GetUID()),
		resource.AllowUpdateIf(func(current, desired runtime.Object) bool {
			// We consider the update to be a no-op and don't allow it if the
			// current and existing secret data are identical.
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
