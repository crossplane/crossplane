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

package converter

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/crossplane/apis/apiextensions/v2alpha1"
)

func TestCustomToManagedResourceDefinitions(t *testing.T) {
	// Test customResourceDefinition
	testCRD := &extv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: extv1.SchemeGroupVersion.String(),
			Kind:       customResourceDefinition,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "buckets.example.org",
			Namespace: "test-namespace",
		},
		Spec: extv1.CustomResourceDefinitionSpec{
			Group: "example.org",
			Names: extv1.CustomResourceDefinitionNames{
				Kind:     "Bucket",
				Plural:   "buckets",
				Singular: "bucket",
			},
			Scope: extv1.NamespaceScoped,
			Versions: []extv1.CustomResourceDefinitionVersion{
				{
					Name:    "v1",
					Served:  true,
					Storage: true,
				},
			},
		},
	}

	// Test unstructured CRD
	testUnstructuredCRD := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": extv1.SchemeGroupVersion.String(),
			"kind":       customResourceDefinition,
			"metadata": map[string]any{
				"name":      "instances.example.org",
				"namespace": "test-namespace",
			},
			"spec": map[string]any{
				"group": "example.org",
				"names": map[string]any{
					"kind":     "Instance",
					"plural":   "instances",
					"singular": "instance",
				},
				"scope": "Namespaced",
				"versions": []any{
					map[string]any{
						"name":    "v1",
						"served":  true,
						"storage": true,
					},
				},
			},
		},
	}

	// Test non-CRD object
	testNonCRD := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]any{
				"name":      "test-deployment",
				"namespace": "test-namespace",
			},
		},
	}

	// Expected MRD from structured CRD
	expectedMRDFromCRD := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": v2alpha1.SchemeGroupVersion.String(),
			"kind":       v2alpha1.ManagedResourceDefinitionKind,
			"metadata": map[string]any{
				"creationTimestamp": nil,
				"name":              "buckets.example.org",
				"namespace":         "test-namespace",
			},
			"spec": map[string]any{
				"group": "example.org",
				"names": map[string]any{
					"kind":     "Bucket",
					"plural":   "buckets",
					"singular": "bucket",
				},
				"scope": "Namespaced",
				"versions": []any{
					map[string]any{
						"name":    "v1",
						"served":  true,
						"storage": true,
					},
				},
			},
			"status": map[string]any{
				"acceptedNames": map[string]any{
					"kind":   "",
					"plural": "",
				},
				"conditions":     nil,
				"storedVersions": nil,
			},
		},
	}

	// Expected MRD from unstructured CRD
	expectedMRDFromUnstructured := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": v2alpha1.SchemeGroupVersion.String(),
			"kind":       v2alpha1.ManagedResourceDefinitionKind,
			"metadata": map[string]any{
				"name":      "instances.example.org",
				"namespace": "test-namespace",
			},
			"spec": map[string]any{
				"group": "example.org",
				"names": map[string]any{
					"kind":     "Instance",
					"plural":   "instances",
					"singular": "instance",
				},
				"scope": "Namespaced",
				"versions": []any{
					map[string]any{
						"name":    "v1",
						"served":  true,
						"storage": true,
					},
				},
			},
		},
	}

	type args struct {
		objects []runtime.Object
	}
	type want struct {
		objects []runtime.Object
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"EmptyInput": {
			reason: "Should return empty slice for empty input",
			args: args{
				objects: []runtime.Object{},
			},
			want: want{
				objects: []runtime.Object{},
			},
		},
		"NonCRDObject": {
			reason: "Should leave non-CRD objects unchanged",
			args: args{
				objects: []runtime.Object{testNonCRD},
			},
			want: want{
				objects: []runtime.Object{testNonCRD},
			},
		},
		"SingleStructuredCRD": {
			reason: "Should convert single structured CRD to MRD",
			args: args{
				objects: []runtime.Object{testCRD},
			},
			want: want{
				objects: []runtime.Object{expectedMRDFromCRD},
			},
		},
		"SingleUnstructuredCRD": {
			reason: "Should convert single unstructured CRD to MRD",
			args: args{
				objects: []runtime.Object{testUnstructuredCRD},
			},
			want: want{
				objects: []runtime.Object{expectedMRDFromUnstructured},
			},
		},
		"MixedObjects": {
			reason: "Should convert only CRD objects, leaving others unchanged",
			args: args{
				objects: []runtime.Object{testCRD, testNonCRD, testUnstructuredCRD},
			},
			want: want{
				objects: []runtime.Object{expectedMRDFromCRD, testNonCRD, expectedMRDFromUnstructured},
			},
		},
		"MultipleCRDs": {
			reason: "Should convert multiple CRDs to MRDs",
			args: args{
				objects: []runtime.Object{testCRD, testUnstructuredCRD},
			},
			want: want{
				objects: []runtime.Object{expectedMRDFromCRD, expectedMRDFromUnstructured},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			gotObjects := CustomToManagedResourceDefinitions(tc.args.objects...)

			if diff := cmp.Diff(tc.want.objects, gotObjects); diff != "" {
				t.Errorf("\n%s\nCustomToManagedResourceDefinitions(...): -want objects, +got objects:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestConvertCRDToMRD(t *testing.T) {
	type args struct {
		in map[string]any
	}
	type want struct {
		out map[string]any
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"BasicCRD": {
			reason: "Should convert basic CRD to MRD format",
			args: args{
				in: map[string]any{
					"apiVersion": extv1.SchemeGroupVersion.String(),
					"kind":       customResourceDefinition,
					"metadata": map[string]any{
						"name": "buckets.example.org",
					},
					"spec": map[string]any{
						"group": "example.org",
						"names": map[string]any{
							"kind":   "Bucket",
							"plural": "buckets",
						},
					},
				},
			},
			want: want{
				out: map[string]any{
					"apiVersion": v2alpha1.SchemeGroupVersion.String(),
					"kind":       v2alpha1.ManagedResourceDefinitionKind,
					"metadata": map[string]any{
						"name": "buckets.example.org",
					},
					"spec": map[string]any{
						"group": "example.org",
						"names": map[string]any{
							"kind":   "Bucket",
							"plural": "buckets",
						},
					},
				},
			},
		},
		"CRDWithComplexSpec": {
			reason: "Should convert CRD with complex spec to MRD, preserving all fields",
			args: args{
				in: map[string]any{
					"apiVersion": extv1.SchemeGroupVersion.String(),
					"kind":       customResourceDefinition,
					"metadata": map[string]any{
						"name":      "databases.example.org",
						"namespace": "test-namespace",
						"labels": map[string]any{
							"app": "test",
						},
					},
					"spec": map[string]any{
						"group": "example.org",
						"names": map[string]any{
							"kind":     "Database",
							"plural":   "databases",
							"singular": "database",
						},
						"scope": "Namespaced",
						"versions": []any{
							map[string]any{
								"name":    "v1",
								"served":  true,
								"storage": true,
								"schema": map[string]any{
									"openAPIV3Schema": map[string]any{
										"type": "object",
										"properties": map[string]any{
											"spec": map[string]any{
												"type": "object",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: want{
				out: map[string]any{
					"apiVersion": v2alpha1.SchemeGroupVersion.String(),
					"kind":       v2alpha1.ManagedResourceDefinitionKind,
					"metadata": map[string]any{
						"name":      "databases.example.org",
						"namespace": "test-namespace",
						"labels": map[string]any{
							"app": "test",
						},
					},
					"spec": map[string]any{
						"group": "example.org",
						"names": map[string]any{
							"kind":     "Database",
							"plural":   "databases",
							"singular": "database",
						},
						"scope": "Namespaced",
						"versions": []any{
							map[string]any{
								"name":    "v1",
								"served":  true,
								"storage": true,
								"schema": map[string]any{
									"openAPIV3Schema": map[string]any{
										"type": "object",
										"properties": map[string]any{
											"spec": map[string]any{
												"type": "object",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		"MinimalCRD": {
			reason: "Should convert minimal CRD to MRD",
			args: args{
				in: map[string]any{
					"apiVersion": extv1.SchemeGroupVersion.String(),
					"kind":       customResourceDefinition,
					"metadata": map[string]any{
						"name": "minimal.example.org",
					},
					"spec": map[string]any{},
				},
			},
			want: want{
				out: map[string]any{
					"apiVersion": v2alpha1.SchemeGroupVersion.String(),
					"kind":       v2alpha1.ManagedResourceDefinitionKind,
					"metadata": map[string]any{
						"name": "minimal.example.org",
					},
					"spec": map[string]any{},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := convertCRDToMRD(tc.args.in)

			if diff := cmp.Diff(tc.want.out, got); diff != "" {
				t.Errorf("\n%s\nconvertCRDToMRD(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
