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

package resolver

import (
	"context"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/name"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/internal/dag"
	fakedag "github.com/crossplane/crossplane/internal/dag/fake"
	"github.com/crossplane/crossplane/internal/xpkg"
	fakexpkg "github.com/crossplane/crossplane/internal/xpkg/fake"
)

const (
	confPkgName = "hasheddan/config-nop-c"
	proPkgName  = "hasheddan/provider-nop-c"
	funcPkgName = "hasheddan/func-nop-c"
)

var (
	coolPkg = v1beta1.LockPackage{
		Name:    "cool-package",
		Type:    v1beta1.ProviderPackageType,
		Source:  "cool-repo/cool-image",
		Version: "v1.0.0",
	}

	coolPkgWithDigest = v1beta1.LockPackage{
		Name:    "cool-package",
		Type:    v1beta1.ProviderPackageType,
		Source:  "cool-repo/cool-image",
		Version: "sha256:ecc25c121431dfc7058754427f97c034ecde26d4aafa0da16d258090e0443904",
	}

	errBoom = errors.New("boom")
	testLog = logging.NewLogrLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(io.Discard)).WithName("testlog"))
)

func TestReconcile(t *testing.T) {
	type args struct {
		mgr manager.Manager
		req reconcile.Request
		rec []ReconcilerOption
	}
	type want struct {
		r   reconcile.Result
		err error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"LockNotFound": {
			reason: "We should not return and error and not requeue if lock not found.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, ""))},
				},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"ErrGetLock": {
			reason: "We should return an error if getting lock fails.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
				},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetLock),
			},
		},
		"ErrRemoveFinalizer": {
			reason: "We should return an error if we fail to remove finalizer.",
			args: args{
				mgr: &fake.Manager{
					Client: test.NewMockClient(),
				},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithFinalizer(resource.FinalizerFns{RemoveFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return errBoom
					}}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errRemoveFinalizer),
			},
		},
		"SuccessfulEmptyList": {
			reason: "We should not return error and not requeue if no packages in lock.",
			args: args{
				mgr: &fake.Manager{
					Client: test.NewMockClient(),
				},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
			},
		},
		"ErrAddFinalizer": {
			reason: "We should return an error if we fail to add finalizer.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							// Populate package list so we attempt reconciliation.
							l := o.(*v1beta1.Lock)
							l.Packages = append(l.Packages, coolPkg)
							return nil
						}),
						MockUpdate: test.NewMockUpdateFn(nil),
					},
				},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return errBoom
					}}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errAddFinalizer),
			},
		},
		"ErrInitDag": {
			reason: "We should not requeue if we fail to initialize DAG.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							// Populate package list so we attempt
							// reconciliation. This is overridden by the mock
							// DAG.
							l := o.(*v1beta1.Lock)
							l.Packages = append(l.Packages, coolPkg)
							return nil
						}),
						MockUpdate: test.NewMockUpdateFn(nil),
					},
				},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewDagFn(func() dag.DAG {
						return &fakedag.MockDag{
							MockInit: func(_ []dag.Node) ([]dag.Node, error) {
								return nil, errBoom
							},
						}
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errBuildDAG),
			},
		},
		"ErrSortDag": {
			reason: "We should return an error if we fail to sort the DAG.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							// Populate package list so we attempt
							// reconciliation. This is overridden by the mock
							// DAG.
							l := o.(*v1beta1.Lock)
							l.Packages = append(l.Packages, coolPkg)
							return nil
						}),
						MockUpdate: test.NewMockUpdateFn(nil),
					},
				},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewDagFn(func() dag.DAG {
						return &fakedag.MockDag{
							MockInit: func(_ []dag.Node) ([]dag.Node, error) {
								return nil, nil
							},
							MockSort: func() ([]string, error) {
								return nil, errBoom
							},
						}
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errSortDAG),
			},
		},
		"SuccessfulNoMissing": {
			reason: "We should not return error and not requeue if no missing dependencies.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							// Populate package list so we attempt
							// reconciliation. This is overridden by the mock
							// DAG.
							l := o.(*v1beta1.Lock)
							l.Packages = append(l.Packages, coolPkg)
							return nil
						}),
						MockUpdate: test.NewMockUpdateFn(nil),
					},
				},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewDagFn(func() dag.DAG {
						return &fakedag.MockDag{
							MockInit: func(_ []dag.Node) ([]dag.Node, error) {
								return nil, nil
							},
							MockSort: func() ([]string, error) {
								return nil, nil
							},
						}
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"SuccessfulNoMissingWithDigest": {
			reason: "We should not return error and not requeue if no missing dependencies with digest.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							// Populate package list so we attempt
							// reconciliation. This is overridden by the mock
							// DAG.
							l := o.(*v1beta1.Lock)
							l.Packages = append(l.Packages, coolPkgWithDigest)
							return nil
						}),
						MockUpdate: test.NewMockUpdateFn(nil),
					},
				},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewDagFn(func() dag.DAG {
						return &fakedag.MockDag{
							MockInit: func(_ []dag.Node) ([]dag.Node, error) {
								return nil, nil
							},
							MockSort: func() ([]string, error) {
								return nil, nil
							},
						}
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"ErrorInvalidDependency": {
			reason: "We should not requeue if dependency is invalid.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							// Populate package list so we attempt
							// reconciliation. This is overridden by the mock
							// DAG.
							l := o.(*v1beta1.Lock)
							l.Packages = append(l.Packages, coolPkg)
							return nil
						}),
						MockUpdate: test.NewMockUpdateFn(nil),
					},
				},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewDagFn(func() dag.DAG {
						return &fakedag.MockDag{
							MockInit: func(_ []dag.Node) ([]dag.Node, error) {
								return []dag.Node{
									&v1beta1.Dependency{
										Package: "not.a.valid.package",
									},
								}, nil
							},
							MockGetNode: func(identifier string) (dag.Node, error) {
								if identifier == "not.a.valid.package" {
									return &v1beta1.Dependency{
										Package: "not.a.valid.package",
									}, nil
								}

								return nil, errors.New("not found")
							},
							MockSort: func() ([]string, error) {
								return nil, nil
							},
						}
					}),
					WithVersionFinder(&DefaultVersionFinder{fetcher: &fakexpkg.MockFetcher{MockTags: fakexpkg.NewMockTagsFn(nil, errBoom)}}),
				},
			},
			want: want{
				r:   reconcile.Result{Requeue: false},
				err: errors.New(errNoValidVersion),
			},
		},
		"ErrorFetchTags": {
			reason: "We should return an error if fail to fetch tags to account for network issues.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							// Populate package list so we attempt
							// reconciliation. This is overridden by the mock
							// DAG.
							l := o.(*v1beta1.Lock)
							l.Packages = append(l.Packages, coolPkg)
							return nil
						}),
						MockUpdate: test.NewMockUpdateFn(nil),
					},
				},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewDagFn(func() dag.DAG {
						return &fakedag.MockDag{
							MockInit: func(_ []dag.Node) ([]dag.Node, error) {
								return []dag.Node{
									&v1beta1.Dependency{
										Package:     "hasheddan/config-nop-b",
										Constraints: "*",
									},
								}, nil
							},
							MockGetNode: func(identifier string) (dag.Node, error) {
								if identifier == "hasheddan/config-nop-b" {
									return &v1beta1.Dependency{
										Package:     "hasheddan/config-nop-b",
										Constraints: "*",
									}, nil
								}

								return nil, errors.New("not found")
							},
							MockSort: func() ([]string, error) {
								return nil, nil
							},
						}
					}),
					WithVersionFinder(&DefaultVersionFinder{fetcher: &fakexpkg.MockFetcher{MockTags: fakexpkg.NewMockTagsFn(nil, errBoom)}, config: &fakexpkg.MockConfigStore{
						MockPullSecretFor: fakexpkg.NewMockConfigStorePullSecretForFn("", "", nil),
					}}),
				},
			},
			want: want{
				err: errors.New(errNoValidVersion),
			},
		},
		"ErrorFetchTagsUpdatable": {
			reason: "We should return an error if fail to fetch tags to account for network issues.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							// Populate package list so we attempt
							// reconciliation. This is overridden by the mock
							// DAG.
							l := o.(*v1beta1.Lock)
							l.Packages = append(l.Packages, coolPkg)
							return nil
						}),
						MockUpdate: test.NewMockUpdateFn(nil),
					},
				},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewDagFn(func() dag.DAG {
						return &fakedag.MockDag{
							MockInit: func(_ []dag.Node) ([]dag.Node, error) {
								return []dag.Node{
									&v1beta1.Dependency{
										Package:     "hasheddan/config-nop-b",
										Constraints: "*",
									},
								}, nil
							},
							MockGetNode: func(identifier string) (dag.Node, error) {
								if identifier == "hasheddan/config-nop-b" {
									return &v1beta1.Dependency{
										Package:     "hasheddan/config-nop-b",
										Constraints: "*",
									}, nil
								}

								return nil, errors.New("not found")
							},
							MockSort: func() ([]string, error) {
								return nil, nil
							},
						}
					}),
					WithVersionFinder(&UpdatableVersionFinder{fetcher: &fakexpkg.MockFetcher{MockTags: fakexpkg.NewMockTagsFn(nil, errBoom)}, config: &fakexpkg.MockConfigStore{
						MockPullSecretFor: fakexpkg.NewMockConfigStorePullSecretForFn("", "", nil),
					}}),
				},
			},
			want: want{
				err: errors.New(errNoValidVersion),
			},
		},
		"ErrorNoValidVersion": {
			reason: "We should not requeue if valid version does not exist for dependency.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							// Populate package list so we attempt
							// reconciliation. This is overridden by the mock
							// DAG.
							l := o.(*v1beta1.Lock)
							l.Packages = append(l.Packages, coolPkg)
							return nil
						}),
						MockUpdate: test.NewMockUpdateFn(nil),
					},
				},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewDagFn(func() dag.DAG {
						return &fakedag.MockDag{
							MockInit: func(_ []dag.Node) ([]dag.Node, error) {
								return []dag.Node{
									&v1beta1.Dependency{
										Package:     "hasheddan/config-nop-b",
										Constraints: ">v1.0.0",
									},
								}, nil
							},
							MockGetNode: func(identifier string) (dag.Node, error) {
								if identifier == "hasheddan/config-nop-b" {
									return &v1beta1.Dependency{
										Package:     "hasheddan/config-nop-b",
										Constraints: ">v1.0.0",
									}, nil
								}

								return nil, errors.New("not found")
							},
							MockSort: func() ([]string, error) {
								return nil, nil
							},
						}
					}),
					WithVersionFinder(&DefaultVersionFinder{fetcher: &fakexpkg.MockFetcher{MockTags: fakexpkg.NewMockTagsFn(nil, nil)}, config: &fakexpkg.MockConfigStore{
						MockPullSecretFor: fakexpkg.NewMockConfigStorePullSecretForFn("", "", nil),
					}}),
				},
			},
			want: want{
				r:   reconcile.Result{Requeue: false},
				err: errors.New(errNoValidVersion),
			},
		},
		"ErrorCreateMissingDependency": {
			reason: "We should return an error if unable to create missing dependency.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							l, ok := o.(*v1beta1.Lock)
							if !ok {
								return kerrors.NewNotFound(schema.GroupResource{}, "")
							}

							// Populate package list so we attempt
							// reconciliation. This is overridden by the mock
							// DAG.
							l.Packages = append(l.Packages, coolPkg)
							return nil
						}),
						MockCreate: test.NewMockCreateFn(errBoom),
						MockUpdate: test.NewMockUpdateFn(nil),
					},
				},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewDagFn(func() dag.DAG {
						return &fakedag.MockDag{
							MockInit: func(_ []dag.Node) ([]dag.Node, error) {
								return []dag.Node{
									&v1beta1.Dependency{
										Package:     confPkgName,
										Constraints: ">v1.0.0",
										Type:        v1beta1.ConfigurationPackageType,
									},
								}, nil
							},
							MockGetNode: func(identifier string) (dag.Node, error) {
								if identifier == confPkgName {
									return &v1beta1.Dependency{
										Package:     confPkgName,
										Constraints: ">v1.0.0",
										Type:        v1beta1.ConfigurationPackageType,
									}, nil
								}

								return nil, errors.New("not found")
							},
							MockSort: func() ([]string, error) {
								return nil, nil
							},
						}
					}),
					WithVersionFinder(&DefaultVersionFinder{fetcher: &fakexpkg.MockFetcher{MockTags: fakexpkg.NewMockTagsFn([]string{"v0.2.0", "v0.3.0", "v1.0.0", "v1.2.0"}, nil)}, config: &fakexpkg.MockConfigStore{
						MockPullSecretFor: fakexpkg.NewMockConfigStorePullSecretForFn("", "", nil),
					}}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errCreateDependency),
			},
		},
		"ErrorCreateMissingDependencyWithDigest": {
			reason: "We should return an error if unable to create missing dependency with digest.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							l, ok := o.(*v1beta1.Lock)
							if !ok {
								return kerrors.NewNotFound(schema.GroupResource{}, "")
							}

							// Populate package list so we attempt
							// reconciliation. This is overridden by the mock
							// DAG.
							l.Packages = append(l.Packages, coolPkgWithDigest)
							return nil
						}),
						MockCreate: test.NewMockCreateFn(errBoom),
						MockUpdate: test.NewMockUpdateFn(nil),
					},
				},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewDagFn(func() dag.DAG {
						return &fakedag.MockDag{
							MockInit: func(_ []dag.Node) ([]dag.Node, error) {
								return []dag.Node{
									&v1beta1.Dependency{
										Package:     confPkgName,
										Constraints: "sha256:ecc25c121431dfc7058754427f97c034ecde26d4aafa0da16d258090e0443904",
										Type:        v1beta1.ConfigurationPackageType,
									},
								}, nil
							},
							MockGetNode: func(identifier string) (dag.Node, error) {
								if identifier == confPkgName {
									return &v1beta1.Dependency{
										Package:     confPkgName,
										Constraints: "sha256:ecc25c121431dfc7058754427f97c034ecde26d4aafa0da16d258090e0443904",
										Type:        v1beta1.ConfigurationPackageType,
									}, nil
								}

								return nil, errors.New("not found")
							},
							MockSort: func() ([]string, error) {
								return nil, nil
							},
						}
					}),
					WithVersionFinder(&DefaultVersionFinder{fetcher: &fakexpkg.MockFetcher{MockTags: fakexpkg.NewMockTagsFn(nil, nil)}, config: &fakexpkg.MockConfigStore{
						MockPullSecretFor: fakexpkg.NewMockConfigStorePullSecretForFn("", "", nil),
					}}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errCreateDependency),
			},
		},
		"SuccessfulCreateMissingDependency": {
			reason: "We should not requeue if able to create missing dependency.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							if _, ok := o.(*v1beta1.Lock); ok {
								// Populate package list so we attempt
								// reconciliation. This is overridden by the mock
								// DAG.
								l := o.(*v1beta1.Lock)
								l.Packages = append(l.Packages, coolPkg)
								return nil
							}

							return kerrors.NewNotFound(schema.GroupResource{}, "")
						}),
						MockCreate: test.NewMockCreateFn(nil),
						MockUpdate: test.NewMockUpdateFn(nil),
					},
				},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewDagFn(func() dag.DAG {
						return &fakedag.MockDag{
							MockInit: func(_ []dag.Node) ([]dag.Node, error) {
								return []dag.Node{
									&v1beta1.Dependency{
										Package:     confPkgName,
										Constraints: ">v1.0.0",
										Type:        v1beta1.ConfigurationPackageType,
									},
								}, nil
							},
							MockGetNode: func(identifier string) (dag.Node, error) {
								if identifier == confPkgName {
									return &v1beta1.Dependency{
										Package:     confPkgName,
										Constraints: ">v1.0.0",
										Type:        v1beta1.ConfigurationPackageType,
									}, nil
								}

								return nil, errors.New("not found")
							},
							MockSort: func() ([]string, error) {
								return nil, nil
							},
						}
					}),
					WithVersionFinder(&DefaultVersionFinder{fetcher: &fakexpkg.MockFetcher{MockTags: fakexpkg.NewMockTagsFn([]string{"v2.0.0", "v0.3.0", "v1.0.0", "v1.2.0"}, nil)}, config: &fakexpkg.MockConfigStore{
						MockPullSecretFor: fakexpkg.NewMockConfigStorePullSecretForFn("", "", nil),
					}}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"SuccessfulCreateMissingDependencyWithDigest": {
			reason: "We should not requeue if able to create missing dependency with digest.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							l, ok := o.(*v1beta1.Lock)
							if !ok {
								return kerrors.NewNotFound(schema.GroupResource{}, "")
							}

							// Populate package list so we attempt
							// reconciliation. This is overridden by the mock
							// DAG.
							l.Packages = append(l.Packages, coolPkg)
							return nil
						}),
						MockCreate: test.NewMockCreateFn(nil),
						MockUpdate: test.NewMockUpdateFn(nil),
					},
				},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewDagFn(func() dag.DAG {
						return &fakedag.MockDag{
							MockInit: func(_ []dag.Node) ([]dag.Node, error) {
								return []dag.Node{
									&v1beta1.Dependency{
										Package:     proPkgName,
										Constraints: "sha256:ecc25c121431dfc7058754427f97c034ecde26d4aafa0da16d258090e0443904",
										Type:        v1beta1.ProviderPackageType,
									},
								}, nil
							},
							MockGetNode: func(identifier string) (dag.Node, error) {
								if identifier == proPkgName {
									return &v1beta1.Dependency{
										Package:     proPkgName,
										Constraints: "sha256:ecc25c121431dfc7058754427f97c034ecde26d4aafa0da16d258090e0443904",
										Type:        v1beta1.ProviderPackageType,
									}, nil
								}

								return nil, errors.New("not found")
							},
							MockSort: func() ([]string, error) {
								return nil, nil
							},
						}
					}),
					WithVersionFinder(&DefaultVersionFinder{fetcher: &fakexpkg.MockFetcher{MockTags: fakexpkg.NewMockTagsFn(nil, nil)}, config: &fakexpkg.MockConfigStore{
						MockPullSecretFor: fakexpkg.NewMockConfigStorePullSecretForFn("", "", nil),
					}}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"SuccessfulUpdateDependencyVersion": {
			reason: "We should not requeue if able to update dependency version.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							switch obj := o.(type) {
							case *v1beta1.Lock:
								// Populate package list so we attempt
								// reconciliation. This is overridden by the mock
								// DAG.
								obj.Packages = append(obj.Packages, coolPkg)
								return nil
							case *v1.Configuration:
								obj.Spec.Package = "hasheddan/config-nop-c:v1.0.0"
								return nil
							}
							return nil
						}),
						MockUpdate: test.NewMockUpdateFn(nil, func(client.Object) error { return nil }),
					},
				},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewDagFn(func() dag.DAG {
						return &fakedag.MockDag{
							MockInit: func(_ []dag.Node) ([]dag.Node, error) {
								return []dag.Node{
									&v1beta1.Dependency{
										Package:     confPkgName,
										Constraints: "v1.0.0",
										Type:        v1beta1.ConfigurationPackageType,
									},
								}, nil
							},
							MockSort: func() ([]string, error) {
								return nil, nil
							},
							MockGetNode: func(identifier string) (dag.Node, error) {
								if identifier == confPkgName {
									return &v1beta1.Dependency{
										Package:     confPkgName,
										Constraints: "v1.0.0",
										Type:        v1beta1.ConfigurationPackageType,
									}, nil
								}

								return nil, errors.New("not found")
							},
						}
					}),
					WithVersionFinder(&UpdatableVersionFinder{fetcher: &fakexpkg.MockFetcher{MockTags: fakexpkg.NewMockTagsFn([]string{"v2.0.0"}, nil)}, config: &fakexpkg.MockConfigStore{
						MockPullSecretFor: fakexpkg.NewMockConfigStorePullSecretForFn("", "", nil),
					}}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"SuccessfulUpdateDependencyVersionWithDigest": {
			reason: "We should not requeue if able to update dependency version with digest.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							switch obj := o.(type) {
							case *v1beta1.Lock:
								// Populate package list so we attempt
								// reconciliation. This is overridden by the mock
								// DAG.
								obj.Packages = append(obj.Packages, coolPkgWithDigest)
								return nil
							case *v1.Function:
								obj.Spec.Package = "hasheddan/func-nop-c:v1.0.0"
								return nil
							}
							return nil
						}),
						MockUpdate: test.NewMockUpdateFn(nil, func(client.Object) error { return nil }),
					},
				},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewDagFn(func() dag.DAG {
						return &fakedag.MockDag{
							MockInit: func(_ []dag.Node) ([]dag.Node, error) {
								return []dag.Node{
									&v1beta1.Dependency{
										Package:     funcPkgName,
										Constraints: "sha256:ecc25c121431dfc7058754427f97c034ecde26d4aafa0da16d258090e0443904",
										Type:        v1beta1.FunctionPackageType,
									},
								}, nil
							},
							MockSort: func() ([]string, error) {
								return nil, nil
							},
							MockGetNode: func(identifier string) (dag.Node, error) {
								if identifier == funcPkgName {
									return &v1beta1.Dependency{
										Package:     funcPkgName,
										Constraints: "sha256:ecc25c121431dfc7058754427f97c034ecde26d4aafa0da16d258090e0443904",
										Type:        v1beta1.FunctionPackageType,
									}, nil
								}

								return nil, errors.New("not found")
							},
						}
					}),
					WithVersionFinder(&UpdatableVersionFinder{fetcher: &fakexpkg.MockFetcher{MockTags: fakexpkg.NewMockTagsFn([]string{"v2.0.0"}, nil)}}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"ErrorCannotGetPackage": {
			reason: "We should return an error if getting package fails.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							if _, ok := o.(*v1beta1.Lock); ok {
								// Populate package list so we attempt
								// reconciliation. This is overridden by the mock
								// DAG.
								l := o.(*v1beta1.Lock)
								l.Packages = append(l.Packages, coolPkg)
								return nil
							}

							return errBoom
						}),
						MockCreate: test.NewMockCreateFn(nil),
						MockUpdate: test.NewMockUpdateFn(nil),
					},
				},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewDagFn(func() dag.DAG {
						return &fakedag.MockDag{
							MockInit: func(_ []dag.Node) ([]dag.Node, error) {
								return []dag.Node{
									&v1beta1.Dependency{
										Package:     confPkgName,
										Constraints: ">v1.0.0",
										Type:        v1beta1.ConfigurationPackageType,
									},
								}, nil
							},
							MockSort: func() ([]string, error) {
								return nil, nil
							},
							MockGetNode: func(identifier string) (dag.Node, error) {
								if identifier == confPkgName {
									return &v1beta1.Dependency{
										Package:     confPkgName,
										Constraints: ">v1.0.0",
										Type:        v1beta1.ConfigurationPackageType,
									}, nil
								}

								return nil, errors.New("not found")
							},
						}
					}),
					WithVersionFinder(&UpdatableVersionFinder{fetcher: &fakexpkg.MockFetcher{MockTags: fakexpkg.NewMockTagsFn([]string{"v0.2.0", "v0.3.0", "v1.0.0", "v1.2.0"}, nil)}, config: &fakexpkg.MockConfigStore{
						MockPullSecretFor: fakexpkg.NewMockConfigStorePullSecretForFn("", "", nil),
					}}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetDependency),
			},
		},
		"ErrorCannotUpdatePackageVersion": {
			reason: "We should return an error and requeue if updating package version fails.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							switch obj := o.(type) {
							case *v1beta1.Lock:
								// Populate package list so we attempt
								// reconciliation. This is overridden by the mock
								// DAG.
								obj.Packages = append(obj.Packages, coolPkg)
								return nil
							case *v1.Configuration:
								c := o.(*v1.Configuration)
								c.Spec.Package = "hasheddan/config-nop-c:v1.0.0"
								return nil
							}
							return nil
						}),
						MockUpdate: test.NewMockUpdateFn(nil, func(o client.Object) error {
							if _, ok := o.(*v1beta1.Lock); ok {
								return nil
							}

							return errBoom
						}),
					},
				},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewDagFn(func() dag.DAG {
						return &fakedag.MockDag{
							MockInit: func(_ []dag.Node) ([]dag.Node, error) {
								return []dag.Node{
									&v1beta1.Dependency{
										Package:     confPkgName,
										Constraints: "v1.0.0",
										Type:        v1beta1.ConfigurationPackageType,
									},
								}, nil
							},
							MockSort: func() ([]string, error) {
								return nil, nil
							},
							MockGetNode: func(identifier string) (dag.Node, error) {
								if identifier == confPkgName {
									return &v1beta1.Dependency{
										Package:     confPkgName,
										Constraints: "v1.0.0",
										Type:        v1beta1.ConfigurationPackageType,
									}, nil
								}

								return nil, errors.New("not found")
							},
						}
					}),
					WithVersionFinder(&UpdatableVersionFinder{fetcher: &fakexpkg.MockFetcher{MockTags: fakexpkg.NewMockTagsFn([]string{"v2.0.0"}, nil)}, config: &fakexpkg.MockConfigStore{
						MockPullSecretFor: fakexpkg.NewMockConfigStorePullSecretForFn("", "", nil),
					}}),
				},
			},
			want: want{
				r:   reconcile.Result{Requeue: true},
				err: errors.Wrap(errBoom, errUpdateDependency),
			},
		},
		"ErrorNoValidVersionUpdatable": {
			reason: "We should return an error if no valid version exists for updatable dependency.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							switch obj := o.(type) {
							case *v1beta1.Lock:
								// Populate package list so we attempt
								// reconciliation. This is overridden by the mock
								// DAG.
								obj.Packages = append(obj.Packages, coolPkg)
								return nil
							case *v1.Function:
								obj.Spec.Package = "hasheddan/function-nop-c:v1.0.0"
								return nil
							}
							return nil
						}),
						MockUpdate: test.NewMockUpdateFn(nil, func(o client.Object) error {
							if _, ok := o.(*v1beta1.Lock); ok {
								return nil
							}

							return errBoom
						}),
					},
				},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewDagFn(func() dag.DAG {
						return &fakedag.MockDag{
							MockInit: func(_ []dag.Node) ([]dag.Node, error) {
								return []dag.Node{
									&v1beta1.Dependency{
										Package:           "hasheddan/function-nop-c",
										Constraints:       "v0.3.0",
										Type:              v1beta1.FunctionPackageType,
										ParentConstraints: []string{">=v0.2.0", "<v0.2.0"},
									},
									&v1beta1.Dependency{
										Package:     confPkgName,
										Constraints: "v1.0.0",
										Type:        v1beta1.ConfigurationPackageType,
									},
									&v1beta1.Dependency{
										Package:     proPkgName,
										Constraints: "v1.0.0",
										Type:        v1beta1.ProviderPackageType,
									},
								}, nil
							},
							MockSort: func() ([]string, error) {
								return nil, nil
							},
							MockGetNode: func(identifier string) (dag.Node, error) {
								switch identifier {
								case confPkgName:
									return &v1beta1.Dependency{
										Package:     confPkgName,
										Constraints: "v1.0.0",
										Type:        v1beta1.ConfigurationPackageType,
									}, nil
								case funcPkgName:
									return &v1beta1.Dependency{
										Package:           funcPkgName,
										Constraints:       "v0.3.0",
										Type:              v1beta1.FunctionPackageType,
										ParentConstraints: []string{">=v0.2.0", "<v0.2.0"},
									}, nil
								case proPkgName:
									return &v1beta1.Dependency{
										Package:     proPkgName,
										Constraints: "v1.0.0",
										Type:        v1beta1.ProviderPackageType,
									}, nil
								}
								return nil, errors.New("not found")
							},
						}
					}),
					WithVersionFinder(&UpdatableVersionFinder{fetcher: &fakexpkg.MockFetcher{MockTags: fakexpkg.NewMockTagsFn([]string{"v1.0.0", "v0.3.0", "v0.2.0", "v0.1.0"}, nil)}, config: &fakexpkg.MockConfigStore{
						MockPullSecretFor: fakexpkg.NewMockConfigStorePullSecretForFn("", "", nil),
					}}),
				},
			},
			want: want{
				err: errors.New(errNoValidVersion),
			},
		},
		"ErrorNoValidVersionUpdatableWithDigest": {
			reason: "We should return an error if no valid version exists for updatable dependency.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							switch obj := o.(type) {
							case *v1beta1.Lock:
								// Populate package list so we attempt
								// reconciliation. This is overridden by the mock
								// DAG.
								obj.Packages = append(obj.Packages, coolPkg)
								return nil
							case *v1.Function:
								obj.Spec.Package = "hasheddan/function-nop-c:v1.0.0"
								return nil
							}
							return nil
						}),
						MockUpdate: test.NewMockUpdateFn(nil, func(o client.Object) error {
							if _, ok := o.(*v1beta1.Lock); ok {
								return nil
							}

							return errBoom
						}),
					},
				},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewDagFn(func() dag.DAG {
						return &fakedag.MockDag{
							MockInit: func(_ []dag.Node) ([]dag.Node, error) {
								return []dag.Node{
									&v1beta1.Dependency{
										Package:           "hasheddan/function-nop-c",
										Constraints:       "v0.3.0",
										Type:              v1beta1.FunctionPackageType,
										ParentConstraints: []string{"sha256:dif25c121431dfc7058754427f97c034ecde26d4aafa0da16d258090e0443904", "sha256:ecc25c121431dfc7058754427f97c034ecde26d4aafa0da16d258090e0443904"},
									},
									&v1beta1.Dependency{
										Package:     confPkgName,
										Constraints: "v1.0.0",
										Type:        v1beta1.ConfigurationPackageType,
									},
									&v1beta1.Dependency{
										Package:     proPkgName,
										Constraints: "v1.0.0",
										Type:        v1beta1.ProviderPackageType,
									},
								}, nil
							},
							MockSort: func() ([]string, error) {
								return nil, nil
							},
							MockGetNode: func(identifier string) (dag.Node, error) {
								switch identifier {
								case confPkgName:
									return &v1beta1.Dependency{
										Package:     confPkgName,
										Constraints: "v1.0.0",
										Type:        v1beta1.ConfigurationPackageType,
									}, nil
								case funcPkgName:
									return &v1beta1.Dependency{
										Package:           funcPkgName,
										Constraints:       "sha256:ecc25c121431dfc7058754427f97c034ecde26d4aafa0da16d258090e0443904",
										Type:              v1beta1.FunctionPackageType,
										ParentConstraints: []string{"sha256:dif25c121431dfc7058754427f97c034ecde26d4aafa0da16d258090e0443904", "sha256:ecc25c121431dfc7058754427f97c034ecde26d4aafa0da16d258090e0443904"},
									}, nil
								case proPkgName:
									return &v1beta1.Dependency{
										Package:     proPkgName,
										Constraints: "v1.0.0",
										Type:        v1beta1.ProviderPackageType,
									}, nil
								}
								return nil, errors.New("not found")
							},
						}
					}),
					WithVersionFinder(&UpdatableVersionFinder{fetcher: &fakexpkg.MockFetcher{MockTags: fakexpkg.NewMockTagsFn([]string{"v1.0.0", "v0.3.0", "v0.2.0", "v0.1.0"}, nil)}, config: &fakexpkg.MockConfigStore{
						MockPullSecretFor: fakexpkg.NewMockConfigStorePullSecretForFn("", "", nil),
					}}),
				},
			},
			want: want{
				err: errors.New(errNoValidVersion),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewReconciler(tc.args.mgr, append(tc.args.rec, WithLogger(testLog))...)
			got, err := r.Reconcile(context.Background(), reconcile.Request{})

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.r, got, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestUpdatableFindValidDependencyVersion(t *testing.T) {
	type args struct {
		fetcher xpkg.Fetcher
		dep     *v1beta1.Dependency
		n       dag.Node
	}
	type want struct {
		ver string
		err error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SuccessfulDigest": {
			reason: "We should return the version if it is a digest.",
			args: args{
				dep: &v1beta1.Dependency{
					Package:     "ezgidemirel/config-nop",
					Constraints: "sha256:ecc25c121431dfc7058754427f97c034ecde26d4aafa0da16d258090e0443904",
				},
			},
			want: want{
				ver: "sha256:ecc25c121431dfc7058754427f97c034ecde26d4aafa0da16d258090e0443904",
			},
		},
		"SuccessfulUpgradeMinValid": {
			reason: "We should return the minimum valid version if upgrade is required",
			args: args{
				fetcher: &fakexpkg.MockFetcher{
					MockTags: fakexpkg.NewMockTagsFn([]string{"v1.0.0", "v1.1.0", "v1.2.0"}, nil),
				},
				dep: &v1beta1.Dependency{
					Package:           "ezgidemirel/config-nop",
					ParentConstraints: []string{">=v1.1.0"},
				},
				n: &v1beta1.LockPackage{
					Source:  "ezgidemirel/config-nop",
					Version: "v1.0.0",
				},
			},
			want: want{
				ver: "v1.1.0",
			},
		},
		"ErrorFetchTags": {
			reason: "We should return an error if fail to fetch tags to account for network issues.",
			args: args{
				fetcher: &fakexpkg.MockFetcher{
					MockTags: fakexpkg.NewMockTagsFn(nil, errBoom),
				},
				dep: &v1beta1.Dependency{
					Package:     "ezgidemirel/config-nop",
					Constraints: "*",
				},
			},
			want: want{
				ver: "",
				err: errors.New(errFetchTags),
			},
		},
		"ErrorInvalidParentConstraints": {
			reason: "We should return an error if parent constraints are invalid.",
			args: args{
				fetcher: &fakexpkg.MockFetcher{
					MockTags: fakexpkg.NewMockTagsFn([]string{"v1.0.0"}, nil),
				},
				dep: &v1beta1.Dependency{
					Package:           "ezgidemirel/config-nop",
					ParentConstraints: []string{"invalid"},
				},
			},
			want: want{
				ver: "",
				err: errors.New(errInvalidConstraint),
			},
		},
		"ErrorDowngradeNotAllowed": {
			reason: "We should return an error if downgrade is not allowed.",
			args: args{
				fetcher: &fakexpkg.MockFetcher{
					MockTags: fakexpkg.NewMockTagsFn([]string{"v1.0.0", "v0.2.0", "v0.3.0"}, nil),
				},
				dep: &v1beta1.Dependency{
					Package:           "ezgidemirel/config-nop",
					ParentConstraints: []string{"v0.2.0"},
				},
				n: &v1beta1.LockPackage{
					Source:  "ezgidemirel/config-nop",
					Version: "v1.0.0",
				},
			},
			want: want{
				ver: "",
				err: errors.New(errDowngradeNotAllowed),
			},
		},
		"NoValidVersion": {
			reason: "We should not requeue if valid version does not exist for dependency.",
			args: args{
				fetcher: &fakexpkg.MockFetcher{
					MockTags: fakexpkg.NewMockTagsFn([]string{"abc", "v0.2.0", "v0.3.0", "v1.0.0"}, nil),
				},
				dep: &v1beta1.Dependency{
					Package:           "ezgidemirel/config-nop",
					ParentConstraints: []string{">v1.0.0"},
				},
				n: &v1beta1.LockPackage{
					Source:  "ezgidemirel/config-nop",
					Version: "v1.0.0",
				},
			},
			want: want{
				ver: "",
			},
		},
	}

	for tcName, tc := range cases {
		t.Run(tcName, func(t *testing.T) {
			u := &UpdatableVersionFinder{
				fetcher: tc.args.fetcher,
				config: &fakexpkg.MockConfigStore{
					MockPullSecretFor: fakexpkg.NewMockConfigStorePullSecretForFn("", "", nil),
				},
			}
			r, _ := name.ParseReference(tc.args.dep.Package) // nolint: errcheck // we will catch anyways if r is nil
			got, err := u.FindValidDependencyVersion(context.Background(), tc.args.dep, r, tc.args.n, testLog)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.ver, got, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
