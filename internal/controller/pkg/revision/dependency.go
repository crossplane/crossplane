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

package revision

import (
	"context"

	"github.com/Masterminds/semver"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
	"github.com/crossplane/crossplane/internal/dag"
	"github.com/crossplane/crossplane/internal/xpkg"
)

const (
	lockName = "lock"

	errNotMeta                   = "meta type is not a valid package"
	errGetOrCreateLock           = "cannot get or create lock"
	errIncompatibleDependencyFmt = "incompatible dependencies: %+v"
	errMissingDependenciesFmt    = "missing dependencies: %+v"
	errDependencyNotInGraph      = "dependency is not present in graph"
	errDependencyNotLockPackage  = "dependency in graph is not a lock package"
)

// DependencyManager is a lock on packages.
type DependencyManager interface {
	Resolve(ctx context.Context, pkg runtime.Object, pr v1.PackageRevision) (found, installed, invalid int, err error)
	RemoveSelf(ctx context.Context, pr v1.PackageRevision) error
}

// PackageDependencyManager is a resolver for packages.
type PackageDependencyManager struct {
	client      client.Client
	newDag      dag.NewDAGFn
	packageType v1alpha1.PackageType
}

// NewPackageDependencyManager creates a new PackageDependencyManager.
func NewPackageDependencyManager(c client.Client, nd dag.NewDAGFn, t v1alpha1.PackageType) *PackageDependencyManager {
	return &PackageDependencyManager{
		client:      c,
		newDag:      nd,
		packageType: t,
	}
}

