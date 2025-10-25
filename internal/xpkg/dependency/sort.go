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

package dependency

import (
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"

	"github.com/crossplane/crossplane/v2/apis/pkg/v1beta1"
)

// SortLockPackages sorts packages in dependency order using topological sort.
// Dependencies are ordered before their dependents, enabling safe sequential
// installation. Returns an error if the dependency graph contains a cycle.
func SortLockPackages(p []v1beta1.LockPackage) ([]v1beta1.LockPackage, error) {
	if len(p) == 0 {
		return p, nil
	}

	// Build package lookup map
	pkgs := make(map[string]v1beta1.LockPackage, len(p))
	for _, pkg := range p {
		pkgs[pkg.Source] = pkg
	}

	// Build adjacency list (source -> list of packages that depend on it)
	dependents := make(map[string][]string)
	for _, pkg := range p {
		for _, dep := range pkg.Dependencies {
			dependents[dep.Package] = append(dependents[dep.Package], pkg.Source)
		}
	}

	// Calculate in-degree (number of dependencies) for each package
	inDegree := make(map[string]int, len(p))
	for _, pkg := range p {
		inDegree[pkg.Source] = len(pkg.Dependencies)
	}

	// Kahn's algorithm: start with packages that have no dependencies
	var queue []string
	for source, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, source)
		}
	}

	// Process packages in dependency order
	sorted := make([]v1beta1.LockPackage, 0, len(p))
	for len(queue) > 0 {
		// Remove first package from queue
		source := queue[0]
		queue = queue[1:]

		// Add to sorted output
		sorted = append(sorted, pkgs[source])

		// Decrease in-degree for all packages that depend on this one
		for _, dependent := range dependents[source] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	// If we didn't process all packages, there's a cycle
	if len(sorted) != len(p) {
		return nil, errors.New("dependency graph contains a cycle")
	}

	return sorted, nil
}
