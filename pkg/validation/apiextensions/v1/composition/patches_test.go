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
	_ "embed"
	"encoding/json"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"

	"k8s.io/utils/pointer"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/crossplane/crossplane/pkg/validation/schema"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

var (
	// got running `kubectl get crds -o json openidconnectproviders.iam.aws.crossplane.io  | jq '.spec.versions[0].schema.openAPIV3Schema |del(.. | .description?)'`
	// from provider: xpkg.upbound.io/crossplane-contrib/provider-aws:v0.38.0
	//go:embed fixtures/complex_schema_openidconnectproviders_v1beta1.json
	complexSchemaOpenIDConnectProvidersV1beta1      []byte
	complexSchemaOpenIDConnectProvidersV1beta1Props = toJSONSchemaProps(complexSchemaOpenIDConnectProvidersV1beta1)
)

func toJSONSchemaProps(in []byte) *apiextensions.JSONSchemaProps {
	p := extv1.JSONSchemaProps{}
	err := json.Unmarshal(in, &p)
	if err != nil {
		panic(err)
	}
	out := apiextensions.JSONSchemaProps{}
	if err := extv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(&p, &out, nil); err != nil {
		panic(err)
	}
	return &out
}

func Test_validateTransforms(t *testing.T) {
	type args struct {
		transforms       []v1.Transform
		fromType, toType schema.KnownJSONType
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Should validate empty transforms to the same type successfully",
			args: args{
				transforms: []v1.Transform{},
				fromType:   "string",
				toType:     "string",
			},
		},
		{
			name:    "Should reject empty transforms to a different type",
			wantErr: true,
			args: args{
				transforms: []v1.Transform{},
				fromType:   "string",
				toType:     "integer",
			},
		},
		{
			name:    "Should accept empty transforms to a different type when its integer to number",
			wantErr: false,
			args: args{
				transforms: []v1.Transform{},
				fromType:   "integer",
				toType:     "number",
			},
		},
		{
			name: "Should validate convert transforms successfully",
			args: args{
				transforms: []v1.Transform{
					{
						Type: v1.TransformTypeConvert,
						Convert: &v1.ConvertTransform{
							ToType: "int64",
						},
					},
				},
				fromType: "string",
				toType:   "integer",
			},
		},
		{
			name: "Should validate convert integer to number transforms successfully",
			args: args{
				transforms: []v1.Transform{
					{
						Type: v1.TransformTypeConvert,
						Convert: &v1.ConvertTransform{
							ToType: "float64",
						},
					},
					{
						Type: v1.TransformTypeConvert,
						Convert: &v1.ConvertTransform{
							ToType: "int64",
						},
					},
				},
				fromType: "string",
				toType:   "number",
			},
		},
		{
			name:    "Should reject convert number to integer transforms",
			wantErr: true,
			args: args{
				transforms: []v1.Transform{
					{
						Type: v1.TransformTypeConvert,
						Convert: &v1.ConvertTransform{
							ToType: "int64",
						},
					},
					{
						Type: v1.TransformTypeConvert,
						Convert: &v1.ConvertTransform{
							ToType: "float64",
						},
					},
				},
				fromType: "string",
				toType:   "integer",
			},
		},
		{
			name: "Should validate multiple convert transforms successfully",
			args: args{
				transforms: []v1.Transform{
					{
						Type: v1.TransformTypeConvert,
						Convert: &v1.ConvertTransform{
							ToType: "int64",
						},
					},
					{
						Type: v1.TransformTypeConvert,
						Convert: &v1.ConvertTransform{
							ToType: "string",
						},
					},
				},
				fromType: "string",
				toType:   "string",
			},
		},
		{
			name:    "Should reject invalid transform types",
			wantErr: true,
			args: args{
				transforms: []v1.Transform{
					{
						Type: v1.TransformType("doesnotexist"),
					},
				},
				fromType: "string",
				toType:   "string",
			},
		},
		{
			name:    "Should return nil if no transforms are provided and the types are the same",
			wantErr: false,
			args: args{
				transforms: nil,
				fromType:   "string",
				toType:     "string",
			},
		},
		{
			name:    "Should return an error if no transforms are provided and the types are not the same",
			wantErr: true,
			args: args{
				transforms: nil,
				fromType:   "string",
				toType:     "integer",
			},
		},
		{
			name:    "Should return nil if no transforms are provided and the types are not the same but the types are integer and number",
			wantErr: false,
			args: args{
				transforms: nil,
				fromType:   "integer",
				toType:     "number",
			},
		},
		{
			name:    "Should return nil if there is no type spec for input and no transforms are provided",
			wantErr: false,
			args: args{
				transforms: nil,
				fromType:   "",
				toType:     "string",
			},
		},
		{
			name:    "Should return nil if there is no type spec for output and no transforms are provided",
			wantErr: false,
			args: args{
				transforms: nil,
				fromType:   "string",
				toType:     "",
			},
		},
		{
			name:    "Should return nil if there are no type spec for input and output and no transforms are provided",
			wantErr: false,
			args: args{
				transforms: nil,
				fromType:   "",
				toType:     "",
			},
		},
		{
			name:    "Should return nil if there are no type spec for input and output and transforms are provided",
			wantErr: false,
			args: args{
				transforms: []v1.Transform{
					{
						Type: v1.TransformTypeConvert,
						Convert: &v1.ConvertTransform{
							ToType: "int64",
						},
					},
				},
				fromType: "",
				toType:   "",
			},
		},
		{
			name:    "Should return an error if there is no type spec for the output, but input is specified and transforms are wrong",
			wantErr: true,
			args: args{
				transforms: []v1.Transform{
					{
						Type: v1.TransformTypeMath,
						Math: &v1.MathTransform{
							Multiply: pointer.Int64(2),
						},
					},
				},
				fromType: "string",
				toType:   "",
			},
		},
		{
			name:    "Should return an error if there is no type spec for the input, but output is specified and transforms are wrong",
			wantErr: true,
			args: args{
				transforms: []v1.Transform{
					{
						Type: v1.TransformTypeMath,
						Math: &v1.MathTransform{
							Multiply: pointer.Int64(2),
						},
					},
				},
				fromType: "",
				toType:   "string",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateIOTypesWithTransforms(tt.args.transforms, tt.args.fromType, tt.args.toType); (err != nil) != tt.wantErr {
				t.Errorf("validateIOTypesWithTransforms() error = %v, wantErrs %v", err, tt.wantErr)
			}
		})
	}
}

