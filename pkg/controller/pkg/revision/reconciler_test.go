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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/parser"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/apis/pkg/v1alpha1"
)

var _ parser.Backend = &ErrBackend{}

type ErrBackend struct{}

func (e *ErrBackend) Init(_ context.Context, _ ...parser.BackendOption) (io.ReadCloser, error) {
	return nil, errors.New("test err")
}

var _ Establisher = &MockEstablisher{}

type MockEstablisher struct {
	MockCheck           func() error
	MockEstablish       func() error
	MockGetResourceRefs func() []runtimev1alpha1.TypedReference
}

func NewMockEstablisher() *MockEstablisher {
	return &MockEstablisher{
		MockCheck:           NewMockCheckFn(nil),
		MockEstablish:       NewMockEstablishFn(nil),
		MockGetResourceRefs: NewMockGetResourceRefs(nil),
	}
}

func NewMockCheckFn(err error) func() error {
	return func() error { return err }
}

func NewMockEstablishFn(err error) func() error {
	return func() error { return err }
}

func NewMockGetResourceRefs(refs []runtimev1alpha1.TypedReference) func() []runtimev1alpha1.TypedReference {
	return func() []runtimev1alpha1.TypedReference { return refs }
}

func (e *MockEstablisher) Check(context.Context, []runtime.Object, resource.Object, bool) error {
	return e.MockCheck()
}

func (e *MockEstablisher) Establish(context.Context, resource.Object, bool) error {
	return e.MockEstablish()
}

func (e *MockEstablisher) GetResourceRefs() []runtimev1alpha1.TypedReference {
	return e.MockGetResourceRefs()
}

func (e *MockEstablisher) Reset() {}

var _ Hook = &MockHook{}

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

