/*
Copyright 2025 The Crossplane Authors.

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

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
)

// Error strings.
const (
	errGetSecret      = "cannot get connection secret of composed resource"
	errConnDetailName = "connection detail is missing name"
)

// A ConnectionDetailsFetcherFn fetches the connection details of the supplied
// resource, if any.
type ConnectionDetailsFetcherFn func(ctx context.Context, o ConnectionSecretOwner) (managed.ConnectionDetails, error)

// FetchConnection calls the FetchConnectionDetailsFn.
func (f ConnectionDetailsFetcherFn) FetchConnection(ctx context.Context, o ConnectionSecretOwner) (managed.ConnectionDetails, error) {
	return f(ctx, o)
}

// A ConnectionDetailsFetcherChain chains multiple ConnectionDetailsFetchers.
type ConnectionDetailsFetcherChain []ConnectionDetailsFetcher

// FetchConnection details of the supplied composed resource, if any.
func (fc ConnectionDetailsFetcherChain) FetchConnection(ctx context.Context, o ConnectionSecretOwner) (managed.ConnectionDetails, error) {
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
func (cdf *SecretConnectionDetailsFetcher) FetchConnection(ctx context.Context, o ConnectionSecretOwner) (managed.ConnectionDetails, error) {
	sref := o.GetWriteConnectionSecretToReference()
	if sref == nil {
		return nil, nil
	}

	// Namespaced resources always write connection secrets to their own
	// namespace. Cluster scoped resources should have a namespace in the ref.
	ns := o.GetNamespace()
	if ns == "" {
		ns = sref.Namespace
	}

	s := &corev1.Secret{}
	nn := types.NamespacedName{Namespace: ns, Name: sref.Name}
	if err := cdf.client.Get(ctx, nn, s); client.IgnoreNotFound(err) != nil {
		return nil, errors.Wrap(err, errGetSecret)
	}
	return s.Data, nil
}
