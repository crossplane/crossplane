/*
Copyright 2022 The Crossplane Authors.

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

package webhook

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

// Validator has to satisfy CustomValidator interface so that it can be used by
// controller-runtime Manager.
var _ webhook.CustomValidator = &Validator{}

var errBoom = errors.New("boom")
var warnings = []string{"warning"}

func TestValidateCreate(t *testing.T) {
	type args struct {
		obj runtime.Object
		fns []ValidateCreateFn
	}
	type want struct {
		err      error
		warnings admission.Warnings
	}
	cases := map[string]struct {
		reason string
		args
		want
	}{
		"Success": {
			reason: "Functions without errors and warnings should be executed successfully",
			args: args{
				fns: []ValidateCreateFn{
					func(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
						return nil, nil
					},
				},
			},
		},
		"SuccessWithWarnings": {
			reason: "Functions with warnings but without errors should be executed successfully",
			args: args{
				fns: []ValidateCreateFn{
					func(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
						return warnings, nil
					},
				},
			},
			want: want{
				warnings: warnings,
			},
		},
		"Failure": {
			reason: "Functions with errors and without warnings should return with error",
			args: args{
				fns: []ValidateCreateFn{
					func(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
						return nil, errBoom
					},
				},
			},
			want: want{
				err: errBoom,
			},
		},
		"FailureWithWarnings": {
			reason: "Functions with errors and warnings should return with error",
			args: args{
				fns: []ValidateCreateFn{
					func(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
						return warnings, errBoom
					},
				},
			},
			want: want{
				warnings: warnings,
				err:      errBoom,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			v := NewValidator(WithValidateCreationFns(tc.fns...))
			warn, err := v.ValidateCreate(context.TODO(), tc.args.obj)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nValidateCreate(...): -want error, +got error\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.warnings, warn, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nValidateCreate(...): -want warnings, +got warnings\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestValidateUpdate(t *testing.T) {
	type args struct {
		oldObj runtime.Object
		newObj runtime.Object
		fns    []ValidateUpdateFn
	}
	type want struct {
		err      error
		warnings admission.Warnings
	}
	cases := map[string]struct {
		reason string
		args
		want
	}{
		"Success": {
			reason: "Functions without errors should be executed successfully",
			args: args{
				fns: []ValidateUpdateFn{
					func(_ context.Context, _, _ runtime.Object) (admission.Warnings, error) {
						return nil, nil
					},
				},
			},
		},
		"SuccessWithWarnings": {
			reason: "Functions without errors but with warnings should be executed successfully with warnings",
			args: args{
				fns: []ValidateUpdateFn{
					func(_ context.Context, _, _ runtime.Object) (admission.Warnings, error) {
						return warnings, nil
					},
				},
			},
			want: want{
				warnings: warnings,
			},
		},
		"Failure": {
			reason: "Functions with errors should return with error",
			args: args{
				fns: []ValidateUpdateFn{
					func(_ context.Context, _, _ runtime.Object) (admission.Warnings, error) {
						return nil, errBoom
					},
				},
			},
			want: want{
				err: errBoom,
			},
		},
		"FailureWithWarnings": {
			reason: "Functions with errors and warnings should return with error and warning",
			args: args{
				fns: []ValidateUpdateFn{
					func(_ context.Context, _, _ runtime.Object) (admission.Warnings, error) {
						return warnings, errBoom
					},
				},
			},
			want: want{
				err:      errBoom,
				warnings: warnings,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			v := NewValidator(WithValidateUpdateFns(tc.fns...))
			warn, err := v.ValidateUpdate(context.TODO(), tc.args.oldObj, tc.args.newObj)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nValidateUpdate(...): -want error, +got error\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.warnings, warn, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nValidateUpdate(...): -want warnings, +got warnings\n%s\n", tc.reason, diff)
			}
		})
	}
}

func TestValidateDelete(t *testing.T) {
	type args struct {
		obj runtime.Object
		fns []ValidateDeleteFn
	}
	type want struct {
		err      error
		warnings admission.Warnings
	}
	cases := map[string]struct {
		reason string
		args
		want
	}{
		"Success": {
			reason: "Functions without errors should be executed successfully",
			args: args{
				fns: []ValidateDeleteFn{
					func(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
						return nil, nil
					},
				},
			},
		},
		"SuccessWithWarnings": {
			reason: "Functions without errors but with warnings should be executed successfully",
			args: args{
				fns: []ValidateDeleteFn{
					func(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
						return warnings, nil
					},
				},
			},
			want: want{
				warnings: warnings,
			},
		},
		"Failure": {
			reason: "Functions with errors should return with error",
			args: args{
				fns: []ValidateDeleteFn{
					func(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
						return nil, errBoom
					},
				},
			},
			want: want{
				err: errBoom,
			},
		},
		"FailureWithWarnings": {
			reason: "Functions with errors and warnings should return with error and warnings",
			args: args{
				fns: []ValidateDeleteFn{
					func(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
						return warnings, errBoom
					},
				},
			},
			want: want{
				err:      errBoom,
				warnings: warnings,
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			v := NewValidator(WithValidateDeletionFns(tc.fns...))
			warn, err := v.ValidateDelete(context.TODO(), tc.args.obj)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nValidateDelete(...): -want error, +got error\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.warnings, warn, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("\n%s\nValidateDelete(...): -want warnings, +got warnings\n%s\n", tc.reason, diff)
			}
		})
	}
}