func TestReconcile(t *testing.T) {
	errBoom := errors.New("boom")

	metaScheme, _ := BuildMetaScheme()
	objScheme, _ := BuildObjectScheme()

	type args struct {
		req reconcile.Request
		rec *Reconciler
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
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: &Reconciler{
					newPackageRevision: func() v1alpha1.PackageRevision { return &v1alpha1.ConfigurationRevision{} },
					client: resource.ClientApplicator{
						Client: &test.MockClient{MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, ""))},
					},
					log:    logging.NewNopLogger(),
					record: event.NewNopRecorder(),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"ErrGetPackageRevision": {
			reason: "We should return an error if getting package fails.",
			args: args{
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: &Reconciler{
					newPackageRevision: func() v1alpha1.PackageRevision { return &v1alpha1.ConfigurationRevision{} },
					client: resource.ClientApplicator{
						Client: &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
					},
					log:    logging.NewNopLogger(),
					record: event.NewNopRecorder(),
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: errors.Wrap(errBoom, errGetPackageRevision),
			},
		},
		"ErrInitParserBackend": {
			reason: "We should requeue after short wait if fail to initialize parser backend.",
			args: args{
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: &Reconciler{
					newPackageRevision: func() v1alpha1.PackageRevision { return &v1alpha1.ConfigurationRevision{} },
					client: resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1alpha1.ConfigurationRevision)
								pr.SetGroupVersionKind(v1alpha1.ConfigurationRevisionGroupVersionKind)
								pr.SetDesiredState(v1alpha1.PackageRevisionActive)
								pr.SetInstallPod(runtimev1alpha1.Reference{Name: "test"})
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1alpha1.ConfigurationRevision{}
								want.SetGroupVersionKind(v1alpha1.ConfigurationRevisionGroupVersionKind)
								want.SetInstallPod(runtimev1alpha1.Reference{Name: "test"})
								want.SetDesiredState(v1alpha1.PackageRevisionActive)
								want.SetConditions(v1alpha1.Unhealthy())

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
						},
					},
					backend: &ErrBackend{},
					log:     logging.NewNopLogger(),
					record:  event.NewNopRecorder(),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"ErrParse": {
			reason: "We should requeue after short wait if fail to parse package.",
			args: args{
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: &Reconciler{
					newPackageRevision: func() v1alpha1.PackageRevision { return &v1alpha1.ConfigurationRevision{} },
					client: resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1alpha1.ConfigurationRevision)
								pr.SetGroupVersionKind(v1alpha1.ConfigurationRevisionGroupVersionKind)
								pr.SetDesiredState(v1alpha1.PackageRevisionActive)
								pr.SetInstallPod(runtimev1alpha1.Reference{Name: "test"})
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1alpha1.ConfigurationRevision{}
								want.SetGroupVersionKind(v1alpha1.ConfigurationRevisionGroupVersionKind)
								want.SetInstallPod(runtimev1alpha1.Reference{Name: "test"})
								want.SetDesiredState(v1alpha1.PackageRevisionActive)
								want.SetConditions(v1alpha1.Unhealthy())

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
						},
					},
					backend: parser.NewEchoBackend(string(providerBytes)),
					parser:  parser.New(runtime.NewScheme(), runtime.NewScheme()),
					log:     logging.NewNopLogger(),
					record:  event.NewNopRecorder(),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"ErrLint": {
			reason: "We should requeue after long wait if linting returns an error.",
			args: args{
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: &Reconciler{
					newPackageRevision: func() v1alpha1.PackageRevision { return &v1alpha1.ConfigurationRevision{} },
					client: resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1alpha1.ConfigurationRevision)
								pr.SetGroupVersionKind(v1alpha1.ConfigurationRevisionGroupVersionKind)
								pr.SetDesiredState(v1alpha1.PackageRevisionActive)
								pr.SetInstallPod(runtimev1alpha1.Reference{Name: "test"})
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1alpha1.ConfigurationRevision{}
								want.SetGroupVersionKind(v1alpha1.ConfigurationRevisionGroupVersionKind)
								want.SetInstallPod(runtimev1alpha1.Reference{Name: "test"})
								want.SetDesiredState(v1alpha1.PackageRevisionActive)
								want.SetConditions(v1alpha1.Unhealthy())

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
						},
					},
					backend: parser.NewEchoBackend(string(providerBytes)),
					linter:  NewPackageLinter(nil, ObjectLinterFns(IsConfiguration), nil),
					parser:  parser.New(metaScheme, objScheme),
					log:     logging.NewNopLogger(),
					record:  event.NewNopRecorder(),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: longWait},
			},
		},
		"ErrOneMeta": {
			reason: "We should requeue after long wait if not exactly one meta package type.",
			args: args{
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: &Reconciler{
					newPackageRevision: func() v1alpha1.PackageRevision { return &v1alpha1.ConfigurationRevision{} },
					client: resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1alpha1.ConfigurationRevision)
								pr.SetGroupVersionKind(v1alpha1.ConfigurationRevisionGroupVersionKind)
								pr.SetDesiredState(v1alpha1.PackageRevisionActive)
								pr.SetInstallPod(runtimev1alpha1.Reference{Name: "test"})
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1alpha1.ConfigurationRevision{}
								want.SetGroupVersionKind(v1alpha1.ConfigurationRevisionGroupVersionKind)
								want.SetInstallPod(runtimev1alpha1.Reference{Name: "test"})
								want.SetDesiredState(v1alpha1.PackageRevisionActive)
								want.SetConditions(v1alpha1.Unhealthy())

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
						},
					},
					backend: parser.NewNopBackend(),
					linter:  NewPackageLinter(nil, nil, nil),
					parser:  parser.New(metaScheme, objScheme),
					log:     logging.NewNopLogger(),
					record:  event.NewNopRecorder(),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: longWait},
			},
		},
		"ErrPreHook": {
			reason: "We should requeue after short wait if pre establishment hook returns an error.",
			args: args{
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: &Reconciler{
					newPackageRevision: func() v1alpha1.PackageRevision { return &v1alpha1.ProviderRevision{} },
					client: resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1alpha1.ProviderRevision)
								pr.SetGroupVersionKind(v1alpha1.ProviderRevisionGroupVersionKind)
								pr.SetDesiredState(v1alpha1.PackageRevisionActive)
								pr.SetInstallPod(runtimev1alpha1.Reference{Name: "test"})
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1alpha1.ProviderRevision{}
								want.SetGroupVersionKind(v1alpha1.ProviderRevisionGroupVersionKind)
								want.SetInstallPod(runtimev1alpha1.Reference{Name: "test"})
								want.SetDesiredState(v1alpha1.PackageRevisionActive)
								want.SetConditions(v1alpha1.Unhealthy())

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
						},
					},
					hook: &MockHook{
						MockPre: NewMockPreFn(errBoom),
					},
					backend: parser.NewEchoBackend(string(providerBytes)),
					linter:  NewPackageLinter(nil, nil, nil),
					parser:  parser.New(metaScheme, objScheme),
					log:     logging.NewNopLogger(),
					record:  event.NewNopRecorder(),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"ErrPostHook": {
			reason: "We should requeue after short wait if post establishment hook returns an error.",
			args: args{
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: &Reconciler{
					newPackageRevision: func() v1alpha1.PackageRevision { return &v1alpha1.ProviderRevision{} },
					client: resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1alpha1.ProviderRevision)
								pr.SetGroupVersionKind(v1alpha1.ProviderRevisionGroupVersionKind)
								pr.SetDesiredState(v1alpha1.PackageRevisionActive)
								pr.SetInstallPod(runtimev1alpha1.Reference{Name: "test"})
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1alpha1.ProviderRevision{}
								want.SetGroupVersionKind(v1alpha1.ProviderRevisionGroupVersionKind)
								want.SetInstallPod(runtimev1alpha1.Reference{Name: "test"})
								want.SetDesiredState(v1alpha1.PackageRevisionActive)
								want.SetConditions(v1alpha1.Unhealthy())

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
						},
					},
					hook: &MockHook{
						MockPre:  NewMockPreFn(nil),
						MockPost: NewMockPostFn(errBoom),
					},
					establisher: NewMockEstablisher(),
					backend:     parser.NewEchoBackend(string(providerBytes)),
					linter:      NewPackageLinter(nil, nil, nil),
					parser:      parser.New(metaScheme, objScheme),
					log:         logging.NewNopLogger(),
					record:      event.NewNopRecorder(),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"SuccessfulActiveRevision": {
			reason: "An active revision should establish control of all of its resources.",
			args: args{
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: &Reconciler{
					newPackageRevision: func() v1alpha1.PackageRevision { return &v1alpha1.ConfigurationRevision{} },
					client: resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1alpha1.ConfigurationRevision)
								pr.SetGroupVersionKind(v1alpha1.ConfigurationRevisionGroupVersionKind)
								pr.SetDesiredState(v1alpha1.PackageRevisionActive)
								pr.SetInstallPod(runtimev1alpha1.Reference{Name: "test"})
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1alpha1.ConfigurationRevision{}
								want.SetGroupVersionKind(v1alpha1.ConfigurationRevisionGroupVersionKind)
								want.SetInstallPod(runtimev1alpha1.Reference{Name: "test"})
								want.SetDesiredState(v1alpha1.PackageRevisionActive)
								want.SetConditions(v1alpha1.Healthy())

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
							MockDelete: test.NewMockDeleteFn(nil),
						},
					},
					hook:        NewNopHook(),
					establisher: NewMockEstablisher(),
					backend:     parser.NewEchoBackend(string(providerBytes)),
					linter:      NewPackageLinter(nil, nil, nil),
					parser:      parser.New(metaScheme, objScheme),
					log:         logging.NewNopLogger(),
					record:      event.NewNopRecorder(),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: longWait},
			},
		},
		"ErrEstablishActiveRevision": {
			reason: "An active revision that fails to establish control should requeue after short wait.",
			args: args{
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: &Reconciler{
					newPackageRevision: func() v1alpha1.PackageRevision { return &v1alpha1.ProviderRevision{} },
					client: resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1alpha1.ProviderRevision)
								pr.SetGroupVersionKind(v1alpha1.ProviderRevisionGroupVersionKind)
								pr.SetDesiredState(v1alpha1.PackageRevisionActive)
								pr.SetInstallPod(runtimev1alpha1.Reference{Name: "test"})
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1alpha1.ProviderRevision{}
								want.SetGroupVersionKind(v1alpha1.ProviderRevisionGroupVersionKind)
								want.SetInstallPod(runtimev1alpha1.Reference{Name: "test"})
								want.SetDesiredState(v1alpha1.PackageRevisionActive)
								want.SetConditions(v1alpha1.Unhealthy())

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
							MockDelete: test.NewMockDeleteFn(nil),
						},
					},
					hook: NewNopHook(),
					establisher: &MockEstablisher{
						MockCheck:           NewMockCheckFn(nil),
						MockEstablish:       NewMockEstablishFn(errBoom),
						MockGetResourceRefs: NewMockGetResourceRefs(nil),
					},
					backend: parser.NewEchoBackend(string(providerBytes)),
					linter:  NewPackageLinter(nil, nil, nil),
					parser:  parser.New(metaScheme, objScheme),
					log:     logging.NewNopLogger(),
					record:  event.NewNopRecorder(),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"SuccessfulInactiveRevision": {
			reason: "An inactive revision should establish ownership of all of its resources.",
			args: args{
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: &Reconciler{
					newPackageRevision: func() v1alpha1.PackageRevision { return &v1alpha1.ConfigurationRevision{} },
					client: resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1alpha1.ConfigurationRevision)
								pr.SetGroupVersionKind(v1alpha1.ConfigurationRevisionGroupVersionKind)
								pr.SetDesiredState(v1alpha1.PackageRevisionInactive)
								pr.SetInstallPod(runtimev1alpha1.Reference{Name: "test"})
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1alpha1.ConfigurationRevision{}
								want.SetGroupVersionKind(v1alpha1.ConfigurationRevisionGroupVersionKind)
								want.SetInstallPod(runtimev1alpha1.Reference{Name: "test"})
								want.SetDesiredState(v1alpha1.PackageRevisionInactive)
								want.SetConditions(v1alpha1.Healthy())

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
							MockDelete: test.NewMockDeleteFn(nil),
						},
					},
					hook:        NewNopHook(),
					establisher: NewMockEstablisher(),
					backend:     parser.NewEchoBackend(string(providerBytes)),
					linter:      NewPackageLinter(nil, nil, nil),
					parser:      parser.New(metaScheme, objScheme),
					log:         logging.NewNopLogger(),
					record:      event.NewNopRecorder(),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: longWait},
			},
		},
		"ErrEstablishInactiveRevision": {
			reason: "An inactive revision that fails to establish ownership should requeue after short wait.",
			args: args{
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: &Reconciler{
					newPackageRevision: func() v1alpha1.PackageRevision { return &v1alpha1.ConfigurationRevision{} },
					client: resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								pr := o.(*v1alpha1.ConfigurationRevision)
								pr.SetGroupVersionKind(v1alpha1.ConfigurationRevisionGroupVersionKind)
								pr.SetDesiredState(v1alpha1.PackageRevisionInactive)
								pr.SetInstallPod(runtimev1alpha1.Reference{Name: "test"})
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1alpha1.ConfigurationRevision{}
								want.SetGroupVersionKind(v1alpha1.ConfigurationRevisionGroupVersionKind)
								want.SetInstallPod(runtimev1alpha1.Reference{Name: "test"})
								want.SetDesiredState(v1alpha1.PackageRevisionInactive)
								want.SetConditions(v1alpha1.Unhealthy())

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
							MockDelete: test.NewMockDeleteFn(nil),
						},
					},
					hook: NewNopHook(),
					establisher: &MockEstablisher{
						MockCheck:           NewMockCheckFn(errBoom),
						MockEstablish:       NewMockEstablishFn(nil),
						MockGetResourceRefs: NewMockGetResourceRefs(nil),
					},
					backend: parser.NewEchoBackend(string(providerBytes)),
					linter:  NewPackageLinter(nil, nil, nil),
					parser:  parser.New(metaScheme, objScheme),
					log:     logging.NewNopLogger(),
					record:  event.NewNopRecorder(),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: shortWait},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := tc.args.rec.Reconcile(reconcile.Request{})

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.r, got, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
