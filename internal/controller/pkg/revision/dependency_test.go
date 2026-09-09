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

package revision

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	pkgmetav1 "github.com/crossplane/crossplane/apis/v2/pkg/meta/v1"
	v1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
	"github.com/crossplane/crossplane/apis/v2/pkg/v1beta1"
	"github.com/crossplane/crossplane/v2/internal/dag"
	dagfake "github.com/crossplane/crossplane/v2/internal/dag/fake"
)

var _ DependencyManager = &PackageDependencyManager{}

func TestResolve(t *testing.T) {
	errBoom := errors.New("boom")
	mockUpdateCallCount := 0

	type args struct {
		dep  *PackageDependencyManager
		meta pkgmetav1.Pkg
		pr   v1.PackageRevision
	}

	type want struct {
		err       error
		total     int
		installed int
		invalid   int
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SuccessfulInactiveNothingToDo": {
			reason: "Should return no error if resolve is called for an inactive revision.",
			args: args{
				meta: &pkgmetav1.Configuration{},
				pr: &v1.ConfigurationRevision{
					Spec: v1.PackageRevisionSpec{
						Package:      "xpkg.crossplane.io/hasheddan/config-nop-a:v0.0.1",
						DesiredState: v1.PackageRevisionInactive,
					},
				},
			},
			want: want{},
		},
		"ErrGetLock": {
			reason: "Should return error if we cannot get lock.",
			args: args{
				dep: &PackageDependencyManager{
					client: &test.MockClient{
						MockGet: test.NewMockGetFn(errBoom),
					},
					log: logging.NewNopLogger(),
				},
				meta: &pkgmetav1.Configuration{},
				pr: &v1.ConfigurationRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionActive,
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetOrCreateLock),
			},
		},
		"ErrCreateLock": {
			reason: "Should return error if we cannot get or create lock.",
			args: args{
				dep: &PackageDependencyManager{
					client: &test.MockClient{
						MockGet:    test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
						MockCreate: test.NewMockCreateFn(errBoom),
					},
					log: logging.NewNopLogger(),
				},
				meta: &pkgmetav1.Configuration{},
				pr: &v1.ConfigurationRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionActive,
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetOrCreateLock),
			},
		},
		"ErrBuildDag": {
			reason: "Should return error if we cannot build DAG.",
			args: args{
				dep: &PackageDependencyManager{
					client: &test.MockClient{
						MockGet:    test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
						MockCreate: test.NewMockCreateFn(nil),
					},
					newDag: func() dag.DAG {
						return &dagfake.MockDag{
							MockInit: func(_ []dag.Node) ([]dag.Node, error) {
								return nil, errBoom
							},
						}
					},
					log: logging.NewNopLogger(),
				},
				meta: &pkgmetav1.Configuration{},
				pr: &v1.ConfigurationRevision{
					Spec: v1.PackageRevisionSpec{
						Package: "xpkg.crossplane.io/hasheddan/config-nop-a:v0.0.1",
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errInitDAG),
			},
		},
		"SuccessfulSelfExistNoDependencies": {
			reason: "Should not return error if self exists and has no dependencies.",
			args: args{
				dep: &PackageDependencyManager{
					client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							l := obj.(*v1beta1.Lock)
							l.Packages = []v1beta1.LockPackage{
								{
									Name:    "config-nop-a-abc123",
									Source:  "xpkg.crossplane.io/hasheddan/config-nop-a",
									Version: "v0.0.1",
								},
							}
							return nil
						}),
					},
					newDag: dag.NewMapDag,
					log:    logging.NewNopLogger(),
				},
				meta: &pkgmetav1.Configuration{},
				pr: &v1.ConfigurationRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: "config-nop-a-abc123",
					},
					Spec: v1.PackageRevisionSpec{
						Package:      "xpkg.crossplane.io/hasheddan/config-nop-a:v0.0.1",
						DesiredState: v1.PackageRevisionActive,
					},
				},
			},
			want: want{},
		},
		"SuccessfulSelfExistWrongVersion": {
			reason: "Should update the lock if the revision is in it with the wrong version.",
			args: args{
				dep: &PackageDependencyManager{
					client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							l := obj.(*v1beta1.Lock)
							l.Packages = []v1beta1.LockPackage{
								{
									Name:    "config-nop-a-abc123",
									Source:  "xpkg.crossplane.io/hasheddan/config-nop-a",
									Version: "v0.0.1",
								},
							}
							return nil
						}),
						MockUpdate: test.NewMockUpdateFn(nil, func(obj client.Object) error {
							l := obj.(*v1beta1.Lock)
							p := l.Packages[0]
							if p.Version != "v0.0.2" {
								return errors.Errorf("lock package updated to incorrect version %q", p.Version)
							}

							return nil
						}),
					},
					newDag: dag.NewMapDag,
					log:    logging.NewNopLogger(),
				},
				meta: &pkgmetav1.Configuration{},
				pr: &v1.ConfigurationRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: "config-nop-a-abc123",
					},
					Spec: v1.PackageRevisionSpec{
						Package:      "xpkg.crossplane.io/hasheddan/config-nop-a:v0.0.2",
						DesiredState: v1.PackageRevisionActive,
					},
				},
			},
			want: want{},
		},
		"ErrorSelfNotExistMissingDirectDependencies": {
			reason: "Should return error if self does not exist and missing direct dependencies.",
			args: args{
				dep: &PackageDependencyManager{
					client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(_ client.Object) error {
							return nil
						}),
						MockUpdate: test.NewMockUpdateFn(nil),
					},
					newDag: dag.NewMapDag,
					log:    logging.NewNopLogger(),
				},
				meta: &pkgmetav1.Configuration{
					Spec: pkgmetav1.ConfigurationSpec{
						MetaSpec: pkgmetav1.MetaSpec{
							DependsOn: []pkgmetav1.Dependency{
								{
									Provider: new("not-here-1"),
								},
								{
									Provider: new("not-here-2"),
									Version:  ">= v2.0.0",
								},
							},
						},
					},
				},
				pr: &v1.ConfigurationRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: "config-nop-a-abc123",
					},
					Spec: v1.PackageRevisionSpec{
						Package:      "xpkg.crossplane.io/hasheddan/config-nop-a:v0.0.1",
						DesiredState: v1.PackageRevisionActive,
					},
				},
			},
			want: want{
				total: 2,
				err:   errors.Errorf(errFmtMissingDependencies, `"not-here-1", "not-here-2" (>= v2.0.0)`),
			},
		},
		"ErrorSelfExistMissingDependencies": {
			reason: "Should return error if self exists and missing dependencies.",
			args: args{
				dep: &PackageDependencyManager{
					client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							l := obj.(*v1beta1.Lock)
							l.Packages = []v1beta1.LockPackage{
								{
									Name:    "config-nop-a-abc123",
									Source:  "xpkg.crossplane.io/hasheddan/config-nop-a",
									Version: "v0.0.1",
									Dependencies: []v1beta1.Dependency{
										{
											Package: "not-here-1",
											Type:    ptr.To(v1beta1.ProviderPackageType),
										},
										{
											Package: "not-here-2",
											Type:    ptr.To(v1beta1.ConfigurationPackageType),
										},
									},
								},
								{
									Source: "not-here-1",
									Dependencies: []v1beta1.Dependency{
										{
											Package: "not-here-3",
											Type:    ptr.To(v1beta1.ProviderPackageType),
										},
									},
								},
							}
							return nil
						}),
						MockUpdate: test.NewMockUpdateFn(nil),
					},
					newDag: dag.NewMapDag,
					log:    logging.NewNopLogger(),
				},
				meta: &pkgmetav1.Configuration{
					Spec: pkgmetav1.ConfigurationSpec{
						MetaSpec: pkgmetav1.MetaSpec{
							DependsOn: []pkgmetav1.Dependency{
								{
									Provider: new("not-here-1"),
								},
								{
									Provider: new("not-here-2"),
								},
							},
						},
					},
				},
				pr: &v1.ConfigurationRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: "config-nop-a-abc123",
					},
					Spec: v1.PackageRevisionSpec{
						Package:      "xpkg.crossplane.io/hasheddan/config-nop-a:v0.0.1",
						DesiredState: v1.PackageRevisionActive,
					},
				},
			},
			want: want{
				total:     3,
				installed: 1,
				err:       errors.Errorf(errFmtMissingDependencies, `"not-here-2", "not-here-3"`),
			},
		},
		"ErrorSelfExistInvalidDependencies": {
			reason: "Should return error if self exists and dependencies have incompatible versions.",
			args: args{
				dep: &PackageDependencyManager{
					client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							l := obj.(*v1beta1.Lock)
							l.Packages = []v1beta1.LockPackage{
								{
									Name:    "config-nop-a-abc123",
									Source:  "xpkg.crossplane.io/hasheddan/config-nop-a",
									Version: "v0.0.1",
									Dependencies: []v1beta1.Dependency{
										{
											Package: "not-here-1",
											Type:    ptr.To(v1beta1.ProviderPackageType),
										},
										{
											Package: "not-here-2",
											Type:    ptr.To(v1beta1.ConfigurationPackageType),
										},
									},
								},
								{
									Source:  "not-here-1",
									Version: "v0.0.1",
									Dependencies: []v1beta1.Dependency{
										{
											Package: "not-here-3",
											Type:    ptr.To(v1beta1.ProviderPackageType),
										},
									},
								},
								{
									Source:  "not-here-2",
									Version: "v0.0.1",
								},
								{
									Source:  "not-here-3",
									Version: "v0.0.1",
								},
							}
							return nil
						}),
						MockUpdate: test.NewMockUpdateFn(nil),
					},
					newDag: dag.NewMapDag,
					log:    logging.NewNopLogger(),
				},
				meta: &pkgmetav1.Configuration{
					Spec: pkgmetav1.ConfigurationSpec{
						MetaSpec: pkgmetav1.MetaSpec{
							DependsOn: []pkgmetav1.Dependency{
								{
									Provider: new("not-here-1"),
									Version:  ">=v0.1.0",
								},
								{
									Provider: new("not-here-2"),
									Version:  ">=v0.1.0",
								},
							},
						},
					},
				},
				pr: &v1.ConfigurationRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: "config-nop-a-abc123",
					},
					Spec: v1.PackageRevisionSpec{
						Package:      "xpkg.crossplane.io/hasheddan/config-nop-a:v0.0.1",
						DesiredState: v1.PackageRevisionActive,
					},
				},
			},
			want: want{
				total:     3,
				installed: 3,
				invalid:   2,
				err:       errors.Errorf(errFmtIncompatibleDependency, "existing package not-here-1@v0.0.1 is incompatible with constraint >=v0.1.0; existing package not-here-2@v0.0.1 is incompatible with constraint >=v0.1.0"),
			},
		},
		"SuccessfulSelfExistValidDependencies": {
			reason: "Should not return error if self exists, all dependencies exist and are valid.",
			args: args{
				dep: &PackageDependencyManager{
					client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							l := obj.(*v1beta1.Lock)
							l.Packages = []v1beta1.LockPackage{
								{
									Name:    "config-nop-a-abc123",
									Source:  "xpkg.crossplane.io/hasheddan/config-nop-a",
									Version: "v0.0.1",
									Dependencies: []v1beta1.Dependency{
										{
											Package: "not-here-1",
											Type:    ptr.To(v1beta1.ProviderPackageType),
										},
										{
											Package: "not-here-2",
											Type:    ptr.To(v1beta1.ConfigurationPackageType),
										},
										{
											Package: "function-not-here-1",
											Type:    ptr.To(v1beta1.FunctionPackageType),
										},
									},
								},
								{
									Source:  "not-here-1",
									Version: "v0.20.0",
									Dependencies: []v1beta1.Dependency{
										{
											Package: "not-here-3",
											Type:    ptr.To(v1beta1.ProviderPackageType),
										},
									},
								},
								{
									Source:  "not-here-2",
									Version: "v0.100.1",
								},
								{
									Source:  "not-here-3",
									Version: "v0.20.0",
								},
								{
									Source:  "function-not-here-1",
									Version: "v0.1.0",
								},
							}
							return nil
						}),
						MockUpdate: test.NewMockUpdateFn(nil),
					},
					newDag: dag.NewMapDag,
					log:    logging.NewNopLogger(),
				},
				meta: &pkgmetav1.Configuration{
					Spec: pkgmetav1.ConfigurationSpec{
						MetaSpec: pkgmetav1.MetaSpec{
							DependsOn: []pkgmetav1.Dependency{
								{
									Provider: new("not-here-1"),
									Version:  ">=v0.1.0",
								},
								{
									Provider: new("not-here-2"),
									Version:  ">=v0.1.0",
								},
								{
									Function: new("function-not-here-1"),
									Version:  ">=v0.1.0",
								},
							},
						},
					},
				},
				pr: &v1.ConfigurationRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: "config-nop-a-abc123",
					},
					Spec: v1.PackageRevisionSpec{
						Package:      "xpkg.crossplane.io/hasheddan/config-nop-a:v0.0.1",
						DesiredState: v1.PackageRevisionActive,
					},
				},
			},
			want: want{
				total:     4,
				installed: 4,
				invalid:   0,
			},
		},
		"SuccessfulDigestWithResolvedVersion": {
			reason: "Should resolve dependencies when a package is installed with a tag@digest reference and ResolvedVersion is set.",
			args: args{
				dep: &PackageDependencyManager{
					client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							l := obj.(*v1beta1.Lock)
							l.Packages = []v1beta1.LockPackage{
								{
									Name:            "provider-family-azure-abc123",
									Source:          "xpkg.upbound.io/upbound/provider-family-azure",
									Version:         "sha256:b6f5cbc791b131a76b8e6b031333dae62db05266d1b12988bb12ff14226215d5",
									ResolvedVersion: "v2.5.6",
								},
								{
									Name:            "provider-azure-storage-def456",
									Source:          "xpkg.upbound.io/upbound/provider-azure-storage",
									Version:         "sha256:2e10e0d89075cfdf3a1fe097f26f1941229e94593d2cf5f3886d8d46e5fb6a49",
									ResolvedVersion: "v2.5.6",
									Dependencies: []v1beta1.Dependency{
										{
											Package:     "xpkg.upbound.io/upbound/provider-family-azure",
											Constraints: "v2.5.6",
										},
									},
								},
							}
							return nil
						}),
						MockUpdate: test.NewMockUpdateFn(nil),
					},
					newDag: dag.NewMapDag,
					log:    logging.NewNopLogger(),
				},
				meta: &pkgmetav1.Provider{
					Spec: pkgmetav1.ProviderSpec{
						MetaSpec: pkgmetav1.MetaSpec{
							DependsOn: []pkgmetav1.Dependency{
								{
									Provider: ptr.To("xpkg.upbound.io/upbound/provider-family-azure"),
									Version:  "v2.5.6",
								},
							},
						},
					},
				},
				pr: &v1.ProviderRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: "provider-azure-storage-def456",
					},
					Spec: v1.ProviderRevisionSpec{
						PackageRevisionSpec: v1.PackageRevisionSpec{
							Package:      "xpkg.upbound.io/upbound/provider-azure-storage:v2.5.6@sha256:2e10e0d89075cfdf3a1fe097f26f1941229e94593d2cf5f3886d8d46e5fb6a49",
							DesiredState: v1.PackageRevisionActive,
						},
						PackageRevisionRuntimeSpec: v1.PackageRevisionRuntimeSpec{},
					},
				},
			},
			want: want{
				total:     1,
				installed: 1,
				invalid:   0,
			},
		},
		"SuccessfulLockPackageSourceMismatch": {
			reason: "Should not return error if source in packages does not match provider revision package.",
			args: args{
				dep: &PackageDependencyManager{
					client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							l := obj.(*v1beta1.Lock)
							if mockUpdateCallCount < 1 {
								l.Packages = []v1beta1.LockPackage{
									{
										Name: "config-nop-a-abc123",
										// Source mistmatch provider revision package
										Source: "xpkg.crossplane.io/hasheddan/config-nop-b",
									},
								}
							} else {
								l.Packages = []v1beta1.LockPackage{}
							}
							return nil
						}),
						MockUpdate: func(_ context.Context, _ client.Object, _ ...client.UpdateOption) error {
							mockUpdateCallCount++
							return nil
						},
					},
					newDag: dag.NewMapDag,
					log:    logging.NewNopLogger(),
				},
				meta: &pkgmetav1.Configuration{},
				pr: &v1.ConfigurationRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: "config-nop-a-abc123",
					},
					Spec: v1.PackageRevisionSpec{
						Package:      "xpkg.crossplane.io/hasheddan/config-nop-a:v0.0.1",
						DesiredState: v1.PackageRevisionActive,
					},
				},
			},
			want: want{},
		},
	}

	for name, tc := range cases {
		mockUpdateCallCount = 0

		t.Run(name, func(t *testing.T) {
			total, installed, invalid, err := tc.args.dep.Resolve(context.TODO(), tc.args.meta, tc.args.pr)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\np.Resolve(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.total, total); diff != "" {
				t.Errorf("\n%s\nTotal(...): -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.installed, installed); diff != "" {
				t.Errorf("\n%s\nInstalled(...): -want, +got:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.invalid, invalid); diff != "" {
				t.Errorf("\n%s\nInvalid(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestParseRef(t *testing.T) {
	const digest = "sha256:b6f5cbc791b131a76b8e6b031333dae62db05266d1b12988bb12ff14226215d5"

	cases := map[string]struct {
		source  string
		tag     string
		digest  string
		wantErr bool
	}{
		"Tag":                 {source: "registry.example.com/ns/pkg:v1.2.3", tag: "v1.2.3"},
		"TagAndDigest":        {source: "registry.example.com/ns/pkg:v1.2.3@" + digest, tag: "v1.2.3", digest: digest},
		"DigestOnly":          {source: "registry.example.com/ns/pkg@" + digest, digest: digest},
		"PortAndTagAndDigest": {source: "registry.example.com:5000/ns/pkg:v1.2.3@" + digest, tag: "v1.2.3", digest: digest},
		"PortAndDigestOnly":   {source: "registry.example.com:5000/ns/pkg@" + digest, digest: digest},
		"PortAndTag":          {source: "registry.example.com:5000/ns/pkg:v1.2.3", tag: "v1.2.3"},
		"MissingTag":          {source: "registry.example.com/ns/pkg", wantErr: true},
		"InvalidDigest":       {source: "registry.example.com/ns/pkg:v1.2.3@sha256:invalid", wantErr: true},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tag, digest, err := parseRef(tc.source)
			if (err != nil) != tc.wantErr {
				t.Fatalf("parseRef(%q) error = %v, want error = %t", tc.source, err, tc.wantErr)
			}
			if tag != tc.tag || digest != tc.digest {
				t.Errorf("parseRef(%q) = (%q, %q), want (%q, %q)", tc.source, tag, digest, tc.tag, tc.digest)
			}
		})
	}
}

func TestResolveUpdatesResolvedVersion(t *testing.T) {
	const source = "registry.example.com:5000/ns/pkg"
	const digest = "sha256:b6f5cbc791b131a76b8e6b031333dae62db05266d1b12988bb12ff14226215d5"

	cases := map[string]struct {
		source         string
		previous       string
		previousSource string
		want           string
	}{
		"NormalizeLegacySource":   {source: source + ":v1.2.3@" + digest, previousSource: source + ":v1.2.3", previous: "v1.2.3", want: "v1.2.3"},
		"BackfillExistingDigest":  {source: source + ":v1.2.3@" + digest, want: "v1.2.3"},
		"UpdateTagWithSameDigest": {source: source + ":v1.2.4@" + digest, previous: "v1.2.3", want: "v1.2.4"},
		"RemoveTagWithSameDigest": {source: source + "@" + digest, previous: "v1.2.3"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			updates := 0
			previousSource := tc.previousSource
			if previousSource == "" {
				previousSource = source
			}
			m := &PackageDependencyManager{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						obj.(*v1beta1.Lock).Packages = []v1beta1.LockPackage{{
							Name:            "pkg-revision",
							Source:          previousSource,
							Type:            new(v1beta1.ConfigurationPackageType),
							Version:         digest,
							ResolvedVersion: tc.previous,
						}}
						return nil
					}),
					MockUpdate: test.NewMockUpdateFn(nil, func(obj client.Object) error {
						updates++
						packages := obj.(*v1beta1.Lock).Packages
						if len(packages) != 1 {
							t.Fatalf("lock has %d packages, want 1", len(packages))
						}
						lp := packages[0]
						if lp.Source != source {
							t.Errorf("lock source = %q, want %q", lp.Source, source)
						}
						if lp.Version != digest || lp.ResolvedVersion != tc.want {
							t.Errorf("updated lock = (%q, %q), want (%q, %q)", lp.Version, lp.ResolvedVersion, digest, tc.want)
						}
						return nil
					}),
				},
				newDag: dag.NewMapDag,
				log:    logging.NewNopLogger(),
			}
			pr := &v1.ConfigurationRevision{
				ObjectMeta: metav1.ObjectMeta{Name: "pkg-revision"},
				Spec:       v1.PackageRevisionSpec{Package: tc.source, DesiredState: v1.PackageRevisionActive},
			}
			if _, _, _, err := m.Resolve(context.Background(), &pkgmetav1.Configuration{}, pr); err != nil {
				t.Fatal(err)
			}
			if updates != 1 {
				t.Errorf("lock updates = %d, want 1", updates)
			}
		})
	}
}
