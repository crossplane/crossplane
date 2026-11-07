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

package manager

import (
	"context"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/conditions"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
	"github.com/crossplane/crossplane-runtime/v2/pkg/xpkg"
	"github.com/crossplane/crossplane-runtime/v2/pkg/xpkg/fake"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	v1 "github.com/crossplane/crossplane/apis/v2/pkg/v1"
)

func TestPackageRevisionID(t *testing.T) {
	digest := "123456789012"

	tests := []struct {
		name       string
		generation int64
	}{
		{name: "generation 1 package creation", generation: 1},
		{name: "generation 2 spec update", generation: 2},
	}

	ids := map[int64]string{}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := packageRevisionID(digest, tc.generation)

			if got == digest {
				t.Fatalf("expected revision ID to differ from package digest")
			}

			ids[tc.generation] = got
		})
	}

	if ids[1] == ids[2] {
		t.Fatalf("expected different generations to produce different revision IDs")
	}
}

func TestReconcile(t *testing.T) {
	errBoom := errors.New("boom")
	testLog := logging.NewLogrLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(io.Discard)).WithName("testlog"))
	pullAlways := corev1.PullAlways
	trueVal := true
	revHistory := int64(1)
	digest := "1234567890123456789012345678901234567890123456789012345678901234"
	revisionName := xpkg.FriendlyID("test", packageRevisionID(digest, 0))

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
		"PackageNotFound": {
			reason: "We should not return and error and not requeue if package not found.",
			args: args{
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: &Reconciler{
					newPackage: func() v1.Package { return &v1.Configuration{} },
					kube: resource.ClientApplicator{
						Client: &test.MockClient{MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, ""))},
					},
					log:        testLog,
					conditions: conditions.ObservedGenerationPropagationManager{},
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"ErrGetPackage": {
			reason: "We should return an error if getting package fails.",
			args: args{
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: &Reconciler{
					newPackage: func() v1.Package { return &v1.Configuration{} },
					kube: resource.ClientApplicator{
						Client: &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
					},
					log:        testLog,
					conditions: conditions.ObservedGenerationPropagationManager{},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetPackage),
			},
		},
		"ErrListRevisions": {
			reason: "We should return an error if listing revisions for a package fails.",
			args: args{
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: &Reconciler{
					newPackage:             func() v1.Package { return &v1.Configuration{} },
					newPackageRevisionList: func() v1.PackageRevisionList { return &v1.ConfigurationRevisionList{} },
					kube: resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet:  test.NewMockGetFn(nil),
							MockList: test.NewMockListFn(errBoom),
						},
					},
					log:        testLog,
					record:     event.NewNopRecorder(),
					conditions: conditions.ObservedGenerationPropagationManager{},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errListRevisions),
			},
		},
		"ErrFetchPackage": {
			reason: "We should return an error if fetching the package fails.",
			args: args{
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: &Reconciler{
					newPackage:             func() v1.Package { return &v1.Configuration{} },
					newPackageRevisionList: func() v1.PackageRevisionList { return &v1.ConfigurationRevisionList{} },
					kube: resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet:  test.NewMockGetFn(nil),
							MockList: test.NewMockListFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
								want := &v1.Configuration{}
								want.SetConditions(v1.Unpacking().WithMessage(errors.Wrap(errBoom, errUnpack).Error()), v1.Unhealthy().WithMessage(errors.Wrap(errBoom, errUnpack).Error()))
								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
						},
						Applicator: resource.ApplyFn(func(_ context.Context, _ client.Object, _ ...resource.ApplyOption) error {
							return nil
						}),
					},
					log:        testLog,
					record:     event.NewNopRecorder(),
					conditions: conditions.ObservedGenerationPropagationManager{},
					pkg: &fake.MockClient{
						MockGet: fake.NewMockGetFn(nil, errBoom),
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errUnpack),
			},
		},
		"SuccessfulRewriteImage": {
			reason: "We should record the rewritten image path if an image config is used.",
			args: args{
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: &Reconciler{
					newPackage:             func() v1.Package { return &v1.Configuration{} },
					newPackageRevision:     func() v1.PackageRevision { return &v1.ConfigurationRevision{} },
					newPackageRevisionList: func() v1.PackageRevisionList { return &v1.ConfigurationRevisionList{} },
					kube: resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								p := o.(*v1.Configuration)
								p.SetName("test")
								p.SetGroupVersionKind(v1.ConfigurationGroupVersionKind)
								p.SetActivationPolicy(&v1.AutomaticActivation)
								return nil
							}),
							MockList: test.NewMockListFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
								want := &v1.Configuration{}
								want.SetName("test")
								want.SetGroupVersionKind(v1.ConfigurationGroupVersionKind)
								want.SetCurrentRevision(revisionName)
								want.SetActivationPolicy(&v1.AutomaticActivation)
								want.SetConditions(v1.Unhealthy().WithMessage("Package revision health is \"Unknown\""))
								want.SetConditions(v1.Active())
								want.SetResolvedSource("gcr.io/new/image/path:v1.0.0")
								want.SetAppliedImageConfigRefs(v1.ImageConfigRef{
									Name:   "imageConfigName",
									Reason: v1.ImageConfigReasonRewrite,
								})
								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
						},
						Applicator: resource.ApplyFn(func(_ context.Context, _ client.Object, _ ...resource.ApplyOption) error {
							return nil
						}),
					},
					pkg: &fake.MockClient{
						MockGet: fake.NewMockGetFn(&xpkg.Package{
							Digest:          "sha256:1234567890123456789012345678901234567890123456789012345678901234",
							ResolvedVersion: "v1.0.0",
							ResolvedSource:  "gcr.io/new/image/path",
							AppliedImageConfigs: []xpkg.ImageConfig{
								{Name: "imageConfigName", Reason: xpkg.ImageConfigReasonRewrite},
							},
						}, nil),
					},
					log:        testLog,
					record:     event.NewNopRecorder(),
					conditions: conditions.ObservedGenerationPropagationManager{},
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"SuccessfulNoExistingRevisionsAutoActivate": {
			reason: "We should be active and not requeue on successful creation of the first revision with auto activation.",
			args: args{
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: &Reconciler{
					newPackage:             func() v1.Package { return &v1.Configuration{} },
					newPackageRevision:     func() v1.PackageRevision { return &v1.ConfigurationRevision{} },
					newPackageRevisionList: func() v1.PackageRevisionList { return &v1.ConfigurationRevisionList{} },
					kube: resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								p := o.(*v1.Configuration)
								p.SetName("test")
								p.SetGroupVersionKind(v1.ConfigurationGroupVersionKind)
								p.SetActivationPolicy(&v1.AutomaticActivation)
								return nil
							}),
							MockList: test.NewMockListFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
								want := &v1.Configuration{}
								want.SetName("test")
								want.SetGroupVersionKind(v1.ConfigurationGroupVersionKind)
								want.SetCurrentRevision(revisionName)
								want.SetActivationPolicy(&v1.AutomaticActivation)
								want.SetConditions(v1.Unhealthy().WithMessage("Package revision health is \"Unknown\""))
								want.SetConditions(v1.Active())
								want.SetResolvedSource("xpkg.crossplane.io/test:v1.0.0")
								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
						},
						Applicator: resource.ApplyFn(func(_ context.Context, _ client.Object, _ ...resource.ApplyOption) error {
							return nil
						}),
					},
					pkg: &fake.MockClient{
						MockGet: fake.NewMockGetFn(&xpkg.Package{
							Digest:          "sha256:1234567890123456789012345678901234567890123456789012345678901234",
							Version:         "v1.0.0",
							Source:          "xpkg.crossplane.io/test",
							ResolvedVersion: "v1.0.0",
							ResolvedSource:  "xpkg.crossplane.io/test",
						}, nil),
					},
					log:        testLog,
					record:     event.NewNopRecorder(),
					conditions: conditions.ObservedGenerationPropagationManager{},
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"SuccessfulNoExistingRevisionsAutoActivatePullAlways": {
			reason: "We should be active and requeue after wait on successful creation of the first revision with auto activation and package pull policy Always.",
			args: args{
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: &Reconciler{
					newPackage:             func() v1.Package { return &v1.Configuration{} },
					newPackageRevision:     func() v1.PackageRevision { return &v1.ConfigurationRevision{} },
					newPackageRevisionList: func() v1.PackageRevisionList { return &v1.ConfigurationRevisionList{} },
					kube: resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								p := o.(*v1.Configuration)
								p.SetName("test")
								p.SetGroupVersionKind(v1.ConfigurationGroupVersionKind)
								p.SetActivationPolicy(&v1.AutomaticActivation)
								p.SetPackagePullPolicy(&pullAlways)
								return nil
							}),
							MockList: test.NewMockListFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
								want := &v1.Configuration{}
								want.SetName("test")
								want.SetGroupVersionKind(v1.ConfigurationGroupVersionKind)
								want.SetCurrentRevision(revisionName)
								want.SetActivationPolicy(&v1.AutomaticActivation)
								want.SetPackagePullPolicy(&pullAlways)
								want.SetConditions(v1.Unhealthy().WithMessage("Package revision health is \"Unknown\""))
								want.SetConditions(v1.Active())
								want.SetResolvedSource("xpkg.crossplane.io/test:v1.0.0")
								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
						},
						Applicator: resource.ApplyFn(func(_ context.Context, _ client.Object, _ ...resource.ApplyOption) error {
							return nil
						}),
					},
					pkg: &fake.MockClient{
						MockGet: fake.NewMockGetFn(&xpkg.Package{
							Digest:          "sha256:1234567890123456789012345678901234567890123456789012345678901234",
							Version:         "v1.0.0",
							Source:          "xpkg.crossplane.io/test",
							ResolvedVersion: "v1.0.0",
							ResolvedSource:  "xpkg.crossplane.io/test",
						}, nil),
					},
					log:        testLog,
					record:     event.NewNopRecorder(),
					conditions: conditions.ObservedGenerationPropagationManager{},
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: pullWait},
			},
		},
		"SuccessfulNoExistingRevisionsManualActivate": {
			reason: "We should be inactive and not requeue on successful creation of the first revision with manual activation policy.",
			args: args{
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: &Reconciler{
					newPackage:             func() v1.Package { return &v1.Configuration{} },
					newPackageRevision:     func() v1.PackageRevision { return &v1.ConfigurationRevision{} },
					newPackageRevisionList: func() v1.PackageRevisionList { return &v1.ConfigurationRevisionList{} },
					kube: resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								p := o.(*v1.Configuration)
								p.SetName("test")
								p.SetGroupVersionKind(v1.ConfigurationGroupVersionKind)
								p.SetActivationPolicy(&v1.ManualActivation)
								return nil
							}),
							MockList: test.NewMockListFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
								want := &v1.Configuration{}
								want.SetName("test")
								want.SetGroupVersionKind(v1.ConfigurationGroupVersionKind)
								want.SetActivationPolicy(&v1.ManualActivation)
								want.SetCurrentRevision(revisionName)
								want.SetConditions(v1.Unhealthy().WithMessage("Package revision health is \"Unknown\""))
								want.SetConditions(v1.Inactive().WithMessage("Package is inactive"))
								want.SetResolvedSource("xpkg.crossplane.io/test:v1.0.0")
								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
						},
						Applicator: resource.ApplyFn(func(_ context.Context, _ client.Object, _ ...resource.ApplyOption) error {
							return nil
						}),
					},
					pkg: &fake.MockClient{
						MockGet: fake.NewMockGetFn(&xpkg.Package{
							Digest:          "sha256:1234567890123456789012345678901234567890123456789012345678901234",
							Version:         "v1.0.0",
							Source:          "xpkg.crossplane.io/test",
							ResolvedVersion: "v1.0.0",
							ResolvedSource:  "xpkg.crossplane.io/test",
						}, nil),
					},
					log:        testLog,
					record:     event.NewNopRecorder(),
					conditions: conditions.ObservedGenerationPropagationManager{},
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"SuccessfulActiveRevisionExists": {
			reason: "We should match revision health and not requeue when active revision already exists.",
			args: args{
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: &Reconciler{
					newPackage:             func() v1.Package { return &v1.Configuration{} },
					newPackageRevision:     func() v1.PackageRevision { return &v1.ConfigurationRevision{} },
					newPackageRevisionList: func() v1.PackageRevisionList { return &v1.ConfigurationRevisionList{} },
					kube: resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								p := o.(*v1.Configuration)
								p.SetName("test")
								p.SetGroupVersionKind(v1.ConfigurationGroupVersionKind)
								return nil
							}),
							MockList: test.NewMockListFn(nil, func(o client.ObjectList) error {
								l := o.(*v1.ConfigurationRevisionList)
								cr := v1.ConfigurationRevision{
									ObjectMeta: metav1.ObjectMeta{
										Name: revisionName,
									},
								}
								cr.SetConditions(v1.RevisionHealthy())
								cr.SetDesiredState(v1.PackageRevisionActive)
								cr.SetRevision(1)
								c := v1.ConfigurationRevisionList{
									Items: []v1.ConfigurationRevision{cr},
								}
								*l = c
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
								want := &v1.Configuration{}
								want.SetName("test")
								want.SetGroupVersionKind(v1.ConfigurationGroupVersionKind)
								want.SetCurrentRevision(revisionName)
								want.SetConditions(v1.Healthy())
								want.SetConditions(v1.Active())
								want.SetResolvedSource("xpkg.crossplane.io/test:v1.0.0")
								if diff := cmp.Diff(want, o, test.EquateConditions()); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
						},
						Applicator: resource.ApplyFn(func(_ context.Context, _ client.Object, _ ...resource.ApplyOption) error {
							return nil
						}),
					},
					pkg: &fake.MockClient{
						MockGet: fake.NewMockGetFn(&xpkg.Package{
							Digest:          "sha256:1234567890123456789012345678901234567890123456789012345678901234",
							Version:         "v1.0.0",
							Source:          "xpkg.crossplane.io/test",
							ResolvedVersion: "v1.0.0",
							ResolvedSource:  "xpkg.crossplane.io/test",
						}, nil),
					},
					log:        testLog,
					record:     event.NewNopRecorder(),
					conditions: conditions.ObservedGenerationPropagationManager{},
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"SuccessfulRevisionExistsNeedsActive": {
			reason: "We should match revision health, set to active, and not requeue when inactive revision already exists and activation policy is automatic.",
			args: args{
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: &Reconciler{
					newPackage:             func() v1.Package { return &v1.Configuration{} },
					newPackageRevision:     func() v1.PackageRevision { return &v1.ConfigurationRevision{} },
					newPackageRevisionList: func() v1.PackageRevisionList { return &v1.ConfigurationRevisionList{} },
					kube: resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								p := o.(*v1.Configuration)
								p.SetName("test")
								p.SetGroupVersionKind(v1.ConfigurationGroupVersionKind)
								return nil
							}),
							MockList: test.NewMockListFn(nil, func(o client.ObjectList) error {
								l := o.(*v1.ConfigurationRevisionList)
								cr := v1.ConfigurationRevision{
									ObjectMeta: metav1.ObjectMeta{
										Name: revisionName,
									},
								}
								cr.SetGroupVersionKind(v1.ConfigurationRevisionGroupVersionKind)
								cr.SetConditions(v1.RevisionHealthy())
								cr.SetDesiredState(v1.PackageRevisionInactive)
								cr.SetRevision(1)
								c := v1.ConfigurationRevisionList{
									Items: []v1.ConfigurationRevision{cr},
								}
								*l = c
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
								want := &v1.Configuration{}
								want.SetName("test")
								want.SetGroupVersionKind(v1.ConfigurationGroupVersionKind)
								want.SetCurrentRevision(revisionName)
								want.SetConditions(v1.Healthy())
								want.SetConditions(v1.Active())
								want.SetResolvedSource("xpkg.crossplane.io/test:v1.0.0")
								if diff := cmp.Diff(want, o, test.EquateConditions()); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
						},
						Applicator: resource.ApplyFn(func(_ context.Context, o client.Object, _ ...resource.ApplyOption) error {
							want := &v1.ConfigurationRevision{}
							want.SetLabels(map[string]string{"pkg.crossplane.io/package": "test"})
							want.SetName(revisionName)
							want.SetOwnerReferences([]metav1.OwnerReference{{
								APIVersion:         v1.SchemeGroupVersion.String(),
								Kind:               v1.ConfigurationKind,
								Name:               "test",
								Controller:         &trueVal,
								BlockOwnerDeletion: &trueVal,
							}})
							want.SetGroupVersionKind(v1.ConfigurationRevisionGroupVersionKind)
							want.SetDesiredState(v1.PackageRevisionActive)
							want.SetConditions(v1.RevisionHealthy())
							want.SetRevision(1)
							if diff := cmp.Diff(want, o, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					pkg: &fake.MockClient{
						MockGet: fake.NewMockGetFn(&xpkg.Package{
							Digest:          "sha256:1234567890123456789012345678901234567890123456789012345678901234",
							Version:         "v1.0.0",
							Source:          "xpkg.crossplane.io/test",
							ResolvedVersion: "v1.0.0",
							ResolvedSource:  "xpkg.crossplane.io/test",
						}, nil),
					},
					log:        testLog,
					record:     event.NewNopRecorder(),
					conditions: conditions.ObservedGenerationPropagationManager{},
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"ErrUpdatePackageRevision": {
			reason: "Failing to update a package revision should cause us to return an error.",
			args: args{
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: &Reconciler{
					newPackage:             func() v1.Package { return &v1.Configuration{} },
					newPackageRevision:     func() v1.PackageRevision { return &v1.ConfigurationRevision{} },
					newPackageRevisionList: func() v1.PackageRevisionList { return &v1.ConfigurationRevisionList{} },
					kube: resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								p := o.(*v1.Configuration)
								p.SetName("test")
								p.SetGroupVersionKind(v1.ConfigurationGroupVersionKind)
								return nil
							}),
							MockList: test.NewMockListFn(nil, func(o client.ObjectList) error {
								l := o.(*v1.ConfigurationRevisionList)
								cr := v1.ConfigurationRevision{
									ObjectMeta: metav1.ObjectMeta{
										Name: revisionName,
									},
								}
								cr.SetGroupVersionKind(v1.ConfigurationRevisionGroupVersionKind)
								cr.SetRevision(1)
								cr.SetConditions(v1.RevisionHealthy())
								cr.SetDesiredState(v1.PackageRevisionInactive)
								c := v1.ConfigurationRevisionList{
									Items: []v1.ConfigurationRevision{cr},
								}
								*l = c
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
								want := &v1.Configuration{}
								want.SetName("test")
								want.SetGroupVersionKind(v1.ConfigurationGroupVersionKind)
								want.SetCurrentRevision(revisionName)
								want.SetConditions(v1.Healthy())
								if diff := cmp.Diff(want, o, test.EquateConditions()); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
						},
						Applicator: resource.ApplyFn(func(_ context.Context, _ client.Object, _ ...resource.ApplyOption) error {
							return errBoom
						}),
					},
					pkg: &fake.MockClient{
						MockGet: fake.NewMockGetFn(&xpkg.Package{
							Digest:          "sha256:1234567890123456789012345678901234567890123456789012345678901234",
							Version:         "v1.0.0",
							Source:          "xpkg.crossplane.io/test",
							ResolvedVersion: "v1.0.0",
							ResolvedSource:  "xpkg.crossplane.io/test",
						}, nil),
					},
					log:        testLog,
					record:     event.NewNopRecorder(),
					conditions: conditions.ObservedGenerationPropagationManager{},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errApplyPackageRevision),
			},
		},
		"SuccessfulTransitionUnhealthy": {
			reason: "If the current revision is unhealthy the package should be also.",
			args: args{
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: &Reconciler{
					newPackage:             func() v1.Package { return &v1.Configuration{} },
					newPackageRevision:     func() v1.PackageRevision { return &v1.ConfigurationRevision{} },
					newPackageRevisionList: func() v1.PackageRevisionList { return &v1.ConfigurationRevisionList{} },
					kube: resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								p := o.(*v1.Configuration)
								p.SetName("test")
								p.SetGroupVersionKind(v1.ConfigurationGroupVersionKind)
								return nil
							}),
							MockList: test.NewMockListFn(nil, func(o client.ObjectList) error {
								l := o.(*v1.ConfigurationRevisionList)
								cr := v1.ConfigurationRevision{
									ObjectMeta: metav1.ObjectMeta{
										Name: revisionName,
									},
								}
								cr.SetGroupVersionKind(v1.ConfigurationRevisionGroupVersionKind)
								cr.SetRevision(1)
								cr.SetConditions(v1.RevisionUnhealthy().WithMessage("some message"))
								cr.SetDesiredState(v1.PackageRevisionActive)
								c := v1.ConfigurationRevisionList{
									Items: []v1.ConfigurationRevision{cr},
								}
								*l = c
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
								want := &v1.Configuration{}
								want.SetName("test")
								want.SetGroupVersionKind(v1.ConfigurationGroupVersionKind)
								want.SetCurrentRevision(revisionName)
								want.SetConditions(v1.Unhealthy().WithMessage("Package revision health is \"False\" with message: some message"))
								want.SetConditions(v1.Active())
								want.SetResolvedSource("xpkg.crossplane.io/test:v1.0.0")
								if diff := cmp.Diff(want, o, test.EquateConditions()); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
						},
						Applicator: resource.ApplyFn(func(_ context.Context, _ client.Object, _ ...resource.ApplyOption) error {
							return nil
						}),
					},
					pkg: &fake.MockClient{
						MockGet: fake.NewMockGetFn(&xpkg.Package{
							Digest:          "sha256:1234567890123456789012345678901234567890123456789012345678901234",
							Version:         "v1.0.0",
							Source:          "xpkg.crossplane.io/test",
							ResolvedVersion: "v1.0.0",
							ResolvedSource:  "xpkg.crossplane.io/test",
						}, nil),
					},
					log:        testLog,
					record:     event.NewNopRecorder(),
					conditions: conditions.ObservedGenerationPropagationManager{},
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"SuccessfulRevisionExistsNeedGC": {
			reason: "We should successfully garbage collect when an old revision falls outside range.",
			args: args{
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: &Reconciler{
					newPackage:             func() v1.Package { return &v1.Configuration{} },
					newPackageRevision:     func() v1.PackageRevision { return &v1.ConfigurationRevision{} },
					newPackageRevisionList: func() v1.PackageRevisionList { return &v1.ConfigurationRevisionList{} },
					kube: resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								p := o.(*v1.Configuration)
								p.SetName("test")
								p.SetGroupVersionKind(v1.ConfigurationGroupVersionKind)
								return nil
							}),
							MockList: test.NewMockListFn(nil, func(o client.ObjectList) error {
								l := o.(*v1.ConfigurationRevisionList)
								cr := v1.ConfigurationRevision{
									ObjectMeta: metav1.ObjectMeta{
										Name: revisionName,
									},
								}
								cr.SetRevision(3)
								cr.SetGroupVersionKind(v1.ConfigurationRevisionGroupVersionKind)
								cr.SetConditions(v1.RevisionHealthy())
								cr.SetDesiredState(v1.PackageRevisionInactive)
								c := v1.ConfigurationRevisionList{
									Items: []v1.ConfigurationRevision{
										cr,
										{
											ObjectMeta: metav1.ObjectMeta{
												Name: "made-the-cut",
											},
											Spec: v1.PackageRevisionSpec{
												Revision: 2,
											},
										},
										{
											ObjectMeta: metav1.ObjectMeta{
												Name: "missed-the-cut",
											},
											Spec: v1.PackageRevisionSpec{
												Revision: 1,
											},
										},
									},
								}
								*l = c
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
								want := &v1.Configuration{}
								want.SetName("test")
								want.SetGroupVersionKind(v1.ConfigurationGroupVersionKind)
								want.SetCurrentRevision(revisionName)
								want.SetConditions(v1.Healthy())
								want.SetConditions(v1.Active())
								want.SetResolvedSource("xpkg.crossplane.io/test:v1.0.0")
								if diff := cmp.Diff(want, o, test.EquateConditions()); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
							MockDelete: test.NewMockDeleteFn(nil),
						},
						Applicator: resource.ApplyFn(func(_ context.Context, o client.Object, _ ...resource.ApplyOption) error {
							want := &v1.ConfigurationRevision{}
							want.SetLabels(map[string]string{"pkg.crossplane.io/package": "test"})
							want.SetName(revisionName)
							want.SetOwnerReferences([]metav1.OwnerReference{{
								APIVersion:         v1.SchemeGroupVersion.String(),
								Kind:               v1.ConfigurationKind,
								Name:               "test",
								Controller:         &trueVal,
								BlockOwnerDeletion: &trueVal,
							}})
							want.SetGroupVersionKind(v1.ConfigurationRevisionGroupVersionKind)
							want.SetDesiredState(v1.PackageRevisionActive)
							want.SetConditions(v1.RevisionHealthy())
							want.SetRevision(3)
							if diff := cmp.Diff(want, o, test.EquateConditions()); diff != "" {
								t.Errorf("-want, +got:\n%s", diff)
							}
							return nil
						}),
					},
					pkg: &fake.MockClient{
						MockGet: fake.NewMockGetFn(&xpkg.Package{
							Digest:          "sha256:1234567890123456789012345678901234567890123456789012345678901234",
							Version:         "v1.0.0",
							Source:          "xpkg.crossplane.io/test",
							ResolvedVersion: "v1.0.0",
							ResolvedSource:  "xpkg.crossplane.io/test",
						}, nil),
					},
					log:        testLog,
					record:     event.NewNopRecorder(),
					conditions: conditions.ObservedGenerationPropagationManager{},
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"ErrGC": {
			reason: "Failure to garbage collect old package revision should cause return an error.",
			args: args{
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: &Reconciler{
					newPackage:             func() v1.Package { return &v1.Configuration{} },
					newPackageRevision:     func() v1.PackageRevision { return &v1.ConfigurationRevision{} },
					newPackageRevisionList: func() v1.PackageRevisionList { return &v1.ConfigurationRevisionList{} },
					kube: resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								p := o.(*v1.Configuration)
								p.SetName("test")
								p.SetGroupVersionKind(v1.ConfigurationGroupVersionKind)
								p.SetRevisionHistoryLimit(&revHistory)
								return nil
							}),
							MockList: test.NewMockListFn(nil, func(o client.ObjectList) error {
								l := o.(*v1.ConfigurationRevisionList)
								cr := v1.ConfigurationRevision{
									ObjectMeta: metav1.ObjectMeta{
										Name: revisionName,
									},
								}
								cr.SetRevision(3)
								cr.SetGroupVersionKind(v1.ConfigurationRevisionGroupVersionKind)
								cr.SetConditions(v1.RevisionHealthy())
								cr.SetDesiredState(v1.PackageRevisionInactive)
								c := v1.ConfigurationRevisionList{
									Items: []v1.ConfigurationRevision{
										cr,
										{
											ObjectMeta: metav1.ObjectMeta{
												Name: "made-the-cut",
											},
											Spec: v1.PackageRevisionSpec{
												Revision:     2,
												DesiredState: v1.PackageRevisionInactive,
											},
										},
										{
											ObjectMeta: metav1.ObjectMeta{
												Name: "missed-the-cut",
											},
											Spec: v1.PackageRevisionSpec{
												Revision:     1,
												DesiredState: v1.PackageRevisionInactive,
											},
										},
									},
								}
								*l = c
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
								want := &v1.Configuration{}
								want.SetName("test")
								want.SetGroupVersionKind(v1.ConfigurationGroupVersionKind)
								want.SetCurrentRevision(revisionName)
								want.SetRevisionHistoryLimit(&revHistory)
								if diff := cmp.Diff(want, o, test.EquateConditions()); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
							MockDelete: test.NewMockDeleteFn(errBoom),
						},
					},
					pkg: &fake.MockClient{
						MockGet: fake.NewMockGetFn(&xpkg.Package{
							Digest:          "sha256:1234567890123456789012345678901234567890123456789012345678901234",
							Version:         "v1.0.0",
							Source:          "xpkg.crossplane.io/test",
							ResolvedVersion: "v1.0.0",
							ResolvedSource:  "xpkg.crossplane.io/test",
						}, nil),
					},
					log:        testLog,
					record:     event.NewNopRecorder(),
					conditions: conditions.ObservedGenerationPropagationManager{},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGCPackageRevision),
			},
		},
		"PauseReconcile": {
			reason: "Pause reconciliation if the pause annotation is set",
			args: args{
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: &Reconciler{
					newPackage:             func() v1.Package { return &v1.Configuration{} },
					newPackageRevision:     func() v1.PackageRevision { return &v1.ConfigurationRevision{} },
					newPackageRevisionList: func() v1.PackageRevisionList { return &v1.ConfigurationRevisionList{} },
					kube: resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								p := o.(*v1.Configuration)
								p.SetName("test")
								p.SetGroupVersionKind(v1.ConfigurationGroupVersionKind)
								p.SetActivationPolicy(&v1.AutomaticActivation)
								p.SetAnnotations(map[string]string{
									meta.AnnotationKeyReconciliationPaused: "true",
								})
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
								want := &v1.Configuration{}
								want.SetName("test")
								want.SetAnnotations(map[string]string{
									meta.AnnotationKeyReconciliationPaused: "true",
								})
								want.SetGroupVersionKind(v1.ConfigurationGroupVersionKind)
								want.SetActivationPolicy(&v1.AutomaticActivation)
								want.SetConditions(xpv2.ReconcilePaused().WithMessage(reconcilePausedMsg))
								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
						},
					},
					pkg: &fake.MockClient{
						MockGet: fake.NewMockGetFn(&xpkg.Package{
							Digest:          "sha256:1234567890123456789012345678901234567890123456789012345678901234",
							Version:         "v1.0.0",
							Source:          "xpkg.crossplane.io/test",
							ResolvedVersion: "v1.0.0",
							ResolvedSource:  "xpkg.crossplane.io/test",
						}, nil),
					},
					log:        testLog,
					record:     event.NewNopRecorder(),
					conditions: conditions.ObservedGenerationPropagationManager{},
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"ResumeReconcile": {
			reason: "We should be active and not requeue on successful creation of the first revision with auto activation.",
			args: args{
				req: reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}},
				rec: &Reconciler{
					newPackage:             func() v1.Package { return &v1.Configuration{} },
					newPackageRevision:     func() v1.PackageRevision { return &v1.ConfigurationRevision{} },
					newPackageRevisionList: func() v1.PackageRevisionList { return &v1.ConfigurationRevisionList{} },
					kube: resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								p := o.(*v1.Configuration)
								p.SetName("test")
								p.SetGroupVersionKind(v1.ConfigurationGroupVersionKind)
								p.SetActivationPolicy(&v1.AutomaticActivation)
								p.SetConditions(xpv2.ReconcilePaused())
								return nil
							}),
							MockList: test.NewMockListFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
								want := &v1.Configuration{}
								want.SetName("test")
								want.SetGroupVersionKind(v1.ConfigurationGroupVersionKind)
								want.SetActivationPolicy(&v1.AutomaticActivation)
								want.Status.Conditions = []xpv2.Condition{}
								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
						},
						Applicator: resource.ApplyFn(func(_ context.Context, _ client.Object, _ ...resource.ApplyOption) error {
							return nil
						}),
					},
					pkg: &fake.MockClient{
						MockGet: fake.NewMockGetFn(&xpkg.Package{
							Digest:          "sha256:1234567890123456789012345678901234567890123456789012345678901234",
							Version:         "v1.0.0",
							Source:          "xpkg.crossplane.io/test",
							ResolvedVersion: "v1.0.0",
							ResolvedSource:  "xpkg.crossplane.io/test",
						}, nil),
					},
					log:        testLog,
					record:     event.NewNopRecorder(),
					conditions: conditions.ObservedGenerationPropagationManager{},
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := tc.args.rec.Reconcile(context.Background(), reconcile.Request{})
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.r, got, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestReconcileExternalRevisions(t *testing.T) {
	testLog := logging.NewLogrLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(io.Discard)).WithName("testlog"))

	rev := func(name string, c ...xpv2.Condition) v1.FunctionRevision {
		r := v1.FunctionRevision{ObjectMeta: metav1.ObjectMeta{Name: name}}
		r.SetConditions(c...)
		return r
	}
	healthy := func(name string) v1.FunctionRevision {
		return rev(name, v1.RevisionHealthy(), v1.RuntimeHealthy())
	}
	unhealthy := func(name string) v1.FunctionRevision {
		return rev(name, v1.RevisionUnhealthy())
	}

	type args struct {
		revs []v1.FunctionRevision
	}

	type want struct {
		active xpv2.Condition
		health xpv2.Condition
		refs   []corev1.LocalObjectReference
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoRevisions": {
			reason: "A Function with no external revisions should be inactive.",
			args:   args{},
			want: want{
				active: v1.Inactive().WithMessage("Package has no external revisions"),
				health: v1.UnknownHealth(),
			},
		},
		"AllHealthy": {
			reason: "A Function should be healthy if all its external revisions are healthy.",
			args: args{
				revs: []v1.FunctionRevision{healthy("b"), healthy("a")},
			},
			want: want{
				active: v1.Active(),
				health: v1.Healthy(),
				refs:   []corev1.LocalObjectReference{{Name: "a"}, {Name: "b"}},
			},
		},
		"NotYetHealthyAndHealthy": {
			reason: "A Function should be unhealthy if one of its external revisions hasn't reported health yet, even if another is healthy.",
			args: args{
				revs: []v1.FunctionRevision{healthy("a"), rev("b")},
			},
			want: want{
				active: v1.Active(),
				health: v1.PackageHealth(&v1.FunctionRevision{}),
				refs:   []corev1.LocalObjectReference{{Name: "a"}, {Name: "b"}},
			},
		},
		"HealthyAndUnhealthy": {
			reason: "A Function should be unhealthy if any of its external revisions is unhealthy, regardless of order.",
			args: args{
				revs: []v1.FunctionRevision{healthy("a"), unhealthy("b"), healthy("c")},
			},
			want: want{
				active: v1.Active(),
				health: v1.PackageHealth(&v1.FunctionRevision{Status: unhealthy("b").Status}),
				refs:   []corev1.LocalObjectReference{{Name: "a"}, {Name: "b"}, {Name: "c"}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var got *v1.Function

			rec := &Reconciler{
				newPackage:             func() v1.Package { return &v1.Function{} },
				newPackageRevision:     func() v1.PackageRevision { return &v1.FunctionRevision{} },
				newPackageRevisionList: func() v1.PackageRevisionList { return &v1.FunctionRevisionList{} },
				kube: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							o.SetName("test")
							o.SetUID("test-uid")
							return nil
						}),
						MockList: test.NewMockListFn(nil, func(o client.ObjectList) error {
							o.(*v1.FunctionRevisionList).Items = tc.args.revs
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
							got = o.(*v1.Function)
							return nil
						}),
					},
				},
				log:        testLog,
				record:     event.NewNopRecorder(),
				conditions: conditions.ObservedGenerationPropagationManager{},
			}

			if _, err := rec.Reconcile(context.Background(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}}); err != nil {
				t.Fatalf("\n%s\nr.Reconcile(...): %s", tc.reason, err)
			}

			if diff := cmp.Diff(tc.want.active, got.GetCondition(v1.TypeInstalled), test.EquateConditions()); diff != "" {
				t.Errorf("\n%s\nInstalled condition: -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.health, got.GetCondition(v1.TypeHealthy), test.EquateConditions()); diff != "" {
				t.Errorf("\n%s\nHealthy condition: -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.refs, got.Status.ExternalRevisionRefs); diff != "" {
				t.Errorf("\n%s\nstatus.externalRevisionRefs: -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestReconcileIgnoresUncontrolledFunctionRevisions(t *testing.T) {
	testLog := logging.NewLogrLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(io.Discard)).WithName("testlog"))
	digest := "1234567890123456789012345678901234567890123456789012345678901234"
	revisionName := xpkg.FriendlyID("test", packageRevisionID(digest, 0))

	// The Function has a package and an active revision it controls. An
	// external revision has also been created for it. We should never
	// deactivate (or otherwise touch) the external revision, but we should
	// record it in the Function's status.
	var got []corev1.LocalObjectReference
	rec := &Reconciler{
		newPackage:             func() v1.Package { return &v1.Function{} },
		newPackageRevision:     func() v1.PackageRevision { return &v1.FunctionRevision{} },
		newPackageRevisionList: func() v1.PackageRevisionList { return &v1.FunctionRevisionList{} },
		kube: resource.ClientApplicator{
			Client: &test.MockClient{
				MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
					if p, ok := o.(*v1.Function); ok {
						p.SetName("test")
						p.SetUID("test-uid")
						p.SetGroupVersionKind(v1.FunctionGroupVersionKind)
						p.Spec.Package = "xpkg.crossplane.io/test:v1.0.0"
					}
					return nil
				}),
				MockList: test.NewMockListFn(nil, func(o client.ObjectList) error {
					managed := v1.FunctionRevision{
						ObjectMeta: metav1.ObjectMeta{
							Name: revisionName,
							OwnerReferences: []metav1.OwnerReference{{
								APIVersion: v1.FunctionGroupVersionKind.GroupVersion().String(),
								Kind:       v1.FunctionKind,
								Name:       "test",
								UID:        "test-uid",
								Controller: new(true),
							}},
						},
					}
					managed.SetConditions(v1.RevisionHealthy(), v1.RuntimeHealthy())
					managed.SetDesiredState(v1.PackageRevisionActive)
					managed.SetRevision(1)

					external := v1.FunctionRevision{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-external",
						},
					}
					external.SetDesiredState(v1.PackageRevisionActive)
					external.SetRevision(1)

					o.(*v1.FunctionRevisionList).Items = []v1.FunctionRevision{managed, external}
					return nil
				}),
				MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
					got = o.(*v1.Function).Status.ExternalRevisionRefs
					return nil
				}),
			},
			Applicator: resource.ApplyFn(func(_ context.Context, o client.Object, _ ...resource.ApplyOption) error {
				if o.GetName() == "test-external" {
					t.Errorf("Apply(...): unexpected call for external revision %q", o.GetName())
				}
				return nil
			}),
		},
		pkg: &fake.MockClient{
			MockGet: fake.NewMockGetFn(&xpkg.Package{
				Digest:          "sha256:" + digest,
				Version:         "v1.0.0",
				Source:          "xpkg.crossplane.io/test",
				ResolvedVersion: "v1.0.0",
				ResolvedSource:  "xpkg.crossplane.io/test",
			}, nil),
		},
		log:        testLog,
		record:     event.NewNopRecorder(),
		conditions: conditions.ObservedGenerationPropagationManager{},
	}

	if _, err := rec.Reconcile(context.Background(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "test"}}); err != nil {
		t.Fatalf("r.Reconcile(...): %s", err)
	}

	want := []corev1.LocalObjectReference{{Name: "test-external"}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("status.externalRevisionRefs: -want, +got:\n%s", diff)
	}
}
