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

package dep

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/internal/xpkg/v2/dep/resolver/image"
)

func TestNew(t *testing.T) {
	providerAws := "crossplane/provider-aws"

	type args struct {
		pkg string
		t   string
	}

	type want struct {
		dep v1beta1.Dependency
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"EmptyVersion": {
			args: args{
				pkg: providerAws,
			},
			want: want{
				dep: v1beta1.Dependency{
					Package:     providerAws,
					Type:        v1beta1.ProviderPackageType,
					Constraints: image.DefaultVer,
				},
			},
		},
		"VersionSupplied": {
			args: args{
				pkg: fmt.Sprintf("%s@%s", providerAws, "v1.0.0"),
			},
			want: want{
				dep: v1beta1.Dependency{
					Package:     providerAws,
					Type:        v1beta1.ProviderPackageType,
					Constraints: "v1.0.0",
				},
			},
		},
		"VersionConstraintSupplied": {
			args: args{
				pkg: fmt.Sprintf("%s@%s", providerAws, ">=v1.0.0"),
				t:   "configuration",
			},
			want: want{
				dep: v1beta1.Dependency{
					Package:     providerAws,
					Type:        v1beta1.ConfigurationPackageType,
					Constraints: ">=v1.0.0",
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			d := NewWithType(tc.args.pkg, tc.args.t)

			if diff := cmp.Diff(tc.want.dep, d); diff != "" {
				t.Errorf("\n%s\nNew(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}
