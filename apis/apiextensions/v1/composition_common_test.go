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
)

func TestReadinessCheckValidate(t *testing.T) {
	type args struct {
		r *ReadinessCheck
	}
	type want struct {
		output *field.Error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ValidTypeNone": {
			reason: "Type none should be valid",
			args: args{
				r: &ReadinessCheck{
					Type: ReadinessCheckTypeNone,
				},
			},
		},
		"ValidTypeMatchString": {
			reason: "Type matchString should be valid",
			args: args{
				r: &ReadinessCheck{
					Type:        ReadinessCheckTypeMatchString,
					MatchString: "foo",
					FieldPath:   "spec.foo",
				},
			},
		},
		"ValidTypeMatchCondition": {
			reason: "Type matchCondition should be valid",
			args: args{
				r: &ReadinessCheck{
					Type: ReadinessCheckTypeMatchCondition,
					MatchCondition: &MatchConditionReadinessCheck{
						Type:   "someType",
						Status: "someStatus",
					},
					FieldPath: "spec.foo",
				},
			},
		},
		"ValidTypeMatchTrue": {
			reason: "Type matchTrue should be valid",
			args: args{
				r: &ReadinessCheck{
					Type:      ReadinessCheckTypeMatchTrue,
					FieldPath: "spec.foo",
				},
			},
		},
		"ValidTypeMatchFalse": {
			reason: "Type matchFalse should be valid",
			args: args{
				r: &ReadinessCheck{
					Type:      ReadinessCheckTypeMatchFalse,
					FieldPath: "spec.foo",
				},
			},
		},
		"InvalidType": {
			reason: "Invalid type",
			args: args{
				r: &ReadinessCheck{
					Type: "foo",
				},
			},
			want: want{
				output: &field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "type",
					BadValue: "foo",
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.args.r.Validate()
			if diff := cmp.Diff(tc.want.output, got, cmpopts.IgnoreFields(field.Error{}, "Detail", "BadValue")); diff != "" {
				t.Errorf("%s\nValidate(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
