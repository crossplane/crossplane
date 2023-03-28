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
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	env "github.com/crossplane/crossplane/internal/controller/apiextensions/composite/environment"
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

	now := metav1.Now()

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
					WithClient(&test.MockClient{
						MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
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
					WithClient(&test.MockClient{
						MockGet: test.NewMockGetFn(errBoom),
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
					WithClient(&test.MockClient{
						MockGet: WithComposite(t, NewComposite(func(cr resource.Composite) {
							cr.SetDeletionTimestamp(&now)
						})),
						MockStatusUpdate: WantComposite(t, NewComposite(func(want resource.Composite) {
							want.SetDeletionTimestamp(&now)
							want.SetConditions(xpv1.Deleting(), xpv1.ReconcileError(errors.Wrap(errBoom, errUnpublish)))
						})),
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
				r: reconcile.Result{Requeue: true},
			},
		},
		"RemoveFinalizerError": {
			reason: "We should return any error encountered while removing finalizer.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClient(&test.MockClient{
						MockGet: WithComposite(t, NewComposite(func(cr resource.Composite) {
							cr.SetDeletionTimestamp(&now)
						})),
						MockStatusUpdate: WantComposite(t, NewComposite(func(cr resource.Composite) {
							cr.SetDeletionTimestamp(&now)
							cr.SetConditions(xpv1.Deleting(), xpv1.ReconcileError(errors.Wrap(errBoom, errRemoveFinalizer)))
						})),
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
				r: reconcile.Result{Requeue: true},
			},
		},
		"SuccessfulDelete": {
			reason: "We should return no error when deleted successfully.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClient(&test.MockClient{
						MockGet: WithComposite(t, NewComposite(func(cr resource.Composite) {
							cr.SetDeletionTimestamp(&now)
						})),
						MockStatusUpdate: WantComposite(t, NewComposite(func(cr resource.Composite) {
							cr.SetDeletionTimestamp(&now)
							cr.SetConditions(xpv1.Deleting(), xpv1.ReconcileSuccess())
						})),
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
				err: nil,
			},
		},
		"AddFinalizerError": {
			reason: "We should return any error encountered while adding finalizer.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClient(&test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: WantComposite(t, NewComposite(func(cr resource.Composite) {
							cr.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errAddFinalizer)))
						})),
					}),
					WithCompositeFinalizer(resource.FinalizerFns{
						AddFinalizerFn: func(ctx context.Context, obj resource.Object) error {
							return errBoom
						},
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: true},
			},
		},
		"SelectCompositionError": {
			reason: "We should return any error encountered while selecting a composition.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClient(&test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: WantComposite(t, NewComposite(func(cr resource.Composite) {
							cr.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errSelectComp)))
						})),
					}),
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, _ resource.Composite) error {
						return errBoom
					})),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: true},
			},
		},
		"FetchCompositionError": {
			reason: "We should return any error encountered while fetching a composition.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClient(&test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: WantComposite(t, NewComposite(func(cr resource.Composite) {
							cr.SetCompositionReference(&corev1.ObjectReference{})
							cr.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errFetchComp)))
						})),
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
				r: reconcile.Result{Requeue: true},
			},
		},
		"ValidateCompositionError": {
			reason: "We should return any error encountered while validating our Composition.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClient(&test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: WantComposite(t, NewComposite(func(cr resource.Composite) {
							cr.SetCompositionReference(&corev1.ObjectReference{})
							cr.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errValidate)))
						})),
					}),
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionFetcher(CompositionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.Composition, error) {
						return &v1.Composition{}, nil
					})),
					WithCompositionValidator(func(_ *v1.Composition) error {
						return errBoom
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: true},
			},
		},
		"ConfigureCompositeError": {
			reason: "We should return any error encountered while configuring the composite resource.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClient(&test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: WantComposite(t, NewComposite(func(cr resource.Composite) {
							cr.SetCompositionReference(&corev1.ObjectReference{})
							cr.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errConfigure)))
						})),
					}),
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionFetcher(CompositionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.Composition, error) {
						return &v1.Composition{}, nil
					})),
					WithCompositionValidator(func(_ *v1.Composition) error { return nil }),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.Composition) error {
						return errBoom
					})),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: true},
			},
		},
		"SelectEnvironmentError": {
			reason: "We should return any error encountered while selecting the environment.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClient(&test.MockClient{
						MockGet:          test.NewMockGetFn(nil),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					}),
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						return nil
					})),
					WithCompositionFetcher(CompositionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.Composition, error) {
						c := &v1.Composition{Spec: v1.CompositionSpec{}}
						return c, nil
					})),
					WithCompositionValidator(func(_ *v1.Composition) error { return nil }),
					WithConfigurator(ConfiguratorFn(func(ctx context.Context, cr resource.Composite, cp *v1.Composition) error { return nil })),
					WithEnvironmentSelector(EnvironmentSelectorFn(func(ctx context.Context, cr resource.Composite, cp *v1.Composition) error {
						return errBoom
					})),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errSelectEnvironment),
			},
		},
		"FetchEnvironmentError": {
			reason: "We should return any error encountered while fetching the environment.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClient(&test.MockClient{
						MockGet:          test.NewMockGetFn(nil),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					}),
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						return nil
					})),
					WithCompositionFetcher(CompositionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.Composition, error) {
						c := &v1.Composition{Spec: v1.CompositionSpec{}}
						return c, nil
					})),
					WithCompositionValidator(func(_ *v1.Composition) error { return nil }),
					WithConfigurator(ConfiguratorFn(func(ctx context.Context, cr resource.Composite, cp *v1.Composition) error { return nil })),
					WithEnvironmentFetcher(EnvironmentFetcherFn(func(ctx context.Context, cr resource.Composite) (*env.Environment, error) {
						return nil, errBoom
					})),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errFetchEnvironment),
			},
		},
		"ComposeResourcesError": {
			reason: "We should return any error encountered while composing resources.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClient(&test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: WantComposite(t, NewComposite(func(cr resource.Composite) {
							cr.SetCompositionReference(&corev1.ObjectReference{})
							cr.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errCompose)))
						})),
					}),
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionFetcher(CompositionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.Composition, error) {
						return &v1.Composition{}, nil
					})),
					WithCompositionValidator(func(_ *v1.Composition) error { return nil }),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.Composition) error {
						return nil
					})),
					WithComposer(ComposerFn(func(ctx context.Context, xr resource.Composite, req CompositionRequest) (CompositionResult, error) {
						return CompositionResult{}, errBoom
					})),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: true},
			},
		},
		"PublishConnectionDetailsError": {
			reason: "We should return any error encountered while publishing connection details.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClient(&test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: WantComposite(t, NewComposite(func(cr resource.Composite) {
							cr.SetCompositionReference(&corev1.ObjectReference{})
							cr.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errPublish)))
						})),
					}),
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionFetcher(CompositionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.Composition, error) {
						return &v1.Composition{}, nil
					})),
					WithCompositionValidator(func(_ *v1.Composition) error { return nil }),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.Composition) error {
						return nil
					})),
					WithComposer(ComposerFn(func(ctx context.Context, xr resource.Composite, req CompositionRequest) (CompositionResult, error) {
						return CompositionResult{}, nil
					})),
					WithConnectionPublishers(managed.ConnectionPublisherFns{
						PublishConnectionFn: func(ctx context.Context, o resource.ConnectionSecretOwner, c managed.ConnectionDetails) (published bool, err error) {
							return false, errBoom
						},
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: true},
			},
		},
		"CompositionWarnings": {
			reason: "We should not requeue if our Composer returned warning events.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClient(&test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: WantComposite(t, NewComposite(func(cr resource.Composite) {
							cr.SetCompositionReference(&corev1.ObjectReference{})
							cr.SetConditions(xpv1.ReconcileSuccess(), xpv1.Available())
						})),
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
					WithCompositionValidator(func(_ *v1.Composition) error { return nil }),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.Composition) error {
						return nil
					})),
					WithComposer(ComposerFn(func(ctx context.Context, xr resource.Composite, req CompositionRequest) (CompositionResult, error) {
						return CompositionResult{
							Events: []event.Event{event.Warning("Warning", errBoom)},
						}, nil
					})),
					WithConnectionPublishers(managed.ConnectionPublisherFns{
						PublishConnectionFn: func(ctx context.Context, o resource.ConnectionSecretOwner, c managed.ConnectionDetails) (published bool, err error) {
							return false, nil
						},
					}),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: defaultPollInterval},
			},
		},
		"ComposedResourcesNotReady": {
			reason: "We should requeue if any of our composed resources are not yet ready.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClient(&test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: WantComposite(t, NewComposite(func(cr resource.Composite) {
							cr.SetCompositionReference(&corev1.ObjectReference{})
							cr.SetConditions(xpv1.ReconcileSuccess(), xpv1.Creating())
						})),
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
					WithCompositionValidator(func(_ *v1.Composition) error { return nil }),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.Composition) error {
						return nil
					})),
					WithComposer(ComposerFn(func(ctx context.Context, xr resource.Composite, req CompositionRequest) (CompositionResult, error) {
						return CompositionResult{
							Composed: []ComposedResource{{
								Ready: false,
							}},
						}, nil
					})),
					WithConnectionPublishers(managed.ConnectionPublisherFns{
						PublishConnectionFn: func(ctx context.Context, o resource.ConnectionSecretOwner, c managed.ConnectionDetails) (published bool, err error) {
							return false, nil
						},
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: true},
			},
		},
		"ComposedResourcesReady": {
			reason: "We should requeue after our poll interval if all of our composed resources are ready.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClient(&test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: WantComposite(t, NewComposite(func(cr resource.Composite) {
							cr.SetCompositionReference(&corev1.ObjectReference{})
							cr.SetConditions(xpv1.ReconcileSuccess(), xpv1.Available())
							cr.SetConnectionDetailsLastPublishedTime(&now)
						})),
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
					WithCompositionValidator(func(_ *v1.Composition) error { return nil }),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.Composition) error {
						return nil
					})),
					WithComposer(ComposerFn(func(ctx context.Context, xr resource.Composite, req CompositionRequest) (CompositionResult, error) {
						return CompositionResult{ConnectionDetails: cd}, nil
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
				r: reconcile.Result{RequeueAfter: defaultPollInterval},
			},
		},
		"ReconciliationPausedSuccessful": {
			reason: `If a composite resource has the pause annotation with value "true", there should be no further requeue requests.`,
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClient(&test.MockClient{
						MockGet: WithComposite(t, NewComposite(func(cr resource.Composite) {
							cr.SetAnnotations(map[string]string{meta.AnnotationKeyReconciliationPaused: "true"})
						})),
						MockStatusUpdate: WantComposite(t, NewComposite(func(cr resource.Composite) {
							cr.SetAnnotations(map[string]string{meta.AnnotationKeyReconciliationPaused: "true"})
							cr.SetConditions(xpv1.ReconcilePaused())
						})),
					}),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"ReconciliationPausedError": {
			reason: `If a composite resource has the pause annotation with value "true" and the status update due to reconciliation being paused fails, error should be reported causing an exponentially backed-off requeue.`,
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClient(&test.MockClient{
						MockGet: WithComposite(t, NewComposite(func(cr resource.Composite) {
							cr.SetAnnotations(map[string]string{meta.AnnotationKeyReconciliationPaused: "true"})
						})),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(errBoom),
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateStatus),
			},
		},
		"ReconciliationResumes": {
			reason: `If a composite resource has the pause annotation with some value other than "true" and the Synced=False/ReconcilePaused status condition, reconciliation should resume with requeueing.`,
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClient(&test.MockClient{
						MockGet: WithComposite(t, NewComposite(func(cr resource.Composite) {
							cr.SetAnnotations(map[string]string{meta.AnnotationKeyReconciliationPaused: ""})
							cr.SetConditions(xpv1.ReconcilePaused())
						})),
						MockStatusUpdate: WantComposite(t, NewComposite(func(cr resource.Composite) {
							cr.SetAnnotations(map[string]string{meta.AnnotationKeyReconciliationPaused: ""})
							cr.SetConditions(xpv1.ReconcileSuccess(), xpv1.Available())
							cr.SetConnectionDetailsLastPublishedTime(&now)
							cr.SetCompositionReference(&corev1.ObjectReference{})
						})),
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
					WithCompositionValidator(func(_ *v1.Composition) error { return nil }),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.Composition) error {
						return nil
					})),
					WithComposer(ComposerFn(func(ctx context.Context, xr resource.Composite, req CompositionRequest) (CompositionResult, error) {
						return CompositionResult{}, nil
					})),
					WithConnectionPublishers(managed.ConnectionPublisherFns{
						PublishConnectionFn: func(ctx context.Context, o resource.ConnectionSecretOwner, got managed.ConnectionDetails) (published bool, err error) {
							return true, nil
						},
					}),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: defaultPollInterval},
			},
		},
		"ReconciliationResumesAfterAnnotationRemoval": {
			reason: `If a composite resource has the pause annotation removed and the Synced=False/ReconcilePaused status condition, reconciliation should resume with requeueing.`,
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClient(&test.MockClient{
						MockGet: WithComposite(t, NewComposite(func(cr resource.Composite) {
							// no annotation atm
							// (but reconciliations were already paused)
							cr.SetConditions(xpv1.ReconcilePaused())
						})),
						MockStatusUpdate: WantComposite(t, NewComposite(func(cr resource.Composite) {
							cr.SetConditions(xpv1.ReconcileSuccess(), xpv1.Available())
							cr.SetConnectionDetailsLastPublishedTime(&now)
							cr.SetCompositionReference(&corev1.ObjectReference{})
						})),
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
					WithCompositionValidator(func(_ *v1.Composition) error { return nil }),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.Composition) error {
						return nil
					})),
					WithComposer(ComposerFn(func(ctx context.Context, xr resource.Composite, req CompositionRequest) (CompositionResult, error) {
						return CompositionResult{}, nil
					})),
					WithConnectionPublishers(managed.ConnectionPublisherFns{
						PublishConnectionFn: func(ctx context.Context, o resource.ConnectionSecretOwner, got managed.ConnectionDetails) (published bool, err error) {
							return true, nil
						},
					}),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: defaultPollInterval},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewReconciler(tc.args.mgr, tc.args.of, tc.args.opts...)
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

