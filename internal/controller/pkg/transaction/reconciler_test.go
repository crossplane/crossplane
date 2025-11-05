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

package transaction

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	v1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"
	"github.com/crossplane/crossplane/v2/apis/pkg/v1alpha1"
	"github.com/crossplane/crossplane/v2/apis/pkg/v1beta1"
	"github.com/crossplane/crossplane/v2/internal/xpkg"
)

type MockLockManager struct {
	MockAcquire func(ctx context.Context, tx *v1alpha1.Transaction) ([]v1beta1.LockPackage, error)
	MockCommit  func(ctx context.Context, tx *v1alpha1.Transaction, packages []v1beta1.LockPackage) error
	MockRelease func(ctx context.Context, tx *v1alpha1.Transaction) error
}

func (m *MockLockManager) Acquire(ctx context.Context, tx *v1alpha1.Transaction) ([]v1beta1.LockPackage, error) {
	return m.MockAcquire(ctx, tx)
}

func (m *MockLockManager) Commit(ctx context.Context, tx *v1alpha1.Transaction, packages []v1beta1.LockPackage) error {
	return m.MockCommit(ctx, tx, packages)
}

func (m *MockLockManager) Release(ctx context.Context, tx *v1alpha1.Transaction) error {
	return m.MockRelease(ctx, tx)
}

type MockDependencySolver struct {
	MockSolve func(ctx context.Context, source string, currentLock []v1beta1.LockPackage) ([]v1beta1.LockPackage, error)
}

func (m *MockDependencySolver) Solve(ctx context.Context, source string, currentLock []v1beta1.LockPackage) ([]v1beta1.LockPackage, error) {
	return m.MockSolve(ctx, source, currentLock)
}

type MockValidator struct {
	MockValidate func(ctx context.Context, tx *v1alpha1.Transaction) error
}

func (m *MockValidator) Validate(ctx context.Context, tx *v1alpha1.Transaction) error {
	return m.MockValidate(ctx, tx)
}

type MockInstaller struct {
	MockInstall func(ctx context.Context, tx *v1alpha1.Transaction, xp *xpkg.Package, version string) error
}

func (m *MockInstaller) Install(ctx context.Context, tx *v1alpha1.Transaction, xp *xpkg.Package, version string) error {
	return m.MockInstall(ctx, tx, xp, version)
}

type MockXpkgClient struct {
	MockGet func(ctx context.Context, ref string, opts ...xpkg.GetOption) (*xpkg.Package, error)
}

func (m *MockXpkgClient) Get(ctx context.Context, ref string, opts ...xpkg.GetOption) (*xpkg.Package, error) {
	return m.MockGet(ctx, ref, opts...)
}

func (m *MockXpkgClient) ListVersions(_ context.Context, _ string, _ ...xpkg.GetOption) ([]string, error) {
	return nil, errors.New("not implemented")
}

