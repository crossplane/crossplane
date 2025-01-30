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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// SortFieldErrors sorts the given field.ErrorList by the error message.
func sortFieldErrors() cmp.Option {
	return cmpopts.SortSlices(func(e1, e2 *field.Error) bool {
		return strings.Compare(e1.Error(), e2.Error()) < 0
	})
}

func TestCompositionValidateMode(t *testing.T) {
	type args struct {
		spec CompositionSpec
	}
	type want struct {
		output field.ErrorList
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ValidPipeline": {
			reason: "A Pipeline mode Composition with an array of pipeline steps is valid",
			args: args{
				spec: CompositionSpec{
					Mode: CompositionModePipeline,
					Pipeline: []PipelineStep{
						{
							Step: "razor",
						},
					},
				},
			},
			want: want{
				output: nil,
			},
		},
		"InvalidPipeline": {
			reason: "A Pipeline mode Composition without an array of pipeline steps is invalid",
			args: args{
				spec: CompositionSpec{
					Mode: CompositionModePipeline,
				},
			},
			want: want{
				output: field.ErrorList{field.Required(field.NewPath("spec", "pipeline"), "this test ignores this field")},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &Composition{
				Spec: tc.args.spec,
			}
			gotErrs := c.validateMode()
			if diff := cmp.Diff(tc.want.output, gotErrs, sortFieldErrors(), cmpopts.IgnoreFields(field.Error{}, "Detail", "BadValue")); diff != "" {
				t.Errorf("%s\nvalidateResourceNames(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestCompositionValidatePipeline(t *testing.T) {
	type args struct {
		comp *Composition
	}
	type want struct {
		output field.ErrorList
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ValidNoFunctions": {
			reason: "no steps should be valid",
			args: args{
				comp: &Composition{
					Spec: CompositionSpec{},
				},
			},
		},
		"ValidFunctions": {
			reason: "steps with valid configuration should be valid",
			args: args{
				comp: &Composition{
					Spec: CompositionSpec{
						Pipeline: []PipelineStep{
							{
								Step: "foo",
							},
							{
								Step: "bar",
							},
						},
					},
				},
			},
		},
		"InvalidDuplicateStepNames": {
			reason: "Invalid steps with duplicate names",
			args: args{
				comp: &Composition{
					Spec: CompositionSpec{
						Pipeline: []PipelineStep{
							{
								Step: "foo",
							},
							{
								Step: "foo",
							},
						},
					},
				},
			},
			want: want{
				output: field.ErrorList{
					{
						Type:     field.ErrorTypeDuplicate,
						Field:    "spec.pipeline[1].step",
						BadValue: "foo",
					},
				},
			},
		},
		"InvalidDuplicateCredentialNames": {
			reason: "A step's credential names must be unique",
			args: args{
				comp: &Composition{
					Spec: CompositionSpec{
						Mode: CompositionModePipeline,
						Pipeline: []PipelineStep{
							{
								Step: "duplicate-creds",
								Credentials: []FunctionCredentials{
									{
										Name: "foo",
									},
									{
										Name: "foo",
									},
								},
							},
						},
					},
				},
			},
			want: want{
				output: field.ErrorList{
					{
						Type:     field.ErrorTypeDuplicate,
						Field:    "spec.pipeline[0].credentials[1].name",
						BadValue: "foo",
					},
				},
			},
		},
		"InvalidMissingSecretRef": {
			reason: "A step's credential must specify a secretRef if its source is a secret",
			args: args{
				comp: &Composition{
					Spec: CompositionSpec{
						Mode: CompositionModePipeline,
						Pipeline: []PipelineStep{
							{
								Step: "duplicate-creds",
								Credentials: []FunctionCredentials{
									{
										Name:   "foo",
										Source: FunctionCredentialsSourceSecret,
									},
								},
							},
						},
					},
				},
			},
			want: want{
				output: field.ErrorList{
					{
						Type:  field.ErrorTypeRequired,
						Field: "spec.pipeline[0].credentials[0].secretRef",
					},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			gotErrs := tc.args.comp.validatePipeline()
			if diff := cmp.Diff(tc.want.output, gotErrs, sortFieldErrors(), cmpopts.IgnoreFields(field.Error{}, "Detail", "BadValue")); diff != "" {
				t.Errorf("%s\nvalidatePipeline(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
