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

package managed

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	"github.com/crossplane/crossplane/apis/v2/apiextensions/v1alpha1"
)

func TestCRDAsUnstructured(t *testing.T) {
	providerRevisionUID := "provider-revision-uid"
	mrdUID := "mrd-uid"

	type args struct {
		mrd *v1alpha1.ManagedResourceDefinition
	}
	type want struct {
		crd *unstructured.Unstructured
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"MinimalMRD": {
			reason: "Should build CRD with minimal required fields",
			args: args{
				mrd: &v1alpha1.ManagedResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "databases.example.com",
						UID:  types.UID(mrdUID),
					},
					Spec: v1alpha1.ManagedResourceDefinitionSpec{
						CustomResourceDefinitionSpec: v1alpha1.CustomResourceDefinitionSpec{
							Group: "example.com",
							Names: extv1.CustomResourceDefinitionNames{
								Kind:   "Database",
								Plural: "databases",
							},
							Scope: extv1.ClusterScoped,
							Versions: []v1alpha1.CustomResourceDefinitionVersion{
								{
									Name:    "v1",
									Served:  true,
									Storage: true,
								},
							},
						},
					},
				},
			},
			want: want{
				crd: &unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "apiextensions.k8s.io/v1",
						"kind":       "CustomResourceDefinition",
						"metadata": map[string]any{
							"name": "databases.example.com",
							"ownerReferences": []any{
								map[string]any{
									"apiVersion": "apiextensions.crossplane.io/v1alpha1",
									"kind":       "ManagedResourceDefinition",
									"name":       "databases.example.com",
									"uid":        mrdUID,
								},
							},
						},
						"spec": map[string]any{
							"group": "example.com",
							"names": map[string]any{
								"kind":   "Database",
								"plural": "databases",
							},
							"scope": "Cluster",
							"versions": []any{
								map[string]any{
									"name":    "v1",
									"served":  true,
									"storage": true,
								},
							},
						},
					},
				},
			},
		},
		"CompleteOptionalFields": {
			reason: "Should include all optional fields when present",
			args: args{
				mrd: &v1alpha1.ManagedResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "databases.example.com",
						UID:  types.UID(mrdUID),
					},
					Spec: v1alpha1.ManagedResourceDefinitionSpec{
						CustomResourceDefinitionSpec: v1alpha1.CustomResourceDefinitionSpec{
							Group: "example.com",
							Names: extv1.CustomResourceDefinitionNames{
								Kind:       "Database",
								Plural:     "databases",
								Singular:   "database",
								ListKind:   "DatabaseList",
								ShortNames: []string{"db"},
								Categories: []string{"all"},
							},
							Scope:                 extv1.NamespaceScoped,
							PreserveUnknownFields: true,
							Conversion:            &extv1.CustomResourceConversion{Strategy: extv1.NoneConverter},
							Versions: []v1alpha1.CustomResourceDefinitionVersion{
								{
									Name:               "v1beta1",
									Served:             true,
									Storage:            false,
									Deprecated:         true,
									DeprecationWarning: ptr.To("v1beta1 is deprecated, use v1 instead"),
									Schema: &v1alpha1.CustomResourceValidation{
										OpenAPIV3Schema: runtime.RawExtension{
											Raw: []byte(`{"type":"object","properties":{"spec":{"type":"object"}}}`),
										},
									},
									Subresources: &extv1.CustomResourceSubresources{
										Status: &extv1.CustomResourceSubresourceStatus{},
										Scale: &extv1.CustomResourceSubresourceScale{
											SpecReplicasPath:   ".spec.replicas",
											StatusReplicasPath: ".status.replicas",
										},
									},
									AdditionalPrinterColumns: []extv1.CustomResourceColumnDefinition{
										{
											Name:     "Ready",
											Type:     "string",
											JSONPath: ".status.conditions[?(@.type=='Ready')].status",
										},
									},
								},
								{
									Name:    "v1",
									Served:  true,
									Storage: true,
								},
							},
						},
					},
				},
			},
			want: want{
				crd: &unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "apiextensions.k8s.io/v1",
						"kind":       "CustomResourceDefinition",
						"metadata": map[string]any{
							"name": "databases.example.com",
							"ownerReferences": []any{
								map[string]any{
									"apiVersion": "apiextensions.crossplane.io/v1alpha1",
									"kind":       "ManagedResourceDefinition",
									"name":       "databases.example.com",
									"uid":        mrdUID,
								},
							},
						},
						"spec": map[string]any{
							"group": "example.com",
							"names": map[string]any{
								"kind":       "Database",
								"plural":     "databases",
								"singular":   "database",
								"listKind":   "DatabaseList",
								"shortNames": []any{"db"},
								"categories": []any{"all"},
							},
							"scope":                 "Namespaced",
							"preserveUnknownFields": true,
							"conversion": map[string]any{
								"strategy": "None",
							},
							"versions": []any{
								map[string]any{
									"name":    "v1",
									"served":  true,
									"storage": true,
								},
								map[string]any{
									"name":               "v1beta1",
									"served":             true,
									"storage":            false,
									"deprecated":         true,
									"deprecationWarning": "v1beta1 is deprecated, use v1 instead",
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
									"subresources": map[string]any{
										"status": map[string]any{},
										"scale": map[string]any{
											"specReplicasPath":   ".spec.replicas",
											"statusReplicasPath": ".status.replicas",
										},
									},
									"additionalPrinterColumns": []any{
										map[string]any{
											"name":     "Ready",
											"type":     "string",
											"jsonPath": ".status.conditions[?(@.type=='Ready')].status",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		"MultipleVersions": {
			reason: "Should sort versions alphabetically",
			args: args{
				mrd: &v1alpha1.ManagedResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "databases.example.com",
						UID:  types.UID(mrdUID),
					},
					Spec: v1alpha1.ManagedResourceDefinitionSpec{
						CustomResourceDefinitionSpec: v1alpha1.CustomResourceDefinitionSpec{
							Group: "example.com",
							Names: extv1.CustomResourceDefinitionNames{
								Kind:   "Database",
								Plural: "databases",
							},
							Scope: extv1.ClusterScoped,
							Versions: []v1alpha1.CustomResourceDefinitionVersion{
								{
									Name:    "v2",
									Served:  true,
									Storage: false,
								},
								{
									Name:    "v1alpha1",
									Served:  true,
									Storage: false,
								},
								{
									Name:    "v1",
									Served:  true,
									Storage: true,
								},
							},
						},
					},
				},
			},
			want: want{
				crd: &unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "apiextensions.k8s.io/v1",
						"kind":       "CustomResourceDefinition",
						"metadata": map[string]any{
							"name": "databases.example.com",
							"ownerReferences": []any{
								map[string]any{
									"apiVersion": "apiextensions.crossplane.io/v1alpha1",
									"kind":       "ManagedResourceDefinition",
									"name":       "databases.example.com",
									"uid":        mrdUID,
								},
							},
						},
						"spec": map[string]any{
							"group": "example.com",
							"names": map[string]any{
								"kind":   "Database",
								"plural": "databases",
							},
							"scope": "Cluster",
							"versions": []any{
								map[string]any{
									"name":    "v1",
									"served":  true,
									"storage": true,
								},
								map[string]any{
									"name":    "v1alpha1",
									"served":  true,
									"storage": false,
								},
								map[string]any{
									"name":    "v2",
									"served":  true,
									"storage": false,
								},
							},
						},
					},
				},
			},
		},

		"MRDWithControllerOwner": {
			reason: "Should propagate controller owner from MRD to CRD",
			args: args{
				mrd: &v1alpha1.ManagedResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "databases.example.com",
						UID:  types.UID(mrdUID),
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         "pkg.crossplane.io/v1",
								Kind:               "ProviderRevision",
								Name:               "provider-example-abc123",
								UID:                types.UID(providerRevisionUID),
								Controller:         ptr.To(true),
								BlockOwnerDeletion: ptr.To(true),
							},
						},
					},
					Spec: v1alpha1.ManagedResourceDefinitionSpec{
						CustomResourceDefinitionSpec: v1alpha1.CustomResourceDefinitionSpec{
							Group: "example.com",
							Names: extv1.CustomResourceDefinitionNames{
								Kind:   "Database",
								Plural: "databases",
							},
							Scope: extv1.ClusterScoped,
							Versions: []v1alpha1.CustomResourceDefinitionVersion{
								{
									Name:    "v1",
									Served:  true,
									Storage: true,
								},
							},
						},
					},
				},
			},
			want: want{
				crd: &unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "apiextensions.k8s.io/v1",
						"kind":       "CustomResourceDefinition",
						"metadata": map[string]any{
							"name": "databases.example.com",
							"ownerReferences": []any{
								map[string]any{
									"apiVersion": "apiextensions.crossplane.io/v1alpha1",
									"kind":       "ManagedResourceDefinition",
									"name":       "databases.example.com",
									"uid":        mrdUID,
								},
								map[string]any{
									"apiVersion":         "pkg.crossplane.io/v1",
									"kind":               "ProviderRevision",
									"name":               "provider-example-abc123",
									"uid":                providerRevisionUID,
									"controller":         true,
									"blockOwnerDeletion": true,
								},
							},
						},
						"spec": map[string]any{
							"group": "example.com",
							"names": map[string]any{
								"kind":   "Database",
								"plural": "databases",
							},
							"scope": "Cluster",
							"versions": []any{
								map[string]any{
									"name":    "v1",
									"served":  true,
									"storage": true,
								},
							},
						},
					},
				},
			},
		},
		"InvalidSchema": {
			reason: "Should return error for invalid JSON schema",
			args: args{
				mrd: &v1alpha1.ManagedResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "databases.example.com",
						UID:  types.UID(mrdUID),
					},
					Spec: v1alpha1.ManagedResourceDefinitionSpec{
						CustomResourceDefinitionSpec: v1alpha1.CustomResourceDefinitionSpec{
							Group: "example.com",
							Names: extv1.CustomResourceDefinitionNames{
								Kind:   "Database",
								Plural: "databases",
							},
							Scope: extv1.ClusterScoped,
							Versions: []v1alpha1.CustomResourceDefinitionVersion{
								{
									Name:    "v1",
									Served:  true,
									Storage: true,
									Schema: &v1alpha1.CustomResourceValidation{
										OpenAPIV3Schema: runtime.RawExtension{
											Raw: []byte(`{invalid json`),
										},
									},
								},
							},
						},
					},
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := CRDAsUnstructured(tc.args.mrd)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nCRDAsUnstructured(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.crd, got); diff != "" {
				t.Errorf("\n%s\nCRDAsUnstructured(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
