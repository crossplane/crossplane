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
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/utils/ptr"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/parser"

	pkgmetav1 "github.com/crossplane/crossplane/v2/apis/pkg/meta/v1"
	v1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"
	"github.com/crossplane/crossplane/v2/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/v2/internal/xpkg"
)

func TestPackagesAddConstraints(t *testing.T) {
	type args struct {
		packages    Packages
		source      string
		constraints []string
	}
	type want struct {
		packages Packages
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NewPackage": {
			reason: "Adding constraints to a new package should create it",
			args: args{
				packages:    Packages{},
				source:      "pkg-a",
				constraints: []string{">=v1.0.0"},
			},
			want: want{
				packages: Packages{
					"pkg-a": &Package{
						Constraints: []string{">=v1.0.0"},
					},
				},
			},
		},
		"ExistingPackage": {
			reason: "Adding constraints to existing package should append them",
			args: args{
				packages: Packages{
					"pkg-a": &Package{
						Constraints: []string{">=v1.0.0"},
					},
				},
				source:      "pkg-a",
				constraints: []string{"<v2.0.0"},
			},
			want: want{
				packages: Packages{
					"pkg-a": &Package{
						Constraints: []string{">=v1.0.0", "<v2.0.0"},
					},
				},
			},
		},
		"MultipleConstraints": {
			reason: "Adding multiple constraints at once should work",
			args: args{
				packages:    Packages{},
				source:      "pkg-a",
				constraints: []string{">=v1.0.0", "<v2.0.0", "!=v1.5.0"},
			},
			want: want{
				packages: Packages{
					"pkg-a": &Package{
						Constraints: []string{">=v1.0.0", "<v2.0.0", "!=v1.5.0"},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.args.packages.AddConstraints(tc.args.source, tc.args.constraints...)

			if diff := cmp.Diff(tc.want.packages, tc.args.packages); diff != "" {
				t.Errorf("%s\nAddConstraints(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestPackagesUnsatisfiedConstraints(t *testing.T) {
	type args struct {
		packages Packages
		source   string
	}
	type want struct {
		constraints []string
		err         error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"PackageNotSeen": {
			reason: "Package never seen should return wildcard constraint",
			args: args{
				packages: Packages{},
				source:   "pkg-a",
			},
			want: want{
				constraints: []string{"*"},
			},
		},
		"NoVersionNoConstraints": {
			reason: "Package with no version and no constraints should return wildcard",
			args: args{
				packages: Packages{
					"pkg-a": &Package{},
				},
				source: "pkg-a",
			},
			want: want{
				constraints: []string{"*"},
			},
		},
		"NoVersionWithConstraints": {
			reason: "Package with no version but has constraints should return those constraints",
			args: args{
				packages: Packages{
					"pkg-a": &Package{
						Constraints: []string{">=v1.0.0", "<v2.0.0"},
					},
				},
				source: "pkg-a",
			},
			want: want{
				constraints: []string{">=v1.0.0", "<v2.0.0"},
			},
		},
		"HasVersionNoConstraints": {
			reason: "Package with version but no constraints should be satisfied",
			args: args{
				packages: Packages{
					"pkg-a": &Package{
						LockPackage: v1beta1.LockPackage{
							Version: "v1.5.0",
						},
					},
				},
				source: "pkg-a",
			},
			want: want{
				constraints: nil,
			},
		},
		"HasVersionSatisfiesConstraints": {
			reason: "Package with version satisfying constraints should be satisfied",
			args: args{
				packages: Packages{
					"pkg-a": &Package{
						LockPackage: v1beta1.LockPackage{
							Version: "v1.5.0",
						},
						Constraints: []string{">=v1.0.0", "<v2.0.0"},
					},
				},
				source: "pkg-a",
			},
			want: want{
				constraints: nil,
			},
		},
		"HasVersionDoesNotSatisfyConstraints": {
			reason: "Package with version not satisfying constraints should return constraints",
			args: args{
				packages: Packages{
					"pkg-a": &Package{
						LockPackage: v1beta1.LockPackage{
							Version: "v0.9.0",
						},
						Constraints: []string{">=v1.0.0", "<v2.0.0"},
					},
				},
				source: "pkg-a",
			},
			want: want{
				constraints: []string{">=v1.0.0", "<v2.0.0"},
			},
		},
		"DigestConstraintMatches": {
			reason: "Package resolved to digest matching digest constraint should be satisfied",
			args: args{
				packages: Packages{
					"pkg-a": &Package{
						LockPackage: v1beta1.LockPackage{
							Version: "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
						},
						Constraints: []string{"sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"},
					},
				},
				source: "pkg-a",
			},
			want: want{
				constraints: nil,
			},
		},
		"DigestConstraintDoesNotMatch": {
			reason: "Package resolved to digest not matching digest constraint should return constraint",
			args: args{
				packages: Packages{
					"pkg-a": &Package{
						LockPackage: v1beta1.LockPackage{
							Version: "sha256:0987654321fedcba0987654321fedcba0987654321fedcba0987654321fedcba",
						},
						Constraints: []string{"sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"},
					},
				},
				source: "pkg-a",
			},
			want: want{
				constraints: []string{"sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"},
			},
		},
		"DigestConstraintNotYetResolved": {
			reason: "Package with digest constraint but no version should return constraint",
			args: args{
				packages: Packages{
					"pkg-a": &Package{
						Constraints: []string{"sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"},
					},
				},
				source: "pkg-a",
			},
			want: want{
				constraints: []string{"sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"},
			},
		},
		"SemverConstraintOnDigestVersion": {
			reason: "Semver constraints on digest-pinned package should return error",
			args: args{
				packages: Packages{
					"pkg-a": &Package{
						LockPackage: v1beta1.LockPackage{
							Version: "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
						},
						Constraints: []string{">=v1.0.0"},
					},
				},
				source: "pkg-a",
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"MixedDigestAndSemverConstraints": {
			reason: "Mixed digest and semver constraints should return error",
			args: args{
				packages: Packages{
					"pkg-a": &Package{
						Constraints: []string{"sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", ">=v1.0.0"},
					},
				},
				source: "pkg-a",
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"ConflictingDigestConstraints": {
			reason: "Multiple different digest constraints should return error",
			args: args{
				packages: Packages{
					"pkg-a": &Package{
						Constraints: []string{"sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", "sha256:fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321"},
					},
				},
				source: "pkg-a",
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"InvalidSemverConstraint": {
			reason: "Invalid semver constraint should return error",
			args: args{
				packages: Packages{
					"pkg-a": &Package{
						LockPackage: v1beta1.LockPackage{
							Version: "v1.0.0",
						},
						Constraints: []string{"not-a-valid-constraint"},
					},
				},
				source: "pkg-a",
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			constraints, err := tc.args.packages.UnsatisfiedConstraints(tc.args.source)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nUnsatisfiedConstraints(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.constraints, constraints); diff != "" {
				t.Errorf("%s\nUnsatisfiedConstraints(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestPackagesFindReachable(t *testing.T) {
	type args struct {
		packages Packages
		root     string
	}
	type want struct {
		reachable Packages
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SinglePackageNoDeps": {
			reason: "Root package with no dependencies should return just itself",
			args: args{
				packages: Packages{
					"pkg-a": &Package{},
				},
				root: "pkg-a",
			},
			want: want{
				reachable: Packages{
					"pkg-a": &Package{},
				},
			},
		},
		"LinearDependencyChain": {
			reason: "Root with linear dependency chain should return all packages",
			args: args{
				packages: Packages{
					"pkg-a": &Package{
						LockPackage: v1beta1.LockPackage{
							Dependencies: []v1beta1.Dependency{
								{Package: "pkg-b"},
							},
						},
					},
					"pkg-b": &Package{
						LockPackage: v1beta1.LockPackage{
							Dependencies: []v1beta1.Dependency{
								{Package: "pkg-c"},
							},
						},
					},
					"pkg-c": &Package{},
				},
				root: "pkg-a",
			},
			want: want{
				reachable: Packages{
					"pkg-a": &Package{
						LockPackage: v1beta1.LockPackage{
							Dependencies: []v1beta1.Dependency{
								{Package: "pkg-b"},
							},
						},
					},
					"pkg-b": &Package{
						LockPackage: v1beta1.LockPackage{
							Dependencies: []v1beta1.Dependency{
								{Package: "pkg-c"},
							},
						},
					},
					"pkg-c": &Package{},
				},
			},
		},
		"DiamondDependency": {
			reason: "Root with diamond dependency should return all packages",
			args: args{
				packages: Packages{
					"pkg-a": &Package{
						LockPackage: v1beta1.LockPackage{
							Dependencies: []v1beta1.Dependency{
								{Package: "pkg-b"},
								{Package: "pkg-c"},
							},
						},
					},
					"pkg-b": &Package{
						LockPackage: v1beta1.LockPackage{
							Dependencies: []v1beta1.Dependency{
								{Package: "pkg-d"},
							},
						},
					},
					"pkg-c": &Package{
						LockPackage: v1beta1.LockPackage{
							Dependencies: []v1beta1.Dependency{
								{Package: "pkg-d"},
							},
						},
					},
					"pkg-d": &Package{},
				},
				root: "pkg-a",
			},
			want: want{
				reachable: Packages{
					"pkg-a": &Package{
						LockPackage: v1beta1.LockPackage{
							Dependencies: []v1beta1.Dependency{
								{Package: "pkg-b"},
								{Package: "pkg-c"},
							},
						},
					},
					"pkg-b": &Package{
						LockPackage: v1beta1.LockPackage{
							Dependencies: []v1beta1.Dependency{
								{Package: "pkg-d"},
							},
						},
					},
					"pkg-c": &Package{
						LockPackage: v1beta1.LockPackage{
							Dependencies: []v1beta1.Dependency{
								{Package: "pkg-d"},
							},
						},
					},
					"pkg-d": &Package{},
				},
			},
		},
		"UnreachablePackages": {
			reason: "Packages not in dependency tree should not be returned",
			args: args{
				packages: Packages{
					"pkg-a": &Package{
						LockPackage: v1beta1.LockPackage{
							Dependencies: []v1beta1.Dependency{
								{Package: "pkg-b"},
							},
						},
					},
					"pkg-b":      &Package{},
					"pkg-orphan": &Package{},
				},
				root: "pkg-a",
			},
			want: want{
				reachable: Packages{
					"pkg-a": &Package{
						LockPackage: v1beta1.LockPackage{
							Dependencies: []v1beta1.Dependency{
								{Package: "pkg-b"},
							},
						},
					},
					"pkg-b": &Package{},
				},
			},
		},
		"MissingDependency": {
			reason: "Missing dependency should not cause error, just skip it",
			args: args{
				packages: Packages{
					"pkg-a": &Package{
						LockPackage: v1beta1.LockPackage{
							Dependencies: []v1beta1.Dependency{
								{Package: "pkg-missing"},
							},
						},
					},
				},
				root: "pkg-a",
			},
			want: want{
				reachable: Packages{
					"pkg-a": &Package{
						LockPackage: v1beta1.LockPackage{
							Dependencies: []v1beta1.Dependency{
								{Package: "pkg-missing"},
							},
						},
					},
				},
			},
		},
		"CircularDependency": {
			reason: "Circular dependencies should not cause infinite loop",
			args: args{
				packages: Packages{
					"pkg-a": &Package{
						LockPackage: v1beta1.LockPackage{
							Dependencies: []v1beta1.Dependency{
								{Package: "pkg-b"},
							},
						},
					},
					"pkg-b": &Package{
						LockPackage: v1beta1.LockPackage{
							Dependencies: []v1beta1.Dependency{
								{Package: "pkg-a"},
							},
						},
					},
				},
				root: "pkg-a",
			},
			want: want{
				reachable: Packages{
					"pkg-a": &Package{
						LockPackage: v1beta1.LockPackage{
							Dependencies: []v1beta1.Dependency{
								{Package: "pkg-b"},
							},
						},
					},
					"pkg-b": &Package{
						LockPackage: v1beta1.LockPackage{
							Dependencies: []v1beta1.Dependency{
								{Package: "pkg-a"},
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			reachable := tc.args.packages.FindReachable(tc.args.root)

			if diff := cmp.Diff(tc.want.reachable, reachable, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("%s\nFindReachable(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestPackagesToLockPackages(t *testing.T) {
	type args struct {
		packages Packages
		root     string
		current  []v1beta1.LockPackage
	}
	type want struct {
		lockPackages []v1beta1.LockPackage
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SinglePackage": {
			reason: "Single resolved package should be converted",
			args: args{
				packages: Packages{
					"pkg-a": &Package{
						LockPackage: v1beta1.LockPackage{
							Source:  "pkg-a",
							Version: "v1.0.0",
						},
					},
				},
				root:    "pkg-a",
				current: []v1beta1.LockPackage{},
			},
			want: want{
				lockPackages: []v1beta1.LockPackage{
					{Source: "pkg-a", Version: "v1.0.0"},
				},
			},
		},
		"MultipleReachablePackages": {
			reason: "All reachable packages should be included",
			args: args{
				packages: Packages{
					"pkg-a": &Package{
						LockPackage: v1beta1.LockPackage{
							Source:  "pkg-a",
							Version: "v1.0.0",
							Dependencies: []v1beta1.Dependency{
								{Package: "pkg-b"},
							},
						},
					},
					"pkg-b": &Package{
						LockPackage: v1beta1.LockPackage{
							Source:  "pkg-b",
							Version: "v2.0.0",
						},
					},
				},
				root:    "pkg-a",
				current: []v1beta1.LockPackage{},
			},
			want: want{
				lockPackages: []v1beta1.LockPackage{
					{
						Source:  "pkg-a",
						Version: "v1.0.0",
						Dependencies: []v1beta1.Dependency{
							{Package: "pkg-b"},
						},
					},
					{Source: "pkg-b", Version: "v2.0.0"},
				},
			},
		},
		"PreserveUnrelatedPackagesFromCurrent": {
			reason: "Packages from other roots in current Lock should be preserved",
			args: args{
				packages: Packages{
					"pkg-a": &Package{
						LockPackage: v1beta1.LockPackage{
							Source:  "pkg-a",
							Version: "v1.0.0",
						},
					},
				},
				root: "pkg-a",
				current: []v1beta1.LockPackage{
					{Source: "pkg-a", Version: "v0.9.0"}, // Will be replaced by resolved version.
					{Source: "pkg-other-root", Version: "v3.0.0"},
				},
			},
			want: want{
				lockPackages: []v1beta1.LockPackage{
					{Source: "pkg-a", Version: "v1.0.0"},
					{Source: "pkg-other-root", Version: "v3.0.0"},
				},
			},
		},
		"MissingDependencyNotIncluded": {
			reason: "Dependencies that don't exist in packages map should not be included",
			args: args{
				packages: Packages{
					"pkg-a": &Package{
						LockPackage: v1beta1.LockPackage{
							Source:  "pkg-a",
							Version: "v1.0.0",
							Dependencies: []v1beta1.Dependency{
								{Package: "pkg-missing"},
							},
						},
					},
				},
				root:    "pkg-a",
				current: []v1beta1.LockPackage{},
			},
			want: want{
				lockPackages: []v1beta1.LockPackage{
					{
						Source:  "pkg-a",
						Version: "v1.0.0",
						Dependencies: []v1beta1.Dependency{
							{Package: "pkg-missing"},
						},
					},
				},
			},
		},
		"UnreachablePackagesNotIncluded": {
			reason: "Packages not reachable from root should not be included",
			args: args{
				packages: Packages{
					"pkg-a": &Package{
						LockPackage: v1beta1.LockPackage{
							Source:  "pkg-a",
							Version: "v1.0.0",
						},
					},
					"pkg-orphan": &Package{
						LockPackage: v1beta1.LockPackage{
							Source:  "pkg-orphan",
							Version: "v2.0.0",
						},
					},
				},
				root:    "pkg-a",
				current: []v1beta1.LockPackage{},
			},
			want: want{
				lockPackages: []v1beta1.LockPackage{
					{Source: "pkg-a", Version: "v1.0.0"},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			lockPackages := tc.args.packages.ToLockPackages(tc.args.root, tc.args.current)

			if diff := cmp.Diff(tc.want.lockPackages, lockPackages, cmpopts.SortSlices(func(a, b v1beta1.LockPackage) bool {
				return a.Source < b.Source
			}), cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("%s\nToLockPackages(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestPackagesValidateVersion(t *testing.T) {
	type args struct {
		packages Packages
		source   string
		version  string
	}
	type want struct {
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoConstraints": {
			reason: "Installing any version when there are no constraints should succeed",
			args: args{
				packages: Packages{},
				source:   "pkg-a",
				version:  "v1.0.0",
			},
			want: want{
				err: nil,
			},
		},
		"SatisfiesSemverConstraint": {
			reason: "Installing version that satisfies existing constraint should succeed",
			args: args{
				packages: Packages{
					"pkg-b": &Package{
						LockPackage: v1beta1.LockPackage{
							Dependencies: []v1beta1.Dependency{
								{Package: "pkg-a", Constraints: ">=v1.0.0"},
							},
						},
					},
				},
				source:  "pkg-a",
				version: "v1.5.0",
			},
			want: want{
				err: nil,
			},
		},
		"DoesNotSatisfySemverConstraint": {
			reason: "Installing version that does not satisfy constraint should fail",
			args: args{
				packages: Packages{
					"pkg-b": &Package{
						LockPackage: v1beta1.LockPackage{
							Dependencies: []v1beta1.Dependency{
								{Package: "pkg-a", Constraints: ">=v1.0.0"},
							},
						},
					},
				},
				source:  "pkg-a",
				version: "v0.9.0",
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"SatisfiesDigestConstraint": {
			reason: "Installing digest that matches constraint should succeed",
			args: args{
				packages: Packages{
					"pkg-b": &Package{
						LockPackage: v1beta1.LockPackage{
							Dependencies: []v1beta1.Dependency{
								{Package: "pkg-a", Constraints: "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"},
							},
						},
					},
				},
				source:  "pkg-a",
				version: "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			},
			want: want{
				err: nil,
			},
		},
		"DoesNotSatisfyDigestConstraint": {
			reason: "Installing digest that does not match constraint should fail",
			args: args{
				packages: Packages{
					"pkg-b": &Package{
						LockPackage: v1beta1.LockPackage{
							Dependencies: []v1beta1.Dependency{
								{Package: "pkg-a", Constraints: "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"},
							},
						},
					},
				},
				source:  "pkg-a",
				version: "sha256:fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321",
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"InvalidConstraint": {
			reason: "Invalid semver constraint should return error",
			args: args{
				packages: Packages{
					"pkg-b": &Package{
						LockPackage: v1beta1.LockPackage{
							Dependencies: []v1beta1.Dependency{
								{Package: "pkg-a", Constraints: "not-valid"},
							},
						},
					},
				},
				source:  "pkg-a",
				version: "v1.0.0",
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := tc.args.packages.ValidateVersion(tc.args.source, tc.args.version)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nValidateVersion(...): -want error, +got error:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestSelectVersion(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		client      xpkg.Client
		source      string
		constraints []string
	}
	type want struct {
		pkg *Package
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"WildcardSelectsOldest": {
			reason: "Wildcard constraint should select oldest available version",
			args: args{
				client: &MockPackageClient{
					MockListVersions: func(_ context.Context, _ string, _ ...xpkg.GetOption) ([]string, error) {
						return []string{"v1.0.0", "v2.0.0", "v3.0.0"}, nil
					},
					MockGet: func(_ context.Context, _ string, _ ...xpkg.GetOption) (*xpkg.Package, error) {
						//nolint:contextcheck // Test helper doesn't need context.
						return NewTestPackage(t, "pkg-a", "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"), nil
					},
				},
				source:      "pkg-a",
				constraints: []string{"*"},
			},
			want: want{
				pkg: &Package{
					LockPackage: v1beta1.LockPackage{
						Name:       "pkg-a-1234567890ab",
						APIVersion: ptr.To(v1.SchemeGroupVersion.String()),
						Kind:       ptr.To("Provider"),
						Source:     "pkg-a",
						Version:    "v1.0.0",
					},
					Digest:      "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
					Constraints: []string{"*"},
				},
			},
		},
		"SingleConstraintSelectsOldest": {
			reason: "Single constraint should select oldest satisfying version",
			args: args{
				client: &MockPackageClient{
					MockListVersions: func(_ context.Context, _ string, _ ...xpkg.GetOption) ([]string, error) {
						return []string{"v0.9.0", "v1.0.0", "v1.5.0", "v2.0.0"}, nil
					},
					MockGet: func(_ context.Context, _ string, _ ...xpkg.GetOption) (*xpkg.Package, error) {
						//nolint:contextcheck // Test helper doesn't need context.
						return NewTestPackage(t, "pkg-a", "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"), nil
					},
				},
				source:      "pkg-a",
				constraints: []string{">=v1.0.0"},
			},
			want: want{
				pkg: &Package{
					LockPackage: v1beta1.LockPackage{
						Name:       "pkg-a-1234567890ab",
						APIVersion: ptr.To(v1.SchemeGroupVersion.String()),
						Kind:       ptr.To("Provider"),
						Source:     "pkg-a",
						Version:    "v1.0.0",
					},
					Digest:      "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
					Constraints: []string{">=v1.0.0"},
				},
			},
		},
		"MultipleConstraintsSelectsOldest": {
			reason: "Multiple constraints should select oldest satisfying all",
			args: args{
				client: &MockPackageClient{
					MockListVersions: func(_ context.Context, _ string, _ ...xpkg.GetOption) ([]string, error) {
						return []string{"v0.9.0", "v1.0.0", "v1.5.0", "v2.0.0", "v3.0.0"}, nil
					},
					MockGet: func(_ context.Context, _ string, _ ...xpkg.GetOption) (*xpkg.Package, error) {
						//nolint:contextcheck // Test helper doesn't need context.
						return NewTestPackage(t, "pkg-a", "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"), nil
					},
				},
				source:      "pkg-a",
				constraints: []string{">=v1.0.0", "<v2.0.0"},
			},
			want: want{
				pkg: &Package{
					LockPackage: v1beta1.LockPackage{
						Name:       "pkg-a-1234567890ab",
						APIVersion: ptr.To(v1.SchemeGroupVersion.String()),
						Kind:       ptr.To("Provider"),
						Source:     "pkg-a",
						Version:    "v1.0.0",
					},
					Digest:      "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
					Constraints: []string{">=v1.0.0", "<v2.0.0"},
				},
			},
		},
		"NoVersionSatisfiesConstraints": {
			reason: "No available version satisfying constraints should return error",
			args: args{
				client: &MockPackageClient{
					MockListVersions: func(_ context.Context, _ string, _ ...xpkg.GetOption) ([]string, error) {
						return []string{"v0.5.0", "v0.9.0"}, nil
					},
				},
				source:      "pkg-a",
				constraints: []string{">=v1.0.0"},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"DigestConstraint": {
			reason: "Digest constraint should fetch that specific digest",
			args: args{
				client: &MockPackageClient{
					MockGet: func(_ context.Context, ref string, _ ...xpkg.GetOption) (*xpkg.Package, error) {
						if ref != "pkg-a@sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef" {
							return nil, errBoom
						}
						//nolint:contextcheck // Test helper doesn't need context.
						return NewTestPackage(t, "pkg-a", "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"), nil
					},
				},
				source:      "pkg-a",
				constraints: []string{"sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"},
			},
			want: want{
				pkg: &Package{
					LockPackage: v1beta1.LockPackage{
						Name:       "pkg-a-1234567890ab",
						APIVersion: ptr.To(v1.SchemeGroupVersion.String()),
						Kind:       ptr.To("Provider"),
						Source:     "pkg-a",
						Version:    "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
					},
					Digest:      "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
					Constraints: []string{"sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"},
				},
			},
		},
		"MixedConstraintTypes": {
			reason: "Mixed digest and semver constraints should return error",
			args: args{
				client:      &MockPackageClient{},
				source:      "pkg-a",
				constraints: []string{"sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", ">=v1.0.0"},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"ConflictingDigestConstraints": {
			reason: "Multiple different digest constraints should return error",
			args: args{
				client:      &MockPackageClient{},
				source:      "pkg-a",
				constraints: []string{"sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", "sha256:fedcba0987654321fedcba0987654321fedcba0987654321fedcba0987654321"},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"ListVersionsError": {
			reason: "Error listing versions should be returned",
			args: args{
				client: &MockPackageClient{
					MockListVersions: func(_ context.Context, _ string, _ ...xpkg.GetOption) ([]string, error) {
						return nil, errBoom
					},
				},
				source:      "pkg-a",
				constraints: []string{">=v1.0.0"},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"GetPackageError": {
			reason: "Error fetching package should be returned",
			args: args{
				client: &MockPackageClient{
					MockListVersions: func(_ context.Context, _ string, _ ...xpkg.GetOption) ([]string, error) {
						return []string{"v1.0.0"}, nil
					},
					MockGet: func(_ context.Context, _ string, _ ...xpkg.GetOption) (*xpkg.Package, error) {
						return nil, errBoom
					},
				},
				source:      "pkg-a",
				constraints: []string{">=v1.0.0"},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := NewTighteningConstraintSolver(tc.args.client)
			pkg, err := s.SelectVersion(context.Background(), tc.args.source, tc.args.constraints)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nSelectVersion(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.pkg, pkg, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("%s\nSelectVersion(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestSolve(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		client  xpkg.Client
		ref     string
		current []v1beta1.LockPackage
	}
	type want struct {
		lockPackages []v1beta1.LockPackage
		err          error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SinglePackageNoDeps": {
			reason: "Root package with no dependencies should return just itself",
			args: args{
				client: &MockPackageClient{
					MockGet: func(_ context.Context, ref string, _ ...xpkg.GetOption) (*xpkg.Package, error) {
						if ref != "pkg-a:v1.0.0" {
							return nil, errBoom
						}
						//nolint:contextcheck // Test helper doesn't need context.
						return NewTestPackage(t, "pkg-a", "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"), nil
					},
				},
				ref:     "pkg-a:v1.0.0",
				current: []v1beta1.LockPackage{},
			},
			want: want{
				lockPackages: []v1beta1.LockPackage{
					{
						Name:       "pkg-a-1234567890ab",
						APIVersion: ptr.To(v1.SchemeGroupVersion.String()),
						Kind:       ptr.To("Provider"),
						Source:     "pkg-a",
						Version:    "v1.0.0",
					},
				},
			},
		},
		"PackageWithDependency": {
			reason: "Root package with single dependency should resolve both",
			args: args{
				client: &MockPackageClient{
					MockGet: func(_ context.Context, ref string, _ ...xpkg.GetOption) (*xpkg.Package, error) {
						switch ref {
						case "pkg-a:v1.0.0":
							//nolint:contextcheck // Test helper doesn't need context.
							return NewTestPackage(t, "pkg-a", "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
								pkgmetav1.Dependency{Provider: ptr.To("pkg-b"), Version: ">=v2.0.0"},
							), nil
						case "pkg-b:v2.0.0":
							//nolint:contextcheck // Test helper doesn't need context.
							return NewTestPackage(t, "pkg-b", "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"), nil
						default:
							return nil, errBoom
						}
					},
					MockListVersions: func(_ context.Context, source string, _ ...xpkg.GetOption) ([]string, error) {
						if source == "pkg-b" {
							return []string{"v1.0.0", "v2.0.0", "v3.0.0"}, nil
						}
						return nil, errBoom
					},
				},
				ref:     "pkg-a:v1.0.0",
				current: []v1beta1.LockPackage{},
			},
			want: want{
				lockPackages: []v1beta1.LockPackage{
					{
						Name:       "pkg-a-aaaaaaaaaaaa",
						APIVersion: ptr.To(v1.SchemeGroupVersion.String()),
						Kind:       ptr.To("Provider"),
						Source:     "pkg-a",
						Version:    "v1.0.0",
						Dependencies: []v1beta1.Dependency{
							{Package: "pkg-b", Type: ptr.To(v1beta1.ProviderPackageType), Constraints: ">=v2.0.0"},
						},
					},
					{
						Name:       "pkg-b-bbbbbbbbbbbb",
						APIVersion: ptr.To(v1.SchemeGroupVersion.String()),
						Kind:       ptr.To("Provider"),
						Source:     "pkg-b",
						Version:    "v2.0.0",
					},
				},
			},
		},
		"PreserveExistingPackages": {
			reason: "Existing packages from current Lock should be preserved",
			args: args{
				client: &MockPackageClient{
					MockGet: func(_ context.Context, ref string, _ ...xpkg.GetOption) (*xpkg.Package, error) {
						if ref != "pkg-a:v1.0.0" {
							return nil, errBoom
						}
						//nolint:contextcheck // Test helper doesn't need context.
						return NewTestPackage(t, "pkg-a", "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"), nil
					},
				},
				ref: "pkg-a:v1.0.0",
				current: []v1beta1.LockPackage{
					{Source: "pkg-other-root", Version: "v2.0.0"},
				},
			},
			want: want{
				lockPackages: []v1beta1.LockPackage{
					{
						Name:       "pkg-a-1234567890ab",
						APIVersion: ptr.To(v1.SchemeGroupVersion.String()),
						Kind:       ptr.To("Provider"),
						Source:     "pkg-a",
						Version:    "v1.0.0",
					},
					{Source: "pkg-other-root", Version: "v2.0.0"},
				},
			},
		},
		"TighteningConstraints": {
			reason: "Multiple dependencies with tightening constraints should resolve correctly",
			args: args{
				client: &MockPackageClient{
					MockGet: func(_ context.Context, ref string, _ ...xpkg.GetOption) (*xpkg.Package, error) {
						switch ref {
						case "pkg-a:v1.0.0":
							//nolint:contextcheck // Test helper doesn't need context.
							return NewTestPackage(t, "pkg-a", "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
								pkgmetav1.Dependency{Provider: ptr.To("pkg-c"), Version: ">=v1.0.0"},
								pkgmetav1.Dependency{Provider: ptr.To("pkg-b"), Version: ">=v1.0.0"},
							), nil
						case "pkg-b:v1.0.0":
							//nolint:contextcheck // Test helper doesn't need context.
							return NewTestPackage(t, "pkg-b", "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
								pkgmetav1.Dependency{Provider: ptr.To("pkg-c"), Version: ">=v2.0.0"},
							), nil
						case "pkg-c:v1.0.0":
							// First fetch of pkg-c before constraint tightening.
							//nolint:contextcheck // Test helper doesn't need context.
							return NewTestPackage(t, "pkg-c", "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"), nil
						case "pkg-c:v2.0.0":
							// Second fetch after tightening constraint to >=v2.0.0.
							//nolint:contextcheck // Test helper doesn't need context.
							return NewTestPackage(t, "pkg-c", "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"), nil
						default:
							return nil, errBoom
						}
					},
					MockListVersions: func(_ context.Context, source string, _ ...xpkg.GetOption) ([]string, error) {
						switch source {
						case "pkg-b":
							return []string{"v1.0.0", "v2.0.0"}, nil
						case "pkg-c":
							return []string{"v1.0.0", "v2.0.0", "v3.0.0"}, nil
						default:
							return nil, errBoom
						}
					},
				},
				ref:     "pkg-a:v1.0.0",
				current: []v1beta1.LockPackage{},
			},
			want: want{
				lockPackages: []v1beta1.LockPackage{
					{
						Name:       "pkg-a-aaaaaaaaaaaa",
						APIVersion: ptr.To(v1.SchemeGroupVersion.String()),
						Kind:       ptr.To("Provider"),
						Source:     "pkg-a",
						Version:    "v1.0.0",
						Dependencies: []v1beta1.Dependency{
							{Package: "pkg-c", Type: ptr.To(v1beta1.ProviderPackageType), Constraints: ">=v1.0.0"},
							{Package: "pkg-b", Type: ptr.To(v1beta1.ProviderPackageType), Constraints: ">=v1.0.0"},
						},
					},
					{
						Name:       "pkg-b-bbbbbbbbbbbb",
						APIVersion: ptr.To(v1.SchemeGroupVersion.String()),
						Kind:       ptr.To("Provider"),
						Source:     "pkg-b",
						Version:    "v1.0.0",
						Dependencies: []v1beta1.Dependency{
							{Package: "pkg-c", Type: ptr.To(v1beta1.ProviderPackageType), Constraints: ">=v2.0.0"},
						},
					},
					{
						Name:       "pkg-c-cccccccccccc",
						APIVersion: ptr.To(v1.SchemeGroupVersion.String()),
						Kind:       ptr.To("Provider"),
						Source:     "pkg-c",
						Version:    "v2.0.0",
					},
				},
			},
		},
		"InvalidPackageRef": {
			reason: "Invalid package reference should return error",
			args: args{
				client:  &MockPackageClient{},
				ref:     "not::valid::ref",
				current: []v1beta1.LockPackage{},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"RootPackageFetchError": {
			reason: "Error fetching root package should be returned",
			args: args{
				client: &MockPackageClient{
					MockGet: func(_ context.Context, _ string, _ ...xpkg.GetOption) (*xpkg.Package, error) {
						return nil, errBoom
					},
				},
				ref:     "pkg-a:v1.0.0",
				current: []v1beta1.LockPackage{},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"RootVersionConflict": {
			reason: "Root version conflicting with existing constraints should return error",
			args: args{
				client: &MockPackageClient{},
				ref:    "pkg-a:v0.5.0",
				current: []v1beta1.LockPackage{
					{
						Source: "pkg-b",
						Dependencies: []v1beta1.Dependency{
							{Package: "pkg-a", Constraints: ">=v1.0.0"},
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
			s := NewTighteningConstraintSolver(tc.args.client)
			lockPackages, err := s.Solve(context.Background(), tc.args.ref, tc.args.current)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nSolve(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.lockPackages, lockPackages, cmpopts.SortSlices(func(a, b v1beta1.LockPackage) bool {
				return a.Source < b.Source
			}), cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("%s\nSolve(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestConvertDependencies(t *testing.T) {
	type args struct {
		deps []pkgmetav1.Dependency
	}
	type want struct {
		lockDeps []v1beta1.Dependency
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"EmptyDependencies": {
			reason: "Empty dependency list should return empty list",
			args: args{
				deps: []pkgmetav1.Dependency{},
			},
			want: want{
				lockDeps: []v1beta1.Dependency{},
			},
		},
		"ModernDependency": {
			reason: "Dependency with apiVersion, kind, and package should be converted",
			args: args{
				deps: []pkgmetav1.Dependency{
					{
						APIVersion: ptr.To(v1.SchemeGroupVersion.String()),
						Kind:       ptr.To("Provider"),
						Package:    ptr.To("pkg-a"),
						Version:    ">=v1.0.0",
					},
				},
			},
			want: want{
				lockDeps: []v1beta1.Dependency{
					{
						APIVersion:  ptr.To(v1.SchemeGroupVersion.String()),
						Kind:        ptr.To("Provider"),
						Package:     "pkg-a",
						Constraints: ">=v1.0.0",
					},
				},
			},
		},
		"LegacyProviderDependency": {
			reason: "Legacy Provider dependency should be converted with Type",
			args: args{
				deps: []pkgmetav1.Dependency{
					{
						Provider: ptr.To("pkg-a"),
						Version:  ">=v1.0.0",
					},
				},
			},
			want: want{
				lockDeps: []v1beta1.Dependency{
					{
						Package:     "pkg-a",
						Type:        ptr.To(v1beta1.ProviderPackageType),
						Constraints: ">=v1.0.0",
					},
				},
			},
		},
		"LegacyConfigurationDependency": {
			reason: "Legacy Configuration dependency should be converted with Type",
			args: args{
				deps: []pkgmetav1.Dependency{
					{
						Configuration: ptr.To("pkg-a"),
						Version:       ">=v1.0.0",
					},
				},
			},
			want: want{
				lockDeps: []v1beta1.Dependency{
					{
						Package:     "pkg-a",
						Type:        ptr.To(v1beta1.ConfigurationPackageType),
						Constraints: ">=v1.0.0",
					},
				},
			},
		},
		"LegacyFunctionDependency": {
			reason: "Legacy Function dependency should be converted with Type",
			args: args{
				deps: []pkgmetav1.Dependency{
					{
						Function: ptr.To("pkg-a"),
						Version:  ">=v1.0.0",
					},
				},
			},
			want: want{
				lockDeps: []v1beta1.Dependency{
					{
						Package:     "pkg-a",
						Type:        ptr.To(v1beta1.FunctionPackageType),
						Constraints: ">=v1.0.0",
					},
				},
			},
		},
		"MixedDependencies": {
			reason: "Mixed modern and legacy dependencies should all be converted",
			args: args{
				deps: []pkgmetav1.Dependency{
					{
						APIVersion: ptr.To(v1.SchemeGroupVersion.String()),
						Kind:       ptr.To("Provider"),
						Package:    ptr.To("pkg-a"),
						Version:    ">=v1.0.0",
					},
					{
						Provider: ptr.To("pkg-b"),
						Version:  ">=v2.0.0",
					},
					{
						Configuration: ptr.To("pkg-c"),
						Version:       ">=v3.0.0",
					},
				},
			},
			want: want{
				lockDeps: []v1beta1.Dependency{
					{
						APIVersion:  ptr.To(v1.SchemeGroupVersion.String()),
						Kind:        ptr.To("Provider"),
						Package:     "pkg-a",
						Constraints: ">=v1.0.0",
					},
					{
						Package:     "pkg-b",
						Type:        ptr.To(v1beta1.ProviderPackageType),
						Constraints: ">=v2.0.0",
					},
					{
						Package:     "pkg-c",
						Type:        ptr.To(v1beta1.ConfigurationPackageType),
						Constraints: ">=v3.0.0",
					},
				},
			},
		},
		"InvalidDependency": {
			reason: "Invalid dependency with no package fields should be skipped",
			args: args{
				deps: []pkgmetav1.Dependency{
					{
						Version: ">=v1.0.0",
						// No package/provider/configuration/function.
					},
					{
						Provider: ptr.To("pkg-a"),
						Version:  ">=v1.0.0",
					},
				},
			},
			want: want{
				lockDeps: []v1beta1.Dependency{
					{
						Package:     "pkg-a",
						Type:        ptr.To(v1beta1.ProviderPackageType),
						Constraints: ">=v1.0.0",
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			lockDeps := ConvertDependencies(tc.args.deps)

			if diff := cmp.Diff(tc.want.lockDeps, lockDeps); diff != "" {
				t.Errorf("%s\nConvertDependencies(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestNewPackage(t *testing.T) {
	type args struct {
		xp          *xpkg.Package
		version     string
		constraints []string
	}
	type want struct {
		pkg *Package
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ValidPackage": {
			reason: "Valid package should be converted correctly",
			args: args{
				xp:          NewTestPackage(t, "pkg-a", "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", pkgmetav1.Dependency{Provider: ptr.To("pkg-b"), Version: ">=v1.0.0"}),
				version:     "v1.0.0",
				constraints: []string{">=v1.0.0", "<v2.0.0"},
			},
			want: want{
				pkg: &Package{
					LockPackage: v1beta1.LockPackage{
						Name:       "pkg-a-1234567890ab",
						APIVersion: ptr.To(v1.SchemeGroupVersion.String()),
						Kind:       ptr.To("Provider"),
						Source:     "pkg-a",
						Version:    "v1.0.0",
						Dependencies: []v1beta1.Dependency{
							{Package: "pkg-b", Type: ptr.To(v1beta1.ProviderPackageType), Constraints: ">=v1.0.0"},
						},
					},
					Digest:      "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
					Constraints: []string{">=v1.0.0", "<v2.0.0"},
				},
			},
		},
		"InvalidDigest": {
			reason: "Invalid digest should return error",
			args: args{
				xp: &xpkg.Package{
					Source:  "pkg-a",
					Digest:  "not-a-valid-digest",
					Package: NewTestPackage(t, "pkg-a", "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef").Package,
				},
				version:     "v1.0.0",
				constraints: []string{},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			pkg, err := NewPackage(tc.args.xp, tc.args.version, tc.args.constraints)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nNewPackage(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.pkg, pkg, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("%s\nNewPackage(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

type MockPackageClient struct {
	MockGet          func(ctx context.Context, ref string, opts ...xpkg.GetOption) (*xpkg.Package, error)
	MockListVersions func(ctx context.Context, source string, opts ...xpkg.GetOption) ([]string, error)
}

func (m *MockPackageClient) Get(ctx context.Context, ref string, opts ...xpkg.GetOption) (*xpkg.Package, error) {
	return m.MockGet(ctx, ref, opts...)
}

func (m *MockPackageClient) ListVersions(ctx context.Context, source string, opts ...xpkg.GetOption) ([]string, error) {
	return m.MockListVersions(ctx, source, opts...)
}

// NewTestPackage creates a simple test package with the given source and digest.
// Optionally includes dependencies if provided.
func NewTestPackage(t *testing.T, source, digest string, deps ...pkgmetav1.Dependency) *xpkg.Package {
	t.Helper()

	// Build metadata JSON with dependencies if provided
	depsJSON := ""
	if len(deps) > 0 {
		depsJSON = `,"spec":{"dependsOn":[`
		for i, dep := range deps {
			if i > 0 {
				depsJSON += ","
			}
			switch {
			case dep.Provider != nil:
				depsJSON += fmt.Sprintf(`{"provider":%q,"version":%q}`, *dep.Provider, dep.Version)
			case dep.Configuration != nil:
				depsJSON += fmt.Sprintf(`{"configuration":%q,"version":%q}`, *dep.Configuration, dep.Version)
			case dep.Function != nil:
				depsJSON += fmt.Sprintf(`{"function":%q,"version":%q}`, *dep.Function, dep.Version)
			}
		}
		depsJSON += `]}`
	}

	providerMeta := fmt.Sprintf(`{"apiVersion":"meta.pkg.crossplane.io/v1","kind":"Provider","metadata":{"name":"provider-test"}%s}`, depsJSON)

	p, err := xpkg.BuildMetaScheme()
	if err != nil {
		t.Fatalf("failed to build meta scheme: %v", err)
	}
	obj, err := xpkg.BuildObjectScheme()
	if err != nil {
		t.Fatalf("failed to build object scheme: %v", err)
	}
	parser := parser.New(p, obj)

	pkg, err := parser.Parse(context.Background(), io.NopCloser(strings.NewReader("---\n"+providerMeta)))
	if err != nil {
		t.Fatalf("failed to parse test package: %v", err)
	}

	return &xpkg.Package{
		Package: pkg,
		Source:  source,
		Digest:  digest,
	}
}
