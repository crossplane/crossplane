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

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

// ConnectionDetailsFetcher fetches the connection details of the Composed resource.
type ConnectionDetailsFetcher interface {
	FetchConnectionDetails(ctx context.Context, cd resource.Composed, t v1.ComposedTemplate) (managed.ConnectionDetails, error)
}

// A ConnectionDetailsFetcherChain chains multiple ConnectionDetailsFetchers.
type ConnectionDetailsFetcherChain []ConnectionDetailsFetcher

// FetchConnectionDetails of the supplied composed resource, if any.
func (fc ConnectionDetailsFetcherChain) FetchConnectionDetails(ctx context.Context, cd resource.Composed, t v1.ComposedTemplate) (managed.ConnectionDetails, error) {
	all := make(managed.ConnectionDetails)
	for _, p := range fc {
		conn, err := p.FetchConnectionDetails(ctx, cd, t)
		if err != nil {
			return all, err
		}
		for k, v := range conn {
			all[k] = v
		}
	}
	return all, nil
}

// SecretStoreConnectionPublisher is a ConnectionPublisher that stores
// connection details on the configured SecretStore.
type SecretStoreConnectionPublisher struct {
	publisher managed.ConnectionPublisher
	filter    []string
}

// NewSecretStoreConnectionPublisher returns a SecretStoreConnectionPublisher
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

// SecretStoreConnectionDetailsFetcher is a ConnectionDetailsFetcher that
// fetches connection details to the configured SecretStore.
type SecretStoreConnectionDetailsFetcher struct {
	fetcher managed.ConnectionDetailsFetcher
}

// NewSecretStoreConnectionDetailsFetcher returns a
// SecretStoreConnectionDetailsFetcher
func NewSecretStoreConnectionDetailsFetcher(f managed.ConnectionDetailsFetcher) *SecretStoreConnectionDetailsFetcher {
	return &SecretStoreConnectionDetailsFetcher{
		fetcher: f,
	}
}

// FetchConnectionDetails of the supplied composed resource, if any.
func (f *SecretStoreConnectionDetailsFetcher) FetchConnectionDetails(ctx context.Context, cd resource.Composed, t v1.ComposedTemplate) (managed.ConnectionDetails, error) { // nolint:gocyclo
	// NOTE(turkenh): Added linter exception for gocyclo similar to existing
	// APIConnectionDetailsFetcher.FetchConnectionDetails method given most
	// of the complexity coming from simply if checks and, I wanted to keep this
	// as identical as possible to aforementioned method. This can be refactored
	// with the removal of "WriteConnectionSecretRef" API.

	so := cd.(resource.ConnectionSecretOwner)
	data, err := f.fetcher.FetchConnection(ctx, so)
	if err != nil {
		return nil, errors.Wrap(err, errFetchSecret)
	}

	conn := managed.ConnectionDetails{}
	for _, d := range t.ConnectionDetails {
		switch tp := connectionDetailType(d); tp {
		case v1.ConnectionDetailTypeFromConnectionSecretKey:
			if d.FromConnectionSecretKey == nil {
				return nil, errors.Errorf(errFmtConnDetailKey, tp)
			}
			if data == nil || data[*d.FromConnectionSecretKey] == nil {
				// We don't consider this an error because it's possible the
				// key will still be written at some point in the future.
				continue
			}
			key := *d.FromConnectionSecretKey
			if d.Name != nil {
				key = *d.Name
			}
			if key != "" {
				conn[key] = data[*d.FromConnectionSecretKey]
			}
		case v1.ConnectionDetailTypeFromFieldPath, v1.ConnectionDetailTypeFromValue, v1.ConnectionDetailTypeUnknown:
			// We do nothing here with these cases, either:
			// - ConnectionDetailTypeFromFieldPath,ConnectionDetailTypeFromValue
			//   => Already covered by APIConnectionDetailsFetcher.FetchConnectionDetails
			// - ConnectionDetailTypeUnknown
			//   => We weren't able to determine the type of this connection detail.
		}
	}

	if len(conn) == 0 {
		return nil, nil
	}

	return conn, nil
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
func (c *SecretStoreConnectionDetailsConfigurator) Configure(ctx context.Context, cp resource.Composite, comp *v1.Composition) error {
	apiVersion, kind := cp.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
	if comp.Spec.CompositeTypeRef.APIVersion != apiVersion || comp.Spec.CompositeTypeRef.Kind != kind {
		return errors.New(errCompositionNotCompatible)
	}

	if cp.GetPublishConnectionDetailsTo() != nil || comp.Spec.PublishConnectionDetailsWithStoreConfigRef == nil {
		return nil
	}

	cp.SetPublishConnectionDetailsTo(&xpv1.PublishConnectionDetailsTo{
		Name: string(cp.GetUID()),
		SecretStoreConfigRef: &xpv1.Reference{
			Name: comp.Spec.PublishConnectionDetailsWithStoreConfigRef.Name,
		},
	})

	return errors.Wrap(c.client.Update(ctx, cp), errUpdateComposite)
}
