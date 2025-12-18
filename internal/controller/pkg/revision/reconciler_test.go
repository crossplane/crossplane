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
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/parser"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	pkgmetav1 "github.com/crossplane/crossplane/v2/apis/pkg/meta/v1"
	v1 "github.com/crossplane/crossplane/v2/apis/pkg/v1"
	"github.com/crossplane/crossplane/v2/internal/features"
	verfake "github.com/crossplane/crossplane/v2/internal/version/fake"
	"github.com/crossplane/crossplane/v2/internal/xpkg"
	xpkgfake "github.com/crossplane/crossplane/v2/internal/xpkg/fake"
	xpkgyaml "github.com/crossplane/crossplane/v2/internal/xpkg/parser/yaml"
)

var _ Establisher = &MockEstablisher{}

type MockEstablisher struct {
	MockEstablish  func(context.Context, []runtime.Object, v1.PackageRevision, bool) ([]xpv1.TypedReference, error)
	MockRelinquish func() error
}

func NewMockEstablisher() *MockEstablisher {
	return &MockEstablisher{
		MockEstablish:  NewMockEstablishFn(nil, nil),
		MockRelinquish: NewMockRelinquishFn(nil),
	}
}

func NewMockEstablishFn(refs []xpv1.TypedReference, err error) func(context.Context, []runtime.Object, v1.PackageRevision, bool) ([]xpv1.TypedReference, error) {
	return func(_ context.Context, _ []runtime.Object, _ v1.PackageRevision, _ bool) ([]xpv1.TypedReference, error) {
		return refs, err
	}
}

func NewMockRelinquishFn(err error) func() error {
	return func() error { return err }
}

