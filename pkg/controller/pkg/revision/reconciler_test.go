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
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/parser"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
	verfake "github.com/crossplane/crossplane/pkg/version/fake"
	"github.com/crossplane/crossplane/pkg/xpkg"
	xpkgfake "github.com/crossplane/crossplane/pkg/xpkg/fake"
)

var _ parser.Backend = &ErrBackend{}

type ErrBackend struct{}

func (e *ErrBackend) Init(_ context.Context, _ ...parser.BackendOption) (io.ReadCloser, error) {
	return nil, errors.New("test err")
}

var _ Establisher = &MockEstablisher{}

type MockEstablisher struct {
	MockEstablish func() ([]runtimev1alpha1.TypedReference, error)
}

func NewMockEstablisher() *MockEstablisher {
	return &MockEstablisher{
		MockEstablish: NewMockEstablishFn(nil, nil),
	}
}

func NewMockEstablishFn(refs []runtimev1alpha1.TypedReference, err error) func() ([]runtimev1alpha1.TypedReference, error) {
	return func() ([]runtimev1alpha1.TypedReference, error) { return refs, err }
}

func (e *MockEstablisher) Establish(context.Context, []runtime.Object, resource.Object, bool) ([]runtimev1alpha1.TypedReference, error) {
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

func (h *MockHook) Pre(context.Context, runtime.Object, v1alpha1.PackageRevision) error {
	return h.MockPre()
}

func (h *MockHook) Post(context.Context, runtime.Object, v1alpha1.PackageRevision) error {
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
					WithNewPackageRevisionFn(func() v1alpha1.PackageRevision { return &v1alpha1.ConfigurationRevision{} }),
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
					WithNewPackageRevisionFn(func() v1alpha1.PackageRevision { return &v1alpha1.ConfigurationRevision{} }),
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
					WithNewPackageRevisionFn(func() v1alpha1.PackageRevision { return &v1alpha1.ConfigurationRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1alpha1.ConfigurationRevision)
								pr.SetGroupVersionKind(v1alpha1.ConfigurationRevisionGroupVersionKind)
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
		"ErrDeletedRemoveFinalizer": {
			reason: "We should requeue after short wait if revision is deleted and we fail to remove finalizer.",
			args: args{
				mgr: &fake.Manager{},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1alpha1.PackageRevision { return &v1alpha1.ConfigurationRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1alpha1.ConfigurationRevision)
								pr.SetGroupVersionKind(v1alpha1.ConfigurationRevisionGroupVersionKind)
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
					WithNewPackageRevisionFn(func() v1alpha1.PackageRevision { return &v1alpha1.ConfigurationRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1alpha1.ConfigurationRevision)
								pr.SetGroupVersionKind(v1alpha1.ConfigurationRevisionGroupVersionKind)
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
					WithNewPackageRevisionFn(func() v1alpha1.PackageRevision { return &v1alpha1.ConfigurationRevision{} }),
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
					WithNewPackageRevisionFn(func() v1alpha1.PackageRevision { return &v1alpha1.ConfigurationRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1alpha1.ConfigurationRevision)
								pr.SetGroupVersionKind(v1alpha1.ConfigurationRevisionGroupVersionKind)
								pr.SetDesiredState(v1alpha1.PackageRevisionActive)
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1alpha1.ConfigurationRevision{}
								want.SetGroupVersionKind(v1alpha1.ConfigurationRevisionGroupVersionKind)
								want.SetDesiredState(v1alpha1.PackageRevisionActive)
								want.SetConditions(v1alpha1.Unhealthy())

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
					WithNewPackageRevisionFn(func() v1alpha1.PackageRevision { return &v1alpha1.ConfigurationRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1alpha1.ConfigurationRevision)
								pr.SetGroupVersionKind(v1alpha1.ConfigurationRevisionGroupVersionKind)
								pr.SetDesiredState(v1alpha1.PackageRevisionActive)
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1alpha1.ConfigurationRevision{}
								want.SetGroupVersionKind(v1alpha1.ConfigurationRevisionGroupVersionKind)
								want.SetDesiredState(v1alpha1.PackageRevisionActive)
								want.SetConditions(v1alpha1.Unhealthy())

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
					WithNewPackageRevisionFn(func() v1alpha1.PackageRevision { return &v1alpha1.ConfigurationRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1alpha1.ConfigurationRevision)
								pr.SetGroupVersionKind(v1alpha1.ConfigurationRevisionGroupVersionKind)
								pr.SetDesiredState(v1alpha1.PackageRevisionActive)
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1alpha1.ConfigurationRevision{}
								want.SetGroupVersionKind(v1alpha1.ConfigurationRevisionGroupVersionKind)
								want.SetDesiredState(v1alpha1.PackageRevisionActive)
								want.SetConditions(v1alpha1.Unhealthy())

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
					WithNewPackageRevisionFn(func() v1alpha1.PackageRevision { return &v1alpha1.ConfigurationRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1alpha1.ConfigurationRevision)
								pr.SetGroupVersionKind(v1alpha1.ConfigurationRevisionGroupVersionKind)
								pr.SetDesiredState(v1alpha1.PackageRevisionActive)
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1alpha1.ConfigurationRevision{}
								want.SetGroupVersionKind(v1alpha1.ConfigurationRevisionGroupVersionKind)
								want.SetDesiredState(v1alpha1.PackageRevisionActive)
								want.SetConditions(v1alpha1.Unhealthy())

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
					WithNewPackageRevisionFn(func() v1alpha1.PackageRevision { return &v1alpha1.ConfigurationRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1alpha1.ConfigurationRevision)
								pr.SetGroupVersionKind(v1alpha1.ConfigurationRevisionGroupVersionKind)
								pr.SetDesiredState(v1alpha1.PackageRevisionActive)
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1alpha1.ConfigurationRevision{}
								want.SetGroupVersionKind(v1alpha1.ConfigurationRevisionGroupVersionKind)
								want.SetDesiredState(v1alpha1.PackageRevisionActive)
								want.SetConditions(v1alpha1.Unhealthy())

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
		"ErrPreHook": {
			reason: "We should requeue after short wait if pre establishment hook returns an error.",
			args: args{
				mgr: &fake.Manager{},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1alpha1.PackageRevision { return &v1alpha1.ProviderRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1alpha1.ProviderRevision)
								pr.SetGroupVersionKind(v1alpha1.ProviderRevisionGroupVersionKind)
								pr.SetDesiredState(v1alpha1.PackageRevisionActive)
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1alpha1.ProviderRevision{}
								want.SetGroupVersionKind(v1alpha1.ProviderRevisionGroupVersionKind)
								want.SetDesiredState(v1alpha1.PackageRevisionActive)
								want.SetConditions(v1alpha1.Unhealthy())

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
					WithNewPackageRevisionFn(func() v1alpha1.PackageRevision { return &v1alpha1.ProviderRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1alpha1.ProviderRevision)
								pr.SetGroupVersionKind(v1alpha1.ProviderRevisionGroupVersionKind)
								pr.SetDesiredState(v1alpha1.PackageRevisionActive)
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1alpha1.ProviderRevision{}
								want.SetGroupVersionKind(v1alpha1.ProviderRevisionGroupVersionKind)
								want.SetDesiredState(v1alpha1.PackageRevisionActive)
								want.SetConditions(v1alpha1.Unhealthy())

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
					WithNewPackageRevisionFn(func() v1alpha1.PackageRevision { return &v1alpha1.ConfigurationRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1alpha1.ConfigurationRevision)
								pr.SetGroupVersionKind(v1alpha1.ConfigurationRevisionGroupVersionKind)
								pr.SetDesiredState(v1alpha1.PackageRevisionActive)
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1alpha1.ConfigurationRevision{}
								want.SetGroupVersionKind(v1alpha1.ConfigurationRevisionGroupVersionKind)
								want.SetDesiredState(v1alpha1.PackageRevisionActive)
								want.SetConditions(v1alpha1.Healthy())

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
				r: reconcile.Result{Requeue: false},
			},
		},
		"SuccessfulActiveRevisionIgnoreConstraints": {
			reason: "An active revision with incompatible Crossplane version should install successfully when constraints ignored.",
			args: args{
				mgr: &fake.Manager{},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1alpha1.PackageRevision { return &v1alpha1.ConfigurationRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1alpha1.ConfigurationRevision)
								pr.SetGroupVersionKind(v1alpha1.ConfigurationRevisionGroupVersionKind)
								pr.SetDesiredState(v1alpha1.PackageRevisionActive)
								pr.SetIgnoreCrossplaneConstraints(&trueVal)
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1alpha1.ConfigurationRevision{}
								want.SetGroupVersionKind(v1alpha1.ConfigurationRevisionGroupVersionKind)
								want.SetDesiredState(v1alpha1.PackageRevisionActive)
								want.SetConditions(v1alpha1.Healthy())
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
				r: reconcile.Result{Requeue: false},
			},
		},
		"ErrEstablishActiveRevision": {
			reason: "An active revision that fails to establish control should requeue after short wait.",
			args: args{
				mgr: &fake.Manager{},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1alpha1.PackageRevision { return &v1alpha1.ProviderRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1alpha1.ProviderRevision)
								pr.SetGroupVersionKind(v1alpha1.ProviderRevisionGroupVersionKind)
								pr.SetDesiredState(v1alpha1.PackageRevisionActive)
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1alpha1.ProviderRevision{}
								want.SetGroupVersionKind(v1alpha1.ProviderRevisionGroupVersionKind)
								want.SetDesiredState(v1alpha1.PackageRevisionActive)
								want.SetConditions(v1alpha1.Unhealthy())

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
					WithNewPackageRevisionFn(func() v1alpha1.PackageRevision { return &v1alpha1.ConfigurationRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1alpha1.ConfigurationRevision)
								pr.SetGroupVersionKind(v1alpha1.ConfigurationRevisionGroupVersionKind)
								pr.SetDesiredState(v1alpha1.PackageRevisionInactive)
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1alpha1.ConfigurationRevision{}
								want.SetGroupVersionKind(v1alpha1.ConfigurationRevisionGroupVersionKind)
								want.SetDesiredState(v1alpha1.PackageRevisionInactive)
								want.SetConditions(v1alpha1.Healthy())

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
				r: reconcile.Result{Requeue: false},
			},
		},
		"ErrEstablishInactiveRevision": {
			reason: "An inactive revision that fails to establish ownership should requeue after short wait.",
			args: args{
				mgr: &fake.Manager{},
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1alpha1.PackageRevision { return &v1alpha1.ConfigurationRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1alpha1.ConfigurationRevision)
								pr.SetGroupVersionKind(v1alpha1.ConfigurationRevisionGroupVersionKind)
								pr.SetDesiredState(v1alpha1.PackageRevisionInactive)
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1alpha1.ConfigurationRevision{}
								want.SetGroupVersionKind(v1alpha1.ConfigurationRevisionGroupVersionKind)
								want.SetDesiredState(v1alpha1.PackageRevisionInactive)
								want.SetConditions(v1alpha1.Unhealthy())

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
