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

// Package dependency implements dependency resolution for Crossplane packages.
package dependency

import (
	"context"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/google/go-containerregistry/pkg/name"
	conregv1 "github.com/google/go-containerregistry/pkg/v1"
	"k8s.io/utils/ptr"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	pkgmetav1 "github.com/crossplane/crossplane/v2/apis/pkg/meta/v1"
	v1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"
	"github.com/crossplane/crossplane/v2/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/v2/internal/xpkg"
)

// TighteningConstraintSolver resolves package dependencies using iterative
// constraint tightening. It traverses the dependency graph, selecting minimum
// versions that satisfy accumulated constraints, then prunes unreachable
// packages.
type TighteningConstraintSolver struct {
	client xpkg.Client
}

// NewTighteningConstraintSolver creates a new TighteningConstraintSolver.
func NewTighteningConstraintSolver(c xpkg.Client) *TighteningConstraintSolver {
	return &TighteningConstraintSolver{
		client: c,
	}
}

// Package represents a package in various stages of resolution. It embeds
// LockPackage to hold the final Lock state, plus additional fields needed
// during resolution.
type Package struct {
	v1beta1.LockPackage

	// Digest is the OCI image digest (e.g., "sha256:abc123...") returned by the
	// registry during resolution. Used for computing Name.
	Digest string

	// Constraints are the accumulated version constraints for this package
	// from all dependents.
	Constraints []string
}

// NewPackage creates a solver Package from a fetched xpkg.Package.
func NewPackage(xp *xpkg.Package, version string, constraints []string) (*Package, error) {
	pkg := &Package{
		LockPackage: v1beta1.LockPackage{
			Name:         xpkg.FriendlyID(xp.Source, xp.DigestHex()),
			Source:       xp.Source,
			Version:      version,
			Dependencies: ConvertDependencies(xp.GetDependencies()),
		},
		Digest:      xp.Digest,
		Constraints: constraints,
	}

	// Extract type information from package metadata and map to pkg.crossplane.io/v1.
	// The metadata may be from meta.pkg.crossplane.io/v1alpha1, v1beta1, or v1,
	// but we always store Package types (Provider, Configuration, Function) not
	// Revision types in the Lock.
	if meta := xp.GetMeta(); meta != nil {
		gvk := meta.GetObjectKind().GroupVersionKind()

		// Map metadata package type to pkg.crossplane.io/v1 Package type.
		// Keep the Kind (Provider, Configuration, Function) from metadata.
		pkg.APIVersion = ptr.To(v1.SchemeGroupVersion.String())
		pkg.Kind = ptr.To(gvk.Kind)
	}

	return pkg, nil
}

// Packages is a map of package sources to their resolution state.
type Packages map[string]*Package

// NewPackagesFrom creates a Packages map initialized from current Lock state.
func NewPackagesFrom(current []v1beta1.LockPackage) Packages {
	p := make(Packages, len(current))
	for _, pkg := range current {
		p[pkg.Source] = &Package{LockPackage: pkg}
	}
	return p
}

// AddConstraints adds version constraints for a package source.
func (p Packages) AddConstraints(source string, constraints ...string) {
	pkg, exists := p[source]
	if !exists {
		p[source] = &Package{Constraints: constraints}
		return
	}
	pkg.Constraints = append(pkg.Constraints, constraints...)
}

