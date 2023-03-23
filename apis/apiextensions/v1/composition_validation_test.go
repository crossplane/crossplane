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
	"testing"

	"github.com/crossplane/crossplane/pkg/validation/errors"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"
)

func TestComposition_validateResourceName(t *testing.T) {
	type fields struct {
		Spec CompositionSpec
	}
	tests := []struct {
		name     string
		fields   fields
		wantErrs field.ErrorList
	}{
		{
			name: "Valid: all named",
			fields: fields{
				Spec: CompositionSpec{
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
		{
			name: "Valid: all anonymous",
			fields: fields{
				Spec: CompositionSpec{
					Resources: []ComposedTemplate{
						{},
						{},
					},
				},
			},
		},
		{
			name: "Invalid: mixed names expecting anonymous",
			fields: fields{
				Spec: CompositionSpec{
					Resources: []ComposedTemplate{
						{},
						{Name: pointer.String("bar")},
					},
				},
			},
			wantErrs: field.ErrorList{
				{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.resources[1].name",
					BadValue: "bar",
				},
			},
		},
		{
			name: "Invalid: mixed names expecting named",
			fields: fields{
				Spec: CompositionSpec{
					Resources: []ComposedTemplate{
						{Name: pointer.String("bar")},
						{},
					},
				},
			},
			wantErrs: field.ErrorList{
				{
					Type:     field.ErrorTypeRequired,
					Field:    "spec.resources[1].name",
					BadValue: "",
				},
			},
		},
		{
			name: "Valid: named with functions",
			fields: fields{
				Spec: CompositionSpec{
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
		{
			name: "Invalid: anonymous with functions",
			fields: fields{
				Spec: CompositionSpec{
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
			wantErrs: field.ErrorList{
				{
					Type:     field.ErrorTypeRequired,
					Field:    "spec.resources[0].name",
					BadValue: "",
				},
			},
		},
		{
			name: "Invalid: duplicate names",
			fields: fields{
				Spec: CompositionSpec{
					Resources: []ComposedTemplate{
						{Name: pointer.String("foo")},
						{Name: pointer.String("bar")},
						{Name: pointer.String("foo")},
					},
				},
			},
			wantErrs: field.ErrorList{
				{
					Type:     field.ErrorTypeDuplicate,
					Field:    "spec.resources[2].name",
					BadValue: "foo",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Composition{
				Spec: tt.fields.Spec,
			}
			gotErrs := c.validateResourceNames()
			if diff := cmp.Diff(tt.wantErrs, gotErrs, cmpopts.IgnoreFields(field.Error{}, "Detail")); diff != "" {
				t.Errorf("\n%s\nvalidateResourceName(...): -want error, +got error: \n%s", tt.name, diff)
			}
		})
	}
}

func TestComposition_validatePatchSets(t *testing.T) {
	tests := []struct {
		name     string
		comp     *Composition
		wantErrs field.ErrorList
	}{
		{
			name: "Valid no patchSets",
			comp: &Composition{
				Spec: CompositionSpec{
					PatchSets: nil,
				},
			},
		},
		{
			name: "Valid patchSets with no patches",
			comp: &Composition{
				Spec: CompositionSpec{
					PatchSets: []PatchSet{},
				},
			},
		},
		{
			name: "Valid patchSets with patches",
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
		{
			name: "Invalid patchSets with nested patchSets",
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
			wantErrs: field.ErrorList{
				{
					Type:  field.ErrorTypeInvalid,
					Field: "spec.patchSets[0].patches[0].type",
				},
			},
		},
		{
			name: "Invalid patchSets with invalid patch",
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
			wantErrs: field.ErrorList{
				{
					Type:  field.ErrorTypeRequired,
					Field: "spec.patchSets[0].patches[0].fromFieldPath",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.comp.validatePatchSets()
			if diff := cmp.Diff(got, tt.wantErrs, errors.SortFieldErrors(), cmpopts.IgnoreFields(field.Error{}, "Detail", "BadValue")); diff != "" {
				t.Errorf("Validate(...) = -want, +got\n%s\n", diff)
			}
		})
	}
}

func TestComposition_validateFunctions(t *testing.T) {
	tests := []struct {
		name     string
		comp     *Composition
		wantErrs field.ErrorList
	}{
		{
			name: "Valid no functions",
			comp: &Composition{
				Spec: CompositionSpec{},
			},
		},
		{
			name: "Valid functions",
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
		{
			name: "Invalid functions with duplicate names",
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
			wantErrs: field.ErrorList{
				{
					Type:     field.ErrorTypeDuplicate,
					Field:    "spec.functions[1].name",
					BadValue: "foo",
				},
			},
		},
		{
			name: "Invalid functions with duplicate names and missing container",
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
			wantErrs: field.ErrorList{
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.comp.validateFunctions()
			if diff := cmp.Diff(got, tt.wantErrs, cmpopts.IgnoreFields(field.Error{}, "Detail")); diff != "" {
				t.Errorf("validateFunctions(...) = -want, +got\n%s\n", diff)
			}
		})
	}
}

func TestComposition_validateResources(t *testing.T) {
	tests := []struct {
		name     string
		comp     *Composition
		wantErrs field.ErrorList
	}{
		{
			name: "Valid no resources",
			comp: &Composition{
				Spec: CompositionSpec{},
			},
		},
		{
			name: "Valid complex named resources",
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
		{
			name: "Invalid complex named resources due to duplicate names",
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
			wantErrs: field.ErrorList{
				{
					Type:     field.ErrorTypeDuplicate,
					Field:    "spec.resources[1].name",
					BadValue: "foo",
				},
			},
		},
		{
			name: "Invalid complex resources due to mixed anonymous resources",
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
			wantErrs: field.ErrorList{
				{
					Type:     field.ErrorTypeRequired,
					Field:    "spec.resources[1].name",
					BadValue: "",
				},
			},
		},
		{
			name: "Invalid complex",
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
			wantErrs: field.ErrorList{
				{
					Type:     field.ErrorTypeRequired,
					Field:    "spec.resources[1].patches[0].fromFieldPath",
					BadValue: "",
				},
				{
					Type:     field.ErrorTypeInvalid,
					Field:    "spec.resources[1].name",
					BadValue: "foo",
				},
				{
					Type:     field.ErrorTypeRequired,
					Field:    "spec.resources[1].readinessChecks[0].matchInteger",
					BadValue: "",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.comp.validateResources()
			if diff := cmp.Diff(got, tt.wantErrs, errors.SortFieldErrors(), cmpopts.IgnoreFields(field.Error{}, "Detail")); diff != "" {
				t.Errorf("validateResources(...) = -want, +got\n%s\n", diff)
			}
		})
	}
}
