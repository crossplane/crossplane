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

package composed

import (
	"context"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
)

const (
	errUnmarshal = "cannot unmarshal base template"
	errFmtPatch  = "cannot apply the patch at index %d"
	errGetSecret = "cannot get connection secret of composed resource"
)

// ConfigureFn is a function that implements Configurator interface.
type ConfigureFn func(cp resource.Composite, cd resource.Composed, t v1alpha1.ComposedTemplate) error

// Configure calls ConfigureFn.
func (c ConfigureFn) Configure(cp resource.Composite, cd resource.Composed, t v1alpha1.ComposedTemplate) error {
	return c(cp, cd, t)
}

// DefaultConfigurator configures the composed resource with given raw template
// and metadata information from composite resource.
type DefaultConfigurator struct{}

// Configure applies the raw template and sets name and generateName.
func (*DefaultConfigurator) Configure(cp resource.Composite, cd resource.Composed, t v1alpha1.ComposedTemplate) error {
	// Any existing name will be overwritten when we unmarshal the template. We
	// store it here so that we can reset it after unmarshalling.
	name := cd.GetName()
	if err := json.Unmarshal(t.Base.Raw, cd); err != nil {
		return errors.Wrap(err, errUnmarshal)
	}
	// Unmarshalling the template will overwrite any existing fields, so we must
	// restore the existing name, if any. We also set generate name in case we
	// haven't yet named this composed resource.
	cd.SetGenerateName(cp.GetName() + "-")
	cd.SetName(name)
	return nil
}

// OverlayFn is a function that implements OverlayApplicator interface.
type OverlayFn func(cp resource.Composite, cd resource.Composed, t v1alpha1.ComposedTemplate) error

// Overlay calls OverlayFn.
func (o OverlayFn) Overlay(cp resource.Composite, cd resource.Composed, t v1alpha1.ComposedTemplate) error {
	return o(cp, cd, t)
}

// DefaultOverlayApplicator applies patches to the composed resource using the
// values on Composite resource and field bindings in ComposedTemplate.
type DefaultOverlayApplicator struct{}

// Overlay applies patches to composed resource.
func (*DefaultOverlayApplicator) Overlay(cp resource.Composite, cd resource.Composed, t v1alpha1.ComposedTemplate) error {
	for i, p := range t.Patches {
		if err := p.Apply(cp, cd); err != nil {
			return errors.Wrapf(err, errFmtPatch, i)
		}
	}
	return nil
}

// FetchFn is a function that implements the ConnectionDetailsFetcher interface.
type FetchFn func(ctx context.Context, cd resource.Composed, t v1alpha1.ComposedTemplate) (managed.ConnectionDetails, error)

// Fetch calls FetchFn.
func (f FetchFn) Fetch(ctx context.Context, cd resource.Composed, t v1alpha1.ComposedTemplate) (managed.ConnectionDetails, error) {
	return f(ctx, cd, t)
}

// APIConnectionDetailsFetcher fetches the connection secret of given composed
// resource if it has a connection secret reference.
type APIConnectionDetailsFetcher struct {
	client client.Client
}

// Fetch returns the connection secret details of composed resource.
func (cdf *APIConnectionDetailsFetcher) Fetch(ctx context.Context, cd resource.Composed, t v1alpha1.ComposedTemplate) (managed.ConnectionDetails, error) {
	sref := cd.GetWriteConnectionSecretToReference()
	if sref == nil {
		return nil, nil
	}
	conn := managed.ConnectionDetails{}
	// It's possible that the composed resource does want to write a
	// connection secret but has not yet. We presume this isn't an issue and
	// that we'll propagate any connection details during a future
	// iteration.
	s := &corev1.Secret{}
	nn := types.NamespacedName{Namespace: sref.Namespace, Name: sref.Name}
	if err := cdf.client.Get(ctx, nn, s); err != nil {
		return nil, errors.Wrap(client.IgnoreNotFound(err), errGetSecret)
	}
	for _, pair := range t.ConnectionDetails {
		if len(s.Data[pair.FromConnectionSecretKey]) == 0 {
			continue
		}
		key := pair.FromConnectionSecretKey
		if pair.Name != nil {
			key = *pair.Name
		}
		conn[key] = s.Data[pair.FromConnectionSecretKey]
	}
	return conn, nil
}
