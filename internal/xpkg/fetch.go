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
	"io/ioutil"
	"net/http"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
)

// Fetcher fetches package images.
type Fetcher interface {
	Fetch(ctx context.Context, ref name.Reference, secrets ...string) (v1.Image, error)
	Head(ctx context.Context, ref name.Reference, secrets ...string) (*v1.Descriptor, error)
	Tags(ctx context.Context, ref name.Reference, secrets ...string) ([]string, error)
}

// K8sFetcher uses kubernetes credentials to fetch package images.
type K8sFetcher struct {
	client    kubernetes.Interface
	namespace string
	transport http.RoundTripper
}

// NewK8sFetcher creates a new K8sFetcher.
func NewK8sFetcher(client kubernetes.Interface, namespace, caBundlePath string) (*K8sFetcher, error) {

	// return early with default transport if no additional CAs
	if caBundlePath == "" {
		return &K8sFetcher{
			client:    client,
			namespace: namespace,
			transport: http.DefaultTransport,
		}, nil
	}

	// Get the SystemCertPool, continue with an empty pool on error
	rootCAs, _ := x509.SystemCertPool()
	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}

	// Read in the cert file
	certs, err := ioutil.ReadFile(filepath.Clean(caBundlePath))
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to append %q to RootCAs", caBundlePath)
	}

	// Append our cert to the system pool
	if ok := rootCAs.AppendCertsFromPEM(certs); !ok {
		return nil, errors.Errorf("No certificates could be parsed from %q", caBundlePath)
	}

	var customCATransport http.RoundTripper = http.DefaultTransport
	customCATransport.(*http.Transport).TLSClientConfig = &tls.Config{RootCAs: rootCAs, MinVersion: tls.VersionTLS12}

	return &K8sFetcher{
		client:    client,
		namespace: namespace,
		transport: customCATransport,
	}, nil
}

// Fetch fetches a package image.
func (i *K8sFetcher) Fetch(ctx context.Context, ref name.Reference, secrets ...string) (v1.Image, error) {
	auth, err := k8schain.New(ctx, i.client, k8schain.Options{
		Namespace:        i.namespace,
		ImagePullSecrets: secrets,
	})
	if err != nil {
		return nil, err
	}
	return remote.Image(ref, remote.WithAuthFromKeychain(auth), remote.WithTransport(i.transport), remote.WithContext(ctx))
}

// Head fetches a package descriptor.
func (i *K8sFetcher) Head(ctx context.Context, ref name.Reference, secrets ...string) (*v1.Descriptor, error) {
	auth, err := k8schain.New(ctx, i.client, k8schain.Options{
		Namespace:        i.namespace,
		ImagePullSecrets: secrets,
	})
	if err != nil {
		return nil, err
	}
	return remote.Head(ref, remote.WithAuthFromKeychain(auth), remote.WithTransport(i.transport), remote.WithContext(ctx))
}

// Tags fetches a package's tags.
func (i *K8sFetcher) Tags(ctx context.Context, ref name.Reference, secrets ...string) ([]string, error) {
	auth, err := k8schain.New(ctx, i.client, k8schain.Options{
		Namespace:        i.namespace,
		ImagePullSecrets: secrets,
	})
	if err != nil {
		return nil, err
	}
	return remote.List(ref.Context(), remote.WithAuthFromKeychain(auth), remote.WithTransport(i.transport), remote.WithContext(ctx))
}

// NopFetcher always returns an empty image and never returns error.
type NopFetcher struct{}

// NewNopFetcher creates a new NopFetcher.
func NewNopFetcher() *NopFetcher {
	return &NopFetcher{}
}

// Fetch fetches an empty image and does not return error.
func (n *NopFetcher) Fetch(ctx context.Context, ref name.Reference, secrets ...string) (v1.Image, error) {
	return empty.Image, nil
}

// Head returns a nil descriptor and does not return error.
func (n *NopFetcher) Head(ctx context.Context, ref name.Reference, secrets ...string) (*v1.Descriptor, error) {
	return nil, nil
}

// Tags returns a nil slice and does not return error.
func (n *NopFetcher) Tags(ctx context.Context, ref name.Reference, secrets ...string) ([]string, error) {
	return nil, nil
}