type CompositeModifier func(cr resource.Composite)

func NewComposite(m ...CompositeModifier) *composite.Unstructured {
	cr := composite.New(composite.WithGroupVersionKind(schema.GroupVersionKind{}))
	for _, fn := range m {
		fn(cr)
	}
	return cr
}

// A get function that supplies the input XR.
func WithComposite(_ *testing.T, cr *composite.Unstructured) func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
	return func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
		if o, ok := obj.(*composite.Unstructured); ok {
			*o = *cr
		}
		return nil
	}
}

// A status update function that ensures the supplied object is the XR we want.
func WantComposite(t *testing.T, want resource.Composite) func(ctx context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
	return func(ctx context.Context, got client.Object, _ ...client.SubResourceUpdateOption) error {
		// Normally we use a custom Equal method on conditions to ignore the
		// lastTransitionTime, but we may be using unstructured types here where
		// the conditions are just a map[string]any.
		diff := cmp.Diff(want, got, cmpopts.EquateApproxTime(3*time.Second))
		if diff != "" {
			t.Errorf("WantComposite(...): -want, +got: %s", diff)
		}
		return nil
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
			if diff := cmp.Diff(tc.want, toXRPatchesFromTAs(tc.args.tas)); diff != "" {
				t.Errorf("\nfilterToXRPatches(...): -want, +got:\n%s", diff)
			}
		})
	}
}
