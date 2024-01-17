// SPDX-FileCopyrightText: 2024 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
)

func TestPatchValidate(t *testing.T) {
	type args struct {
		patch *Patch
	}

	type want struct {
		err *field.Error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ValidFromCompositeFieldPath": {
			reason: "FromCompositeFieldPath patch with FromFieldPath set should be valid",
			args: args{
				patch: &Patch{
					Type:          PatchTypeFromCompositeFieldPath,
					FromFieldPath: ptr.To("spec.forProvider.foo"),
				},
			},
		},
		"FromCompositeFieldPathWithInvalidTransforms": {
			reason: "FromCompositeFieldPath with invalid transforms should return error",
			args: args{
				patch: &Patch{
					Type:          PatchTypeFromCompositeFieldPath,
					FromFieldPath: ptr.To("spec.forProvider.foo"),
					Transforms: []Transform{
						{
							Type: TransformTypeMath,
							Math: nil,
						},
					},
				},
			},
			want: want{
				err: &field.Error{
					Type:  field.ErrorTypeRequired,
					Field: "transforms[0].math",
				},
			},
		},
		"InvalidFromCompositeFieldPathMissingFromFieldPath": {
			reason: "Invalid FromCompositeFieldPath missing FromFieldPath should return error",
			args: args{
				patch: &Patch{
					Type:          PatchTypeFromCompositeFieldPath,
					FromFieldPath: nil,
				},
			},
			want: want{
				err: &field.Error{
					Type:  field.ErrorTypeRequired,
					Field: "fromFieldPath",
				},
			},
		},
		"InvalidFromCompositeFieldPathMissingToFieldPath": {
			reason: "Invalid ToCompositeFieldPath missing ToFieldPath should return error",
			args: args{
				patch: &Patch{
					Type:        PatchTypeToCompositeFieldPath,
					ToFieldPath: nil,
				},
			},
			want: want{
				&field.Error{
					Type:  field.ErrorTypeRequired,
					Field: "fromFieldPath",
				},
			},
		},
		"InvalidPatchSetMissingPatchSetName": {
			reason: "Invalid PatchSet missing PatchSetName should return error",
			args: args{
				patch: &Patch{
					Type: PatchTypePatchSet,
				},
			},
			want: want{
				err: &field.Error{
					Type:  field.ErrorTypeRequired,
					Field: "patchSetName",
				},
			},
		},
		"InvalidCombineMissingCombine": {
			reason: "Invalid Combine missing Combine should return error",
			args: args{
				patch: &Patch{
					Type: PatchTypeCombineToComposite,
				},
			},
			want: want{
				err: &field.Error{
					Type:  field.ErrorTypeRequired,
					Field: "combine",
				},
			},
		},
		"InvalidCombineMissingToFieldPath": {
			reason: "Invalid Combine missing ToFieldPath should return error",
			args: args{
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
			},
			want: want{
				err: &field.Error{
					Type:  field.ErrorTypeRequired,
					Field: "toFieldPath",
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.args.patch.Validate()
			if diff := cmp.Diff(tc.want.err, err, cmpopts.IgnoreFields(field.Error{}, "Detail", "BadValue")); diff != "" {
				t.Errorf("%s\nValidate(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
