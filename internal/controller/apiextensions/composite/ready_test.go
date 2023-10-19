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

package composite

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

var _ ReadinessChecker = ReadinessCheckerFn(IsReady)

func TestIsReady(t *testing.T) {
	type args struct {
		ctx context.Context
		o   ConditionedObject
		rc  []ReadinessCheck
	}
	type want struct {
		ready bool
		err   error
	}
	cases := map[string]struct {
		reason string
		args
		want
	}{
		"NoCustomCheckReady": {
			reason: "If no custom check is given, Ready condition should be used",
			args: args{
				o: composed.New(composed.WithConditions(xpv1.Available())),
			},
			want: want{
				ready: true,
			},
		},
		"NoCustomCheckNotReady": {
			reason: "If no custom check is given, Ready condition should be used",
			args: args{
				o: composed.New(composed.WithConditions(xpv1.Unavailable())),
			},
			want: want{
				ready: false,
			},
		},
		"MatchConditionReady": {
			reason: "If a match condition is explicitly specified it should be used",
			args: args{
				o: composed.New(composed.WithConditions(xpv1.Available())),
				rc: []ReadinessCheck{{
					Type: ReadinessCheckTypeMatchCondition,
					MatchCondition: &MatchConditionReadinessCheck{
						Type:   xpv1.TypeReady,
						Status: corev1.ConditionTrue,
					},
				}},
			},
			want: want{
				ready: true,
			},
		},
		"MatchConditionNotReady": {
			reason: "If a match condition is explicitly specified it should be used",
			args: args{
				o: composed.New(composed.WithConditions(xpv1.Unavailable())),
				rc: []ReadinessCheck{{
					Type: ReadinessCheckTypeMatchCondition,
					MatchCondition: &MatchConditionReadinessCheck{
						Type:   xpv1.TypeReady,
						Status: corev1.ConditionTrue,
					},
				}},
			},
			want: want{
				ready: false,
			},
		},
		"ExplictNone": {
			reason: "If the only readiness check is explicitly 'None' the resource is always ready.",
			args: args{
				o:  composed.New(),
				rc: []ReadinessCheck{{Type: ReadinessCheckTypeNone}},
			},
			want: want{
				ready: true,
			},
		},
		"NonEmptyMissingFieldPath": {
			reason: "If the value cannot be fetched due to fieldPath being missing, an error should be returned",
			args: args{
				o: composed.New(),
				rc: []ReadinessCheck{{
					Type: ReadinessCheckTypeNonEmpty,
				}},
			},
			want: want{
				err: errors.Wrapf(errors.Wrap(errors.Errorf(errFmtRequiresFieldPath, ReadinessCheckTypeNonEmpty), errInvalidCheck), errFmtRunCheck, 0),
			},
		},
		"NonEmptyErr": {
			reason: "If the value cannot be fetched due to fieldPath being misconfigured, error should be returned",
			args: args{
				o: composed.New(),
				rc: []ReadinessCheck{{
					Type:      ReadinessCheckTypeNonEmpty,
					FieldPath: ptr.To("metadata..uid"),
				}},
			},
			want: want{
				err: errors.Wrapf(fieldpath.Pave(nil).GetValueInto("metadata..uid", nil), errFmtRunCheck, 0),
			},
		},
		"NonEmptyFalse": {
			reason: "If the field does not have value, NonEmpty check should return false",
			args: args{
				o: composed.New(),
				rc: []ReadinessCheck{{
					Type:      ReadinessCheckTypeNonEmpty,
					FieldPath: ptr.To("metadata.uid"),
				}},
			},
			want: want{
				ready: false,
			},
		},
		"NonEmptyTrue": {
			reason: "If the field does have a value, NonEmpty check should return true",
			args: args{
				o: composed.New(func(r *composed.Unstructured) {
					r.SetUID("olala")
				}),
				rc: []ReadinessCheck{{
					Type:      ReadinessCheckTypeNonEmpty,
					FieldPath: ptr.To("metadata.uid"),
				}},
			},
			want: want{
				ready: true,
			},
		},
		"MatchStringErr": {
			reason: "If the value cannot be fetched due to fieldPath being misconfigured, error should be returned",
			args: args{
				o: composed.New(),
				rc: []ReadinessCheck{{
					Type:        ReadinessCheckTypeMatchString,
					FieldPath:   ptr.To("metadata..uid"),
					MatchString: ptr.To("cool"),
				}},
			},
			want: want{
				err: errors.Wrapf(fieldpath.Pave(nil).GetValueInto("metadata..uid", nil), errFmtRunCheck, 0),
			},
		},
		"MatchStringMissing": {
			reason: "If the value cannot be fetched due to a missing matchstring, we should return an error",
			args: args{
				o: composed.New(),
				rc: []ReadinessCheck{{
					Type:      ReadinessCheckTypeMatchString,
					FieldPath: ptr.To("metadata..uid"),
				}},
			},
			want: want{
				err: errors.Wrapf(errors.Wrap(errors.Errorf(errFmtRequiresMatchString, ReadinessCheckTypeMatchString), errInvalidCheck), errFmtRunCheck, 0),
			},
		},
		"MatchStringFalse": {
			reason: "If the value of the field does not match, it should return false",
			args: args{
				o: composed.New(),
				rc: []ReadinessCheck{{
					Type:        ReadinessCheckTypeMatchString,
					FieldPath:   ptr.To("metadata.uid"),
					MatchString: ptr.To("olala"),
				}},
			},
			want: want{
				ready: false,
			},
		},
		"MatchStringTrue": {
			reason: "If the value of the field does match, it should return true",
			args: args{
				o: composed.New(func(r *composed.Unstructured) {
					r.SetUID("olala")
				}),
				rc: []ReadinessCheck{{
					Type:        ReadinessCheckTypeMatchString,
					FieldPath:   ptr.To("metadata.uid"),
					MatchString: ptr.To("olala"),
				}},
			},
			want: want{
				ready: true,
			},
		},
		"MatchIntegerErr": {
			reason: "If the value cannot be fetched due to fieldPath being misconfigured, error should be returned",
			args: args{
				o: composed.New(),
				rc: []ReadinessCheck{{
					Type:         ReadinessCheckTypeMatchInteger,
					FieldPath:    ptr.To("metadata..uid"),
					MatchInteger: ptr.To[int64](42),
				}},
			},
			want: want{
				err: errors.Wrapf(fieldpath.Pave(nil).GetValueInto("metadata..uid", nil), errFmtRunCheck, 0),
			},
		},
		"MatchIntegerMissing": {
			reason: "If the value cannot be fetched due to a missing matchinteger, we should return an error",
			args: args{
				o: composed.New(),
				rc: []ReadinessCheck{{
					Type:      ReadinessCheckTypeMatchInteger,
					FieldPath: ptr.To("metadata..uid"),
				}},
			},
			want: want{
				err: errors.Wrapf(errors.Wrap(errors.Errorf(errFmtRequiresMatchInteger, ReadinessCheckTypeMatchInteger), errInvalidCheck), errFmtRunCheck, 0),
			},
		},
		"MatchIntegerFalse": {
			reason: "If the value of the field does not match, it should return false",
			args: args{
				o: composed.New(func(r *composed.Unstructured) {
					r.Object = map[string]any{
						"spec": map[string]any{
							"someNum": int64(6),
						},
					}
				}),
				rc: []ReadinessCheck{{
					Type:         ReadinessCheckTypeMatchInteger,
					FieldPath:    ptr.To("spec.someNum"),
					MatchInteger: ptr.To[int64](5),
				}},
			},
			want: want{
				ready: false,
			},
		},
		"MatchIntegerTrue": {
			reason: "If the value of the field does match, it should return true",
			args: args{
				o: composed.New(func(r *composed.Unstructured) {
					r.Object = map[string]any{
						"spec": map[string]any{
							"someNum": int64(5),
						},
					}
				}),
				rc: []ReadinessCheck{{
					Type:         ReadinessCheckTypeMatchInteger,
					FieldPath:    ptr.To("spec.someNum"),
					MatchInteger: ptr.To[int64](5),
				}},
			},
			want: want{
				ready: true,
			},
		},
		"MatchTrueMissing": {
			reason: "If the field is missing, it should return false",
			args: args{
				o: composed.New(func(r *composed.Unstructured) {
					r.Object = map[string]any{
						"spec": map[string]any{},
					}
				}),
				rc: []ReadinessCheck{{
					Type:      ReadinessCheckTypeMatchTrue,
					FieldPath: ptr.To("spec.someBool"),
				}},
			},
			want: want{
				ready: false,
			},
		},
		"MatchTrueReady": {
			reason: "If the value of the field is true, it should return true",
			args: args{
				o: composed.New(func(r *composed.Unstructured) {
					r.Object = map[string]any{
						"spec": map[string]any{
							"someBool": true,
						},
					}
				}),
				rc: []ReadinessCheck{{
					Type:      ReadinessCheckTypeMatchTrue,
					FieldPath: ptr.To("spec.someBool"),
				}},
			},
			want: want{
				ready: true,
			},
		},
		"MatchTrueNotReady": {
			reason: "If the value of the field is false, it should return false",
			args: args{
				o: composed.New(func(r *composed.Unstructured) {
					r.Object = map[string]any{
						"spec": map[string]any{
							"someBool": false,
						},
					}
				}),
				rc: []ReadinessCheck{{
					Type:      ReadinessCheckTypeMatchTrue,
					FieldPath: ptr.To("spec.someBool"),
				}},
			},
			want: want{
				ready: false,
			},
		},
		"MatchFalseMissing": {
			reason: "If the field is missing, it should return false",
			args: args{
				o: composed.New(func(r *composed.Unstructured) {
					r.Object = map[string]any{
						"spec": map[string]any{},
					}
				}),
				rc: []ReadinessCheck{{
					Type:      ReadinessCheckTypeMatchFalse,
					FieldPath: ptr.To("spec.someBool"),
				}},
			},
			want: want{
				ready: false,
			},
		},
		"MatchFalseReady": {
			reason: "If the value of the field is false, it should return true",
			args: args{
				o: composed.New(func(r *composed.Unstructured) {
					r.Object = map[string]any{
						"spec": map[string]any{
							"someBool": false,
						},
					}
				}),
				rc: []ReadinessCheck{{
					Type:      ReadinessCheckTypeMatchFalse,
					FieldPath: ptr.To("spec.someBool"),
				}},
			},
			want: want{
				ready: true,
			},
		},
		"MatchFalseNotReady": {
			reason: "If the value of the field is true, it should return false",
			args: args{
				o: composed.New(func(r *composed.Unstructured) {
					r.Object = map[string]any{
						"spec": map[string]any{
							"someBool": true,
						},
					}
				}),
				rc: []ReadinessCheck{{
					Type:      ReadinessCheckTypeMatchFalse,
					FieldPath: ptr.To("spec.someBool"),
				}},
			},
			want: want{
				ready: false,
			},
		},
		"UnknownType": {
			reason: "If unknown type is chosen, it should return an error",
			args: args{
				o:  composed.New(),
				rc: []ReadinessCheck{{Type: "Olala"}},
			},
			want: want{
				err: errors.Wrapf(errors.Wrap(errors.Errorf(errFmtUnknownCheck, "Olala"), errInvalidCheck), errFmtRunCheck, 0),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ready, err := IsReady(tc.args.ctx, tc.args.o, tc.args.rc...)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nIsReady(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.ready, ready); diff != "" {
				t.Errorf("\n%s\nIsReady(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
