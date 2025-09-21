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
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	"github.com/crossplane/crossplane/v2/apis/apiextensions/v1alpha1"
)

// invalidJSONObject is a runtime.Object that fails to marshal to JSON.
type invalidJSONObject struct {
	metav1.TypeMeta
	metav1.ObjectMeta
	Channel chan struct{} `json:"channel"` // channels cannot be marshaled to JSON
}

func (i *invalidJSONObject) DeepCopyObject() runtime.Object {
	return &invalidJSONObject{
		TypeMeta: metav1.TypeMeta{
			APIVersion: customResourceDefinitionKind.GroupVersion().String(),
			Kind:       customResourceDefinitionKind.Kind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "invalid-json-object",
		},
		Channel: make(chan struct{}),
	}
}

func (i *invalidJSONObject) GetObjectKind() schema.ObjectKind {
	return i
}

func (i *invalidJSONObject) GroupVersionKind() schema.GroupVersionKind {
	return customResourceDefinitionKind
}

func (i *invalidJSONObject) SetGroupVersionKind(gvk schema.GroupVersionKind) {
	i.APIVersion = gvk.GroupVersion().String()
	i.Kind = gvk.Kind
}

func TestCustomToManagedResourceDefinitions(t *testing.T) {
	// Test CustomResourceDefinition
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

	// Test ProviderConfig CRD (should not be converted to MRD)
	testProviderConfigCRD := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": extv1.SchemeGroupVersion.String(),
			"kind":       customResourceDefinition,
			"metadata": map[string]any{
				"name": "providerconfigs.aws.upbound.io",
			},
			"spec": map[string]any{
				"group": "aws.upbound.io",
				"names": map[string]any{
					"kind":     "ProviderConfig",
					"plural":   "providerconfigs",
					"singular": "providerconfig",
				},
				"scope": "Cluster",
			},
		},
	}

	// Test ClusterProviderConfig CRD (should not be converted to MRD)
	testClusterProviderConfigCRD := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": extv1.SchemeGroupVersion.String(),
			"kind":       customResourceDefinition,
			"metadata": map[string]any{
				"name": "clusterproviderconfigs.aws.m.upbound.io",
			},
			"spec": map[string]any{
				"group": "aws.m.upbound.io",
				"names": map[string]any{
					"kind":     "ClusterProviderConfig",
					"plural":   "clusterproviderconfigs",
					"singular": "clusterproviderconfig",
				},
				"scope": "Cluster",
			},
		},
	}

	// Test ProviderConfigUsage CRD (should not be converted to MRD)
	testProviderConfigUsageCRD := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": extv1.SchemeGroupVersion.String(),
			"kind":       customResourceDefinition,
			"metadata": map[string]any{
				"name": "providerconfigusages.aws.upbound.io",
			},
			"spec": map[string]any{
				"group": "aws.upbound.io",
				"names": map[string]any{
					"kind":     "ProviderConfigUsage",
					"plural":   "providerconfigusages",
					"singular": "providerconfigusage",
				},
				"scope": "Cluster",
			},
		},
	}

	// Expected MRD from structured CRD (inactive)
	expectedMRDFromCRDInactive := &v1alpha1.ManagedResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.crossplane.io/v1alpha1",
			Kind:       "ManagedResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "buckets.example.org",
			Namespace: "test-namespace",
		},
		Spec: v1alpha1.ManagedResourceDefinitionSpec{
			CustomResourceDefinitionSpec: v1alpha1.CustomResourceDefinitionSpec{
				Group: "example.org",
				Names: extv1.CustomResourceDefinitionNames{
					Kind:     "Bucket",
					Plural:   "buckets",
					Singular: "bucket",
				},
				Scope: "Namespaced",
				Versions: []v1alpha1.CustomResourceDefinitionVersion{
					{
						Name:    "v1",
						Served:  true,
						Storage: true,
					},
				},
			},
		},
	}

	// Expected MRD from structured CRD (active)
	expectedMRDFromCRDActive := &v1alpha1.ManagedResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.crossplane.io/v1alpha1",
			Kind:       "ManagedResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "buckets.example.org",
			Namespace: "test-namespace",
		},
		Spec: v1alpha1.ManagedResourceDefinitionSpec{
			CustomResourceDefinitionSpec: v1alpha1.CustomResourceDefinitionSpec{
				Group: "example.org",
				Names: extv1.CustomResourceDefinitionNames{
					Kind:     "Bucket",
					Plural:   "buckets",
					Singular: "bucket",
				},
				Scope: "Namespaced",
				Versions: []v1alpha1.CustomResourceDefinitionVersion{
					{
						Name:    "v1",
						Served:  true,
						Storage: true,
					},
				},
			},
			State: v1alpha1.ManagedResourceDefinitionActive,
		},
	}

	// Expected MRD from unstructured CRD (inactive)
	expectedMRDFromUnstructuredInactive := &v1alpha1.ManagedResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.crossplane.io/v1alpha1",
			Kind:       "ManagedResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "instances.example.org",
			Namespace: "test-namespace",
		},
		Spec: v1alpha1.ManagedResourceDefinitionSpec{
			CustomResourceDefinitionSpec: v1alpha1.CustomResourceDefinitionSpec{
				Group: "example.org",
				Names: extv1.CustomResourceDefinitionNames{
					Kind:     "Instance",
					Plural:   "instances",
					Singular: "instance",
				},
				Scope: "Namespaced",
				Versions: []v1alpha1.CustomResourceDefinitionVersion{
					{
						Name:    "v1",
						Served:  true,
						Storage: true,
					},
				},
			},
		},
	}

	// Expected MRD from unstructured CRD (active)
	expectedMRDFromUnstructuredActive := &v1alpha1.ManagedResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.crossplane.io/v1alpha1",
			Kind:       "ManagedResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "instances.example.org",
			Namespace: "test-namespace",
		},
		Spec: v1alpha1.ManagedResourceDefinitionSpec{
			CustomResourceDefinitionSpec: v1alpha1.CustomResourceDefinitionSpec{
				Group: "example.org",
				Names: extv1.CustomResourceDefinitionNames{
					Kind:     "Instance",
					Plural:   "instances",
					Singular: "instance",
				},
				Scope: "Namespaced",
				Versions: []v1alpha1.CustomResourceDefinitionVersion{
					{
						Name:    "v1",
						Served:  true,
						Storage: true,
					},
				},
			},
			State: v1alpha1.ManagedResourceDefinitionActive,
		},
	}

	type args struct {
		defaultActive bool
		objects       []runtime.Object
	}
	type want struct {
		objects []runtime.Object
		err     error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"EmptyInput": {
			reason: "Should return empty slice for empty input",
			args: args{
				defaultActive: false,
				objects:       []runtime.Object{},
			},
			want: want{
				objects: []runtime.Object{},
			},
		},
		"NonCRDObject": {
			reason: "Should leave non-CRD objects unchanged",
			args: args{
				defaultActive: false,
				objects:       []runtime.Object{testNonCRD},
			},
			want: want{
				objects: []runtime.Object{testNonCRD},
			},
		},
		"SingleStructuredCRDInactive": {
			reason: "Should convert single structured CRD to MRD (inactive)",
			args: args{
				defaultActive: false,
				objects:       []runtime.Object{testCRD},
			},
			want: want{
				objects: []runtime.Object{expectedMRDFromCRDInactive},
			},
		},
		"SingleStructuredCRDActive": {
			reason: "Should convert single structured CRD to MRD (active)",
			args: args{
				defaultActive: true,
				objects:       []runtime.Object{testCRD},
			},
			want: want{
				objects: []runtime.Object{expectedMRDFromCRDActive},
			},
		},
		"SingleUnstructuredCRDInactive": {
			reason: "Should convert single unstructured CRD to MRD (inactive)",
			args: args{
				defaultActive: false,
				objects:       []runtime.Object{testUnstructuredCRD},
			},
			want: want{
				objects: []runtime.Object{expectedMRDFromUnstructuredInactive},
			},
		},
		"SingleUnstructuredCRDActive": {
			reason: "Should convert single unstructured CRD to MRD (active)",
			args: args{
				defaultActive: true,
				objects:       []runtime.Object{testUnstructuredCRD},
			},
			want: want{
				objects: []runtime.Object{expectedMRDFromUnstructuredActive},
			},
		},
		"MixedObjectsInactive": {
			reason: "Should convert only CRD objects, leaving others unchanged (inactive)",
			args: args{
				defaultActive: false,
				objects:       []runtime.Object{testCRD, testNonCRD, testUnstructuredCRD},
			},
			want: want{
				objects: []runtime.Object{expectedMRDFromCRDInactive, testNonCRD, expectedMRDFromUnstructuredInactive},
			},
		},
		"MixedObjectsActive": {
			reason: "Should convert only CRD objects, leaving others unchanged (active)",
			args: args{
				defaultActive: true,
				objects:       []runtime.Object{testCRD, testNonCRD, testUnstructuredCRD},
			},
			want: want{
				objects: []runtime.Object{expectedMRDFromCRDActive, testNonCRD, expectedMRDFromUnstructuredActive},
			},
		},
		"MultipleCRDsInactive": {
			reason: "Should convert multiple CRDs to MRDs (inactive)",
			args: args{
				defaultActive: false,
				objects:       []runtime.Object{testCRD, testUnstructuredCRD},
			},
			want: want{
				objects: []runtime.Object{expectedMRDFromCRDInactive, expectedMRDFromUnstructuredInactive},
			},
		},
		"MultipleCRDsActive": {
			reason: "Should convert multiple CRDs to MRDs (active)",
			args: args{
				defaultActive: true,
				objects:       []runtime.Object{testCRD, testUnstructuredCRD},
			},
			want: want{
				objects: []runtime.Object{expectedMRDFromCRDActive, expectedMRDFromUnstructuredActive},
			},
		},
		"InvalidJSON": {
			reason: "Should return error when JSON marshaling fails",
			args: args{
				defaultActive: false,
				objects:       []runtime.Object{&invalidJSONObject{}},
			},
			want: want{
				objects: []runtime.Object{&invalidJSONObject{}},
				// We expect an error but don't care about the exact type since it's wrapped
				err: errors.New("some error"),
			},
		},
		"ProviderConfigCRD": {
			reason: "Should not convert ProviderConfig CRD to MRD",
			args: args{
				defaultActive: false,
				objects:       []runtime.Object{testProviderConfigCRD},
			},
			want: want{
				objects: []runtime.Object{testProviderConfigCRD},
			},
		},
		"ClusterProviderConfigCRD": {
			reason: "Should not convert ClusterProviderConfig CRD to MRD",
			args: args{
				defaultActive: true,
				objects:       []runtime.Object{testClusterProviderConfigCRD},
			},
			want: want{
				objects: []runtime.Object{testClusterProviderConfigCRD},
			},
		},
		"ProviderConfigUsageCRD": {
			reason: "Should not convert ProviderConfigUsage CRD to MRD",
			args: args{
				defaultActive: false,
				objects:       []runtime.Object{testProviderConfigUsageCRD},
			},
			want: want{
				objects: []runtime.Object{testProviderConfigUsageCRD},
			},
		},
		"MixedWithProviderConfigCRDs": {
			reason: "Should convert MR CRDs but leave provider config CRDs unchanged",
			args: args{
				defaultActive: false,
				objects: []runtime.Object{
					testCRD,                      // Should be converted to MRD
					testProviderConfigCRD,        // Should remain as CRD
					testClusterProviderConfigCRD, // Should remain as CRD
					testNonCRD,                   // Should remain unchanged
				},
			},
			want: want{
				objects: []runtime.Object{
					expectedMRDFromCRDInactive,   // testCRD converted to MRD
					testProviderConfigCRD,        // Left as CRD
					testClusterProviderConfigCRD, // Left as CRD
					testNonCRD,                   // Left unchanged
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			gotObjects, err := CustomToManagedResourceDefinitions(tc.args.defaultActive, tc.args.objects...)

			if tc.want.err != nil {
				// Just check that we got an error when we expected one
				if err == nil {
					t.Errorf("\n%s\nCustomToManagedResourceDefinitions(...): expected error but got none", tc.reason)
				}
			} else {
				if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
					t.Errorf("\n%s\nCustomToManagedResourceDefinitions(...): -want error, +got error:\n%s", tc.reason, diff)
				}
			}
			if diff := cmp.Diff(tc.want.objects, gotObjects); diff != "" {
				t.Errorf("\n%s\nCustomToManagedResourceDefinitions(...): -want objects, +got objects:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestConvertCRDToMRD(t *testing.T) {
	type args struct {
		defaultActive bool
		in            map[string]any
	}
	type want struct {
		out *v1alpha1.ManagedResourceDefinition
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"BasicCRDInactive": {
			reason: "Should convert basic CRD to MRD format (inactive)",
			args: args{
				defaultActive: false,
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
				out: &v1alpha1.ManagedResourceDefinition{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apiextensions.crossplane.io/v1alpha1",
						Kind:       "ManagedResourceDefinition",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "buckets.example.org",
					},
					Spec: v1alpha1.ManagedResourceDefinitionSpec{
						CustomResourceDefinitionSpec: v1alpha1.CustomResourceDefinitionSpec{
							Group: "example.org",
							Names: extv1.CustomResourceDefinitionNames{
								Kind:   "Bucket",
								Plural: "buckets",
							},
						},
					},
				},
			},
		},
		"BasicCRDActive": {
			reason: "Should convert basic CRD to MRD format (active)",
			args: args{
				defaultActive: true,
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
				out: &v1alpha1.ManagedResourceDefinition{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apiextensions.crossplane.io/v1alpha1",
						Kind:       "ManagedResourceDefinition",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "buckets.example.org",
					},
					Spec: v1alpha1.ManagedResourceDefinitionSpec{
						CustomResourceDefinitionSpec: v1alpha1.CustomResourceDefinitionSpec{
							Group: "example.org",
							Names: extv1.CustomResourceDefinitionNames{
								Kind:   "Bucket",
								Plural: "buckets",
							},
						},
						State: v1alpha1.ManagedResourceDefinitionActive,
					},
				},
			},
		},
		"CRDWithComplexSpecActive": {
			reason: "Should convert CRD with complex spec to MRD, preserving all fields (active)",
			args: args{
				defaultActive: true,
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
				out: &v1alpha1.ManagedResourceDefinition{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apiextensions.crossplane.io/v1alpha1",
						Kind:       "ManagedResourceDefinition",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "databases.example.org",
						Namespace: "test-namespace",
						Labels: map[string]string{
							"app": "test",
						},
					},
					Spec: v1alpha1.ManagedResourceDefinitionSpec{
						CustomResourceDefinitionSpec: v1alpha1.CustomResourceDefinitionSpec{
							Group: "example.org",
							Names: extv1.CustomResourceDefinitionNames{
								Kind:     "Database",
								Plural:   "databases",
								Singular: "database",
							},
							Scope: "Namespaced",
							Versions: []v1alpha1.CustomResourceDefinitionVersion{
								{
									Name:    "v1",
									Served:  true,
									Storage: true,
									Schema: &v1alpha1.CustomResourceValidation{
										OpenAPIV3Schema: runtime.RawExtension{
											Raw: []byte(`{"properties":{"spec":{"type":"object"}},"type":"object"}`),
										},
									},
								},
							},
						},
						State: v1alpha1.ManagedResourceDefinitionActive,
					},
				},
			},
		},
		"MinimalCRDInactive": {
			reason: "Should convert minimal CRD to MRD (inactive)",
			args: args{
				defaultActive: false,
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
				out: &v1alpha1.ManagedResourceDefinition{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apiextensions.crossplane.io/v1alpha1",
						Kind:       "ManagedResourceDefinition",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "minimal.example.org",
					},
				},
			},
		},
		"MinimalCRDActive": {
			reason: "Should convert minimal CRD to MRD (active)",
			args: args{
				defaultActive: true,
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
				out: &v1alpha1.ManagedResourceDefinition{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apiextensions.crossplane.io/v1alpha1",
						Kind:       "ManagedResourceDefinition",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "minimal.example.org",
					},
					Spec: v1alpha1.ManagedResourceDefinitionSpec{
						State: v1alpha1.ManagedResourceDefinitionActive,
					},
				},
			},
		},
		"EmptyInputActive": {
			reason: "Should handle empty input gracefully (active)",
			args: args{
				defaultActive: true,
				in:            map[string]any{},
			},
			want: want{
				out: &v1alpha1.ManagedResourceDefinition{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apiextensions.crossplane.io/v1alpha1",
						Kind:       "ManagedResourceDefinition",
					},
					Spec: v1alpha1.ManagedResourceDefinitionSpec{
						State: v1alpha1.ManagedResourceDefinitionActive,
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := convertCRDToMRD(tc.args.defaultActive, tc.args.in)

			if tc.want.err != nil {
				// Just check that we got an error when we expected one
				if err == nil {
					t.Errorf("\n%s\nconvertCRDToMRD(...): expected error but got none", tc.reason)
				}
			} else {
				if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
					t.Errorf("\n%s\nconvertCRDToMRD(...): -want error, +got error:\n%s", tc.reason, diff)
				}
			}
			if diff := cmp.Diff(tc.want.out, got); diff != "" {
				t.Errorf("\n%s\nconvertCRDToMRD(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
