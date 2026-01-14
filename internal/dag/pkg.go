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

package dag

import (
	"github.com/crossplane/crossplane/v2/apis/pkg/v1beta1"
)

var (
	_ Node = &DependencyNode{}
	_ Node = &PackageNode{}
)

// DependencyNode is a DAG node representing a package dependency.
type DependencyNode struct {
	v1beta1.Dependency
}

// Neighbors in is a no-op for dependencies because we are not yet aware of its
// dependencies.
func (d *DependencyNode) Neighbors() []Node {
	return nil
}

// AddNeighbors adds parent constraints to a dependency in the DAG.
func (d *DependencyNode) AddNeighbors(nodes ...Node) error {
	for _, n := range nodes {
		n.AddParentConstraints([]string{d.Constraints})
	}

	return nil
}

// PackageNode is a DAG node representing a package.
type PackageNode struct {
	v1beta1.LockPackage
}

// Neighbors returns dependencies of a LockPackage.
func (l *PackageNode) Neighbors() []Node {
	nodes := make([]Node, len(l.Dependencies))
	for i, r := range l.Dependencies {
		nodes[i] = &DependencyNode{r}
	}

	return nodes
}

// AddNeighbors adds dependencies to a LockPackage and
// updates the parent constraints of the dependencies in the DAG.
func (l *PackageNode) AddNeighbors(nodes ...Node) error {
	for _, n := range nodes {
		for _, dep := range l.Dependencies {
			if dep.Identifier() == n.Identifier() {
				n.AddParentConstraints([]string{dep.Constraints})
				break
			}
		}
	}

	return nil
}

// PackagesToNodes converts LockPackages to DAG nodes.
func PackagesToNodes(pkgs ...v1beta1.LockPackage) []Node {
	nodes := make([]Node, len(pkgs))
	for i, r := range pkgs {
		nodes[i] = &PackageNode{r}
	}

	return nodes
}
