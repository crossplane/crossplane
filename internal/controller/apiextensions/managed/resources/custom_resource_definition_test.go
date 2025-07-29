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

package resources

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
)

func TestEmptyCustomResourceDefinition(t *testing.T) {
	type args struct {
		mrd *v1alpha1.ManagedResourceDefinition
	}
	type want struct {
		crd *extv1.CustomResourceDefinition
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"BasicMRD": {
			reason: "Should create an empty CRD with the same name as the MRD",
			args: args{
				mrd: &v1alpha1.ManagedResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-mrd",
					},
				},
			},
			want: want{
				crd: &extv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-mrd",
					},
				},
			},
		},
		"MRDWithNamespace": {
			reason: "Should create an empty CRD with the same name as the MRD, ignoring namespace",
			args: args{
				mrd: &v1alpha1.ManagedResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "namespaced-mrd",
						Namespace: "some-namespace",
					},
				},
			},
			want: want{
				crd: &extv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "namespaced-mrd",
					},
				},
			},
		},
		"MRDWithComplexName": {
			reason: "Should handle complex names correctly",
			args: args{
				mrd: &v1alpha1.ManagedResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "databases.example.com",
					},
				},
			},
			want: want{
				crd: &extv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "databases.example.com",
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := EmptyCustomResourceDefinition(tc.args.mrd)
			if diff := cmp.Diff(tc.want.crd, got); diff != "" {
				t.Errorf("\n%s\nEmptyCustomResourceDefinition(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestMergeCustomResourceDefinitionInto(t *testing.T) {
	validSchema := &v1alpha1.CustomResourceValidation{
		OpenAPIV3Schema: runtime.RawExtension{
			Raw: []byte(`{"type": "object", "properties": {"spec": {"type": "object"}}}`),
		},
	}
	invalidSchema := &v1alpha1.CustomResourceValidation{
		OpenAPIV3Schema: runtime.RawExtension{
			Raw: []byte(`{invalid json`),
		},
	}

	type args struct {
		mrd *v1alpha1.ManagedResourceDefinition
		crd *extv1.CustomResourceDefinition
	}
	type want struct {
		crd *extv1.CustomResourceDefinition
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"BasicMerge": {
			reason: "Should merge basic CRD spec fields from MRD",
			args: args{
				mrd: &v1alpha1.ManagedResourceDefinition{
					Spec: v1alpha1.ManagedResourceDefinitionSpec{
						CustomResourceDefinitionSpec: v1alpha1.CustomResourceDefinitionSpec{
							Group: "example.com",
							Names: extv1.CustomResourceDefinitionNames{
								Plural:   "databases",
								Singular: "database",
								Kind:     "Database",
							},
							Scope: extv1.ClusterScoped,
							Versions: []v1alpha1.CustomResourceDefinitionVersion{
								{
									Name:    "v1",
									Served:  true,
									Storage: true,
									Schema:  validSchema,
								},
							},
						},
					},
				},
				crd: &extv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "databases.example.com",
					},
				},
			},
			want: want{
				crd: &extv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: "databases.example.com",
					},
					Spec: extv1.CustomResourceDefinitionSpec{
						Group: "example.com",
						Names: extv1.CustomResourceDefinitionNames{
							Plural:   "databases",
							Singular: "database",
							Kind:     "Database",
						},
						Scope: extv1.ClusterScoped,
						Versions: []extv1.CustomResourceDefinitionVersion{
							{
								Name:    "v1",
								Served:  true,
								Storage: true,
								Schema: &extv1.CustomResourceValidation{
									OpenAPIV3Schema: &extv1.JSONSchemaProps{
										Type: "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Type: "object",
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
		"MultipleVersions": {
			reason: "Should handle multiple versions and sort them alphabetically",
			args: args{
				mrd: &v1alpha1.ManagedResourceDefinition{
					Spec: v1alpha1.ManagedResourceDefinitionSpec{
						CustomResourceDefinitionSpec: v1alpha1.CustomResourceDefinitionSpec{
							Group: "example.com",
							Names: extv1.CustomResourceDefinitionNames{
								Plural: "databases",
								Kind:   "Database",
							},
							Scope: extv1.NamespaceScoped,
							Versions: []v1alpha1.CustomResourceDefinitionVersion{
								{
									Name:    "v2",
									Served:  true,
									Storage: false,
									Schema:  validSchema,
								},
								{
									Name:    "v1",
									Served:  true,
									Storage: true,
									Schema:  validSchema,
								},
								{
									Name:    "v1beta1",
									Served:  false,
									Storage: false,
									Schema:  validSchema,
								},
							},
						},
					},
				},
				crd: &extv1.CustomResourceDefinition{},
			},
			want: want{
				crd: &extv1.CustomResourceDefinition{
					Spec: extv1.CustomResourceDefinitionSpec{
						Group: "example.com",
						Names: extv1.CustomResourceDefinitionNames{
							Plural: "databases",
							Kind:   "Database",
						},
						Scope: extv1.NamespaceScoped,
						Versions: []extv1.CustomResourceDefinitionVersion{
							{
								Name:    "v1",
								Served:  true,
								Storage: true,
								Schema: &extv1.CustomResourceValidation{
									OpenAPIV3Schema: &extv1.JSONSchemaProps{
										Type: "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Type: "object",
											},
										},
									},
								},
							},
							{
								Name:    "v1beta1",
								Served:  false,
								Storage: false,
								Schema: &extv1.CustomResourceValidation{
									OpenAPIV3Schema: &extv1.JSONSchemaProps{
										Type: "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Type: "object",
											},
										},
									},
								},
							},
							{
								Name:    "v2",
								Served:  true,
								Storage: false,
								Schema: &extv1.CustomResourceValidation{
									OpenAPIV3Schema: &extv1.JSONSchemaProps{
										Type: "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Type: "object",
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
		"WithConversion": {
			reason: "Should handle conversion settings",
			args: args{
				mrd: &v1alpha1.ManagedResourceDefinition{
					Spec: v1alpha1.ManagedResourceDefinitionSpec{
						CustomResourceDefinitionSpec: v1alpha1.CustomResourceDefinitionSpec{
							Group: "example.com",
							Names: extv1.CustomResourceDefinitionNames{
								Plural: "databases",
								Kind:   "Database",
							},
							Scope: extv1.ClusterScoped,
							Conversion: &extv1.CustomResourceConversion{
								Strategy: extv1.WebhookConverter,
							},
							Versions: []v1alpha1.CustomResourceDefinitionVersion{
								{
									Name:    "v1",
									Served:  true,
									Storage: true,
									Schema:  validSchema,
								},
							},
						},
					},
				},
				crd: &extv1.CustomResourceDefinition{},
			},
			want: want{
				crd: &extv1.CustomResourceDefinition{
					Spec: extv1.CustomResourceDefinitionSpec{
						Group: "example.com",
						Names: extv1.CustomResourceDefinitionNames{
							Plural: "databases",
							Kind:   "Database",
						},
						Scope: extv1.ClusterScoped,
						Conversion: &extv1.CustomResourceConversion{
							Strategy: extv1.WebhookConverter,
						},
						Versions: []extv1.CustomResourceDefinitionVersion{
							{
								Name:    "v1",
								Served:  true,
								Storage: true,
								Schema: &extv1.CustomResourceValidation{
									OpenAPIV3Schema: &extv1.JSONSchemaProps{
										Type: "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Type: "object",
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
		"WithDeprecation": {
			reason: "Should handle deprecated versions with warning",
			args: args{
				mrd: &v1alpha1.ManagedResourceDefinition{
					Spec: v1alpha1.ManagedResourceDefinitionSpec{
						CustomResourceDefinitionSpec: v1alpha1.CustomResourceDefinitionSpec{
							Group: "example.com",
							Names: extv1.CustomResourceDefinitionNames{
								Plural: "databases",
								Kind:   "Database",
							},
							Scope: extv1.ClusterScoped,
							Versions: []v1alpha1.CustomResourceDefinitionVersion{
								{
									Name:               "v1",
									Served:             true,
									Storage:            true,
									Deprecated:         true,
									DeprecationWarning: ptr.To("This version is deprecated"),
									Schema:             validSchema,
								},
							},
						},
					},
				},
				crd: &extv1.CustomResourceDefinition{},
			},
			want: want{
				crd: &extv1.CustomResourceDefinition{
					Spec: extv1.CustomResourceDefinitionSpec{
						Group: "example.com",
						Names: extv1.CustomResourceDefinitionNames{
							Plural: "databases",
							Kind:   "Database",
						},
						Scope: extv1.ClusterScoped,
						Versions: []extv1.CustomResourceDefinitionVersion{
							{
								Name:               "v1",
								Served:             true,
								Storage:            true,
								Deprecated:         true,
								DeprecationWarning: ptr.To("This version is deprecated"),
								Schema: &extv1.CustomResourceValidation{
									OpenAPIV3Schema: &extv1.JSONSchemaProps{
										Type: "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Type: "object",
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
		"WithSubresources": {
			reason: "Should handle subresources",
			args: args{
				mrd: &v1alpha1.ManagedResourceDefinition{
					Spec: v1alpha1.ManagedResourceDefinitionSpec{
						CustomResourceDefinitionSpec: v1alpha1.CustomResourceDefinitionSpec{
							Group: "example.com",
							Names: extv1.CustomResourceDefinitionNames{
								Plural: "databases",
								Kind:   "Database",
							},
							Scope: extv1.ClusterScoped,
							Versions: []v1alpha1.CustomResourceDefinitionVersion{
								{
									Name:    "v1",
									Served:  true,
									Storage: true,
									Schema:  validSchema,
									Subresources: &extv1.CustomResourceSubresources{
										Status: &extv1.CustomResourceSubresourceStatus{},
									},
								},
							},
						},
					},
				},
				crd: &extv1.CustomResourceDefinition{},
			},
			want: want{
				crd: &extv1.CustomResourceDefinition{
					Spec: extv1.CustomResourceDefinitionSpec{
						Group: "example.com",
						Names: extv1.CustomResourceDefinitionNames{
							Plural: "databases",
							Kind:   "Database",
						},
						Scope: extv1.ClusterScoped,
						Versions: []extv1.CustomResourceDefinitionVersion{
							{
								Name:    "v1",
								Served:  true,
								Storage: true,
								Schema: &extv1.CustomResourceValidation{
									OpenAPIV3Schema: &extv1.JSONSchemaProps{
										Type: "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Type: "object",
											},
										},
									},
								},
								Subresources: &extv1.CustomResourceSubresources{
									Status: &extv1.CustomResourceSubresourceStatus{},
								},
							},
						},
					},
				},
			},
		},
		"WithAdditionalPrinterColumns": {
			reason: "Should handle additional printer columns",
			args: args{
				mrd: &v1alpha1.ManagedResourceDefinition{
					Spec: v1alpha1.ManagedResourceDefinitionSpec{
						CustomResourceDefinitionSpec: v1alpha1.CustomResourceDefinitionSpec{
							Group: "example.com",
							Names: extv1.CustomResourceDefinitionNames{
								Plural: "databases",
								Kind:   "Database",
							},
							Scope: extv1.ClusterScoped,
							Versions: []v1alpha1.CustomResourceDefinitionVersion{
								{
									Name:    "v1",
									Served:  true,
									Storage: true,
									Schema:  validSchema,
									AdditionalPrinterColumns: []extv1.CustomResourceColumnDefinition{
										{
											Name:        "Status",
											Type:        "string",
											JSONPath:    ".status.phase",
											Description: "Status of the database",
										},
									},
								},
							},
						},
					},
				},
				crd: &extv1.CustomResourceDefinition{},
			},
			want: want{
				crd: &extv1.CustomResourceDefinition{
					Spec: extv1.CustomResourceDefinitionSpec{
						Group: "example.com",
						Names: extv1.CustomResourceDefinitionNames{
							Plural: "databases",
							Kind:   "Database",
						},
						Scope: extv1.ClusterScoped,
						Versions: []extv1.CustomResourceDefinitionVersion{
							{
								Name:    "v1",
								Served:  true,
								Storage: true,
								Schema: &extv1.CustomResourceValidation{
									OpenAPIV3Schema: &extv1.JSONSchemaProps{
										Type: "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Type: "object",
											},
										},
									},
								},
								AdditionalPrinterColumns: []extv1.CustomResourceColumnDefinition{
									{
										Name:        "Status",
										Type:        "string",
										JSONPath:    ".status.phase",
										Description: "Status of the database",
									},
								},
							},
						},
					},
				},
			},
		},
		"WithSelectableFields": {
			reason: "Should handle selectable fields",
			args: args{
				mrd: &v1alpha1.ManagedResourceDefinition{
					Spec: v1alpha1.ManagedResourceDefinitionSpec{
						CustomResourceDefinitionSpec: v1alpha1.CustomResourceDefinitionSpec{
							Group: "example.com",
							Names: extv1.CustomResourceDefinitionNames{
								Plural: "databases",
								Kind:   "Database",
							},
							Scope: extv1.ClusterScoped,
							Versions: []v1alpha1.CustomResourceDefinitionVersion{
								{
									Name:    "v1",
									Served:  true,
									Storage: true,
									Schema:  validSchema,
									SelectableFields: []extv1.SelectableField{
										{
											JSONPath: ".spec.engine",
										},
									},
								},
							},
						},
					},
				},
				crd: &extv1.CustomResourceDefinition{},
			},
			want: want{
				crd: &extv1.CustomResourceDefinition{
					Spec: extv1.CustomResourceDefinitionSpec{
						Group: "example.com",
						Names: extv1.CustomResourceDefinitionNames{
							Plural: "databases",
							Kind:   "Database",
						},
						Scope: extv1.ClusterScoped,
						Versions: []extv1.CustomResourceDefinitionVersion{
							{
								Name:    "v1",
								Served:  true,
								Storage: true,
								Schema: &extv1.CustomResourceValidation{
									OpenAPIV3Schema: &extv1.JSONSchemaProps{
										Type: "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Type: "object",
											},
										},
									},
								},
								SelectableFields: []extv1.SelectableField{
									{
										JSONPath: ".spec.engine",
									},
								},
							},
						},
					},
				},
			},
		},
		"WithPreserveUnknownFields": {
			reason: "Should handle preserve unknown fields",
			args: args{
				mrd: &v1alpha1.ManagedResourceDefinition{
					Spec: v1alpha1.ManagedResourceDefinitionSpec{
						CustomResourceDefinitionSpec: v1alpha1.CustomResourceDefinitionSpec{
							Group: "example.com",
							Names: extv1.CustomResourceDefinitionNames{
								Plural: "databases",
								Kind:   "Database",
							},
							Scope:                 extv1.ClusterScoped,
							PreserveUnknownFields: true,
							Versions: []v1alpha1.CustomResourceDefinitionVersion{
								{
									Name:    "v1",
									Served:  true,
									Storage: true,
									Schema:  validSchema,
								},
							},
						},
					},
				},
				crd: &extv1.CustomResourceDefinition{},
			},
			want: want{
				crd: &extv1.CustomResourceDefinition{
					Spec: extv1.CustomResourceDefinitionSpec{
						Group: "example.com",
						Names: extv1.CustomResourceDefinitionNames{
							Plural: "databases",
							Kind:   "Database",
						},
						Scope:                 extv1.ClusterScoped,
						PreserveUnknownFields: true,
						Versions: []extv1.CustomResourceDefinitionVersion{
							{
								Name:    "v1",
								Served:  true,
								Storage: true,
								Schema: &extv1.CustomResourceValidation{
									OpenAPIV3Schema: &extv1.JSONSchemaProps{
										Type: "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Type: "object",
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
		"InvalidSchemaError": {
			reason: "Should return an error if schema conversion fails",
			args: args{
				mrd: &v1alpha1.ManagedResourceDefinition{
					Spec: v1alpha1.ManagedResourceDefinitionSpec{
						CustomResourceDefinitionSpec: v1alpha1.CustomResourceDefinitionSpec{
							Group: "example.com",
							Names: extv1.CustomResourceDefinitionNames{
								Plural: "databases",
								Kind:   "Database",
							},
							Scope: extv1.ClusterScoped,
							Versions: []v1alpha1.CustomResourceDefinitionVersion{
								{
									Name:    "v1",
									Served:  true,
									Storage: true,
									Schema:  invalidSchema,
								},
							},
						},
					},
				},
				crd: &extv1.CustomResourceDefinition{},
			},
			want: want{
				crd: &extv1.CustomResourceDefinition{
					Spec: extv1.CustomResourceDefinitionSpec{
						Group: "example.com",
						Names: extv1.CustomResourceDefinitionNames{
							Plural: "databases",
							Kind:   "Database",
						},
						Scope:    extv1.ClusterScoped,
						Versions: []extv1.CustomResourceDefinitionVersion{{}},
					},
				},
				err: errors.Wrap(errors.New("invalid character 'i' looking for beginning of object key string"), errParseValidation),
			},
		},
		"EmptyVersions": {
			reason: "Should handle empty versions list",
			args: args{
				mrd: &v1alpha1.ManagedResourceDefinition{
					Spec: v1alpha1.ManagedResourceDefinitionSpec{
						CustomResourceDefinitionSpec: v1alpha1.CustomResourceDefinitionSpec{
							Group: "example.com",
							Names: extv1.CustomResourceDefinitionNames{
								Plural: "databases",
								Kind:   "Database",
							},
							Scope:    extv1.ClusterScoped,
							Versions: []v1alpha1.CustomResourceDefinitionVersion{},
						},
					},
				},
				crd: &extv1.CustomResourceDefinition{},
			},
			want: want{
				crd: &extv1.CustomResourceDefinition{
					Spec: extv1.CustomResourceDefinitionSpec{
						Group: "example.com",
						Names: extv1.CustomResourceDefinitionNames{
							Plural: "databases",
							Kind:   "Database",
						},
						Scope:    extv1.ClusterScoped,
						Versions: []extv1.CustomResourceDefinitionVersion{},
					},
				},
			},
		},
		"MergeIntoPreviousValues": {
			reason: "Should merge into and replace existing CRD values",
			args: args{
				mrd: &v1alpha1.ManagedResourceDefinition{
					Spec: v1alpha1.ManagedResourceDefinitionSpec{
						CustomResourceDefinitionSpec: v1alpha1.CustomResourceDefinitionSpec{
							Group: "newgroup.com",
							Names: extv1.CustomResourceDefinitionNames{
								Plural:   "newdatabases",
								Singular: "newdatabase",
								Kind:     "NewDatabase",
							},
							Scope: extv1.NamespaceScoped,
							Versions: []v1alpha1.CustomResourceDefinitionVersion{
								{
									Name:    "v2",
									Served:  true,
									Storage: true,
									Schema:  validSchema,
								},
							},
							PreserveUnknownFields: true,
						},
					},
				},
				crd: &extv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "olddatabases.oldgroup.com",
						Annotations: map[string]string{"existing": "annotation"},
					},
					Spec: extv1.CustomResourceDefinitionSpec{
						Group: "oldgroup.com",
						Names: extv1.CustomResourceDefinitionNames{
							Plural:   "olddatabases",
							Singular: "olddatabase",
							Kind:     "OldDatabase",
						},
						Scope: extv1.ClusterScoped,
						Versions: []extv1.CustomResourceDefinitionVersion{
							{
								Name:    "v1beta1",
								Served:  false,
								Storage: false,
							},
						},
						PreserveUnknownFields: false,
					},
				},
			},
			want: want{
				crd: &extv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "olddatabases.oldgroup.com",
						Annotations: map[string]string{"existing": "annotation"},
					},
					Spec: extv1.CustomResourceDefinitionSpec{
						Group: "newgroup.com",
						Names: extv1.CustomResourceDefinitionNames{
							Plural:   "newdatabases",
							Singular: "newdatabase",
							Kind:     "NewDatabase",
						},
						Scope: extv1.NamespaceScoped,
						Versions: []extv1.CustomResourceDefinitionVersion{
							{
								Name:    "v2",
								Served:  true,
								Storage: true,
								Schema: &extv1.CustomResourceValidation{
									OpenAPIV3Schema: &extv1.JSONSchemaProps{
										Type: "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Type: "object",
											},
										},
									},
								},
							},
						},
						PreserveUnknownFields: true,
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := MergeCustomResourceDefinitionInto(tc.args.mrd, tc.args.crd)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nMergeCustomResourceDefinitionInto(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.crd, tc.args.crd); diff != "" {
				t.Errorf("\n%s\nMergeCustomResourceDefinitionInto(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestToCustomResourceValidation(t *testing.T) {
	validJSON := `{"type": "object", "properties": {"spec": {"type": "object"}}}`
	invalidJSON := `{invalid json`

	type args struct {
		given *v1alpha1.CustomResourceValidation
	}
	type want struct {
		validation *extv1.CustomResourceValidation
		err        error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ValidSchema": {
			reason: "Should successfully convert valid schema",
			args: args{
				given: &v1alpha1.CustomResourceValidation{
					OpenAPIV3Schema: runtime.RawExtension{
						Raw: []byte(validJSON),
					},
				},
			},
			want: want{
				validation: &extv1.CustomResourceValidation{
					OpenAPIV3Schema: &extv1.JSONSchemaProps{
						Type: "object",
						Properties: map[string]extv1.JSONSchemaProps{
							"spec": {
								Type: "object",
							},
						},
					},
				},
			},
		},
		"ComplexSchema": {
			reason: "Should handle complex schema with multiple properties",
			args: args{
				given: &v1alpha1.CustomResourceValidation{
					OpenAPIV3Schema: runtime.RawExtension{
						Raw: []byte(`{
							"type": "object",
							"properties": {
								"spec": {
									"type": "object",
									"properties": {
										"replicas": {"type": "integer", "minimum": 1},
										"image": {"type": "string"}
									},
									"required": ["image"]
								},
								"status": {
									"type": "object",
									"properties": {
										"phase": {"type": "string"}
									}
								}
							},
							"required": ["spec"]
						}`),
					},
				},
			},
			want: want{
				validation: &extv1.CustomResourceValidation{
					OpenAPIV3Schema: &extv1.JSONSchemaProps{
						Type: "object",
						Properties: map[string]extv1.JSONSchemaProps{
							"spec": {
								Type: "object",
								Properties: map[string]extv1.JSONSchemaProps{
									"replicas": {
										Type:    "integer",
										Minimum: ptr.To(1.0),
									},
									"image": {
										Type: "string",
									},
								},
								Required: []string{"image"},
							},
							"status": {
								Type: "object",
								Properties: map[string]extv1.JSONSchemaProps{
									"phase": {
										Type: "string",
									},
								},
							},
						},
						Required: []string{"spec"},
					},
				},
			},
		},
		"NilValidation": {
			reason: "Should return an error for nil validation",
			args: args{
				given: nil,
			},
			want: want{
				err: errors.New(errCustomResourceValidationNil),
			},
		},
		"InvalidJSON": {
			reason: "Should return an error for invalid JSON in schema",
			args: args{
				given: &v1alpha1.CustomResourceValidation{
					OpenAPIV3Schema: runtime.RawExtension{
						Raw: []byte(invalidJSON),
					},
				},
			},
			want: want{
				err: errors.Wrap(errors.New("invalid character 'i' looking for beginning of object key string"), errParseValidation),
			},
		},
		"EmptySchema": {
			reason: "Should handle empty schema",
			args: args{
				given: &v1alpha1.CustomResourceValidation{
					OpenAPIV3Schema: runtime.RawExtension{
						Raw: []byte(`{}`),
					},
				},
			},
			want: want{
				validation: &extv1.CustomResourceValidation{
					OpenAPIV3Schema: &extv1.JSONSchemaProps{},
				},
			},
		},
		"SchemaWithArrayType": {
			reason: "Should handle schema with array types",
			args: args{
				given: &v1alpha1.CustomResourceValidation{
					OpenAPIV3Schema: runtime.RawExtension{
						Raw: []byte(`{
							"type": "object",
							"properties": {
								"spec": {
									"type": "object",
									"properties": {
										"items": {
											"type": "array",
											"items": {"type": "string"}
										}
									}
								}
							}
						}`),
					},
				},
			},
			want: want{
				validation: &extv1.CustomResourceValidation{
					OpenAPIV3Schema: &extv1.JSONSchemaProps{
						Type: "object",
						Properties: map[string]extv1.JSONSchemaProps{
							"spec": {
								Type: "object",
								Properties: map[string]extv1.JSONSchemaProps{
									"items": {
										Type: "array",
										Items: &extv1.JSONSchemaPropsOrArray{
											Schema: &extv1.JSONSchemaProps{
												Type: "string",
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
		"SchemaWithEnum": {
			reason: "Should handle schema with enum values",
			args: args{
				given: &v1alpha1.CustomResourceValidation{
					OpenAPIV3Schema: runtime.RawExtension{
						Raw: []byte(`{
							"type": "object",
							"properties": {
								"spec": {
									"type": "object",
									"properties": {
										"size": {
											"type": "string",
											"enum": ["small", "medium", "large"]
										}
									}
								}
							}
						}`),
					},
				},
			},
			want: want{
				validation: &extv1.CustomResourceValidation{
					OpenAPIV3Schema: &extv1.JSONSchemaProps{
						Type: "object",
						Properties: map[string]extv1.JSONSchemaProps{
							"spec": {
								Type: "object",
								Properties: map[string]extv1.JSONSchemaProps{
									"size": {
										Type: "string",
										Enum: []extv1.JSON{
											{Raw: []byte(`"small"`)},
											{Raw: []byte(`"medium"`)},
											{Raw: []byte(`"large"`)},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := toCustomResourceValidation(tc.args.given)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ntoCustomResourceValidation(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.validation, got); diff != "" {
				t.Errorf("\n%s\ntoCustomResourceValidation(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