// UnsatisfiedConstraints returns the constraints that need to be satisfied for
// a package. Returns empty slice if package is already resolved and satisfies
// all constraints. Returns wildcard constraint if package has never been seen.
// Returns error if constraints conflict with current package state (e.g.,
// semver constraints on digest-pinned package).
func (p Packages) UnsatisfiedConstraints(source string) ([]string, error) {
	pkg, exists := p[source]
	if !exists {
		// Never seen this package - resolve with wildcard.
		return []string{"*"}, nil
	}

	// No constraints at all - handle based on whether resolved.
	if len(pkg.Constraints) == 0 {
		if pkg.Version == "" {
			// No version, no constraints - resolve with wildcard.
			return []string{"*"}, nil
		}
		// Has version, no constraints - already satisfied.
		return nil, nil
	}

	// Has constraints - separate by type (digest vs semver) and validate.
	digests := []string{}
	semvers := make([]string, 0, len(pkg.Constraints)) // Most likely all semvers.
	for _, c := range pkg.Constraints {
		if _, err := conregv1.NewHash(c); err == nil {
			digests = append(digests, c)
			continue
		}
		semvers = append(semvers, c)
	}

	// Cannot mix digest and semver constraints.
	if len(digests) > 0 && len(semvers) > 0 {
		return nil, errors.Errorf("package %s has conflicting constraint types: cannot mix digest constraints %v with semver constraints %v", source, digests, semvers)
	}

	// All constraints are digests.
	if len(digests) > 0 {
		// Verify all digest constraints are identical.
		first := digests[0]
		for _, d := range digests[1:] {
			if d != first {
				return nil, errors.Errorf("package %s has conflicting digest constraints: %v", source, digests)
			}
		}

		// Not yet resolved - need to resolve to this digest.
		if pkg.Version == "" {
			return []string{first}, nil
		}

		// Already resolved to required digest - satisfied.
		if pkg.Version == first {
			return nil, nil
		}

		// Resolved to different digest - need to re-resolve.
		return []string{first}, nil
	}

	// All constraints are semver.
	// Not yet resolved - return semver constraints.
	if pkg.Version == "" {
		return semvers, nil
	}

	// Resolved, but version is a digest (can't evaluate semver against digest).
	if _, err := conregv1.NewHash(pkg.Version); err == nil {
		return nil, errors.Errorf("package %s is pinned to digest %s but has semver constraints %v; cannot evaluate semver constraints against digest-pinned packages", source, pkg.Version, semvers)
	}

	// Happy path: Resolved to semver tag, all constraints are semver - check satisfaction.
	combined, err := semver.NewConstraint(strings.Join(semvers, ", "))
	if err != nil {
		return nil, errors.Wrapf(err, "invalid semver constraints %v for package %s", semvers, source)
	}

	current, err := semver.NewVersion(pkg.Version)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid semver version %s for package %s", pkg.Version, source)
	}

	// Current version satisfies constraints - already satisfied.
	if combined.Check(current) {
		return nil, nil
	}

	// Current version doesn't satisfy - need to re-resolve.
	return semvers, nil
}

// ValidateVersion checks if a specific version of a package can be installed
// given existing constraints from other packages. Returns error if the version
// conflicts with any existing package's dependency constraints.
func (p Packages) ValidateVersion(source string, version string) error {
	for src, pkg := range p {
		for _, dep := range pkg.Dependencies {
			if dep.Package != source || dep.Constraints == "" {
				continue
			}

			// Check if version satisfies this existing constraint.
			if _, err := conregv1.NewHash(version); err == nil {
				// Version is a digest - can only satisfy digest constraints.
				if version != dep.Constraints {
					return errors.Errorf("cannot install %s at digest %s: package %s requires %s", source, version, src, dep.Constraints)
				}
				continue
			}

			// Version is a tag - check semver constraints.
			constraint, err := semver.NewConstraint(dep.Constraints)
			if err != nil {
				return errors.Wrapf(err, "invalid constraint %s from package %s", dep.Constraints, src)
			}
			sv, err := semver.NewVersion(version)
			if err != nil {
				return errors.Wrapf(err, "cannot parse version %s as semver", version)
			}

			if !constraint.Check(sv) {
				return errors.Errorf("cannot install %s %s: package %s requires %s %s", source, version, src, source, dep.Constraints)
			}
		}
	}
	return nil
}

