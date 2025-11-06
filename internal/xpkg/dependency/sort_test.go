/*
Copyright 2025 The Crossplane Authors.

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

package dependency

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/crossplane/crossplane/v2/apis/pkg/v1beta1"
)

func TestSortLockPackages(t *testing.T) {
	type args struct {
		packages []v1beta1.LockPackage
	}
	type want struct {
		sorted []v1beta1.LockPackage
		err    error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"EmptyList": {
			reason: "Sorting an empty list should return an empty list",
			args: args{
				packages: []v1beta1.LockPackage{},
			},
			want: want{
				sorted: []v1beta1.LockPackage{},
			},
		},
		"SinglePackage": {
			reason: "A single package with no dependencies should be returned as-is",
			args: args{
				packages: []v1beta1.LockPackage{
					{Source: "pkg-a", Dependencies: []v1beta1.Dependency{}},
				},
			},
			want: want{
				sorted: []v1beta1.LockPackage{
					{Source: "pkg-a", Dependencies: []v1beta1.Dependency{}},
				},
			},
		},
		"SimpleDependency": {
			reason: "Package B depends on A, so A should come first",
			args: args{
				packages: []v1beta1.LockPackage{
					{
						Source: "pkg-b",
						Dependencies: []v1beta1.Dependency{
							{Package: "pkg-a"},
						},
					},
					{Source: "pkg-a", Dependencies: []v1beta1.Dependency{}},
				},
			},
			want: want{
				sorted: []v1beta1.LockPackage{
					{Source: "pkg-a", Dependencies: []v1beta1.Dependency{}},
					{
						Source: "pkg-b",
						Dependencies: []v1beta1.Dependency{
							{Package: "pkg-a"},
						},
					},
				},
			},
		},
		"ChainDependency": {
			reason: "C depends on B depends on A, so order should be A, B, C",
			args: args{
				packages: []v1beta1.LockPackage{
					{
						Source: "pkg-c",
						Dependencies: []v1beta1.Dependency{
							{Package: "pkg-b"},
						},
					},
					{
						Source: "pkg-b",
						Dependencies: []v1beta1.Dependency{
							{Package: "pkg-a"},
						},
					},
					{Source: "pkg-a", Dependencies: []v1beta1.Dependency{}},
				},
			},
			want: want{
				sorted: []v1beta1.LockPackage{
					{Source: "pkg-a", Dependencies: []v1beta1.Dependency{}},
					{
						Source: "pkg-b",
						Dependencies: []v1beta1.Dependency{
							{Package: "pkg-a"},
						},
					},
					{
						Source: "pkg-c",
						Dependencies: []v1beta1.Dependency{
							{Package: "pkg-b"},
						},
					},
				},
			},
		},
		"CircularDependency": {
			reason: "Circular dependencies should return an error",
			args: args{
				packages: []v1beta1.LockPackage{
					{
						Source: "pkg-a",
						Dependencies: []v1beta1.Dependency{
							{Package: "pkg-b"},
						},
					},
					{
						Source: "pkg-b",
						Dependencies: []v1beta1.Dependency{
							{Package: "pkg-a"},
						},
					},
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"CircularDependencyThreePackages": {
			reason: "Circular dependencies with three packages should return an error",
			args: args{
				packages: []v1beta1.LockPackage{
					{
						Source: "pkg-a",
						Dependencies: []v1beta1.Dependency{
							{Package: "pkg-b"},
						},
					},
					{
						Source: "pkg-b",
						Dependencies: []v1beta1.Dependency{
							{Package: "pkg-c"},
						},
					},
					{
						Source: "pkg-c",
						Dependencies: []v1beta1.Dependency{
							{Package: "pkg-a"},
						},
					},
				},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			sorted, err := SortLockPackages(tc.args.packages)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nSortLockPackages(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.sorted, sorted); diff != "" {
				t.Errorf("%s\nSortLockPackages(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
