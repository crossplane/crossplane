/*
Copyright 2023 The Crossplane Authors.
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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

func withReadinessChecks(index int, rcs ...v1.ReadinessCheck) compositionBuilderOption {
	return func(c *v1.Composition) {
		c.Spec.Resources[index].ReadinessChecks = rcs
	}
}

func TestValidateReadinessCheck(t *testing.T) {
	type args struct {
		comp    *v1.Composition
		gkToCRD map[schema.GroupKind]apiextensions.CustomResourceDefinition
	}
	type want struct {
		errs field.ErrorList
	}
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "should accept empty readiness check",
			args: args{
				comp:    buildDefaultComposition(t, v1.SchemaAwareCompositionValidationModeLoose, nil),
				gkToCRD: defaultGKToCRDs(),
			},
			want: want{
				errs: nil,
			},
		},
		{
			name: "should accept valid readiness check - none type",
			args: args{
				comp: buildDefaultComposition(t, v1.SchemaAwareCompositionValidationModeLoose, nil, withReadinessChecks(
					0,
					v1.ReadinessCheck{
						Type: v1.ReadinessCheckTypeNone,
					},
				)),
				gkToCRD: defaultGKToCRDs(),
			},
			want: want{
				errs: nil,
			},
		},
		{
			name: "should accept valid readiness check - nonEmpty type",
			args: args{
				comp: buildDefaultComposition(t, v1.SchemaAwareCompositionValidationModeLoose, nil, withReadinessChecks(
					0,
					v1.ReadinessCheck{
						Type:      v1.ReadinessCheckTypeNonEmpty,
						FieldPath: "spec.someOtherField",
					},
				)),
				gkToCRD: defaultGKToCRDs(),
			},
			want: want{
				errs: nil,
			},
		},
		{
			name: "should accept valid readiness check - matchTrue type",
			args: args{
				comp: buildDefaultComposition(t, v1.SchemaAwareCompositionValidationModeLoose, nil, withReadinessChecks(
					0,
					v1.ReadinessCheck{
						Type:      v1.ReadinessCheckTypeMatchTrue,
						FieldPath: "spec.someOtherField",
					},
				)),
				gkToCRD: buildGkToCRDs(
					defaultManagedCrdBuilder().withOption(func(crd *extv1.CustomResourceDefinition) {
						crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"].Properties["someOtherField"] = extv1.JSONSchemaProps{
							Type: "boolean",
						}
					}).build()),
			},
			want: want{
				errs: nil,
			},
		},
		{
			name: "should accept valid readiness check - matchFalse type",
			args: args{
				comp: buildDefaultComposition(t, v1.SchemaAwareCompositionValidationModeLoose, nil, withReadinessChecks(
					0,
					v1.ReadinessCheck{
						Type:      v1.ReadinessCheckTypeMatchFalse,
						FieldPath: "spec.someOtherField",
					},
				)),
				gkToCRD: buildGkToCRDs(
					defaultManagedCrdBuilder().withOption(func(crd *extv1.CustomResourceDefinition) {
						crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"].Properties["someOtherField"] = extv1.JSONSchemaProps{
							Type: "boolean",
						}
					}).build()),
			},
			want: want{
				errs: nil,
			},
		},
		{
			name: "should accept valid readiness check - matchString type",
			args: args{
				comp: buildDefaultComposition(t, v1.SchemaAwareCompositionValidationModeLoose, nil, withReadinessChecks(
					0,
					v1.ReadinessCheck{
						Type:        v1.ReadinessCheckTypeMatchString,
						MatchString: "bob",
						FieldPath:   "spec.someOtherField",
					},
				)),
				gkToCRD: defaultGKToCRDs(),
			},
			want: want{
				errs: nil,
			},
		},
		{
			name: "should reject invalid readiness check - matchInteger type",
			args: args{
				comp: buildDefaultComposition(t, v1.SchemaAwareCompositionValidationModeLoose, nil, withReadinessChecks(
					0,
					v1.ReadinessCheck{
						Type:         v1.ReadinessCheckTypeMatchInteger,
						MatchInteger: 0,
						FieldPath:    "spec.someField",
					},
				)),
				gkToCRD: buildGkToCRDs(
					defaultManagedCrdBuilder().withOption(func(crd *extv1.CustomResourceDefinition) {
						crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"].Properties["someField"] = extv1.JSONSchemaProps{
							Type: "string",
						}
					}).build()),
			},
			want: want{
				errs: field.ErrorList{
					{
						Type:     field.ErrorTypeInvalid,
						Field:    "spec.resources[0].readinessCheck[0].fieldPath",
						BadValue: "spec.someField",
					},
				},
			},
		},
		{
			name: "should accept valid readiness check - matchInteger type",
			args: args{
				comp: buildDefaultComposition(t, v1.SchemaAwareCompositionValidationModeLoose, nil, withReadinessChecks(
					0,
					v1.ReadinessCheck{
						Type:         v1.ReadinessCheckTypeMatchInteger,
						MatchInteger: 15,
						FieldPath:    "spec.someField",
					},
				)),
				gkToCRD: buildGkToCRDs(
					defaultManagedCrdBuilder().withOption(func(crd *extv1.CustomResourceDefinition) {
						crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"].Properties["someField"] = extv1.JSONSchemaProps{
							Type: "integer",
						}
					}).build()),
			},
			want: want{
				errs: nil,
			},
		},
		{
			name: "should reject invalid readiness check - matchInteger type - type mismatch",
			args: args{
				comp: buildDefaultComposition(t, v1.SchemaAwareCompositionValidationModeLoose, nil, withReadinessChecks(
					0,
					v1.ReadinessCheck{
						Type:         v1.ReadinessCheckTypeMatchInteger,
						MatchInteger: 10,
						FieldPath:    "spec.someField",
					},
				)),
				gkToCRD: buildGkToCRDs(
					defaultManagedCrdBuilder().withOption(func(crd *extv1.CustomResourceDefinition) {
						crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"].Properties["someField"] = extv1.JSONSchemaProps{
							Type: "string",
						}
					}).build()),
			},
			want: want{
				errs: field.ErrorList{
					{
						Type:     field.ErrorTypeInvalid,
						Field:    "spec.resources[0].readinessCheck[0].fieldPath",
						BadValue: "spec.someField",
					},
				},
			},
		},
		{
			name: "should reject invalid readiness check - matchInteger type - type mismatch - multiple versions",
			args: args{
				comp: buildDefaultComposition(t, v1.SchemaAwareCompositionValidationModeLoose, nil, withReadinessChecks(
					0,
					v1.ReadinessCheck{
						Type:         v1.ReadinessCheckTypeMatchInteger,
						MatchInteger: 10,
						FieldPath:    "spec.someField",
					},
				)),
				gkToCRD: buildGkToCRDs(
					defaultManagedCrdBuilder().withOption(func(crd *extv1.CustomResourceDefinition) {
						crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"].Properties["someField"] = extv1.JSONSchemaProps{
							Type: "string",
						}
						alpha := crd.Spec.Versions[0].DeepCopy()
						alpha.Name = "v1alpha1"
						crd.Spec.Versions = append(crd.Spec.Versions, *alpha)
						crd.Spec.Versions[1].Schema.OpenAPIV3Schema.Properties["spec"].Properties["someField"] = extv1.JSONSchemaProps{
							Type: "integer",
						}
					}).build()),
			},
			want: want{
				errs: field.ErrorList{
					{
						Type:     field.ErrorTypeInvalid,
						Field:    "spec.resources[0].readinessCheck[0].fieldPath",
						BadValue: "spec.someField",
					},
				},
			},
		},
		{
			name: "should accept valid readiness check - matchInteger type - free object allowed",
			args: args{
				comp: buildDefaultComposition(t, v1.SchemaAwareCompositionValidationModeLoose, nil, withReadinessChecks(
					0,
					v1.ReadinessCheck{
						Type:         v1.ReadinessCheckTypeMatchInteger,
						MatchInteger: 10,
						FieldPath:    "status.atProvider.manifest.status.readyReplicas",
					},
				)),
				gkToCRD: buildGkToCRDs(
					defaultManagedCrdBuilder().withOption(func(crd *extv1.CustomResourceDefinition) {
						crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["status"] = extv1.JSONSchemaProps{
							Type: "object",
							Properties: map[string]extv1.JSONSchemaProps{
								"atProvider": {
									Type: "object",
									Properties: map[string]extv1.JSONSchemaProps{
										"manifest": {
											Type:                   "object",
											XPreserveUnknownFields: ptr.To(true),
										},
									},
								},
							},
						}
					}).build()),
			},
			want: want{
				errs: nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := NewValidator(WithCRDGetterFromMap(tt.args.gkToCRD))
			if err != nil {
				t.Fatalf("NewValidator() error = %v", err)
			}
			got := v.validateReadinessChecksWithSchemas(context.TODO(), tt.args.comp)
			if diff := cmp.Diff(tt.want.errs, got, sortFieldErrors(), cmpopts.IgnoreFields(field.Error{}, "Detail")); diff != "" {
				t.Errorf("validateReadinessChecksWithSchemas(...) = -want, +got\n%s\n", diff)
			}
		})
	}
}
