/*
Copyright 2022 The Crossplane Authors.

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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

// Error strings.
const (
	errGetSecret      = "cannot get connection secret of composed resource"
	errConnDetailName = "connection detail is missing name"
)

// A ConnectionDetailsFetcherFn fetches the connection details of the supplied
// resource, if any.
type ConnectionDetailsFetcherFn func(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error)

// FetchConnection calls the FetchConnectionDetailsFn.
func (f ConnectionDetailsFetcherFn) FetchConnection(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
	return f(ctx, o)
}

// A ConnectionDetailsFetcherChain chains multiple ConnectionDetailsFetchers.
type ConnectionDetailsFetcherChain []managed.ConnectionDetailsFetcher

// FetchConnection details of the supplied composed resource, if any.
func (fc ConnectionDetailsFetcherChain) FetchConnection(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
	all := make(managed.ConnectionDetails)
	for _, p := range fc {
		conn, err := p.FetchConnection(ctx, o)
		if err != nil {
			return nil, err
		}
		for k, v := range conn {
			all[k] = v
		}
	}
	return all, nil
}

// An SecretConnectionDetailsFetcher may use the API server to read connection
// details from a Kubernetes Secret.
type SecretConnectionDetailsFetcher struct {
	client client.Reader
}

// NewSecretConnectionDetailsFetcher returns a ConnectionDetailsFetcher that may
// use the API server to read connection details from a Kubernetes Secret.
func NewSecretConnectionDetailsFetcher(c client.Client) *SecretConnectionDetailsFetcher {
	return &SecretConnectionDetailsFetcher{client: c}
}

// FetchConnection details of the supplied composed resource from its Kubernetes
// connection secret, per its WriteConnectionSecretToRef, if any.
func (cdf *SecretConnectionDetailsFetcher) FetchConnection(ctx context.Context, o resource.ConnectionSecretOwner) (managed.ConnectionDetails, error) {
	sref := o.GetWriteConnectionSecretToReference()
	if sref == nil {
		// secret but has not yet. We presume this isn't an issue and that we'll
		// propagate any connection details during a future iteration.
		return nil, nil
	}
	s := &corev1.Secret{}
	nn := types.NamespacedName{Namespace: sref.Namespace, Name: sref.Name}
	if err := cdf.client.Get(ctx, nn, s); client.IgnoreNotFound(err) != nil {
		return nil, errors.Wrap(err, errGetSecret)
	}
	return s.Data, nil
}

// SecretStoreConnectionPublisher is a ConnectionPublisher that stores
// connection details on the configured SecretStore.
type SecretStoreConnectionPublisher struct {
	publisher managed.ConnectionPublisher
	filter    []string
}

// NewSecretStoreConnectionPublisher returns a SecretStoreConnectionPublisher.
func NewSecretStoreConnectionPublisher(p managed.ConnectionPublisher, filter []string) *SecretStoreConnectionPublisher {
	return &SecretStoreConnectionPublisher{
		publisher: p,
		filter:    filter,
	}
}

// PublishConnection details for the supplied resource.
func (p *SecretStoreConnectionPublisher) PublishConnection(ctx context.Context, o resource.ConnectionSecretOwner, c managed.ConnectionDetails) (published bool, err error) {
	// This resource does not want to expose a connection secret.
	if o.GetPublishConnectionDetailsTo() == nil {
		return false, nil
	}

	data := map[string][]byte{}
	m := map[string]bool{}
	for _, key := range p.filter {
		m[key] = true
	}

	for key, val := range c {
		// If the filter does not have any keys, we allow all given keys to be
		// published.
		if len(m) == 0 || m[key] {
			data[key] = val
		}
	}

	return p.publisher.PublishConnection(ctx, o, data)
}

// UnpublishConnection details for the supplied resource.
func (p *SecretStoreConnectionPublisher) UnpublishConnection(ctx context.Context, o resource.ConnectionSecretOwner, c managed.ConnectionDetails) error {
	return p.publisher.UnpublishConnection(ctx, o, c)
}

// NewSecretStoreConnectionDetailsConfigurator returns a Configurator that
// configures a composite resource using its composition.
func NewSecretStoreConnectionDetailsConfigurator(c client.Client) *SecretStoreConnectionDetailsConfigurator {
	return &SecretStoreConnectionDetailsConfigurator{client: c}
}

// A SecretStoreConnectionDetailsConfigurator configures a composite resource
// using its composition.
type SecretStoreConnectionDetailsConfigurator struct {
	client client.Client
}

// Configure any required fields that were omitted from the composite resource
// by copying them from its composition.
func (c *SecretStoreConnectionDetailsConfigurator) Configure(ctx context.Context, cp resource.Composite, rev *v1.CompositionRevision) error {
	apiVersion, kind := cp.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
	if rev.Spec.CompositeTypeRef.APIVersion != apiVersion || rev.Spec.CompositeTypeRef.Kind != kind {
		return errors.New(errCompositionNotCompatible)
	}

	if cp.GetPublishConnectionDetailsTo() != nil || rev.Spec.PublishConnectionDetailsWithStoreConfigRef == nil {
		return nil
	}

	cp.SetPublishConnectionDetailsTo(&xpv1.PublishConnectionDetailsTo{
		Name: string(cp.GetUID()),
		SecretStoreConfigRef: &xpv1.Reference{
			Name: rev.Spec.PublishConnectionDetailsWithStoreConfigRef.Name,
		},
	})

	return errors.Wrap(c.client.Update(ctx, cp), errUpdateComposite)
}
