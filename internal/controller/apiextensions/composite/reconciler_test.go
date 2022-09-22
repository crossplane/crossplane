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
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
)

func TestReconcile(t *testing.T) {
	errBoom := errors.New("boom")
	testLog := logging.NewLogrLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(io.Discard)).WithName("testlog"))
	cd := managed.ConnectionDetails{"a": []byte("b")}

	type args struct {
		mgr       manager.Manager
		of        resource.CompositeKind
		opts      []ReconcilerOption
		composite *composite.Unstructured
	}
	type want struct {
		r         reconcile.Result
		composite *composite.Unstructured
		err       error
	}

	now := metav1.Now()

	type compositeModifier func(o resource.Composite)
	withComposite := func(mods ...compositeModifier) *composite.Unstructured {
		co := composite.New(composite.WithGroupVersionKind(schema.FromAPIVersionAndKind("", "")))
		for _, m := range mods {
			m(co)
		}
		return co
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
				r: reconcile.Result{Requeue: false},
			},
		},
		"GetCompositeResourceError": {
			reason: "We should return error encountered while getting the composite resource.",
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
				err: errors.Wrap(errBoom, errGet),
			},
		},
		"UnpublishConnectionError": {
			reason: "We should return any error encountered while unpublishing connection details.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								if o, ok := obj.(*composite.Unstructured); ok {
									now := metav1.Now()
									o.SetDeletionTimestamp(&now)
								}
								return nil
							}),
						},
					}),
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithConnectionPublishers(managed.ConnectionPublisherFns{
						UnpublishConnectionFn: func(ctx context.Context, o resource.ConnectionSecretOwner, c managed.ConnectionDetails) error {
							return errBoom
						},
					}),
				},
			},
			want: want{
				composite: withComposite(func(o resource.Composite) {
					o.SetDeletionTimestamp(&now)
					o.SetConditions(xpv1.Deleting(), xpv1.ReconcileError(errors.Wrap(errBoom, errUnpublish)))
				}),
				r: reconcile.Result{Requeue: true},
			},
		},
		"RemoveFinalizerError": {
			reason: "We should return any error encountered while removing finalizer.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								if o, ok := obj.(*composite.Unstructured); ok {
									now := metav1.Now()
									o.SetDeletionTimestamp(&now)
								}
								return nil
							}),
						},
					}),
					WithCompositeFinalizer(resource.FinalizerFns{
						RemoveFinalizerFn: func(ctx context.Context, obj resource.Object) error {
							return errBoom
						},
					}),
					WithConnectionPublishers(managed.ConnectionPublisherFns{
						UnpublishConnectionFn: func(ctx context.Context, o resource.ConnectionSecretOwner, c managed.ConnectionDetails) error {
							return nil
						},
					}),
				},
			},
			want: want{
				composite: withComposite(func(o resource.Composite) {
					o.SetDeletionTimestamp(&now)
					o.SetConditions(xpv1.Deleting(), xpv1.ReconcileError(errors.Wrap(errBoom, errRemoveFinalizer)))
				}),
				r: reconcile.Result{Requeue: true},
			},
		},
		"SuccessfulDelete": {
			reason: "We should return no error when deleted successfully.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
								if o, ok := obj.(*composite.Unstructured); ok {
									now := metav1.Now()
									o.SetDeletionTimestamp(&now)
								}
								return nil
							}),
						},
					}),
					WithCompositeFinalizer(resource.FinalizerFns{
						RemoveFinalizerFn: func(ctx context.Context, obj resource.Object) error {
							return nil
						},
					}),
					WithConnectionPublishers(managed.ConnectionPublisherFns{
						UnpublishConnectionFn: func(ctx context.Context, o resource.ConnectionSecretOwner, c managed.ConnectionDetails) error {
							return nil
						},
					}),
				},
			},
			want: want{
				composite: withComposite(func(o resource.Composite) {
					o.SetDeletionTimestamp(&now)
					o.SetConditions(xpv1.Deleting(), xpv1.ReconcileSuccess())
				}),
			},
		},
		"AddFinalizerError": {
			reason: "We should return any error encountered while adding finalizer.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil),
						},
					}),
					WithCompositeFinalizer(resource.FinalizerFns{
						AddFinalizerFn: func(ctx context.Context, obj resource.Object) error {
							return errBoom
						},
					}),
				},
			},
			want: want{
				composite: withComposite(func(o resource.Composite) {
					o.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errAddFinalizer)))
				}),
				r: reconcile.Result{Requeue: true},
			},
		},
		"SelectCompositionError": {
			reason: "We should return any error encountered while selecting a composition.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil),
						},
					}),
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, _ resource.Composite) error {
						return errBoom
					})),
				},
			},
			want: want{
				composite: withComposite(func(o resource.Composite) {
					o.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errSelectComp)))
				}),
				r: reconcile.Result{Requeue: true},
			},
		},
		"FetchCompositionError": {
			reason: "We should return any error encountered while fetching a composition.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil),
						},
					}),
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionFetcher(CompositionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.Composition, error) {
						return nil, errBoom
					})),
				},
			},
			want: want{
				composite: withComposite(func(o resource.Composite) {
					o.SetCompositionReference(&corev1.ObjectReference{})
					o.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errFetchComp)))
				}),
				r: reconcile.Result{Requeue: true},
			},
		},
		"ValidateCompositionError": {
			reason: "We should return any error encountered while validating our Composition.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil),
						},
					}),
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionFetcher(CompositionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.Composition, error) {
						return &v1.Composition{}, nil
					})),
					WithCompositionValidator(CompositionValidatorFn(func(_ *v1.Composition) error { return errBoom })),
				},
			},
			want: want{
				composite: withComposite(func(o resource.Composite) {
					o.SetCompositionReference(&corev1.ObjectReference{})
					o.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errValidate)))
				}),
				r: reconcile.Result{Requeue: true},
			},
		},
		"ConfigureCompositeError": {
			reason: "We should return any error encountered while configuring the composite resource.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil),
						},
					}),
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionFetcher(CompositionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.Composition, error) {
						return &v1.Composition{}, nil
					})),
					WithCompositionValidator(CompositionValidatorFn(func(_ *v1.Composition) error { return nil })),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.Composition) error {
						return errBoom
					})),
				},
			},
			want: want{
				composite: withComposite(func(o resource.Composite) {
					o.SetCompositionReference(&corev1.ObjectReference{})
					o.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errConfigure)))
				}),
				r: reconcile.Result{Requeue: true},
			},
		},
		"ComposedTemplatesError": {
			reason: "We should return any error encountered while inlining a composition's patchsets.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil),
						},
						Applicator: resource.ApplyFn(func(c context.Context, r client.Object, ao ...resource.ApplyOption) error {
							return nil
						}),
					}),
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionFetcher(CompositionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.Composition, error) {
						c := &v1.Composition{Spec: v1.CompositionSpec{
							Resources: []v1.ComposedTemplate{{
								Patches: []v1.Patch{{
									Type:         v1.PatchTypePatchSet,
									PatchSetName: pointer.StringPtr("nonexistent-patchset"),
								}},
							}},
						}}
						return c, nil
					})),
					WithCompositionValidator(CompositionValidatorFn(func(_ *v1.Composition) error { return nil })),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.Composition) error {
						return nil
					})),
				},
			},
			want: want{
				composite: withComposite(func(o resource.Composite) {
					o.SetCompositionReference(&corev1.ObjectReference{})
					o.SetConditions(xpv1.ReconcileError(errors.Wrap(errors.New("cannot find PatchSet by name nonexistent-patchset"), errInline)))
				}),
				r: reconcile.Result{Requeue: true},
			},
		},
		"AssociateTemplatesError": {
			reason: "We should return any error encountered while associating Composition templates with composed resources.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil),
						},
						Applicator: resource.ApplyFn(func(c context.Context, r client.Object, ao ...resource.ApplyOption) error {
							return nil
						}),
					}),
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionFetcher(CompositionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.Composition, error) {
						return &v1.Composition{}, nil
					})),
					WithCompositionValidator(CompositionValidatorFn(func(_ *v1.Composition) error { return nil })),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.Composition) error {
						return nil
					})),
					WithCompositionTemplateAssociator(CompositionTemplateAssociatorFn(func(context.Context, resource.Composite, []v1.ComposedTemplate) ([]TemplateAssociation, error) {
						return nil, errBoom
					})),
				},
			},
			want: want{
				composite: withComposite(func(o resource.Composite) {
					o.SetCompositionReference(&corev1.ObjectReference{})
					o.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errAssociate)))
				}),
				r: reconcile.Result{Requeue: true},
			},
		},
		"UpdateCompositeError": {
			reason: "We should return any error encountered while updating our composite resource with references.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet:    test.NewMockGetFn(nil),
							MockUpdate: test.NewMockUpdateFn(errBoom),
						},
					}),
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionFetcher(CompositionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.Composition, error) {
						c := &v1.Composition{Spec: v1.CompositionSpec{
							Resources: []v1.ComposedTemplate{{}},
						}}
						return c, nil
					})),
					WithCompositionValidator(CompositionValidatorFn(func(_ *v1.Composition) error { return nil })),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.Composition) error {
						return nil
					})),
					WithRenderer(RendererFn(func(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1.ComposedTemplate) error {
						return nil
					})),
				},
			},
			want: want{
				composite: withComposite(func(o resource.Composite) {
					o.SetResourceReferences([]corev1.ObjectReference{})
					o.SetCompositionReference(&corev1.ObjectReference{})
					o.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errUpdateComposite)))
				}),
				r: reconcile.Result{Requeue: true},
			},
		},
		"ApplyComposedError": {
			reason: "We should return any error encountered while applying a composed resource.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet:    test.NewMockGetFn(nil),
							MockUpdate: test.NewMockUpdateFn(nil),
						},
						Applicator: resource.ApplyFn(func(c context.Context, r client.Object, ao ...resource.ApplyOption) error {
							return errBoom
						}),
					}),
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionFetcher(CompositionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.Composition, error) {
						c := &v1.Composition{Spec: v1.CompositionSpec{
							Resources: []v1.ComposedTemplate{{}},
						}}
						return c, nil
					})),
					WithCompositionValidator(CompositionValidatorFn(func(_ *v1.Composition) error { return nil })),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.Composition) error {
						return nil
					})),
					WithRenderer(RendererFn(func(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1.ComposedTemplate) error {
						return nil
					})),
				},
			},
			want: want{
				composite: withComposite(func(o resource.Composite) {
					o.SetResourceReferences([]corev1.ObjectReference{})
					o.SetCompositionReference(&corev1.ObjectReference{})
					o.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errApply)))
				}),
				r: reconcile.Result{Requeue: true},
			},
		},
		"FetchConnectionDetailsError": {
			reason: "We should return any error encountered while fetching a composed resource's connection details.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet:    test.NewMockGetFn(nil),
							MockUpdate: test.NewMockUpdateFn(nil),
						},
						Applicator: resource.ApplyFn(func(c context.Context, r client.Object, ao ...resource.ApplyOption) error {
							return nil
						}),
					}),
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionFetcher(CompositionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.Composition, error) {
						c := &v1.Composition{Spec: v1.CompositionSpec{
							Resources: []v1.ComposedTemplate{{}},
						}}
						return c, nil
					})),
					WithCompositionValidator(CompositionValidatorFn(func(_ *v1.Composition) error { return nil })),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.Composition) error {
						return nil
					})),
					WithRenderer(RendererFn(func(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1.ComposedTemplate) error {
						return nil
					})),
					WithConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, cd resource.Composed, t v1.ComposedTemplate) (managed.ConnectionDetails, error) {
						return nil, errBoom
					})),
				},
			},
			want: want{
				composite: withComposite(func(o resource.Composite) {
					o.SetResourceReferences([]corev1.ObjectReference{})
					o.SetCompositionReference(&corev1.ObjectReference{})
					o.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errFetchSecret)))
				}),
				r: reconcile.Result{Requeue: true},
			},
		},
		"CheckReadinessError": {
			reason: "We should return any error encountered while checking whether a composed resource is ready.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet:          test.NewMockGetFn(nil),
							MockUpdate:       test.NewMockUpdateFn(nil),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil),
						},
						Applicator: resource.ApplyFn(func(c context.Context, r client.Object, ao ...resource.ApplyOption) error {
							return nil
						}),
					}),
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionFetcher(CompositionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.Composition, error) {
						c := &v1.Composition{Spec: v1.CompositionSpec{
							Resources: []v1.ComposedTemplate{{}},
						}}
						return c, nil
					})),
					WithCompositionValidator(CompositionValidatorFn(func(_ *v1.Composition) error { return nil })),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.Composition) error {
						return nil
					})),
					WithRenderer(RendererFn(func(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1.ComposedTemplate) error {
						return nil
					})),
					WithConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, cd resource.Composed, t v1.ComposedTemplate) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithReadinessChecker(ReadinessCheckerFn(func(ctx context.Context, cd resource.Composed, t v1.ComposedTemplate) (ready bool, err error) {
						return false, errBoom
					})),
				},
			},
			want: want{
				composite: withComposite(func(o resource.Composite) {
					o.SetResourceReferences([]corev1.ObjectReference{})
					o.SetCompositionReference(&corev1.ObjectReference{})
					o.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errReadiness)))
				}),
				r: reconcile.Result{Requeue: true},
			},
		},
		"CompositeRenderError": {
			reason: "We should return any error encountered while rendering the Composite.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet:          test.NewMockGetFn(nil),
							MockUpdate:       test.NewMockUpdateFn(nil),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil),
						},
						Applicator: resource.ApplyFn(func(c context.Context, r client.Object, ao ...resource.ApplyOption) error {
							return nil
						}),
					}),
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionFetcher(CompositionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.Composition, error) {
						c := &v1.Composition{Spec: v1.CompositionSpec{
							Resources: []v1.ComposedTemplate{{}},
						}}
						return c, nil
					})),
					WithCompositionValidator(CompositionValidatorFn(func(_ *v1.Composition) error { return nil })),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.Composition) error {
						return nil
					})),
					WithRenderer(RendererFn(func(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1.ComposedTemplate) error {
						return nil
					})),
					WithConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, cd resource.Composed, t v1.ComposedTemplate) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithReadinessChecker(ReadinessCheckerFn(func(ctx context.Context, cd resource.Composed, t v1.ComposedTemplate) (ready bool, err error) {
						return false, nil
					})),
					WithCompositeRenderer(RendererFn(func(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1.ComposedTemplate) error {
						return errBoom
					})),
				},
			},
			want: want{
				composite: withComposite(func(o resource.Composite) {
					o.SetResourceReferences([]corev1.ObjectReference{})
					o.SetCompositionReference(&corev1.ObjectReference{})
					o.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errRenderCR)))
				}),
				r: reconcile.Result{Requeue: true},
			},
		},
		"CompositeUpdateError": {
			reason: "We should return any error encountered while updating the Composite.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet:          test.NewMockGetFn(nil),
							MockUpdate:       test.NewMockUpdateFn(errBoom),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil),
						},
						Applicator: resource.ApplyFn(func(c context.Context, obj client.Object, ao ...resource.ApplyOption) error {
							// annotation will be set by mock composite render
							if obj.GetAnnotations()["composite-rendered"] == "true" {
								return errBoom
							}
							return nil
						}),
					}),
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionFetcher(CompositionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.Composition, error) {
						c := &v1.Composition{Spec: v1.CompositionSpec{
							Resources: []v1.ComposedTemplate{{}},
						}}
						return c, nil
					})),
					WithCompositionValidator(CompositionValidatorFn(func(_ *v1.Composition) error { return nil })),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.Composition) error {
						return nil
					})),
					WithRenderer(RendererFn(func(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1.ComposedTemplate) error {
						return nil
					})),
					WithConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, cd resource.Composed, t v1.ComposedTemplate) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithReadinessChecker(ReadinessCheckerFn(func(ctx context.Context, cd resource.Composed, t v1.ComposedTemplate) (ready bool, err error) {
						return true, nil
					})),
					WithCompositeRenderer(RendererFn(func(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1.ComposedTemplate) error {
						// use arbitrary annotation to track api-server requests
						// made after composite render
						cp.SetAnnotations(map[string]string{"composite-rendered": "true"})
						return nil
					})),
				},
			},
			want: want{
				composite: withComposite(func(o resource.Composite) {
					o.SetResourceReferences([]corev1.ObjectReference{})
					o.SetCompositionReference(&corev1.ObjectReference{})
					o.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errUpdateComposite)))
				}),
				r: reconcile.Result{Requeue: true},
			},
		},
		"CompositeUpdateEarlyExit": {
			reason: "We should early exit to be immediately enqueued if the composite is updated by composed.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet:          test.NewMockGetFn(nil),
							MockUpdate:       test.NewMockUpdateFn(nil),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil),
						},
						Applicator: resource.ApplyFn(func(c context.Context, obj client.Object, ao ...resource.ApplyOption) error {
							// annotation will be set by mock composite render
							if obj.GetAnnotations()["composite-rendered"] == "true" {
								// Set composite resource version to indicate update was not a no-op.
								obj.SetResourceVersion("1")
							}
							return nil
						}),
					}),
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionFetcher(CompositionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.Composition, error) {
						c := &v1.Composition{Spec: v1.CompositionSpec{
							Resources: []v1.ComposedTemplate{{}},
						}}
						return c, nil
					})),
					WithCompositionValidator(CompositionValidatorFn(func(_ *v1.Composition) error { return nil })),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.Composition) error {
						return nil
					})),
					WithRenderer(RendererFn(func(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1.ComposedTemplate) error {
						return nil
					})),
					WithConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, cd resource.Composed, t v1.ComposedTemplate) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithReadinessChecker(ReadinessCheckerFn(func(ctx context.Context, cd resource.Composed, t v1.ComposedTemplate) (ready bool, err error) {
						return true, nil
					})),
					WithCompositeRenderer(RendererFn(func(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1.ComposedTemplate) error {
						// use arbitrary annotation to track api-server requests
						// made after composite render
						cp.SetAnnotations(map[string]string{"composite-rendered": "true"})
						return nil
					})),
				},
			},
			want: want{
				composite: withComposite(func(o resource.Composite) {
					o.SetResourceReferences([]corev1.ObjectReference{})
					o.SetCompositionReference(&corev1.ObjectReference{})
					o.SetAnnotations(map[string]string{"composite-rendered": "true"})
				}),
				r: reconcile.Result{Requeue: false},
			},
		},
		"PublishConnectionDetailsError": {
			reason: "We should return any error encountered while publishing connection details.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet:          test.NewMockGetFn(nil),
							MockUpdate:       test.NewMockUpdateFn(nil),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil),
						},
						Applicator: resource.ApplyFn(func(c context.Context, r client.Object, ao ...resource.ApplyOption) error {
							return nil
						}),
					}),
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionFetcher(CompositionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.Composition, error) {
						return &v1.Composition{}, nil
					})),
					WithCompositionValidator(CompositionValidatorFn(func(_ *v1.Composition) error { return nil })),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.Composition) error {
						return nil
					})),
					WithConnectionPublishers(managed.ConnectionPublisherFns{
						PublishConnectionFn: func(ctx context.Context, o resource.ConnectionSecretOwner, c managed.ConnectionDetails) (published bool, err error) {
							return false, errBoom
						},
					}),
				},
			},
			want: want{
				composite: withComposite(func(o resource.Composite) {
					o.SetResourceReferences([]corev1.ObjectReference{})
					o.SetCompositionReference(&corev1.ObjectReference{})
					o.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errPublish)))
				}),
				r: reconcile.Result{Requeue: true},
			},
		},
		"ComposedResourcesNotReady": {
			reason: "We should requeue if any of our composed resources are not yet ready.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet:          test.NewMockGetFn(nil),
							MockUpdate:       test.NewMockUpdateFn(nil),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil),
						},
						Applicator: resource.ApplyFn(func(c context.Context, r client.Object, ao ...resource.ApplyOption) error {
							return nil
						}),
					}),
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionFetcher(CompositionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.Composition, error) {
						c := &v1.Composition{Spec: v1.CompositionSpec{
							Resources: []v1.ComposedTemplate{{}},
						}}
						return c, nil
					})),
					WithCompositionValidator(CompositionValidatorFn(func(_ *v1.Composition) error { return nil })),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.Composition) error {
						return nil
					})),
					WithRenderer(RendererFn(func(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1.ComposedTemplate) error {
						return nil
					})),
					WithConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, cd resource.Composed, t v1.ComposedTemplate) (managed.ConnectionDetails, error) {
						return nil, nil
					})),
					WithReadinessChecker(ReadinessCheckerFn(func(ctx context.Context, cd resource.Composed, t v1.ComposedTemplate) (ready bool, err error) {
						// Our one resource is not ready.
						return false, nil
					})),
					WithConnectionPublishers(managed.ConnectionPublisherFns{
						PublishConnectionFn: func(ctx context.Context, o resource.ConnectionSecretOwner, c managed.ConnectionDetails) (published bool, err error) {
							return false, nil
						},
					}),
				},
			},
			want: want{
				composite: withComposite(func(o resource.Composite) {
					o.SetResourceReferences([]corev1.ObjectReference{})
					o.SetCompositionReference(&corev1.ObjectReference{})
					o.SetConditions(xpv1.ReconcileSuccess(), xpv1.Creating())
				}),
				r: reconcile.Result{Requeue: true},
			},
		},
		"ComposedResourcesReady": {
			reason: "We should requeue after our poll interval if all of our composed resources are ready.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet:          test.NewMockGetFn(nil),
							MockUpdate:       test.NewMockUpdateFn(nil),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil),
						},
						Applicator: resource.ApplyFn(func(c context.Context, r client.Object, ao ...resource.ApplyOption) error {
							return nil
						}),
					}),
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionFetcher(CompositionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.Composition, error) {
						c := &v1.Composition{Spec: v1.CompositionSpec{
							Resources: []v1.ComposedTemplate{{}},
						}}
						return c, nil
					})),
					WithCompositionValidator(CompositionValidatorFn(func(_ *v1.Composition) error { return nil })),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.Composition) error {
						return nil
					})),
					WithRenderer(RendererFn(func(ctx context.Context, cp resource.Composite, cd resource.Composed, t v1.ComposedTemplate) error {
						return nil
					})),
					WithConnectionDetailsFetcher(ConnectionDetailsFetcherFn(func(ctx context.Context, _ resource.Composed, t v1.ComposedTemplate) (managed.ConnectionDetails, error) {
						return cd, nil
					})),
					WithReadinessChecker(ReadinessCheckerFn(func(ctx context.Context, cd resource.Composed, t v1.ComposedTemplate) (ready bool, err error) {
						// Our one resource is ready.
						return true, nil
					})),
					WithConnectionPublishers(managed.ConnectionPublisherFns{
						PublishConnectionFn: func(ctx context.Context, o resource.ConnectionSecretOwner, got managed.ConnectionDetails) (published bool, err error) {
							want := cd
							if diff := cmp.Diff(want, got); diff != "" {
								t.Errorf("PublishConnection(...): -want, +got:\n%s", diff)
							}
							return true, nil
						},
					}),
				},
			},
			want: want{
				composite: withComposite(func(o resource.Composite) {
					o.SetResourceReferences([]corev1.ObjectReference{})
					o.SetCompositionReference(&corev1.ObjectReference{})
					o.SetConditions(xpv1.ReconcileSuccess(), xpv1.Available())
					o.SetConnectionDetailsLastPublishedTime(&now)
				}),
				r: reconcile.Result{RequeueAfter: defaultPollInterval},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// Create wrapper around the Get and Status().Update funcs of the
			// client mock to preserver the composite data.
			tc.args.opts = append(tc.args.opts, func(r *Reconciler) {
				var customGet test.MockGetFn
				var customStatusUpdate test.MockStatusUpdateFn
				mockGet := func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
					if o, ok := obj.(*composite.Unstructured); ok && tc.args.composite != nil {
						tc.args.composite.DeepCopyInto(&o.Unstructured)
						return nil
					}
					if customGet != nil {
						return customGet(ctx, key, obj)
					}
					return nil
				}

				mockStatusUpdate := func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
					if o, ok := obj.(*composite.Unstructured); ok {
						if tc.args.composite != nil {
							o.DeepCopyInto(&tc.args.composite.Unstructured)
						} else {
							tc.args.composite = o
						}
						return nil
					}
					if customStatusUpdate != nil {
						return customStatusUpdate(ctx, obj, opts...)
					}
					return nil
				}

				if mockClient, ok := r.client.Client.(*test.MockClient); ok {
					customGet = mockClient.MockGet
					customStatusUpdate = mockClient.MockStatusUpdate
					mockClient.MockGet = mockGet
					mockClient.MockStatusUpdate = mockStatusUpdate
				} else {
					r.client.Client = &test.MockClient{
						MockGet:          mockGet,
						MockStatusUpdate: mockStatusUpdate,
					}
				}
			})

			r := NewReconciler(tc.args.mgr, tc.args.of, append(tc.args.opts, WithLogger(testLog))...)
			got, err := r.Reconcile(context.Background(), reconcile.Request{})

			if diff := cmp.Diff(tc.want.composite, tc.args.composite, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want, +got:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.r, got, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestFilterToXRPatches(t *testing.T) {
	toXR1 := v1.Patch{
		Type: v1.PatchTypeToCompositeFieldPath,
	}
	toXR2 := v1.Patch{
		Type: v1.PatchTypeCombineToComposite,
	}
	fromXR1 := v1.Patch{
		Type: v1.PatchTypeFromCompositeFieldPath,
	}
	fromXR2 := v1.Patch{
		Type: v1.PatchTypeCombineFromComposite,
	}
	type args struct {
		tas []TemplateAssociation
	}
	tests := map[string]struct {
		args args
		want []v1.Patch
	}{
		"NonEmptyToXRPatches": {
			args: args{
				tas: []TemplateAssociation{
					{
						Template: v1.ComposedTemplate{
							Patches: []v1.Patch{toXR1, toXR2, fromXR1, fromXR2},
						},
					},
				},
			},
			want: []v1.Patch{toXR1, toXR2},
		},
		"NoToXRPatches": {
			args: args{
				tas: []TemplateAssociation{
					{
						Template: v1.ComposedTemplate{
							Patches: []v1.Patch{fromXR1, fromXR2},
						},
					},
				},
			},
			want: []v1.Patch{},
		},
		"EmptyToXRPatches": {
			args: args{},
			want: []v1.Patch{},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if diff := cmp.Diff(tc.want, filterToXRPatches(tc.args.tas)); diff != "" {
				t.Errorf("\nfilterToXRPatches(...): -want, +got:\n%s", diff)
			}
		})
	}
}
