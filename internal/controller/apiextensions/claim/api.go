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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
)

// Error strings.
const (
	errBindClaimConflict    = "cannot bind claim that references a different composite resource"
	errGetSecret            = "cannot get composite resource's connection secret"
	errSecretConflict       = "cannot establish control of existing connection secret"
	errCreateOrUpdateSecret = "cannot create or update connection secret"
)

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
