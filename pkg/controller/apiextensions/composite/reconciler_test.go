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

package composite

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/crossplane/crossplane/apis/apiextensions/v1beta1"
)

func TestReconcile(t *testing.T) {
	errBoom := errors.New("boom")
	cd := managed.ConnectionDetails{"a": []byte("b")}

	type args struct {
		mgr  manager.Manager
		of   resource.CompositeKind
		opts []ReconcilerOption
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
		"CompositeResourceNotFound": {
			reason: "We should not return an error if the composite resource was not found.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
						},
					}),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"GetCompositeResourceError": {
			reason: "We should return errors encountered while getting the composite resource.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(errBoom),
						},
					}),
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: errors.Wrap(errBoom, errGet),
			},
		},
		"SelectCompositionError": {
			reason: "We should requeue after a short wait if we encounter an error while selecting a composition.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil),
						},
					}),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, _ resource.Composite) error {
						return errBoom
					})),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"GetCompositionError": {
			reason: "We should requeue after a short wait if we encounter an error while getting a composition.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
								if _, ok := obj.(*v1beta1.Composition); ok {
									return errBoom
								}
								return nil
							}),
						},
					}),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"ConfigureCompositeError": {
			reason: "We should requeue after a short wait if we encounter an error while configuring the composite resource.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil),
						},
					}),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithConfigurator(ConfiguratorFn(func(ctx context.Context, cr resource.Composite, cp *v1beta1.Composition) error {
						return errBoom
					})),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"RenderComposedError": {
			reason: "We should requeue after a short wait if we encounter an error while rendering a composed resource.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
								if comp, ok := obj.(*v1beta1.Composition); ok {
									comp.Spec.Resources = []v1beta1.ComposedTemplate{{}}
								}
								return nil
							}),
						},
					}),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithConfigurator(ConfiguratorFn(func(ctx context.Context, cr resource.Composite, cp *v1beta1.Composition) error {
						return nil
					})),
					WithRenderer(RendererFn(func(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1beta1.ComposedTemplate) error {
						return errBoom
					})),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"UpdateCompositeError": {
			reason: "We should requeue after a short wait if we encounter an error while updating our composite resource with references.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
								if comp, ok := obj.(*v1beta1.Composition); ok {
									comp.Spec.Resources = []v1beta1.ComposedTemplate{{}}
								}
								return nil
							}),
							MockUpdate: test.NewMockUpdateFn(errBoom),
						},
					}),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithConfigurator(ConfiguratorFn(func(ctx context.Context, cr resource.Composite, cp *v1beta1.Composition) error {
						return nil
					})),
					WithRenderer(RendererFn(func(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1beta1.ComposedTemplate) error {
						return nil
					})),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"ApplyComposedError": {
			reason: "We should requeue after a short wait if we encounter an error while applying a composed resource.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
								if comp, ok := obj.(*v1beta1.Composition); ok {
									comp.Spec.Resources = []v1beta1.ComposedTemplate{{}}
								}
								return nil
							}),
							MockUpdate: test.NewMockUpdateFn(nil),
						},
						Applicator: resource.ApplyFn(func(c context.Context, r runtime.Object, ao ...resource.ApplyOption) error {
							return errBoom
						}),
					}),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithConfigurator(ConfiguratorFn(func(ctx context.Context, cr resource.Composite, cp *v1beta1.Composition) error {
						return nil
					})),
					WithRenderer(RendererFn(func(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1beta1.ComposedTemplate) error {
						return nil
					})),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"FetchConnectionDetailsError": {
			reason: "We should requeue after a short wait if we encounter an error while fetching a composed resource's connection details.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
								if comp, ok := obj.(*v1beta1.Composition); ok {
									comp.Spec.Resources = []v1beta1.ComposedTemplate{{}}
								}
								return nil
							}),
							MockUpdate: test.NewMockUpdateFn(nil),
						},
						Applicator: resource.ApplyFn(func(c context.Context, r runtime.Object, ao ...resource.ApplyOption) error {
							return nil
						}),
					}),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithConfigurator(ConfiguratorFn(func(ctx context.Context, cr resource.Composite, cp *v1beta1.Composition) error {
						return nil
					})),
					WithRenderer(RendererFn(func(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1beta1.ComposedTemplate) error {
						return nil
					})),
					WithConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, cd resource.Composed, t v1beta1.ComposedTemplate) (managed.ConnectionDetails, error) {
						return nil, errBoom
					})),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"CheckReadinessError": {
			reason: "We should requeue after a short wait if we encounter an error while checking whether a composed resource is ready.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
								if comp, ok := obj.(*v1beta1.Composition); ok {
									comp.Spec.Resources = []v1beta1.ComposedTemplate{{}}
								}
								return nil
							}),
							MockUpdate: test.NewMockUpdateFn(nil),
						},
						Applicator: resource.ApplyFn(func(c context.Context, r runtime.Object, ao ...resource.ApplyOption) error {
							return nil
						}),
					}),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithConfigurator(ConfiguratorFn(func(ctx context.Context, cr resource.Composite, cp *v1beta1.Composition) error {
						return nil
					})),
					WithRenderer(RendererFn(func(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1beta1.ComposedTemplate) error {
						return nil
					})),
					WithConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, cd resource.Composed, t v1beta1.ComposedTemplate) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithReadinessChecker(ReadinessCheckerFn(func(ctx context.Context, cd resource.Composed, t v1beta1.ComposedTemplate) (ready bool, err error) {
						return false, errBoom
					})),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"PublishConnectionDetailsError": {
			reason: "We should requeue after a short wait if we encounter an error while publishing connection details.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet:    test.NewMockGetFn(nil),
							MockUpdate: test.NewMockUpdateFn(nil),
						},
						Applicator: resource.ApplyFn(func(c context.Context, r runtime.Object, ao ...resource.ApplyOption) error {
							return nil
						}),
					}),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithConfigurator(ConfiguratorFn(func(ctx context.Context, cr resource.Composite, cp *v1beta1.Composition) error {
						return nil
					})),
					WithConnectionPublisher(ConnectionPublisherFn(func(ctx context.Context, o resource.ConnectionSecretOwner, c managed.ConnectionDetails) (published bool, err error) {
						return false, errBoom
					})),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"ComposedResourcesNotReady": {
			reason: "We should requeue after a short wait if any of our composed resources are not yet ready.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
								if comp, ok := obj.(*v1beta1.Composition); ok {
									comp.Spec.Resources = []v1beta1.ComposedTemplate{{}}
								}
								return nil
							}),
							MockUpdate:       test.NewMockUpdateFn(nil),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil),
						},
						Applicator: resource.ApplyFn(func(c context.Context, r runtime.Object, ao ...resource.ApplyOption) error {
							return nil
						}),
					}),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithRenderer(RendererFn(func(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1beta1.ComposedTemplate) error {
						return nil
					})),
					WithConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, cd resource.Composed, t v1beta1.ComposedTemplate) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithReadinessChecker(ReadinessCheckerFn(func(ctx context.Context, cd resource.Composed, t v1beta1.ComposedTemplate) (ready bool, err error) {
						// Our one resource is not ready.
						return false, nil
					})),
					WithConfigurator(ConfiguratorFn(func(ctx context.Context, cr resource.Composite, cp *v1beta1.Composition) error {
						return nil
					})),
					WithConnectionPublisher(ConnectionPublisherFn(func(ctx context.Context, o resource.ConnectionSecretOwner, c managed.ConnectionDetails) (published bool, err error) {
						return false, nil
					})),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"ComposedResourcesReady": {
			reason: "We should requeue after a long wait if all of our composed resources are ready.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj runtime.Object) error {
								if comp, ok := obj.(*v1beta1.Composition); ok {
									comp.Spec.Resources = []v1beta1.ComposedTemplate{{}}
								}
								return nil
							}),
							MockUpdate:       test.NewMockUpdateFn(nil),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil),
						},
						Applicator: resource.ApplyFn(func(c context.Context, r runtime.Object, ao ...resource.ApplyOption) error {
							return nil
						}),
					}),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithRenderer(RendererFn(func(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1beta1.ComposedTemplate) error {
						return nil
					})),
					WithConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, _ resource.Composed, t v1beta1.ComposedTemplate) (managed.ConnectionDetails, error) {
						return cd, nil
					})),
					WithReadinessChecker(ReadinessCheckerFn(func(ctx context.Context, cd resource.Composed, t v1beta1.ComposedTemplate) (ready bool, err error) {
						// Our one resource is ready.
						return true, nil
					})),
					WithConfigurator(ConfiguratorFn(func(ctx context.Context, cr resource.Composite, cp *v1beta1.Composition) error {
						return nil
					})),
					WithConnectionPublisher(ConnectionPublisherFn(func(ctx context.Context, o resource.ConnectionSecretOwner, got managed.ConnectionDetails) (published bool, err error) {
						want := cd
						if diff := cmp.Diff(want, got); diff != "" {
							t.Errorf("PublishConnection(...): -want, +got:\n%s", diff)
						}
						return true, nil
					})),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: longWait},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewReconciler(tc.args.mgr, tc.args.of, tc.args.opts...)
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
