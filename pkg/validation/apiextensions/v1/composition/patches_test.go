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
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xperrors "github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/pkg/validation/internal/schema"

	_ "embed"
)

var (
	// got running `kubectl get crds -o json openidconnectproviders.iam.aws.crossplane.io  | jq -c --raw-output '.spec.versions[0].schema.openAPIV3Schema |del(.. | .description?)'`
	// from provider: xpkg.upbound.io/crossplane-contrib/provider-aws:v0.38.0
	//go:embed testdata/complex_schema_openidconnectproviders_v1beta1.json
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

func TestValidateTransforms(t *testing.T) {
	type args struct {
		transforms       []v1.Transform
		fromType, toType schema.KnownJSONType
	}
	type want struct {
		err *field.Error
	}
	tests := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"AcceptEmptyTransformsSameType": {
			reason: "Should accept empty transforms to the same type successfully",
			args: args{
				transforms: []v1.Transform{},
				fromType:   "string",
				toType:     "string",
			},
		},
		"AcceptNilTransformsSameType": {
			reason: "Should accept if no transforms are provided and the types are the same",
			want:   want{err: nil},
			args: args{
				transforms: nil,
				fromType:   "string",
				toType:     "string",
			},
		},
		"RejectEmptyTransformsWrongTypes": {
			reason: "Should reject empty transforms to a different type",
			want: want{err: &field.Error{
				Type:  field.ErrorTypeRequired,
				Field: "transforms",
			}},
			args: args{
				transforms: []v1.Transform{},
				fromType:   "string",
				toType:     "integer",
			},
		},
		"RejectNilTransformsWrongTypes": {
			reason: "Should reject if no transforms are provided and the types are not the same",
			want: want{err: &field.Error{
				Type:  field.ErrorTypeRequired,
				Field: "transforms",
			}},
			args: args{
				transforms: nil,
				fromType:   "string",
				toType:     "integer",
			},
		},
		"AcceptEmptyTransformsCompatibleTypes": {
			reason: "Should accept empty transforms to a different type when its integer to number",
			want:   want{err: nil},
			args: args{
				transforms: []v1.Transform{},
				fromType:   "integer",
				toType:     "number",
			},
		},
		"AcceptNilTransformsCompatibleTypes": {
			reason: "Should accept if no transforms are provided and the types are not the same but the types are integer and number",
			want:   want{err: nil},
			args: args{
				transforms: nil,
				fromType:   "integer",
				toType:     "number",
			},
		},
		"AcceptConvertTransforms": {
			reason: "Should accept convert transforms successfully",
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
		"AcceptConvertTransformsMultiple": {
			reason: "Should accept convert integer to number transforms successfully",
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
		"RejectConvertTransformsNumberToInteger": {
			reason: "Should reject convert number to integer transforms",
			want: want{err: &field.Error{
				Type:  field.ErrorTypeInvalid,
				Field: "transforms",
			}},
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
		"AcceptValidChainedConvertTransforms": {
			reason: "Should accept valid chained convert transforms",
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
		"RejectInvalidTransformType": {
			reason: "Should reject invalid transform types",
			want: want{err: &field.Error{
				Type:  field.ErrorTypeInvalid,
				Field: "transforms[0]",
			}},
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
		"AcceptNilTransformsNoFromType": {
			reason: "Should accept if there is no type spec for input and no transforms are provided",
			want:   want{err: nil},
			args: args{
				transforms: nil,
				fromType:   "",
				toType:     "string",
			},
		},
		"AcceptNilTransformsNoToType": {
			reason: "Should accept if there is no type spec for output and no transforms are provided",
			want:   want{err: nil},
			args: args{
				transforms: nil,
				fromType:   "string",
				toType:     "",
			},
		},
		"AcceptNoInputOutputNoTransforms": {
			reason: "Should accept if there are no type spec for input and output and no transforms are provided",
			want:   want{err: nil},
			args: args{
				transforms: nil,
				fromType:   "",
				toType:     "",
			},
		},
		"AcceptNoInputOutputWithTransforms": {
			reason: "Should accept if there are no type spec for input and output and transforms are provided",
			want:   want{err: nil},
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
		"RejectNoToTypeInvalidInputType": {
			reason: "Should reject if there is no type spec for the output, but input is specified and transforms are wrong",
			want: want{err: &field.Error{
				Type:  field.ErrorTypeInvalid,
				Field: "transforms[0]",
			}},
			args: args{
				transforms: []v1.Transform{
					{
						Type: v1.TransformTypeMath,
						Math: &v1.MathTransform{
							Multiply: ptr.To[int64](2),
						},
					},
				},
				fromType: "string",
				toType:   "",
			},
		},
		"RejectNoInputTypeWrongOutputTypeForTransforms": {
			reason: "Should return an error if there is no type spec for the input, but output is specified and transforms are wrong",
			want: want{err: &field.Error{
				Type:  field.ErrorTypeInvalid,
				Field: "transforms",
			}},
			args: args{
				transforms: []v1.Transform{
					{
						Type: v1.TransformTypeMath,
						Math: &v1.MathTransform{
							Multiply: ptr.To[int64](2),
						},
					},
				},
				fromType: "",
				toType:   "string",
			},
		},
		"AcceptObjectInputTypeToJsonStringTransform": {
			reason: "Should accept object input type with json string transform",
			want:   want{err: nil},
			args: args{
				transforms: []v1.Transform{
					{
						Type: v1.TransformTypeString,
						String: &v1.StringTransform{
							Type:    v1.StringTransformTypeConvert,
							Convert: &[]v1.StringConversionType{v1.StringConversionTypeToJSON}[0],
						},
					},
				},
				fromType: "object",
				toType:   "string",
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := validateIOTypesWithTransforms(tc.args.transforms, tc.args.fromType, tc.args.toType)
			if diff := cmp.Diff(tc.want.err, err, cmpopts.IgnoreFields(field.Error{}, "BadValue", "Detail")); diff != "" {
				t.Errorf("\n%s\nvalidateIOTypesWithTransforms(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestValidateFieldPath(t *testing.T) {
	type args struct {
		schema    *apiextensions.JSONSchemaProps
		fieldPath string
	}
	type want struct {
		err       error
		fieldType schema.KnownJSONType
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"AcceptValidFieldPath": {
			reason: "Should validate a valid field path",
			want:   want{err: nil, fieldType: "string"},
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
		"AcceptMetadataLabelsValue": {
			reason: "Should validate a valid field path",
			want:   want{err: nil, fieldType: "string"},
			args: args{
				fieldPath: "metadata.labels[networks.aws.platformref.upbound.io/network-id]",
				schema: &apiextensions.JSONSchemaProps{
					Properties: map[string]apiextensions.JSONSchemaProps{
						"metadata": *getDefaultMetadataSchema(),
					},
				},
			},
		},
		"RejectInvalidFieldPath": {
			reason: "Should return an error for an invalid field path",
			want:   want{err: xperrors.Errorf(errFmtFieldInvalid, "wrong")},
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
		"AcceptFieldPathXPreserveUnknownFields": {
			reason: "Should not return an error for an undefined but accepted field path",
			want:   want{err: nil, fieldType: ""},
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
		"AcceptValidArray": {
			reason: "Should validate arrays properly",
			want:   want{err: nil, fieldType: "string"},
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
		"AcceptComplexSchema": {
			reason: "Should validate properly with complex schema",
			want:   want{err: nil, fieldType: "string"},
			args: args{
				fieldPath: "spec.forProvider.clientIDList[0]",
				// parse the schema from json
				schema: complexSchemaOpenIDConnectProvidersV1beta1Props,
			},
		},
		"RejectComplexAboveMaxItems": {
			reason: "Should error if above max items",
			want:   want{err: xperrors.Errorf(errFmtArrayIndexAboveMax, 101, 99)},
			args: args{
				fieldPath: "spec.forProvider.clientIDList[101]",
				// parse the schema from json
				schema: complexSchemaOpenIDConnectProvidersV1beta1Props,
			},
		},
		"AcceptBelowMinItemsRequiredChain": {
			reason: "Should accept if below min items, and mark as required if the whole chain is required",
			want:   want{err: nil, fieldType: "string"},
			args: args{
				fieldPath: "spec.forProvider.thumbprintList[0]",
				// parse the schema from json
				schema: complexSchemaOpenIDConnectProvidersV1beta1Props,
			},
		},
		"AcceptMetadataUID": {
			reason: "Should accept metadata.uid",
			want:   want{err: nil, fieldType: "string"},
			args: args{
				fieldPath: "metadata.uid",
				schema:    &apiextensions.JSONSchemaProps{Properties: map[string]apiextensions.JSONSchemaProps{"metadata": {Type: "object"}}},
			},
		},
		"AcceptXPreserveUnknownFieldsInAdditionalProperties": {
			reason: "Should properly handle x-preserve-unknown-fields even if defined in a nested schema",
			want:   want{err: nil, fieldType: ""},
			args: args{
				fieldPath: "data.someField",
				schema: &apiextensions.JSONSchemaProps{
					Properties: map[string]apiextensions.JSONSchemaProps{
						"data": {
							Type: "object",
							AdditionalProperties: &apiextensions.JSONSchemaPropsOrBool{
								Schema: &apiextensions.JSONSchemaProps{
									XPreserveUnknownFields: &[]bool{true}[0],
								},
							},
						}}}},
		},
		"AcceptAnnotations": {
			want: want{err: nil, fieldType: "string"},
			args: args{
				fieldPath: "metadata.annotations[cooler-field]",
				schema:    getDefaultSchema(),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			gotFieldType, err := validateFieldPath(tc.args.schema, tc.args.fieldPath)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nvalidateFieldPath(...): -want error, +got error: %s\n", tc.reason, diff)
				return
			}
			if diff := cmp.Diff(tc.want.fieldType, gotFieldType); diff != "" {
				t.Errorf("\n%s\nvalidateFieldPath(...): -want, +got: %s\n", tc.reason, diff)
			}
		})
	}
}

func TestValidateFieldPathSegmentIndex(t *testing.T) {
	type args struct {
		parent  *apiextensions.JSONSchemaProps
		segment fieldpath.Segment
	}
	type want struct {
		err error
	}
	cases := map[string]struct {
		name string
		args args
		want want
	}{
		"RejectParentNotArray": {
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
			want: want{err: xperrors.Errorf(errFmtIndexAccessWrongType, "string")},
		},
		"AcceptParentArray": {
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
			want: want{err: nil},
		},
		"AcceptMinSizeArrayBelowRequired": {
			name: "Should return no error and required if the parent is an array, accessing element below min size",
			args: args{
				parent: &apiextensions.JSONSchemaProps{
					Type:     "array",
					MinItems: &[]int64{2}[0],
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
			want: want{err: nil},
		},
		"AcceptMinSizeArrayAboveNotRequired": {
			name: "Should return no error and not required if the parent is an array, accessing element above min size",
			args: args{
				parent: &apiextensions.JSONSchemaProps{
					Type:     "array",
					MinItems: &[]int64{2}[0],
					Items: &apiextensions.JSONSchemaPropsOrArray{
						Schema: &apiextensions.JSONSchemaProps{
							Type: "string",
						},
					},
				},
				segment: fieldpath.Segment{
					Type:  fieldpath.SegmentIndex,
					Index: 3,
				},
			},
			want: want{err: nil},
		},
		"AcceptIndex0MinSize1": {
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
			want: want{err: nil},
		},
		"RejectAboveMaxIndex": {
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
			want: want{err: xperrors.Errorf(errFmtArrayIndexAboveMax, 1, 0)},
		},
		"AcceptBelowMaxIndex": {
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
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := validateFieldPathSegmentIndex(tc.args.parent, tc.args.segment)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nvalidateFieldPathSegmentIndex(...): -want, +got: %s\n", tc.name, diff)
			}
		})
	}
}

func TestValidateFieldPathSegmentField(t *testing.T) {
	type args struct {
		parent  *apiextensions.JSONSchemaProps
		segment fieldpath.Segment
	}
	type want struct {
		err error
	}
	cases := map[string]struct {
		name string
		args args
		want want
	}{
		"RejectParentNotObject": {
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
			want: want{err: xperrors.Errorf(errFmtFieldAccessWrongType, "foo", "string")},
		},
		"AcceptFieldNotPresent": {
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
			want: want{err: nil},
		},
		"AcceptFieldNotPresentWithXPreserveUnknownFields": {
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
			want: want{err: nil},
		},
		"AcceptFieldPresentWithXPreserveUnknownFieldsRequired": {
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
			want: want{err: nil},
		},
		"AcceptFieldNotPresentWithAdditionalProperties": {
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
			want: want{err: nil},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateFieldPathSegmentField(tt.args.parent, tt.args.segment)
			if diff := cmp.Diff(tt.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nvalidateFieldPathSegmentField(...): -want, +got: %s\n", tt.name, diff)
			}
		})
	}
}

func TestGetSchemaForVersion(t *testing.T) {
	type args struct {
		crd     *apiextensions.CustomResourceDefinition
		version string
	}
	type want struct {
		schema *apiextensions.JSONSchemaProps
	}
	fooSchema := &apiextensions.JSONSchemaProps{
		Type: "object",
		Properties: map[string]apiextensions.JSONSchemaProps{
			"foo": {
				Type: "string",
			},
		},
	}
	barSchema := &apiextensions.JSONSchemaProps{
		Type: "object",
		Properties: map[string]apiextensions.JSONSchemaProps{
			"bar": {
				Type: "string",
			},
		},
	}
	cases := map[string]struct {
		name string
		args args
		want want
	}{
		"GetSchemaForVersionTopLevel": {
			name: "Should return the schema for the given version",
			args: args{
				crd: &apiextensions.CustomResourceDefinition{
					Spec: apiextensions.CustomResourceDefinitionSpec{
						Validation: &apiextensions.CustomResourceValidation{
							OpenAPIV3Schema: fooSchema,
						},
					},
				},
			},
			want: want{
				schema: fooSchema,
			},
		},
		"GetSchemaForVersionTopLevelAlways": {
			name: "Should return the schema for the given version always, even if additional versions are specified, which should never happen",
			args: args{
				crd: &apiextensions.CustomResourceDefinition{
					Spec: apiextensions.CustomResourceDefinitionSpec{
						Validation: &apiextensions.CustomResourceValidation{
							OpenAPIV3Schema: fooSchema,
						},
						Versions: []apiextensions.CustomResourceDefinitionVersion{
							{
								Name:    "v1",
								Served:  true,
								Storage: true,
								Schema: &apiextensions.CustomResourceValidation{
									OpenAPIV3Schema: barSchema,
								},
							},
						},
					},
				},
				version: "v1",
			},
			want: want{
				schema: fooSchema,
			},
		},
		"GetSchemaForVersionExisting": {
			name: "Should return the schema for the given version",
			args: args{
				crd: &apiextensions.CustomResourceDefinition{
					Spec: apiextensions.CustomResourceDefinitionSpec{
						Versions: []apiextensions.CustomResourceDefinitionVersion{
							{
								Name:    "v1alpha1",
								Served:  true,
								Storage: true,
								Schema: &apiextensions.CustomResourceValidation{
									OpenAPIV3Schema: fooSchema,
								},
							},
							{
								Name:    "v1",
								Served:  true,
								Storage: true,
								Schema: &apiextensions.CustomResourceValidation{
									OpenAPIV3Schema: barSchema,
								},
							},
						},
					},
				},
				version: "v1",
			},
			want: want{
				schema: barSchema,
			},
		},
		"GetSchemaForVersionNotExisting": {
			name: "Should return nil if the given version does not exist",
			args: args{
				crd: &apiextensions.CustomResourceDefinition{
					Spec: apiextensions.CustomResourceDefinitionSpec{
						Versions: []apiextensions.CustomResourceDefinitionVersion{
							{
								Name:    "v1",
								Served:  true,
								Storage: true,
								Schema: &apiextensions.CustomResourceValidation{
									OpenAPIV3Schema: &apiextensions.JSONSchemaProps{
										Type: "object",
									},
								},
							},
						},
					},
				},
				version: "v2",
			},
			want: want{
				schema: nil,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := getSchemaForVersion(tc.args.crd, tc.args.version)
			if diff := cmp.Diff(tc.want.schema, got); diff != "" {
				t.Errorf("\n%s\ngetSchemaForVersion(...): -want, +got: %s\n", name, diff)
			}
		})
	}

}

func TestComposedTemplateGetBaseObject(t *testing.T) {
	type args struct {
		ct *v1.ComposedTemplate
	}
	type want struct {
		output client.Object
		err    error
	}
	tests := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ValidBaseObject": {
			reason: "Valid base object should be parsed properly",
			args: args{
				ct: &v1.ComposedTemplate{
					Base: runtime.RawExtension{
						Raw: []byte(`{"apiVersion":"v1","kind":"Service","metadata":{"name":"foo"}}`),
					},
				},
			},
			want: want{
				output: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "Service",
						"metadata": map[string]interface{}{
							"name": "foo",
						},
					},
				},
			},
		},
		"InvalidBaseObject": {
			reason: "Invalid base object should return an error",
			args: args{
				ct: &v1.ComposedTemplate{
					Base: runtime.RawExtension{
						Raw: []byte(`{$$$WRONG$$$:"v1","kind":"Service","metadata":{"name":"foo"}}`),
					},
				},
			},
			want: want{
				err: xperrors.Wrap(errors.New("invalid character '$' looking for beginning of object key string"), errUnableToParse),
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := GetBaseObject(tc.args.ct)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGetBaseObject(...): -want error, +got error: \n%s", tc.reason, diff)
				return
			}
			if diff := cmp.Diff(tc.want.output, got); diff != "" {
				t.Errorf("\n%s\nGetBaseObject(...): -want, +got: \n%s", tc.reason, diff)
			}
		})
	}
}

func TestIsValidInputForTransform(t *testing.T) {
	type args struct {
		t        *v1.Transform
		fromType v1.TransformIOType
	}
	type want struct {
		err bool
	}
	tests := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ValidStringTransformInputString": {
			reason: "Valid String transformType should not return an error with input string",
			args: args{
				fromType: v1.TransformIOTypeString,
				t: &v1.Transform{
					Type: v1.TransformTypeString,
					String: &v1.StringTransform{
						Type:    v1.StringTransformTypeConvert,
						Convert: ptr.To(v1.StringConversionTypeToUpper),
					},
				},
			},
		},
		"ValidStringTransformInputObjectToJson": {
			reason: "Valid String transformType should not return an error with input object if toJson",
			args: args{
				fromType: v1.TransformIOTypeObject,
				t: &v1.Transform{
					Type: v1.TransformTypeString,
					String: &v1.StringTransform{
						Type:    v1.StringTransformTypeConvert,
						Convert: ptr.To(v1.StringConversionTypeToJSON),
					},
				},
			},
		},
		"InValidStringTransformInputObjectToUpper": {
			reason: "Valid String transformType should not return an error with input string",
			args: args{
				fromType: v1.TransformIOTypeObject,
				t: &v1.Transform{
					Type: v1.TransformTypeString,
					String: &v1.StringTransform{
						Type:    v1.StringTransformTypeConvert,
						Convert: ptr.To(v1.StringConversionTypeToUpper),
					},
				},
			},
			want: want{
				err: true,
			},
		},
		"ValidStringTransformInputArrayJoin": {
			reason: "Valid String transformType should not return an error with input array if join",
			args: args{
				fromType: v1.TransformIOTypeArray,
				t: &v1.Transform{
					Type: v1.TransformTypeString,
					String: &v1.StringTransform{
						Type: v1.StringTransformTypeJoin,
						Join: &v1.StringTransformJoin{
							Separator: ",",
						},
					},
				},
			},
		},
		"InvalidStringTransformInputArrayJoin": {
			reason: "Valid String transformType should return an error with input object if join",
			args: args{
				fromType: v1.TransformIOTypeObject,
				t: &v1.Transform{
					Type: v1.TransformTypeString,
					String: &v1.StringTransform{
						Type: v1.StringTransformTypeJoin,
						Join: &v1.StringTransformJoin{
							Separator: ",",
						},
					},
				},
			},
			want: want{
				err: true,
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := IsValidInputForTransform(tc.args.t, tc.args.fromType)
			if tc.want.err != (err != nil) {
				t.Errorf("\n%s\nIsValidInputForTransform(...): -want error, +got error: \n%s", tc.reason, err)
				return
			}
		})
	}
}
