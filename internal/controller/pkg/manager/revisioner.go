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

	"github.com/google/go-containerregistry/pkg/name"
	corev1 "k8s.io/api/core/v1"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/internal/xpkg"
)

const (
	errBadReference = "package tag is not a valid reference"
	errFetchPackage = "failed to fetch package digest from remote"
)

// Revisioner extracts a revision name for a package source.
type Revisioner interface {
	Revision(ctx context.Context, p v1.Package) (string, error)
}

// PackageRevisioner extracts a revision name for a package source.
type PackageRevisioner struct {
	fetcher  xpkg.Fetcher
	registry string
}

// A PackageRevisionerOption sets configuration for a package revisioner.
type PackageRevisionerOption func(r *PackageRevisioner)

// WithDefaultRegistry sets the default registry that a package revisioner will use.
func WithDefaultRegistry(registry string) PackageRevisionerOption {
	return func(r *PackageRevisioner) {
		r.registry = registry
	}
}

// NewPackageRevisioner returns a new PackageRevisioner.
func NewPackageRevisioner(fetcher xpkg.Fetcher, opts ...PackageRevisionerOption) *PackageRevisioner {
	r := &PackageRevisioner{
		fetcher: fetcher,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Revision extracts a revision name for a package source.
func (r *PackageRevisioner) Revision(ctx context.Context, p v1.Package) (string, error) {
	pullPolicy := p.GetPackagePullPolicy()
	if pullPolicy != nil && *pullPolicy == corev1.PullNever {
		return xpkg.FriendlyID(p.GetName(), p.GetSource()), nil
	}
	if pullPolicy != nil && *pullPolicy == corev1.PullIfNotPresent {
		if p.GetCurrentIdentifier() == p.GetSource() {
			return p.GetCurrentRevision(), nil
		}
	}
	ref, err := name.ParseReference(p.GetSource(), name.WithDefaultRegistry(r.registry))
	if err != nil {
		return "", errors.Wrap(err, errBadReference)
	}
	d, err := r.fetcher.Head(ctx, ref, v1.RefNames(p.GetPackagePullSecrets())...)
	if err != nil || d == nil {
		return "", errors.Wrap(err, errFetchPackage)
	}
	return xpkg.FriendlyID(p.GetName(), d.Digest.Hex), nil
}

// NopRevisioner returns an empty revision name.
type NopRevisioner struct{}

// NewNopRevisioner creates a NopRevisioner.
func NewNopRevisioner() *NopRevisioner {
	return &NopRevisioner{}
}

// Revision returns an empty revision name and no error.
func (d *NopRevisioner) Revision(context.Context, v1.Package) (string, error) {
	return "", nil
}
