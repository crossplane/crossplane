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

package manager

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestPackNHash(t *testing.T) {
	type args struct {
		pkg  string
		hash string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   string
	}{
		"BothUnderLimit": {
			reason: "If both package and hash are under limit neither should be truncated.",
			args: args{
				pkg:  "provider-aws",
				hash: "1234567",
			},
			want: "provider-aws-1234567",
		},
		"PackageOverLimit": {
			reason: "If package is over limit it should be truncated.",
			args: args{
				pkg:  "provider-aws-plusabunchofothernonsensethatisgoingtogetslicedoff",
				hash: "1234567",
			},
			want: "provider-aws-plusabunchofothernonsensethatisgoingt-1234567",
		},
		"HashOverLimit": {
			reason: "If hash is over limit it should be truncated.",
			args: args{
				pkg:  "provider-aws",
				hash: "1234567891234567",
			},
			want: "provider-aws-123456789123",
		},
		"BothOverLimit": {
			reason: "If both package and hash are over limit both should be truncated.",
			args: args{
				pkg:  "provider-aws-plusabunchofothernonsensethatisgoingtogetslicedoff",
				hash: "1234567891234567",
			},
			want: "provider-aws-plusabunchofothernonsensethatisgoingt-123456789123",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			want := packNHash(tc.args.pkg, tc.args.hash)

			if diff := cmp.Diff(tc.want, want); diff != "" {
				t.Errorf("\n%s\nPackNHash(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
