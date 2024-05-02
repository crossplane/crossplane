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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/internal/dag"
	dagfake "github.com/crossplane/crossplane/internal/dag/fake"
)

var _ DependencyManager = &PackageDependencyManager{}

func TestResolve(t *testing.T) {
	errBoom := errors.New("boom")

	type args struct {
		dep  *PackageDependencyManager
		meta runtime.Object
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
						Package:      "hasheddan/config-nop-a:v0.0.1",
						DesiredState: v1.PackageRevisionInactive,
					},
				},
			},
			want: want{},
		},
		"ErrNotMeta": {
			reason: "Should return error if not a valid package meta type.",
			args: args{
				dep:  &PackageDependencyManager{},
				meta: &v1.Configuration{},
				pr: &v1.ConfigurationRevision{
					Spec: v1.PackageRevisionSpec{
						DesiredState: v1.PackageRevisionActive,
					},
				},
			},
			want: want{
				err: errors.New(errNotMeta),
			},
		},
		"ErrGetLock": {
			reason: "Should return error if we cannot get lock.",
			args: args{
				dep: &PackageDependencyManager{
					client: &test.MockClient{
						MockGet: test.NewMockGetFn(errBoom),
					},
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
				},
				meta: &pkgmetav1.Configuration{},
				pr: &v1.ConfigurationRevision{
					Spec: v1.PackageRevisionSpec{
						Package: "hasheddan/config-nop-a:v0.0.1",
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
									Name:   "config-nop-a-abc123",
									Source: "hasheddan/config-nop-a",
								},
							}
							return nil
						}),
					},
					newDag: func() dag.DAG {
						return &dagfake.MockDag{
							MockInit: func(_ []dag.Node) ([]dag.Node, error) {
								return nil, nil
							},
							MockTraceNode: func(_ string) (map[string]dag.Node, error) {
								return nil, nil
							},
						}
					},
				},
				meta: &pkgmetav1.Configuration{},
				pr: &v1.ConfigurationRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: "config-nop-a-abc123",
					},
					Spec: v1.PackageRevisionSpec{
						Package:      "hasheddan/config-nop-a:v0.0.1",
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
					newDag: func() dag.DAG {
						return &dagfake.MockDag{
							MockInit: func(_ []dag.Node) ([]dag.Node, error) {
								return nil, nil
							},
							MockNodeExists: func(_ string) bool {
								return false
							},
							MockAddNode: func(_ dag.Node) error {
								return nil
							},
							MockAddOrUpdateNodes: func(_ ...dag.Node) {},
						}
					},
				},
				meta: &pkgmetav1.Configuration{
					Spec: pkgmetav1.ConfigurationSpec{
						MetaSpec: pkgmetav1.MetaSpec{
							DependsOn: []pkgmetav1.Dependency{
								{
									Provider: ptr.To("not-here-1"),
								},
								{
									Provider: ptr.To("not-here-2"),
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
						Package:      "hasheddan/config-nop-a:v0.0.1",
						DesiredState: v1.PackageRevisionActive,
					},
				},
			},
			want: want{
				total: 2,
				err:   errors.Errorf(errFmtMissingDependencies, []string{"not-here-1", "not-here-2"}),
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
									Name:   "config-nop-a-abc123",
									Source: "hasheddan/config-nop-a",
									Dependencies: []v1beta1.Dependency{
										{
											Package: "not-here-1",
											Type:    v1beta1.ProviderPackageType,
										},
										{
											Package: "not-here-2",
											Type:    v1beta1.ConfigurationPackageType,
										},
									},
								},
								{
									Source: "not-here-1",
									Dependencies: []v1beta1.Dependency{
										{
											Package: "not-here-3",
											Type:    v1beta1.ProviderPackageType,
										},
									},
								},
							}
							return nil
						}),
						MockUpdate: test.NewMockUpdateFn(nil),
					},
					newDag: func() dag.DAG {
						return &dagfake.MockDag{
							MockInit: func(_ []dag.Node) ([]dag.Node, error) {
								return []dag.Node{
									&v1beta1.Dependency{
										Package: "not-here-2",
									},
									&v1beta1.Dependency{
										Package: "not-here-3",
									},
								}, nil
							},
							MockTraceNode: func(_ string) (map[string]dag.Node, error) {
								return map[string]dag.Node{
									"not-here-1": &v1beta1.Dependency{},
									"not-here-2": &v1beta1.Dependency{},
									"not-here-3": &v1beta1.Dependency{},
								}, nil
							},
						}
					},
				},
				meta: &pkgmetav1.Configuration{
					Spec: pkgmetav1.ConfigurationSpec{
						MetaSpec: pkgmetav1.MetaSpec{
							DependsOn: []pkgmetav1.Dependency{
								{
									Provider: ptr.To("not-here-1"),
								},
								{
									Provider: ptr.To("not-here-2"),
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
						Package:      "hasheddan/config-nop-a:v0.0.1",
						DesiredState: v1.PackageRevisionActive,
					},
				},
			},
			want: want{
				total:     3,
				installed: 1,
				err:       errors.Errorf(errFmtMissingDependencies, []string{"not-here-2", "not-here-3"}),
			},
		},
		"ErrorSelfExistInvalidDependencies": {
			reason: "Should return error if self exists and missing dependencies.",
			args: args{
				dep: &PackageDependencyManager{
					client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							l := obj.(*v1beta1.Lock)
							l.Packages = []v1beta1.LockPackage{
								{
									Name:   "config-nop-a-abc123",
									Source: "hasheddan/config-nop-a",
									Dependencies: []v1beta1.Dependency{
										{
											Package: "not-here-1",
											Type:    v1beta1.ProviderPackageType,
										},
										{
											Package: "not-here-2",
											Type:    v1beta1.ConfigurationPackageType,
										},
									},
								},
								{
									Source: "not-here-1",
									Dependencies: []v1beta1.Dependency{
										{
											Package: "not-here-3",
											Type:    v1beta1.ProviderPackageType,
										},
									},
								},
							}
							return nil
						}),
						MockUpdate: test.NewMockUpdateFn(nil),
					},
					newDag: func() dag.DAG {
						return &dagfake.MockDag{
							MockInit: func(_ []dag.Node) ([]dag.Node, error) {
								return nil, nil
							},
							MockTraceNode: func(_ string) (map[string]dag.Node, error) {
								return map[string]dag.Node{
									"not-here-1": &v1beta1.Dependency{},
									"not-here-2": &v1beta1.Dependency{},
									"not-here-3": &v1beta1.Dependency{},
								}, nil
							},
							MockGetNode: func(s string) (dag.Node, error) {
								if s == "not-here-1" {
									return &v1beta1.LockPackage{
										Source:  "not-here-1",
										Version: "v0.0.1",
									}, nil
								}
								if s == "not-here-2" {
									return &v1beta1.LockPackage{
										Source:  "not-here-2",
										Version: "v0.0.1",
									}, nil
								}
								return nil, nil
							},
						}
					},
				},
				meta: &pkgmetav1.Configuration{
					Spec: pkgmetav1.ConfigurationSpec{
						MetaSpec: pkgmetav1.MetaSpec{
							DependsOn: []pkgmetav1.Dependency{
								{
									Provider: ptr.To("not-here-1"),
									Version:  ">=v0.1.0",
								},
								{
									Provider: ptr.To("not-here-2"),
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
						Package:      "hasheddan/config-nop-a:v0.0.1",
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
									Name:   "config-nop-a-abc123",
									Source: "hasheddan/config-nop-a",
									Dependencies: []v1beta1.Dependency{
										{
											Package: "not-here-1",
											Type:    v1beta1.ProviderPackageType,
										},
										{
											Package: "not-here-2",
											Type:    v1beta1.ConfigurationPackageType,
										},
										{
											Package: "function-not-here-1",
											Type:    v1beta1.FunctionPackageType,
										},
									},
								},
								{
									Source: "not-here-1",
									Dependencies: []v1beta1.Dependency{
										{
											Package: "not-here-3",
											Type:    v1beta1.ProviderPackageType,
										},
									},
								},
							}
							return nil
						}),
						MockUpdate: test.NewMockUpdateFn(nil),
					},
					newDag: func() dag.DAG {
						return &dagfake.MockDag{
							MockInit: func(_ []dag.Node) ([]dag.Node, error) {
								return nil, nil
							},
							MockNodeExists: func(_ string) bool {
								return true
							},
							MockTraceNode: func(_ string) (map[string]dag.Node, error) {
								return map[string]dag.Node{
									"not-here-1":          &v1beta1.Dependency{},
									"not-here-2":          &v1beta1.Dependency{},
									"not-here-3":          &v1beta1.Dependency{},
									"function-not-here-1": &v1beta1.Dependency{},
								}, nil
							},
							MockGetNode: func(s string) (dag.Node, error) {
								if s == "not-here-1" {
									return &v1beta1.LockPackage{
										Source:  "not-here-1",
										Version: "v0.20.0",
									}, nil
								}
								if s == "not-here-2" {
									return &v1beta1.LockPackage{
										Source:  "not-here-2",
										Version: "v0.100.1",
									}, nil
								}
								if s == "function-not-here-1" {
									return &v1beta1.LockPackage{
										Source:  "function-not-here-1",
										Version: "v0.1.0",
									}, nil
								}

								return nil, nil
							},
						}
					},
				},
				meta: &pkgmetav1.Configuration{
					Spec: pkgmetav1.ConfigurationSpec{
						MetaSpec: pkgmetav1.MetaSpec{
							DependsOn: []pkgmetav1.Dependency{
								{
									Provider: ptr.To("not-here-1"),
									Version:  ">=v0.1.0",
								},
								{
									Provider: ptr.To("not-here-2"),
									Version:  ">=v0.1.0",
								},
								{
									Function: ptr.To("function-not-here-1"),
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
						Package:      "hasheddan/config-nop-a:v0.0.1",
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
	}

	for name, tc := range cases {
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
