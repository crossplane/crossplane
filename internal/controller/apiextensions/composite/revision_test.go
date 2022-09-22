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
	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
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
	rev := &v1alpha1.CompositionRevision{
		Spec: v1alpha1.CompositionRevisionSpec{
			CompositeTypeRef: v1alpha1.TypeReference{
				APIVersion: "v",
				Kind:       "k",
			},
			PatchSets: []v1alpha1.PatchSet{{
				Name: "p",
				Patches: []v1alpha1.Patch{{
					Type:          v1alpha1.PatchType("t"),
					FromFieldPath: pointer.String("from"),
					Combine: &v1alpha1.Combine{
						Strategy: v1alpha1.CombineStrategy("s"),
						Variables: []v1alpha1.CombineVariable{{
							FromFieldPath: "from",
						}},
						String: &v1alpha1.StringCombine{
							Format: "f",
						},
					},
					ToFieldPath:  pointer.String("to"),
					PatchSetName: pointer.String("n"),
					Transforms: []v1alpha1.Transform{{
						Type: v1alpha1.TransformType("t"),
						Math: &v1alpha1.MathTransform{
							Multiply: pointer.Int64(42),
						},
						Map: &v1alpha1.MapTransform{
							Pairs: map[string]extv1.JSON{"k": asJSON("v")},
						},
						String: &v1alpha1.StringTransform{
							Type:   v1alpha1.StringTransformTypeFormat,
							Format: pointer.String("f"),
						},
						Convert: &v1alpha1.ConvertTransform{
							ToType: "t",
						},
					}},
					Policy: &v1alpha1.PatchPolicy{
						FromFieldPath: func() *v1alpha1.FromFieldPathPolicy {
							p := v1alpha1.FromFieldPathPolicy("p")
							return &p
						}(),
					},
				}},
			}},
			Resources: []v1alpha1.ComposedTemplate{{
				Name: pointer.String("t"),
				Base: runtime.RawExtension{Raw: []byte("bytes")},
				Patches: []v1alpha1.Patch{{
					Type:          v1alpha1.PatchType("t"),
					FromFieldPath: pointer.String("from"),
					Combine: &v1alpha1.Combine{
						Strategy: v1alpha1.CombineStrategy("s"),
						Variables: []v1alpha1.CombineVariable{{
							FromFieldPath: "from",
						}},
						String: &v1alpha1.StringCombine{
							Format: "f",
						},
					},
					ToFieldPath:  pointer.String("to"),
					PatchSetName: pointer.String("n"),
					Transforms: []v1alpha1.Transform{
						{
							Type: v1alpha1.TransformTypeMath,
							Math: &v1alpha1.MathTransform{
								Multiply: pointer.Int64(42),
							},
						},
						{
							Type: v1alpha1.TransformTypeMap,
							Map: &v1alpha1.MapTransform{
								Pairs: map[string]extv1.JSON{"k": asJSON("v")},
							},
						},
						{
							Type: v1alpha1.TransformTypeString,
							String: &v1alpha1.StringTransform{
								Type:   v1alpha1.StringTransformTypeFormat,
								Format: pointer.String("fmt"),
							},
						},
						{
							Type: v1alpha1.TransformTypeString,
							String: &v1alpha1.StringTransform{
								Type: v1alpha1.StringTransformTypeConvert,
								Convert: func() *v1alpha1.StringConversionType {
									t := v1alpha1.StringConversionTypeToUpper
									return &t
								}(),
							},
						},
						{
							Type: v1alpha1.TransformTypeString,
							String: &v1alpha1.StringTransform{
								Type: v1alpha1.StringTransformTypeTrimSuffix,
								Trim: pointer.String("trim"),
							},
						},
						{
							Type: v1alpha1.TransformTypeString,
							String: &v1alpha1.StringTransform{
								Type: v1alpha1.StringTransformTypeRegexp,
								Regexp: &v1alpha1.StringTransformRegexp{
									Match: "https://twitter.com/junyer/status/699892454749700096",
									Group: pointer.Int(0),
								},
							},
						},
						{
							Type: v1alpha1.TransformTypeConvert,
							Convert: &v1alpha1.ConvertTransform{
								ToType: v1alpha1.ConvertTransformTypeBool,
							},
						},
					},
					Policy: &v1alpha1.PatchPolicy{
						FromFieldPath: func() *v1alpha1.FromFieldPathPolicy {
							p := v1alpha1.FromFieldPathPolicy("p")
							return &p
						}(),
					},
				}},
				ConnectionDetails: []v1alpha1.ConnectionDetail{{
					Name: pointer.String("cd"),
					Type: func() *v1alpha1.ConnectionDetailType {
						t := v1alpha1.ConnectionDetailType("t")
						return &t
					}(),
					FromConnectionSecretKey: pointer.String("k"),
					FromFieldPath:           pointer.String("p"),
					Value:                   pointer.String("v"),
				}},
				ReadinessChecks: []v1alpha1.ReadinessCheck{{
					Type:         v1alpha1.ReadinessCheckType("c"),
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