// Resolve resolves package dependencies.
func (m *PackageDependencyManager) Resolve(ctx context.Context, pkg runtime.Object, pr v1.PackageRevision) (found, installed, invalid int, err error) { // nolint:gocyclo
	pack, ok := xpkg.TryConvertToPkg(pkg, &pkgmetav1.Provider{}, &pkgmetav1.Configuration{})
	if !ok {
		return found, installed, invalid, errors.New(errNotMeta)
	}

	// Copy package dependencies into Lock Dependencies.
	sources := make([]v1alpha1.Dependency, len(pack.GetDependencies()))
	for i, dep := range pack.GetDependencies() {
		pdep := v1alpha1.Dependency{}
		if dep.Configuration != nil {
			pdep.Package = *dep.Configuration
			pdep.Type = v1alpha1.ConfigurationPackageType
		} else if dep.Provider != nil {
			pdep.Package = *dep.Provider
			pdep.Type = v1alpha1.ProviderPackageType
		}
		pdep.Constraints = dep.Version
		sources[i] = pdep
	}

	found = len(sources)

	// Get the lock.
	lock := &v1alpha1.Lock{}
	err = m.client.Get(ctx, types.NamespacedName{Name: lockName}, lock)
	if kerrors.IsNotFound(err) {
		// If lock does not exist and we are inactive then we can return early
		// because our only operation would be to remove self.
		if pr.GetDesiredState() == v1.PackageRevisionInactive {
			return found, installed, invalid, nil
		}
		lock.Name = lockName
		err = m.client.Create(ctx, lock, &client.CreateOptions{})
	}
	if err != nil {
		return found, installed, invalid, errors.Wrap(err, errGetOrCreateLock)
	}

	prRef, err := name.ParseReference(pr.GetSource(), name.WithDefaultRegistry(""))
	if err != nil {
		return found, installed, invalid, err
	}

	lockRef := xpkg.ParsePackageSourceFromReference(prRef)
	selfIndex := intPointer(-1)
	d := m.newDag()
	implied, err := d.Init(v1alpha1.ToNodes(lock.Packages...), dag.FindIndex(lockRef, selfIndex))
	if err != nil {
		return found, installed, invalid, err
	}

	// If we are inactive, all we want to do is remove self.
	if pr.GetDesiredState() == v1.PackageRevisionInactive {
		if *selfIndex >= 0 {
			lock.Packages = append(lock.Packages[:*selfIndex], lock.Packages[*selfIndex+1:]...)
			return found, installed, invalid, m.client.Update(ctx, lock)
		}
		return found, installed, invalid, nil
	}

	// NOTE(hasheddan): consider adding health of package to lock so that it can
	// be rolled up to any dependent packages.
	self := v1alpha1.LockPackage{
		Name:         pr.GetName(),
		Type:         m.packageType,
		Source:       lockRef,
		Version:      prRef.Identifier(),
		Dependencies: sources,
	}

	// If we don't exist in lock then we should add self.
	if *selfIndex == -1 {
		lock.Packages = append(lock.Packages, self)
		if err := m.client.Update(ctx, lock); err != nil {
			return found, installed, invalid, err
		}
		// Package may exist in the graph as a dependency, or may not exist at
		// all. We need to either convert it to a full node or add it.
		d.AddOrUpdateNodes(&self)

		// If any direct dependencies are missing we skip checking for
		// transitive ones.
		var missing []string
		for _, dep := range self.Dependencies {
			if d.NodeExists(dep.Identifier()) {
				installed++
				continue
			}
			missing = append(missing, dep.Identifier())
		}
		if installed != found {
			return found, installed, invalid, errors.Errorf(errMissingDependenciesFmt, missing)
		}
	}

	tree, err := d.TraceNode(lockRef)
	if err != nil {
		return found, installed, invalid, err
	}
	found = len(tree)
	installed = found
	// Check if any dependencies or transitive dependencies are missing (implied).
	var missing []string
	for _, imp := range implied {
		if _, ok := tree[imp.Identifier()]; ok {
			installed--
			missing = append(missing, imp.Identifier())
		}
	}
	if len(missing) != 0 {
		return found, installed, invalid, errors.Errorf(errMissingDependenciesFmt, missing)
	}

	// All of our dependencies and transitive dependencies must exist. Check
	// that neighbors have valid versions.
	var invalidDeps []string
	for _, dep := range self.Dependencies {
		n, err := d.GetNode(dep.Package)
		if err != nil {
			return found, installed, invalid, errors.New(errDependencyNotInGraph)
		}
		lp, ok := n.(*v1alpha1.LockPackage)
		if !ok {
			return found, installed, invalid, errors.New(errDependencyNotLockPackage)
		}
		c, err := semver.NewConstraint(dep.Constraints)
		if err != nil {
			return found, installed, invalid, err
		}
		v, err := semver.NewVersion(lp.Version)
		if err != nil {
			return found, installed, invalid, err
		}
		if !c.Check(v) {
			invalidDeps = append(invalidDeps, lp.Identifier())
		}
	}
	invalid = len(invalidDeps)
	if invalid > 0 {
		return found, installed, invalid, errors.Errorf(errIncompatibleDependencyFmt, invalidDeps)
	}
	return found, installed, invalid, nil
}

// RemoveSelf removes a package from the lock.
func (m *PackageDependencyManager) RemoveSelf(ctx context.Context, pr v1.PackageRevision) error {
	prRef, err := name.ParseReference(pr.GetSource(), name.WithDefaultRegistry(""))
	if err != nil {
		return err
	}

	// Get the lock.
	lock := &v1alpha1.Lock{}
	err = m.client.Get(ctx, types.NamespacedName{Name: lockName}, lock)
	if kerrors.IsNotFound(err) {
		// If lock does not exist then we don't need to remove self.
		return nil
	}
	if err != nil {
		return err
	}

	// Find self and remove. If we don't exist, its a no-op.
	lockRef := xpkg.ParsePackageSourceFromReference(prRef)
	for i, lp := range lock.Packages {
		if lp.Source == lockRef {
			lock.Packages = append(lock.Packages[:i], lock.Packages[i+1:]...)
			return m.client.Update(ctx, lock)
		}
	}
	return nil
}

func intPointer(i int) *int {
	return &i
}