func (e *MockEstablisher) Establish(ctx context.Context, objs []runtime.Object, pr v1.PackageRevision, ctrl bool) ([]xpv1.TypedReference, error) {
	return e.MockEstablish(ctx, objs, pr, ctrl)
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

var providerYAML = []byte(`
apiVersion: meta.pkg.crossplane.io/v1
kind: Provider
metadata:
  name: test
  annotations:
    author: crossplane
spec:
  crossplane:
    version: ">v0.13.0"
`)

func parsePackage(yaml []byte) *parser.Package {
	p, err := xpkgyaml.New()
	if err != nil {
		panic(err)
	}
	pkg, err := p.Parse(context.Background(), io.NopCloser(bytes.NewReader(yaml)))
	if err != nil {
		panic(err)
	}
	return pkg
}

func mockPackage() *xpkg.Package {
	return &xpkg.Package{
		Package:         parsePackage(providerYAML),
		Digest:          "sha256:1234567890123456789012345678901234567890123456789012345678901234",
		Version:         "v1.0.0",
		Source:          "xpkg.crossplane.io/test",
		ResolvedVersion: "v1.0.0",
		ResolvedSource:  "xpkg.crossplane.io/test",
	}
}

func mockEmptyPackage() *xpkg.Package {
	return &xpkg.Package{
		Package:         parser.NewPackage(),
		Digest:          "sha256:1234567890123456789012345678901234567890123456789012345678901234",
		Version:         "v1.0.0",
		Source:          "xpkg.crossplane.io/test",
		ResolvedVersion: "v1.0.0",
		ResolvedSource:  "xpkg.crossplane.io/test",
	}
}

func TestReconcile(t *testing.T) {
	errBoom := errors.New("boom")
	testLog := logging.NewLogrLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(io.Discard)).WithName("testlog"))
	now := metav1.Now()
	trueVal := true

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
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errAddFinalizer),
			},
		},
		"ErrGetPackage": {
			reason: "We should return an error if we fail to get the package.",
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
								want.SetConditions(v1.RevisionUnhealthy().WithMessage("cannot get package: boom"))

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
					WithClient(&xpkgfake.MockClient{
						MockGet: xpkgfake.NewMockGetFn(nil, errBoom),
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetPackage),
			},
		},
		"ErrValidate": {
			reason: "We should return an error if fail to validate the package.",
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
								want.SetResolvedSource("xpkg.crossplane.io/test:v1.0.0")
								want.SetConditions(v1.RevisionUnhealthy().WithMessage("validating package contents failed: boom"))

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
					WithClient(&xpkgfake.MockClient{
						MockGet: xpkgfake.NewMockGetFn(mockPackage(), nil),
					}),
					WithValidator(&MockLinter{MockLint: NewMockLintFn(errBoom)}),
					WithLinter(&MockLinter{MockLint: NewMockLintFn(nil)}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errValidatePackage),
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
								want.SetResolvedSource("xpkg.crossplane.io/test:v1.0.0")
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
								want.SetResolvedSource("xpkg.crossplane.io/test:v1.0.0")
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
					WithClient(&xpkgfake.MockClient{
						MockGet: xpkgfake.NewMockGetFn(mockPackage(), nil),
					}),
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
								want.SetResolvedSource("xpkg.crossplane.io/test:v1.0.0")
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
					WithClient(&xpkgfake.MockClient{
						MockGet: xpkgfake.NewMockGetFn(mockEmptyPackage(), nil),
					}),
					WithLinter(&MockLinter{MockLint: NewMockLintFn(nil)}),
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
								want.SetResolvedSource("xpkg.crossplane.io/test:v1.0.0")
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
					WithClient(&xpkgfake.MockClient{
						MockGet: xpkgfake.NewMockGetFn(mockPackage(), nil),
					}),
					WithLinter(&MockLinter{MockLint: NewMockLintFn(nil)}),
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
								want.SetResolvedSource("xpkg.crossplane.io/test:v1.0.0")
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
								want.SetResolvedSource("xpkg.crossplane.io/test:v1.0.0")
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
					WithClient(&xpkgfake.MockClient{
						MockGet: xpkgfake.NewMockGetFn(mockPackage(), nil),
					}),
					WithLinter(&MockLinter{MockLint: NewMockLintFn(nil)}),
					WithVersioner(&verfake.MockVersioner{MockInConstraints: verfake.NewMockInConstraintsFn(true, nil)}),
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
								want.SetResolvedSource("xpkg.crossplane.io/test:v1.0.0")
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
								want.SetResolvedSource("xpkg.crossplane.io/test:v1.0.0")
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
					WithClient(&xpkgfake.MockClient{
						MockGet: xpkgfake.NewMockGetFn(mockPackage(), nil),
					}),
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
								want.SetResolvedSource("xpkg.crossplane.io/test:v1.0.0")
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
								want.SetResolvedSource("xpkg.crossplane.io/test:v1.0.0")
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
					WithClient(&xpkgfake.MockClient{
						MockGet: xpkgfake.NewMockGetFn(mockPackage(), nil),
					}),
					WithLinter(&MockLinter{MockLint: NewMockLintFn(nil)}),
					WithVersioner(&verfake.MockVersioner{MockInConstraints: verfake.NewMockInConstraintsFn(false, nil)}),
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
								want.SetResolvedSource("xpkg.crossplane.io/test:v1.0.0")
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
								want.SetResolvedSource("xpkg.crossplane.io/test:v1.0.0")
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
					WithClient(&xpkgfake.MockClient{
						MockGet: xpkgfake.NewMockGetFn(mockPackage(), nil),
					}),
					WithLinter(&MockLinter{MockLint: NewMockLintFn(nil)}),
					WithVersioner(&verfake.MockVersioner{MockInConstraints: verfake.NewMockInConstraintsFn(true, nil)}),
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
				},
			},
			want: want{
				err: errors.Wrap(errors.Wrap(errBoom, errReleaseObjects), errDeactivateRevision),
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
								want.SetResolvedSource("xpkg.crossplane.io/test:v1.0.0")
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
								want.SetResolvedSource("xpkg.crossplane.io/test:v1.0.0")
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
					WithClient(&xpkgfake.MockClient{
						MockGet: xpkgfake.NewMockGetFn(mockPackage(), nil),
					}),
					WithLinter(&MockLinter{MockLint: NewMockLintFn(nil)}),
					WithVersioner(&verfake.MockVersioner{MockInConstraints: verfake.NewMockInConstraintsFn(true, nil)}),
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

func signatureVerificationEnabled() *feature.Flags {
	f := &feature.Flags{}
	f.Enable(features.EnableAlphaSignatureVerification)

	return f
}
