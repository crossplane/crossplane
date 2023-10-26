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

func withConnectionDetails(index int, cds ...v1.ConnectionDetail) compositionBuilderOption {
	return func(c *v1.Composition) {
		c.Spec.Resources[index].ConnectionDetails = cds
	}
}

func TestValidateConnectionDetails(t *testing.T) {
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
			name: "should accept empty connection details",
			args: args{
				comp:    buildDefaultComposition(t, v1.SchemaAwareCompositionValidationModeLoose, nil),
				gkToCRD: defaultGKToCRDs(),
			},
			want: want{
				errs: nil,
			},
		},
		{
			name: "should accept valid connection details, unknown type",
			args: args{
				comp: buildDefaultComposition(t, v1.SchemaAwareCompositionValidationModeLoose, nil, withConnectionDetails(
					0,
					v1.ConnectionDetail{
						Type: &[]v1.ConnectionDetailType{v1.ConnectionDetailTypeUnknown}[0],
					},
				)),
				gkToCRD: defaultGKToCRDs(),
			},
			want: want{
				errs: nil,
			},
		},
		{
			name: "should accept valid connection detail specifying a valid fromFieldPath",
			args: args{
				comp: buildDefaultComposition(t, v1.SchemaAwareCompositionValidationModeLoose, nil, withConnectionDetails(
					0,
					v1.ConnectionDetail{
						FromFieldPath: ptr.To("spec.someOtherField"),
					},
				)),
				gkToCRD: defaultGKToCRDs(),
			},
			want: want{
				errs: nil,
			},
		},
		{
			name: "should reject invalid connection detail specifying an invalid fromFieldPath",
			args: args{
				comp: buildDefaultComposition(t, v1.SchemaAwareCompositionValidationModeLoose, nil, withConnectionDetails(
					0,
					v1.ConnectionDetail{
						FromFieldPath: ptr.To("spec.someWrongField"),
					},
					v1.ConnectionDetail{
						FromFieldPath: ptr.To("spec.someField"),
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
						Field:    "spec.resources[0].connectionDetails[0].fromFieldPath",
						BadValue: "spec.someWrongField",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := NewValidator(WithCRDGetterFromMap(tt.args.gkToCRD))
			if err != nil {
				t.Fatalf("NewValidator() error = %v", err)
			}
			got := v.validateConnectionDetailsWithSchemas(context.TODO(), tt.args.comp)
			if diff := cmp.Diff(got, tt.want.errs, sortFieldErrors(), cmpopts.IgnoreFields(field.Error{}, "Detail")); diff != "" {
				t.Errorf("validateConnectionDetailsWithSchemas(...) = -want, +got\n%s\n", diff)
			}
		})
	}
}
