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
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/utils/ptr"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/parser"

	pkgmetav1 "github.com/crossplane/crossplane/v2/apis/pkg/meta/v1"
	"github.com/crossplane/crossplane/v2/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/v2/internal/xpkg"
)

const (
	providerNoDeps   = `{"apiVersion":"meta.pkg.crossplane.io/v1","kind":"Provider","metadata":{"name":"test"}}`
	providerWithDep  = `{"apiVersion":"meta.pkg.crossplane.io/v1","kind":"Provider","metadata":{"name":"test"},"spec":{"dependsOn":[{"provider":"xpkg.crossplane.io/provider-family-aws","version":">=v1.0.0"}]}}`
	providerCircular = `{"apiVersion":"meta.pkg.crossplane.io/v1","kind":"Provider","metadata":{"name":"test"},"spec":{"dependsOn":[{"provider":"xpkg.crossplane.io/provider-aws","version":">=v1.0.0"}]}}`
)

var _ xpkg.Client = &MockClient{}

type MockClient struct {
	MockGet          func(_ context.Context, _ string, _ ...xpkg.GetOption) (*xpkg.Package, error)
	MockListVersions func(_ context.Context, _ string, _ ...xpkg.GetOption) ([]string, error)
}

func (m *MockClient) Get(ctx context.Context, ref string, opts ...xpkg.GetOption) (*xpkg.Package, error) {
	return m.MockGet(ctx, ref, opts...)
}

func (m *MockClient) ListVersions(ctx context.Context, source string, opts ...xpkg.GetOption) ([]string, error) {
	return m.MockListVersions(ctx, source, opts...)
}

