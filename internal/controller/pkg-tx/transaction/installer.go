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

package transaction

import (
	"cmp"
	"context"
	"slices"

	"github.com/google/go-containerregistry/pkg/name"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	pkgmetav1 "github.com/crossplane/crossplane/v2/apis/pkg/meta/v1"
	v1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"
	"github.com/crossplane/crossplane/v2/apis/pkg/v1alpha1"
	"github.com/crossplane/crossplane/v2/internal/xpkg"
	"github.com/crossplane/crossplane/v2/internal/xpkg/dependency"
)

const (
	labelTransaction = "pkg.crossplane.io/transaction"
)

// Establisher establishes control or ownership of a set of resources in the
// API server by checking that control or ownership can be established for all
// resources and then establishing it.
type Establisher interface {
	Establish(ctx context.Context, objects []runtime.Object, parent v1.PackageRevision, control bool) ([]xpv1.TypedReference, error)
}

// PackageInstaller installs packages in dependency order.
type PackageInstaller struct {
	kube    client.Client
	pkg     xpkg.Client
	objects Establisher
}

// InstallPackages installs all packages in the transaction in dependency order.
func (i *PackageInstaller) InstallPackages(ctx context.Context, tx *v1alpha1.Transaction) error {
	sorted, err := dependency.SortLockPackages(tx.Status.ProposedLockPackages)
	if err != nil {
		return errors.Wrap(err, "cannot sort packages by dependency order")
	}

	for _, lockPkg := range sorted {
		xp, err := i.pkg.Get(ctx, lockPkg.Source+"@"+lockPkg.Version)
		if err != nil {
			return errors.Wrapf(err, "cannot fetch package %s", lockPkg.Source)
		}

		if err := i.InstallPackage(ctx, tx, xp); err != nil {
			return errors.Wrapf(err, "cannot install package %s", lockPkg.Source)
		}

		if err := i.InstallPackageRevision(ctx, tx, xp); err != nil {
			return errors.Wrapf(err, "cannot install package revision %s", lockPkg.Source)
		}

		if err := i.InstallObjects(ctx, tx, xp); err != nil {
			return errors.Wrapf(err, "cannot install objects for package %s", lockPkg.Source)
		}
	}

	return nil
}

// InstallPackage creates or updates the Package resource. Uses CreateOrUpdate
// so it's idempotent for both new packages and existing ones.
func (i *PackageInstaller) InstallPackage(ctx context.Context, tx *v1alpha1.Transaction, xp *xpkg.Package) error {
	pkg, _, err := NewPackageAndRevision(xp)
	if err != nil {
		return err
	}

	_, err = ctrl.CreateOrUpdate(ctx, i.kube, pkg, func() error {
		pkg.SetSource(xp.Source + "@" + xp.Digest) // TODO(negz): Use tag.
		meta.AddLabels(pkg, map[string]string{
			labelTransaction: tx.GetName(),
		})

		return nil
	})

	return errors.Wrap(err, "cannot create or update package")
}

// InstallPackageRevision installs a PackageRevision by creating or updating it,
// deactivating other revisions, and garbage collecting old revisions.
//
// The package manager maintains at most one active revision per package at any
// time. When a new revision is installed, all other revisions are deactivated.
// This ensures a clean transition between package versions.
func (i *PackageInstaller) InstallPackageRevision(ctx context.Context, tx *v1alpha1.Transaction, xp *xpkg.Package) error {
	pkg, rev, err := NewPackageAndRevision(xp)
	if err != nil {
		return err
	}

	if err := i.kube.Get(ctx, types.NamespacedName{Name: pkg.GetName()}, pkg); err != nil {
		return errors.Wrap(err, "cannot get package")
	}

	_, list, err := NewPackageAndRevisionList(xp)
	if err != nil {
		return err
	}

	if err := i.kube.List(ctx, list, client.MatchingLabels{v1.LabelParentPackage: pkg.GetName()}); err != nil {
		return errors.Wrap(err, "cannot list package revisions")
	}

	revs := list.GetRevisions()

	// Find the highest revision number to ensure our new revision gets a
	// higher number. Revision numbers are monotonically increasing.
	var maxRevision int64
	for _, r := range revs {
		if r.GetRevision() > maxRevision {
			maxRevision = r.GetRevision()
		}
	}

	// Deactivate all other revisions before activating the new one. This
	// ensures only one revision is active at a time, regardless of the
	// package's activation policy.
	for _, r := range revs {
		if r.GetName() == rev.GetName() {
			continue
		}
		if r.GetDesiredState() != v1.PackageRevisionActive {
			continue
		}
		r.SetDesiredState(v1.PackageRevisionInactive)
		if err := i.kube.Update(ctx, r); err != nil {
			return errors.Wrapf(err, "cannot deactivate revision %s", r.GetName())
		}
	}

	if _, err = ctrl.CreateOrUpdate(ctx, i.kube, rev, func() error {
		// The current revision should always be the highest numbered revision.
		// This ensures monotonically increasing revision numbers even when
		// reverting to an older package version.
		if rev.GetRevision() < maxRevision || maxRevision == 0 {
			rev.SetRevision(maxRevision + 1)
		}

		// Package owns the revision for lifecycle management.
		meta.AddOwnerReference(rev, meta.AsOwner(meta.TypedReferenceTo(pkg, pkg.GetObjectKind().GroupVersionKind())))

		meta.AddLabels(rev, map[string]string{
			v1.LabelParentPackage: pkg.GetName(),
			labelTransaction:      tx.GetName(),
		})

		// Propagate package configuration to revision.
		rev.SetSource(xp.Source + "@" + xp.Digest) // TODO(negz): Use tag?
		rev.SetPackagePullPolicy(pkg.GetPackagePullPolicy())
		rev.SetPackagePullSecrets(pkg.GetPackagePullSecrets())
		rev.SetIgnoreCrossplaneConstraints(pkg.GetIgnoreCrossplaneConstraints())
		rev.SetSkipDependencyResolution(pkg.GetSkipDependencyResolution())
		rev.SetCommonLabels(pkg.GetCommonLabels())

		// Propagate runtime configuration for Provider and Function packages.
		if pwr, ok := pkg.(v1.PackageWithRuntime); ok {
			if prwr, ok := rev.(v1.PackageRevisionWithRuntime); ok {
				prwr.SetRuntimeConfigRef(pwr.GetRuntimeConfigRef())
				prwr.SetTLSServerSecretName(pwr.GetTLSServerSecretName())
				prwr.SetTLSClientSecretName(pwr.GetTLSClientSecretName())
			}
		}

		// Only activate if not already active and the package has automatic
		// activation policy. Manual activation policy means the user must
		// explicitly activate revisions.
		if rev.GetDesiredState() != v1.PackageRevisionActive && ptr.Deref(pkg.GetActivationPolicy(), v1.AutomaticActivation) == v1.AutomaticActivation {
			rev.SetDesiredState(v1.PackageRevisionActive)
		}

		return nil
	}); err != nil {
		return errors.Wrap(err, "cannot create or update package revision")
	}

	// Garbage collect old revisions if we exceed the history limit. The
	// default limit is 1, meaning only the current revision is kept. Setting
	// to 0 disables garbage collection. We check len > limit+1 because the
	// list includes the revision we just created/updated.
	limit := ptr.Deref(pkg.GetRevisionHistoryLimit(), 1)
	if limit == 0 || len(revs) <= int(limit)+1 {
		return nil
	}

	slices.SortFunc(revs, func(a, b v1.PackageRevision) int {
		return cmp.Compare(a.GetRevision(), b.GetRevision())
	})

	err = resource.IgnoreNotFound(i.kube.Delete(ctx, revs[0]))
	return errors.Wrapf(err, "cannot garbage collect revision %s", revs[0].GetName())
}