func Test_validateFieldPath(t *testing.T) {
	type args struct {
		schema    *apiextensions.JSONSchemaProps
		fieldPath string
	}
	tests := []struct {
		name          string
		args          args
		wantFieldType schema.KnownJSONType
		wantRequired  bool
		wantErr       bool
	}{
		{
			name:          "Should validate a valid field path",
			wantFieldType: "string",
			wantRequired:  false,
			wantErr:       false,
			args: args{
				fieldPath: "spec.forProvider.foo",
				schema: &apiextensions.JSONSchemaProps{
					Properties: map[string]apiextensions.JSONSchemaProps{
						"spec": {
							Properties: map[string]apiextensions.JSONSchemaProps{
								"forProvider": {
									Properties: map[string]apiextensions.JSONSchemaProps{
										"foo": {Type: "string"}}}}}}}},
		},
		{
			name:          "Should validate a valid field path with a field required the whole chain",
			wantFieldType: "string",
			wantRequired:  true,
			wantErr:       false,
			args: args{
				fieldPath: "spec.forProvider.foo",
				schema: &apiextensions.JSONSchemaProps{
					Required: []string{"spec"},
					Properties: map[string]apiextensions.JSONSchemaProps{
						"spec": {
							Required: []string{"forProvider"},
							Properties: map[string]apiextensions.JSONSchemaProps{
								"forProvider": {
									Required: []string{"foo"},
									Properties: map[string]apiextensions.JSONSchemaProps{
										"foo": {Type: "string"}}}}}}}},
		},
		{
			name:          "Should not return that a field is required if it is not the whole chain",
			wantFieldType: "string",
			wantRequired:  false,
			wantErr:       false,
			args: args{
				fieldPath: "spec.forProvider.foo",
				schema: &apiextensions.JSONSchemaProps{
					Required: []string{"spec"},
					Properties: map[string]apiextensions.JSONSchemaProps{
						"spec": {
							Required: []string{"forProvider"},
							Properties: map[string]apiextensions.JSONSchemaProps{
								"forProvider": {
									Properties: map[string]apiextensions.JSONSchemaProps{
										"foo": {Type: "string"}}}}}}}},
		},
		{
			name:    "Should return an error for an invalid field path",
			wantErr: true,
			args: args{
				fieldPath: "spec.forProvider.wrong",
				schema: &apiextensions.JSONSchemaProps{
					Properties: map[string]apiextensions.JSONSchemaProps{
						"spec": {
							Properties: map[string]apiextensions.JSONSchemaProps{
								"forProvider": {
									Properties: map[string]apiextensions.JSONSchemaProps{
										"foo": {Type: "string"}}}}}}}},
		},
		{
			name:          "Should not return an error for an undefined by accepted field path",
			wantErr:       false,
			wantFieldType: "",
			wantRequired:  false,
			args: args{
				fieldPath: "spec.forProvider.wrong",
				schema: &apiextensions.JSONSchemaProps{
					Properties: map[string]apiextensions.JSONSchemaProps{
						"spec": {
							Properties: map[string]apiextensions.JSONSchemaProps{
								"forProvider": {
									Properties: map[string]apiextensions.JSONSchemaProps{
										"foo": {Type: "string"}},
									XPreserveUnknownFields: &[]bool{true}[0],
								}}}}}},
		},
		{
			name:          "Should validate arrays properly",
			wantFieldType: "string",
			wantRequired:  false,
			wantErr:       false,
			args: args{
				fieldPath: "spec.forProvider.foo[0].bar",
				schema: &apiextensions.JSONSchemaProps{
					Properties: map[string]apiextensions.JSONSchemaProps{
						"spec": {
							Properties: map[string]apiextensions.JSONSchemaProps{
								"forProvider": {
									Properties: map[string]apiextensions.JSONSchemaProps{
										"foo": {
											Type: "array",
											Items: &apiextensions.JSONSchemaPropsOrArray{
												Schema: &apiextensions.JSONSchemaProps{
													Properties: map[string]apiextensions.JSONSchemaProps{
														"bar": {Type: "string"}}}}}}}}}}}},
		},
		{
			name:          "Should validate arrays properly with a field not required the whole chain, minimum length 1",
			wantFieldType: "string",
			wantRequired:  false,
			wantErr:       false,
			args: args{
				fieldPath: "spec.forProvider.foo[1].bar",
				schema: &apiextensions.JSONSchemaProps{
					Properties: map[string]apiextensions.JSONSchemaProps{
						"spec": {
							Properties: map[string]apiextensions.JSONSchemaProps{
								"forProvider": {
									Properties: map[string]apiextensions.JSONSchemaProps{
										"foo": {
											Type:     "array",
											MinItems: &[]int64{1}[0],
											Items: &apiextensions.JSONSchemaPropsOrArray{
												Schema: &apiextensions.JSONSchemaProps{
													Required: []string{"bar"},
													Properties: map[string]apiextensions.JSONSchemaProps{
														"bar": {Type: "string"}}}}}}}}}}}},
		},
		{
			name:          "Should validate arrays properly with a field required the whole chain, minimum length 1",
			wantFieldType: "string",
			wantRequired:  true,
			wantErr:       false,
			args: args{
				fieldPath: "spec.forProvider.foo[1].bar",
				schema: &apiextensions.JSONSchemaProps{
					Required: []string{"spec"},
					Properties: map[string]apiextensions.JSONSchemaProps{
						"spec": {
							Required: []string{"forProvider"},
							Properties: map[string]apiextensions.JSONSchemaProps{
								"forProvider": {
									Required: []string{"foo"},
									Properties: map[string]apiextensions.JSONSchemaProps{
										"foo": {
											Type:     "array",
											MinItems: &[]int64{1}[0],
											Items: &apiextensions.JSONSchemaPropsOrArray{
												Schema: &apiextensions.JSONSchemaProps{
													Required: []string{"bar"},
													Properties: map[string]apiextensions.JSONSchemaProps{
														"bar": {Type: "string"}}}}}}}}}}}},
		},
		{
			name:          "Should validate properly with complex schema",
			wantFieldType: "string",
			wantRequired:  false,
			wantErr:       false,
			args: args{
				fieldPath: "spec.forProvider.clientIDList[0]",
				// parse the schema from json
				schema: complexSchemaOpenIDConnectProvidersV1beta1Props,
			},
		},
		{
			name:    "Should error if above max items",
			wantErr: true,
			args: args{
				fieldPath: "spec.forProvider.clientIDList[101]",
				// parse the schema from json
				schema: complexSchemaOpenIDConnectProvidersV1beta1Props,
			},
		},
		{
			name:          "Should accept if below min items, and mark as required if the whole chain is required",
			wantErr:       false,
			wantRequired:  true,
			wantFieldType: "string",
			args: args{
				fieldPath: "spec.forProvider.thumbprintList[0]",
				// parse the schema from json
				schema: complexSchemaOpenIDConnectProvidersV1beta1Props,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFieldType, gotRequired, err := validateFieldPath(tt.args.schema, tt.args.fieldPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateFieldPath() error = %v, wantErrs %v", err, tt.wantErr)
				return
			}
			if gotFieldType != tt.wantFieldType {
				t.Errorf("validateFieldPath() gotFieldType = %v, want %v", gotFieldType, tt.wantFieldType)
			}
			if gotRequired != tt.wantRequired {
				t.Errorf("validateFieldPath() gotRequired = %v, want %v", gotRequired, tt.wantRequired)
			}
		})
	}
}

