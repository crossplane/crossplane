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

	"github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/internal/dag"
	fakedag "github.com/crossplane/crossplane/internal/dag/fake"
	fakexpkg "github.com/crossplane/crossplane/internal/xpkg/fake"
)

func TestReconcile(t *testing.T) {
	errBoom := errors.New("boom")
	testLog := logging.NewLogrLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(io.Discard)).WithName("testlog"))

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
				r: reconcile.Result{Requeue: false},
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
							MockSort: func() ([]string, error) {
								return nil, nil
							},
						}
					}),
					WithFetcher(&fakexpkg.MockFetcher{
						MockTags: fakexpkg.NewMockTagsFn(nil, errBoom),
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errFetchTags),
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
							MockSort: func() ([]string, error) {
								return nil, nil
							},
						}
					}),
					WithFetcher(&fakexpkg.MockFetcher{
						MockTags: fakexpkg.NewMockTagsFn([]string{"v0.2.0", "v0.3.0", "v1.0.0"}, nil),
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
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
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
