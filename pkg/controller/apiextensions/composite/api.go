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

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
)

// Error strings.
const (
	errApplySecret = "cannot apply connection secret"
)

// APIFilteredSecretPublisher publishes ConnectionDetails content after filtering
// it through a set of permitted keys.
type APIFilteredSecretPublisher struct {
	client resource.Applicator
	filter []string
}

// NewAPIFilteredSecretPublisher returns a ConnectionPublisher that only
// publishes connection secret keys that are included in the supplied filter.
func NewAPIFilteredSecretPublisher(c client.Client, filter []string) ConnectionPublisher {
	return &APIFilteredSecretPublisher{client: resource.NewAPIPatchingApplicator(c), filter: filter}
}

// PublishConnection publishes the supplied ConnectionDetails to a Secret in the
// same namespace as the supplied Managed resource. It is a no-op if the secret
// already exists with the supplied ConnectionDetails.
func (a *APIFilteredSecretPublisher) PublishConnection(ctx context.Context, o resource.ConnectionSecretOwner, c managed.ConnectionDetails) error {
	// This resource does not want to expose a connection secret.
	if o.GetWriteConnectionSecretToReference() == nil {
		return nil
	}

	s := resource.ConnectionSecretFor(o, o.GetObjectKind().GroupVersionKind())
	m := map[string]bool{}
	// TODO(muvaf): Should empty filter allow all keys?
	for _, key := range a.filter {
		m[key] = true
	}
	for key, val := range c {
		if _, ok := m[key]; ok {
			s.Data[key] = val
		}
	}

	return errors.Wrap(a.client.Apply(ctx, s, resource.ConnectionSecretMustBeControllableBy(o.GetUID())), errApplySecret)
}

// UnpublishConnection is no-op since PublishConnection only creates resources
// that will be garbage collected by Kubernetes when the managed resource is
// deleted.
func (a *APIFilteredSecretPublisher) UnpublishConnection(_ context.Context, _ resource.ConnectionSecretOwner, _ managed.ConnectionDetails) error {
	return nil
}

// NewSelectorResolver returns a SelectorResolver for composite resource.
func NewSelectorResolver(c client.Client) Resolver {
	return &SelectorResolver{client: c}
}

// SelectorResolver is used to resolve the composition selector on the instance
// to composition reference.
type SelectorResolver struct {
	client client.Client
}

// ResolveSelector resolves selector to a reference if it doesn't exist.
func (r *SelectorResolver) ResolveSelector(ctx context.Context, cr resource.Composite) error {
	// TODO(muvaf): need to block the deletion of composition via finalizer once
	// it's selected since it's integral to this resource.
	// TODO(muvaf): We don't rely on UID in practice. It should not be there
	// because it will make confusion if the resource is backed up and restored
	// to another cluster
	if cr.GetCompositionReference() != nil {
		return nil
	}
	labels := map[string]string{}
	sel := cr.GetCompositionSelector()
	if sel != nil {
		labels = sel.MatchLabels
	}
	list := &v1alpha1.CompositionList{}
	if err := r.client.List(ctx, list, client.MatchingLabels(labels)); err != nil {
		return errors.Wrap(err, "cannot list compositions")
	}
	apiVersion, kind := cr.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
	for _, comp := range list.Items {
		if comp.Spec.From.APIVersion != apiVersion || comp.Spec.From.Kind != kind {
			continue
		}
		cr.SetCompositionReference(meta.ReferenceTo(comp.DeepCopy(), v1alpha1.CompositionGroupVersionKind))
		cr.SetReclaimPolicy(comp.Spec.ReclaimPolicy)
		cr.SetWriteConnectionSecretToReference(&runtimev1alpha1.SecretReference{
			Name:      string(cr.GetUID()),
			Namespace: comp.Spec.WriteConnectionSecretsToNamespace,
		})
		return errors.Wrap(r.client.Update(ctx, cr), "cannot update composite resource")
	}
	return errors.New("no compatible composition has been found that has the given labels")
}
