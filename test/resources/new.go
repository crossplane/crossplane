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

// Package resources tests resource interactions with the CEL defined in the on-disk CRD file.
package resources

import (
	"testing"

	apiextv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	apiextv1beta1 "github.com/crossplane/crossplane/apis/apiextensions/v1beta1"
	apiextv2alpha1 "github.com/crossplane/crossplane/apis/apiextensions/v2alpha1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	pkgv1beta1 "github.com/crossplane/crossplane/apis/pkg/v1beta1"
	protectionv1beta1 "github.com/crossplane/crossplane/apis/protection/v1beta1"
)

// New creates a new instance of the specified type T with the appropriate
// GroupVersionKind set. This is used in testing to create properly initialized
// objects that can be used with the validation and defaulting functions.
func New[T any](t *testing.T) *T {
	t.Helper()

	var obj T
	switch v := any(&obj).(type) {
	// apiextensions.crossplane.io/v2alpha1
	case *apiextv2alpha1.CompositeResourceDefinition:
		v.SetGroupVersionKind(apiextv2alpha1.CompositeResourceDefinitionGroupVersionKind)

	// apiextensions.crossplane.io/v1
	case *apiextv1.CompositeResourceDefinition:
		v.SetGroupVersionKind(apiextv1.CompositeResourceDefinitionGroupVersionKind)
	case *apiextv1.CompositionRevision:
		v.SetGroupVersionKind(apiextv1.CompositionRevisionGroupVersionKind)
	case *apiextv1.Composition:
		v.SetGroupVersionKind(apiextv1.CompositionGroupVersionKind)

	// apiextensions.crossplane.io/v1beta1
	case *apiextv1beta1.EnvironmentConfig:
		v.SetGroupVersionKind(apiextv1beta1.EnvironmentConfigGroupVersionKind)
	case *apiextv1beta1.Usage:
		v.SetGroupVersionKind(apiextv1beta1.UsageGroupVersionKind)

	// pkg.crossplane.io/v1
	case *pkgv1.ConfigurationRevision:
		v.SetGroupVersionKind(pkgv1.ConfigurationRevisionGroupVersionKind)
	case *pkgv1.Configuration:
		v.SetGroupVersionKind(pkgv1.ConfigurationGroupVersionKind)
	case *pkgv1.Function:
		v.SetGroupVersionKind(pkgv1.FunctionGroupVersionKind)
	case *pkgv1.FunctionRevision:
		v.SetGroupVersionKind(pkgv1.FunctionRevisionGroupVersionKind)
	case *pkgv1.ProviderRevision:
		v.SetGroupVersionKind(pkgv1.ProviderRevisionGroupVersionKind)
	case *pkgv1.Provider:
		v.SetGroupVersionKind(pkgv1.ProviderGroupVersionKind)

	// pkg.crossplane.io/v1beta1
	case *pkgv1beta1.DeploymentRuntimeConfig:
		v.SetGroupVersionKind(pkgv1beta1.DeploymentRuntimeConfigGroupVersionKind)
	case *pkgv1beta1.Function:
		v.SetGroupVersionKind(pkgv1beta1.FunctionGroupVersionKind)
	case *pkgv1beta1.FunctionRevision:
		v.SetGroupVersionKind(pkgv1beta1.FunctionRevisionGroupVersionKind)
	case *pkgv1beta1.Lock:
		v.SetGroupVersionKind(pkgv1beta1.LockGroupVersionKind)
	case *pkgv1beta1.ImageConfig:
		v.SetGroupVersionKind(pkgv1beta1.ImageConfigGroupVersionKind)

	// protection.crossplane.io/v1beta1
	case *protectionv1beta1.ClusterUsage:
		v.SetGroupVersionKind(protectionv1beta1.ClusterUsageGroupVersionKind)
	case *protectionv1beta1.Usage:
		v.SetGroupVersionKind(protectionv1beta1.UsageGroupVersionKind)

	default:
		t.Fatalf("unknown object %T", obj)
	}

	return &obj
}