// FindReachable performs a depth-first search from the root package to find
// all reachable packages and returns them as a filtered Packages map.
func (p Packages) FindReachable(root string) Packages {
	seen := make(map[string]bool)
	reachable := make(Packages)

	var visit func(string)
	visit = func(src string) {
		if seen[src] {
			return
		}
		seen[src] = true

		pkg, exists := p[src]
		if !exists {
			return
		}

		// Add package to reachable set.
		reachable[src] = pkg

		for _, dep := range pkg.Dependencies {
			visit(dep.Package)
		}
	}

	visit(root)
	return reachable
}

// ToLockPackages converts the resolution state to LockPackages for storage.
// It prunes packages orphaned by constraint tightening during resolution, and
// preserves packages from other dependency trees (other roots in the Lock).
func (p Packages) ToLockPackages(root string, current []v1beta1.LockPackage) []v1beta1.LockPackage {
	// Find packages reachable from root. This excludes:
	// 1. Packages added during resolution but later orphaned when constraints
	//    tightened (e.g., provider-c v1.1 depended on provider-x, but after
	//    re-resolving to v2.0 it no longer does - provider-x gets dropped).
	// 2. Packages from other roots (e.g., provider-b when resolving
	//    provider-a).
	reachable := p.FindReachable(root)

	result := make([]v1beta1.LockPackage, 0, len(reachable))

	// Add reachable packages from this resolution.
	for _, pkg := range reachable {
		result = append(result, pkg.LockPackage)
	}

	// Preserve packages from other roots. We add them back because they're
	// separate installed packages we didn't touch. We check current (not
	// the Packages map) so orphaned packages that were never installed get
	// dropped, while installed packages get preserved.
	for _, lp := range current {
		if _, exists := reachable[lp.Source]; !exists {
			result = append(result, lp)
		}
	}

	return result
}

// Solve resolves all dependencies starting from the given root package
// reference. The root package is installed at the exact version specified in
// the reference, while dependencies are resolved using iterative constraint
// tightening to find minimum versions that satisfy all accumulated constraints.
//
// The ref parameter must be a complete OCI reference including version or
// digest (e.g., "registry.io/org/package:v1.0.0" or
// "registry.io/org/package@sha256:...").
func (s *TighteningConstraintSolver) Solve(ctx context.Context, ref string, current []v1beta1.LockPackage) ([]v1beta1.LockPackage, error) {
	// Parse root reference to extract source and version.
	r, err := name.ParseReference(ref)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot parse package reference %s", ref)
	}

	source := xpkg.ParsePackageSourceFromReference(r)
	version := r.Identifier()

	// Initialize packages from current Lock.
	packages := NewPackagesFrom(current)

	// Validate root version against existing constraints.
	if err := packages.ValidateVersion(source, version); err != nil {
		return nil, err
	}

	// Fetch root package to discover its dependencies.
	xpkg, err := s.client.Get(ctx, ref)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot fetch root package %s", ref)
	}

	// Add root as already resolved (user's explicit choice, no resolution needed).
	rootPkg, err := NewPackage(xpkg, version, nil)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create root package")
	}
	packages[source] = rootPkg

	queue := make([]string, 0, len(rootPkg.Dependencies))

	// Queue root's dependencies for resolution.
	for _, dep := range rootPkg.Dependencies {
		if dep.Constraints != "" {
			packages.AddConstraints(dep.Package, dep.Constraints)
		}
		queue = append(queue, dep.Package)
	}

	// Process dependency queue using iterative constraint tightening.
	for len(queue) > 0 {
		src := queue[0]
		queue = queue[1:]

		// Check what constraints need to be satisfied.
		constraints, err := packages.UnsatisfiedConstraints(src)
		if err != nil {
			return nil, err
		}
		if len(constraints) == 0 {
			// All constraints satisfied, skip.
			continue
		}

		// Select version and fetch to discover dependencies.
		resolvedPkg, err := s.SelectVersion(ctx, src, constraints)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot resolve package %s", src)
		}

		// Store resolved package.
		packages[src] = resolvedPkg

		// Add dependencies to queue with their constraints.
		for _, dep := range resolvedPkg.Dependencies {
			if dep.Constraints != "" {
				packages.AddConstraints(dep.Package, dep.Constraints)
			}
			queue = append(queue, dep.Package)
		}
	}

	return packages.ToLockPackages(source, current), nil
}

