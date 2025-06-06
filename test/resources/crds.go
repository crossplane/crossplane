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
	"path/filepath"
	rntm "runtime"
	"testing"

	apiextv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	apiextv1beta1 "github.com/crossplane/crossplane/apis/apiextensions/v1beta1"
	apiextv2alpha1 "github.com/crossplane/crossplane/apis/apiextensions/v2alpha1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	pkgv1beta1 "github.com/crossplane/crossplane/apis/pkg/v1beta1"
	protectionv1beta1 "github.com/crossplane/crossplane/apis/protection/v1beta1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apitest "k8s.io/apiextensions-apiserver/pkg/test"
)

// CRDFor loads the CRD for the specified type T from the cluster/crds directory
// and returns both the CRD object and the version being used for the type.
func CRDFor[T any](t *testing.T) (*apiextensionsv1.CustomResourceDefinition, string) {
	t.Helper()

	path, version := CRDPathVersionFor[T](t)
	return apitest.MustLoadManifest[apiextensionsv1.CustomResourceDefinition](t, path), version
}

// CRDPathVersionFor returns the file path to the CRD YAML file and the API version
// for the specified type T. The path is relative to the test directory and points
// to the corresponding CRD file in cluster/crds.
func CRDPathVersionFor[T any](t *testing.T) (string, string) {
	t.Helper()

	version := "unknown"

	// Path is relative to _this_ file. But go gets confused when we call CRDPathVersionFor from somewhere else.
	// So lets make `path` an absolute path.
	_, file, _, ok := rntm.Caller(0)
	if !ok {
		t.Fatal("could not determine CRD file location")
	}
	file, _ = filepath.Abs(filepath.Join(file, "..", ".."))
	path := filepath.Dir(file)
	path = filepath.Join(path, "cluster", "crds")

	var obj T
	switch any(obj).(type) {
	// apiextensions.crossplane.io/v2alpha1
	case apiextv2alpha1.CompositeResourceDefinition:
		version = apiextv2alpha1.CompositeResourceDefinitionGroupVersionKind.Version
		path = filepath.Join(path, "apiextensions.crossplane.io_compositeresourcedefinitions.yaml")

	// apiextensions.crossplane.io/v1
	case apiextv1.CompositeResourceDefinition:
		version = apiextv1.CompositeResourceDefinitionGroupVersionKind.Version
		path = filepath.Join(path, "apiextensions.crossplane.io_compositeresourcedefinitions.yaml")
	case apiextv1.CompositionRevision:
		version = apiextv1.CompositionRevisionGroupVersionKind.Version
		path = filepath.Join(path, "apiextensions.crossplane.io_compositionrevisions.yaml")
	case apiextv1.Composition:
		version = apiextv1.CompositionGroupVersionKind.Version
		path = filepath.Join(path, "apiextensions.crossplane.io_compositions.yaml")

	// apiextensions.crossplane.io/v1beta1
	case apiextv1beta1.EnvironmentConfig:
		version = apiextv1beta1.EnvironmentConfigGroupVersionKind.Version
		path = filepath.Join(path, "apiextensions.crossplane.io_environmentconfigs.yaml")
	case apiextv1beta1.Usage:
		version = apiextv1beta1.UsageGroupVersionKind.Version
		path = filepath.Join(path, "apiextensions.crossplane.io_usages.yaml")

	// pkg.crossplane.io/v1
	case pkgv1.ConfigurationRevision:
		version = pkgv1.ConfigurationRevisionGroupVersionKind.Version
		path = filepath.Join(path, "pkg.crossplane.io_configurationrevisions.yaml")
	case pkgv1.Configuration:
		version = pkgv1.ConfigurationGroupVersionKind.Version
		path = filepath.Join(path, "pkg.crossplane.io_configurations.yaml")
	case pkgv1beta1.DeploymentRuntimeConfig:
		version = pkgv1beta1.DeploymentRuntimeConfigGroupVersionKind.Version
		path = filepath.Join(path, "pkg.crossplane.io_deploymentruntimeconfigs.yaml")
	case pkgv1.Function:
		version = pkgv1.FunctionGroupVersionKind.Version
		path = filepath.Join(path, "pkg.crossplane.io_functions.yaml")
	case pkgv1.FunctionRevision:
		version = pkgv1.FunctionRevisionGroupVersionKind.Version
		path = filepath.Join(path, "pkg.crossplane.io_functionrevisions.yaml")
	case pkgv1.ProviderRevision:
		version = pkgv1.ProviderRevisionGroupVersionKind.Version
		path = filepath.Join(path, "pkg.crossplane.io_providerrevisions.yaml")
	case pkgv1.Provider:
		version = pkgv1.FunctionGroupVersionKind.Version
		path = filepath.Join(path, "pkg.crossplane.io_providers.yaml")

	// pkg.crossplane.io/v1beta1
	case pkgv1beta1.Function:
		version = pkgv1beta1.FunctionGroupVersionKind.Version
		path = filepath.Join(path, "pkg.crossplane.io_functions.yaml")
	case pkgv1beta1.FunctionRevision:
		version = pkgv1beta1.FunctionRevisionGroupVersionKind.Version
		path = filepath.Join(path, "pkg.crossplane.io_functionrevisions.yaml")
	case pkgv1beta1.Lock:
		version = pkgv1beta1.LockGroupVersionKind.Version
		path = filepath.Join(path, "pkg.crossplane.io_locks.yaml")
	case pkgv1beta1.ImageConfig:
		version = pkgv1beta1.ImageConfigGroupVersionKind.Version
		path = filepath.Join(path, "pkg.crossplane.io_imageconfigs.yaml")

	// protection.crossplane.io/v1beta1
	case protectionv1beta1.ClusterUsage:
		version = protectionv1beta1.ClusterUsageGroupVersionKind.Version
		path = filepath.Join(path, "protection.crossplane.io_clusterusages.yaml")
	case protectionv1beta1.Usage:
		version = protectionv1beta1.UsageGroupVersionKind.Version
		path = filepath.Join(path, "protection.crossplane.io_usages.yaml")

	default:
		t.Fatalf("unknown object %T", obj)
	}

	return path, version
}
