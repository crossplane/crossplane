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

package xpkg

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net/http"

	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
)

func init() { //nolint:gochecknoinits // See comment below.
	// NOTE(hasheddan): we set the logrus package-level logger to discard output
	// due to the fact that the AWS ECR credential helper uses it to log errors
	// when parsing registry server URL, which happens any time a package is
	// pulled from a non-ECR registry.
	// https://github.com/awslabs/amazon-ecr-credential-helper/issues/308
	logrus.SetOutput(io.Discard)
}

// Fetcher fetches package images.
type Fetcher interface {
	Fetch(ctx context.Context, ref name.Reference, secrets ...string) (v1.Image, error)
	Head(ctx context.Context, ref name.Reference, secrets ...string) (*v1.Descriptor, error)
	Tags(ctx context.Context, ref name.Reference, secrets ...string) ([]string, error)
}

// K8sFetcher uses kubernetes credentials to fetch package images.
type K8sFetcher struct {
	client         kubernetes.Interface
	namespace      string
	serviceAccount string
	transport      http.RoundTripper
	userAgent      string
}

// FetcherOpt can be used to add optional parameters to NewK8sFetcher.
type FetcherOpt func(k *K8sFetcher) error

// WithCustomCA is a FetcherOpt that can be used to add a custom CA bundle to a K8sFetcher.
func WithCustomCA(rootCAs *x509.CertPool) FetcherOpt {
	return func(k *K8sFetcher) error {
		t, ok := k.transport.(*http.Transport)
		if !ok {
			return errors.New("Fetcher transport is not an HTTP transport")
		}

		t.TLSClientConfig = &tls.Config{RootCAs: rootCAs, MinVersion: tls.VersionTLS12}
		return nil
	}
}

// WithUserAgent is a FetcherOpt that can be used to set the user agent on all HTTP requests.
func WithUserAgent(userAgent string) FetcherOpt {
	return func(k *K8sFetcher) error {
		// TODO(hasheddan): go-containerregistry currently does not allow for
		// removal of the go-containerregistry user-agent header, so the
		// provided one is appended rather than replacing. In the future, this
		// should be replaced with wrapping the transport with
		// transport.NewUserAgent.
		k.userAgent = userAgent
		return nil
	}
}

// WithNamespace is a FetcherOpt that sets the Namespace for fetching package
// pull secrets.
func WithNamespace(ns string) FetcherOpt {
	return func(k *K8sFetcher) error {
		k.namespace = ns
		return nil
	}
}

// WithServiceAccount is a FetcherOpt that sets the ServiceAccount name for
// fetching package pull secrets.
func WithServiceAccount(sa string) FetcherOpt {
	return func(k *K8sFetcher) error {
		k.serviceAccount = sa
		return nil
	}
}

// NewK8sFetcher creates a new K8sFetcher.
func NewK8sFetcher(client kubernetes.Interface, opts ...FetcherOpt) (*K8sFetcher, error) {
	dt, ok := remote.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, errors.Errorf("default transport was not a %T", &http.Transport{})
	}
	k := &K8sFetcher{
		client:    client,
		transport: dt.Clone(),
	}

	for _, o := range opts {
		if err := o(k); err != nil {
			return nil, err
		}
	}

	return k, nil
}

// Fetch fetches a package image.
func (i *K8sFetcher) Fetch(ctx context.Context, ref name.Reference, secrets ...string) (v1.Image, error) {
	auth, err := k8schain.New(ctx, i.client, k8schain.Options{
		Namespace:          i.namespace,
		ServiceAccountName: i.serviceAccount,
		ImagePullSecrets:   secrets,
	})
	if err != nil {
		return nil, err
	}
	return remote.Image(ref,
		remote.WithAuthFromKeychain(auth),
		remote.WithTransport(i.transport),
		remote.WithContext(ctx),
		remote.WithUserAgent(i.userAgent),
	)
}

// Head fetches a package descriptor.
func (i *K8sFetcher) Head(ctx context.Context, ref name.Reference, secrets ...string) (*v1.Descriptor, error) {
	auth, err := k8schain.New(ctx, i.client, k8schain.Options{
		Namespace:          i.namespace,
		ServiceAccountName: i.serviceAccount,
		ImagePullSecrets:   secrets,
	})
	if err != nil {
		return nil, err
	}
	d, err := remote.Head(ref,
		remote.WithAuthFromKeychain(auth),
		remote.WithTransport(i.transport),
		remote.WithContext(ctx),
		remote.WithUserAgent(i.userAgent),
	)
	if err != nil || d == nil {
		rd, gErr := remote.Get(ref,
			remote.WithAuthFromKeychain(auth),
			remote.WithTransport(i.transport),
			remote.WithContext(ctx),
			remote.WithUserAgent(i.userAgent),
		)
		if gErr != nil {
			return nil, errors.Wrapf(gErr, "failed to fetch package descriptor with a GET request after a previous HEAD request failure: %v", err)
		}
		return &rd.Descriptor, nil
	}
	return d, nil
}

// Tags fetches a package's tags.
func (i *K8sFetcher) Tags(ctx context.Context, ref name.Reference, secrets ...string) ([]string, error) {
	auth, err := k8schain.New(ctx, i.client, k8schain.Options{
		Namespace:          i.namespace,
		ServiceAccountName: i.serviceAccount,
		ImagePullSecrets:   secrets,
	})
	if err != nil {
		return nil, err
	}
	return remote.List(ref.Context(),
		remote.WithAuthFromKeychain(auth),
		remote.WithTransport(i.transport),
		remote.WithContext(ctx),
		remote.WithUserAgent(i.userAgent),
	)
}

// NopFetcher always returns an empty image and never returns error.
type NopFetcher struct{}

// NewNopFetcher creates a new NopFetcher.
func NewNopFetcher() *NopFetcher {
	return &NopFetcher{}
}

// Fetch fetches an empty image and does not return error.
func (n *NopFetcher) Fetch(_ context.Context, _ name.Reference, _ ...string) (v1.Image, error) {
	return empty.Image, nil
}

// Head returns a nil descriptor and does not return error.
func (n *NopFetcher) Head(_ context.Context, _ name.Reference, _ ...string) (*v1.Descriptor, error) {
	return nil, nil
}

// Tags returns a nil slice and does not return error.
func (n *NopFetcher) Tags(_ context.Context, _ name.Reference, _ ...string) ([]string, error) {
	return nil, nil
}
