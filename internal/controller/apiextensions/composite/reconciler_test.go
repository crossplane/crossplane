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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composed"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/composite"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/internal/engine"
)

var _ Composer = ComposerSelectorFn(func(_ *v1.CompositionMode) Composer { return nil })

func TestReconcile(t *testing.T) {
	errBoom := errors.New("boom")
	cd := managed.ConnectionDetails{"a": []byte("b")}

	type args struct {
		client client.Client
		of     resource.CompositeKind
		opts   []ReconcilerOption
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
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"GetCompositeResourceError": {
			reason: "We should return error encountered while getting the composite resource.",
			args: args{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGet),
			},
		},
		"UnpublishConnectionError": {
			reason: "We should return any error encountered while unpublishing connection details.",
			args: args{
				client: &test.MockClient{
					MockGet: WithComposite(t, NewComposite(func(cr resource.Composite) {
						cr.SetDeletionTimestamp(&now)
					})),
					MockStatusUpdate: WantComposite(t, NewComposite(func(want resource.Composite) {
						want.SetDeletionTimestamp(&now)
						want.SetConditions(xpv1.Deleting(), xpv1.ReconcileError(errors.Wrap(errBoom, errUnpublish)))
					})),
				},
				opts: []ReconcilerOption{
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithConnectionPublishers(managed.ConnectionPublisherFns{
						UnpublishConnectionFn: func(_ context.Context, _ resource.ConnectionSecretOwner, _ managed.ConnectionDetails) error {
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
				client: &test.MockClient{
					MockGet: WithComposite(t, NewComposite(func(cr resource.Composite) {
						cr.SetDeletionTimestamp(&now)
					})),
					MockStatusUpdate: WantComposite(t, NewComposite(func(cr resource.Composite) {
						cr.SetDeletionTimestamp(&now)
						cr.SetConditions(xpv1.Deleting(), xpv1.ReconcileError(errors.Wrap(errBoom, errRemoveFinalizer)))
					})),
				},
				opts: []ReconcilerOption{
					WithCompositeFinalizer(resource.FinalizerFns{
						RemoveFinalizerFn: func(_ context.Context, _ resource.Object) error {
							return errBoom
						},
					}),
					WithConnectionPublishers(managed.ConnectionPublisherFns{
						UnpublishConnectionFn: func(_ context.Context, _ resource.ConnectionSecretOwner, _ managed.ConnectionDetails) error {
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
				client: &test.MockClient{
					MockGet: WithComposite(t, NewComposite(func(cr resource.Composite) {
						cr.SetDeletionTimestamp(&now)
					})),
					MockStatusUpdate: WantComposite(t, NewComposite(func(cr resource.Composite) {
						cr.SetDeletionTimestamp(&now)
						cr.SetConditions(xpv1.Deleting(), xpv1.ReconcileSuccess())
					})),
				},
				opts: []ReconcilerOption{
					WithCompositeFinalizer(resource.FinalizerFns{
						RemoveFinalizerFn: func(_ context.Context, _ resource.Object) error {
							return nil
						},
					}),
					WithConnectionPublishers(managed.ConnectionPublisherFns{
						UnpublishConnectionFn: func(_ context.Context, _ resource.ConnectionSecretOwner, _ managed.ConnectionDetails) error {
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
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
					MockStatusUpdate: WantComposite(t, NewComposite(func(cr resource.Composite) {
						cr.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errAddFinalizer)))
					})),
				},
				opts: []ReconcilerOption{
					WithCompositeFinalizer(resource.FinalizerFns{
						AddFinalizerFn: func(_ context.Context, _ resource.Object) error {
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
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
					MockStatusUpdate: WantComposite(t, NewComposite(func(cr resource.Composite) {
						cr.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errSelectComp)))
					})),
				},
				opts: []ReconcilerOption{
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
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
					MockStatusUpdate: WantComposite(t, NewComposite(func(cr resource.Composite) {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						cr.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errFetchComp)))
					})),
				},
				opts: []ReconcilerOption{
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionRevisionFetcher(CompositionRevisionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.CompositionRevision, error) {
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
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
					MockStatusUpdate: WantComposite(t, NewComposite(func(cr resource.Composite) {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						cr.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errValidate)))
					})),
				},
				opts: []ReconcilerOption{
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionRevisionFetcher(CompositionRevisionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.CompositionRevision, error) {
						return &v1.CompositionRevision{}, nil
					})),
					WithCompositionRevisionValidator(CompositionRevisionValidatorFn(func(_ *v1.CompositionRevision) error {
						return errBoom
					})),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: true},
			},
		},
		"ConfigureCompositeError": {
			reason: "We should return any error encountered while configuring the composite resource.",
			args: args{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
					MockStatusUpdate: WantComposite(t, NewComposite(func(cr resource.Composite) {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						cr.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errConfigure)))
					})),
				},
				opts: []ReconcilerOption{
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionRevisionFetcher(CompositionRevisionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.CompositionRevision, error) {
						return &v1.CompositionRevision{}, nil
					})),
					WithCompositionRevisionValidator(CompositionRevisionValidatorFn(func(_ *v1.CompositionRevision) error { return nil })),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.CompositionRevision) error {
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
				client: &test.MockClient{
					MockGet:          test.NewMockGetFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				opts: []ReconcilerOption{
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, _ resource.Composite) error {
						return nil
					})),
					WithCompositionRevisionFetcher(CompositionRevisionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.CompositionRevision, error) {
						c := &v1.CompositionRevision{Spec: v1.CompositionRevisionSpec{}}
						return c, nil
					})),
					WithCompositionRevisionValidator(CompositionRevisionValidatorFn(func(_ *v1.CompositionRevision) error { return nil })),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.CompositionRevision) error { return nil })),
					WithEnvironmentSelector(EnvironmentSelectorFn(func(_ context.Context, _ resource.Composite, _ *v1.CompositionRevision) error {
						return errBoom
					})),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: true},
			},
		},
		"FetchEnvironmentError": {
			reason: "We should requeue on any error encountered while fetching the environment.",
			args: args{
				client: &test.MockClient{
					MockGet:          test.NewMockGetFn(nil),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
				},
				opts: []ReconcilerOption{
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, _ resource.Composite) error {
						return nil
					})),
					WithCompositionRevisionFetcher(CompositionRevisionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.CompositionRevision, error) {
						c := &v1.CompositionRevision{Spec: v1.CompositionRevisionSpec{}}
						return c, nil
					})),
					WithCompositionRevisionValidator(CompositionRevisionValidatorFn(func(_ *v1.CompositionRevision) error { return nil })),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.CompositionRevision) error { return nil })),
					WithEnvironmentFetcher(EnvironmentFetcherFn(func(_ context.Context, _ EnvironmentFetcherRequest) (*Environment, error) {
						return nil, errBoom
					})),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: true},
			},
		},
		"ComposeResourcesError": {
			reason: "We should return any error encountered while composing resources.",
			args: args{
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
					MockStatusUpdate: WantComposite(t, NewComposite(func(cr resource.Composite) {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						cr.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errCompose)))
					})),
				},
				opts: []ReconcilerOption{
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionRevisionFetcher(CompositionRevisionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.CompositionRevision, error) {
						return &v1.CompositionRevision{}, nil
					})),
					WithCompositionRevisionValidator(CompositionRevisionValidatorFn(func(_ *v1.CompositionRevision) error { return nil })),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.CompositionRevision) error {
						return nil
					})),
					WithComposer(ComposerFn(func(_ context.Context, _ *composite.Unstructured, _ CompositionRequest) (CompositionResult, error) {
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
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
					MockStatusUpdate: WantComposite(t, NewComposite(func(cr resource.Composite) {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						cr.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errPublish)))
					})),
				},
				opts: []ReconcilerOption{
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionRevisionFetcher(CompositionRevisionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.CompositionRevision, error) {
						return &v1.CompositionRevision{}, nil
					})),
					WithCompositionRevisionValidator(CompositionRevisionValidatorFn(func(_ *v1.CompositionRevision) error { return nil })),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.CompositionRevision) error {
						return nil
					})),
					WithComposer(ComposerFn(func(_ context.Context, _ *composite.Unstructured, _ CompositionRequest) (CompositionResult, error) {
						return CompositionResult{}, nil
					})),
					WithConnectionPublishers(managed.ConnectionPublisherFns{
						PublishConnectionFn: func(_ context.Context, _ resource.ConnectionSecretOwner, _ managed.ConnectionDetails) (published bool, err error) {
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
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
					MockStatusUpdate: WantComposite(t, NewComposite(func(xr resource.Composite) {
						xr.SetCompositionReference(&corev1.ObjectReference{})
						xr.SetConditions(xpv1.ReconcileSuccess(), xpv1.Available())
					})),
				},
				opts: []ReconcilerOption{
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionRevisionFetcher(CompositionRevisionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.CompositionRevision, error) {
						c := &v1.CompositionRevision{Spec: v1.CompositionRevisionSpec{
							Resources: []v1.ComposedTemplate{{}},
						}}
						return c, nil
					})),
					WithCompositionRevisionValidator(CompositionRevisionValidatorFn(func(_ *v1.CompositionRevision) error { return nil })),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.CompositionRevision) error {
						return nil
					})),
					WithComposer(ComposerFn(func(_ context.Context, _ *composite.Unstructured, _ CompositionRequest) (CompositionResult, error) {
						return CompositionResult{
							Events: []event.Event{event.Warning("Warning", errBoom)},
						}, nil
					})),
					WithConnectionPublishers(managed.ConnectionPublisherFns{
						PublishConnectionFn: func(_ context.Context, _ resource.ConnectionSecretOwner, _ managed.ConnectionDetails) (published bool, err error) {
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
				client: &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
					MockStatusUpdate: WantComposite(t, NewComposite(func(cr resource.Composite) {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						cr.SetConditions(xpv1.ReconcileSuccess(), xpv1.Creating().WithMessage("Unready resources: cat, cow, elephant, and 1 more"))
					})),
				},
				opts: []ReconcilerOption{
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionRevisionFetcher(CompositionRevisionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.CompositionRevision, error) {
						c := &v1.CompositionRevision{Spec: v1.CompositionRevisionSpec{
							Resources: []v1.ComposedTemplate{{}},
						}}
						return c, nil
					})),
					WithCompositionRevisionValidator(CompositionRevisionValidatorFn(func(_ *v1.CompositionRevision) error { return nil })),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.CompositionRevision) error {
						return nil
					})),
					WithComposer(ComposerFn(func(_ context.Context, _ *composite.Unstructured, _ CompositionRequest) (CompositionResult, error) {
						return CompositionResult{
							Composed: []ComposedResource{{
								ResourceName: "elephant",
								Ready:        false,
								Synced:       true,
							}, {
								ResourceName: "cow",
								Ready:        false,
								Synced:       true,
							}, {
								ResourceName: "pig",
								Ready:        true,
								Synced:       true,
							}, {
								ResourceName: "cat",
								Ready:        false,
								Synced:       true,
							}, {
								ResourceName: "dog",
								Ready:        true,
								Synced:       true,
							}, {
								ResourceName: "snake",
								Ready:        false,
								Synced:       true,
							}},
						}, nil
					})),
					WithConnectionPublishers(managed.ConnectionPublisherFns{
						PublishConnectionFn: func(_ context.Context, _ resource.ConnectionSecretOwner, _ managed.ConnectionDetails) (published bool, err error) {
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
				client: &test.MockClient{
					MockGet: WithComposite(t, NewComposite(func(cr resource.Composite) {
						cr.SetResourceReferences([]corev1.ObjectReference{{
							APIVersion: "example.org/v1",
							Kind:       "ComposedResource",
						}})
					})),
					MockStatusUpdate: WantComposite(t, NewComposite(func(cr resource.Composite) {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						cr.SetResourceReferences([]corev1.ObjectReference{{
							APIVersion: "example.org/v1",
							Kind:       "ComposedResource",
						}})
						cr.SetConditions(xpv1.ReconcileSuccess(), xpv1.Available())
						cr.SetConnectionDetailsLastPublishedTime(&now)
					})),
				},
				opts: []ReconcilerOption{
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionRevisionFetcher(CompositionRevisionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.CompositionRevision, error) {
						c := &v1.CompositionRevision{Spec: v1.CompositionRevisionSpec{
							Resources: []v1.ComposedTemplate{{}},
						}}
						return c, nil
					})),
					WithCompositionRevisionValidator(CompositionRevisionValidatorFn(func(_ *v1.CompositionRevision) error { return nil })),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.CompositionRevision) error {
						return nil
					})),
					WithComposer(ComposerFn(func(_ context.Context, _ *composite.Unstructured, _ CompositionRequest) (CompositionResult, error) {
						return CompositionResult{ConnectionDetails: cd}, nil
					})),
					WithConnectionPublishers(managed.ConnectionPublisherFns{
						PublishConnectionFn: func(_ context.Context, _ resource.ConnectionSecretOwner, got managed.ConnectionDetails) (published bool, err error) {
							want := cd
							if diff := cmp.Diff(want, got); diff != "" {
								t.Errorf("PublishConnection(...): -want, +got:\n%s", diff)
							}
							return true, nil
						},
					}),
					WithWatchStarter("cool-controller", nil, WatchStarterFn(func(_ string, ws ...engine.Watch) error {
						cd := composed.New(composed.FromReference(corev1.ObjectReference{
							APIVersion: "example.org/v1",
							Kind:       "ComposedResource",
						}))
						want := []engine.Watch{engine.WatchFor(cd, engine.WatchTypeComposedResource, nil)}

						if diff := cmp.Diff(want, ws, cmp.AllowUnexported(engine.Watch{})); diff != "" {
							t.Errorf("StartWatches(...): -want, +got:\n%s", diff)
						}

						return nil
					})),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: defaultPollInterval},
			},
		},
		"ReconciliationPausedSuccessful": {
			reason: `If a composite resource has the pause annotation with value "true", there should be no further requeue requests.`,
			args: args{
				client: &test.MockClient{
					MockGet: WithComposite(t, NewComposite(func(cr resource.Composite) {
						cr.SetAnnotations(map[string]string{meta.AnnotationKeyReconciliationPaused: "true"})
					})),
					MockStatusUpdate: WantComposite(t, NewComposite(func(cr resource.Composite) {
						cr.SetAnnotations(map[string]string{meta.AnnotationKeyReconciliationPaused: "true"})
						cr.SetConditions(xpv1.ReconcilePaused().WithMessage(reconcilePausedMsg))
					})),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"ReconciliationPausedError": {
			reason: `If a composite resource has the pause annotation with value "true" and the status update due to reconciliation being paused fails, error should be reported causing an exponentially backed-off requeue.`,
			args: args{
				client: &test.MockClient{
					MockGet: WithComposite(t, NewComposite(func(cr resource.Composite) {
						cr.SetAnnotations(map[string]string{meta.AnnotationKeyReconciliationPaused: "true"})
					})),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(errBoom),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateStatus),
			},
		},
		"ReconciliationResumes": {
			reason: `If a composite resource has the pause annotation with some value other than "true" and the Synced=False/ReconcilePaused status condition, reconciliation should resume with requeueing.`,
			args: args{
				client: &test.MockClient{
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
				},
				opts: []ReconcilerOption{
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionRevisionFetcher(CompositionRevisionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.CompositionRevision, error) {
						c := &v1.CompositionRevision{Spec: v1.CompositionRevisionSpec{
							Resources: []v1.ComposedTemplate{{}},
						}}
						return c, nil
					})),
					WithCompositionRevisionValidator(CompositionRevisionValidatorFn(func(_ *v1.CompositionRevision) error { return nil })),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.CompositionRevision) error {
						return nil
					})),
					WithComposer(ComposerFn(func(_ context.Context, _ *composite.Unstructured, _ CompositionRequest) (CompositionResult, error) {
						return CompositionResult{}, nil
					})),
					WithConnectionPublishers(managed.ConnectionPublisherFns{
						PublishConnectionFn: func(_ context.Context, _ resource.ConnectionSecretOwner, _ managed.ConnectionDetails) (published bool, err error) {
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
				client: &test.MockClient{
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
				},
				opts: []ReconcilerOption{
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionRevisionFetcher(CompositionRevisionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.CompositionRevision, error) {
						c := &v1.CompositionRevision{Spec: v1.CompositionRevisionSpec{
							Resources: []v1.ComposedTemplate{{}},
						}}
						return c, nil
					})),
					WithCompositionRevisionValidator(CompositionRevisionValidatorFn(func(_ *v1.CompositionRevision) error { return nil })),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.CompositionRevision) error {
						return nil
					})),
					WithComposer(ComposerFn(func(_ context.Context, _ *composite.Unstructured, _ CompositionRequest) (CompositionResult, error) {
						return CompositionResult{}, nil
					})),
					WithConnectionPublishers(managed.ConnectionPublisherFns{
						PublishConnectionFn: func(_ context.Context, _ resource.ConnectionSecretOwner, _ managed.ConnectionDetails) (published bool, err error) {
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
			r := NewReconciler(tc.args.client, tc.args.of, tc.args.opts...)
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
func WantComposite(t *testing.T, want resource.Composite) func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
	t.Helper()
	return func(_ context.Context, got client.Object, _ ...client.SubResourceUpdateOption) error {
		t.Helper()
		// Normally we use a custom Equal method on conditions to ignore the
		// lastTransitionTime, but we may be using unstructured types here where
		// the conditions are just a map[string]any.
		diff := cmp.Diff(want, got, cmpopts.AcyclicTransformer("StringToTime", func(s string) any {
			ts, err := time.Parse(time.RFC3339, s)
			if err != nil {
				return s
			}
			return ts
		}), cmpopts.EquateApproxTime(3*time.Second))
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