// SelectVersion selects the minimum (oldest) version satisfying all
// constraints and fetches it to discover dependencies.
//
// For digest constraints, fetches the exact digest. For semver constraints,
// implements the "minimum version preference" strategy: when multiple versions
// satisfy the constraints, the oldest is chosen to minimize upgrade risk.
//
// Returns a fully populated Package ready to be stored in the Packages map.
func (s *TighteningConstraintSolver) SelectVersion(ctx context.Context, source string, constraints []string) (*Package, error) {
	// Separate constraints by type and validate.
	digests := []string{}
	semvers := make([]string, 0, len(constraints)) // Most likely all semvers.
	for _, c := range constraints {
		if _, err := conregv1.NewHash(c); err == nil {
			digests = append(digests, c)
			continue
		}
		semvers = append(semvers, c)
	}

	// Cannot mix digest and semver constraints.
	if len(digests) > 0 && len(semvers) > 0 {
		return nil, errors.Errorf("cannot mix digest constraints %v with semver constraints %v", digests, semvers)
	}

	// Handle digest constraints.
	if len(digests) > 0 {
		// Verify all digest constraints are identical.
		first := digests[0]
		for _, d := range digests[1:] {
			if d != first {
				return nil, errors.Errorf("conflicting digest constraints: %v", digests)
			}
		}

		// Fetch the digest.
		ref := source + "@" + first
		pkg, err := s.client.Get(ctx, ref)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot fetch %s", ref)
		}
		return NewPackage(pkg, first, constraints)
	}

	// Handle semver constraints.
	combined, err := semver.NewConstraint(strings.Join(semvers, ", "))
	if err != nil {
		return nil, errors.Wrapf(err, "invalid constraints %v", semvers)
	}

	// List available versions.
	versions, err := s.client.ListVersions(ctx, source)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot list versions for %s", source)
	}

	// Find minimum (oldest) version satisfying constraints.
	for _, v := range versions {
		sv, err := semver.NewVersion(v)
		if err != nil {
			continue // Skip invalid semver.
		}

		if !combined.Check(sv) {
			continue // Doesn't satisfy constraints.
		}

		// Fetch this version to get dependencies.
		ref := source + ":" + v
		pkg, err := s.client.Get(ctx, ref)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot fetch %s", ref)
		}

		return NewPackage(pkg, v, constraints)
	}

	return nil, errors.Errorf("no version of %s satisfies constraints %v", source, semvers)
}

// ConvertDependencies converts package metadata dependencies to Lock
// dependencies.
//
// This handles the conversion from the package.yaml format (which uses
// different fields for different package types) to the unified Lock format.
func ConvertDependencies(deps []pkgmetav1.Dependency) []v1beta1.Dependency {
	result := make([]v1beta1.Dependency, 0, len(deps))

	for _, dep := range deps {
		lockDep := v1beta1.Dependency{
			Constraints: dep.Version,
		}

		// Handle three dependency formats:
		// 1. Modern: apiVersion + kind + package (explicit)
		// 2. Legacy: type-specific fields (provider/configuration/function)
		// 3. Invalid: skip if none match
		switch {
		case dep.APIVersion != nil && dep.Kind != nil && dep.Package != nil:
			lockDep.APIVersion = dep.APIVersion
			lockDep.Kind = dep.Kind
			lockDep.Package = *dep.Package
		case dep.Configuration != nil:
			lockDep.Package = *dep.Configuration
			t := v1beta1.ConfigurationPackageType
			lockDep.Type = &t
		case dep.Provider != nil:
			lockDep.Package = *dep.Provider
			t := v1beta1.ProviderPackageType
			lockDep.Type = &t
		case dep.Function != nil:
			lockDep.Package = *dep.Function
			t := v1beta1.FunctionPackageType
			lockDep.Type = &t
		default:
			continue
		}

		result = append(result, lockDep)
	}

	return result
}