func TestFilterAndSortVersions(t *testing.T) {
	type args struct {
		tags []string
	}
	type want struct {
		versions []string
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"EmptyTags": {
			reason: "Empty tags should return empty list",
			args: args{
				tags: []string{},
			},
			want: want{
				versions: []string{},
			},
		},
		"ValidSemverTags": {
			reason: "Valid semver tags should be sorted ascending",
			args: args{
				tags: []string{"v1.2.3", "v1.0.0", "v2.0.0", "v1.5.0"},
			},
			want: want{
				versions: []string{"v1.0.0", "v1.2.3", "v1.5.0", "v2.0.0"},
			},
		},
		"MixedValidAndInvalid": {
			reason: "Invalid tags should be filtered out",
			args: args{
				tags: []string{"v1.2.3", "latest", "v1.0.0", "main", "v2.0.0"},
			},
			want: want{
				versions: []string{"v1.0.0", "v1.2.3", "v2.0.0"},
			},
		},
		"PrereleaseTags": {
			reason: "Prerelease tags should be included and sorted correctly",
			args: args{
				tags: []string{"v1.0.0", "v1.0.0-alpha", "v1.0.0-beta", "v1.0.1"},
			},
			want: want{
				versions: []string{"v1.0.0-alpha", "v1.0.0-beta", "v1.0.0", "v1.0.1"},
			},
		},
		"WithoutVPrefix": {
			reason: "Tags without v prefix should be handled",
			args: args{
				tags: []string{"1.2.3", "1.0.0", "2.0.0"},
			},
			want: want{
				versions: []string{"1.0.0", "1.2.3", "2.0.0"},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := xpkg.FilterAndSortVersions(tc.args.tags)

			if diff := cmp.Diff(tc.want.versions, got); diff != "" {
				t.Errorf("\n%s\nFilterAndSortVersions(...): -want, +got:\n%s", tc.reason, diff)
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
			reason: "Empty dependencies should return empty list",
			args: args{
				deps: []pkgmetav1.Dependency{},
			},
			want: want{
				lockDeps: []v1beta1.Dependency{},
			},
		},
		"ModernFormat": {
			reason: "Modern format with apiVersion, kind, and package should be converted",
			args: args{
				deps: []pkgmetav1.Dependency{
					{
						APIVersion: ptr.To("pkg.crossplane.io/v1"),
						Kind:       ptr.To("Provider"),
						Package:    ptr.To("xpkg.crossplane.io/provider-aws"),
						Version:    ">=v1.0.0",
					},
				},
			},
			want: want{
				lockDeps: []v1beta1.Dependency{
					{
						APIVersion:  ptr.To("pkg.crossplane.io/v1"),
						Kind:        ptr.To("Provider"),
						Package:     "xpkg.crossplane.io/provider-aws",
						Constraints: ">=v1.0.0",
					},
				},
			},
		},
		"LegacyProviderFormat": {
			reason: "Legacy provider format should be converted with type",
			args: args{
				deps: []pkgmetav1.Dependency{
					{
						Provider: ptr.To("xpkg.crossplane.io/provider-aws"),
						Version:  ">=v1.0.0",
					},
				},
			},
			want: want{
				lockDeps: []v1beta1.Dependency{
					{
						Package:     "xpkg.crossplane.io/provider-aws",
						Constraints: ">=v1.0.0",
						Type:        ptr.To(v1beta1.ProviderPackageType),
					},
				},
			},
		},
		"LegacyConfigurationFormat": {
			reason: "Legacy configuration format should be converted with type",
			args: args{
				deps: []pkgmetav1.Dependency{
					{
						Configuration: ptr.To("xpkg.crossplane.io/configuration-app"),
						Version:       ">=v1.0.0",
					},
				},
			},
			want: want{
				lockDeps: []v1beta1.Dependency{
					{
						Package:     "xpkg.crossplane.io/configuration-app",
						Constraints: ">=v1.0.0",
						Type:        ptr.To(v1beta1.ConfigurationPackageType),
					},
				},
			},
		},
		"LegacyFunctionFormat": {
			reason: "Legacy function format should be converted with type",
			args: args{
				deps: []pkgmetav1.Dependency{
					{
						Function: ptr.To("xpkg.crossplane.io/function-auto-ready"),
						Version:  ">=v1.0.0",
					},
				},
			},
			want: want{
				lockDeps: []v1beta1.Dependency{
					{
						Package:     "xpkg.crossplane.io/function-auto-ready",
						Constraints: ">=v1.0.0",
						Type:        ptr.To(v1beta1.FunctionPackageType),
					},
				},
			},
		},
		"MixedFormats": {
			reason: "Mixed modern and legacy formats should be converted correctly",
			args: args{
				deps: []pkgmetav1.Dependency{
					{
						APIVersion: ptr.To("pkg.crossplane.io/v1"),
						Kind:       ptr.To("Provider"),
						Package:    ptr.To("xpkg.crossplane.io/provider-aws"),
						Version:    ">=v1.0.0",
					},
					{
						Provider: ptr.To("xpkg.crossplane.io/provider-gcp"),
						Version:  ">=v2.0.0",
					},
				},
			},
			want: want{
				lockDeps: []v1beta1.Dependency{
					{
						APIVersion:  ptr.To("pkg.crossplane.io/v1"),
						Kind:        ptr.To("Provider"),
						Package:     "xpkg.crossplane.io/provider-aws",
						Constraints: ">=v1.0.0",
					},
					{
						Package:     "xpkg.crossplane.io/provider-gcp",
						Constraints: ">=v2.0.0",
						Type:        ptr.To(v1beta1.ProviderPackageType),
					},
				},
			},
		},
		"InvalidDependency": {
			reason: "Invalid dependency with no package reference should be skipped",
			args: args{
				deps: []pkgmetav1.Dependency{
					{
						Version: ">=v1.0.0",
					},
					{
						Provider: ptr.To("xpkg.crossplane.io/provider-aws"),
						Version:  ">=v1.0.0",
					},
				},
			},
			want: want{
				lockDeps: []v1beta1.Dependency{
					{
						Package:     "xpkg.crossplane.io/provider-aws",
						Constraints: ">=v1.0.0",
						Type:        ptr.To(v1beta1.ProviderPackageType),
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := ConvertDependencies(tc.args.deps)

			if diff := cmp.Diff(tc.want.lockDeps, got); diff != "" {
				t.Errorf("\n%s\nConvertDependencies(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestFindMinimumCompatibleVersion(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		source      string
		constraints []string
	}
	type want struct {
		version *ResolvedVersion
		err     error
	}

	cases := map[string]struct {
		reason string
		client *MockClient
		args   args
		want   want
	}{
		"NoVersionsSatisfyConstraints": {
			reason: "Should return error when no versions satisfy constraints",
			client: &MockClient{
				MockListVersions: func(_ context.Context, _ string, _ ...xpkg.GetOption) ([]string, error) {
					return []string{"v1.0.0", "v1.1.0"}, nil
				},
			},
			args: args{
				source:      "xpkg.crossplane.io/provider-aws",
				constraints: []string{">=v2.0.0"},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"MinimumVersionSelected": {
			reason: "Should select minimum version satisfying all constraints",
			client: &MockClient{
				MockListVersions: func(_ context.Context, _ string, _ ...xpkg.GetOption) ([]string, error) {
					return []string{"v1.0.0", "v1.5.0", "v2.0.0", "v2.5.0"}, nil
				},
				MockGet: func(_ context.Context, _ string, _ ...xpkg.GetOption) (*xpkg.Package, error) {
					return &xpkg.Package{
						Digest: "sha256:abc123",
					}, nil
				},
			},
			args: args{
				source:      "xpkg.crossplane.io/provider-aws",
				constraints: []string{">=v1.5.0", "<v3.0.0"},
			},
			want: want{
				version: &ResolvedVersion{
					Tag:    "v1.5.0",
					Digest: "sha256:abc123",
				},
			},
		},
		"MultipleConstraintsANDed": {
			reason: "Multiple constraints should be ANDed together",
			client: &MockClient{
				MockListVersions: func(_ context.Context, _ string, _ ...xpkg.GetOption) ([]string, error) {
					return []string{"v1.0.0", "v1.5.0", "v2.0.0", "v2.5.0"}, nil
				},
				MockGet: func(_ context.Context, _ string, _ ...xpkg.GetOption) (*xpkg.Package, error) {
					return &xpkg.Package{
						Digest: "sha256:def456",
					}, nil
				},
			},
			args: args{
				source:      "xpkg.crossplane.io/provider-aws",
				constraints: []string{">=v1.0.0", ">=v1.5.0", "<v2.5.0"},
			},
			want: want{
				version: &ResolvedVersion{
					Tag:    "v1.5.0",
					Digest: "sha256:def456",
				},
			},
		},
		"WildcardConstraint": {
			reason: "Empty constraints should match any version",
			client: &MockClient{
				MockListVersions: func(_ context.Context, _ string, _ ...xpkg.GetOption) ([]string, error) {
					return []string{"v1.0.0", "v2.0.0", "v3.0.0"}, nil
				},
				MockGet: func(_ context.Context, _ string, _ ...xpkg.GetOption) (*xpkg.Package, error) {
					return &xpkg.Package{
						Digest: "sha256:wildcard",
					}, nil
				},
			},
			args: args{
				source:      "xpkg.crossplane.io/provider-aws",
				constraints: []string{},
			},
			want: want{
				version: &ResolvedVersion{
					Tag:    "v1.0.0",
					Digest: "sha256:wildcard",
				},
			},
		},
		"ListVersionsError": {
			reason: "Should return error when listing versions fails",
			client: &MockClient{
				MockListVersions: func(_ context.Context, _ string, _ ...xpkg.GetOption) ([]string, error) {
					return nil, errBoom
				},
			},
			args: args{
				source:      "xpkg.crossplane.io/provider-aws",
				constraints: []string{">=v1.0.0"},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"GetError": {
			reason: "Should return error when fetching package fails",
			client: &MockClient{
				MockListVersions: func(_ context.Context, _ string, _ ...xpkg.GetOption) ([]string, error) {
					return []string{"v1.0.0"}, nil
				},
				MockGet: func(_ context.Context, _ string, _ ...xpkg.GetOption) (*xpkg.Package, error) {
					return nil, errBoom
				},
			},
			args: args{
				source:      "xpkg.crossplane.io/provider-aws",
				constraints: []string{">=v1.0.0"},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := NewTwoPassSolver(tc.client)
			got, err := s.FindMinimumCompatibleVersion(context.Background(), tc.args.source, tc.args.constraints)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nFindMinimumCompatibleVersion(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.version, got); diff != "" {
				t.Errorf("\n%s\nFindMinimumCompatibleVersion(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestBuildGraph(t *testing.T) {
	errBoom := errors.New("boom")

	// Helper to parse package JSON into a Package
	parsePackage := func(t *testing.T, jsonStr string) *xpkg.Package {
		t.Helper()
		meta, _ := xpkg.BuildMetaScheme()
		obj, _ := xpkg.BuildObjectScheme()
		p := parser.New(meta, obj)
		pkg, err := p.Parse(context.Background(), io.NopCloser(strings.NewReader(jsonStr)))
		if err != nil {
			t.Fatalf("failed to parse package: %v", err)
		}
		return &xpkg.Package{Package: pkg}
	}

	type args struct {
		source  string
		current []v1beta1.LockPackage
	}
	type want struct {
		graph Graph
		err   error
	}

	cases := map[string]struct {
		reason string
		client *MockClient
		args   args
		want   want
	}{
		"SimplePackageNoDeps": {
			reason: "Package with no dependencies should be added to graph",
			client: &MockClient{
				MockGet: func(_ context.Context, _ string, _ ...xpkg.GetOption) (*xpkg.Package, error) {
					return parsePackage(t, providerNoDeps), nil
				},
			},
			args: args{
				source:  "xpkg.crossplane.io/provider-aws",
				current: []v1beta1.LockPackage{},
			},
			want: want{
				graph: Graph{
					"xpkg.crossplane.io/provider-aws": {},
				},
			},
		},
		"PackageWithDependency": {
			reason: "Package dependencies should be collected recursively",
			client: &MockClient{
				MockGet: func(_ context.Context, ref string, _ ...xpkg.GetOption) (*xpkg.Package, error) {
					if strings.Contains(ref, "provider-aws-s3") {
						return parsePackage(t, providerWithDep), nil
					}
					return parsePackage(t, providerNoDeps), nil
				},
			},
			args: args{
				source:  "xpkg.crossplane.io/provider-aws-s3",
				current: []v1beta1.LockPackage{},
			},
			want: want{
				graph: Graph{
					"xpkg.crossplane.io/provider-aws-s3": {
						{
							Package:     "xpkg.crossplane.io/provider-family-aws",
							Constraints: ">=v1.0.0",
							Type:        ptr.To(v1beta1.ProviderPackageType),
						},
					},
					"xpkg.crossplane.io/provider-family-aws": {},
				},
			},
		},
		"CircularDependency": {
			reason: "Circular dependencies should be detected and return error",
			client: &MockClient{
				MockGet: func(_ context.Context, _ string, _ ...xpkg.GetOption) (*xpkg.Package, error) {
					return parsePackage(t, providerCircular), nil
				},
			},
			args: args{
				source:  "xpkg.crossplane.io/provider-aws",
				current: []v1beta1.LockPackage{},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
		"FetchError": {
			reason: "Fetch errors should be propagated",
			client: &MockClient{
				MockGet: func(_ context.Context, _ string, _ ...xpkg.GetOption) (*xpkg.Package, error) {
					return nil, errBoom
				},
			},
			args: args{
				source:  "xpkg.crossplane.io/provider-aws",
				current: []v1beta1.LockPackage{},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := NewTwoPassSolver(tc.client)
			got, err := s.BuildGraph(context.Background(), tc.args.source, tc.args.current)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nBuildGraph(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.graph, got); diff != "" {
				t.Errorf("\n%s\nBuildGraph(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestSelectVersions(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		graph   Graph
		current []v1beta1.LockPackage
	}
	type want struct {
		packages []v1beta1.LockPackage
		err      error
	}

	cases := map[string]struct {
		reason string
		client *MockClient
		args   args
		want   want
	}{
		"PreserveUnchangedPackages": {
			reason: "Packages not in constraints should be preserved from current lock",
			client: &MockClient{
				MockListVersions: func(_ context.Context, _ string, _ ...xpkg.GetOption) ([]string, error) {
					return []string{"v1.0.0"}, nil
				},
				MockGet: func(_ context.Context, _ string, _ ...xpkg.GetOption) (*xpkg.Package, error) {
					return &xpkg.Package{
						Digest: "sha256:new123",
					}, nil
				},
			},
			args: args{
				graph: Graph{
					"xpkg.crossplane.io/provider-aws": {},
					"xpkg.crossplane.io/some-package": {
						{
							Package:     "xpkg.crossplane.io/provider-aws",
							Constraints: ">=v1.0.0",
						},
					},
				},
				current: []v1beta1.LockPackage{
					{
						Source:  "xpkg.crossplane.io/provider-gcp",
						Version: "sha256:old123",
					},
				},
			},
			want: want{
				packages: []v1beta1.LockPackage{
					{
						Source:  "xpkg.crossplane.io/provider-gcp",
						Version: "sha256:old123",
					},
					{
						Source:       "xpkg.crossplane.io/provider-aws",
						Version:      "sha256:new123",
						Dependencies: []v1beta1.Dependency{},
					},
				},
			},
		},
		"ResolveNewPackages": {
			reason: "New packages should be resolved with MVS",
			client: &MockClient{
				MockListVersions: func(_ context.Context, _ string, _ ...xpkg.GetOption) ([]string, error) {
					return []string{"v1.0.0", "v1.5.0", "v2.0.0"}, nil
				},
				MockGet: func(_ context.Context, _ string, _ ...xpkg.GetOption) (*xpkg.Package, error) {
					return &xpkg.Package{
						Digest: "sha256:abc123",
					}, nil
				},
			},
			args: args{
				graph: Graph{
					"xpkg.crossplane.io/provider-aws": {},
					"xpkg.crossplane.io/some-package": {
						{
							Package:     "xpkg.crossplane.io/provider-aws",
							Constraints: ">=v1.5.0, <v2.0.0",
						},
					},
				},
				current: []v1beta1.LockPackage{},
			},
			want: want{
				packages: []v1beta1.LockPackage{
					{
						Source:       "xpkg.crossplane.io/provider-aws",
						Version:      "sha256:abc123",
						Dependencies: []v1beta1.Dependency{},
					},
				},
			},
		},
		"ErrorFindingVersion": {
			reason: "Should return error if version cannot be found",
			client: &MockClient{
				MockListVersions: func(_ context.Context, _ string, _ ...xpkg.GetOption) ([]string, error) {
					return nil, errBoom
				},
			},
			args: args{
				graph: Graph{
					"xpkg.crossplane.io/provider-aws": {},
					"xpkg.crossplane.io/some-package": {
						{
							Package:     "xpkg.crossplane.io/provider-aws",
							Constraints: ">=v1.0.0",
						},
					},
				},
				current: []v1beta1.LockPackage{},
			},
			want: want{
				err: cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := NewTwoPassSolver(tc.client)
			got, err := s.SelectVersions(context.Background(), tc.args.graph, tc.args.current)

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nSelectVersions(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.packages, got, cmpopts.SortSlices(func(a, b v1beta1.LockPackage) bool {
				return a.Source < b.Source
			}), cmpopts.IgnoreFields(v1beta1.LockPackage{}, "Name")); diff != "" {
				t.Errorf("\n%s\nSelectVersions(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
