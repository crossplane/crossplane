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
	"fmt"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	pkgName "github.com/google/go-containerregistry/pkg/name"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	fakexpkg "github.com/crossplane/crossplane/internal/xpkg/fake"
)

const (
	digest1 = "sha256:ecc25c121431dfc7058754427f97c034ecde26d4aafa0da16d258090e0443904"
	digest2 = "sha256:3c25c121431dfc7058754427f97c034ecde26d4aafa0da16d258090e04439040"
)

var (
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
							l.Packages = append(l.Packages, v1beta1.LockPackage{
								Name:    "cool-package",
								Type:    v1beta1.ProviderPackageType,
								Source:  "cool-repo/cool-image",
								Version: "v0.0.1",
							})
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
							l.Packages = append(l.Packages, v1beta1.LockPackage{
								Name:    "cool-package",
								Type:    v1beta1.ProviderPackageType,
								Source:  "cool-repo/cool-image",
								Version: "v0.0.1",
							})
							return nil
						}),
						MockUpdate:       test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
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
							l.Packages = append(l.Packages, v1beta1.LockPackage{
								Name:    "cool-package",
								Type:    v1beta1.ProviderPackageType,
								Source:  "cool-repo/cool-image",
								Version: "v0.0.1",
							})
							return nil
						}),
						MockUpdate:       test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
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
							l.Packages = append(l.Packages, v1beta1.LockPackage{
								Name:    "cool-package",
								Type:    v1beta1.ProviderPackageType,
								Source:  "cool-repo/cool-image",
								Version: "v0.0.1",
							})
							return nil
						}),
						MockUpdate:       test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
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
							l.Packages = append(l.Packages, v1beta1.LockPackage{
								Name:    "cool-package",
								Type:    v1beta1.ProviderPackageType,
								Source:  "cool-repo/cool-image",
								Version: "sha256:ecc25c121431dfc7058754427f97c034ecde26d4aafa0da16d258090e0443904",
							})
							return nil
						}),
						MockUpdate:       test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
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
							l.Packages = append(l.Packages, v1beta1.LockPackage{
								Name:    "cool-package",
								Type:    v1beta1.ProviderPackageType,
								Source:  "cool-repo/cool-image",
								Version: "v0.0.1",
							})
							return nil
						}),
						MockUpdate:       test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
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
							MockSort: func() ([]string, error) {
								return nil, nil
							},
						}
					}),
				},
			},
			want: want{
				r:   reconcile.Result{Requeue: false},
				err: errors.Wrap(errors.Wrap(errors.New("improper constraint: "), errInvalidConstraint), errFindDependency),
			},
		},
		"ErrorGetPullSecretFromImageConfig": {
			reason: "We should return an error if fail to get pull secret from configs.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							// Populate package list so we attempt
							// reconciliation. This is overridden by the mock
							// DAG.
							l := o.(*v1beta1.Lock)
							l.Packages = append(l.Packages, v1beta1.LockPackage{
								Name:    "cool-package",
								Type:    v1beta1.ProviderPackageType,
								Source:  "cool-repo/cool-image",
								Version: "v0.0.1",
							})
							return nil
						}),
						MockUpdate:       test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
				},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewDagFn(func() dag.DAG {
						return &fakedag.MockDag{
							MockInit: func(_ []dag.Node) ([]dag.Node, error) {
								return []dag.Node{
									&v1beta1.Dependency{
										Package:     "registry1.com/acme-co/configuration-foo",
										Constraints: "v0.0.1",
									},
								}, nil
							},
							MockSort: func() ([]string, error) {
								return nil, nil
							},
						}
					}),
					WithFetcher(&fakexpkg.MockFetcher{
						MockTags: fakexpkg.NewMockTagsFn(nil, errBoom),
					}),
					WithConfigStore(&fakexpkg.MockConfigStore{
						MockPullSecretFor: fakexpkg.NewMockConfigStorePullSecretForFn("", "", errBoom),
					}),
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrap(errBoom, errGetPullConfig), errFindDependency),
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
							l.Packages = append(l.Packages, v1beta1.LockPackage{
								Name:    "cool-package",
								Type:    v1beta1.ProviderPackageType,
								Source:  "cool-repo/cool-image",
								Version: "v0.0.1",
							})
							return nil
						}),
						MockUpdate:       test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
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
							MockSort: func() ([]string, error) {
								return nil, nil
							},
						}
					}),
					WithFetcher(&fakexpkg.MockFetcher{
						MockTags: fakexpkg.NewMockTagsFn(nil, errBoom),
					}),
					WithConfigStore(&fakexpkg.MockConfigStore{
						MockPullSecretFor: fakexpkg.NewMockConfigStorePullSecretForFn("", "", nil),
					}),
				},
			},
			want: want{
				err: errors.Wrap(errors.New(errFetchTags), errFindDependency),
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
							l.Packages = append(l.Packages, v1beta1.LockPackage{
								Name:    "cool-package",
								Type:    v1beta1.ProviderPackageType,
								Source:  "cool-repo/cool-image",
								Version: "v0.0.1",
							})
							return nil
						}),
						MockUpdate:       test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
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
							MockSort: func() ([]string, error) {
								return nil, nil
							},
						}
					}),
					WithFetcher(&fakexpkg.MockFetcher{
						MockTags: fakexpkg.NewMockTagsFn([]string{"v0.2.0", "v0.3.0", "v1.0.0"}, nil),
					}),
					WithConfigStore(&fakexpkg.MockConfigStore{
						MockPullSecretFor: fakexpkg.NewMockConfigStorePullSecretForFn("", "", nil),
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"ErrorCreateMissingDependency": {
			reason: "We should return an error if unable to create missing dependency.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							// Populate package list so we attempt
							// reconciliation. This is overridden by the mock
							// DAG.
							l := o.(*v1beta1.Lock)
							l.Packages = append(l.Packages, v1beta1.LockPackage{
								Name:    "cool-package",
								Type:    v1beta1.ProviderPackageType,
								Source:  "cool-repo/cool-image",
								Version: "v0.0.1",
							})
							return nil
						}),
						MockCreate:       test.NewMockCreateFn(errBoom),
						MockUpdate:       test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
				},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewDagFn(func() dag.DAG {
						return &fakedag.MockDag{
							MockInit: func(_ []dag.Node) ([]dag.Node, error) {
								return []dag.Node{
									&v1beta1.Dependency{
										Package:     "hasheddan/config-nop-c",
										Constraints: ">v1.0.0",
										Type:        v1beta1.ConfigurationPackageType,
									},
								}, nil
							},
							MockSort: func() ([]string, error) {
								return nil, nil
							},
						}
					}),
					WithFetcher(&fakexpkg.MockFetcher{
						MockTags: fakexpkg.NewMockTagsFn([]string{"v0.2.0", "v0.3.0", "v1.0.0", "v1.2.0"}, nil),
					}),
					WithConfigStore(&fakexpkg.MockConfigStore{
						MockPullSecretFor: fakexpkg.NewMockConfigStorePullSecretForFn("", "", nil),
					}),
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
							// Populate package list so we attempt
							// reconciliation. This is overridden by the mock
							// DAG.
							l := o.(*v1beta1.Lock)
							l.Packages = append(l.Packages, v1beta1.LockPackage{
								Name:    "cool-package",
								Type:    v1beta1.ProviderPackageType,
								Source:  "cool-repo/cool-image",
								Version: "sha256:ecc25c121431dfc7058754427f97c034ecde26d4aafa0da16d258090e0443904",
							})
							return nil
						}),
						MockCreate:       test.NewMockCreateFn(errBoom),
						MockUpdate:       test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
				},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewDagFn(func() dag.DAG {
						return &fakedag.MockDag{
							MockInit: func(_ []dag.Node) ([]dag.Node, error) {
								return []dag.Node{
									&v1beta1.Dependency{
										Package:     "hasheddan/config-nop-c",
										Constraints: "sha256:ecc25c121431dfc7058754427f97c034ecde26d4aafa0da16d258090e0443904",
										Type:        v1beta1.ConfigurationPackageType,
									},
								}, nil
							},
							MockSort: func() ([]string, error) {
								return nil, nil
							},
						}
					}),
					WithFetcher(&fakexpkg.MockFetcher{
						MockTags: fakexpkg.NewMockTagsFn([]string{"v0.2.0", "v0.3.0", "v1.0.0", "v1.2.0"}, nil),
					}),
					WithConfigStore(&fakexpkg.MockConfigStore{
						MockPullSecretFor: fakexpkg.NewMockConfigStorePullSecretForFn("", "", nil),
					}),
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
							// Populate package list so we attempt
							// reconciliation. This is overridden by the mock
							// DAG.
							l := o.(*v1beta1.Lock)
							l.Packages = append(l.Packages, v1beta1.LockPackage{
								Name:    "cool-package",
								Type:    v1beta1.ProviderPackageType,
								Source:  "cool-repo/cool-image",
								Version: "v0.0.1",
							})
							return nil
						}),
						MockCreate:       test.NewMockCreateFn(nil),
						MockUpdate:       test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
				},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewDagFn(func() dag.DAG {
						return &fakedag.MockDag{
							MockInit: func(_ []dag.Node) ([]dag.Node, error) {
								return []dag.Node{
									&v1beta1.Dependency{
										Package:     "hasheddan/config-nop-c",
										Constraints: ">v1.0.0",
										Type:        v1beta1.ConfigurationPackageType,
									},
								}, nil
							},
							MockSort: func() ([]string, error) {
								return nil, nil
							},
						}
					}),
					WithFetcher(&fakexpkg.MockFetcher{
						MockTags: fakexpkg.NewMockTagsFn([]string{"v0.2.0", "v0.3.0", "v1.0.0", "v1.2.0"}, nil),
					}),
					WithConfigStore(&fakexpkg.MockConfigStore{
						MockPullSecretFor: fakexpkg.NewMockConfigStorePullSecretForFn("", "", nil),
					}),
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
							// Populate package list so we attempt
							// reconciliation. This is overridden by the mock
							// DAG.
							l := o.(*v1beta1.Lock)
							l.Packages = append(l.Packages, v1beta1.LockPackage{
								Name:    "cool-package",
								Type:    v1beta1.ProviderPackageType,
								Source:  "cool-repo/cool-image",
								Version: "v0.0.1",
							})
							return nil
						}),
						MockCreate:       test.NewMockCreateFn(nil),
						MockUpdate:       test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
				},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewDagFn(func() dag.DAG {
						return &fakedag.MockDag{
							MockInit: func(_ []dag.Node) ([]dag.Node, error) {
								return []dag.Node{
									&v1beta1.Dependency{
										Package:     "hasheddan/provider-nop-c",
										Constraints: "sha256:ecc25c121431dfc7058754427f97c034ecde26d4aafa0da16d258090e0443904",
										Type:        v1beta1.ProviderPackageType,
									},
								}, nil
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
		"SuccessfulUpdateDependency": {
			reason: "We should update the dependency if the flag is enabled and there is a valid version.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							l := o.(*v1beta1.Lock)
							l.Packages = append(l.Packages, v1beta1.LockPackage{
								Name:    "cool-package",
								Type:    v1beta1.ProviderPackageType,
								Source:  "cool-repo/cool-image",
								Version: "v0.0.1",
							})
							return nil
						}),
						MockUpdate:       test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
						MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
							l := obj.(*v1.ProviderList)
							l.Items = append(l.Items, v1.Provider{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "this-is-a-cool-image",
									Namespace: "crossplane-system",
								},
								Spec: v1.ProviderSpec{
									PackageSpec: v1.PackageSpec{
										Package: "cool-repo/cool-image:v0.0.1",
									},
								},
							})
							return nil
						}),
					},
				},
				rec: []ReconcilerOption{
					WithUpgradesEnabled(),
					WithConfigStore(&fakexpkg.MockConfigStore{
						MockPullSecretFor: fakexpkg.NewMockConfigStorePullSecretForFn("", "", nil),
					}),
					WithFetcher(&fakexpkg.MockFetcher{
						MockTags: fakexpkg.NewMockTagsFn([]string{"v0.0.1", "v1.0.0", "v1.0.1", "v2.0.0"}, nil),
					}),
					WithNewDagFn(func() dag.DAG {
						return &fakedag.MockDag{
							MockInit: func(_ []dag.Node) ([]dag.Node, error) {
								return []dag.Node{
									&v1beta1.Dependency{
										Package:     "cool-repo/cool-image",
										Constraints: ">v1.0.0",
										Type:        v1beta1.ProviderPackageType,
									},
								}, nil
							},
							MockSort: func() ([]string, error) {
								return nil, nil
							},
							MockGetNode: func(_ string) (dag.Node, error) {
								return &v1beta1.Dependency{
									Package: "cool-repo/cool-image",
									ParentConstraints: []string{
										">v1.0.0",
									},
									Type: v1beta1.ProviderPackageType,
								}, nil
							},
						}
					}),
				},
			},
		},
		"SuccessfulUpdateDependencyWithDigest": {
			reason: "We should update the dependency if the flag is enabled and there is a valid digest.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							l := o.(*v1beta1.Lock)
							l.Packages = append(l.Packages, v1beta1.LockPackage{
								Name:    "cool-package",
								Type:    v1beta1.ProviderPackageType,
								Source:  "cool-repo/cool-image",
								Version: "v0.0.1",
							})
							return nil
						}),
						MockUpdate:       test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
						MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
							l := obj.(*v1.ProviderList)
							l.Items = append(l.Items, v1.Provider{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "this-is-a-cool-image",
									Namespace: "crossplane-system",
								},
								Spec: v1.ProviderSpec{
									PackageSpec: v1.PackageSpec{
										Package: "cool-repo/cool-image:v0.0.1",
									},
								},
							})
							return nil
						}),
					},
				},
				rec: []ReconcilerOption{
					WithUpgradesEnabled(),
					WithConfigStore(&fakexpkg.MockConfigStore{
						MockPullSecretFor: fakexpkg.NewMockConfigStorePullSecretForFn("", "", nil),
					}),
					WithFetcher(&fakexpkg.MockFetcher{
						MockTags: fakexpkg.NewMockTagsFn([]string{"v0.0.1", "v1.0.0", "v1.0.1", "v2.0.0"}, nil),
					}),
					WithNewDagFn(func() dag.DAG {
						return &fakedag.MockDag{
							MockInit: func(_ []dag.Node) ([]dag.Node, error) {
								return []dag.Node{
									&v1beta1.Dependency{
										Package:     "cool-repo/cool-image",
										Constraints: ">v1.0.0",
										Type:        v1beta1.ProviderPackageType,
									},
								}, nil
							},
							MockSort: func() ([]string, error) {
								return nil, nil
							},
							MockGetNode: func(_ string) (dag.Node, error) {
								return &v1beta1.Dependency{
									Package: "cool-repo/cool-image",
									ParentConstraints: []string{
										digest1,
										digest1,
									},
									Type: v1beta1.ProviderPackageType,
								}, nil
							},
						}
					}),
				},
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

