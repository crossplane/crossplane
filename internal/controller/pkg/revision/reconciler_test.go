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
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/parser"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	verfake "github.com/crossplane/crossplane/internal/version/fake"
	"github.com/crossplane/crossplane/internal/xpkg"
	xpkgfake "github.com/crossplane/crossplane/internal/xpkg/fake"
)

var _ parser.Backend = &ErrBackend{}

type ErrBackend struct{}

func (e *ErrBackend) Init(_ context.Context, _ ...parser.BackendOption) (io.ReadCloser, error) {
	return nil, errors.New("test err")
}

var _ Establisher = &MockEstablisher{}

type MockEstablisher struct {
	MockEstablish func() ([]xpv1.TypedReference, error)
}

func NewMockEstablisher() *MockEstablisher {
	return &MockEstablisher{
		MockEstablish: NewMockEstablishFn(nil, nil),
	}
}

func NewMockEstablishFn(refs []xpv1.TypedReference, err error) func() ([]xpv1.TypedReference, error) {
	return func() ([]xpv1.TypedReference, error) { return refs, err }
}

func (e *MockEstablisher) Establish(context.Context, []runtime.Object, resource.Object, bool) ([]xpv1.TypedReference, error) {
	return e.MockEstablish()
}

var _ Hooks = &MockHook{}

type MockHook struct {
	MockPre  func() error
	MockPost func() error
}

func NewMockPreFn(err error) func() error {
	return func() error { return err }
}

func NewMockPostFn(err error) func() error {
	return func() error { return err }
}

func (h *MockHook) Pre(context.Context, runtime.Object, v1.PackageRevision) error {
	return h.MockPre()
}

func (h *MockHook) Post(context.Context, runtime.Object, v1.PackageRevision) error {
	return h.MockPost()
}

var _ parser.Linter = &MockLinter{}

type MockLinter struct {
	MockLint func() error
}

func NewMockLintFn(err error) func() error {
	return func() error { return err }
}

func (m *MockLinter) Lint(*parser.Package) error {
	return m.MockLint()
}

type MockDependencyManager struct {
	MockResolve    func() (int, int, int, error)
	MockRemoveSelf func() error
}

func NewMockResolveFn(total, installed, invalid int, err error) func() (int, int, int, error) {
	return func() (int, int, int, error) { return total, installed, invalid, err }
}

func NewMockRemoveSelfFn(err error) func() error {
	return func() error { return err }
}

func (m *MockDependencyManager) Resolve(ctx context.Context, pkg runtime.Object, pr v1.PackageRevision) (int, int, int, error) {
	return m.MockResolve()
}

func (m *MockDependencyManager) RemoveSelf(ctx context.Context, pr v1.PackageRevision) error {
	return m.MockRemoveSelf()
}

var providerBytes = []byte(`apiVersion: meta.pkg.crossplane.io/v1alpha1
kind: Provider
metadata:
  name: test
spec:
  controller:
    image: crossplane/provider-test-controller:v0.0.1
  crossplane:
    version: ">v0.13.0"`)

