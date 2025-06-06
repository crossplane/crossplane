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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/defaulting"
	apitest "k8s.io/apiextensions-apiserver/pkg/test"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/yaml"

	apiextv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	apiextv1beta1 "github.com/crossplane/crossplane/apis/apiextensions/v1beta1"
	apiextv2alpha1 "github.com/crossplane/crossplane/apis/apiextensions/v2alpha1"
	pkgv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	pkgv1beta1 "github.com/crossplane/crossplane/apis/pkg/v1beta1"
	protectionv1beta1 "github.com/crossplane/crossplane/apis/protection/v1beta1"
)

func CRDFor[T any](t *testing.T) (*apiextensionsv1.CustomResourceDefinition, string) {
	t.Helper()

	path, version := CRDPathVersionFor[T](t)
	return apitest.MustLoadManifest[apiextensionsv1.CustomResourceDefinition](t, path), version
}

func CRDPathVersionFor[T any](t *testing.T) (string, string) {
	t.Helper()

	version := "unknown"
	path := filepath.Join("..", "..", "cluster", "crds")

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

func ValidatorFor[T any](t *testing.T) func(obj, old *T) field.ErrorList {
	t.Helper()

	path, version := CRDPathVersionFor[T](t)

	val, err := apitest.VersionValidatorFromFile(t, path, version)
	if err != nil {
		t.Fatalf("unable to load validator from path %q at version %q: %v", path, version, err)
	}
	return func(obj, old *T) field.ErrorList {
		t.Helper()
		return val(obj, old)
	}
}

func DefaultFor[T any](t *testing.T) func(any) {
	t.Helper()
	path, version := CRDPathVersionFor[T](t)
	defaults := VersionDefaultsFromFile(t, path)
	defaultFn, found := defaults[version]
	if !found {
		t.Fatalf("version %q not found in defaults", version)
	}
	return defaultFn
}

// VersionDefaultsFromFile extracts the defaulters by version from a CRD file and returns
// a Defaulter func for testing against samples.
func VersionDefaultsFromFile(t *testing.T, crdFilePath string) map[string]func(any) {
	data, err := os.ReadFile(crdFilePath)
	require.NoError(t, err)

	var crd apiextensionsv1.CustomResourceDefinition
	err = yaml.Unmarshal(data, &crd)
	require.NoError(t, err)

	ret := map[string]func(any){}
	for _, v := range crd.Spec.Versions {
		var internalSchema apiextensions.JSONSchemaProps
		if err := apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(v.Schema.OpenAPIV3Schema, &internalSchema, nil); err != nil {
			t.Fatalf("failed to convert JSONSchemaProps for version %s: %v", v.Name, err)
		}
		structuralSchema, err := schema.NewStructural(&internalSchema)
		if err != nil {
			t.Fatalf("failed to create StructuralSchema for version %s: %v", v.Name, err)
		}

		ret[v.Name] = func(obj any) {
			u := ToUnstructured(t, obj)
			defaulting.Default(u.Object, structuralSchema)
			FromUnstructured(t, u, obj)
		}
	}

	return ret
}

func FromUnstructured(t *testing.T, u *unstructured.Unstructured, obj any) {
	t.Helper()
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, obj)
	if err != nil {
		t.Fatalf("failed to convert from unstructured: %v", err)
	}
}

func ToUnstructured(t *testing.T, obj any) *unstructured.Unstructured {
	t.Helper()
	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		t.Fatalf("failed to convert to unstructured: %v", err)
	}
	return &unstructured.Unstructured{Object: content}
}

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
