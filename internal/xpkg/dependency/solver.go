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

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	pkgmetav1 "github.com/crossplane/crossplane/v2/apis/pkg/meta/v1"
	"github.com/crossplane/crossplane/v2/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/v2/internal/xpkg"
)

// Constraints map package sources to their version constraints. Multiple
// constraints for the same package are ANDed together.
type Constraints map[string][]string

// A Graph maps package sources to their dependencies.
type Graph map[string][]v1beta1.Dependency

// Constraints computes the constraint set from the dependency graph.
// It aggregates all version constraints that apply to each package by
// iterating through all dependencies in the graph.
func (g Graph) Constraints() Constraints {
	constraints := make(Constraints)
	for _, deps := range g {
		for _, dep := range deps {
			constraints[dep.Package] = append(constraints[dep.Package], dep.Constraints)
		}
	}
	return constraints
}

// ResolvedVersion represents a resolved package version.
type ResolvedVersion struct {
	// Tag is the semantic version tag (e.g., "v1.2.3").
	Tag string

	// Digest is the immutable OCI digest (e.g., "sha256:abc123...").
	Digest string
}

// TwoPassSolver implements dependency resolution using a two-pass algorithm:
// Pass 1: Build the dependency graph by fetching and parsing packages
// Pass 2: Select versions using Minimum Version Selection (MVS).
//
// This approach ensures complete dependency graph knowledge before selecting
// versions, enabling optimal version selection and clear error messages when
// constraints cannot be satisfied.
type TwoPassSolver struct {
	client xpkg.Client
}

// NewTwoPassSolver creates a new TwoPassSolver.
func NewTwoPassSolver(c xpkg.Client) *TwoPassSolver {
	return &TwoPassSolver{
		client: c,
	}
}

// Solve resolves all dependencies in two passes.
func (s *TwoPassSolver) Solve(ctx context.Context, source string, current []v1beta1.LockPackage) ([]v1beta1.LockPackage, error) {
	graph, err := s.BuildGraph(ctx, source, current)
	if err != nil {
		return nil, err
	}

	resolved, err := s.SelectVersions(ctx, graph, current)
	if err != nil {
		return nil, err
	}

	return resolved, nil
}

// BuildGraph fetches all packages and builds the dependency graph.
//
// This is Pass 1 of the two-pass algorithm. It recursively fetches and parses
// all packages to build a complete dependency graph. Version constraints are
// derived from the graph on-demand via Graph.Constraints().
func (s *TwoPassSolver) BuildGraph(ctx context.Context, source string, current []v1beta1.LockPackage) (Graph, error) {
	// Initialize graph from current Lock. Packages already in the Lock are
	// marked as visited to avoid re-fetching them.
	graph := make(Graph)
	visited := make(map[string]bool)
	inProgress := make(map[string]bool)

	for _, pkg := range current {
		visited[pkg.Source] = true
		graph[pkg.Source] = pkg.Dependencies
	}

	// Start recursive traversal from the requested package.
	if err := s.build(ctx, source, graph, visited, inProgress); err != nil {
		return nil, err
	}

	return graph, nil
}

// build recursively builds the dependency graph by fetching a
// package and its dependencies. It detects cycles and avoids re-fetching
// packages that are already in the graph.
func (s *TwoPassSolver) build(ctx context.Context, source string, g Graph, visited, inProgress map[string]bool) error {
	// Detect cycles: if we're already processing this package in our call
	// stack, we've found a circular dependency.
	if inProgress[source] {
		return errors.Errorf("circular dependency detected: %s", source)
	}

	// Skip packages we've already processed (either from current Lock or
	// earlier in this traversal).
	if visited[source] {
		return nil
	}

	// Mark as in-progress for cycle detection, remove when done.
	inProgress[source] = true
	defer delete(inProgress, source)

	pkg, err := s.client.Get(ctx, source)
	if err != nil {
		return errors.Wrapf(err, "cannot fetch package %s", source)
	}

	deps := ConvertDependencies(pkg.GetDependencies())
	g[source] = deps
	visited[source] = true

	// Recursively process dependencies.
	for _, dep := range deps {
		if err := s.build(ctx, dep.Package, g, visited, inProgress); err != nil {
			return err
		}
	}

	return nil
}

