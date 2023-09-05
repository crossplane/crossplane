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
	"k8s.io/utils/pointer"
)

// SortFieldErrors sorts the given field.ErrorList by the error message.
func sortFieldErrors() cmp.Option {
	return cmpopts.SortSlices(func(e1, e2 *field.Error) bool {
		return strings.Compare(e1.Error(), e2.Error()) < 0
	})
}

func TestCompositionValidateResourceName(t *testing.T) {
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
		"ValidAllNamed": {
			reason: "All resources are named - valid",
			args: args{
				spec: CompositionSpec{
					Resources: []ComposedTemplate{
						{
							Name: pointer.String("foo"),
						},
						{
							Name: pointer.String("bar"),
						},
					},
				},
			},
		},
		"ValidAllAnonymous": {
			reason: "All resources are anonymous - valid",
			args: args{
				spec: CompositionSpec{
					Resources: []ComposedTemplate{
						{},
						{},
					},
				},
			},
		},
		"InvalidMixedNamesExpectingAnonymous": {
			reason: "starting with anonymous resources and mixing named resources is invalid",
			args: args{
				spec: CompositionSpec{
					Resources: []ComposedTemplate{
						{},
						{Name: pointer.String("bar")},
					},
				},
			},
			want: want{
				output: field.ErrorList{
					{
						Type:     field.ErrorTypeInvalid,
						Field:    "spec.resources[1].name",
						BadValue: "bar",
					},
				},
			},
		},
		"InvalidMixedNamesExpectingNamed": {
			reason: "starting with named resources and mixing anonymous resources is invalid",
			args: args{
				spec: CompositionSpec{
					Resources: []ComposedTemplate{
						{Name: pointer.String("bar")},
						{},
					},
				},
			},
			want: want{
				output: field.ErrorList{
					{
						Type:     field.ErrorTypeRequired,
						Field:    "spec.resources[1].name",
						BadValue: "",
					},
				},
			},
		},
		"ValidNamedWithFunctions": {
			reason: "named resources with functions are valid",
			args: args{
				spec: CompositionSpec{
					Resources: []ComposedTemplate{
						{Name: pointer.String("foo")},
						{Name: pointer.String("bar")},
					},
					Functions: []Function{
						{
							Name: "baz",
						},
					},
				},
			},
		},
		"InvalidAnonymousWithFunctions": {
			reason: "anonymous resources with functions are invalid",
			args: args{
				spec: CompositionSpec{
					Resources: []ComposedTemplate{
						{},
					},
					Functions: []Function{
						{
							Name: "foo",
						},
					},
				},
			},
			want: want{
				output: field.ErrorList{
					{
						Type:     field.ErrorTypeRequired,
						Field:    "spec.resources[0].name",
						BadValue: "",
					},
				},
			},
		},
		"InvalidDuplicateNames": {
			reason: "duplicate resource names are invalid",
			args: args{
				spec: CompositionSpec{
					Resources: []ComposedTemplate{
						{Name: pointer.String("foo")},
						{Name: pointer.String("bar")},
						{Name: pointer.String("foo")},
					},
				},
			},
			want: want{
				output: field.ErrorList{
					{
						Type:     field.ErrorTypeDuplicate,
						Field:    "spec.resources[2].name",
						BadValue: "foo",
					},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := &Composition{
				Spec: tc.args.spec,
			}
			gotErrs := c.validateResourceNames()
			if diff := cmp.Diff(tc.want.output, gotErrs, sortFieldErrors(), cmpopts.IgnoreFields(field.Error{}, "Detail", "BadValue")); diff != "" {
				t.Errorf("%s\nvalidateResourceNames(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestCompositionValidatePatchSets(t *testing.T) {
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
		"ValidNoPatchSets": {
			reason: "no patchSets should be valid",
			args: args{
				comp: &Composition{
					Spec: CompositionSpec{
						PatchSets: nil,
					},
				},
			},
		},
		"ValidEmptyPatchSets": {
			reason: "empty patchSets should be valid",
			args: args{
				comp: &Composition{
					Spec: CompositionSpec{
						PatchSets: []PatchSet{},
					},
				},
			},
		},
		"ValidPatchSets": {
			reason: "patchSets with valid patches should be valid",
			args: args{
				comp: &Composition{
					Spec: CompositionSpec{
						PatchSets: []PatchSet{
							{
								Name: "foo",
								Patches: []Patch{
									{
										FromFieldPath: pointer.String("spec.foo"),
									},
								},
							},
						},
					},
				},
			},
		},
		"InvalidNestedPatchSets": {
			reason: "patchSets with nested patchSets should be invalid",
			args: args{
				comp: &Composition{
					Spec: CompositionSpec{
						PatchSets: []PatchSet{
							{
								Name: "foo",
								Patches: []Patch{
									{
										Type: PatchTypePatchSet,
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
						Type:  field.ErrorTypeInvalid,
						Field: "spec.patchSets[0].patches[0].type",
					},
				},
			},
		},
		"InvalidPatchSetsWithInvalidPatch": {
			reason: "patchSets with invalid patches should be invalid",
			args: args{
				comp: &Composition{
					Spec: CompositionSpec{
						PatchSets: []PatchSet{
							{
								Name: "foo",
								Patches: []Patch{
									{
										Type: PatchTypeFromCompositeFieldPath,
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
						Field: "spec.patchSets[0].patches[0].fromFieldPath",
					},
				},
			},
		},
		"InvalidPatchSetNameReferencedByResource": {
			reason: "should return an error if a non existing patchSet is referenced by a resource",
			args: args{
				comp: &Composition{
					Spec: CompositionSpec{
						PatchSets: []PatchSet{
							{
								Name: "foo",
								Patches: []Patch{
									{
										Type:          PatchTypeFromCompositeFieldPath,
										FromFieldPath: pointer.String("spec.something"),
									},
								},
							},
						},
						Resources: []ComposedTemplate{
							{
								Patches: []Patch{
									{
										Type:         PatchTypePatchSet,
										PatchSetName: pointer.String("wrong"),
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
						Type:  field.ErrorTypeInvalid,
						Field: "spec.resources[0].patches[0].patchSetName",
					},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			gotErrs := tc.args.comp.validatePatchSets()
			if diff := cmp.Diff(tc.want.output, gotErrs, sortFieldErrors(), cmpopts.IgnoreFields(field.Error{}, "Detail", "BadValue")); diff != "" {
				t.Errorf("%s\nvalidatePatchSets(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestCompositionValidateFunctions(t *testing.T) {
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
			reason: "no functions should be valid",
			args: args{
				comp: &Composition{
					Spec: CompositionSpec{},
				},
			},
		},
		"ValidFunctions": {
			reason: "functions with valid configuration should be valid",
			args: args{
				comp: &Composition{
					Spec: CompositionSpec{
						Functions: []Function{
							{
								Name: "foo",
								Type: FunctionTypeContainer,
								Container: &ContainerFunction{
									Image: "foo",
								},
							},
							{
								Name: "bar",
								Type: FunctionTypeContainer,
								Container: &ContainerFunction{
									Image: "bar",
								},
							},
						},
					},
				},
			},
		},
		"InvalidDuplicateFuctionNames": {
			reason: "Invalid functions with duplicate names",
			args: args{
				comp: &Composition{
					Spec: CompositionSpec{
						Functions: []Function{
							{
								Name: "foo",
								Type: FunctionTypeContainer,
								Container: &ContainerFunction{
									Image: "foo",
								},
							},
							{
								Name: "foo",
								Type: FunctionTypeContainer,
								Container: &ContainerFunction{
									Image: "bar",
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
						Field:    "spec.functions[1].name",
						BadValue: "foo",
					},
				},
			},
		},
		"InvalidDuplicateFuctionNamesAndMissingContainer": {
			reason: "functions with duplicate names and missing container should return both validation errors",
			args: args{
				comp: &Composition{
					Spec: CompositionSpec{
						Functions: []Function{
							{
								Name: "foo",
								Type: FunctionTypeContainer,
								Container: &ContainerFunction{
									Image: "foo",
								},
							},
							{
								Name: "foo",
								Type: FunctionTypeContainer,
							},
						},
					},
				},
			},
			want: want{
				output: field.ErrorList{
					{
						Type:     field.ErrorTypeDuplicate,
						Field:    "spec.functions[1].name",
						BadValue: "foo",
					},
					{
						Type:     field.ErrorTypeRequired,
						Field:    "spec.functions[1].container",
						BadValue: "",
					},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			gotErrs := tc.args.comp.validateFunctions()
			if diff := cmp.Diff(tc.want.output, gotErrs, sortFieldErrors(), cmpopts.IgnoreFields(field.Error{}, "Detail", "BadValue")); diff != "" {
				t.Errorf("%s\nvalidateFunctions(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestCompositionValidateResources(t *testing.T) {
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
		"ValidNoResources": {
			reason: "no resources should be valid",
			args: args{
				comp: &Composition{
					Spec: CompositionSpec{},
				},
			},
		},
		"ValidComplexNamedResources": {
			reason: "complex named resources should be valid",
			args: args{
				comp: &Composition{
					Spec: CompositionSpec{
						Resources: []ComposedTemplate{
							{
								Name: pointer.String("foo"),
							},
							{
								Name: pointer.String("bar"),
								Patches: []Patch{
									{
										Type:          PatchTypeFromCompositeFieldPath,
										FromFieldPath: pointer.String("spec.foo"),
									},
								},
								ReadinessChecks: []ReadinessCheck{
									{
										Type: ReadinessCheckTypeNone,
									},
								},
							},
						},
					},
				},
			},
		},
		"InvalidComplexNamedResourcesDueToDuplicateNames": {
			reason: "complex named resources with duplicate names should be invalid",
			args: args{
				comp: &Composition{
					Spec: CompositionSpec{
						Resources: []ComposedTemplate{
							{
								Name: pointer.String("foo"),
							},
							{
								Name: pointer.String("foo"),
								Patches: []Patch{
									{
										Type:          PatchTypeFromCompositeFieldPath,
										FromFieldPath: pointer.String("spec.foo"),
									},
								},
								ReadinessChecks: []ReadinessCheck{
									{
										Type: ReadinessCheckTypeNone,
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
						Field:    "spec.resources[1].name",
						BadValue: "foo",
					},
				},
			},
		},
		"InvalidComplexNamedResourcesDueToNameMixing": {
			reason: "complex resources mixing named and anonymous resources should be invalid",
			args: args{
				comp: &Composition{
					Spec: CompositionSpec{
						Resources: []ComposedTemplate{
							{
								Name: pointer.String("foo"),
							},
							{
								Patches: []Patch{
									{
										Type:          PatchTypeFromCompositeFieldPath,
										FromFieldPath: pointer.String("spec.foo"),
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
						Type:     field.ErrorTypeRequired,
						Field:    "spec.resources[1].name",
						BadValue: "",
					},
				},
			},
		},
		"InvalidComplexResource": {
			reason: "complex resource with invalid patches and readiness checks should be invalid",
			args: args{
				comp: &Composition{
					Spec: CompositionSpec{
						Resources: []ComposedTemplate{
							{},
							{
								Name: pointer.String("foo"),
								Patches: []Patch{
									{
										Type: PatchTypeFromCompositeFieldPath,
									},
								},
								ReadinessChecks: []ReadinessCheck{
									{
										Type:         ReadinessCheckTypeMatchInteger,
										MatchInteger: 0,
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
						Type:     field.ErrorTypeInvalid,
						Field:    "spec.resources[1].name",
						BadValue: "foo",
					},
					{
						Type:     field.ErrorTypeRequired,
						Field:    "spec.resources[1].patches[0].fromFieldPath",
						BadValue: "",
					},
					{
						Type:     field.ErrorTypeRequired,
						Field:    "spec.resources[1].readinessChecks[0].matchInteger",
						BadValue: "",
					},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.args.comp.validateResources()
			if diff := cmp.Diff(tc.want.output, got, sortFieldErrors(), cmpopts.IgnoreFields(field.Error{}, "Detail", "BadValue")); diff != "" {
				t.Errorf("%s\nvalidateResources(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestCompositionValidateEnvironment(t *testing.T) {
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
		"ValidEmptyEnvironment": {
			reason: "Should accept an empty environment",
			args: args{
				comp: &Composition{
					Spec: CompositionSpec{
						Environment: &EnvironmentConfiguration{},
					}}},
		},
		"ValidNilEnvironment": {
			reason: "Should accept a nil environment",
			args: args{
				comp: &Composition{
					Spec: CompositionSpec{
						Environment: nil,
					}}},
		},
		"ValidEnvironment": {
			reason: "Should accept a valid environment",
			args: args{
				comp: &Composition{
					Spec: CompositionSpec{
						Environment: &EnvironmentConfiguration{
							Patches: []EnvironmentPatch{
								{
									Type:          PatchTypeFromCompositeFieldPath,
									FromFieldPath: pointer.String("spec.foo"),
									ToFieldPath:   pointer.String("metadata.annotations[\"foo\"]"),
								},
							},
							EnvironmentConfigs: []EnvironmentSource{
								{
									Type: EnvironmentSourceTypeReference,
									Ref: &EnvironmentSourceReference{
										Name: "foo",
									},
								},
								{
									Type: EnvironmentSourceTypeSelector,
									Selector: &EnvironmentSourceSelector{
										MatchLabels: []EnvironmentSourceSelectorLabelMatcher{
											{
												Type:               EnvironmentSourceSelectorLabelMatcherTypeFromCompositeFieldPath,
												Key:                "foo",
												ValueFromFieldPath: pointer.String("spec.foo"),
											}}}}}}}}},
		},
		"InvalidPatchEnvironment": {
			reason: "Should reject an environment declaring an invalid patch",
			want: want{
				output: field.ErrorList{
					{
						Type:  field.ErrorTypeRequired,
						Field: "spec.environment.patches[0].fromFieldPath",
					},
				},
			},
			args: args{
				comp: &Composition{
					Spec: CompositionSpec{
						Environment: &EnvironmentConfiguration{
							Patches: []EnvironmentPatch{
								{
									Type: PatchTypeFromCompositeFieldPath,
									//FromFieldPath: pointer.String("spec.foo"), // missing
									ToFieldPath: pointer.String("metadata.annotations[\"foo\"]"),
								},
							},
							EnvironmentConfigs: []EnvironmentSource{
								{
									Type: EnvironmentSourceTypeReference,
									Ref: &EnvironmentSourceReference{
										Name: "foo",
									},
								},
								{
									Type: EnvironmentSourceTypeSelector,
									Selector: &EnvironmentSourceSelector{
										MatchLabels: []EnvironmentSourceSelectorLabelMatcher{
											{
												Type:               EnvironmentSourceSelectorLabelMatcherTypeFromCompositeFieldPath,
												Key:                "foo",
												ValueFromFieldPath: pointer.String("spec.foo"),
											}}}}}}}}},
		},
		"InvalidEnvironmentSourceReferenceNoName": {
			reason: "Should reject a invalid environment, due to a missing name",
			want: want{
				output: field.ErrorList{
					{
						Type:  field.ErrorTypeRequired,
						Field: "spec.environment.environmentConfigs[0].ref.name",
					},
				},
			},
			args: args{
				comp: &Composition{
					Spec: CompositionSpec{
						Environment: &EnvironmentConfiguration{
							EnvironmentConfigs: []EnvironmentSource{
								{
									Type: EnvironmentSourceTypeReference,
									Ref:  &EnvironmentSourceReference{
										//Name: "foo", // missing
									},
								},
								{
									Type: EnvironmentSourceTypeSelector,
									Selector: &EnvironmentSourceSelector{
										MatchLabels: []EnvironmentSourceSelectorLabelMatcher{
											{
												Type:               EnvironmentSourceSelectorLabelMatcherTypeFromCompositeFieldPath,
												Key:                "foo",
												ValueFromFieldPath: pointer.String("spec.foo"),
											}}}}}}}}},
		},
		"InvalidEnvironmentSourceSelectorNoKey": {
			reason: "Should reject a invalid environment due to a missing key in a selector",
			want: want{
				output: field.ErrorList{
					{
						Type:  field.ErrorTypeRequired,
						Field: "spec.environment.environmentConfigs[1].selector.matchLabels[0].key",
					},
				},
			},
			args: args{
				comp: &Composition{
					Spec: CompositionSpec{
						Environment: &EnvironmentConfiguration{
							EnvironmentConfigs: []EnvironmentSource{
								{
									Type: EnvironmentSourceTypeReference,
									Ref: &EnvironmentSourceReference{
										Name: "foo",
									},
								},
								{
									Type: EnvironmentSourceTypeSelector,
									Selector: &EnvironmentSourceSelector{
										MatchLabels: []EnvironmentSourceSelectorLabelMatcher{
											{
												Type: EnvironmentSourceSelectorLabelMatcherTypeFromCompositeFieldPath,
												//Key:                "foo", // missing
												ValueFromFieldPath: pointer.String("spec.foo"),
											}}}}}}}}},
		},
		"InvalidMultipleErrors": {
			reason: "Should reject a invalid environment due to multiple errors, reporting all of them",
			args: args{
				comp: &Composition{
					Spec: CompositionSpec{
						Environment: &EnvironmentConfiguration{
							Patches: []EnvironmentPatch{
								{
									Type: PatchTypeFromCompositeFieldPath,
									//FromFieldPath: pointer.String("spec.foo"), // missing
									ToFieldPath: pointer.String("metadata.annotations[\"foo\"]"),
								},
							},
							EnvironmentConfigs: []EnvironmentSource{
								{
									Type: EnvironmentSourceTypeReference,
									Ref:  &EnvironmentSourceReference{
										//Name: "foo", // missing
									},
								},
								{
									Type: EnvironmentSourceTypeSelector,
									Selector: &EnvironmentSourceSelector{
										MatchLabels: []EnvironmentSourceSelectorLabelMatcher{
											{
												Type: EnvironmentSourceSelectorLabelMatcherTypeFromCompositeFieldPath,
												//Key:                "foo", // missing
												ValueFromFieldPath: pointer.String("spec.foo"),
											}}}}}}}}},
			want: want{
				output: field.ErrorList{
					{
						Type:  field.ErrorTypeRequired,
						Field: "spec.environment.patches[0].fromFieldPath",
					},
					{
						Type:  field.ErrorTypeRequired,
						Field: "spec.environment.environmentConfigs[0].ref.name",
					},
					{
						Type:  field.ErrorTypeRequired,
						Field: "spec.environment.environmentConfigs[1].selector.matchLabels[0].key",
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			gotErrs := tc.args.comp.validateEnvironment()
			if diff := cmp.Diff(tc.want.output, gotErrs, sortFieldErrors(), cmpopts.IgnoreFields(field.Error{}, "Detail", "BadValue")); diff != "" {
				t.Errorf("%s\nvalidateEnvironment(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