func Test_validateFieldPathSegmentIndex(t *testing.T) {
	type args struct {
		parent  *apiextensions.JSONSchemaProps
		segment fieldpath.Segment
	}
	tests := []struct {
		name         string
		args         args
		wantRequired bool
		wantErr      bool
	}{
		{
			name: "Should return an error if the parent is not an array",
			args: args{
				parent: &apiextensions.JSONSchemaProps{
					Type: "string",
				},
				segment: fieldpath.Segment{
					Type:  fieldpath.SegmentIndex,
					Index: 1,
				},
			},
			wantErr:      true,
			wantRequired: false,
		},
		{
			name: "Should return no error if the parent is an array",
			args: args{
				parent: &apiextensions.JSONSchemaProps{
					Type: "array",
					Items: &apiextensions.JSONSchemaPropsOrArray{
						Schema: &apiextensions.JSONSchemaProps{
							Type: "string",
						},
					},
				},
				segment: fieldpath.Segment{
					Type:  fieldpath.SegmentIndex,
					Index: 1,
				},
			},
			wantErr:      false,
			wantRequired: false,
		},
		{
			name: "Should return no error and required if the parent is an array with min size 1",
			args: args{
				parent: &apiextensions.JSONSchemaProps{
					Type:     "array",
					MinItems: &[]int64{1}[0],
					Items: &apiextensions.JSONSchemaPropsOrArray{
						Schema: &apiextensions.JSONSchemaProps{
							Type: "string",
						},
					},
				},
				segment: fieldpath.Segment{
					Type:  fieldpath.SegmentIndex,
					Index: 1,
				},
			},
			wantErr:      false,
			wantRequired: true,
		},
		{
			name: "Should return no error and required if the parent is an array with min size 1 and the index is 0",
			args: args{
				parent: &apiextensions.JSONSchemaProps{
					Type:     "array",
					MinItems: &[]int64{1}[0],
					Items: &apiextensions.JSONSchemaPropsOrArray{
						Schema: &apiextensions.JSONSchemaProps{
							Type: "string",
						},
					},
				},
				segment: fieldpath.Segment{
					Type:  fieldpath.SegmentIndex,
					Index: 0,
				},
			},
			wantErr:      false,
			wantRequired: true,
		},
		{
			name: "Should return an error if accessing an index that is above the max items",
			args: args{
				parent: &apiextensions.JSONSchemaProps{
					Type:     "array",
					MaxItems: &[]int64{1}[0],
					Items: &apiextensions.JSONSchemaPropsOrArray{
						Schema: &apiextensions.JSONSchemaProps{
							Type: "string",
						},
					},
				},
				segment: fieldpath.Segment{
					Type:  fieldpath.SegmentIndex,
					Index: 1,
				},
			},
			wantErr:      true,
			wantRequired: false,
		},
		{
			name: "Should return no error if accessing an index that is below the max items",
			args: args{
				parent: &apiextensions.JSONSchemaProps{
					Type:     "array",
					MaxItems: &[]int64{10}[0],
					Items: &apiextensions.JSONSchemaPropsOrArray{
						Schema: &apiextensions.JSONSchemaProps{
							Type: "string",
						},
					},
				},
				segment: fieldpath.Segment{
					Type:  fieldpath.SegmentIndex,
					Index: 1,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, required, err := validateFieldPathSegmentIndex(tt.args.parent, tt.args.segment)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateFieldPathSegmentIndex() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if required != tt.wantRequired {
				t.Errorf("validateFieldPathSegmentIndex() required = %v, want %v", required, tt.wantRequired)
			}
		})
	}
}

func Test_validateFieldPathSegmentField(t *testing.T) {
	type args struct {
		parent  *apiextensions.JSONSchemaProps
		segment fieldpath.Segment
	}
	tests := []struct {
		name         string
		args         args
		wantRequired bool
		wantErr      bool
	}{
		{
			name: "Should return an error if the parent is not an object",
			args: args{
				parent: &apiextensions.JSONSchemaProps{
					Type: "string",
				},
				segment: fieldpath.Segment{
					Type:  fieldpath.SegmentField,
					Field: "foo",
				},
			},
			wantErr:      true,
			wantRequired: false,
		},
		{
			name: "Should return no error if the parent is an object and the field is present",
			args: args{
				parent: &apiextensions.JSONSchemaProps{
					Type: "object",
					Properties: map[string]apiextensions.JSONSchemaProps{
						"foo": {
							Type: "string",
						},
					},
				},
				segment: fieldpath.Segment{
					Type:  fieldpath.SegmentField,
					Field: "foo",
				},
			},
			wantErr:      false,
			wantRequired: false,
		},
		{
			name: "Should return no error if the parent is an object and the field is present and required",
			args: args{
				parent: &apiextensions.JSONSchemaProps{
					Type:     "object",
					Required: []string{"foo"},
					Properties: map[string]apiextensions.JSONSchemaProps{
						"foo": {
							Type: "string",
						},
					},
				},
				segment: fieldpath.Segment{
					Type:  fieldpath.SegmentField,
					Field: "foo",
				},
			},
			wantErr:      false,
			wantRequired: true,
		},
		{
			name: "Should return no error with XPreserveUnknownFields accessing a missing field",
			args: args{
				parent: &apiextensions.JSONSchemaProps{
					Type:                   "object",
					XPreserveUnknownFields: &[]bool{true}[0],
					Properties: map[string]apiextensions.JSONSchemaProps{
						"foo": {
							Type: "string",
						},
					},
				},
				segment: fieldpath.Segment{
					Type:  fieldpath.SegmentField,
					Field: "bar",
				},
			},
			wantErr:      false,
			wantRequired: false,
		},
		{
			name: "Should return no error with XPreserveUnknownFields, but required if a known required field is accessed",
			args: args{
				parent: &apiextensions.JSONSchemaProps{
					Type:                   "object",
					XPreserveUnknownFields: &[]bool{true}[0],
					Required:               []string{"foo"},
					Properties: map[string]apiextensions.JSONSchemaProps{
						"foo": {
							Type: "string",
						},
					},
				},
				segment: fieldpath.Segment{
					Type:  fieldpath.SegmentField,
					Field: "foo",
				},
			},
			wantErr:      false,
			wantRequired: true,
		},
		{
			name: "Should return no error with AdditionalProperties accessing a missing field",
			args: args{
				parent: &apiextensions.JSONSchemaProps{
					Type:                 "object",
					AdditionalProperties: &apiextensions.JSONSchemaPropsOrBool{Allows: true},
					Properties: map[string]apiextensions.JSONSchemaProps{
						"foo": {
							Type: "string",
						},
					},
				},
				segment: fieldpath.Segment{
					Type:  fieldpath.SegmentField,
					Field: "bar",
				},
			},
			wantErr:      false,
			wantRequired: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, required, err := validateFieldPathSegmentField(tt.args.parent, tt.args.segment)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateFieldPathSegmentField() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if required != tt.wantRequired {
				t.Errorf("validateFieldPathSegmentField() got1 = %v, want %v", required, tt.wantRequired)
			}
		})
	}
}
