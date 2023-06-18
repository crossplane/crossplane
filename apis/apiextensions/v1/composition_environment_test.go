package v1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
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

func TestEnvironmentIsNoop(t *testing.T) {

	withResolvePolicy := func() *v1.ResolvePolicy {
		p := v1.ResolvePolicyAlways
		return &p
	}

	type args struct {
		refs []corev1.ObjectReference
		ec   *EnvironmentConfiguration
	}

	cases := map[string]struct {
		reason string
		args   args
		want   bool
	}{
		"DontResolveWhenHaveRefs": {
			reason: "Should not resolve when composite has refs",
			args: args{
				ec: &EnvironmentConfiguration{
					EnvironmentConfigs: []EnvironmentSource{{}},
				},
				refs: []corev1.ObjectReference{{}},
			},
			want: true,
		},
		"ResolveWhenNoRefs": {
			reason: "Should resolve when no refs are held",
			args: args{
				ec: &EnvironmentConfiguration{
					EnvironmentConfigs: []EnvironmentSource{{}},
				},
				refs: []corev1.ObjectReference{},
			},
			want: false,
		},
		"ResolveWhenPolicyAlways": {
			reason: "Should resolve when resolve policy is Always",
			args: args{
				ec: &EnvironmentConfiguration{
					EnvironmentConfigs: []EnvironmentSource{
						{},
					},
					Policy: &v1.Policy{
						Resolve: withResolvePolicy(),
					},
				},
				refs: []corev1.ObjectReference{
					{},
					{},
				},
			},
			want: false,
		},
		"DontResolveWhenPolicyNotAlways": {
			reason: "Should not resolve when resolve policy is not Always",
			args: args{
				ec: &EnvironmentConfiguration{
					EnvironmentConfigs: []EnvironmentSource{
						{},
					},
				},
				refs: []corev1.ObjectReference{
					{},
					{},
				},
			},
			want: true,
		},
	}

	for name, tc := range cases {

		t.Run(name, func(t *testing.T) {
			got := tc.args.ec.IsNoop(tc.args.refs)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("%s\nIsNoop(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
