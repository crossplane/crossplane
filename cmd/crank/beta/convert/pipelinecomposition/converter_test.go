/*
Copyright 2024 The Crossplane Authors.

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

package pipelinecomposition

import (
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	commonv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

func TestSetMissingConnectionDetailFields(t *testing.T) {
	kubeconfigKey := "kubeconfig"
	fv := v1.ConnectionDetailTypeFromValue
	ffp := v1.ConnectionDetailTypeFromFieldPath
	fcsk := v1.ConnectionDetailTypeFromConnectionSecretKey
	type args struct {
		sk v1.ConnectionDetail
	}
	type want struct {
		sk v1.ConnectionDetail
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ConnectionDetailMissingKeyAndName": {
			reason: "Correctly add Type and Name",
			args: args{
				sk: v1.ConnectionDetail{
					FromConnectionSecretKey: &kubeconfigKey,
				},
			},
			want: want{
				sk: v1.ConnectionDetail{
					Name:                    &kubeconfigKey,
					FromConnectionSecretKey: &kubeconfigKey,
					Type:                    &fcsk,
				},
			},
		},
		"FromValueMissingType": {
			reason: "Correctly add Type",
			args: args{
				sk: v1.ConnectionDetail{
					Name:  &kubeconfigKey,
					Value: &kubeconfigKey,
				},
			},
			want: want{
				sk: v1.ConnectionDetail{
					Name:  &kubeconfigKey,
					Value: &kubeconfigKey,
					Type:  &fv,
				},
			},
		},
		"FromFieldPathMissingType": {
			reason: "Correctly add Type",
			args: args{
				sk: v1.ConnectionDetail{
					Name:          &kubeconfigKey,
					FromFieldPath: &kubeconfigKey,
				},
			},
			want: want{
				sk: v1.ConnectionDetail{
					Name:          &kubeconfigKey,
					FromFieldPath: &kubeconfigKey,
					Type:          &ffp,
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			sk := setMissingConnectionDetailFields(tc.args.sk)
			if diff := cmp.Diff(tc.want.sk, sk); diff != "" {
				t.Errorf("%s\nsetMissingConnectionDetailFields(...): -want i, +got i:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestConvertPnTToPipeline(t *testing.T) {
	timeNow := metav1.NewTime(time.Now())
	pipelineMode := v1.CompositionModePipeline
	alwaysResolve := commonv1.ResolvePolicyAlways
	typeFromCompositeFieldPath := v1.PatchTypeFromCompositeFieldPath
	fieldPath := "spec.test"
	stringFmt := "test1-%s"
	intp := int64(1010)
	type args struct {
		c               *v1.Composition
		functionRefName string
	}
	type want struct {
		c   *v1.Composition
		err error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NilInput": {
			reason: "Nil Input should return an error",
			args:   args{},
			want: want{
				err: errors.New(errNilComposition),
			},
		},
		"WithExistingPipeline": {
			reason: "If Pipeline Mode is set, return Composition unmodified",
			args: args{
				c: &v1.Composition{
					Spec: v1.CompositionSpec{
						Mode: &pipelineMode,
					},
				},
			},
			want: want{
				c: &v1.Composition{
					Spec: v1.CompositionSpec{
						Mode: &pipelineMode,
					},
				},
				err: nil,
			},
		},
		"WithEnvironmentConfig": {
			reason: "CorrectlyHandleEnvironmentConfig",
			args: args{
				c: &v1.Composition{
					Spec: v1.CompositionSpec{
						PatchSets: []v1.PatchSet{
							{
								Name: "test-patchset",
								Patches: []v1.Patch{
									{
										Type:          v1.PatchTypeFromCompositeFieldPath,
										FromFieldPath: &fieldPath,
										ToFieldPath:   &fieldPath,
										Transforms: []v1.Transform{
											{
												String: &v1.StringTransform{
													Format: &stringFmt,
												},
											},
											{
												Math: &v1.MathTransform{
													Multiply: &intp,
												},
											},
										},
										Policy: &v1.PatchPolicy{
											FromFieldPath: ptr.To(v1.FromFieldPathPolicyOptional),
											MergeOptions: &commonv1.MergeOptions{
												KeepMapValues: ptr.To(true),
											},
										},
									},
									{
										Type:          v1.PatchTypeCombineFromComposite,
										FromFieldPath: &fieldPath,
										ToFieldPath:   &fieldPath,
									},
								},
							},
						},
						Resources: []v1.ComposedTemplate{},
						Environment: &v1.EnvironmentConfiguration{
							EnvironmentConfigs: []v1.EnvironmentSource{
								{
									Type: "Reference",
									Ref: &v1.EnvironmentSourceReference{
										Name: "ref",
									},
								},
							},
							Patches: []v1.EnvironmentPatch{
								{
									Type:          typeFromCompositeFieldPath,
									FromFieldPath: &fieldPath,
									ToFieldPath:   &fieldPath,
								},
								{
									Type:          typeFromCompositeFieldPath,
									FromFieldPath: &fieldPath,
									ToFieldPath:   &fieldPath,
									Transforms: []v1.Transform{
										{
											Type: v1.TransformTypeString,
											String: &v1.StringTransform{
												Format: &stringFmt,
											},
										},
										{
											Type: v1.TransformTypeMath,
											Math: &v1.MathTransform{
												Multiply: &intp,
											},
										},
									},
									Policy: &v1.PatchPolicy{
										MergeOptions: &commonv1.MergeOptions{
											AppendSlice: ptr.To(true),
										},
									},
								},
							},
							Policy: &commonv1.Policy{
								Resolve: &alwaysResolve,
							},
						},
					},
				},
			},
			want: want{
				c: &v1.Composition{
					ObjectMeta: metav1.ObjectMeta{
						CreationTimestamp: timeNow,
					},
					Spec: v1.CompositionSpec{
						Mode: &pipelineMode,
						Environment: &v1.EnvironmentConfiguration{
							EnvironmentConfigs: []v1.EnvironmentSource{
								{
									Type: "Reference",
									Ref: &v1.EnvironmentSourceReference{
										Name: "ref",
									},
								},
							},
							Policy: &commonv1.Policy{
								Resolve: &alwaysResolve,
							},
						},
						Pipeline: []v1.PipelineStep{
							{
								FunctionRef: v1.FunctionReference{Name: "function-patch-and-transform"},
								Step:        "patch-and-transform",
								Input: &runtime.RawExtension{
									Object: &unstructured.Unstructured{
										Object: map[string]any{
											"apiVersion": string("pt.fn.crossplane.io/v1beta1"),
											"kind":       string("Resources"),
											"environment": &Environment{
												Patches: []EnvironmentPatch{
													{
														EnvironmentPatch: v1.EnvironmentPatch{
															Type:          typeFromCompositeFieldPath,
															FromFieldPath: &fieldPath,
															ToFieldPath:   &fieldPath,
														},
													},
													{
														EnvironmentPatch: v1.EnvironmentPatch{
															Type:          typeFromCompositeFieldPath,
															FromFieldPath: &fieldPath,
															ToFieldPath:   &fieldPath,
															Transforms: []v1.Transform{
																{
																	Type: v1.TransformTypeString,
																	String: &v1.StringTransform{
																		Format: &stringFmt,
																		Type:   v1.StringTransformTypeFormat,
																	},
																},
																{
																	Type: v1.TransformTypeMath,
																	Math: &v1.MathTransform{
																		Multiply: &intp,
																		Type:     v1.MathTransformTypeMultiply,
																	},
																},
															},
														},
														Policy: &PatchPolicy{
															ToFieldPath: ptr.To(ToFieldPathPolicyForceMergeObjectsAppendArrays),
														},
													},
												},
											},
											"patchSets": []PatchSet{
												{
													Name: "test-patchset",
													Patches: []Patch{
														{
															Patch: v1.Patch{
																Type:          v1.PatchTypeFromCompositeFieldPath,
																FromFieldPath: &fieldPath,
																ToFieldPath:   &fieldPath,
																Transforms: []v1.Transform{
																	{
																		Type: v1.TransformTypeString,
																		String: &v1.StringTransform{
																			Format: &stringFmt,
																			Type:   v1.StringTransformTypeFormat,
																		},
																	},
																	{
																		Type: v1.TransformTypeMath,
																		Math: &v1.MathTransform{
																			Multiply: &intp,
																			Type:     v1.MathTransformTypeMultiply,
																		},
																	},
																},
															},
															Policy: &PatchPolicy{
																FromFieldPath: ptr.To(v1.FromFieldPathPolicyOptional),
																ToFieldPath:   ptr.To(ToFieldPathPolicyMergeObjects),
															},
														},
														{
															Patch: v1.Patch{
																Type:          v1.PatchTypeCombineFromComposite,
																FromFieldPath: &fieldPath,
																ToFieldPath:   &fieldPath,
															},
														},
													},
												},
											},
											"resources": []ComposedTemplate{},
										},
									},
								},
							},
						},
					},
				},
				err: nil,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := convertPnTToPipeline(tc.args.c, tc.args.functionRefName)
			if diff := cmp.Diff(tc.want.c, got, cmpopts.EquateApproxTime(time.Second*2)); diff != "" {
				t.Errorf("%s\nconvertPnTToPipeline(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("%s\nconvertPnTToPipeline(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestSetTransformTypeRequiredFields(t *testing.T) {
	group := 1
	mult := int64(1024)
	tobase64 := v1.StringConversionTypeToBase64
	type args struct {
		tt v1.Transform
	}
	type want struct {
		tt v1.Transform
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"MathMultiplyMissingType": {
			reason: "Correctly add Type and Name",
			args: args{
				tt: v1.Transform{
					Math: &v1.MathTransform{Multiply: &mult},
				},
			},
			want: want{
				tt: v1.Transform{
					Type: v1.TransformTypeMath,
					Math: &v1.MathTransform{Multiply: &mult, Type: v1.MathTransformTypeMultiply},
				},
			},
		},
		"MathClampMinMissingType": {
			reason: "Correctly add Type and Name",
			args: args{
				tt: v1.Transform{
					Math: &v1.MathTransform{ClampMin: &mult},
				},
			},
			want: want{
				tt: v1.Transform{
					Type: v1.TransformTypeMath,
					Math: &v1.MathTransform{
						ClampMin: &mult,
						Type:     v1.MathTransformTypeClampMin,
					},
				},
			},
		},
		"MathClampMaxMissingType": {
			reason: "Correctly add Type and Name",
			args: args{
				tt: v1.Transform{
					Math: &v1.MathTransform{ClampMax: &mult},
				},
			},
			want: want{
				tt: v1.Transform{
					Type: v1.TransformTypeMath,
					Math: &v1.MathTransform{
						ClampMax: &mult,
						Type:     v1.MathTransformTypeClampMax,
					},
				},
			},
		},
		"StringConvertMissingType": {
			reason: "Correctly add Type and Name",
			args: args{
				tt: v1.Transform{
					String: &v1.StringTransform{
						Convert: &tobase64,
					},
				},
			},
			want: want{
				tt: v1.Transform{
					Type: v1.TransformTypeString,
					String: &v1.StringTransform{
						Type:    v1.StringTransformTypeConvert,
						Convert: &tobase64,
					},
				},
			},
		},
		"StringRegexMissingType": {
			reason: "Correctly add Type and Name",
			args: args{
				tt: v1.Transform{
					String: &v1.StringTransform{
						Regexp: &v1.StringTransformRegexp{
							Match: "'^eu-(.*)-'",
							Group: &group,
						},
					},
				},
			},
			want: want{
				tt: v1.Transform{
					Type: v1.TransformTypeString,
					String: &v1.StringTransform{
						Type: v1.StringTransformTypeRegexp,
						Regexp: &v1.StringTransformRegexp{
							Match: "'^eu-(.*)-'",
							Group: &group,
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tt := setTransformTypeRequiredFields(tc.args.tt)
			if diff := cmp.Diff(tc.want.tt, tt); diff != "" {
				t.Errorf("%s\nsetTransformTypeRequiredFields(...): -want i, +got i:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestProcessFunctionInput(t *testing.T) {
	typeFromCompositeFieldPath := v1.PatchTypeFromCompositeFieldPath
	fieldPath := "spec.test"
	stringFmt := "test2-%s"
	intp := int64(1010)
	type args struct {
		input *Input
	}
	cases := map[string]struct {
		reason string
		args   args
		want   *runtime.RawExtension
	}{
		"EmptyInput": {
			reason: "EmptyInput will generate GVK",
			args: args{
				input: &Input{},
			},
			want: &runtime.RawExtension{
				Object: &unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion":  "pt.fn.crossplane.io/v1beta1",
						"kind":        "Resources",
						"environment": (*Environment)(nil),
						"patchSets":   []PatchSet{},
						"resources":   []ComposedTemplate{},
					},
				},
			},
		},
		"InputDefined": {
			reason: "Input Fields defined",
			args: args{
				input: &Input{
					PatchSets: []v1.PatchSet{
						{
							Name: "test-patchset",
							Patches: []v1.Patch{
								{
									Type:          v1.PatchTypeFromCompositeFieldPath,
									FromFieldPath: &fieldPath,
									ToFieldPath:   &fieldPath,
									Transforms: []v1.Transform{
										{
											String: &v1.StringTransform{
												Format: &stringFmt,
											},
										},
										{
											Math: &v1.MathTransform{
												Multiply: &intp,
											},
										},
									},
								},
								{
									Type:          v1.PatchTypeCombineFromComposite,
									FromFieldPath: &fieldPath,
									ToFieldPath:   &fieldPath,
								},
							},
						},
					},
					Resources: []v1.ComposedTemplate{},
					Environment: &v1.EnvironmentConfiguration{
						Patches: []v1.EnvironmentPatch{
							{
								Type:          typeFromCompositeFieldPath,
								FromFieldPath: &fieldPath,
								ToFieldPath:   &fieldPath,
							},
						},
					},
				},
			},
			want: &runtime.RawExtension{
				Object: &unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "pt.fn.crossplane.io/v1beta1",
						"kind":       "Resources",
						"environment": &Environment{
							Patches: []EnvironmentPatch{
								{
									EnvironmentPatch: v1.EnvironmentPatch{
										Type:          typeFromCompositeFieldPath,
										FromFieldPath: &fieldPath,
										ToFieldPath:   &fieldPath,
									},
								},
							},
						},
						"patchSets": []PatchSet{
							{
								Name: "test-patchset",
								Patches: []Patch{
									{
										Patch: v1.Patch{
											Type:          v1.PatchTypeFromCompositeFieldPath,
											FromFieldPath: &fieldPath,
											ToFieldPath:   &fieldPath,
											Transforms: []v1.Transform{
												{
													Type: v1.TransformTypeString,
													String: &v1.StringTransform{
														Format: &stringFmt,
														Type:   v1.StringTransformTypeFormat,
													},
												},
												{
													Type: v1.TransformTypeMath,
													Math: &v1.MathTransform{
														Multiply: &intp,
														Type:     v1.MathTransformTypeMultiply,
													},
												},
											},
										},
									},
									{
										Patch: v1.Patch{
											Type:          v1.PatchTypeCombineFromComposite,
											FromFieldPath: &fieldPath,
											ToFieldPath:   &fieldPath,
										},
									},
								},
							},
						},
						"resources": []ComposedTemplate{},
					},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := processFunctionInput(tc.args.input)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("%s\nprocessFunctionInput(...): -want i, +got i:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestSetMissingPatchSetFields(t *testing.T) {
	fieldPath := "spec.id"
	stringFmt := "test3-%s"
	intp := int64(1010)
	type args struct {
		patchSet v1.PatchSet
	}
	cases := map[string]struct {
		reason string
		args   args
		want   v1.PatchSet
	}{
		"TransformArrayMissingFields": {
			reason: "Nested missing Types are filled in for a transform array",
			args: args{
				v1.PatchSet{
					Name: "test-patchset",
					Patches: []v1.Patch{
						{
							Type:          v1.PatchTypeFromCompositeFieldPath,
							FromFieldPath: &fieldPath,
							ToFieldPath:   &fieldPath,
							Transforms: []v1.Transform{
								{
									String: &v1.StringTransform{
										Format: &stringFmt,
									},
								},
								{
									Math: &v1.MathTransform{
										Multiply: &intp,
									},
								},
							},
						},
						{
							Type:          v1.PatchTypeCombineFromComposite,
							FromFieldPath: &fieldPath,
							ToFieldPath:   &fieldPath,
						},
					},
				},
			},
			want: v1.PatchSet{
				Name: "test-patchset",
				Patches: []v1.Patch{
					{
						Type:          v1.PatchTypeFromCompositeFieldPath,
						FromFieldPath: &fieldPath,
						ToFieldPath:   &fieldPath,
						Transforms: []v1.Transform{
							{
								Type: v1.TransformTypeString,
								String: &v1.StringTransform{
									Type:   v1.StringTransformTypeFormat,
									Format: &stringFmt,
								},
							},
							{
								Type: v1.TransformTypeMath,
								Math: &v1.MathTransform{
									Type:     v1.MathTransformTypeMultiply,
									Multiply: &intp,
								},
							},
						},
					},
					{
						Type:          v1.PatchTypeCombineFromComposite,
						FromFieldPath: &fieldPath,
						ToFieldPath:   &fieldPath,
					},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := setMissingPatchSetFields(tc.args.patchSet)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("%s\nsetMissingPatchSetFields(...): -want i, +got i:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestSetMissingEnvironmentPatchFields(t *testing.T) {
	fieldPath := "spec.id"
	stringFmt := "test4-%s"
	intp := int64(1010)
	type args struct {
		patch v1.EnvironmentPatch
	}
	cases := map[string]struct {
		reason string
		args   args
		want   v1.EnvironmentPatch
	}{
		"PatchWithoutTransforms": {
			args: args{
				v1.EnvironmentPatch{
					Type:          v1.PatchTypeCombineFromComposite,
					FromFieldPath: &fieldPath,
					ToFieldPath:   &fieldPath,
				},
			},
			want: v1.EnvironmentPatch{
				Type:          v1.PatchTypeCombineFromComposite,
				FromFieldPath: &fieldPath,
				ToFieldPath:   &fieldPath,
			},
		},
		"TransformArrayMissingFields": {
			reason: "Nested missing Types are filled in for a transform array",
			args: args{
				v1.EnvironmentPatch{
					Type:          v1.PatchTypeFromCompositeFieldPath,
					FromFieldPath: &fieldPath,
					ToFieldPath:   &fieldPath,
					Transforms: []v1.Transform{
						{
							String: &v1.StringTransform{
								Format: &stringFmt,
							},
						},
						{
							Math: &v1.MathTransform{
								Multiply: &intp,
							},
						},
					},
				},
			},
			want: v1.EnvironmentPatch{
				Type:          v1.PatchTypeFromCompositeFieldPath,
				FromFieldPath: &fieldPath,
				ToFieldPath:   &fieldPath,
				Transforms: []v1.Transform{
					{
						Type: v1.TransformTypeString,
						String: &v1.StringTransform{
							Type:   v1.StringTransformTypeFormat,
							Format: &stringFmt,
						},
					},
					{
						Type: v1.TransformTypeMath,
						Math: &v1.MathTransform{
							Type:     v1.MathTransformTypeMultiply,
							Multiply: &intp,
						},
					},
				},
			},
		},
		"PatchWithoutType": {
			args: args{
				v1.EnvironmentPatch{
					FromFieldPath: &fieldPath,
					ToFieldPath:   &fieldPath,
				},
			},
			want: v1.EnvironmentPatch{
				Type:          v1.PatchTypeFromCompositeFieldPath,
				FromFieldPath: &fieldPath,
				ToFieldPath:   &fieldPath,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := setMissingEnvironmentPatchFields(tc.args.patch)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("%s\nsetMissingEnvironmentPatchFields(...): -want i, +got i:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestSetMissingPatchFields(t *testing.T) {
	fieldPath := "spec.id"
	stringFmt := "test5-%s"
	intp := int64(1010)
	type args struct {
		patch v1.Patch
	}
	cases := map[string]struct {
		reason string
		args   args
		want   v1.Patch
	}{
		"PatchWithoutTransforms": {
			args: args{
				v1.Patch{
					Type:          v1.PatchTypeCombineFromComposite,
					FromFieldPath: &fieldPath,
					ToFieldPath:   &fieldPath,
				},
			},
			want: v1.Patch{
				Type:          v1.PatchTypeCombineFromComposite,
				FromFieldPath: &fieldPath,
				ToFieldPath:   &fieldPath,
			},
		},
		"TransformArrayMissingFields": {
			reason: "Nested missing Types are filled in for a transform array",
			args: args{
				v1.Patch{
					Type:          v1.PatchTypeFromCompositeFieldPath,
					FromFieldPath: &fieldPath,
					ToFieldPath:   &fieldPath,
					Transforms: []v1.Transform{
						{
							String: &v1.StringTransform{
								Format: &stringFmt,
							},
						},
						{
							Math: &v1.MathTransform{
								Multiply: &intp,
							},
						},
					},
				},
			},
			want: v1.Patch{
				Type:          v1.PatchTypeFromCompositeFieldPath,
				FromFieldPath: &fieldPath,
				ToFieldPath:   &fieldPath,
				Transforms: []v1.Transform{
					{
						Type: v1.TransformTypeString,
						String: &v1.StringTransform{
							Type:   v1.StringTransformTypeFormat,
							Format: &stringFmt,
						},
					},
					{
						Type: v1.TransformTypeMath,
						Math: &v1.MathTransform{
							Type:     v1.MathTransformTypeMultiply,
							Multiply: &intp,
						},
					},
				},
			},
		},
		"PatchWithoutType": {
			args: args{
				v1.Patch{
					FromFieldPath: &fieldPath,
					ToFieldPath:   &fieldPath,
				},
			},
			want: v1.Patch{
				Type:          v1.PatchTypeFromCompositeFieldPath,
				FromFieldPath: &fieldPath,
				ToFieldPath:   &fieldPath,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := setMissingPatchFields(tc.args.patch)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("%s\nsetMissingPatchFields(...): -want i, +got i:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestSetMissingResourceFields(t *testing.T) {
	name := "resource-0"
	empty := ""
	str := "crossplane"
	fcsk := v1.ConnectionDetailTypeFromConnectionSecretKey
	baseNoName := map[string]any{
		"apiVersion": "nop.crossplane.io/v1",
		"kind":       "TestResource",
		"spec":       map[string]any{},
	}

	type args struct {
		idx int
		rs  v1.ComposedTemplate
	}
	cases := map[string]struct {
		reason string
		args   args
		want   v1.ComposedTemplate
	}{
		"NoNameProvided": {
			reason: "ResourceName Not provided",
			args: args{
				rs: v1.ComposedTemplate{
					Base: runtime.RawExtension{
						Object: &unstructured.Unstructured{Object: baseNoName},
					},
					Patches:           []v1.Patch{},
					ConnectionDetails: []v1.ConnectionDetail{},
				},
			},
			want: v1.ComposedTemplate{
				Name: &name,
				Base: runtime.RawExtension{
					Object: &unstructured.Unstructured{Object: baseNoName},
				},
				Patches:           []v1.Patch{},
				ConnectionDetails: []v1.ConnectionDetail{},
			},
		},
		"EmptyNameProvided": {
			reason: "ResourceName Not provided",
			args: args{
				rs: v1.ComposedTemplate{
					Name: &empty,
					Base: runtime.RawExtension{
						Object: &unstructured.Unstructured{Object: baseNoName},
					},
					Patches:           []v1.Patch{},
					ConnectionDetails: []v1.ConnectionDetail{},
				},
			},
			want: v1.ComposedTemplate{
				Name: &name,
				Base: runtime.RawExtension{
					Object: &unstructured.Unstructured{Object: baseNoName},
				},
				Patches:           []v1.Patch{},
				ConnectionDetails: []v1.ConnectionDetail{},
			},
		},
		"NameProvidedWithConnectionDetail": {
			reason: "ResourceName Not provided",
			args: args{
				rs: v1.ComposedTemplate{
					Name: &name,
					Base: runtime.RawExtension{
						Object: &unstructured.Unstructured{Object: baseNoName},
					},
					Patches: []v1.Patch{},
					ConnectionDetails: []v1.ConnectionDetail{
						{FromConnectionSecretKey: &str},
					},
				},
			},
			want: v1.ComposedTemplate{
				Name: &name,
				Base: runtime.RawExtension{
					Object: &unstructured.Unstructured{Object: baseNoName},
				},
				Patches: []v1.Patch{},
				ConnectionDetails: []v1.ConnectionDetail{
					{
						FromConnectionSecretKey: &str,
						Type:                    &fcsk,
						Name:                    &str,
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := setMissingResourceFields(tc.args.idx, tc.args.rs)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("%s\nsetMissingResourceFields(...): -want i, +got i:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestMigratePatchPolicyInResources(t *testing.T) {
	cases := map[string]struct {
		reason string
		args   []v1.ComposedTemplate
		want   []ComposedTemplate
	}{
		"ResourcesHasSimplePatches": {
			reason: "Composed Resources has simple patches",
			args: []v1.ComposedTemplate{
				{
					Name: ptr.To("resource-0"),
					Patches: []v1.Patch{
						{
							Type:          v1.PatchTypeToCompositeFieldPath,
							FromFieldPath: ptr.To("envVal"),
							ToFieldPath:   ptr.To("spec.val"),
							Policy:        nil,
						},
						{
							Type:          v1.PatchTypeToCompositeFieldPath,
							FromFieldPath: ptr.To("envVal"),
							ToFieldPath:   ptr.To("spec.val"),
							Policy: &v1.PatchPolicy{
								FromFieldPath: ptr.To(v1.FromFieldPathPolicyOptional),
								MergeOptions: &commonv1.MergeOptions{
									KeepMapValues: ptr.To(true),
								},
							},
						},
					},
				},
			},
			want: []ComposedTemplate{
				{
					ComposedTemplate: v1.ComposedTemplate{
						Name:    ptr.To("resource-0"),
						Patches: nil,
					},
					Patches: []Patch{
						{
							Patch: v1.Patch{
								Type:          v1.PatchTypeToCompositeFieldPath,
								FromFieldPath: ptr.To("envVal"),
								ToFieldPath:   ptr.To("spec.val"),
								Policy:        nil,
							},
							Policy: nil,
						},
						{
							Patch: v1.Patch{
								Type:          v1.PatchTypeToCompositeFieldPath,
								FromFieldPath: ptr.To("envVal"),
								ToFieldPath:   ptr.To("spec.val"),
								Policy:        nil,
							},
							Policy: &PatchPolicy{
								FromFieldPath: ptr.To(v1.FromFieldPathPolicyOptional),
								ToFieldPath:   ptr.To(ToFieldPathPolicyMergeObjects),
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := MigratePatchPolicyInResources(tc.args)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("MigratePatchPolicyInResources() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMigratePatchPolicyInPatchSets(t *testing.T) {
	cases := map[string]struct {
		reason string
		args   []v1.PatchSet
		want   []PatchSet
	}{
		"PatchSetHasSimplePatches": {
			reason: "PatchSet has simple patches",
			args: []v1.PatchSet{
				{
					Name: "patchset-0",
					Patches: []v1.Patch{
						{
							Type:          v1.PatchTypeToCompositeFieldPath,
							FromFieldPath: ptr.To("envVal"),
							ToFieldPath:   ptr.To("spec.val"),
							Policy:        nil,
						},
						{
							Type:          v1.PatchTypeToCompositeFieldPath,
							FromFieldPath: ptr.To("envVal"),
							ToFieldPath:   ptr.To("spec.val"),
							Policy: &v1.PatchPolicy{
								FromFieldPath: ptr.To(v1.FromFieldPathPolicyOptional),
								MergeOptions: &commonv1.MergeOptions{
									KeepMapValues: ptr.To(true),
								},
							},
						},
					},
				},
			},
			want: []PatchSet{
				{
					Name: "patchset-0",
					Patches: []Patch{
						{
							Patch: v1.Patch{
								Type:          v1.PatchTypeToCompositeFieldPath,
								FromFieldPath: ptr.To("envVal"),
								ToFieldPath:   ptr.To("spec.val"),
								Policy:        nil,
							},
							Policy: nil,
						},
						{
							Patch: v1.Patch{
								Type:          v1.PatchTypeToCompositeFieldPath,
								FromFieldPath: ptr.To("envVal"),
								ToFieldPath:   ptr.To("spec.val"),
								Policy:        nil,
							},
							Policy: &PatchPolicy{
								FromFieldPath: ptr.To(v1.FromFieldPathPolicyOptional),
								ToFieldPath:   ptr.To(ToFieldPathPolicyMergeObjects),
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := MigratePatchPolicyInPatchSets(tc.args)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("MigratePatchPolicyInPatchSets() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMigratePatchPolicyInEnvironment(t *testing.T) {
	cases := map[string]struct {
		reason string
		args   *v1.EnvironmentConfiguration
		want   *Environment
	}{
		"EnvironmentNil": {
			reason: "Environment is nil",
			args:   nil,
			want:   nil,
		},
		"EnvironmentHasNoPatches": {
			reason: "Environment has no patches",
			args:   &v1.EnvironmentConfiguration{Patches: []v1.EnvironmentPatch{}},
			want:   nil,
		},
		"EnvironmentHasSimplePatches": {
			reason: "Environment has simple patches",
			args: &v1.EnvironmentConfiguration{
				Patches: []v1.EnvironmentPatch{
					{
						Type:          v1.PatchTypeToCompositeFieldPath,
						FromFieldPath: ptr.To("envVal"),
						ToFieldPath:   ptr.To("spec.val"),
						Policy:        nil,
					},
					{
						Type:          v1.PatchTypeToCompositeFieldPath,
						FromFieldPath: ptr.To("envVal"),
						ToFieldPath:   ptr.To("spec.val"),
						Policy: &v1.PatchPolicy{
							FromFieldPath: ptr.To(v1.FromFieldPathPolicyOptional),
							MergeOptions: &commonv1.MergeOptions{
								KeepMapValues: ptr.To(true),
							},
						},
					},
				},
			},
			want: &Environment{
				Patches: []EnvironmentPatch{
					{
						EnvironmentPatch: v1.EnvironmentPatch{
							Type:          v1.PatchTypeToCompositeFieldPath,
							FromFieldPath: ptr.To("envVal"),
							ToFieldPath:   ptr.To("spec.val"),
							Policy:        nil,
						},
						Policy: nil,
					},
					{
						EnvironmentPatch: v1.EnvironmentPatch{
							Type:          v1.PatchTypeToCompositeFieldPath,
							FromFieldPath: ptr.To("envVal"),
							ToFieldPath:   ptr.To("spec.val"),
							Policy:        nil,
						},
						Policy: &PatchPolicy{
							FromFieldPath: ptr.To(v1.FromFieldPathPolicyOptional),
							ToFieldPath:   ptr.To(ToFieldPathPolicyMergeObjects),
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := MigratePatchPolicyInEnvironment(tc.args)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("MigratePatchPolicyInEnvironment() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

/*
#	MergeOptions    appendSlice     keepMapValues   policy.toFieldPath
1	nil             N/A             n/A             nil (defaults to Replace)
2	non-nil         nil or false    true            MergeObjects
3	non-nil         true            nil or false    ForceMergeObjectsAppendArrays
4	non-nil         nil or false    nil or false    ForceMergeObjects
5	non-nil         true            True            MergeObjectsAppendArrays
*/

