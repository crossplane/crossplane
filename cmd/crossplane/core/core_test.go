/*
Copyright 2026 The Crossplane Authors.

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

package core

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/customresourcesgate"
)

// The API extensions cache strips schemas from CRDs. Functions can require
// CRDs, which they read from the same cache as unstructured objects. This test
// guards that the transform leaves unstructured CRDs intact.
func TestTransformStripCRDSchema(t *testing.T) {
	schema := &extv1.CustomResourceValidation{OpenAPIV3Schema: &extv1.JSONSchemaProps{Type: "object"}}
	crd := func() *extv1.CustomResourceDefinition {
		return &extv1.CustomResourceDefinition{
			TypeMeta: metav1.TypeMeta{APIVersion: "apiextensions.k8s.io/v1", Kind: "CustomResourceDefinition"},
			Spec: extv1.CustomResourceDefinitionSpec{
				Versions: []extv1.CustomResourceDefinitionVersion{{Name: "v1", Schema: schema}},
			},
		}
	}

	cases := map[string]struct {
		reason string
		obj    func(t *testing.T) any
		want   bool
	}{
		"TypedCRD": {
			reason: "Typed CRDs, read by the informer garbage collector and the OpenAPI cache invalidation, should have their schemas stripped.",
			obj:    func(_ *testing.T) any { return crd() },
			want:   false,
		},
		"UnstructuredCRD": {
			reason: "Unstructured CRDs, read by functions that require them, should keep their schemas.",
			obj: func(t *testing.T) any {
				t.Helper()
				u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(crd())
				if err != nil {
					t.Fatal(err)
				}
				return &kunstructured.Unstructured{Object: u}
			},
			want: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := customresourcesgate.TransformStripCRDSchema(tc.obj(t))
			if err != nil {
				t.Fatalf("\n%s\nTransformStripCRDSchema(...): unexpected error: %v", tc.reason, err)
			}

			if diff := cmp.Diff(tc.want, hasSchema(t, got)); diff != "" {
				t.Errorf("\n%s\nTransformStripCRDSchema(...): -want schema, +got schema:\n%s", tc.reason, diff)
			}
		})
	}
}

func hasSchema(t *testing.T, obj any) bool {
	t.Helper()

	switch o := obj.(type) {
	case *extv1.CustomResourceDefinition:
		return o.Spec.Versions[0].Schema != nil
	case *kunstructured.Unstructured:
		versions, _, err := kunstructured.NestedSlice(o.Object, "spec", "versions")
		if err != nil {
			t.Fatal(err)
		}
		_, ok := versions[0].(map[string]any)["schema"]
		return ok
	default:
		t.Fatalf("unexpected type %T", obj)
		return false
	}
}
