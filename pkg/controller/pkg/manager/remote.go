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

package manager

import (
	"context"

	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"k8s.io/client-go/kubernetes"

	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
)

// Digester acquires a digest for a package.
type Digester interface {
	Fetch(context.Context, v1alpha1.Package) (string, error)
}

// Remote gets image digest from a remote registry.
type Remote struct {
	namespace string
	client    kubernetes.Interface
}

// NewRemote returns a new remote digester.
func NewRemote(client kubernetes.Interface, namespace string) *Remote {
	return &Remote{
		client:    client,
		namespace: namespace,
	}
}

// Fetch acquires image digest from a registry.
func (d *Remote) Fetch(ctx context.Context, p v1alpha1.Package) (string, error) {
	ref, err := name.ParseReference(p.GetSource())
	if err != nil {
		return "", err
	}
	auth, err := k8schain.New(ctx, d.client, k8schain.Options{
		Namespace:        d.namespace,
		ImagePullSecrets: v1alpha1.RefNames(p.GetPackagePullSecrets()),
	})
	if err != nil {
		return "", err
	}
	img, err := remote.Image(ref, remote.WithAuthFromKeychain(auth))
	if err != nil {
		return "", err
	}
	h, err := img.Digest()
	if err != nil {
		return "", err
	}
	return h.Hex, nil
}

// NopDigester returns an empty digest.
type NopDigester struct{}

// NewNopDigester creates a NopDigester.
func NewNopDigester() *NopDigester {
	return &NopDigester{}
}

// Fetch returns an empty digest and no error.
func (d *NopDigester) Fetch(context.Context, v1alpha1.Package) (string, error) {
	return "", nil
}
