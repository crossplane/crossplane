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

package xcrd

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/crossplane/crossplane-runtime/v2/pkg/fieldpath"
)

func TestBreakingChangesError(t *testing.T) {
	type args struct {
		versions []VersionResult
	}

	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoBreakingChanges": {
			reason: "Should return nil when there are no breaking changes",
			args: args{
				versions: []VersionResult{
					{
						Version: "v1",
						Changes: []SchemaChange{
							{
								Type:    ChangeTypeEnumExpanded,
								Message: "enum value added",
							},
						},
					},
				},
			},
			want: want{
				err: nil,
			},
		},
		"SingleBreakingChange": {
			reason: "Should return error with single breaking change",
			args: args{
				versions: []VersionResult{
					{
						Version: "v1",
						Changes: []SchemaChange{
							{
								Path:    fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("field1")},
								Type:    ChangeTypeFieldRemoved,
								Message: "field removed",
							},
						},
					},
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"MultipleBreakingChanges": {
			reason: "Should return error with count of additional breaking changes",
			args: args{
				versions: []VersionResult{
					{
						Version: "v1",
						Changes: []SchemaChange{
							{
								Path:    fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("field1")},
								Type:    ChangeTypeFieldRemoved,
								Message: "field removed",
							},
							{
								Path:    fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("field2")},
								Type:    ChangeTypeTypeChanged,
								Message: "type changed",
							},
						},
					},
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"MultipleVersionsWithBreakingChanges": {
			reason: "Should return error for first version with breaking changes",
			args: args{
				versions: []VersionResult{
					{
						Version: "v1",
						Changes: []SchemaChange{
							{
								Type: ChangeTypeEnumExpanded,
							},
						},
					},
					{
						Version: "v1beta1",
						Changes: []SchemaChange{
							{
								Path:    fieldpath.Segments{fieldpath.Field("spec"), fieldpath.Field("name")},
								Type:    ChangeTypePatternChanged,
								Message: "pattern changed",
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
			got := BreakingChangesError(tc.args.versions...)

			if diff := cmp.Diff(tc.want.err, got, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nBreakingChangesError(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestCompareSchemas(t *testing.T) {
	testComparator := func(path fieldpath.Segments, existing, proposed *extv1.JSONSchemaProps) []SchemaChange {
		if existing.Type != proposed.Type {
			return []SchemaChange{
				{
					Path:    path,
					Type:    ChangeTypeTypeChanged,
					Message: "test type changed",
				},
			}
		}
		return nil
	}

	type args struct {
		existing *extv1.CustomResourceDefinition
		proposed *extv1.CustomResourceDefinition
		opts     []CompareOption
	}

	type want struct {
		results []VersionResult
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"IdenticalSchemas": {
			reason: "Should return no changes when schemas are identical",
			args: args{
				existing: &extv1.CustomResourceDefinition{
					Spec: extv1.CustomResourceDefinitionSpec{
						Versions: []extv1.CustomResourceDefinitionVersion{
							{
								Name: "v1",
								Schema: &extv1.CustomResourceValidation{
									OpenAPIV3Schema: &extv1.JSONSchemaProps{
										Type: "object",
									},
								},
							},
						},
					},
				},
				proposed: &extv1.CustomResourceDefinition{
					Spec: extv1.CustomResourceDefinitionSpec{
						Versions: []extv1.CustomResourceDefinitionVersion{
							{
								Name: "v1",
								Schema: &extv1.CustomResourceValidation{
									OpenAPIV3Schema: &extv1.JSONSchemaProps{
										Type: "object",
									},
								},
							},
						},
					},
				},
				opts: []CompareOption{WithComparators(testComparator)},
			},
			want: want{
				results: []VersionResult{
					{
						Version: "v1",
						Changes: nil,
					},
				},
			},
		},
		"DetectsChangesAtRoot": {
			reason: "Should detect changes at the root schema level",
			args: args{
				existing: &extv1.CustomResourceDefinition{
					Spec: extv1.CustomResourceDefinitionSpec{
						Versions: []extv1.CustomResourceDefinitionVersion{
							{
								Name: "v1",
								Schema: &extv1.CustomResourceValidation{
									OpenAPIV3Schema: &extv1.JSONSchemaProps{
										Type: "object",
									},
								},
							},
						},
					},
				},
				proposed: &extv1.CustomResourceDefinition{
					Spec: extv1.CustomResourceDefinitionSpec{
						Versions: []extv1.CustomResourceDefinitionVersion{
							{
								Name: "v1",
								Schema: &extv1.CustomResourceValidation{
									OpenAPIV3Schema: &extv1.JSONSchemaProps{
										Type: "string",
									},
								},
							},
						},
					},
				},
				opts: []CompareOption{WithComparators(testComparator)},
			},
			want: want{
				results: []VersionResult{
					{
						Version: "v1",
						Changes: []SchemaChange{
							{
								Path:    fieldpath.Segments{},
								Type:    ChangeTypeTypeChanged,
								Message: "test type changed",
							},
						},
					},
				},
			},
		},
		"RecursesIntoProperties": {
			reason: "Should recurse into properties and detect nested changes",
			args: args{
				existing: &extv1.CustomResourceDefinition{
					Spec: extv1.CustomResourceDefinitionSpec{
						Versions: []extv1.CustomResourceDefinitionVersion{
							{
								Name: "v1",
								Schema: &extv1.CustomResourceValidation{
									OpenAPIV3Schema: &extv1.JSONSchemaProps{
										Type: "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Type: "object",
												Properties: map[string]extv1.JSONSchemaProps{
													"field1": {Type: "string"},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				proposed: &extv1.CustomResourceDefinition{
					Spec: extv1.CustomResourceDefinitionSpec{
						Versions: []extv1.CustomResourceDefinitionVersion{
							{
								Name: "v1",
								Schema: &extv1.CustomResourceValidation{
									OpenAPIV3Schema: &extv1.JSONSchemaProps{
										Type: "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"spec": {
												Type: "object",
												Properties: map[string]extv1.JSONSchemaProps{
													"field1": {Type: "integer"},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				opts: []CompareOption{WithComparators(testComparator)},
			},
			want: want{
				results: []VersionResult{
					{
						Version: "v1",
						Changes: []SchemaChange{
							{
								Path: fieldpath.Segments{
									fieldpath.Field("spec"),
									fieldpath.Field("field1"),
								},
								Type:    ChangeTypeTypeChanged,
								Message: "test type changed",
							},
						},
					},
				},
			},
		},
		"RecursesIntoArrayItems": {
			reason: "Should recurse into array items and detect changes",
			args: args{
				existing: &extv1.CustomResourceDefinition{
					Spec: extv1.CustomResourceDefinitionSpec{
						Versions: []extv1.CustomResourceDefinitionVersion{
							{
								Name: "v1",
								Schema: &extv1.CustomResourceValidation{
									OpenAPIV3Schema: &extv1.JSONSchemaProps{
										Type: "object",
										Properties: map[string]extv1.JSONSchemaProps{
											"items": {
												Type: "array",
												Items: &extv1.JSONSchemaPropsOrArray{
													Schema: &extv1.JSONSchemaProps{
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
				proposed: &extv1.CustomResourceDefinition{
					Spec: extv1.CustomResourceDefinitionSpec{
						Versions: []extv1.CustomResourceDefinitionVersion{
							{
								Name: "v1",
								Schema: &extv1.CustomResourceValidation{
									OpenAPIV3Schema: &extv1.JSONSchemaProps{
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
				opts: []CompareOption{WithComparators(testComparator)},
			},
			want: want{
				results: []VersionResult{
					{
						Version: "v1",
						Changes: []SchemaChange{
							{
								Path: fieldpath.Segments{
									fieldpath.Field("items"),
									fieldpath.Field("items"),
								},
								Type:    ChangeTypeTypeChanged,
								Message: "test type changed",
							},
						},
					},
				},
			},
		},
		"SkipsAlphaVersions": {
			reason: "Should skip alpha versions when alpha exemption is enabled",
			args: args{
				existing: &extv1.CustomResourceDefinition{
					Spec: extv1.CustomResourceDefinitionSpec{
						Versions: []extv1.CustomResourceDefinitionVersion{
							{
								Name: "v1alpha1",
								Schema: &extv1.CustomResourceValidation{
									OpenAPIV3Schema: &extv1.JSONSchemaProps{
										Type: "object",
									},
								},
							},
						},
					},
				},
				proposed: &extv1.CustomResourceDefinition{
					Spec: extv1.CustomResourceDefinitionSpec{
						Versions: []extv1.CustomResourceDefinitionVersion{
							{
								Name: "v1alpha1",
								Schema: &extv1.CustomResourceValidation{
									OpenAPIV3Schema: &extv1.JSONSchemaProps{
										Type: "string",
									},
								},
							},
						},
					},
				},
				opts: []CompareOption{
					WithComparators(testComparator),
					WithAlphaExemption(),
				},
			},
			want: want{
				results: []VersionResult{},
			},
		},
		"ComparesMultipleVersions": {
			reason: "Should compare all non-alpha versions independently",
			args: args{
				existing: &extv1.CustomResourceDefinition{
					Spec: extv1.CustomResourceDefinitionSpec{
						Versions: []extv1.CustomResourceDefinitionVersion{
							{
								Name: "v1",
								Schema: &extv1.CustomResourceValidation{
									OpenAPIV3Schema: &extv1.JSONSchemaProps{
										Type: "object",
									},
								},
							},
							{
								Name: "v1beta1",
								Schema: &extv1.CustomResourceValidation{
									OpenAPIV3Schema: &extv1.JSONSchemaProps{
										Type: "object",
									},
								},
							},
						},
					},
				},
				proposed: &extv1.CustomResourceDefinition{
					Spec: extv1.CustomResourceDefinitionSpec{
						Versions: []extv1.CustomResourceDefinitionVersion{
							{
								Name: "v1",
								Schema: &extv1.CustomResourceValidation{
									OpenAPIV3Schema: &extv1.JSONSchemaProps{
										Type: "string",
									},
								},
							},
							{
								Name: "v1beta1",
								Schema: &extv1.CustomResourceValidation{
									OpenAPIV3Schema: &extv1.JSONSchemaProps{
										Type: "integer",
									},
								},
							},
						},
					},
				},
				opts: []CompareOption{WithComparators(testComparator)},
			},
			want: want{
				results: []VersionResult{
					{
						Version: "v1",
						Changes: []SchemaChange{
							{
								Path:    fieldpath.Segments{},
								Type:    ChangeTypeTypeChanged,
								Message: "test type changed",
							},
						},
					},
					{
						Version: "v1beta1",
						Changes: []SchemaChange{
							{
								Path:    fieldpath.Segments{},
								Type:    ChangeTypeTypeChanged,
								Message: "test type changed",
							},
						},
					},
				},
			},
		},
		"IgnoresNewVersions": {
			reason: "Should not compare versions that don't exist in existing CRD",
			args: args{
				existing: &extv1.CustomResourceDefinition{
					Spec: extv1.CustomResourceDefinitionSpec{
						Versions: []extv1.CustomResourceDefinitionVersion{
							{
								Name: "v1",
								Schema: &extv1.CustomResourceValidation{
									OpenAPIV3Schema: &extv1.JSONSchemaProps{
										Type: "object",
									},
								},
							},
						},
					},
				},
				proposed: &extv1.CustomResourceDefinition{
					Spec: extv1.CustomResourceDefinitionSpec{
						Versions: []extv1.CustomResourceDefinitionVersion{
							{
								Name: "v1",
								Schema: &extv1.CustomResourceValidation{
									OpenAPIV3Schema: &extv1.JSONSchemaProps{
										Type: "object",
									},
								},
							},
							{
								Name: "v2",
								Schema: &extv1.CustomResourceValidation{
									OpenAPIV3Schema: &extv1.JSONSchemaProps{
										Type: "string",
									},
								},
							},
						},
					},
				},
				opts: []CompareOption{WithComparators(testComparator)},
			},
			want: want{
				results: []VersionResult{
					{
						Version: "v1",
						Changes: nil,
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := CompareSchemas(tc.args.existing, tc.args.proposed, tc.args.opts...)

			if diff := cmp.Diff(tc.want.results, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("%s\nCompareSchemas(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
