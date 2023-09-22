// Copyright 2021 Upbound Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package xpkg

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestTypes(t *testing.T) {

	type args struct {
		pkgType string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   bool
	}{
		"NotAPackageType": {
			reason: "We should return false when given an invalid package.",
			args: args{
				pkgType: "fake",
			},
			want: false,
		},
		"ConfigurationIsPackage": {
			reason: "We should return true when given a configuration package.",
			args: args{
				pkgType: "configuration",
			},
			want: true,
		},
		"ProviderIsPackage": {
			reason: "We should return true when given a provider package.",
			args: args{
				pkgType: "provider",
			},
			want: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			p := Package(tc.args.pkgType)
			valid := p.IsValid()

			if diff := cmp.Diff(tc.want, valid); diff != "" {
				t.Errorf("\n%s\nIsValid(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
