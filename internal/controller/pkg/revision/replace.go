/*
Copyright 2024 The Crossplane Authors.

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

package revision

import (
	"context"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/internal/xpkg"
)

// A SourceResolver resolves a package source to an installed package.
type SourceResolver interface {
	// Resolve the supplied package source to an installed package. A package
	// source is an OCI reference without a tag or digest. Returns an error that
	// satisfies kerrors.IsNotFound if no installed package is found.
	Resolve(ctx context.Context, source string) (v1.Package, error)
}

// A SourceResolverFn is a function that satisfies SourceResolver.
type SourceResolverFn func(ctx context.Context, source string) (v1.Package, error)

// Resolve a source a package source to an installed package.
func (fn SourceResolverFn) Resolve(ctx context.Context, source string) (v1.Package, error) {
	return fn(ctx, source)
}

// A NopSourceResolver is a SourceResolver that always returns not found.
type NopSourceResolver struct{}

// NewNopSourceResolver returns a SourceResolver that always returns not found.
func NewNopSourceResolver() *NopSourceResolver { return &NopSourceResolver{} }

// Resolve always returns an error that satisfies kerrors.IsNotFound.
func (r *NopSourceResolver) Resolve(_ context.Context, _ string) (v1.Package, error) {
	return nil, kerrors.NewNotFound(schema.GroupResource{Group: "pkg.crossplane.io"}, "")
}

// A ListResolver resolves a source to a package by listing packages and
// filtering by source.
type ListResolver struct {
	client  client.Reader
	pkgList v1.PackageList
}

// NewListResolver returns a SourceResolver that resolves a source to a package
// by listing packages and filtering by source.
func NewListResolver(c client.Reader, pl v1.PackageList) *ListResolver {
	return &ListResolver{client: c, pkgList: pl}
}

// Resolve the supplied source to an installed package. Returns an error that
// satisfies kerrors.IsNotFound if there's no package installed from the source.
func (r *ListResolver) Resolve(ctx context.Context, source string) (v1.Package, error) {
	// If we made it this far, no active package revision matched our source.
	// Check active packages.
	pl := r.pkgList.DeepCopyObject().(v1.PackageList) //nolint:forcetypeassert // Guaranteed to be this type.
	if err := r.client.List(ctx, pl); err != nil {
		return nil, errors.Wrap(err, "cannot list packages")
	}

	for _, pkg := range pl.GetPackages() {
		if s, err := xpkg.ParsePackageSourceFromString(pkg.GetSource()); err == nil && s == source {
			return pkg, nil
		}
	}

	// TODO(negz): Return our own error?
	return nil, kerrors.NewNotFound(schema.GroupResource{Group: "pkg.crossplane.io"}, "")
}

// A PackageDeactivator deactivates a package.
type PackageDeactivator interface {
	// Deactivate the supplied package and its active revision. Returns true if
	// it updates a package, a revision, or both.
	Deactivate(ctx context.Context, pkg v1.Package) (deactivated bool, err error)
}

// A PackageDeactivatorFn is a function that satisfies the PackageDeactivator
// interface.
type PackageDeactivatorFn func(ctx context.Context, pkg v1.Package) (bool, error)

// Deactivate the supplied package and its active revision.
func (fn PackageDeactivatorFn) Deactivate(ctx context.Context, pkg v1.Package) (bool, error) {
	return fn(ctx, pkg)
}

// A NopPackageDeactivator does nothing.
type NopPackageDeactivator struct{}

// NewNopPackageDeactivator returns a PackageDeactivator that does nothing.
func NewNopPackageDeactivator() *NopPackageDeactivator { return &NopPackageDeactivator{} }

// Deactivate does nothing.
func (d *NopPackageDeactivator) Deactivate(_ context.Context, _ v1.Package) (bool, error) {
	return false, nil
}

// A PackageAndRevisionDeactivator deactivates a package and its active
// revision. It sets the package's revision activation policy to manual, then
// finds its active revision and sets its desired state to inactive.
type PackageAndRevisionDeactivator struct {
	client  client.Client
	revList v1.PackageRevisionList
}

// NewPackageAndRevisionDeactivator deactivates a package and its active
// revision.
func NewPackageAndRevisionDeactivator(c client.Client, rl v1.PackageRevisionList) *PackageAndRevisionDeactivator {
	return &PackageAndRevisionDeactivator{client: c, revList: rl}
}

// Deactivate deactivates the supplied package and its active revision.
func (d *PackageAndRevisionDeactivator) Deactivate(ctx context.Context, pkg v1.Package) (bool, error) {
	deactivated := false

	// Set the revision activation policy to manual before we deactivate the
	// active revision. This ensures the package controller won't reactivate it.
	if ptr.Deref(pkg.GetActivationPolicy(), v1.AutomaticActivation) != v1.ManualActivation {
		pkg.SetActivationPolicy(&v1.ManualActivation)
		if err := d.client.Update(ctx, pkg); err != nil {
			return false, errors.Wrap(err, "cannot set package revision activation policy to Manual")
		}
		deactivated = true
	}

	rl := d.revList.DeepCopyObject().(v1.PackageRevisionList) //nolint:forcetypeassert // Guaranteed to be this type.
	if err := d.client.List(ctx, rl); err != nil {
		return deactivated, errors.Wrap(err, "cannot list package revisions")
	}

	for _, rev := range rl.GetRevisions() {
		if rev.GetDesiredState() == v1.PackageRevisionInactive {
			// Already inactive.
			continue
		}
		for _, ref := range rev.GetOwnerReferences() {
			if !ptr.Deref(ref.Controller, false) {
				// Not the controller of this revision - so not a package.
				continue
			}

			if ref.UID != pkg.GetUID() {
				// Not this package's active revision
				continue
			}

			// This is our package's active revision. Deactivate it.
			rev.SetDesiredState(v1.PackageRevisionInactive)
			return true, errors.Wrapf(d.client.Update(ctx, rev), "cannot deactivate package revision %q", rev.GetName())
		}
	}

	// We want to return true if we made any change - i.e. if we either set the
	// package's revision activation policy to manual or deactivated a revision.
	return deactivated, nil
}
