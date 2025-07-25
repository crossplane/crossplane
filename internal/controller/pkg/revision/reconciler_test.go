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
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/feature"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/parser"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	pkgmetav1 "github.com/crossplane/crossplane/apis/pkg/meta/v1"
	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/crossplane/crossplane/internal/features"
	verfake "github.com/crossplane/crossplane/internal/version/fake"
	"github.com/crossplane/crossplane/internal/xpkg"
	xpkgfake "github.com/crossplane/crossplane/internal/xpkg/fake"
)

var _ parser.Backend = &ErrBackend{}

type ErrBackend struct{ err error }

func (e *ErrBackend) Init(_ context.Context, _ ...parser.BackendOption) (io.ReadCloser, error) {
	return nil, e.err
}

var _ Establisher = &MockEstablisher{}

type MockEstablisher struct {
	MockEstablish  func() ([]xpv1.TypedReference, error)
	MockRelinquish func() error
}

func NewMockEstablisher() *MockEstablisher {
	return &MockEstablisher{
		MockEstablish:  NewMockEstablishFn(nil, nil),
		MockRelinquish: NewMockRelinquishFn(nil),
	}
}

func NewMockEstablishFn(refs []xpv1.TypedReference, err error) func() ([]xpv1.TypedReference, error) {
	return func() ([]xpv1.TypedReference, error) { return refs, err }
}

func NewMockRelinquishFn(err error) func() error {
	return func() error { return err }
}

func (e *MockEstablisher) Establish(context.Context, []runtime.Object, v1.PackageRevision, bool) ([]xpv1.TypedReference, error) {
	return e.MockEstablish()
}

func (e *MockEstablisher) ReleaseObjects(context.Context, v1.PackageRevision) error {
	return e.MockRelinquish()
}

var _ parser.Linter = &MockLinter{}

type MockLinter struct {
	MockLint func() error
}

func NewMockLintFn(err error) func() error {
	return func() error { return err }
}

func (m *MockLinter) Lint(parser.Lintable) error {
	return m.MockLint()
}

type MockParseFn func(context.Context, io.ReadCloser) (*parser.Package, error)

