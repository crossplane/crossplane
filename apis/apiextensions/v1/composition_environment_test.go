package v1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"
)

func TestEnvironmentPatchValidate(t *testing.T) {
	type args struct {
		envPatch *EnvironmentPatch
	}
	type want struct {
		output *field.Error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ValidPatch": {
			reason: "Should accept a valid patch",
			args: args{
				envPatch: &EnvironmentPatch{
					Type:          PatchTypeFromCompositeFieldPath,
					FromFieldPath: pointer.String("spec.foo"),
					ToFieldPath:   pointer.String("metadata.annotations[\"foo\"]"),
				},
			},
			want: want{output: nil},
		},
		"InvalidPatchMissingFromFieldPath": {
			reason: "Should reject a patch missing fromFieldPath",
			args: args{
				envPatch: &EnvironmentPatch{
					Type:        PatchTypeFromCompositeFieldPath,
					ToFieldPath: pointer.String("metadata.annotations[\"foo\"]"),
				},
			},
			want: want{
				output: &field.Error{
					Type:  field.ErrorTypeRequired,
					Field: "fromFieldPath",
				},
			},
		},
		"InvalidPatchMissingToFieldPath": {
			reason: "Should reject a patch missing toFieldPath",
			args: args{
				envPatch: &EnvironmentPatch{
					Type:          PatchTypeCombineToComposite,
					FromFieldPath: pointer.String("spec.foo"),
					Combine:       nil, // required
				},
			},
			want: want{
				output: &field.Error{
					Type:  field.ErrorTypeRequired,
					Field: "combine",
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.args.envPatch.Validate()
			if diff := cmp.Diff(tc.want.output, got, cmpopts.IgnoreFields(field.Error{}, "Detail", "BadValue")); diff != "" {
				t.Errorf("%s\nValidate(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
