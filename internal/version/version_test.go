/*
Copyright 2020 The Crossplane Authors.

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

package version

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func TestInRange(t *testing.T) {
	type args struct {
		version string
		r       string
	}
	type want struct {
		is  bool
		err error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ValidInRange": {
			reason: "Should return true when a valid semantic version is in a valid range.",
			args: args{
				version: "v0.13.0",
				r:       ">0.12.0",
			},
			want: want{
				is: true,
			},
		},
		"ValidNotInRange": {
			reason: "Should return false when a valid semantic version is not in a valid range.",
			args: args{
				version: "v0.13.0",
				r:       ">0.13.0",
			},
			want: want{
				is: false,
			},
		},
		"InvalidVersion": {
			reason: "Should return error when version is invalid.",
			args: args{
				version: "v0a.13.0",
			},
			want: want{
				err: errors.New("Invalid Semantic Version"),
			},
		},
		"InvalidRange": {
			reason: "Should return error when range is invalid.",
			args: args{
				version: "v0.13.0",
				r:       ">a2",
			},
			want: want{
				err: errors.New("improper constraint: >a2"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			version = tc.args.version
			is, err := New().InConstraints(tc.args.r)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nInRange(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.is, is, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nInRange(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
