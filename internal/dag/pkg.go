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
	"fmt"

	"github.com/crossplane/crossplane/apis/v2/pkg/v1beta1"
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

// PackageNode is a DAG node representing a package. More than one version of a
// package may be installed at once, in which case a single PackageNode
// represents all of them: the embedded LockPackage holds the first observed
// version, while versions holds all installed versions and dependencies holds
// the union of all installed versions' dependencies.
type PackageNode struct {
	v1beta1.LockPackage

	versions     []string
	dependencies []v1beta1.Dependency
}

// GetVersions returns every installed version this node represents.
func (l *PackageNode) GetVersions() []string {
	return l.versions
}

// Neighbors returns dependencies of a LockPackage.
func (l *PackageNode) Neighbors() []Node {
	nodes := make([]Node, len(l.dependencies))
	for i, r := range l.dependencies {
		nodes[i] = &DependencyNode{r}
	}

	return nodes
}

// AddNeighbors adds dependencies to a LockPackage and updates the parent
// constraints of the dependencies in the DAG.
func (l *PackageNode) AddNeighbors(nodes ...Node) error {
	for _, n := range nodes {
		for _, dep := range l.dependencies {
			if dep.Identifier() == n.Identifier() {
				n.AddParentConstraints([]string{dep.Constraints})
				break
			}
		}
	}

	return nil
}

// PackagesToNodes converts LockPackages to DAG nodes. Packages that share a
// source are merged into a single node.
func PackagesToNodes(pkgs ...v1beta1.LockPackage) []Node {
	// Preserve first-seen order so the result is deterministic.
	order := make([]string, 0, len(pkgs))
	bySource := make(map[string]*PackageNode, len(pkgs))
	seenDep := make(map[string]map[string]bool, len(pkgs))
	seenVer := make(map[string]map[string]bool, len(pkgs))

	for i := range pkgs {
		pkg := pkgs[i]
		src := pkg.Identifier()

		n, ok := bySource[src]
		if !ok {
			n = &PackageNode{LockPackage: pkg}
			bySource[src] = n
			seenDep[src] = make(map[string]bool)
			seenVer[src] = make(map[string]bool)
			order = append(order, src)
		}

		for _, dep := range pkg.Dependencies {
			// Dedupe on the (package, constraints) pair so distinct constraints
			// from different versions are all preserved.
			key := fmt.Sprintf("%s@%s", dep.Package, dep.Constraints)
			if seenDep[src][key] {
				continue
			}

			seenDep[src][key] = true
			n.dependencies = append(n.dependencies, dep)
		}

		if pkg.Version != "" && !seenVer[src][pkg.Version] {
			seenVer[src][pkg.Version] = true
			n.versions = append(n.versions, pkg.Version)
		}
	}

	nodes := make([]Node, 0, len(order))
	for _, src := range order {
		nodes = append(nodes, bySource[src])
	}

	return nodes
}
