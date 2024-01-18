package pipelinecomposition

import (
	"errors"
	"testing"
	"time"

	commonv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
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
			sk := SetMissingConnectionDetailFields(tc.args.sk)
			if diff := cmp.Diff(tc.want.sk, sk); diff != "" {
				t.Errorf("%s\nPopulateConnectionSecret(...): -want i, +got i:\n%s", tc.reason, diff)
			}

		})
	}
}

func TestNewPipelineCompositionFromExisting(t *testing.T) {
	timeNow := metav1.NewTime(time.Now())
	pipelineMode := v1.CompositionModePipeline
	alwaysResolve := commonv1.ResolvePolicyAlways
	typeFromCompositeFieldPath := v1.PatchTypeFromCompositeFieldPath
	fieldPath := "spec.test"
	stringFmt := "test-%s"
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
				err: errors.New(ErrNilComposition),
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
											"environment": &v1.EnvironmentConfiguration{
												Patches: []v1.EnvironmentPatch{
													{
														Type:          typeFromCompositeFieldPath,
														FromFieldPath: &fieldPath,
														ToFieldPath:   &fieldPath,
													},
												},
											},
											"patchSets": []v1.PatchSet{
												{
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
														{
															Type:          v1.PatchTypeCombineFromComposite,
															FromFieldPath: &fieldPath,
															ToFieldPath:   &fieldPath,
														},
													},
												},
											},
											"resources": []v1.ComposedTemplate{},
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
			got, err := NewPipelineCompositionFromExisting(tc.args.c, tc.args.functionRefName)
			if diff := cmp.Diff(tc.want.c, got, cmpopts.EquateApproxTime(time.Second*2)); diff != "" {
				t.Errorf("%s\nNewPipelineCompositionFromExisting(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, EquateErrors()); diff != "" {
				t.Errorf("%s\nNewPipelineCompositionFromExisting(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestSetTransformTypeRequiredFields(t *testing.T) {
	group := int(1)
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
			tt := SetTransformTypeRequiredFields(tc.args.tt)
			if diff := cmp.Diff(tc.want.tt, tt); diff != "" {
				t.Errorf("%s\nPopulateTransformType(...): -want i, +got i:\n%s", tc.reason, diff)
			}

		})
	}
}

func EquateErrors() cmp.Option {
	return cmp.Comparer(func(a, b error) bool {
		if a == nil || b == nil {
			return a == nil && b == nil
		}
		return a.Error() == b.Error()
	})
}

func TestNewPatchAndTransformFunctionInput(t *testing.T) {
	typeFromCompositeFieldPath := v1.PatchTypeFromCompositeFieldPath
	fieldPath := "spec.test"
	stringFmt := "test-%s"
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
						"apiVersion":  string("pt.fn.crossplane.io/v1beta1"),
						"kind":        string("Resources"),
						"environment": (*v1.EnvironmentConfiguration)(nil),
						"patchSets":   []v1.PatchSet{},
						"resources":   []v1.ComposedTemplate{},
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
						"apiVersion": string("pt.fn.crossplane.io/v1beta1"),
						"kind":       string("Resources"),
						"environment": &v1.EnvironmentConfiguration{
							Patches: []v1.EnvironmentPatch{
								{
									Type:          typeFromCompositeFieldPath,
									FromFieldPath: &fieldPath,
									ToFieldPath:   &fieldPath,
								},
							},
						},
						"patchSets": []v1.PatchSet{
							{
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
									{
										Type:          v1.PatchTypeCombineFromComposite,
										FromFieldPath: &fieldPath,
										ToFieldPath:   &fieldPath,
									},
								},
							},
						},
						"resources": []v1.ComposedTemplate{},
					},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := NewPatchAndTransformFunctionInput(tc.args.input)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("%s\nNewPatchAndTransformFunctionInput(...): -want i, +got i:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestSetMissingPatchSetFields(t *testing.T) {
	fieldPath := "spec.id"
	stringFmt := "test-%s"
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
			got := SetMissingPatchSetFields(tc.args.patchSet)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("%s\nNewSetMissingPatchFields(...): -want i, +got i:\n%s", tc.reason, diff)
			}

		})
	}
}

func TestSetMissingPatchFields(t *testing.T) {
	fieldPath := "spec.id"
	stringFmt := "test-%s"
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
			}},
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
			}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := SetMissingPatchFields(tc.args.patch)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("%s\nNewSetMissingPatchFields(...): -want i, +got i:\n%s", tc.reason, diff)
			}
		})
	}
}

func Test_emptyString(t *testing.T) {
	empty := ""
	nonEmpty := "xp"
	type args struct {
		s *string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "nil string",
			args: args{},
			want: true,
		},
		{
			name: "empty string",
			args: args{s: &empty},
			want: true,
		},
		{
			name: "nonEmpty string",
			args: args{s: &nonEmpty},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := emptyString(tt.args.s); got != tt.want {
				t.Errorf("emptyString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetMissingResourceFields(t *testing.T) {
	name := "testresource-0"
	empty := ""
	str := "crossplane"
	fcsk := v1.ConnectionDetailTypeFromConnectionSecretKey
	var baseNoName = map[string]any{
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
			got := SetMissingResourceFields(tc.args.idx, tc.args.rs)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("%s\nSetMissingResourceFields(...): -want i, +got i:\n%s", tc.reason, diff)
			}
		})
	}
}
