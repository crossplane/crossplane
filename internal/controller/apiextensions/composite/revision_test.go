/*
Copyright 2021 The Crossplane Authors.

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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/apis/apiextensions/v1beta1"
)

func TestAsComposition(t *testing.T) {
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

	sf := "f"
	rev := &v1beta1.CompositionRevision{
		Spec: v1beta1.CompositionRevisionSpec{
			CompositeTypeRef: v1beta1.TypeReference{
				APIVersion: "v",
				Kind:       "k",
			},
			PatchSets: []v1beta1.PatchSet{{
				Name: "p",
				Patches: []v1beta1.Patch{{
					Type:          v1beta1.PatchType("t"),
					FromFieldPath: pointer.String("from"),
					Combine: &v1beta1.Combine{
						Strategy: v1beta1.CombineStrategy("s"),
						Variables: []v1beta1.CombineVariable{{
							FromFieldPath: "from",
						}},
						String: &v1beta1.StringCombine{
							Format: "f",
						},
					},
					ToFieldPath:  pointer.String("to"),
					PatchSetName: pointer.String("n"),
					Transforms: []v1beta1.Transform{{
						Type: v1beta1.TransformType("t"),
						Math: &v1beta1.MathTransform{
							Multiply: pointer.Int64(42),
						},
						Map: &v1beta1.MapTransform{
							Pairs: map[string]extv1.JSON{"k": asJSON("v")},
						},
						Match: &v1beta1.MatchTransform{
							Patterns: []v1beta1.MatchTransformPattern{
								{
									Type:    v1beta1.MatchTransformPatternTypeLiteral,
									Literal: pointer.String("literal"),
									Regexp:  pointer.String("regexp"),
									Result:  asJSON("value"),
								},
							},
							FallbackValue: asJSON("value"),
						},
						String: &v1beta1.StringTransform{
							Type:   v1beta1.StringTransformTypeFormat,
							Format: pointer.String("f"),
						},
						Convert: &v1beta1.ConvertTransform{
							ToType: "t",
						},
					}},
					Policy: &v1beta1.PatchPolicy{
						FromFieldPath: func() *v1beta1.FromFieldPathPolicy {
							p := v1beta1.FromFieldPathPolicy("p")
							return &p
						}(),
					},
				}},
			}},
			Resources: []v1beta1.ComposedTemplate{{
				Name: pointer.String("t"),
				Base: runtime.RawExtension{Raw: []byte("bytes")},
				Patches: []v1beta1.Patch{{
					Type:          v1beta1.PatchType("t"),
					FromFieldPath: pointer.String("from"),
					Combine: &v1beta1.Combine{
						Strategy: v1beta1.CombineStrategy("s"),
						Variables: []v1beta1.CombineVariable{{
							FromFieldPath: "from",
						}},
						String: &v1beta1.StringCombine{
							Format: "f",
						},
					},
					ToFieldPath:  pointer.String("to"),
					PatchSetName: pointer.String("n"),
					Transforms: []v1beta1.Transform{
						{
							Type: v1beta1.TransformTypeMath,
							Math: &v1beta1.MathTransform{
								Multiply: pointer.Int64(42),
							},
						},
						{
							Type: v1beta1.TransformTypeMap,
							Map: &v1beta1.MapTransform{
								Pairs: map[string]extv1.JSON{"k": asJSON("v")},
							},
						},
						{
							Type: v1beta1.TransformTypeMatch,
							Match: &v1beta1.MatchTransform{
								Patterns: []v1beta1.MatchTransformPattern{
									{
										Type:    v1beta1.MatchTransformPatternTypeLiteral,
										Literal: pointer.String("literal"),
										Regexp:  pointer.String("regexp"),
										Result:  asJSON("value"),
									},
								},
								FallbackValue: asJSON("value"),
							},
						},
						{
							Type: v1beta1.TransformTypeString,
							String: &v1beta1.StringTransform{
								Type:   v1beta1.StringTransformTypeFormat,
								Format: pointer.String("fmt"),
							},
						},
						{
							Type: v1beta1.TransformTypeString,
							String: &v1beta1.StringTransform{
								Type: v1beta1.StringTransformTypeConvert,
								Convert: func() *v1beta1.StringConversionType {
									t := v1beta1.StringConversionTypeToUpper
									return &t
								}(),
							},
						},
						{
							Type: v1beta1.TransformTypeString,
							String: &v1beta1.StringTransform{
								Type: v1beta1.StringTransformTypeTrimSuffix,
								Trim: pointer.String("trim"),
							},
						},
						{
							Type: v1beta1.TransformTypeString,
							String: &v1beta1.StringTransform{
								Type: v1beta1.StringTransformTypeRegexp,
								Regexp: &v1beta1.StringTransformRegexp{
									Match: "https://twitter.com/junyer/status/699892454749700096",
									Group: pointer.Int(0),
								},
							},
						},
						{
							Type: v1beta1.TransformTypeConvert,
							Convert: &v1beta1.ConvertTransform{
								ToType: v1beta1.ConvertTransformTypeBool,
							},
						},
					},
					Policy: &v1beta1.PatchPolicy{
						FromFieldPath: func() *v1beta1.FromFieldPathPolicy {
							p := v1beta1.FromFieldPathPolicy("p")
							return &p
						}(),
					},
				}},
				ConnectionDetails: []v1beta1.ConnectionDetail{{
					Name: pointer.String("cd"),
					Type: func() *v1beta1.ConnectionDetailType {
						t := v1beta1.ConnectionDetailType("t")
						return &t
					}(),
					FromConnectionSecretKey: pointer.String("k"),
					FromFieldPath:           pointer.String("p"),
					Value:                   pointer.String("v"),
				}},
				ReadinessChecks: []v1beta1.ReadinessCheck{{
					Type:         v1beta1.ReadinessCheckType("c"),
					FieldPath:    "p",
					MatchString:  "s",
					MatchInteger: 42,
				}},
			}},
		},
	}

	want := &v1.Composition{
		Spec: v1.CompositionSpec{
			CompositeTypeRef: v1.TypeReference{
				APIVersion: "v",
				Kind:       "k",
			},
			PatchSets: []v1.PatchSet{{
				Name: "p",
				Patches: []v1.Patch{{
					Type:          v1.PatchType("t"),
					FromFieldPath: pointer.String("from"),
					Combine: &v1.Combine{
						Strategy: v1.CombineStrategy("s"),
						Variables: []v1.CombineVariable{{
							FromFieldPath: "from",
						}},
						String: &v1.StringCombine{
							Format: "f",
						},
					},
					ToFieldPath:  pointer.String("to"),
					PatchSetName: pointer.String("n"),
					Transforms: []v1.Transform{{
						Type: v1.TransformType("t"),
						Math: &v1.MathTransform{
							Multiply: pointer.Int64(42),
						},
						Map: &v1.MapTransform{
							Pairs: map[string]extv1.JSON{"k": asJSON("v")},
						},
						Match: &v1.MatchTransform{
							Patterns: []v1.MatchTransformPattern{
								{
									Type:    v1.MatchTransformPatternTypeLiteral,
									Literal: pointer.String("literal"),
									Regexp:  pointer.String("regexp"),
									Result:  asJSON("value"),
								},
							},
							FallbackValue: asJSON("value"),
						},
						String: &v1.StringTransform{
							Type:   v1.StringTransformTypeFormat,
							Format: &sf,
						},
						Convert: &v1.ConvertTransform{
							ToType: "t",
						},
					}},
					Policy: &v1.PatchPolicy{
						FromFieldPath: func() *v1.FromFieldPathPolicy {
							p := v1.FromFieldPathPolicy("p")
							return &p
						}(),
					},
				}},
			}},
			Resources: []v1.ComposedTemplate{{
				Name: pointer.String("t"),
				Base: runtime.RawExtension{Raw: []byte("bytes")},
				Patches: []v1.Patch{{
					Type:          v1.PatchType("t"),
					FromFieldPath: pointer.String("from"),
					Combine: &v1.Combine{
						Strategy: v1.CombineStrategy("s"),
						Variables: []v1.CombineVariable{{
							FromFieldPath: "from",
						}},
						String: &v1.StringCombine{
							Format: "f",
						},
					},
					ToFieldPath:  pointer.String("to"),
					PatchSetName: pointer.String("n"),
					Transforms: []v1.Transform{
						{
							Type: v1.TransformTypeMath,
							Math: &v1.MathTransform{
								Multiply: pointer.Int64(42),
							},
						},
						{
							Type: v1.TransformTypeMap,
							Map: &v1.MapTransform{
								Pairs: map[string]extv1.JSON{"k": asJSON("v")},
							},
						},
						{
							Type: v1.TransformTypeMatch,
							Match: &v1.MatchTransform{
								Patterns: []v1.MatchTransformPattern{
									{
										Type:    v1.MatchTransformPatternTypeLiteral,
										Literal: pointer.String("literal"),
										Regexp:  pointer.String("regexp"),
										Result:  asJSON("value"),
									},
								},
								FallbackValue: asJSON("value"),
							},
						},
						{
							Type: v1.TransformTypeString,
							String: &v1.StringTransform{
								Type:   v1.StringTransformTypeFormat,
								Format: pointer.String("fmt"),
							},
						},
						{
							Type: v1.TransformTypeString,
							String: &v1.StringTransform{
								Type: v1.StringTransformTypeConvert,
								Convert: func() *v1.StringConversionType {
									t := v1.StringConversionTypeToUpper
									return &t
								}(),
							},
						},
						{
							Type: v1.TransformTypeString,
							String: &v1.StringTransform{
								Type: v1.StringTransformTypeTrimSuffix,
								Trim: pointer.String("trim"),
							},
						},
						{
							Type: v1.TransformTypeString,
							String: &v1.StringTransform{
								Type: v1.StringTransformTypeRegexp,
								Regexp: &v1.StringTransformRegexp{
									Match: "https://twitter.com/junyer/status/699892454749700096",
									Group: pointer.Int(0),
								},
							},
						},
						{
							Type: v1.TransformTypeConvert,
							Convert: &v1.ConvertTransform{
								ToType: v1.ConvertTransformTypeBool,
							},
						},
					},
					Policy: &v1.PatchPolicy{
						FromFieldPath: func() *v1.FromFieldPathPolicy {
							p := v1.FromFieldPathPolicy("p")
							return &p
						}(),
					},
				}},
				ConnectionDetails: []v1.ConnectionDetail{{
					Name: pointer.String("cd"),
					Type: func() *v1.ConnectionDetailType {
						t := v1.ConnectionDetailType("t")
						return &t
					}(),
					FromConnectionSecretKey: pointer.String("k"),
					FromFieldPath:           pointer.String("p"),
					Value:                   pointer.String("v"),
				}},
				ReadinessChecks: []v1.ReadinessCheck{{
					Type:         v1.ReadinessCheckType("c"),
					FieldPath:    "p",
					MatchString:  "s",
					MatchInteger: 42,
				}},
			}},
		},
	}

	got := AsComposition(rev)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("AsComposition(): -want, +got:\n%s", diff)
	}

}