func TestPatchPolicy(t *testing.T) {
	cases := map[string]struct {
		reason string
		args   *v1.PatchPolicy
		want   *PatchPolicy
	}{
		"PatchPolicyWithNilMergeOptionsAndFromFieldPath": { // case 1
			reason: "MergeOptions and FromFieldPath are nil",
			args: &v1.PatchPolicy{
				FromFieldPath: nil,
				MergeOptions:  nil,
			},
			want: nil,
		},
		"PatchPolicyWithNilMergeOptions": { // case 1
			reason: "MergeOptions is nil",
			args: &v1.PatchPolicy{
				FromFieldPath: ptr.To(v1.FromFieldPathPolicyOptional),
				MergeOptions:  nil,
			},
			want: &PatchPolicy{
				FromFieldPath: ptr.To(v1.FromFieldPathPolicyOptional),
				ToFieldPath:   nil,
			},
		},
		"PatchPolicyWithKeepMapValuesTrueAppendSliceNil": {
			reason: "AppendSlice is nil && KeepMapValues is true", // case 2
			args: &v1.PatchPolicy{
				FromFieldPath: ptr.To(v1.FromFieldPathPolicyOptional),
				MergeOptions: &commonv1.MergeOptions{
					KeepMapValues: ptr.To(true),
				},
			},
			want: &PatchPolicy{
				FromFieldPath: ptr.To(v1.FromFieldPathPolicyOptional),
				ToFieldPath:   ptr.To(ToFieldPathPolicyMergeObjects),
			},
		},
		"PatchPolicyWithKeepMapValuesTrueAppendSliceFalse": {
			reason: "AppendSlice is false && KeepMapValues is true", // case 2
			args: &v1.PatchPolicy{
				FromFieldPath: ptr.To(v1.FromFieldPathPolicyOptional),
				MergeOptions: &commonv1.MergeOptions{
					KeepMapValues: ptr.To(true),
					AppendSlice:   ptr.To(false),
				},
			},
			want: &PatchPolicy{
				FromFieldPath: ptr.To(v1.FromFieldPathPolicyOptional),
				ToFieldPath:   ptr.To(ToFieldPathPolicyMergeObjects),
			},
		},
		"PatchPolicyWithTrueAppendSliceInMergeOptions": { // case 3
			reason: "AppendSlice is true && KeepMapValues is nil",
			args: &v1.PatchPolicy{
				FromFieldPath: ptr.To(v1.FromFieldPathPolicyOptional),
				MergeOptions: &commonv1.MergeOptions{
					AppendSlice: ptr.To(true),
				},
			},
			want: &PatchPolicy{
				FromFieldPath: ptr.To(v1.FromFieldPathPolicyOptional),
				ToFieldPath:   ptr.To(ToFieldPathPolicyForceMergeObjectsAppendArrays),
			},
		},
		"PatchPolicyWithTrueAppendSliceFalseKeepMapValuesInMergeOptions": { // case 3
			reason: "AppendSlice is true && KeepMapValues is false",
			args: &v1.PatchPolicy{
				FromFieldPath: ptr.To(v1.FromFieldPathPolicyOptional),
				MergeOptions: &commonv1.MergeOptions{
					AppendSlice:   ptr.To(true),
					KeepMapValues: ptr.To(false),
				},
			},
			want: &PatchPolicy{
				FromFieldPath: ptr.To(v1.FromFieldPathPolicyOptional),
				ToFieldPath:   ptr.To(ToFieldPathPolicyForceMergeObjectsAppendArrays),
			},
		},
		"PatchPolicyWithEmptyMergeOptions": { // case 4
			reason: "Both AppendSlice and KeepMapValues are nil",
			args: &v1.PatchPolicy{
				FromFieldPath: ptr.To(v1.FromFieldPathPolicyOptional),
				MergeOptions:  &commonv1.MergeOptions{},
			},
			want: &PatchPolicy{
				FromFieldPath: ptr.To(v1.FromFieldPathPolicyOptional),
				ToFieldPath:   ptr.To(ToFieldPathPolicyForceMergeObjects),
			},
		},
		"PatchPolicyWithNilKeepMapValuesInMergeOptions": { // case 4
			reason: "AppendSlice is false and KeepMapValues is nil",
			args: &v1.PatchPolicy{
				FromFieldPath: ptr.To(v1.FromFieldPathPolicyOptional),
				MergeOptions: &commonv1.MergeOptions{
					AppendSlice: ptr.To(false),
				},
			},
			want: &PatchPolicy{
				FromFieldPath: ptr.To(v1.FromFieldPathPolicyOptional),
				ToFieldPath:   ptr.To(ToFieldPathPolicyForceMergeObjects),
			},
		},
		"PatchPolicyWithNilAppendSliceInMergeOptions": { // case 4
			reason: "AppendSlice is nil and KeepMapValues is false",
			args: &v1.PatchPolicy{
				FromFieldPath: ptr.To(v1.FromFieldPathPolicyOptional),
				MergeOptions: &commonv1.MergeOptions{
					KeepMapValues: ptr.To(false),
				},
			},
			want: &PatchPolicy{
				FromFieldPath: ptr.To(v1.FromFieldPathPolicyOptional),
				ToFieldPath:   ptr.To(ToFieldPathPolicyForceMergeObjects),
			},
		},
		"PatchPolicyWithBothKeepMapValuesAndAppendSliceFalse": { // case 4
			reason: "Both KeepMapValues and AppendSlice is false",
			args: &v1.PatchPolicy{
				FromFieldPath: nil,
				MergeOptions: &commonv1.MergeOptions{
					KeepMapValues: ptr.To(false),
					AppendSlice:   ptr.To(false),
				},
			},
			want: &PatchPolicy{
				FromFieldPath: nil,
				ToFieldPath:   ptr.To(ToFieldPathPolicyForceMergeObjects),
			},
		},
		"PatchPolicyWithKeepMapValuesTrueAppendSliceTrue": { // case 5
			reason: "Both KeepMapValues and AppendSlice is true",
			args: &v1.PatchPolicy{
				FromFieldPath: ptr.To(v1.FromFieldPathPolicyRequired),
				MergeOptions: &commonv1.MergeOptions{
					KeepMapValues: ptr.To(true),
					AppendSlice:   ptr.To(true),
				},
			},
			want: &PatchPolicy{
				FromFieldPath: ptr.To(v1.FromFieldPathPolicyRequired),
				ToFieldPath:   ptr.To(ToFieldPathPolicyMergeObjectsAppendArrays),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := migratePatchPolicy(tc.args)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("%s\npatchPolicy(...): -want i, +got i:\n%s", tc.reason, diff)
			}
		})
	}
}