// SelectVersions selects specific versions for all packages using MVS.
//
// This is Pass 2 of the two-pass algorithm. With the complete dependency graph
// from Pass 1, it computes constraints and selects the minimum (oldest) version
// that satisfies all constraints for each package, following Go's Minimum
// Version Selection strategy for stability and reproducibility.
func (s *TwoPassSolver) SelectVersions(ctx context.Context, g Graph, current []v1beta1.LockPackage) ([]v1beta1.LockPackage, error) {
	// Compute constraints from the dependency graph.
	constraints := g.Constraints()

	result := make([]v1beta1.LockPackage, 0, len(current)+len(constraints))

	// Preserve packages from current Lock that have no new constraints.
	// If a package has no entry in constraints, it means no package
	// in the new dependency tree depends on it, so we keep it unchanged.
	// Packages that DO have constraints (even if they're in current Lock)
	// will be re-resolved below to satisfy the new constraints.
	for _, pkg := range current {
		if _, hasConstraints := constraints[pkg.Source]; !hasConstraints {
			result = append(result, pkg)
		}
	}

	// Resolve versions for packages that need it. These are packages that
	// appear in the new dependency tree (have constraints).
	for source, pkgConstraints := range constraints {
		// Find the minimum version satisfying all accumulated constraints.
		// This is the core of MVS: pick the oldest version that works.
		ver, err := s.FindMinimumCompatibleVersion(ctx, source, pkgConstraints)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot find version of %s satisfying constraints %v", source, pkgConstraints)
		}

		// Store resolved version with its digest (immutable reference) and
		// dependencies from the graph.
		result = append(result, v1beta1.LockPackage{
			Name:         xpkg.FriendlyID(source, ver.Digest),
			Source:       source,
			Version:      ver.Digest,
			Dependencies: g[source],
		})
	}

	return result, nil
}

// FindMinimumCompatibleVersion finds the minimum (oldest) version that
// satisfies all constraints. This implements Minimum Version Selection (MVS)
// for stability and reproducibility.
//
// The algorithm:
// 1. Combines all constraints using AND logic
// 2. Lists all available versions from the registry
// 3. Filters to versions satisfying the combined constraint
// 4. Returns the minimum (oldest) satisfying version
// 5. Resolves the tag to an immutable digest.
func (s *TwoPassSolver) FindMinimumCompatibleVersion(ctx context.Context, source string, constraints []string) (*ResolvedVersion, error) {
	// Combine constraints using AND logic. Empty constraints match any version.
	if len(constraints) == 0 {
		constraints = []string{"*"}
	}
	combined, err := semver.NewConstraint(strings.Join(constraints, ", "))
	if err != nil {
		return nil, err
	}

	versions, err := s.client.ListVersions(ctx, source)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot list versions for %s", source)
	}

	// Find first (minimum) version satisfying constraints. This is MVS:
	// we iterate oldest to newest and return the first match.
	for _, v := range versions {
		sv, err := semver.NewVersion(v)
		if err != nil {
			continue
		}

		// Doesn't satisfy our combined constraints.
		if !combined.Check(sv) {
			continue
		}

		// Fetch the package to get its digest.
		pkg, err := s.client.Get(ctx, source+":"+v)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot fetch %s:%s", source, v)
		}

		return &ResolvedVersion{Tag: v, Digest: pkg.Digest}, nil
	}

	return nil, errors.Errorf("no version of %s satisfies constraints %v", source, constraints)
}

// ConvertDependencies converts package metadata dependencies to Lock dependencies.
//
// This handles the conversion from the package.yaml format (which uses different
// fields for different package types) to the unified Lock format.
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