func TestReconcile(t *testing.T) {
	type params struct {
		mgr  manager.Manager
		pkg  xpkg.Client
		opts []ReconcilerOption
	}

	type want struct {
		r   reconcile.Result
		err error
	}

	cases := map[string]struct {
		reason string
		params params
		want   want
	}{
		"NotFound": {
			reason: "We should return early if the Transaction was not found.",
			params: params{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
					},
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"GetError": {
			reason: "We should return an error if we can't get the Transaction",
			params: params{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(errors.New("boom")),
					},
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: cmpopts.AnyError,
			},
		},
		"Deleted": {
			reason: "We should return early if the Transaction was deleted.",
			params: params{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							tx := &v1alpha1.Transaction{
								ObjectMeta: metav1.ObjectMeta{
									DeletionTimestamp: ptr.To(metav1.Now()),
								},
							}
							tx.DeepCopyInto(obj.(*v1alpha1.Transaction))
							return nil
						}),
					},
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"Complete": {
			reason: "We should return early if the Transaction is complete.",
			params: params{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							tx := &v1alpha1.Transaction{}
							tx.SetConditions(v1alpha1.TransactionComplete())
							tx.DeepCopyInto(obj.(*v1alpha1.Transaction))
							return nil
						}),
					},
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"RetryLimitReached": {
			reason: "We should return early if the Transaction retry limit was reached.",
			params: params{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							tx := &v1alpha1.Transaction{
								Spec: v1alpha1.TransactionSpec{
									RetryLimit: ptr.To[int64](3),
								},
								Status: v1alpha1.TransactionStatus{
									Failures: 3,
								},
							}
							tx.DeepCopyInto(obj.(*v1alpha1.Transaction))
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"UpdateStatusToRunningError": {
			reason: "We should return an error if we can't update the Transaction's status to indicate it's running.",
			params: params{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							tx := &v1alpha1.Transaction{
								Spec: v1alpha1.TransactionSpec{
									Change: v1alpha1.ChangeTypeInstall,
									Install: &v1alpha1.InstallSpec{
										Package: v1alpha1.PackageSnapshot{
											Spec: v1alpha1.PackageSnapshotSpec{
												PackageSpec: v1.PackageSpec{
													Package: "xpkg.io/test/pkg:v1.0.0",
												},
											},
										},
									},
								},
							}
							tx.DeepCopyInto(obj.(*v1alpha1.Transaction))
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(errors.New("boom")),
					},
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: cmpopts.AnyError,
			},
		},
		"LockAcquireError": {
			reason: "We should return an error if we can't acquire the lock.",
			params: params{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							tx := &v1alpha1.Transaction{
								Spec: v1alpha1.TransactionSpec{
									Change: v1alpha1.ChangeTypeInstall,
									Install: &v1alpha1.InstallSpec{
										Package: v1alpha1.PackageSnapshot{
											Spec: v1alpha1.PackageSnapshotSpec{
												PackageSpec: v1.PackageSpec{
													Package: "xpkg.io/test/pkg:v1.0.0",
												},
											},
										},
									},
								},
							}
							tx.DeepCopyInto(obj.(*v1alpha1.Transaction))
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
				},
				opts: []ReconcilerOption{
					WithLockManager(&MockLockManager{
						MockAcquire: func(_ context.Context, _ *v1alpha1.Transaction) ([]v1beta1.LockPackage, error) {
							return nil, errors.New("boom")
						},
						MockRelease: func(_ context.Context, _ *v1alpha1.Transaction) error {
							return nil
						},
					}),
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: cmpopts.AnyError,
			},
		},
		"LockHeldByAnotherTransaction": {
			reason: "We should update status and return if the lock is held by another transaction.",
			params: params{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							tx := &v1alpha1.Transaction{
								Spec: v1alpha1.TransactionSpec{
									Change: v1alpha1.ChangeTypeInstall,
									Install: &v1alpha1.InstallSpec{
										Package: v1alpha1.PackageSnapshot{
											Spec: v1alpha1.PackageSnapshotSpec{
												PackageSpec: v1.PackageSpec{
													Package: "xpkg.io/test/pkg:v1.0.0",
												},
											},
										},
									},
								},
							}
							tx.DeepCopyInto(obj.(*v1alpha1.Transaction))
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
				},
				opts: []ReconcilerOption{
					WithLockManager(&MockLockManager{
						MockAcquire: func(_ context.Context, _ *v1alpha1.Transaction) ([]v1beta1.LockPackage, error) {
							return nil, ErrLockHeldByAnotherTransaction
						},
						MockRelease: func(_ context.Context, _ *v1alpha1.Transaction) error {
							return nil
						},
					}),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"DependencySolveError": {
			reason: "We should return an error if dependency solving fails.",
			params: params{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							tx := &v1alpha1.Transaction{
								Spec: v1alpha1.TransactionSpec{
									Change: v1alpha1.ChangeTypeInstall,
									Install: &v1alpha1.InstallSpec{
										Package: v1alpha1.PackageSnapshot{
											Spec: v1alpha1.PackageSnapshotSpec{
												PackageSpec: v1.PackageSpec{
													Package: "xpkg.io/test/pkg:v1.0.0",
												},
											},
										},
									},
								},
							}
							tx.DeepCopyInto(obj.(*v1alpha1.Transaction))
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
				},
				opts: []ReconcilerOption{
					WithLockManager(&MockLockManager{
						MockAcquire: func(_ context.Context, _ *v1alpha1.Transaction) ([]v1beta1.LockPackage, error) {
							return []v1beta1.LockPackage{}, nil
						},
						MockRelease: func(_ context.Context, _ *v1alpha1.Transaction) error {
							return nil
						},
					}),
					WithDependencySolver(&MockDependencySolver{
						MockSolve: func(_ context.Context, _ string, _ []v1beta1.LockPackage) ([]v1beta1.LockPackage, error) {
							return nil, errors.New("boom")
						},
					}),
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: cmpopts.AnyError,
			},
		},
		"ValidationError": {
			reason: "We should return an error if validation fails.",
			params: params{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							tx := &v1alpha1.Transaction{
								Spec: v1alpha1.TransactionSpec{
									Change: v1alpha1.ChangeTypeInstall,
									Install: &v1alpha1.InstallSpec{
										Package: v1alpha1.PackageSnapshot{
											Spec: v1alpha1.PackageSnapshotSpec{
												PackageSpec: v1.PackageSpec{
													Package: "xpkg.io/test/pkg:v1.0.0",
												},
											},
										},
									},
								},
							}
							tx.DeepCopyInto(obj.(*v1alpha1.Transaction))
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
				},
				opts: []ReconcilerOption{
					WithLockManager(&MockLockManager{
						MockAcquire: func(_ context.Context, _ *v1alpha1.Transaction) ([]v1beta1.LockPackage, error) {
							return []v1beta1.LockPackage{}, nil
						},
						MockRelease: func(_ context.Context, _ *v1alpha1.Transaction) error {
							return nil
						},
					}),
					WithDependencySolver(&MockDependencySolver{
						MockSolve: func(_ context.Context, _ string, _ []v1beta1.LockPackage) ([]v1beta1.LockPackage, error) {
							return []v1beta1.LockPackage{}, nil
						},
					}),
					WithValidator(&MockValidator{
						MockValidate: func(_ context.Context, _ *v1alpha1.Transaction) error {
							return errors.New("boom")
						},
					}),
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: cmpopts.AnyError,
			},
		},
		"InstallationError": {
			reason: "We should return an error if package installation fails.",
			params: params{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							tx := &v1alpha1.Transaction{
								Spec: v1alpha1.TransactionSpec{
									Change: v1alpha1.ChangeTypeInstall,
									Install: &v1alpha1.InstallSpec{
										Package: v1alpha1.PackageSnapshot{
											Spec: v1alpha1.PackageSnapshotSpec{
												PackageSpec: v1.PackageSpec{
													Package: "xpkg.io/test/pkg:v1.0.0",
												},
											},
										},
									},
								},
								Status: v1alpha1.TransactionStatus{
									ProposedLockPackages: []v1beta1.LockPackage{{
										Source:  "xpkg.io/test/pkg",
										Version: "v1.0.0",
									}},
								},
							}
							tx.DeepCopyInto(obj.(*v1alpha1.Transaction))
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
				},
				pkg: &MockXpkgClient{
					MockGet: func(_ context.Context, _ string, _ ...xpkg.GetOption) (*xpkg.Package, error) {
						return &xpkg.Package{}, nil
					},
				},
				opts: []ReconcilerOption{
					WithLockManager(&MockLockManager{
						MockAcquire: func(_ context.Context, _ *v1alpha1.Transaction) ([]v1beta1.LockPackage, error) {
							return []v1beta1.LockPackage{}, nil
						},
						MockRelease: func(_ context.Context, _ *v1alpha1.Transaction) error {
							return nil
						},
					}),
					WithDependencySolver(&MockDependencySolver{
						MockSolve: func(_ context.Context, _ string, _ []v1beta1.LockPackage) ([]v1beta1.LockPackage, error) {
							return []v1beta1.LockPackage{{
								Source:  "xpkg.io/test/pkg",
								Version: "v1.0.0",
							}}, nil
						},
					}),
					WithValidator(&MockValidator{
						MockValidate: func(_ context.Context, _ *v1alpha1.Transaction) error {
							return nil
						},
					}),
					WithInstaller(&MockInstaller{
						MockInstall: func(_ context.Context, _ *v1alpha1.Transaction, _ *xpkg.Package, _ string) error {
							return errors.New("boom")
						},
					}),
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: cmpopts.AnyError,
			},
		},
		"LockCommitError": {
			reason: "We should return an error if we can't commit the lock.",
			params: params{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							tx := &v1alpha1.Transaction{
								Spec: v1alpha1.TransactionSpec{
									Change: v1alpha1.ChangeTypeInstall,
									Install: &v1alpha1.InstallSpec{
										Package: v1alpha1.PackageSnapshot{
											Spec: v1alpha1.PackageSnapshotSpec{
												PackageSpec: v1.PackageSpec{
													Package: "xpkg.io/test/pkg:v1.0.0",
												},
											},
										},
									},
								},
							}
							tx.DeepCopyInto(obj.(*v1alpha1.Transaction))
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
				},
				opts: []ReconcilerOption{
					WithLockManager(&MockLockManager{
						MockAcquire: func(_ context.Context, _ *v1alpha1.Transaction) ([]v1beta1.LockPackage, error) {
							return []v1beta1.LockPackage{}, nil
						},
						MockCommit: func(_ context.Context, _ *v1alpha1.Transaction, _ []v1beta1.LockPackage) error {
							return errors.New("boom")
						},
						MockRelease: func(_ context.Context, _ *v1alpha1.Transaction) error {
							return nil
						},
					}),
					WithDependencySolver(&MockDependencySolver{
						MockSolve: func(_ context.Context, _ string, _ []v1beta1.LockPackage) ([]v1beta1.LockPackage, error) {
							return []v1beta1.LockPackage{}, nil
						},
					}),
					WithValidator(&MockValidator{
						MockValidate: func(_ context.Context, _ *v1alpha1.Transaction) error {
							return nil
						},
					}),
					WithInstaller(&MockInstaller{
						MockInstall: func(_ context.Context, _ *v1alpha1.Transaction, _ *xpkg.Package, _ string) error {
							return nil
						},
					}),
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: cmpopts.AnyError,
			},
		},
		"Success": {
			reason: "We should successfully complete a transaction.",
			params: params{
				mgr: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							tx := &v1alpha1.Transaction{
								Spec: v1alpha1.TransactionSpec{
									Change: v1alpha1.ChangeTypeInstall,
									Install: &v1alpha1.InstallSpec{
										Package: v1alpha1.PackageSnapshot{
											Spec: v1alpha1.PackageSnapshotSpec{
												PackageSpec: v1.PackageSpec{
													Package: "xpkg.io/test/pkg:v1.0.0",
												},
											},
										},
									},
								},
								Status: v1alpha1.TransactionStatus{
									ProposedLockPackages: []v1beta1.LockPackage{{
										Source:  "xpkg.io/test/pkg",
										Version: "v1.0.0",
									}},
								},
							}
							tx.DeepCopyInto(obj.(*v1alpha1.Transaction))
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
				},
				pkg: &MockXpkgClient{
					MockGet: func(_ context.Context, _ string, _ ...xpkg.GetOption) (*xpkg.Package, error) {
						return &xpkg.Package{}, nil
					},
				},
				opts: []ReconcilerOption{
					WithLockManager(&MockLockManager{
						MockAcquire: func(_ context.Context, _ *v1alpha1.Transaction) ([]v1beta1.LockPackage, error) {
							return []v1beta1.LockPackage{}, nil
						},
						MockCommit: func(_ context.Context, _ *v1alpha1.Transaction, _ []v1beta1.LockPackage) error {
							return nil
						},
						MockRelease: func(_ context.Context, _ *v1alpha1.Transaction) error {
							return nil
						},
					}),
					WithDependencySolver(&MockDependencySolver{
						MockSolve: func(_ context.Context, _ string, _ []v1beta1.LockPackage) ([]v1beta1.LockPackage, error) {
							return []v1beta1.LockPackage{{
								Source:  "xpkg.io/test/pkg",
								Version: "v1.0.0",
							}}, nil
						},
					}),
					WithValidator(&MockValidator{
						MockValidate: func(_ context.Context, _ *v1alpha1.Transaction) error {
							return nil
						},
					}),
					WithInstaller(&MockInstaller{
						MockInstall: func(_ context.Context, _ *v1alpha1.Transaction, _ *xpkg.Package, _ string) error {
							return nil
						},
					}),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			pkg := tc.params.pkg
			if pkg == nil {
				pkg = &MockXpkgClient{
					MockGet: func(_ context.Context, _ string, _ ...xpkg.GetOption) (*xpkg.Package, error) {
						return nil, errors.New("not implemented")
					},
				}
			}
			r := NewReconciler(tc.params.mgr.GetClient(), pkg, tc.params.opts...)
			got, err := r.Reconcile(context.Background(), reconcile.Request{})

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nReconcile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.r, got); diff != "" {
				t.Errorf("\n%s\nReconcile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