// InstallObjects installs the package's objects (CRDs, XRDs, Compositions,
// webhooks, etc.) by establishing control of them in the API server.
func (i *PackageInstaller) InstallObjects(ctx context.Context, tx *v1alpha1.Transaction, xp *xpkg.Package) error {
	_, rev, err := NewPackageAndRevision(xp)
	if err != nil {
		return err
	}

	// Label all objs with the transaction for traceability.
	objs := xp.GetObjects()
	for _, obj := range objs {
		if mo, ok := obj.(metav1.Object); ok {
			meta.AddLabels(mo, map[string]string{
				labelTransaction: tx.GetName(),
			})
		}
	}

	// Establish control of all objects in the package. The Establisher handles
	// CRDs, webhooks, and other Kubernetes objects.
	_, err = i.objects.Establish(ctx, objs, rev, true)
	return errors.Wrap(err, "cannot establish control of package objects")
}

// NewPackageAndRevision creates template Package and PackageRevision resources
// with name and source pre-filled based on the xpkg.Package metadata.
func NewPackageAndRevision(xp *xpkg.Package) (v1.Package, v1.PackageRevision, error) {
	var pkg v1.Package
	var rev v1.PackageRevision

	// Packages need TypeMeta because we call GetObjectKind below.
	switch xp.GetMeta().(type) {
	case *pkgmetav1.Provider:
		pkg = &v1.Provider{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1.SchemeGroupVersion.String(),
				Kind:       v1.ProviderKind,
			},
		}
		rev = &v1.ProviderRevision{}
	case *pkgmetav1.Configuration:
		pkg = &v1.Configuration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1.SchemeGroupVersion.String(),
				Kind:       v1.ConfigurationKind,
			},
		}
		rev = &v1.ConfigurationRevision{}
	case *pkgmetav1.Function:
		pkg = &v1.Function{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1.SchemeGroupVersion.String(),
				Kind:       v1.FunctionKind,
			},
		}
		rev = &v1.FunctionRevision{}
	default:
		return nil, nil, errors.Errorf("unknown package type %T", xp.GetMeta())
	}

	// Parse the source to extract repository for naming
	ref, err := name.ParseReference(xp.Source)
	if err != nil {
		return nil, nil, errors.Wrap(err, "cannot parse package source")
	}

	name := xpkg.ToDNSLabel(ref.Context().RepositoryStr())
	pkg.SetName(name)
	rev.SetName(xpkg.FriendlyID(name, xp.Digest))
	meta.AddOwnerReference(rev, meta.AsOwner(meta.TypedReferenceTo(pkg, pkg.GetObjectKind().GroupVersionKind())))

	return pkg, rev, nil
}

// NewPackageAndRevisionList creates empty PackageList and PackageRevisionList
// suitable for listing packages and revisions of the appropriate type.
func NewPackageAndRevisionList(xp *xpkg.Package) (v1.PackageList, v1.PackageRevisionList, error) {
	var pkgList v1.PackageList
	var revList v1.PackageRevisionList

	switch xp.GetMeta().(type) {
	case *pkgmetav1.Provider:
		pkgList = &v1.ProviderList{}
		revList = &v1.ProviderRevisionList{}
	case *pkgmetav1.Configuration:
		pkgList = &v1.ConfigurationList{}
		revList = &v1.ConfigurationRevisionList{}
	case *pkgmetav1.Function:
		pkgList = &v1.FunctionList{}
		revList = &v1.FunctionRevisionList{}
	default:
		return nil, nil, errors.Errorf("unknown package type %T", xp.GetMeta())
	}

	return pkgList, revList, nil
}
