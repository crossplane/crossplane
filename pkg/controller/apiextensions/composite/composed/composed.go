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
	"sigs.k8s.io/controller-runtime/pkg/client"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
)

// Error strings
const (
	errApply       = "cannot apply composed resource"
	errFetchSecret = "cannot fetch connection secret"
	errOverlay     = "cannot apply overlay"
	errConfigure   = "cannot configure composed resource"
)

// Configurator is used to configure the Composed resource.
type Configurator interface {
	Configure(cp resource.Composite, cd resource.Composed, t v1alpha1.ComposedTemplate) error
}

// OverlayApplicator is used to apply an overlay at each reconcile.
type OverlayApplicator interface {
	Overlay(cp resource.Composite, cd resource.Composed, t v1alpha1.ComposedTemplate) error
}

// ConnectionDetailsFetcher fetches the connection details of the Composed resource.
type ConnectionDetailsFetcher interface {
	Fetch(ctx context.Context, cd resource.Composed, t v1alpha1.ComposedTemplate) (managed.ConnectionDetails, error)
}

// Observation is the result of composed reconciliation.
type Observation struct {
	Ref               corev1.ObjectReference
	ConnectionDetails managed.ConnectionDetails
	Ready             bool
}

// WithClientApplicator returns a ComposerOption that changes the ClientApplicator of
// Composer.
func WithClientApplicator(ca resource.ClientApplicator) ComposerOption {
	return func(composer *Composer) {
		composer.client = ca
	}
}

// WithConnectionDetailFetcher returns a ComposerOption that changes the
// ConnectionDetailsFetcher of Composer.
func WithConnectionDetailFetcher(cdf ConnectionDetailsFetcher) ComposerOption {
	return func(composer *Composer) {
		composer.ConnectionDetailsFetcher = cdf
	}
}

// WithOverlayApplicator returns a ComposerOption that changes the
// OverlayApplicator of Composer.
func WithOverlayApplicator(oa OverlayApplicator) ComposerOption {
	return func(composer *Composer) {
		composer.OverlayApplicator = oa
	}
}

// WithConfigurator returns a ComposerOption that changes the Configurator of
// Composer.
func WithConfigurator(c Configurator) ComposerOption {
	return func(composer *Composer) {
		composer.Configurator = c
	}
}

type connection struct {
	ConnectionDetailsFetcher
}

type composed struct {
	Configurator
	OverlayApplicator
}

// ComposerOption configures the Composer object.
type ComposerOption func(*Composer)

// NewComposer returns a new Composer that composes infrastructure resources
// in a Kubernetes API server.
func NewComposer(kube client.Client, opts ...ComposerOption) *Composer {
	c := &Composer{
		client: resource.ClientApplicator{
			Client:     kube,
			Applicator: resource.NewAPIPatchingApplicator(kube),
		},
		composed: composed{
			Configurator:      &DefaultConfigurator{},
			OverlayApplicator: &DefaultOverlayApplicator{},
		},
		connection: connection{
			ConnectionDetailsFetcher: &APIConnectionDetailsFetcher{client: kube},
		},
	}

	for _, f := range opts {
		f(c)
	}
	return c
}

// An Composer composes infrastructure resources in a Kubernetes API server.
type Composer struct {
	client resource.ClientApplicator
	connection
	composed
}

// Compose the supplied Composed resource into the supplied Composite resource
// using the supplied CompositeTemplate.
func (r *Composer) Compose(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1alpha1.ComposedTemplate) (Observation, error) {

	// Doing the configuration only once or continuously is subject to discussion
	// in https://github.com/crossplane/crossplane/issues/1481
	// Until it's resolved, it's done in every reconcile.
	if err := r.composed.Configure(cp, cd, t); err != nil {
		return Observation{}, errors.Wrap(err, errConfigure)
	}

	// Overlay is applied to the Composed resource in all cases so that we can
	// keep Composed resource up-to-date with the changes in Composite resource.
	if err := r.composed.Overlay(cp, cd, t); err != nil {
		return Observation{}, errors.Wrap(err, errOverlay)
	}

	// Connection details are fetched in all cases in a best-effort mode, i.e.
	// it doesn't return error if the secret does not exist or the resource
	// does not publish a secret at all.
	conn, err := r.connection.Fetch(ctx, cd, t)
	if err != nil {
		return Observation{}, errors.Wrap(err, errFetchSecret)
	}

	// We use AddOwnerReference rather than AddControllerReference because we
	// don't need the latter to check whether a controller reference is already
	// set.
	meta.AddOwnerReference(cd, meta.AsController(meta.ReferenceTo(cp, cp.GetObjectKind().GroupVersionKind())))

	// Apply should be the last operation of this function so that we can return
	// the reference to be stored in the Composite resource immediately.
	if err := r.client.Apply(ctx, cd, resource.MustBeControllableBy(cp.GetUID())); err != nil {
		return Observation{}, errors.Wrap(err, errApply)
	}

	obs := Observation{
		Ref:               *meta.ReferenceTo(cd, cd.GetObjectKind().GroupVersionKind()),
		Ready:             resource.IsConditionTrue(cd.GetCondition(runtimev1alpha1.TypeReady)),
		ConnectionDetails: conn,
	}
	return obs, nil
}