func TestReconcile(t *testing.T) {
	errBoom := errors.New("boom")
	now := metav1.Now()
	trueVal := true

	metaScheme, _ := xpkg.BuildMetaScheme()
	objScheme, _ := xpkg.BuildObjectScheme()

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
		"PackageRevisionNotFound": {
			reason: "We should not return and error and not requeue if package not found.",
			args: args{
				mgr: &fake.Manager{},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ConfigurationRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, ""))},
					}),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"ErrGetPackageRevision": {
			reason: "We should return an error if getting package fails.",
			args: args{
				mgr: &fake.Manager{},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ConfigurationRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
					}),
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: errors.Wrap(errBoom, errGetPackageRevision),
			},
		},
		"ErrDeletedClearCache": {
			reason: "We should requeue after short wait if revision is deleted and we fail to clear image cache.",
			args: args{
				mgr: &fake.Manager{},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithCache(&xpkgfake.MockCache{
						MockDelete: xpkgfake.NewMockCacheDeleteFn(errBoom),
					}),
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ConfigurationRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1.ConfigurationRevision)
								pr.SetGroupVersionKind(v1.ConfigurationRevisionGroupVersionKind)
								pr.SetDeletionTimestamp(&now)
								return nil
							}),
						},
					}),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"ErrDeletedRemoveSelf": {
			reason: "We should requeue after short wait if revision is deleted and we fail to remove it from package Lock.",
			args: args{
				mgr: &fake.Manager{},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ConfigurationRevision{} }),
					WithDependencyManager(&MockDependencyManager{
						MockRemoveSelf: NewMockRemoveSelfFn(errBoom),
					}),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1.ConfigurationRevision)
								pr.SetGroupVersionKind(v1.ConfigurationRevisionGroupVersionKind)
								pr.SetDeletionTimestamp(&now)
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1.ConfigurationRevision{}
								want.SetGroupVersionKind(v1.ConfigurationRevisionGroupVersionKind)
								want.SetDeletionTimestamp(&now)
								want.SetConditions(v1.Unhealthy())

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
						},
					}),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"ErrDeletedRemoveFinalizer": {
			reason: "We should requeue after short wait if revision is deleted and we fail to remove finalizer.",
			args: args{
				mgr: &fake.Manager{},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ConfigurationRevision{} }),
					WithDependencyManager(&MockDependencyManager{
						MockRemoveSelf: NewMockRemoveSelfFn(nil),
					}),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1.ConfigurationRevision)
								pr.SetGroupVersionKind(v1.ConfigurationRevisionGroupVersionKind)
								pr.SetDeletionTimestamp(&now)
								return nil
							}),
						},
					}),
					WithFinalizer(resource.FinalizerFns{RemoveFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return errBoom
					}}),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"SuccessfulDeleted": {
			reason: "We should not requeue if revision is deleted and we successfully remove finalizer.",
			args: args{
				mgr: &fake.Manager{},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ConfigurationRevision{} }),
					WithDependencyManager(&MockDependencyManager{
						MockRemoveSelf: NewMockRemoveSelfFn(nil),
					}),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1.ConfigurationRevision)
								pr.SetGroupVersionKind(v1.ConfigurationRevisionGroupVersionKind)
								pr.SetDeletionTimestamp(&now)
								return nil
							}),
						},
					}),
					WithFinalizer(resource.FinalizerFns{RemoveFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return errBoom
					}}),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"ErrAddFinalizer": {
			reason: "We should requeue after short wait if we fail to add finalizer.",
			args: args{
				mgr: &fake.Manager{},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ConfigurationRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil),
						},
					}),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return errBoom
					}}),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"ErrInitParserBackend": {
			reason: "We should requeue after short wait if we fail to initialize parser backend.",
			args: args{
				mgr: &fake.Manager{},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ConfigurationRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1.ConfigurationRevision)
								pr.SetGroupVersionKind(v1.ConfigurationRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionActive)
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1.ConfigurationRevision{}
								want.SetGroupVersionKind(v1.ConfigurationRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionActive)
								want.SetConditions(v1.Unhealthy())

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
						},
					}),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return nil
					}}),
					WithParserBackend(&ErrBackend{}),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"ErrParse": {
			reason: "We should requeue after short wait if fail to parse package.",
			args: args{
				mgr: &fake.Manager{},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ConfigurationRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1.ConfigurationRevision)
								pr.SetGroupVersionKind(v1.ConfigurationRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionActive)
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1.ConfigurationRevision{}
								want.SetGroupVersionKind(v1.ConfigurationRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionActive)
								want.SetConditions(v1.Unhealthy())

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
						},
					}),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return nil
					}}),
					WithParser(parser.New(runtime.NewScheme(), runtime.NewScheme())),
					WithParserBackend(parser.NewEchoBackend(string(providerBytes))),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"ErrLint": {
			reason: "We should requeue after long wait if linting returns an error.",
			args: args{
				mgr: &fake.Manager{},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ConfigurationRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1.ConfigurationRevision)
								pr.SetGroupVersionKind(v1.ConfigurationRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionActive)
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1.ConfigurationRevision{}
								want.SetGroupVersionKind(v1.ConfigurationRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionActive)
								want.SetConditions(v1.Unhealthy())

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
						},
					}),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return nil
					}}),
					WithParser(parser.New(metaScheme, objScheme)),
					WithParserBackend(parser.NewEchoBackend(string(providerBytes))),
					WithLinter(&MockLinter{MockLint: NewMockLintFn(errBoom)}),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: longWait},
			},
		},
		"ErrCrossplaneConstraints": {
			reason: "We should not requeue if Crossplane version is incompatible.",
			args: args{
				mgr: &fake.Manager{},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ConfigurationRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1.ConfigurationRevision)
								pr.SetGroupVersionKind(v1.ConfigurationRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionActive)
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1.ConfigurationRevision{}
								want.SetGroupVersionKind(v1.ConfigurationRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionActive)
								want.SetConditions(v1.Unhealthy())

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
						},
					}),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return nil
					}}),
					WithParser(parser.New(metaScheme, objScheme)),
					WithParserBackend(parser.NewEchoBackend(string(providerBytes))),
					WithLinter(&MockLinter{MockLint: NewMockLintFn(nil)}),
					WithVersioner(&verfake.MockVersioner{
						MockInConstraints:    verfake.NewMockInConstraintsFn(false, errBoom),
						MockGetVersionString: verfake.NewMockGetVersionStringFn("v0.11.0"),
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"ErrOneMeta": {
			reason: "We should requeue after long wait if not exactly one meta package type.",
			args: args{
				mgr: &fake.Manager{},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ConfigurationRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1.ConfigurationRevision)
								pr.SetGroupVersionKind(v1.ConfigurationRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionActive)
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1.ConfigurationRevision{}
								want.SetGroupVersionKind(v1.ConfigurationRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionActive)
								want.SetConditions(v1.Unhealthy())

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
						},
					}),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return nil
					}}),
					WithParser(parser.New(metaScheme, objScheme)),
					WithParserBackend(parser.NewNopBackend()),
					WithLinter(&MockLinter{MockLint: NewMockLintFn(nil)}),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: longWait},
			},
		},
		"ErrResolveDependencies": {
			reason: "We should requeue after short wait if we fail to resolve dependencies.",
			args: args{
				mgr: &fake.Manager{},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ProviderRevision{} }),
					WithDependencyManager(&MockDependencyManager{
						MockResolve: NewMockResolveFn(0, 0, 0, errBoom),
					}),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1.ProviderRevision)
								pr.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionActive)
								pr.SetSkipDependencyResolution(pointer.BoolPtr(false))
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1.ProviderRevision{}
								want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionActive)
								want.SetSkipDependencyResolution(pointer.BoolPtr(false))
								want.SetConditions(v1.UnknownHealth())

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
						},
					}),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return nil
					}}),
					WithParser(parser.New(metaScheme, objScheme)),
					WithParserBackend(parser.NewEchoBackend(string(providerBytes))),
					WithLinter(&MockLinter{MockLint: NewMockLintFn(nil)}),
					WithVersioner(&verfake.MockVersioner{MockInConstraints: verfake.NewMockInConstraintsFn(true, nil)}),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"ErrPreHook": {
			reason: "We should requeue after short wait if pre establishment hook returns an error.",
			args: args{
				mgr: &fake.Manager{},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ProviderRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1.ProviderRevision)
								pr.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionActive)
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1.ProviderRevision{}
								want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionActive)
								want.SetConditions(v1.Unhealthy())

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
						},
					}),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return nil
					}}),
					WithHooks(&MockHook{
						MockPre: NewMockPreFn(errBoom),
					}),
					WithParser(parser.New(metaScheme, objScheme)),
					WithParserBackend(parser.NewEchoBackend(string(providerBytes))),
					WithLinter(&MockLinter{MockLint: NewMockLintFn(nil)}),
					WithVersioner(&verfake.MockVersioner{MockInConstraints: verfake.NewMockInConstraintsFn(true, nil)}),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"ErrPostHook": {
			reason: "We should requeue after short wait if post establishment hook returns an error.",
			args: args{
				mgr: &fake.Manager{},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ProviderRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1.ProviderRevision)
								pr.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionActive)
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1.ProviderRevision{}
								want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionActive)
								want.SetConditions(v1.Unhealthy())

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
						},
					}),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return nil
					}}),
					WithHooks(&MockHook{
						MockPre:  NewMockPreFn(nil),
						MockPost: NewMockPostFn(errBoom),
					}),
					WithEstablisher(NewMockEstablisher()),
					WithParser(parser.New(metaScheme, objScheme)),
					WithParserBackend(parser.NewEchoBackend(string(providerBytes))),
					WithLinter(&MockLinter{MockLint: NewMockLintFn(nil)}),
					WithVersioner(&verfake.MockVersioner{MockInConstraints: verfake.NewMockInConstraintsFn(true, nil)}),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"SuccessfulActiveRevision": {
			reason: "An active revision should establish control of all of its resources.",
			args: args{
				mgr: &fake.Manager{},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ConfigurationRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1.ConfigurationRevision)
								pr.SetGroupVersionKind(v1.ConfigurationRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionActive)
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1.ConfigurationRevision{}
								want.SetGroupVersionKind(v1.ConfigurationRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionActive)
								want.SetConditions(v1.Healthy())

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
							MockDelete: test.NewMockDeleteFn(nil),
						},
					}),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return nil
					}}),
					WithHooks(NewNopHooks()),
					WithEstablisher(NewMockEstablisher()),
					WithParser(parser.New(metaScheme, objScheme)),
					WithParserBackend(parser.NewEchoBackend(string(providerBytes))),
					WithLinter(&MockLinter{MockLint: NewMockLintFn(nil)}),
					WithVersioner(&verfake.MockVersioner{MockInConstraints: verfake.NewMockInConstraintsFn(true, nil)}),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: longWait},
			},
		},
		"SuccessfulActiveRevisionIgnoreConstraints": {
			reason: "An active revision with incompatible Crossplane version should install successfully when constraints ignored.",
			args: args{
				mgr: &fake.Manager{},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ConfigurationRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1.ConfigurationRevision)
								pr.SetGroupVersionKind(v1.ConfigurationRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionActive)
								pr.SetIgnoreCrossplaneConstraints(&trueVal)
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1.ConfigurationRevision{}
								want.SetGroupVersionKind(v1.ConfigurationRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionActive)
								want.SetConditions(v1.Healthy())
								want.SetIgnoreCrossplaneConstraints(&trueVal)

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
							MockDelete: test.NewMockDeleteFn(nil),
						},
					}),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return nil
					}}),
					WithHooks(NewNopHooks()),
					WithEstablisher(NewMockEstablisher()),
					WithParser(parser.New(metaScheme, objScheme)),
					WithParserBackend(parser.NewEchoBackend(string(providerBytes))),
					WithLinter(&MockLinter{MockLint: NewMockLintFn(nil)}),
					WithVersioner(&verfake.MockVersioner{MockInConstraints: verfake.NewMockInConstraintsFn(false, nil)}),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: longWait},
			},
		},
		"ErrEstablishActiveRevision": {
			reason: "An active revision that fails to establish control should requeue after short wait.",
			args: args{
				mgr: &fake.Manager{},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ProviderRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1.ProviderRevision)
								pr.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionActive)
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1.ProviderRevision{}
								want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionActive)
								want.SetConditions(v1.Unhealthy())

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
							MockDelete: test.NewMockDeleteFn(nil),
						},
					}),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return nil
					}}),
					WithHooks(NewNopHooks()),
					WithEstablisher(&MockEstablisher{
						MockEstablish: NewMockEstablishFn(nil, errBoom),
					}),
					WithParser(parser.New(metaScheme, objScheme)),
					WithParserBackend(parser.NewEchoBackend(string(providerBytes))),
					WithLinter(&MockLinter{MockLint: NewMockLintFn(nil)}),
					WithVersioner(&verfake.MockVersioner{MockInConstraints: verfake.NewMockInConstraintsFn(true, nil)}),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"SuccessfulInactiveRevision": {
			reason: "An inactive revision should establish ownership of all of its resources.",
			args: args{
				mgr: &fake.Manager{},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ConfigurationRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1.ConfigurationRevision)
								pr.SetGroupVersionKind(v1.ConfigurationRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionInactive)
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1.ConfigurationRevision{}
								want.SetGroupVersionKind(v1.ConfigurationRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionInactive)
								want.SetConditions(v1.Healthy())

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
							MockDelete: test.NewMockDeleteFn(nil),
						},
					}),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return nil
					}}),
					WithHooks(NewNopHooks()),
					WithEstablisher(NewMockEstablisher()),
					WithParser(parser.New(metaScheme, objScheme)),
					WithParserBackend(parser.NewEchoBackend(string(providerBytes))),
					WithLinter(&MockLinter{MockLint: NewMockLintFn(nil)}),
					WithVersioner(&verfake.MockVersioner{MockInConstraints: verfake.NewMockInConstraintsFn(true, nil)}),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: longWait},
			},
		},
		"ErrEstablishInactiveRevision": {
			reason: "An inactive revision that fails to establish ownership should requeue after short wait.",
			args: args{
				mgr: &fake.Manager{},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ConfigurationRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1.ConfigurationRevision)
								pr.SetGroupVersionKind(v1.ConfigurationRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionInactive)
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1.ConfigurationRevision{}
								want.SetGroupVersionKind(v1.ConfigurationRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionInactive)
								want.SetConditions(v1.Unhealthy())

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
							MockDelete: test.NewMockDeleteFn(nil),
						},
					}),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return nil
					}}),
					WithHooks(NewNopHooks()),
					WithEstablisher(&MockEstablisher{
						MockEstablish: NewMockEstablishFn(nil, errBoom),
					}),
					WithParser(parser.New(metaScheme, objScheme)),
					WithParserBackend(parser.NewEchoBackend(string(providerBytes))),
					WithLinter(&MockLinter{MockLint: NewMockLintFn(nil)}),
					WithVersioner(&verfake.MockVersioner{MockInConstraints: verfake.NewMockInConstraintsFn(true, nil)}),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: shortWait},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewReconciler(tc.args.mgr, tc.args.rec...)
			got, err := r.Reconcile(reconcile.Request{})

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.r, got, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