func (fn MockParseFn) Parse(ctx context.Context, r io.ReadCloser) (*parser.Package, error) {
	return fn(ctx, r)
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

func (m *MockDependencyManager) Resolve(_ context.Context, _ pkgmetav1.Pkg, _ v1.PackageRevision) (int, int, int, error) {
	return m.MockResolve()
}

func (m *MockDependencyManager) RemoveSelf(_ context.Context, _ v1.PackageRevision) error {
	return m.MockRemoveSelf()
}

var providerBytes = []byte(`apiVersion: meta.pkg.crossplane.io/v1
kind: Provider
metadata:
  name: test
  annotations:
    author: crossplane
spec:
  controller:
    image: crossplane/provider-test-controller:v0.0.1
  crossplane:
    version: ">v0.13.0"`)

func TestReconcile(t *testing.T) {
	errBoom := errors.New("boom")
	testLog := logging.NewLogrLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(io.Discard)).WithName("testlog"))
	now := metav1.Now()
	pullPolicy := corev1.PullNever
	trueVal := true

	metaScheme, _ := xpkg.BuildMetaScheme()
	objScheme, _ := xpkg.BuildObjectScheme()

	type args struct {
		mgr manager.Manager
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
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ProviderRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, ""))},
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"ErrGetPackageRevision": {
			reason: "We should return an error if getting package fails.",
			args: args{
				mgr: &fake.Manager{},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ProviderRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetPackageRevision),
			},
		},
		"ErrDeletedClearCache": {
			reason: "We should return an error if revision is deleted and we fail to clear image cache.",
			args: args{
				mgr: &fake.Manager{},
				rec: []ReconcilerOption{
					WithCache(&xpkgfake.MockCache{
						MockDelete: xpkgfake.NewMockCacheDeleteFn(errBoom),
					}),
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ProviderRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								pr := o.(*v1.ProviderRevision)
								pr.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								pr.SetDeletionTimestamp(&now)
								return nil
							}),
						},
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errDeleteCache),
			},
		},
		"ErrDeletedRemoveSelf": {
			reason: "We should return an error if revision is deleted and we fail to remove it from package Lock.",
			args: args{
				mgr: &fake.Manager{},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ProviderRevision{} }),
					WithDependencyManager(&MockDependencyManager{
						MockRemoveSelf: NewMockRemoveSelfFn(errBoom),
					}),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								pr := o.(*v1.ProviderRevision)
								pr.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								pr.SetDeletionTimestamp(&now)
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(_ client.Object) error {
								t.Errorf("StatusUpdate should not be called")
								return nil
							}),
						},
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errRemoveLock),
			},
		},
		"ErrDeletedRemoveFinalizer": {
			reason: "We should return an error if revision is deleted and we fail to remove finalizer.",
			args: args{
				mgr: &fake.Manager{},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ProviderRevision{} }),
					WithDependencyManager(&MockDependencyManager{
						MockRemoveSelf: NewMockRemoveSelfFn(nil),
					}),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								pr := o.(*v1.ProviderRevision)
								pr.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
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
				err: errors.Wrap(errBoom, errRemoveFinalizer),
			},
		},
		"SuccessfulDeleted": {
			reason: "We should not requeue if revision is deleted and we successfully remove finalizer.",
			args: args{
				mgr: &fake.Manager{},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ProviderRevision{} }),
					WithDependencyManager(&MockDependencyManager{
						MockRemoveSelf: NewMockRemoveSelfFn(nil),
					}),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								pr := o.(*v1.ProviderRevision)
								pr.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								pr.SetDeletionTimestamp(&now)
								return nil
							}),
						},
					}),
					WithFinalizer(resource.FinalizerFns{RemoveFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return nil
					}}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"ErrAddFinalizer": {
			reason: "We should return an error if we fail to add finalizer.",
			args: args{
				mgr: &fake.Manager{},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ProviderRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil),
						},
					}),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return errBoom
					}}),
					WithConfigStore(&xpkgfake.MockConfigStore{
						MockPullSecretFor: xpkgfake.NewMockConfigStorePullSecretForFn("", "", nil),
						MockRewritePath:   xpkgfake.NewMockRewritePathFn("", "", nil),
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errAddFinalizer),
			},
		},
		"ErrGetFromCacheSuccessfulDelete": {
			reason: "We should return an error if package content is in cache, we cannot get it, but we remove it successfully.",
			args: args{
				mgr: &fake.Manager{},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ProviderRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								pr := o.(*v1.ProviderRevision)
								pr.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionActive)
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(_ client.Object) error {
								t.Errorf("StatusUpdate should not be called")
								return nil
							}),
						},
					}),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return nil
					}}),
					WithCache(&xpkgfake.MockCache{
						MockHas:    xpkgfake.NewMockCacheHasFn(true),
						MockGet:    xpkgfake.NewMockCacheGetFn(nil, errBoom),
						MockDelete: xpkgfake.NewMockCacheDeleteFn(nil),
					}),
					WithConfigStore(&xpkgfake.MockConfigStore{
						MockPullSecretFor: xpkgfake.NewMockConfigStorePullSecretForFn("", "", nil),
						MockRewritePath:   xpkgfake.NewMockRewritePathFn("", "", nil),
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetCache),
			},
		},
		"ErrGetPackagePullSecretFromImageConfigs": {
			reason: "We should return an error if we cannot get package pull secret from image configs.",
			args: args{
				mgr: &fake.Manager{},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ProviderRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								pr := o.(*v1.ProviderRevision)
								pr.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionActive)
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(_ client.Object) error {
								return nil
							}),
						},
					}),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return nil
					}}),
					WithConfigStore(&xpkgfake.MockConfigStore{
						MockPullSecretFor: xpkgfake.NewMockConfigStorePullSecretForFn("", "", errBoom),
						MockRewritePath:   xpkgfake.NewMockRewritePathFn("", "", nil),
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetPullConfig),
			},
		},
		"ErrRewriteImageFromImageConfigs": {
			reason: "We should return an error if we cannot rewrite the package path using image configs.",
			args: args{
				mgr: &fake.Manager{},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ProviderRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								pr := o.(*v1.ProviderRevision)
								pr.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionActive)
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(_ client.Object) error {
								return nil
							}),
						},
					}),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return nil
					}}),
					WithConfigStore(&xpkgfake.MockConfigStore{
						MockPullSecretFor: xpkgfake.NewMockConfigStorePullSecretForFn("", "", nil),
						MockRewritePath:   xpkgfake.NewMockRewritePathFn("", "", errBoom),
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errRewriteImage),
			},
		},
		"ErrGetFromCacheFailedDelete": {
			reason: "We should return an error if package content is in cache, we cannot get it, and we fail to remove it.",
			args: args{
				mgr: &fake.Manager{},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ProviderRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								pr := o.(*v1.ProviderRevision)
								pr.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionActive)
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(_ client.Object) error {
								t.Errorf("StatusUpdate should not be called")
								return nil
							}),
						},
					}),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return nil
					}}),
					WithCache(&xpkgfake.MockCache{
						MockHas:    xpkgfake.NewMockCacheHasFn(true),
						MockGet:    xpkgfake.NewMockCacheGetFn(nil, errBoom),
						MockDelete: xpkgfake.NewMockCacheDeleteFn(errBoom),
					}),
					WithConfigStore(&xpkgfake.MockConfigStore{
						MockPullSecretFor: xpkgfake.NewMockConfigStorePullSecretForFn("", "", nil),
						MockRewritePath:   xpkgfake.NewMockRewritePathFn("", "", nil),
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetCache),
			},
		},
		"ErrNotInCachePullPolicyNever": {
			reason: "We should return an error if package content is not in cache and pull policy is Never.",
			args: args{
				mgr: &fake.Manager{},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision {
						return &v1.ProviderRevision{
							Spec: v1.ProviderRevisionSpec{
								PackageRevisionSpec: v1.PackageRevisionSpec{
									PackagePullPolicy: &pullPolicy,
								},
							},
						}
					}),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								pr := o.(*v1.ProviderRevision)
								pr.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionActive)
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
								want := &v1.ProviderRevision{}
								want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionActive)
								want.SetPackagePullPolicy(&pullPolicy)
								want.SetConditions(v1.RevisionUnhealthy().WithMessage("failed to get pre-cached package with pull policy Never"))

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
					WithCache(&xpkgfake.MockCache{
						MockHas: xpkgfake.NewMockCacheHasFn(false),
					}),
					WithConfigStore(&xpkgfake.MockConfigStore{
						MockPullSecretFor: xpkgfake.NewMockConfigStorePullSecretForFn("", "", nil),
						MockRewritePath:   xpkgfake.NewMockRewritePathFn("", "", nil),
					}),
				},
			},
			want: want{
				err: errors.New(errPullPolicyNever),
			},
		},
		"ErrInitParserBackend": {
			reason: "We should return an error if we fail to initialize parser backend.",
			args: args{
				mgr: &fake.Manager{},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ProviderRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								pr := o.(*v1.ProviderRevision)
								pr.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionActive)
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
								want := &v1.ProviderRevision{}
								want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionActive)
								want.SetConditions(v1.RevisionUnhealthy().WithMessage("cannot initialize parser backend: boom"))

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
					WithCache(&xpkgfake.MockCache{
						MockHas: xpkgfake.NewMockCacheHasFn(false),
					}),
					WithParserBackend(&ErrBackend{err: errBoom}),
					WithConfigStore(&xpkgfake.MockConfigStore{
						MockPullSecretFor: xpkgfake.NewMockConfigStorePullSecretForFn("", "", nil),
						MockRewritePath:   xpkgfake.NewMockRewritePathFn("", "", nil),
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errInitParserBackend),
			},
		},
		"ErrParseFromCache": {
			reason: "We should return an error if fail to parse the package from the cache.",
			args: args{
				mgr: &fake.Manager{},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ProviderRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								pr := o.(*v1.ProviderRevision)
								pr.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionActive)
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
								want := &v1.ProviderRevision{}
								want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionActive)
								want.SetConditions(v1.RevisionUnhealthy().WithMessage("cannot parse package contents: boom"))

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
					WithParser(MockParseFn(func(_ context.Context, _ io.ReadCloser) (*parser.Package, error) { return nil, errBoom })),
					WithCache(&xpkgfake.MockCache{
						MockHas: xpkgfake.NewMockCacheHasFn(true),
						MockGet: xpkgfake.NewMockCacheGetFn(io.NopCloser(bytes.NewBuffer(providerBytes)), nil),
					}),
					WithConfigStore(&xpkgfake.MockConfigStore{
						MockPullSecretFor: xpkgfake.NewMockConfigStorePullSecretForFn("", "", nil),
						MockRewritePath:   xpkgfake.NewMockRewritePathFn("", "", nil),
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errParsePackage),
			},
		},
		"ErrParseFromImage": {
			reason: "We should return an error if we fail to parse the package from the image.",
			args: args{
				mgr: &fake.Manager{},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ProviderRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								pr := o.(*v1.ProviderRevision)
								pr.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionActive)
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
								want := &v1.ProviderRevision{}
								want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionActive)
								want.SetConditions(v1.RevisionUnhealthy().WithMessage("cannot parse package contents: boom"))

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
					WithParser(MockParseFn(func(_ context.Context, _ io.ReadCloser) (*parser.Package, error) { return nil, errBoom })),
					WithParserBackend(parser.NewEchoBackend(string(providerBytes))),
					WithCache(&xpkgfake.MockCache{
						MockHas:   xpkgfake.NewMockCacheHasFn(false),
						MockStore: xpkgfake.NewMockCacheStoreFn(nil),
					}),
					WithConfigStore(&xpkgfake.MockConfigStore{
						MockPullSecretFor: xpkgfake.NewMockConfigStorePullSecretForFn("", "", nil),
						MockRewritePath:   xpkgfake.NewMockRewritePathFn("", "", nil),
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errParsePackage),
			},
		},
		"ErrParseFromImageFailedCache": {
			reason: "We should return an error if we fail to parse the package from the image and fail to cache.",
			args: args{
				mgr: &fake.Manager{},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ProviderRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								pr := o.(*v1.ProviderRevision)
								pr.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionActive)
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
								want := &v1.ProviderRevision{}
								want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionActive)
								want.SetConditions(v1.RevisionUnhealthy().WithMessage("cannot parse package contents: boom"))

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
					WithParser(MockParseFn(func(_ context.Context, _ io.ReadCloser) (*parser.Package, error) { return nil, errBoom })),
					WithParserBackend(parser.NewEchoBackend(string(providerBytes))),
					WithCache(&xpkgfake.MockCache{
						MockHas:    xpkgfake.NewMockCacheHasFn(false),
						MockStore:  xpkgfake.NewMockCacheStoreFn(errBoom),
						MockDelete: xpkgfake.NewMockCacheDeleteFn(nil),
					}),
					WithConfigStore(&xpkgfake.MockConfigStore{
						MockPullSecretFor: xpkgfake.NewMockConfigStorePullSecretForFn("", "", nil),
						MockRewritePath:   xpkgfake.NewMockRewritePathFn("", "", nil),
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errParsePackage),
			},
		},
		"ErrParseFromImageFailedCacheFailedDelete": {
			reason: "We should return an error if we fail to parse the package from the image, fail to cache, and fail to delete from cache.",
			args: args{
				mgr: &fake.Manager{},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ProviderRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								pr := o.(*v1.ProviderRevision)
								pr.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionActive)
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
								want := &v1.ProviderRevision{}
								want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionActive)
								want.SetConditions(v1.RevisionUnhealthy().WithMessage("cannot parse package contents: boom"))

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
					WithParser(MockParseFn(func(_ context.Context, _ io.ReadCloser) (*parser.Package, error) { return nil, errBoom })),
					WithParserBackend(parser.NewEchoBackend(string(providerBytes))),
					WithCache(&xpkgfake.MockCache{
						MockHas:    xpkgfake.NewMockCacheHasFn(false),
						MockStore:  xpkgfake.NewMockCacheStoreFn(errBoom),
						MockDelete: xpkgfake.NewMockCacheDeleteFn(errBoom),
					}),
					WithConfigStore(&xpkgfake.MockConfigStore{
						MockPullSecretFor: xpkgfake.NewMockConfigStorePullSecretForFn("", "", nil),
						MockRewritePath:   xpkgfake.NewMockRewritePathFn("", "", nil),
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errParsePackage),
			},
		},
		"ErrLint": {
			reason: "We should return an error if fail to lint the package.",
			args: args{
				mgr: &fake.Manager{},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ProviderRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								pr := o.(*v1.ProviderRevision)
								pr.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionActive)
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
								want := &v1.ProviderRevision{}
								want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionActive)
								want.SetConditions(v1.RevisionUnhealthy().WithMessage("linting package contents failed: boom"))

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
					WithCache(&xpkgfake.MockCache{
						MockHas: xpkgfake.NewMockCacheHasFn(false),
						MockStore: func(_ string, rc io.ReadCloser) error {
							_, err := io.ReadAll(rc)
							return err
						},
					}),
					WithConfigStore(&xpkgfake.MockConfigStore{
						MockPullSecretFor: xpkgfake.NewMockConfigStorePullSecretForFn("", "", nil),
						MockRewritePath:   xpkgfake.NewMockRewritePathFn("", "", nil),
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errLintPackage),
			},
		},
		"ErrCrossplaneConstraints": {
			reason: "We should not requeue if Crossplane version is incompatible.",
			args: args{
				mgr: &fake.Manager{},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ProviderRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								pr := o.(*v1.ProviderRevision)
								pr.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionActive)
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
								want := &v1.ProviderRevision{}
								want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionActive)
								want.SetConditions(v1.RevisionUnhealthy().WithMessage("incompatible Crossplane version: package is not compatible with Crossplane version (v0.11.0): boom"))
								want.SetAnnotations(map[string]string{"author": "crossplane"})

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
							MockUpdate: test.NewMockUpdateFn(nil, func(o client.Object) error {
								want := &v1.ProviderRevision{}
								want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionActive)
								want.SetAnnotations(map[string]string{"author": "crossplane"})
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
					WithCache(&xpkgfake.MockCache{
						MockHas: xpkgfake.NewMockCacheHasFn(false),
						MockStore: func(_ string, rc io.ReadCloser) error {
							_, err := io.ReadAll(rc)
							return err
						},
					}),
					WithLinter(&MockLinter{MockLint: NewMockLintFn(nil)}),
					WithVersioner(&verfake.MockVersioner{
						MockInConstraints:    verfake.NewMockInConstraintsFn(false, errBoom),
						MockGetVersionString: verfake.NewMockGetVersionStringFn("v0.11.0"),
					}),
					WithConfigStore(&xpkgfake.MockConfigStore{
						MockPullSecretFor: xpkgfake.NewMockConfigStorePullSecretForFn("", "", nil),
						MockRewritePath:   xpkgfake.NewMockRewritePathFn("", "", nil),
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"ErrOneMeta": {
			reason: "We should return an error if not exactly one meta package type.",
			args: args{
				mgr: &fake.Manager{},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ProviderRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								pr := o.(*v1.ProviderRevision)
								pr.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionActive)
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
								want := &v1.ProviderRevision{}
								want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionActive)
								want.SetConditions(v1.RevisionUnhealthy().WithMessage("cannot install package with multiple meta types"))

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
					WithParserBackend(parser.NewEchoBackend("")),
					WithCache(&xpkgfake.MockCache{
						MockHas:   xpkgfake.NewMockCacheHasFn(false),
						MockStore: xpkgfake.NewMockCacheStoreFn(nil),
					}),
					WithLinter(&MockLinter{MockLint: NewMockLintFn(nil)}),
					WithConfigStore(&xpkgfake.MockConfigStore{
						MockPullSecretFor: xpkgfake.NewMockConfigStorePullSecretForFn("", "", nil),
						MockRewritePath:   xpkgfake.NewMockRewritePathFn("", "", nil),
					}),
				},
			},
			want: want{
				err: errors.New(errNotOneMeta),
			},
		},
		"ErrUpdateAnnotations": {
			reason: "We should return an error if we fail to update our annotations.",
			args: args{
				mgr: &fake.Manager{},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ProviderRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								pr := o.(*v1.ProviderRevision)
								pr.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionActive)
								return nil
							}),
							MockUpdate: test.NewMockUpdateFn(errBoom),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
								want := &v1.ProviderRevision{}
								want.SetAnnotations(map[string]string{"author": "crossplane"})
								want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionActive)
								want.SetConditions(v1.RevisionUnhealthy().WithMessage("cannot update package revision object metadata: boom"))

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
					WithCache(&xpkgfake.MockCache{
						MockHas: xpkgfake.NewMockCacheHasFn(false),
						MockStore: func(_ string, rc io.ReadCloser) error {
							_, err := io.ReadAll(rc)
							return err
						},
					}),
					WithLinter(&MockLinter{MockLint: NewMockLintFn(nil)}),
					WithConfigStore(&xpkgfake.MockConfigStore{
						MockPullSecretFor: xpkgfake.NewMockConfigStorePullSecretForFn("", "", nil),
						MockRewritePath:   xpkgfake.NewMockRewritePathFn("", "", nil),
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateMeta),
			},
		},
		"ErrResolveDependencies": {
			reason: "We should return an error if we fail to resolve dependencies.",
			args: args{
				mgr: &fake.Manager{},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ProviderRevision{} }),
					WithDependencyManager(&MockDependencyManager{
						MockResolve: NewMockResolveFn(0, 0, 0, errBoom),
					}),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								pr := o.(*v1.ProviderRevision)
								pr.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionActive)
								pr.SetSkipDependencyResolution(ptr.To(false))
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
								want := &v1.ProviderRevision{}
								want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionActive)
								want.SetSkipDependencyResolution(ptr.To(false))
								want.SetAnnotations(map[string]string{"author": "crossplane"})
								want.SetConditions(v1.RevisionUnhealthy().WithMessage("cannot resolve package dependencies: boom"))

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
							MockUpdate: test.NewMockUpdateFn(nil, func(o client.Object) error {
								want := &v1.ProviderRevision{}
								want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionActive)
								want.SetAnnotations(map[string]string{"author": "crossplane"})
								want.SetSkipDependencyResolution(ptr.To(false))
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
					WithCache(&xpkgfake.MockCache{
						MockHas: xpkgfake.NewMockCacheHasFn(false),
						MockStore: func(_ string, rc io.ReadCloser) error {
							_, err := io.ReadAll(rc)
							return err
						},
					}),
					WithLinter(&MockLinter{MockLint: NewMockLintFn(nil)}),
					WithVersioner(&verfake.MockVersioner{MockInConstraints: verfake.NewMockInConstraintsFn(true, nil)}),
					WithConfigStore(&xpkgfake.MockConfigStore{
						MockPullSecretFor: xpkgfake.NewMockConfigStorePullSecretForFn("", "", nil),
						MockRewritePath:   xpkgfake.NewMockRewritePathFn("", "", nil),
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errResolveDeps),
			},
		},
		"SuccessfulActiveRevision": {
			reason: "An active revision should establish control of all of its resources.",
			args: args{
				mgr: &fake.Manager{},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ProviderRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								pr := o.(*v1.ProviderRevision)
								pr.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionActive)
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
								want := &v1.ProviderRevision{}
								want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionActive)
								want.SetAnnotations(map[string]string{"author": "crossplane"})
								want.SetConditions(v1.RevisionHealthy())

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
							MockUpdate: test.NewMockUpdateFn(nil, func(o client.Object) error {
								want := &v1.ProviderRevision{}
								want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionActive)
								want.SetAnnotations(map[string]string{"author": "crossplane"})
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
					WithEstablisher(NewMockEstablisher()),
					WithParser(parser.New(metaScheme, objScheme)),
					WithParserBackend(parser.NewEchoBackend(string(providerBytes))),
					WithCache(&xpkgfake.MockCache{
						MockHas: xpkgfake.NewMockCacheHasFn(false),
						MockStore: func(_ string, rc io.ReadCloser) error {
							_, err := io.ReadAll(rc)
							return err
						},
					}),
					WithLinter(&MockLinter{MockLint: NewMockLintFn(nil)}),
					WithVersioner(&verfake.MockVersioner{MockInConstraints: verfake.NewMockInConstraintsFn(true, nil)}),
					WithConfigStore(&xpkgfake.MockConfigStore{
						MockPullSecretFor: xpkgfake.NewMockConfigStorePullSecretForFn("", "", nil),
						MockRewritePath:   xpkgfake.NewMockRewritePathFn("", "", nil),
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"SuccessfulActiveRevisionImageConfigRewrite": {
			reason: "An active revision should be updated when its image is rewritten by an image config.",
			args: args{
				mgr: &fake.Manager{},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ProviderRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								pr := o.(*v1.ProviderRevision)
								pr.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionActive)
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
								want := &v1.ProviderRevision{}
								want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionActive)
								want.SetResolvedSource("new/image/path")
								want.SetAppliedImageConfigRefs(v1.ImageConfigRef{
									Name:   "imageConfigName",
									Reason: v1.ImageConfigReasonRewrite,
								})

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
					WithEstablisher(NewMockEstablisher()),
					WithParser(parser.New(metaScheme, objScheme)),
					WithParserBackend(parser.NewEchoBackend(string(providerBytes))),
					WithCache(&xpkgfake.MockCache{
						MockHas: xpkgfake.NewMockCacheHasFn(false),
						MockStore: func(_ string, rc io.ReadCloser) error {
							_, err := io.ReadAll(rc)
							return err
						},
					}),
					WithLinter(&MockLinter{MockLint: NewMockLintFn(nil)}),
					WithVersioner(&verfake.MockVersioner{MockInConstraints: verfake.NewMockInConstraintsFn(true, nil)}),
					WithConfigStore(&xpkgfake.MockConfigStore{
						MockPullSecretFor: xpkgfake.NewMockConfigStorePullSecretForFn("", "", nil),
						MockRewritePath:   xpkgfake.NewMockRewritePathFn("imageConfigName", "new/image/path", nil),
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: true},
			},
		},
		"SuccessfulActiveRevisionImageConfigRewritten": {
			reason: "An active revision should install when its image has been rewritten by an image config on a previous reconcile.",
			args: args{
				mgr: &fake.Manager{},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ProviderRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								pr := o.(*v1.ProviderRevision)
								pr.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionActive)
								pr.SetResolvedSource("new/image/path")
								pr.SetAppliedImageConfigRefs(v1.ImageConfigRef{
									Name:   "imageConfigName",
									Reason: v1.ImageConfigReasonRewrite,
								})
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
								want := &v1.ProviderRevision{}
								want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionActive)
								want.SetAnnotations(map[string]string{"author": "crossplane"})
								want.SetConditions(v1.RevisionHealthy())
								want.SetResolvedSource("new/image/path")
								want.SetAppliedImageConfigRefs(v1.ImageConfigRef{
									Name:   "imageConfigName",
									Reason: v1.ImageConfigReasonRewrite,
								})

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
							MockUpdate: test.NewMockUpdateFn(nil, func(o client.Object) error {
								want := &v1.ProviderRevision{}
								want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionActive)
								want.SetAnnotations(map[string]string{"author": "crossplane"})
								want.SetResolvedSource("new/image/path")
								want.SetAppliedImageConfigRefs(v1.ImageConfigRef{
									Name:   "imageConfigName",
									Reason: v1.ImageConfigReasonRewrite,
								})

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
					WithEstablisher(NewMockEstablisher()),
					WithParser(parser.New(metaScheme, objScheme)),
					WithParserBackend(parser.NewEchoBackend(string(providerBytes))),
					WithCache(&xpkgfake.MockCache{
						MockHas: xpkgfake.NewMockCacheHasFn(false),
						MockStore: func(_ string, rc io.ReadCloser) error {
							_, err := io.ReadAll(rc)
							return err
						},
					}),
					WithLinter(&MockLinter{MockLint: NewMockLintFn(nil)}),
					WithVersioner(&verfake.MockVersioner{MockInConstraints: verfake.NewMockInConstraintsFn(true, nil)}),
					WithConfigStore(&xpkgfake.MockConfigStore{
						MockPullSecretFor: xpkgfake.NewMockConfigStorePullSecretForFn("", "", nil),
						MockRewritePath:   xpkgfake.NewMockRewritePathFn("imageConfigName", "new/image/path", nil),
					}),
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
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ProviderRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								pr := o.(*v1.ProviderRevision)
								pr.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionActive)
								pr.SetIgnoreCrossplaneConstraints(&trueVal)
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
								want := &v1.ProviderRevision{}
								want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionActive)
								want.SetAnnotations(map[string]string{"author": "crossplane"})
								want.SetConditions(v1.RevisionHealthy())
								want.SetIgnoreCrossplaneConstraints(&trueVal)

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
							MockUpdate: test.NewMockUpdateFn(nil, func(o client.Object) error {
								want := &v1.ProviderRevision{}
								want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionActive)
								want.SetAnnotations(map[string]string{"author": "crossplane"})
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
					WithEstablisher(NewMockEstablisher()),
					WithParser(parser.New(metaScheme, objScheme)),
					WithParserBackend(parser.NewEchoBackend(string(providerBytes))),
					WithCache(&xpkgfake.MockCache{
						MockHas: xpkgfake.NewMockCacheHasFn(false),
						MockStore: func(_ string, rc io.ReadCloser) error {
							_, err := io.ReadAll(rc)
							return err
						},
					}),
					WithLinter(&MockLinter{MockLint: NewMockLintFn(nil)}),
					WithVersioner(&verfake.MockVersioner{MockInConstraints: verfake.NewMockInConstraintsFn(false, nil)}),
					WithConfigStore(&xpkgfake.MockConfigStore{
						MockPullSecretFor: xpkgfake.NewMockConfigStorePullSecretForFn("", "", nil),
						MockRewritePath:   xpkgfake.NewMockRewritePathFn("", "", nil),
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"ErrEstablishActiveRevision": {
			reason: "An active revision that fails to establish control should return an error.",
			args: args{
				mgr: &fake.Manager{},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ProviderRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								pr := o.(*v1.ProviderRevision)
								pr.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionActive)
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
								want := &v1.ProviderRevision{}
								want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionActive)
								want.SetAnnotations(map[string]string{"author": "crossplane"})
								want.SetConditions(v1.RevisionUnhealthy().WithMessage("cannot establish control of object: boom"))

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
							MockDelete: test.NewMockDeleteFn(nil),
							MockUpdate: test.NewMockUpdateFn(nil, func(o client.Object) error {
								want := &v1.ProviderRevision{}
								want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionActive)
								want.SetAnnotations(map[string]string{"author": "crossplane"})
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
					WithEstablisher(&MockEstablisher{
						MockEstablish: NewMockEstablishFn(nil, errBoom),
					}),
					WithParser(parser.New(metaScheme, objScheme)),
					WithParserBackend(parser.NewEchoBackend(string(providerBytes))),
					WithCache(&xpkgfake.MockCache{
						MockHas: xpkgfake.NewMockCacheHasFn(false),
						MockStore: func(_ string, rc io.ReadCloser) error {
							_, err := io.ReadAll(rc)
							return err
						},
					}),
					WithLinter(&MockLinter{MockLint: NewMockLintFn(nil)}),
					WithVersioner(&verfake.MockVersioner{MockInConstraints: verfake.NewMockInConstraintsFn(true, nil)}),
					WithConfigStore(&xpkgfake.MockConfigStore{
						MockPullSecretFor: xpkgfake.NewMockConfigStorePullSecretForFn("", "", nil),
						MockRewritePath:   xpkgfake.NewMockRewritePathFn("", "", nil),
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errEstablishControl),
			},
		},
		"ErrEstablishInactiveRevision": {
			reason: "An inactive revision that fails to establish ownership should return an error.",
			args: args{
				mgr: &fake.Manager{},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision {
						return &v1.ProviderRevision{
							Status: v1.ProviderRevisionStatus{
								PackageRevisionStatus: v1.PackageRevisionStatus{
									ObjectRefs: []xpv1.TypedReference{
										{
											APIVersion: "apiextensions.k8s.io/v1",
											Kind:       "CustomResourceDefinition",
											Name:       "releases.helm.crossplane.io",
										},
									},
								},
							},
						}
					}),
					WithDependencyManager(&MockDependencyManager{
						MockRemoveSelf: NewMockRemoveSelfFn(nil),
					}),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								pr := o.(*v1.ProviderRevision)
								pr.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionInactive)
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
								want := &v1.ProviderRevision{
									Status: v1.ProviderRevisionStatus{
										PackageRevisionStatus: v1.PackageRevisionStatus{
											ObjectRefs: []xpv1.TypedReference{
												{
													APIVersion: "apiextensions.k8s.io/v1",
													Kind:       "CustomResourceDefinition",
													Name:       "releases.helm.crossplane.io",
												},
											},
										},
									},
								}
								want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionInactive)
								want.SetConditions(v1.RevisionHealthy())

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
					WithEstablisher(&MockEstablisher{
						MockRelinquish: func() error {
							return errBoom
						},
					}),
					WithConfigStore(&xpkgfake.MockConfigStore{
						MockPullSecretFor: xpkgfake.NewMockConfigStorePullSecretForFn("", "", nil),
						MockRewritePath:   xpkgfake.NewMockRewritePathFn("", "", nil),
					}),
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrap(errBoom, errReleaseObjects), errDeactivateRevision),
			},
		},
		"SuccessfulInactiveRevisionWithoutObjectRefs": {
			reason: "An inactive revision without ObjectRefs should be deactivated successfully by pulling/parsing the package again.",
			args: args{
				mgr: &fake.Manager{},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ProviderRevision{} }),
					WithDependencyManager(&MockDependencyManager{
						MockRemoveSelf: NewMockRemoveSelfFn(nil),
					}),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								pr := o.(*v1.ProviderRevision)
								pr.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionInactive)
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
								want := &v1.ProviderRevision{}
								want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionInactive)
								want.SetAnnotations(map[string]string{"author": "crossplane"})
								want.SetConditions(v1.RevisionHealthy())

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
							MockUpdate: test.NewMockUpdateFn(nil, func(o client.Object) error {
								want := &v1.ProviderRevision{}
								want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionInactive)
								want.SetAnnotations(map[string]string{"author": "crossplane"})
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
					WithEstablisher(NewMockEstablisher()),
					WithParser(parser.New(metaScheme, objScheme)),
					WithParserBackend(parser.NewEchoBackend(string(providerBytes))),
					WithCache(&xpkgfake.MockCache{
						MockHas: xpkgfake.NewMockCacheHasFn(false),
						MockStore: func(_ string, rc io.ReadCloser) error {
							_, err := io.ReadAll(rc)
							return err
						},
					}),
					WithLinter(&MockLinter{MockLint: NewMockLintFn(nil)}),
					WithVersioner(&verfake.MockVersioner{MockInConstraints: verfake.NewMockInConstraintsFn(true, nil)}),
					WithConfigStore(&xpkgfake.MockConfigStore{
						MockPullSecretFor: xpkgfake.NewMockConfigStorePullSecretForFn("", "", nil),
						MockRewritePath:   xpkgfake.NewMockRewritePathFn("", "", nil),
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"SuccessfulInactiveRevisionWithObjectRefs": {
			reason: "An inactive revision with ObjectRefs should be deactivated successfully without pulling/parsing the package again.",
			args: args{
				mgr: &fake.Manager{},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision {
						return &v1.ProviderRevision{
							Status: v1.ProviderRevisionStatus{
								PackageRevisionStatus: v1.PackageRevisionStatus{
									ObjectRefs: []xpv1.TypedReference{
										{
											APIVersion: "apiextensions.k8s.io/v1",
											Kind:       "CustomResourceDefinition",
											Name:       "releases.helm.crossplane.io",
										},
									},
								},
							},
						}
					}),
					WithDependencyManager(&MockDependencyManager{
						MockRemoveSelf: NewMockRemoveSelfFn(nil),
					}),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								pr := o.(*v1.ProviderRevision)
								pr.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionInactive)
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
								want := &v1.ProviderRevision{
									Status: v1.ProviderRevisionStatus{
										PackageRevisionStatus: v1.PackageRevisionStatus{
											ObjectRefs: []xpv1.TypedReference{
												{
													APIVersion: "apiextensions.k8s.io/v1",
													Kind:       "CustomResourceDefinition",
													Name:       "releases.helm.crossplane.io",
												},
											},
										},
									},
								}
								want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionInactive)
								want.SetConditions(v1.RevisionHealthy())

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
					WithEstablisher(NewMockEstablisher()),
					WithConfigStore(&xpkgfake.MockConfigStore{
						MockPullSecretFor: xpkgfake.NewMockConfigStorePullSecretForFn("", "", nil),
						MockRewritePath:   xpkgfake.NewMockRewritePathFn("", "", nil),
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"PauseReconcile": {
			reason: "Pause reconciliation if the pause annotation is set.",
			args: args{
				mgr: &fake.Manager{},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ProviderRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								pr := o.(*v1.ProviderRevision)
								pr.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionActive)
								pr.SetAnnotations(map[string]string{
									meta.AnnotationKeyReconciliationPaused: "true",
								})
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
								want := &v1.ProviderRevision{}
								want.SetAnnotations(map[string]string{
									meta.AnnotationKeyReconciliationPaused: "true",
								})
								want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionActive)
								want.SetConditions(xpv1.ReconcilePaused().WithMessage(reconcilePausedMsg))

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
					WithEstablisher(NewMockEstablisher()),
					WithParser(parser.New(metaScheme, objScheme)),
					WithParserBackend(parser.NewEchoBackend(string(providerBytes))),
					WithCache(&xpkgfake.MockCache{
						MockHas: xpkgfake.NewMockCacheHasFn(false),
						MockStore: func(_ string, rc io.ReadCloser) error {
							_, err := io.ReadAll(rc)
							return err
						},
					}),
					WithLinter(&MockLinter{MockLint: NewMockLintFn(nil)}),
					WithVersioner(&verfake.MockVersioner{MockInConstraints: verfake.NewMockInConstraintsFn(true, nil)}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"ResumeReconcile": {
			reason: "An active revision should establish control of all of its resources.",
			args: args{
				mgr: &fake.Manager{},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ProviderRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								pr := o.(*v1.ProviderRevision)
								pr.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionActive)
								pr.SetConditions(xpv1.ReconcilePaused())
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
								want := &v1.ProviderRevision{}
								want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionActive)
								want.Status.Conditions = []xpv1.Condition{}

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
							MockUpdate: test.NewMockUpdateFn(nil, func(o client.Object) error {
								want := &v1.ProviderRevision{}
								want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionActive)
								want.SetAnnotations(map[string]string{"author": "crossplane"})
								want.CleanConditions()
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
					WithEstablisher(NewMockEstablisher()),
					WithParser(parser.New(metaScheme, objScheme)),
					WithParserBackend(parser.NewEchoBackend(string(providerBytes))),
					WithCache(&xpkgfake.MockCache{
						MockHas: xpkgfake.NewMockCacheHasFn(false),
						MockStore: func(_ string, rc io.ReadCloser) error {
							_, err := io.ReadAll(rc)
							return err
						},
					}),
					WithLinter(&MockLinter{MockLint: NewMockLintFn(nil)}),
					WithVersioner(&verfake.MockVersioner{MockInConstraints: verfake.NewMockInConstraintsFn(true, nil)}),
					WithConfigStore(&xpkgfake.MockConfigStore{
						MockPullSecretFor: xpkgfake.NewMockConfigStorePullSecretForFn("", "", nil),
						MockRewritePath:   xpkgfake.NewMockRewritePathFn("", "", nil),
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"WaitForSignatureVerifiedCondition": {
			reason: "We should wait until signature verification is complete before proceeding and communicate this with the Healthy condition.",
			args: args{
				mgr: &fake.Manager{},
				rec: []ReconcilerOption{
					WithFeatureFlags(signatureVerificationEnabled()),
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ProviderRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(_ client.Object) error {
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
								want := &v1.ProviderRevision{}
								want.SetConditions(v1.AwaitingVerification())
								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
						},
					}),
					WithConfigStore(&xpkgfake.MockConfigStore{
						MockPullSecretFor: xpkgfake.NewMockConfigStorePullSecretForFn("", "", nil),
						MockRewritePath:   xpkgfake.NewMockRewritePathFn("", "", nil),
					}),
				},
			},
		},
		"WaitForSignatureVerifiedConditionIfFailed": {
			reason: "We should keep waiting if signature verification failed and communicate this with the Healthy condition.",
			args: args{
				mgr: &fake.Manager{},
				rec: []ReconcilerOption{
					WithFeatureFlags(signatureVerificationEnabled()),
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ProviderRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								pr := o.(*v1.ProviderRevision)
								pr.SetConditions(v1.VerificationFailed("foo", errBoom))
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
								want := &v1.ProviderRevision{}
								want.SetConditions(v1.VerificationFailed("foo", errBoom))
								want.SetConditions(v1.AwaitingVerification())
								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
						},
					}),
					WithConfigStore(&xpkgfake.MockConfigStore{
						MockPullSecretFor: xpkgfake.NewMockConfigStorePullSecretForFn("", "", nil),
						MockRewritePath:   xpkgfake.NewMockRewritePathFn("", "", nil),
					}),
				},
			},
		},
		"WaitForSignatureVerifiedConditionIfIncomplete": {
			reason: "We should keep waiting if signature verification incomplete and communicate this with the Healthy condition.",
			args: args{
				mgr: &fake.Manager{},
				rec: []ReconcilerOption{
					WithFeatureFlags(signatureVerificationEnabled()),
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ProviderRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								pr := o.(*v1.ProviderRevision)
								pr.SetConditions(v1.VerificationIncomplete(errBoom))
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
								want := &v1.ProviderRevision{}
								want.SetConditions(v1.VerificationIncomplete(errBoom))
								want.SetConditions(v1.AwaitingVerification())
								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
						},
					}),
					WithConfigStore(&xpkgfake.MockConfigStore{
						MockPullSecretFor: xpkgfake.NewMockConfigStorePullSecretForFn("", "", nil),
						MockRewritePath:   xpkgfake.NewMockRewritePathFn("", "", nil),
					}),
				},
			},
		},
		"SuccessfulActiveRevisionSuccessfulVerification": {
			reason: "An active revision should establish control of all of its resources.",
			args: args{
				mgr: &fake.Manager{},
				rec: []ReconcilerOption{
					WithFeatureFlags(signatureVerificationEnabled()),
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ProviderRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								pr := o.(*v1.ProviderRevision)
								pr.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionActive)
								pr.SetConditions(v1.VerificationSucceeded("foo"))
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
								want := &v1.ProviderRevision{}
								want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionActive)
								want.SetAnnotations(map[string]string{"author": "crossplane"})
								want.SetConditions(v1.VerificationSucceeded("foo"))
								want.SetConditions(v1.RevisionHealthy())

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
							MockUpdate: test.NewMockUpdateFn(nil, func(o client.Object) error {
								want := &v1.ProviderRevision{}
								want.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								want.SetDesiredState(v1.PackageRevisionActive)
								want.SetAnnotations(map[string]string{"author": "crossplane"})
								want.SetConditions(v1.VerificationSucceeded("foo"))
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
					WithEstablisher(NewMockEstablisher()),
					WithParser(parser.New(metaScheme, objScheme)),
					WithParserBackend(parser.NewEchoBackend(string(providerBytes))),
					WithCache(&xpkgfake.MockCache{
						MockHas: xpkgfake.NewMockCacheHasFn(false),
						MockStore: func(_ string, rc io.ReadCloser) error {
							_, err := io.ReadAll(rc)
							return err
						},
					}),
					WithLinter(&MockLinter{MockLint: NewMockLintFn(nil)}),
					WithVersioner(&verfake.MockVersioner{MockInConstraints: verfake.NewMockInConstraintsFn(true, nil)}),
					WithConfigStore(&xpkgfake.MockConfigStore{
						MockPullSecretFor: xpkgfake.NewMockConfigStorePullSecretForFn("", "", nil),
						MockRewritePath:   xpkgfake.NewMockRewritePathFn("", "", nil),
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"PullSecretConfigChangedRequeue": {
			reason: "Should requeue when pull secret config changes to persist the status immediately.",
			args: args{
				mgr: &fake.Manager{},
				rec: []ReconcilerOption{
					WithNewPackageRevisionFn(func() v1.PackageRevision { return &v1.ProviderRevision{} }),
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								pr := o.(*v1.ProviderRevision)
								pr.SetGroupVersionKind(v1.ProviderRevisionGroupVersionKind)
								pr.SetDesiredState(v1.PackageRevisionActive)
								// Start with no applied image config refs to simulate a change
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
								pr := o.(*v1.ProviderRevision)
								// Verify that the pull secret config ref was set
								refs := pr.GetAppliedImageConfigRefs()
								if len(refs) != 1 {
									t.Errorf("Expected 1 applied image config ref, got %d", len(refs))
									return nil
								}
								if refs[0].Name != "test-config" || refs[0].Reason != v1.ImageConfigReasonSetPullSecret {
									t.Errorf("Expected pull secret config ref with name 'test-config' and reason SetPullSecret, got %+v", refs[0])
								}
								return nil
							}),
						},
					}),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return nil
					}}),
					WithConfigStore(&xpkgfake.MockConfigStore{
						MockPullSecretFor: xpkgfake.NewMockConfigStorePullSecretForFn("test-config", "test-secret", nil),
						MockRewritePath:   xpkgfake.NewMockRewritePathFn("", "", nil),
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: true},
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

func signatureVerificationEnabled() *feature.Flags {
	f := &feature.Flags{}
	f.Enable(features.EnableAlphaSignatureVerification)

	return f
}
