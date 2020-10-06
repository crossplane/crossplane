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

	corev1 "k8s.io/api/core/v1"

	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
	"github.com/crossplane/crossplane/pkg/xpkg"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/pkg/errors"
)

const (
	errFetchPackage = "failed to fetch package from remote"
)

// Digester extracts a digest for a package source.
type Digester interface {
	Digest(context.Context, v1alpha1.Package) (string, error)
}

// PackageDigester extracts a digest for a package source.
type PackageDigester struct {
	fetcher xpkg.Fetcher
}

// NewPackageDigester returns a new PackageDigester.
func NewPackageDigester(fetcher xpkg.Fetcher) *PackageDigester {
	return &PackageDigester{
		fetcher: fetcher,
	}
}

// Digest extracts a digest for a package source.
func (d *PackageDigester) Digest(ctx context.Context, p v1alpha1.Package) (string, error) {
	pullPolicy := p.GetPackagePullPolicy()
	if pullPolicy != nil && *pullPolicy == corev1.PullNever {
		return p.GetSource(), nil
	}
	ref, err := name.ParseReference(p.GetSource())
	if err != nil {
		return "", err
	}
	img, err := d.fetcher.Fetch(ctx, ref, v1alpha1.RefNames(p.GetPackagePullSecrets()))
	if err != nil {
		return "", errors.Wrap(err, errFetchPackage)
	}
	h, err := img.Digest()
	if err != nil {
		return "", err
	}
	return h.Hex, nil
}

// NopDigester returns an empty image digest.
type NopDigester struct{}

// NewNopDigester creates a NopDigester.
func NewNopDigester() *NopDigester {
	return &NopDigester{}
}

// Digest returns an empty image digest and no error.
func (d *NopDigester) Digest(context.Context, v1alpha1.Package) (string, error) {
	return "", nil
}
