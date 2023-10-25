package v1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xperrors "github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestCompositionGetValidationMode(t *testing.T) {
	type args struct {
		comp *Composition
	}
	type want struct {
		mode CompositionValidationMode
		err  error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ValidDefault": {
			reason: "Default mode should be returned if not specified",
			args: args{
				comp: &Composition{
					Spec: CompositionSpec{},
				},
			},
			want: want{
				mode: DefaultSchemaAwareCompositionValidationMode,
			},
		},
		"ValidStrict": {
			reason: "Strict mode should be returned if specified",
			args: args{
				comp: &Composition{
					ObjectMeta: v1.ObjectMeta{
						Annotations: map[string]string{
							SchemaAwareCompositionValidationModeAnnotation: string(SchemaAwareCompositionValidationModeStrict),
						},
					},
				},
			},
			want: want{
				mode: SchemaAwareCompositionValidationModeStrict,
			},
		},
		"ValidLoose": {
			reason: "Loose mode should be returned if specified",
			args: args{
				comp: &Composition{
					ObjectMeta: v1.ObjectMeta{
						Annotations: map[string]string{
							SchemaAwareCompositionValidationModeAnnotation: string(SchemaAwareCompositionValidationModeLoose),
						},
					},
				},
			},
			want: want{
				mode: SchemaAwareCompositionValidationModeLoose,
			},
		},
		"ValidWarn": {
			reason: "Warn mode should be returned if specified",
			args: args{
				comp: &Composition{
					ObjectMeta: v1.ObjectMeta{
						Annotations: map[string]string{
							SchemaAwareCompositionValidationModeAnnotation: string(SchemaAwareCompositionValidationModeWarn),
						},
					},
				},
			},
			want: want{
				mode: SchemaAwareCompositionValidationModeWarn,
			},
		},
		"InvalidValue": {
			reason: "An error should be returned if an invalid value is specified",
			args: args{
				comp: &Composition{
					ObjectMeta: v1.ObjectMeta{
						Annotations: map[string]string{
							SchemaAwareCompositionValidationModeAnnotation: "invalid",
						},
					},
				},
			},
			want: want{
				err: xperrors.Errorf(errFmtInvalidCompositionValidationMode, "invalid"),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := tc.args.comp.GetSchemaAwareValidationMode()
			if diff := cmp.Diff(tc.want.mode, got); diff != "" {
				t.Errorf("\n%s\nGetValidationMode(...) -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nGetValidationMode(...) -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
