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
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/internal/dag"
)

// TODO(negz): Do InstalledPackage and Dependency need to be separate types?
// They're almost the same structs.

// AsDAGNodes converts a user-facing LockPackage type to its internal DAG type.
func AsDAGNodes(pkgs ...v1beta1.LockPackage) []dag.Node {
	nodes := make([]dag.Node, 0, len(pkgs))
	for _, pkg := range pkgs {
		deps := make([]Dependency, len(pkg.Dependencies))
		for i := range pkg.Dependencies {
			deps[i] = Dependency{
				Source:      pkg.Dependencies[i].Package,
				Constraints: pkg.Dependencies[i].Constraints,
				Type:        DependencyType(pkg.Dependencies[i].Type),
			}
		}

		nodes = append(nodes, &InstalledPackage{
			Source:       pkg.Source,
			Version:      pkg.Version,
			Dependencies: deps,
		})

		// When a package replaces another package, we add them as a 'virtual'
		// installed package in the DAG.
		for _, source := range pkg.Replaces {
			nodes = append(nodes, &InstalledPackage{
				Source:       source,
				Version:      pkg.Version,
				Dependencies: deps,
			})
		}
	}
	return nodes
}

// InstalledPackage is an installed Crossplane package.
type InstalledPackage struct {
	// Source is the OCI image name without a tag or digest.
	Source string

	// Version is the tag or digest of the OCI image.
	Version string

	// Dependencies are the list of dependencies of this package. The order of
	// the dependencies will dictate the order in which they are resolved.
	Dependencies []Dependency

	// ParentConstraints is a list of constraints that are passed down from
	// packages that depend on this one.
	ParentConstraints []string
}

// Identifier returns the source of an InstalledPackage.
func (p *InstalledPackage) Identifier() string {
	return p.Source
}

// GetConstraints returns the version of an InstalledPackage.
func (p *InstalledPackage) GetConstraints() string {
	return p.Version
}

// GetParentConstraints returns the parent constraints of an InstalledPackage.
func (p *InstalledPackage) GetParentConstraints() []string {
	return p.ParentConstraints
}

// AddParentConstraints appends passed constraints to the existing parent constraints.
func (p *InstalledPackage) AddParentConstraints(pc []string) {
	p.ParentConstraints = append(p.ParentConstraints, pc...)
}

// Neighbors returns dependencies of an InstalledPackage.
func (p *InstalledPackage) Neighbors() []dag.Node {
	nodes := make([]dag.Node, len(p.Dependencies))
	for i, r := range p.Dependencies {
		nodes[i] = &r
	}
	return nodes
}

// AddNeighbors adds parent constraints to dependencies of an InstalledPackage.
// When passed a package that the InstalledPackage depends on, it adds the
// InstalledPackage's constraints to it as parent constraints.
//
// An InstalledPackage should always have all dependencies declared before being
// added to the DAG, so AddNeighbors doesn't actually add any neighbors.
func (p *InstalledPackage) AddNeighbors(nodes ...dag.Node) error {
	for _, n := range nodes {
		for _, dep := range p.Dependencies {
			if dep.Identifier() == n.Identifier() {
				n.AddParentConstraints([]string{dep.Constraints})
				break
			}
		}
	}
	return nil
}

// A DependencyType is a type of package.
type DependencyType string

// Types of packages.
const (
	DependencyTypeConfiguration DependencyType = "Configuration"
	DependencyTypeProvider      DependencyType = "Provider"
	DependencyTypeFunction      DependencyType = "Function"
)

// A Dependency is a dependency of a package in the lock.
type Dependency struct {
	// Source is the OCI image name without a tag or digest.
	Source string

	// Constraints is a valid semver range or a digest, which will be used to
	// select a valid dependency version.
	Constraints string

	// ParentConstraints is a list of constraints that are passed down from the
	// parent package to the dependency.
	ParentConstraints []string

	// TODO(negz): Type is an odd field. It's the only field that's not used to
	// figure out what dependencies to install. Instead, it's used once they're
	// solved to determine what type of package to create to satisfy the
	// dependency. It's the only field that doesn't satisfy a DAG interface.
	// Perhaps we shouldn't include it here? We could instead look it up in the
	// Lock dependencies by its source.

	// The type of this dependency.
	Type DependencyType
}

// Identifier returns a dependency's source.
func (d *Dependency) Identifier() string {
	return d.Source
}

// GetConstraints returns a dependency's constrain.
func (d *Dependency) GetConstraints() string {
	return d.Constraints
}

// GetParentConstraints returns a dependency's parent constraints.
func (d *Dependency) GetParentConstraints() []string {
	return d.ParentConstraints
}

// AddParentConstraints appends passed constraints to the existing parent constraints.
func (d *Dependency) AddParentConstraints(pc []string) {
	d.ParentConstraints = append(d.ParentConstraints, pc...)
}

// Neighbors in is a no-op for dependencies because we are not yet aware of its
// dependencies.
func (d *Dependency) Neighbors() []dag.Node {
	return nil
}

// AddNeighbors adds parent constraints to a dependency in the DAG.
func (d *Dependency) AddNeighbors(nodes ...dag.Node) error {
	for _, n := range nodes {
		n.AddParentConstraints([]string{d.Constraints})
	}
	return nil
}