func TestSplitPackage(t *testing.T) {
	type args struct {
		p string
	}
	type want struct {
		repo    string
		version string
		err     error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"PackageWithVersion": {
			reason: "We should be able to split package and version.",
			args: args{
				p: "cool-repo/cool-image:v0.0.1",
			},
			want: want{
				repo:    "cool-repo/cool-image",
				version: "v0.0.1",
			},
		},
		"PackageWithoutDigest": {
			reason: "We should be able to split package and version without digest.",
			args: args{
				p: "cool-repo/cool-image@sha256:ecc25c121431dfc7058754427f97c034ecde26d4aafa0da16d258090e0443904",
			},
			want: want{
				repo:    "cool-repo/cool-image",
				version: "sha256:ecc25c121431dfc7058754427f97c034ecde26d4aafa0da16d258090e0443904",
			},
		},
		"PackageWithoutVersion": {
			reason: "We should return an error if package does not have version.",
			args: args{
				p: "cool-repo/cool-image-no-version",
			},
			want: want{
				err: errors.Errorf(errFmtSplit, 1),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r, v, err := splitPackage(tc.args.p)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.repo, r, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.version, v, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestFindDigestToUpdate(t *testing.T) {
	type args struct {
		node dag.Node
	}
	type want struct {
		digest string
		err    error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"AllSameDigests": {
			reason: "We should be able to find the digest to update.",
			args: args{
				node: &v1beta1.Dependency{
					Package: "cool-repo/cool-image",
					ParentConstraints: []string{
						digest1,
						digest1,
					},
				},
			},
			want: want{
				digest: digest1,
			},
		},
		"DifferentDigests": {
			reason: "We should return an error if digests are different.",
			args: args{
				node: &v1beta1.Dependency{
					Package: "cool-repo/cool-image",
					ParentConstraints: []string{
						digest1,
						digest2,
					},
				},
			},
			want: want{
				err: errors.Errorf(errFmtDiffDigests, fmt.Sprintf("[%s %s]", digest1, digest2)),
			},
		},
		"AllVersions": {
			reason: "We should return an empty string if all parent constraints are versions.",
			args: args{
				node: &v1beta1.Dependency{
					Package:           "cool-repo/cool-image",
					ParentConstraints: []string{"v0.0.1", "v0.0.2"},
				},
			},
			want: want{
				digest: "",
				err:    nil,
			},
		},
		"MixedConstraintTypes": {
			reason: "We should return an error if both versions and digests are present.",
			args: args{
				node: &v1beta1.Dependency{
					Package: "cool-repo/cool-image",
					ParentConstraints: []string{
						"v0.0.1",
						digest1,
					},
				},
			},
			want: want{
				err: errors.Errorf(errFmtDiffConstraintTypes, fmt.Sprintf("[v0.0.1 %s]", digest1)),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := findDigestToUpdate(tc.args.node)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.digest, got, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestReconcilerFindDependencyVersionToUpgrade(t *testing.T) {
	type args struct {
		mgr    manager.Manager
		insVer string
		dep    dag.Node
		rec    []ReconcilerOption
	}
	type want struct {
		version string
		err     error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"SuccessReturnDigest": {
			reason: "We should be able to find the digest to update.",
			args: args{
				mgr:    &fake.Manager{Client: test.NewMockClient()},
				insVer: "v0.0.1",
				dep: &v1beta1.Dependency{
					Package: "cool-repo/cool-image",
					ParentConstraints: []string{
						digest1,
						digest1,
					},
				},
			},
			want: want{
				version: digest1,
			},
		},
		"ErrorMixedParentConstraints": {
			reason: "We should return an error if parent constraints are mixed.",
			args: args{
				mgr:    &fake.Manager{Client: test.NewMockClient()},
				insVer: "v0.0.1",
				dep: &v1beta1.Dependency{
					Package: "cool-repo/cool-image",
					ParentConstraints: []string{
						digest1,
						"v0.0.1",
					},
				},
			},
			want: want{
				err: errors.Errorf(errFmtDiffConstraintTypes, fmt.Sprintf("[%s v0.0.1]", digest1)),
			},
		},
		"SuccessReturnVersion": {
			reason: "We should be able to find the version to update.",
			args: args{
				mgr:    &fake.Manager{Client: test.NewMockClient()},
				insVer: "v1.0.0",
				dep: &v1beta1.Dependency{
					Package: "cool-repo/cool-image",
					ParentConstraints: []string{
						">=v1.0.0",
						"v2.0.0",
					},
				},
				rec: []ReconcilerOption{
					WithFetcher(&fakexpkg.MockFetcher{
						MockTags: fakexpkg.NewMockTagsFn([]string{"v1.0.0", "v1.0.1", "v2.0.0", "v3.0.0"}, nil),
					}),
					WithConfigStore(&fakexpkg.MockConfigStore{
						MockPullSecretFor: fakexpkg.NewMockConfigStorePullSecretForFn("", "", nil),
					}),
				},
			},
			want: want{
				version: "v2.0.0",
			},
		},
		"ErrorNoValidVersion": {
			reason: "We should return an error if no valid version exists for dependency.",
			args: args{
				mgr:    &fake.Manager{Client: test.NewMockClient()},
				insVer: "v1.0.0",
				dep: &v1beta1.Dependency{
					Package: "cool-repo/cool-image",
					ParentConstraints: []string{
						">=v1.0.0",
						"v2.0.0",
					},
				},
				rec: []ReconcilerOption{
					WithFetcher(&fakexpkg.MockFetcher{
						MockTags: fakexpkg.NewMockTagsFn([]string{"v1.0.0", "v1.0.1"}, nil),
					}),
					WithConfigStore(&fakexpkg.MockConfigStore{
						MockPullSecretFor: fakexpkg.NewMockConfigStorePullSecretForFn("", "", nil),
					}),
				},
			},
			want: want{
				err: errors.Errorf(errFmtNoValidVersion, "cool-repo/cool-image", "[>=v1.0.0 v2.0.0]"),
			},
		},
		"ErrorNoValidVersionDowngrade": {
			reason: "We should return an error if no valid version exists for dependency and downgrade is not allowed.",
			args: args{
				mgr:    &fake.Manager{Client: test.NewMockClient()},
				insVer: "v1.0.0",
				dep: &v1beta1.Dependency{
					Package: "cool-repo/cool-image",
					ParentConstraints: []string{
						"<=v1.0.0",
						"v0.0.1",
					},
				},
				rec: []ReconcilerOption{
					WithFetcher(&fakexpkg.MockFetcher{
						MockTags: fakexpkg.NewMockTagsFn([]string{"v0.0.1", "v1.0.0"}, nil),
					}),
					WithConfigStore(&fakexpkg.MockConfigStore{
						MockPullSecretFor: fakexpkg.NewMockConfigStorePullSecretForFn("", "", nil),
					}),
				},
			},
			want: want{
				err: errors.Errorf(errFmtNoValidVersion, "cool-repo/cool-image", "[<=v1.0.0 v0.0.1]"),
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewReconciler(tc.args.mgr, append(tc.args.rec, WithLogger(testLog))...)
			ref, _ := pkgName.ParseReference(tc.args.dep.Identifier())
			got, err := r.findDependencyVersionToUpgrade(context.Background(), ref, tc.args.insVer, tc.args.dep, testLog)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.findDependencyVersionToUpgrade(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.version, got, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.findDependencyVersionToUpgrade(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestReconcilerGetPackageWithRef(t *testing.T) {
	type args struct {
		mgr    manager.Manager
		pkgRef string
		t      v1beta1.PackageType
		rec    []ReconcilerOption
	}
	type want struct {
		pkg v1.Package
		err error
	}
	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ProviderPackage": {
			reason: "We should be able to get the provider package with the given ID.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
						MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
							l := obj.(*v1.ProviderList)
							l.Items = append(l.Items, v1.Provider{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "this-is-a-cool-image",
									Namespace: "crossplane-system",
								},
								Spec: v1.ProviderSpec{
									PackageSpec: v1.PackageSpec{
										Package: "cool-repo/cool-image:v0.0.1",
									},
								},
							})
							return nil
						}),
					},
				},
				pkgRef: "cool-repo/cool-image:v0.0.1",
				t:      v1beta1.ProviderPackageType,
			},
			want: want{
				pkg: &v1.Provider{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "this-is-a-cool-image",
						Namespace: "crossplane-system",
					},
					Spec: v1.ProviderSpec{
						PackageSpec: v1.PackageSpec{
							Package: "cool-repo/cool-image:v0.0.1",
						},
					},
				},
			},
		},
		"ConfigurationPackageNotFound": {
			reason: "We should be able to get the configuration package with the given ID.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
						MockList: test.NewMockListFn(nil, func(_ client.ObjectList) error {
							return nil
						}),
					},
				},
				pkgRef: "cool-repo/cool-image:v1.2.3",
				t:      v1beta1.ConfigurationPackageType,
			},
			want: want{
				pkg: nil,
			},
		},
		"FunctionWithDigest": {
			reason: "We should be able to get the function package with the given ID.",
			args: args{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
						MockList: test.NewMockListFn(nil, func(obj client.ObjectList) error {
							l := obj.(*v1.FunctionList)
							l.Items = append(l.Items, v1.Function{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "func-with-digest",
									Namespace: "crossplane-system",
								},
								Spec: v1.FunctionSpec{
									PackageSpec: v1.PackageSpec{
										Package: "cool-repo/cool-image@sha256:ecc25c121431dfc7058754427f97c034ecde26d4aafa0da16d258090e0443904",
									},
								},
							})
							return nil
						}),
					},
				},
				pkgRef: "cool-repo/cool-image@sha256:ecc25c121431dfc7058754427f97c034ecde26d4aafa0da16d258090e0443904",
				t:      v1beta1.FunctionPackageType,
			},
			want: want{
				pkg: &v1.Function{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "func-with-digest",
						Namespace: "crossplane-system",
					},
					Spec: v1.FunctionSpec{
						PackageSpec: v1.PackageSpec{
							Package: "cool-repo/cool-image@sha256:ecc25c121431dfc7058754427f97c034ecde26d4aafa0da16d258090e0443904",
						},
					},
				},
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewReconciler(tc.args.mgr, append(tc.args.rec, WithLogger(testLog))...)
			got, err := r.getPackageWithRef(context.Background(), tc.args.pkgRef, tc.args.t)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.getPackageWithRef(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.pkg, got, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.getPackageWithRef(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
