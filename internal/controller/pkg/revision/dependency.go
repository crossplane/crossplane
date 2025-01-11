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
	"fmt"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/google/go-containerregistry/pkg/name"
	conregv1 "github.com/google/go-containerregistry/pkg/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"

	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/internal/dag"
	"github.com/crossplane/crossplane/internal/xpkg"
)

const (
	lockName = "lock"

	errNotMeta                   = "meta type is not a valid package"
	errGetOrCreateLock           = "cannot get or create lock"
	errInitDAG                   = "cannot initialize dependency graph from the packages in the lock"
	errFmtIncompatibleDependency = "incompatible dependencies: %s"
	errFmtMissingDependencies    = "missing dependencies: %+v"
	errDependencyNotInGraph      = "dependency is not present in graph"
	errDependencyNotLockPackage  = "dependency in graph is not a lock package"
)

// DependencyManager is a lock on packages.
type DependencyManager interface {
	Resolve(ctx context.Context, meta pkgmetav1.Pkg, pr v1.PackageRevision) (found, installed, invalid int, err error)
	RemoveSelf(ctx context.Context, pr v1.PackageRevision) error
}

// PackageDependencyManager is a resolver for packages.
type PackageDependencyManager struct {
	client      client.Client
	newDag      dag.NewDAGFn
	packageType schema.GroupVersionKind
}

// NewPackageDependencyManager creates a new PackageDependencyManager.
func NewPackageDependencyManager(c client.Client, nd dag.NewDAGFn, pkgType schema.GroupVersionKind) *PackageDependencyManager {
	return &PackageDependencyManager{
		client:      c,
		newDag:      nd,
		packageType: pkgType,
	}
}

// Resolve resolves package dependencies.
func (m *PackageDependencyManager) Resolve(ctx context.Context, meta pkgmetav1.Pkg, pr v1.PackageRevision) (found, installed, invalid int, err error) { //nolint:gocognit // TODO(negz): Can this be refactored for less complexity?
	// If we are inactive, we don't need to resolve dependencies.
	if pr.GetDesiredState() == v1.PackageRevisionInactive {
		return 0, 0, 0, nil
	}

	// Copy package dependencies into Lock Dependencies.
	sources := make([]v1beta1.Dependency, len(meta.GetDependencies()))
	for i, dep := range meta.GetDependencies() {
		pdep := v1beta1.Dependency{}
		switch {
		// If the GVK and package are specified explicitly they take precedence.
		case dep.APIVersion != nil && dep.Kind != nil && dep.Package != nil:
			pdep.APIVersion = dep.APIVersion
			pdep.Kind = dep.Kind
			pdep.Package = *dep.Package
		case dep.Configuration != nil:
			pdep.Package = *dep.Configuration
			pdep.Type = ptr.To(v1beta1.ConfigurationPackageType)
		case dep.Provider != nil:
			pdep.Package = *dep.Provider
			pdep.Type = ptr.To(v1beta1.ProviderPackageType)
		case dep.Function != nil:
			pdep.Package = *dep.Function
			pdep.Type = ptr.To(v1beta1.FunctionPackageType)
		default:
			return 0, 0, 0, errors.Errorf("encountered an invalid dependency: package dependencies must specify either a valid type, or an explicit apiVersion, kind, and package")
		}
		pdep.Constraints = dep.Version
		sources[i] = pdep
	}

	found = len(sources)

	// Get the lock.
	lock := &v1beta1.Lock{}
	err = m.client.Get(ctx, types.NamespacedName{Name: lockName}, lock)
	if kerrors.IsNotFound(err) {
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

	d := m.newDag()
	implied, err := d.Init(v1beta1.ToNodes(lock.Packages...))
	if err != nil {
		return found, installed, invalid, errors.Wrap(err, errInitDAG)
	}

	lockRef := xpkg.ParsePackageSourceFromReference(prRef)
	// NOTE(hasheddan): consider adding health of package to lock so that it can
	// be rolled up to any dependent packages.
	self := v1beta1.LockPackage{
		APIVersion:   ptr.To(m.packageType.GroupVersion().String()),
		Kind:         ptr.To(m.packageType.Kind),
		Name:         pr.GetName(),
		Source:       lockRef,
		Version:      prRef.Identifier(),
		Dependencies: sources,
	}

	// Delete packages in lock with same name and distinct source
	// This is a corner case when source is updated but image SHA is not (i.e. relocate same image
	// to another registry)
	for _, lp := range lock.Packages {
		if self.Name == lp.Name && self.Type == lp.Type && self.Source != lp.Identifier() {
			if err := m.RemoveSelf(ctx, pr); err != nil {
				return found, installed, invalid, err
			}
			// refresh the lock to be in sync with the contents
			if err = m.client.Get(ctx, types.NamespacedName{Name: lockName}, lock); err != nil {
				return found, installed, invalid, err
			}
			break
		}
	}

	prExists := false
	for _, lp := range lock.Packages {
		if lp.Name == pr.GetName() {
			prExists = true
			break
		}
	}

	// If we don't exist in lock then we should add self.
	if !prExists {
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
			return found, installed, invalid, errors.Errorf(errFmtMissingDependencies, missing)
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
		return found, installed, invalid, errors.Errorf(errFmtMissingDependencies, missing)
	}

	// All of our dependencies and transitive dependencies must exist. Check
	// that neighbors have valid versions.
	var invalidDeps []string
	for _, dep := range self.Dependencies {
		n, err := d.GetNode(dep.Package)
		if err != nil {
			return found, installed, invalid, errors.New(errDependencyNotInGraph)
		}
		lp, ok := n.(*v1beta1.LockPackage)
		if !ok {
			return found, installed, invalid, errors.New(errDependencyNotLockPackage)
		}

		// Check if the constraint is a digest, if so, compare it directly.
		if d, err := conregv1.NewHash(dep.Constraints); err == nil {
			if lp.Version != d.String() {
				return found, installed, invalid, errors.Errorf("existing package %s@%s is incompatible with constraint %s", lp.Identifier(), lp.Version, strings.TrimSpace(dep.Constraints))
			}
			continue
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
			s := fmt.Sprintf("existing package %s@%s", lp.Identifier(), lp.Version)
			if dep.Constraints != "" {
				s = fmt.Sprintf("%s is incompatible with constraint %s", s, strings.TrimSpace(dep.Constraints))
			}
			invalidDeps = append(invalidDeps, s)
		}
	}
	invalid = len(invalidDeps)
	if invalid > 0 {
		return found, installed, invalid, errors.Errorf(errFmtIncompatibleDependency, strings.Join(invalidDeps, "; "))
	}
	return found, installed, invalid, nil
}

// RemoveSelf removes a package from the lock.
func (m *PackageDependencyManager) RemoveSelf(ctx context.Context, pr v1.PackageRevision) error {
	// Get the lock.
	lock := &v1beta1.Lock{}
	err := m.client.Get(ctx, types.NamespacedName{Name: lockName}, lock)
	if kerrors.IsNotFound(err) {
		// If lock does not exist then we don't need to remove self.
		return nil
	}
	if err != nil {
		return err
	}

	// Find self and remove. If we don't exist, its a no-op.
	for i, lp := range lock.Packages {
		if lp.Name == pr.GetName() {
			lock.Packages = append(lock.Packages[:i], lock.Packages[i+1:]...)
			return m.client.Update(ctx, lock)
		}
	}
	return nil
}
