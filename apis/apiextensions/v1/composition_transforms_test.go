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

package v1

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestTransformValidate(t *testing.T) {
	type args struct {
		transform *Transform
	}
	type want struct {
		err *field.Error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ValidMathMultiply": {
			reason: "Math transform with MathTransform Multiply set should be valid",
			args: args{
				transform: &Transform{
					Type: TransformTypeMath,
					Math: &MathTransform{
						Type:     MathTransformTypeMultiply,
						Multiply: ptr.To[int64](2),
					},
				},
			},
		},
		"ValidMathDefaultType": {
			reason: "Math transform with MathTransform Default set should be valid",
			args: args{
				transform: &Transform{
					Type: TransformTypeMath,
					Math: &MathTransform{
						Multiply: ptr.To[int64](2),
					},
				},
			},
		},
		"ValidMathClampMin": {
			reason: "Math transform with valid MathTransform ClampMin set should be valid",
			args: args{
				transform: &Transform{
					Type: TransformTypeMath,
					Math: &MathTransform{
						Type:     MathTransformTypeClampMin,
						ClampMin: ptr.To[int64](10),
					},
				},
			},
		},
		"InvalidMathWrongSpec": {
			reason: "Math transform with invalid MathTransform set should be invalid",
			args: args{
				transform: &Transform{
					Type: TransformTypeMath,
					Math: &MathTransform{
						Type:     MathTransformTypeMultiply,
						ClampMin: ptr.To[int64](10),
					},
				},
			},
			want: want{
				&field.Error{
					Type:  field.ErrorTypeRequired,
					Field: "math.multiply",
				},
			},
		},
		"InvalidMathNotDefinedAtAll": {
			reason: "Math transform with no MathTransform set should be invalid",
			args: args{
				transform: &Transform{
					Type: TransformTypeMath,
					Math: nil,
				},
			},
			want: want{
				&field.Error{
					Type:  field.ErrorTypeRequired,
					Field: "math",
				},
			},
		},
		"ValidMap": {
			reason: "Map transform with MapTransform set should be valid",
			args: args{
				transform: &Transform{
					Type: TransformTypeMap,
					Map: &MapTransform{
						Pairs: map[string]extv1.JSON{
							"foo": {Raw: []byte(`"bar"`)},
						},
					},
				},
			},
		},
		"InvalidMapNoMap": {
			reason: "Map transform with no map set should be invalid",
			args: args{
				transform: &Transform{
					Type: TransformTypeMap,
					Map:  nil,
				},
			},
			want: want{
				err: &field.Error{
					Type:  field.ErrorTypeRequired,
					Field: "map",
				},
			},
		},
		"InvalidMapNoPairs": {
			reason: "Map transform with no pairs in map should be invalid",
			args: args{
				transform: &Transform{
					Type: TransformTypeMap,
					Map:  &MapTransform{},
				},
			},
			want: want{
				err: &field.Error{
					Type:  field.ErrorTypeRequired,
					Field: "map.pairs",
				},
			},
		},
		"InvalidMatchNoMatch": {
			reason: "Match transform with no match set should be invalid",
			args: args{
				transform: &Transform{
					Type:  TransformTypeMatch,
					Match: nil,
				},
			},
			want: want{
				&field.Error{
					Type:  field.ErrorTypeRequired,
					Field: "match",
				},
			},
		},
		"InvalidMatchEmptyTransform": {
			reason: "Match transform with empty MatchTransform should be invalid",
			args: args{
				transform: &Transform{
					Type:  TransformTypeMatch,
					Match: &MatchTransform{},
				},
			},
			want: want{
				err: &field.Error{
					Type:  field.ErrorTypeRequired,
					Field: "match.patterns",
				},
			},
		},
		"ValidMatchTransformRegexp": {
			reason: "Match transform with valid MatchTransform of type regexp should be valid",
			args: args{
				transform: &Transform{
					Type: TransformTypeMatch,
					Match: &MatchTransform{
						Patterns: []MatchTransformPattern{
							{
								Type:   MatchTransformPatternTypeRegexp,
								Regexp: ptr.To(".*"),
							},
						},
					},
				},
			},
		},
		"InvalidMatchTransformRegexp": {
			reason: "Match transform with an invalid MatchTransform of type regexp with a bad regexp should be invalid",
			args: args{
				transform: &Transform{
					Type: TransformTypeMatch,
					Match: &MatchTransform{
						Patterns: []MatchTransformPattern{
							{
								Type:   MatchTransformPatternTypeRegexp,
								Regexp: ptr.To("?"),
							},
						},
					},
				},
			},
			want: want{
				err: &field.Error{
					Type:  field.ErrorTypeInvalid,
					Field: "match.patterns[0].regexp",
				},
			},
		},
		"ValidMatchTransformString": {
			reason: "Match transform with valid MatchTransform of type literal should be valid",
			args: args{
				transform: &Transform{
					Type: TransformTypeMatch,
					Match: &MatchTransform{
						Patterns: []MatchTransformPattern{
							{
								Type:    MatchTransformPatternTypeLiteral,
								Literal: ptr.To("foo"),
							},
							{
								Literal: ptr.To("bar"),
							},
						},
					},
				},
			},
		},
		"InvalidStringNoString": {
			reason: "String transform with no string set should be invalid",
			args: args{
				transform: &Transform{
					Type:   TransformTypeString,
					String: nil,
				},
			},
			want: want{
				err: &field.Error{
					Type:  field.ErrorTypeRequired,
					Field: "string",
				},
			},
		},
		"ValidString": {
			reason: "String transform with set string should be valid",
			args: args{
				transform: &Transform{
					Type: TransformTypeString,
					String: &StringTransform{
						Format: ptr.To("foo"),
					},
				},
			},
		},
		"InvalidConvertMissingConvert": {
			reason: "Convert transform missing Convert should be invalid",
			args: args{
				transform: &Transform{
					Type:    TransformTypeConvert,
					Convert: nil,
				},
			},
			want: want{
				err: &field.Error{
					Type:  field.ErrorTypeRequired,
					Field: "convert",
				},
			},
		},
		"InvalidConvertUnknownFormat": {
			reason: "Convert transform with unknown format should be invalid",
			args: args{
				transform: &Transform{
					Type: TransformTypeConvert,
					Convert: &ConvertTransform{
						Format: &[]ConvertTransformFormat{"foo"}[0],
					},
				},
			},
			want: want{
				err: &field.Error{
					Type:  field.ErrorTypeInvalid,
					Field: "convert.format",
				},
			},
		},
		"InvalidConvertUnknownToType": {
			reason: "Convert transform with unknown toType should be invalid",
			args: args{
				transform: &Transform{
					Type: TransformTypeConvert,
					Convert: &ConvertTransform{
						ToType: TransformIOType("foo"),
					},
				},
			},
			want: want{
				err: &field.Error{
					Type:  field.ErrorTypeInvalid,
					Field: "convert.toType",
				},
			},
		},
		"ValidConvert": {
			reason: "Convert transform with valid format and toType should be valid",
			args: args{
				transform: &Transform{
					Type: TransformTypeConvert,
					Convert: &ConvertTransform{
						Format: &[]ConvertTransformFormat{ConvertTransformFormatNone}[0],
						ToType: TransformIOTypeInt,
					},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.args.transform.Validate()
			if diff := cmp.Diff(tc.want.err, err, cmpopts.IgnoreFields(field.Error{}, "Detail", "BadValue")); diff != "" {
				t.Errorf("%s\nValidate(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestTransformGetOutputType(t *testing.T) {
	type args struct {
		transform *Transform
	}
	type want struct {
		output *TransformIOType
		err    error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"MapTransform": {
			reason: "Output of Math transform should be float64",
			args: args{
				transform: &Transform{
					Type: TransformTypeMath,
				},
			},
			want: want{
				output: &[]TransformIOType{TransformIOTypeFloat64}[0],
			},
		},
		"ConvertTransform": {
			reason: "Output of Convert transform, no validation, should be the type specified",
			args: args{
				transform: &Transform{
					Type:    TransformTypeConvert,
					Convert: &ConvertTransform{ToType: "fakeType"},
				},
			},
			want: want{
				output: &[]TransformIOType{"fakeType"}[0],
			},
		},
		"ErrorUnknownType": {
			reason: "Output of Unknown transform type returns an error",
			args: args{
				transform: &Transform{
					Type: "fakeType",
				},
			},
			want: want{
				err: fmt.Errorf("unable to get output type, unknown transform type: fakeType"),
			},
		},
		"MapTransformNil": {
			reason: "Output of Map transform is nil",
			args: args{
				transform: &Transform{
					Type: TransformTypeMap,
				},
			},
		},
		"MatchTransformNil": {
			reason: "Output of Match transform is nil",
			args: args{
				transform: &Transform{
					Type: TransformTypeMatch,
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := tc.args.transform.GetOutputType()
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("%s\nGetOutputType(...): -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.output, got); diff != "" {
				t.Errorf("%s\nGetOutputType(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
