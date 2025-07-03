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

// Package shared contains shared types and constants used across multiple versions of the apiextensions API.
package shared

// CompositeResourceScope specifies the scope of a composite resource.
type CompositeResourceScope string

// Composite resource scopes.
const (
	CompositeResourceScopeNamespaced CompositeResourceScope = "Namespaced"
	CompositeResourceScopeCluster    CompositeResourceScope = "Cluster"

	// Deprecated: CompositeResourceScopeLegacyCluster is deprecated and will be removed in a future
	// version.
	CompositeResourceScopeLegacyCluster CompositeResourceScope = "LegacyCluster"
)
