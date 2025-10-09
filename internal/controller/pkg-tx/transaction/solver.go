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
	"context"

	"github.com/crossplane/crossplane/v2/apis/pkg/v1beta1"
)

// DependencySolver resolves package dependencies to concrete digests.
type DependencySolver interface {
	// Solve takes a package reference (potentially with a tag or version constraint)
	// and the current Lock state, then:
	// - Fetches the package from the OCI registry
	// - Resolves tags to specific digests
	// - Recursively resolves all dependencies
	// - Validates version constraints are satisfiable
	// - Detects circular dependencies
	// - Returns the complete proposed Lock state with all packages at specific digests
	//
	// Returns an error if resolution fails (missing packages, constraint conflicts,
	// circular dependencies, network errors, etc.)
	Solve(ctx context.Context, source string, currentLock []v1beta1.LockPackage) ([]v1beta1.LockPackage, error)
}
