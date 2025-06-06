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
	apitest "k8s.io/apiextensions-apiserver/pkg/test"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"os"
	"reflect"
	"testing"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/defaulting"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// DefaultFor creates a defaulting function for the specified type T by loading
// the default values from the corresponding CRD file. The returned function can
// be used to apply defaults to objects of type T as defined in the CRD schema.
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

// VersionDefaultsFromFile extracts the defaulting functions by version from a CRD file.
// It parses the CRD's OpenAPI schema and creates defaulting functions for each version
// that can be used to apply default values to objects during testing.
func VersionDefaultsFromFile(t *testing.T, crdFilePath string) map[string]func(any) {
	data, err := os.ReadFile(crdFilePath)
	if err != nil {
		t.Fatalf("failed to read CRD file at path %q: %v", crdFilePath, err)
	}

	var crd apiextensionsv1.CustomResourceDefinition
	err = yaml.Unmarshal(data, &crd)
	if err != nil {
		t.Fatalf("failed to unmarshal CRD: %v", err)
	}

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

// ValidatorFor creates a CEL validation function for the specified type T by loading
// the validation rules from the corresponding CRD file. The returned function can
// be used to validate objects of type T against the CEL rules defined in the CRD.
func ValidatorFor[T any](t *testing.T) func(obj, old *T) field.ErrorList {
	t.Helper()

	path, version := CRDPathVersionFor[T](t)

	val, err := apitest.VersionValidatorFromFile(t, path, version)
	if err != nil {
		t.Fatalf("unable to load validator from path %q at version %q: %v", path, version, err)
	}
	return func(obj, old *T) field.ErrorList {
		t.Helper()

		defaultFn := DefaultFor[T](t)
		defaultFn(obj)

		return val(ToUnstructured(t, obj).Object, ToUnstructured(t, old).Object)
	}
}

// FromUnstructured converts an unstructured object back to a typed object.
// This is used in testing to convert objects that have been processed by
// the defaulting webhook back to their original typed form.
func FromUnstructured(t *testing.T, u *unstructured.Unstructured, obj any) {
	t.Helper()
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, obj)
	if err != nil {
		t.Fatalf("failed to convert from unstructured: %v", err)
	}
}

// ToUnstructured converts a typed object to an unstructured object.
// This is used in testing to convert objects to a form that can be processed
// by the defaulting webhook and validation functions.
func ToUnstructured(t *testing.T, obj any) *unstructured.Unstructured {
	t.Helper()
	if obj == nil || reflect.ValueOf(obj).IsNil() {
		return new(unstructured.Unstructured)
	}
	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		t.Fatalf("failed to convert to unstructured: %v", err)
	}
	return &unstructured.Unstructured{Object: content}
}
