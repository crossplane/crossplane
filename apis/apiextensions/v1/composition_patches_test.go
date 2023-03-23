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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"
)

func TestPatch_Validate(t *testing.T) {
	tests := []struct {
		name  string
		patch *Patch
		want  *field.Error
	}{
		{
			name: "Valid FromCompositeFieldPath no transforms",
			patch: &Patch{
				Type:          PatchTypeFromCompositeFieldPath,
				FromFieldPath: pointer.String("spec.forProvider.foo"),
			},
			want: nil,
		},
		{
			name: "Invalid FromCompositeFieldPath with invalid transforms",
			patch: &Patch{
				Type:          PatchTypeFromCompositeFieldPath,
				FromFieldPath: pointer.String("spec.forProvider.foo"),
				Transforms: []Transform{
					{
						Type: TransformTypeMath,
						Math: nil,
					},
				},
			},
			want: &field.Error{
				Type:  field.ErrorTypeRequired,
				Field: "transforms[0].math",
			},
		},
		{
			name: "Invalid FromCompositeFieldPath missing FromFieldPath",
			patch: &Patch{
				Type:          PatchTypeFromCompositeFieldPath,
				FromFieldPath: nil,
			},
			want: &field.Error{
				Type:  field.ErrorTypeRequired,
				Field: "fromFieldPath",
			},
		},
		{
			name: "Invalid ToCompositeFieldPath missing ToFieldPath",
			patch: &Patch{
				Type:        PatchTypeToCompositeFieldPath,
				ToFieldPath: nil,
			},
			want: &field.Error{
				Type:  field.ErrorTypeRequired,
				Field: "fromFieldPath",
			},
		},
		{
			name: "Invalid PatchSet missing PatchSetName",
			patch: &Patch{
				Type: PatchTypePatchSet,
			},
			want: &field.Error{
				Type:  field.ErrorTypeRequired,
				Field: "patchSetName",
			},
		},
		{
			name: "Invalid Combine missing Combine",
			patch: &Patch{
				Type: PatchTypeCombineToComposite,
			},
			want: &field.Error{
				Type:  field.ErrorTypeRequired,
				Field: "combine",
			},
		},
		{
			name: "Invalid Combine missing ToFieldPath",
			patch: &Patch{
				Type: PatchTypeCombineToComposite,
				Combine: &Combine{
					Variables: []CombineVariable{
						{
							FromFieldPath: "spec.forProvider.foo",
						},
					},
				},
				ToFieldPath: nil,
			},
			want: &field.Error{
				Type:  field.ErrorTypeRequired,
				Field: "toFieldPath",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.patch.Validate()
			if diff := cmp.Diff(got, tt.want, cmpopts.IgnoreFields(field.Error{}, "Detail", "BadValue")); diff != "" {
				t.Errorf("Validate(...): -want, +got:\n%s", diff)
			}
		})
	}
}
