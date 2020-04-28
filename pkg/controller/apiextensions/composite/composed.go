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
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
)

// Error strings
const (
	errUnmarshal = "cannot unmarshal base composed resource"
	errApply     = "cannot apply composed resource"
	errGetSecret = "cannot get connection secret of composed resource"

	errFmtPatch = "cannot apply the patch at index %d"
)

// Observation is the result of composed reconciliation.
type Observation struct {
	Ref               v1.ObjectReference
	ConnectionDetails managed.ConnectionDetails
	Ready             bool
}

// NewAPIComposer returns a new Composer that composes infrastructure resources
// in a Kubernetes API server.
func NewAPIComposer(c client.Client) *APIComposer {
	return &APIComposer{
		client: resource.ClientApplicator{
			Client:     c,
			Applicator: resource.NewAPIPatchingApplicator(c),
		},
	}
}

// An APIComposer composes infrastructure resources in a Kubernetes API server.
type APIComposer struct {
	client resource.ClientApplicator
}

// Compose the supplied Composed resource into the supplied Composite resource
// using the supplied CompositeTemplate.
func (r *APIComposer) Compose(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1alpha1.ComposedTemplate) (Observation, error) {
	// Any existing name will be overwritten when we unmarshal the template. We
	// store it here so that we can reset it after unmarshalling.
	name := cd.GetName()

	if err := json.Unmarshal(t.Base.Raw, cd); err != nil {
		return Observation{}, errors.Wrap(err, errUnmarshal)
	}

	// Umarshalling the template will overwrite any existing fields, so we must
	// restore the existing name, if any. We also set generate name in case we
	// haven't yet named this composed resource.
	cd.SetGenerateName(cp.GetName() + "-")
	cd.SetName(name)

	for i, p := range t.Patches {
		if err := p.Apply(cp, cd); err != nil {
			return Observation{}, errors.Wrapf(err, errFmtPatch, i)
		}
	}

	// NOTE(negz): We use AddOwnerReference rather than AddControllerReference
	// because we don't need the latter to check whether a controller reference
	// is already set.
	meta.AddOwnerReference(cd, meta.AsController(meta.ReferenceTo(cp, cp.GetObjectKind().GroupVersionKind())))
	if err := r.client.Apply(ctx, cd, resource.MustBeControllableBy(cp.GetUID())); err != nil {
		return Observation{}, errors.Wrap(err, errApply)
	}

	ref := *meta.ReferenceTo(cd, cd.GetObjectKind().GroupVersionKind())
	sref := cd.GetWriteConnectionSecretToReference()

	// The composed resource does not want to write a connection secret.
	if sref == nil {
		return Observation{Ref: ref}, nil
	}

	s := &corev1.Secret{}
	nn := types.NamespacedName{Namespace: sref.Namespace, Name: sref.Name}
	if err := r.client.Get(ctx, nn, s); err != nil {
		// The composed resource does want to write a connection secret but has
		// not yet. We presume this isn't an issue and that we'll propagate any
		// connection details during a future iteration.
		return Observation{Ref: ref}, errors.Wrap(resource.IgnoreNotFound(err), errGetSecret)
	}

	obs := Observation{
		Ref:               *meta.ReferenceTo(cd, cd.GetObjectKind().GroupVersionKind()),
		ConnectionDetails: managed.ConnectionDetails{},
	}

	for _, pair := range t.ConnectionDetails {
		key := pair.FromConnectionSecretKey
		if pair.Name != nil {
			key = *pair.Name
		}
		obs.ConnectionDetails[key] = s.Data[pair.FromConnectionSecretKey]
	}

	return obs, nil
}
