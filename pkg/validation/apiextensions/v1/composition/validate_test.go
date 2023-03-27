/*
Copyright 2023 the Crossplane Authors.

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

package composition

import (
	"context"
	"testing"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

// TODO(phisco): move to validate actual paths of errors instead of wantErr being just a bool
func TestValidateComposition(t *testing.T) {
	type args struct {
		comp      *v1.Composition
		gvkToCRDs map[schema.GroupVersionKind]apiextensions.CustomResourceDefinition
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "Should reject a Composition if no CRDs are available",
			wantErr: true,
			args: args{
				comp:      buildDefaultComposition(t, v1.CompositionValidationModeStrict, map[string]any{"someOtherField": "test"}),
				gvkToCRDs: nil,
			},
		}, {
			name:    "Should accept a valid Composition if all CRDs are available",
			wantErr: false,
			args: args{
				gvkToCRDs: defaultGVKToCRDs(),
				comp:      buildDefaultComposition(t, v1.CompositionValidationModeStrict, map[string]any{"someOtherField": "test"}),
			},
		},
		{
			name:    "Should accept a Composition not defining a required field in a resource if all CRDs are available",
			wantErr: false,
			args: args{
				gvkToCRDs: defaultGVKToCRDs(),
				comp:      buildDefaultComposition(t, v1.CompositionValidationModeStrict, nil),
			},
		},
		{
			name:    "Should accept a Composition with a required field defined only by a patch if all CRDs are available",
			wantErr: false,
			args: args{
				gvkToCRDs: defaultGVKToCRDs(),
				comp: buildDefaultComposition(t, v1.CompositionValidationModeStrict, nil, withPatches(0, v1.Patch{
					Type:          v1.PatchTypeFromCompositeFieldPath,
					FromFieldPath: toPointer("spec.someField"),
					ToFieldPath:   toPointer("spec.someOtherField"),
				})),
			},
		}, {
			name:    "Should reject a Composition with a patch using a field not allowed by the the Composite resource, if all CRDs are found",
			wantErr: true,
			args: args{
				gvkToCRDs: defaultGVKToCRDs(),
				comp: buildDefaultComposition(t, v1.CompositionValidationModeStrict, nil, withPatches(0, v1.Patch{
					Type:          v1.PatchTypeFromCompositeFieldPath,
					FromFieldPath: toPointer("spec.someWrongField"),
					ToFieldPath:   toPointer("spec.someOtherField"),
				})),
			},
		}, {
			name:    "Should reject a Composition with a patch using a field not allowed by the schema of the Managed resource, if all CRDs are found",
			wantErr: true,
			args: args{
				gvkToCRDs: defaultGVKToCRDs(),
				comp: buildDefaultComposition(t, v1.CompositionValidationModeStrict, map[string]any{"someOtherField": "test"}, withPatches(0, v1.Patch{
					Type:          v1.PatchTypeFromCompositeFieldPath,
					FromFieldPath: toPointer("spec.someField"),
					ToFieldPath:   toPointer("spec.soapis/apiextensions/v1/composition_types.go:31meOtherWrongField"),
				})),
			},
		}, {
			name:    "Should reject a Composition with a patch between two different types, if all CRDs are found",
			wantErr: true,
			args: args{
				gvkToCRDs: buildGvkToCRDs(
					defaultCompositeCrdBuilder().withOption(func(crd *extv1.CustomResourceDefinition) {
						crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"].Properties["someField"] = extv1.JSONSchemaProps{
							Type: "integer",
						}
					}).build(),
					defaultManagedCrdBuilder().build(),
				),
				comp: buildDefaultComposition(t, v1.CompositionValidationModeStrict, nil, withPatches(0, v1.Patch{
					Type:          v1.PatchTypeFromCompositeFieldPath,
					FromFieldPath: toPointer("spec.someField"),
					ToFieldPath:   toPointer("spec.someOtherField"),
				})),
			},
		}, {
			name:    "Should reject a Composition with a math transformation resulting in the wrong final type, if validation mode is strict and all CRDs are found",
			wantErr: true,
			args: args{
				gvkToCRDs: defaultGVKToCRDs(),
				comp: buildDefaultComposition(t, v1.CompositionValidationModeLoose, nil, withPatches(0, v1.Patch{
					Type:          v1.PatchTypeFromCompositeFieldPath,
					FromFieldPath: toPointer("spec.someField"),
					ToFieldPath:   toPointer("spec.someOtherField"),
					Transforms: []v1.Transform{{
						Type: v1.TransformTypeMath,
						Math: &v1.MathTransform{
							Multiply: toPointer(int64(2)),
						},
					}},
				})),
			},
		},
		{
			name:    "Should reject a Composition with a convert transformation resulting in the wrong final type, if all CRDs are found",
			wantErr: true,
			args: args{
				gvkToCRDs: defaultGVKToCRDs(),
				comp: buildDefaultComposition(t, v1.CompositionValidationModeLoose, nil, withPatches(0, v1.Patch{
					Type:          v1.PatchTypeFromCompositeFieldPath,
					FromFieldPath: toPointer("spec.someField"),
					ToFieldPath:   toPointer("spec.someOtherField"),
					Transforms: []v1.Transform{{
						Type: v1.TransformTypeConvert,
						Convert: &v1.ConvertTransform{
							ToType: "int64",
						},
					}},
				})),
			},
		},
		{
			name: "Should accept a Composition with a combine patch, if all CRDs are found",
			args: args{
				gvkToCRDs: buildGvkToCRDs(
					defaultCompositeCrdBuilder().withOption(func(crd *extv1.CustomResourceDefinition) {
						spec := crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"]
						spec.Properties["someOtherOtherField"] = extv1.JSONSchemaProps{
							Type: "string",
						}

						spec.Required = append(spec.Required,
							"someOtherOtherField")
						crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"] = spec
					}).build(),
					defaultManagedCrdBuilder().build(),
				),
				comp: buildDefaultComposition(t, v1.CompositionValidationModeLoose, nil, withPatches(0, v1.Patch{
					Type: v1.PatchTypeCombineFromComposite,
					Combine: &v1.Combine{
						Variables: []v1.CombineVariable{
							{
								FromFieldPath: "spec.someField",
							},
							{
								FromFieldPath: "spec.someOtherOtherField",
							},
						},
						Strategy: v1.CombineStrategyString,
						String: &v1.StringCombine{
							Format: "%s-%s",
						},
					},
					ToFieldPath: toPointer("spec.someOtherField"),
				})),
			},
		},
		{
			name:    "Should reject a Composition with a combine patch with mismatched required fields, if all CRDs are found",
			wantErr: true,
			args: args{
				gvkToCRDs: buildGvkToCRDs(
					defaultCompositeCrdBuilder().withOption(func(crd *extv1.CustomResourceDefinition) {
						spec := crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"]
						spec.Properties["someNonReqField"] = extv1.JSONSchemaProps{
							Type: "string",
						}
					}).build(),
					defaultManagedCrdBuilder().build(),
				),
				comp: buildDefaultComposition(t, v1.CompositionValidationModeLoose, nil, withPatches(0, v1.Patch{
					Type: v1.PatchTypeCombineFromComposite,
					Combine: &v1.Combine{
						Variables: []v1.CombineVariable{
							{
								FromFieldPath: "spec.someField",
							},
							{
								FromFieldPath: "spec.someNonReqField",
							},
						},
						Strategy: v1.CombineStrategyString,
						String: &v1.StringCombine{
							Format: "%s-%s",
						},
					},
					ToFieldPath: toPointer("spec.someOtherField"),
				})),
			},
		},
		{
			name:    "Should reject a Composition with a combine patch with missing fields, if validation mode is strict and all CRDs are found",
			wantErr: true,
			args: args{
				gvkToCRDs: defaultGVKToCRDs(),
				comp: buildDefaultComposition(t, v1.CompositionValidationModeStrict, nil, withPatches(0, v1.Patch{
					Type: v1.PatchTypeCombineFromComposite,
					Combine: &v1.Combine{
						Variables: []v1.CombineVariable{
							{
								FromFieldPath: "spec.someField",
							},
							{
								FromFieldPath: "spec.someNonDefinedField",
							},
						},
						Strategy: v1.CombineStrategyString,
						String: &v1.StringCombine{
							Format: "%s-%s",
						},
					},
					ToFieldPath: toPointer("spec.someOtherField"),
				})),
			},
		},
		{
			name:    "Should accept Composition using an EnvironmentConfig related PatchType, if all CRDs are found",
			wantErr: false,
			args: args{
				gvkToCRDs: defaultGVKToCRDs(),
				comp: buildDefaultComposition(t, v1.CompositionValidationModeLoose, nil, withPatches(0, v1.Patch{
					Type:          v1.PatchTypeFromEnvironmentFieldPath,
					FromFieldPath: toPointer("spec.someField"),
					ToFieldPath:   toPointer("spec.someOtherField"),
				})),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := NewValidator(WithCRDGetterFromMap(tt.args.gvkToCRDs))
			if err != nil {
				t.Fatalf("NewValidator() error = %v", err)
			}
			if err := v.Validate(context.TODO(), tt.args.comp); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
