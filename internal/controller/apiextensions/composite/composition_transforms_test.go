/*
Copyright 2022 The Crossplane Authors.

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

package composite

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/pointer"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

func TestMapResolve(t *testing.T) {
	asJSON := func(val interface{}) extv1.JSON {
		raw, err := json.Marshal(val)
		if err != nil {
			t.Fatal(err)
		}
		res := extv1.JSON{}
		if err := json.Unmarshal(raw, &res); err != nil {
			t.Fatal(err)
		}
		return res
	}

	type args struct {
		t v1.MapTransform
		i any
	}
	type want struct {
		o   any
		err error
	}

	cases := map[string]struct {
		args
		want
	}{
		"NonStringInput": {
			args: args{
				i: 5,
			},
			want: want{
				err: errors.Errorf(errFmtMapTypeNotSupported, "int"),
			},
		},
		"KeyNotFound": {
			args: args{
				i: "ola",
			},
			want: want{
				err: errors.Errorf(errFmtMapNotFound, "ola"),
			},
		},
		"SuccessString": {
			args: args{
				t: v1.MapTransform{Pairs: map[string]extv1.JSON{"ola": asJSON("voila")}},
				i: "ola",
			},
			want: want{
				o: "voila",
			},
		},
		"SuccessNumber": {
			args: args{
				t: v1.MapTransform{Pairs: map[string]extv1.JSON{"ola": asJSON(1.0)}},
				i: "ola",
			},
			want: want{
				o: 1.0,
			},
		},
		"SuccessBoolean": {
			args: args{
				t: v1.MapTransform{Pairs: map[string]extv1.JSON{"ola": asJSON(true)}},
				i: "ola",
			},
			want: want{
				o: true,
			},
		},
		"SuccessObject": {
			args: args{
				t: v1.MapTransform{Pairs: map[string]extv1.JSON{"ola": asJSON(map[string]interface{}{"foo": "bar"})}},
				i: "ola",
			},
			want: want{
				o: map[string]interface{}{"foo": "bar"},
			},
		},
		"SuccessSlice": {
			args: args{
				t: v1.MapTransform{Pairs: map[string]extv1.JSON{"ola": asJSON([]string{"foo", "bar"})}},
				i: "ola",
			},
			want: want{
				o: []interface{}{"foo", "bar"},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ResolveMap(tc.t, tc.i)

			if diff := cmp.Diff(tc.want.o, got); diff != "" {
				t.Errorf("Resolve(b): -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("Resolve(b): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestMatchResolve(t *testing.T) {
	asJSON := func(val interface{}) extv1.JSON {
		raw, err := json.Marshal(val)
		if err != nil {
			t.Fatal(err)
		}
		res := extv1.JSON{}
		if err := json.Unmarshal(raw, &res); err != nil {
			t.Fatal(err)
		}
		return res
	}

	type args struct {
		t v1.MatchTransform
		i any
	}
	type want struct {
		o   any
		err error
	}

	cases := map[string]struct {
		args
		want
	}{
		"ErrNonStringInput": {
			args: args{
				t: v1.MatchTransform{
					Patterns: []v1.MatchTransformPattern{
						{
							Type:    v1.MatchTransformPatternTypeLiteral,
							Literal: pointer.String("5"),
						},
					},
				},
				i: 5,
			},
			want: want{
				err: errors.Wrapf(errors.Errorf(errFmtMatchInputTypeInvalid, "int"), errFmtMatchPattern, 0),
			},
		},
		"NoPatternsFallback": {
			args: args{
				t: v1.MatchTransform{
					Patterns:      []v1.MatchTransformPattern{},
					FallbackValue: asJSON("bar"),
				},
				i: "foo",
			},
			want: want{
				o: "bar",
			},
		},
		"NoPatternsFallbackNil": {
			args: args{
				t: v1.MatchTransform{
					Patterns:      []v1.MatchTransformPattern{},
					FallbackValue: asJSON(nil),
				},
				i: "foo",
			},
			want: want{},
		},
		"MatchLiteral": {
			args: args{
				t: v1.MatchTransform{
					Patterns: []v1.MatchTransformPattern{
						{
							Type:    v1.MatchTransformPatternTypeLiteral,
							Literal: pointer.String("foo"),
							Result:  asJSON("bar"),
						},
					},
				},
				i: "foo",
			},
			want: want{
				o: "bar",
			},
		},
		"MatchLiteralFirst": {
			args: args{
				t: v1.MatchTransform{
					Patterns: []v1.MatchTransformPattern{
						{
							Type:    v1.MatchTransformPatternTypeLiteral,
							Literal: pointer.String("foo"),
							Result:  asJSON("bar"),
						},
						{
							Type:    v1.MatchTransformPatternTypeLiteral,
							Literal: pointer.String("foo"),
							Result:  asJSON("not this"),
						},
					},
				},
				i: "foo",
			},
			want: want{
				o: "bar",
			},
		},
		"MatchLiteralWithResultStruct": {
			args: args{
				t: v1.MatchTransform{
					Patterns: []v1.MatchTransformPattern{
						{
							Type:    v1.MatchTransformPatternTypeLiteral,
							Literal: pointer.String("foo"),
							Result: asJSON(map[string]interface{}{
								"Hello": "World",
							}),
						},
					},
				},
				i: "foo",
			},
			want: want{
				o: map[string]interface{}{
					"Hello": "World",
				},
			},
		},
		"MatchLiteralWithResultSlice": {
			args: args{
				t: v1.MatchTransform{
					Patterns: []v1.MatchTransformPattern{
						{
							Type:    v1.MatchTransformPatternTypeLiteral,
							Literal: pointer.String("foo"),
							Result: asJSON([]string{
								"Hello", "World",
							}),
						},
					},
				},
				i: "foo",
			},
			want: want{
				o: []any{
					"Hello", "World",
				},
			},
		},
		"MatchLiteralWithResultNumber": {
			args: args{
				t: v1.MatchTransform{
					Patterns: []v1.MatchTransformPattern{
						{
							Type:    v1.MatchTransformPatternTypeLiteral,
							Literal: pointer.String("foo"),
							Result:  asJSON(5),
						},
					},
				},
				i: "foo",
			},
			want: want{
				o: 5.0,
			},
		},
		"MatchLiteralWithResultBool": {
			args: args{
				t: v1.MatchTransform{
					Patterns: []v1.MatchTransformPattern{
						{
							Type:    v1.MatchTransformPatternTypeLiteral,
							Literal: pointer.String("foo"),
							Result:  asJSON(true),
						},
					},
				},
				i: "foo",
			},
			want: want{
				o: true,
			},
		},
		"MatchLiteralWithResultNil": {
			args: args{
				t: v1.MatchTransform{
					Patterns: []v1.MatchTransformPattern{
						{
							Type:    v1.MatchTransformPatternTypeLiteral,
							Literal: pointer.String("foo"),
							Result:  asJSON(nil),
						},
					},
				},
				i: "foo",
			},
			want: want{},
		},
		"MatchRegexp": {
			args: args{
				t: v1.MatchTransform{
					Patterns: []v1.MatchTransformPattern{
						{
							Type:   v1.MatchTransformPatternTypeRegexp,
							Regexp: pointer.String("^foo.*$"),
							Result: asJSON("Hello World"),
						},
					},
				},
				i: "foobar",
			},
			want: want{
				o: "Hello World",
			},
		},
		"ErrMissingRegexp": {
			args: args{
				t: v1.MatchTransform{
					Patterns: []v1.MatchTransformPattern{
						{
							Type: v1.MatchTransformPatternTypeRegexp,
						},
					},
				},
			},
			want: want{
				err: errors.Wrapf(errors.Errorf(errFmtRequiredField, "regexp", string(v1.MatchTransformPatternTypeRegexp)), errFmtMatchPattern, 0),
			},
		},
		"ErrInvalidRegexp": {
			args: args{
				t: v1.MatchTransform{
					Patterns: []v1.MatchTransformPattern{
						{
							Type:   v1.MatchTransformPatternTypeRegexp,
							Regexp: pointer.String("?="),
						},
					},
				},
			},
			want: want{
				// This might break if Go's regexp changes its internal error
				// messages:
				err: errors.Wrapf(errors.Wrapf(errors.Wrap(errors.Wrap(errors.New("`?`"), "missing argument to repetition operator"), "error parsing regexp"), errMatchRegexpCompile), errFmtMatchPattern, 0),
			},
		},
		"ErrMissingLiteral": {
			args: args{
				t: v1.MatchTransform{
					Patterns: []v1.MatchTransformPattern{
						{
							Type: v1.MatchTransformPatternTypeLiteral,
						},
					},
				},
			},
			want: want{
				err: errors.Wrapf(errors.Errorf(errFmtRequiredField, "literal", string(v1.MatchTransformPatternTypeLiteral)), errFmtMatchPattern, 0),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ResolveMatch(tc.args.t, tc.i)

			if diff := cmp.Diff(tc.want.o, got); diff != "" {
				t.Errorf("Resolve(b): -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("Resolve(b): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestMathResolve(t *testing.T) {
	m := int64(2)

	type args struct {
		multiplier *int64
		i          any
	}
	type want struct {
		o   any
		err error
	}

	cases := map[string]struct {
		args
		want
	}{
		"NoMultiplier": {
			args: args{
				i: 25,
			},
			want: want{
				err: errors.New(errMathNoMultiplier),
			},
		},
		"NonNumberInput": {
			args: args{
				multiplier: &m,
				i:          "ola",
			},
			want: want{
				err: errors.New(errMathInputNonNumber),
			},
		},
		"Success": {
			args: args{
				multiplier: &m,
				i:          3,
			},
			want: want{
				o: 3 * m,
			},
		},
		"SuccessInt64": {
			args: args{
				multiplier: &m,
				i:          int64(3),
			},
			want: want{
				o: 3 * m,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tr := v1.MathTransform{Multiply: tc.multiplier}
			got, err := ResolveMath(tr, tc.i)

			if diff := cmp.Diff(tc.want.o, got); diff != "" {
				t.Errorf("Resolve(b): -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("Resolve(b): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestStringResolve(t *testing.T) {

	type args struct {
		stype   v1.StringTransformType
		fmts    *string
		convert *v1.StringConversionType
		trim    *string
		regexp  *v1.StringTransformRegexp
		i       any
	}
	type want struct {
		o   string
		err error
	}
	sFmt := "verycool%s"
	iFmt := "the largest %d"

	upper := v1.StringConversionTypeToUpper
	lower := v1.StringConversionTypeToLower
	tobase64 := v1.StringConversionTypeToBase64
	frombase64 := v1.StringConversionTypeFromBase64
	toJSON := v1.StringConversionTypeToJSON
	wrongConvertType := v1.StringConversionType("Something")
	toSha1 := v1.StringConversionTypeToSHA1
	toSha256 := v1.StringConversionTypeToSHA256
	toSha512 := v1.StringConversionTypeToSHA512

	prefix := "https://"
	suffix := "-test"

	cases := map[string]struct {
		args
		want
	}{
		"NotSupportedType": {
			args: args{
				stype: "Something",
				i:     "value",
			},
			want: want{
				err: errors.Errorf(errStringTransformTypeFailed, "Something"),
			},
		},
		"FmtFailed": {
			args: args{
				stype: v1.StringTransformTypeFormat,
				i:     "value",
			},
			want: want{
				err: errors.Errorf(errStringTransformTypeFormat, string(v1.StringTransformTypeFormat)),
			},
		},
		"FmtString": {
			args: args{
				stype: v1.StringTransformTypeFormat,
				fmts:  &sFmt,
				i:     "thing",
			},
			want: want{
				o: "verycoolthing",
			},
		},
		"FmtInteger": {
			args: args{
				stype: v1.StringTransformTypeFormat,
				fmts:  &iFmt,
				i:     8,
			},
			want: want{
				o: "the largest 8",
			},
		},
		"ConvertNotSet": {
			args: args{
				stype: v1.StringTransformTypeConvert,
				i:     "crossplane",
			},
			want: want{
				err: errors.Errorf(errStringTransformTypeConvert, string(v1.StringTransformTypeConvert)),
			},
		},
		"ConvertTypFailed": {
			args: args{
				stype:   v1.StringTransformTypeConvert,
				convert: &wrongConvertType,
				i:       "crossplane",
			},
			want: want{
				err: errors.Errorf(errStringConvertTypeFailed, wrongConvertType),
			},
		},
		"ConvertToUpper": {
			args: args{
				stype:   v1.StringTransformTypeConvert,
				convert: &upper,
				i:       "crossplane",
			},
			want: want{
				o: "CROSSPLANE",
			},
		},
		"ConvertToLower": {
			args: args{
				stype:   v1.StringTransformTypeConvert,
				convert: &lower,
				i:       "CrossPlane",
			},
			want: want{
				o: "crossplane",
			},
		},
		"ConvertToBase64": {
			args: args{
				stype:   v1.StringTransformTypeConvert,
				convert: &tobase64,
				i:       "CrossPlane",
			},
			want: want{
				o: "Q3Jvc3NQbGFuZQ==",
			},
		},
		"ConvertFromBase64": {
			args: args{
				stype:   v1.StringTransformTypeConvert,
				convert: &frombase64,
				i:       "Q3Jvc3NQbGFuZQ==",
			},
			want: want{
				o: "CrossPlane",
			},
		},
		"ConvertFromBase64Error": {
			args: args{
				stype:   v1.StringTransformTypeConvert,
				convert: &frombase64,
				i:       "ThisStringIsNotBase64",
			},
			want: want{
				o:   "N\x18\xacJ\xda\xe2\x9e\x02,6\x8bAjǺ",
				err: errors.Wrap(errors.New("illegal base64 data at input byte 20"), errDecodeString),
			},
		},
		"ConvertToSha1": {
			args: args{
				stype:   v1.StringTransformTypeConvert,
				convert: &toSha1,
				i:       "Crossplane",
			},
			want: want{
				o: "f9fd1da3c0cc298643ff098a0c59febf1d8b7b84",
			},
		},
		"ConvertToSha1Error": {
			args: args{
				stype:   v1.StringTransformTypeConvert,
				convert: &toSha1,
				i:       func() {},
			},
			want: want{
				o:   "0000000000000000000000000000000000000000",
				err: errors.Wrap(errors.Wrap(errors.New("json: unsupported type: func()"), errMarshalJSON), errHash),
			},
		},
		"ConvertToSha256": {
			args: args{
				stype:   v1.StringTransformTypeConvert,
				convert: &toSha256,
				i:       "Crossplane",
			},
			want: want{
				o: "e84ae541a0725d73154ee76b7ac3fec4b007dd01ed701d506cd7e7a45bb48935",
			},
		},
		"ConvertToSha256Error": {
			args: args{
				stype:   v1.StringTransformTypeConvert,
				convert: &toSha256,
				i:       func() {},
			},
			want: want{
				o:   "0000000000000000000000000000000000000000000000000000000000000000",
				err: errors.Wrap(errors.Wrap(errors.New("json: unsupported type: func()"), errMarshalJSON), errHash),
			},
		},
		"ConvertToSha512": {
			args: args{
				stype:   v1.StringTransformTypeConvert,
				convert: &toSha512,
				i:       "Crossplane",
			},
			want: want{
				o: "b48622a3f487b8cb7748b356c9531cf54d9125c1456689c115744821f3dafd59c8c7d4dc5627c4a1e4082c67ee9f4528365a644a01a0c46d6dd0a6d979c8f51f",
			},
		},
		"ConvertToSha512Error": {
			args: args{
				stype:   v1.StringTransformTypeConvert,
				convert: &toSha512,
				i:       func() {},
			},
			want: want{
				o:   "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
				err: errors.Wrap(errors.Wrap(errors.New("json: unsupported type: func()"), errMarshalJSON), errHash),
			},
		},
		"TrimPrefix": {
			args: args{
				stype: v1.StringTransformTypeTrimPrefix,
				trim:  &prefix,
				i:     "https://crossplane.io",
			},
			want: want{
				o: "crossplane.io",
			},
		},
		"TrimSuffix": {
			args: args{
				stype: v1.StringTransformTypeTrimSuffix,
				trim:  &suffix,
				i:     "my-string-test",
			},
			want: want{
				o: "my-string",
			},
		},
		"TrimPrefixWithoutMatch": {
			args: args{
				stype: v1.StringTransformTypeTrimPrefix,
				trim:  &prefix,
				i:     "crossplane.io",
			},
			want: want{
				o: "crossplane.io",
			},
		},
		"TrimSuffixWithoutMatch": {
			args: args{
				stype: v1.StringTransformTypeTrimSuffix,
				trim:  &suffix,
				i:     "my-string",
			},
			want: want{
				o: "my-string",
			},
		},
		"RegexpNotCompiling": {
			args: args{
				stype: v1.StringTransformTypeRegexp,
				regexp: &v1.StringTransformRegexp{
					Match: "[a-z",
				},
				i: "my-string",
			},
			want: want{
				err: errors.Wrap(errors.New("error parsing regexp: missing closing ]: `[a-z`"), errStringTransformTypeRegexpFailed),
			},
		},
		"RegexpSimpleMatch": {
			args: args{
				stype: v1.StringTransformTypeRegexp,
				regexp: &v1.StringTransformRegexp{
					Match: "[0-9]",
				},
				i: "my-1-string",
			},
			want: want{
				o: "1",
			},
		},
		"RegexpCaptureGroup": {
			args: args{
				stype: v1.StringTransformTypeRegexp,
				regexp: &v1.StringTransformRegexp{
					Match: "my-([0-9]+)-string",
					Group: pointer.Int(1),
				},
				i: "my-1-string",
			},
			want: want{
				o: "1",
			},
		},
		"RegexpNoSuchCaptureGroup": {
			args: args{
				stype: v1.StringTransformTypeRegexp,
				regexp: &v1.StringTransformRegexp{
					Match: "my-([0-9]+)-string",
					Group: pointer.Int(2),
				},
				i: "my-1-string",
			},
			want: want{
				err: errors.Errorf(errStringTransformTypeRegexpNoMatch, "my-([0-9]+)-string", 2),
			},
		},
		"ConvertToJSONSuccess": {
			args: args{
				stype:   v1.StringTransformTypeConvert,
				convert: &toJSON,
				i: map[string]any{
					"foo": "bar",
				},
			},
			want: want{
				o: "{\"foo\":\"bar\"}",
			},
		},
		"ConvertToJSONFail": {
			args: args{
				stype:   v1.StringTransformTypeConvert,
				convert: &toJSON,
				i: map[string]any{
					"foo": func() {},
				},
			},
			want: want{
				o:   "",
				err: errors.Wrap(errors.New("json: unsupported type: func()"), errMarshalJSON),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			tr := v1.StringTransform{Type: tc.stype,
				Format:  tc.fmts,
				Convert: tc.convert,
				Trim:    tc.trim,
				Regexp:  tc.regexp,
			}

			got, err := ResolveString(tr, tc.i)

			if diff := cmp.Diff(tc.want.o, got); diff != "" {
				t.Errorf("Resolve(b): -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("Resolve(b): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestConvertResolve(t *testing.T) {
	type args struct {
		to     v1.TransformIOType
		format *v1.ConvertTransformFormat
		i      any
	}
	type want struct {
		o   any
		err error
	}

	cases := map[string]struct {
		args
		want
	}{
		"StringToBool": {
			args: args{
				i:  "true",
				to: v1.TransformIOTypeBool,
			},
			want: want{
				o: true,
			},
		},
		"StringToFloat64": {
			args: args{
				i:  "1000",
				to: v1.TransformIOTypeFloat64,
			},
			want: want{
				o: 1000.0,
			},
		},
		"StringToQuantityFloat64": {
			args: args{
				i:      "1000m",
				to:     v1.TransformIOTypeFloat64,
				format: (*v1.ConvertTransformFormat)(pointer.String(string(v1.ConvertTransformFormatQuantity))),
			},
			want: want{
				o: 1.0,
			},
		},
		"StringToQuantityFloat64InvalidFormat": {
			args: args{
				i:      "1000 blabla",
				to:     v1.TransformIOTypeFloat64,
				format: (*v1.ConvertTransformFormat)(pointer.String(string(v1.ConvertTransformFormatQuantity))),
			},
			want: want{
				err: resource.ErrFormatWrong,
			},
		},
		"SameTypeNoOp": {
			args: args{
				i:  true,
				to: v1.TransformIOTypeBool,
			},
			want: want{
				o: true,
			},
		},
		"IntAliasToInt64": {
			args: args{
				i:  int64(1),
				to: v1.TransformIOTypeInt,
			},
			want: want{
				o: int64(1),
			},
		},
		"InputTypeNotSupported": {
			args: args{
				i:  []int{64},
				to: v1.TransformIOTypeString,
			},
			want: want{
				err: errors.Errorf(errFmtConvertInputTypeNotSupported, []int{}),
			},
		},
		"ConversionPairFormatNotSupported": {
			args: args{
				i:      100,
				to:     v1.TransformIOTypeString,
				format: (*v1.ConvertTransformFormat)(pointer.String(string(v1.ConvertTransformFormatQuantity))),
			},
			want: want{
				err: errors.Errorf(errConvertFormatPairNotSupported, "int", "string", string(v1.ConvertTransformFormatQuantity)),
			},
		},
		"ConversionPairNotSupported": {
			args: args{
				i:  "[64]",
				to: "[]int",
			},
			want: want{
				err: errors.Errorf(errConvertFormatPairNotSupported, "string", "[]int", string(v1.ConvertTransformFormatNone)),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tr := v1.ConvertTransform{ToType: tc.args.to, Format: tc.format}
			got, err := ResolveConvert(tr, tc.i)

			if diff := cmp.Diff(tc.want.o, got); diff != "" {
				t.Errorf("Resolve(b): -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("Resolve(b): -want, +got:\n%s", diff)
			}
		})
	}
}
