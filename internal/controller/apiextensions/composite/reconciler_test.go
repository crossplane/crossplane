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
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kunstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/claim"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/composed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/composite"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/unstructured/reference"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/crossplane/crossplane/v2/internal/circuit"
	"github.com/crossplane/crossplane/v2/internal/controller/apiextensions/composite/dependency"
	"github.com/crossplane/crossplane/v2/internal/engine"
	"github.com/crossplane/crossplane/v2/internal/features"
)

func TestReconcile(t *testing.T) {
	errBoom := errors.New("boom")
	cd := managed.ConnectionDetails{"a": []byte("b")}

	type args struct {
		c    client.Client
		of   schema.GroupVersionKind
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
				c: &test.MockClient{
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
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(errBoom),
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: cmpopts.AnyError,
			},
		},
		"RemoveFinalizerError": {
			reason: "We should return any error encountered while removing finalizer.",
			args: args{
				c: &test.MockClient{
					MockGet: WithComposite(t, NewComposite(func(cr *composite.Unstructured) {
						cr.SetDeletionTimestamp(&now)
					})),
					MockStatusUpdate: WantComposite(t, NewComposite(func(cr *composite.Unstructured) {
						cr.SetDeletionTimestamp(&now)
						cr.SetConditions(v1.WatchCircuitClosed(), xpv2.Deleting(), xpv2.ReconcileError(errors.Wrap(errBoom, errRemoveFinalizer)))
					})),
				},
				opts: []ReconcilerOption{
					WithCompositeFinalizer(resource.FinalizerFns{
						RemoveFinalizerFn: func(_ context.Context, _ resource.Object) error {
							return errBoom
						},
					}),
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: cmpopts.AnyError,
			},
		},
		"SuccessfulDelete": {
			reason: "We should return no error when deleted successfully.",
			args: args{
				c: &test.MockClient{
					MockGet: WithComposite(t, NewComposite(func(cr *composite.Unstructured) {
						cr.SetDeletionTimestamp(&now)
					})),
					MockStatusUpdate: WantComposite(t, NewComposite(func(cr *composite.Unstructured) {
						cr.SetDeletionTimestamp(&now)
						cr.SetConditions(v1.WatchCircuitClosed(), xpv2.Deleting(), xpv2.ReconcileSuccess())
					})),
				},
				opts: []ReconcilerOption{
					WithCompositeFinalizer(resource.FinalizerFns{
						RemoveFinalizerFn: func(_ context.Context, _ resource.Object) error {
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
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
					MockStatusUpdate: WantComposite(t, NewComposite(func(cr *composite.Unstructured) {
						cr.SetConditions(v1.WatchCircuitClosed(), xpv2.ReconcileError(errors.Wrap(errBoom, errAddFinalizer)))
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
				r:   reconcile.Result{},
				err: cmpopts.AnyError,
			},
		},
		"SelectCompositionError": {
			reason: "We should return any error encountered while selecting a composition.",
			args: args{
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
					MockStatusUpdate: WantComposite(t, NewComposite(func(cr *composite.Unstructured) {
						cr.SetConditions(v1.WatchCircuitClosed(), xpv2.ReconcileError(errors.Wrap(errBoom, errSelectComp)))
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
				r:   reconcile.Result{},
				err: cmpopts.AnyError,
			},
		},
		"FetchCompositionError": {
			reason: "We should return any error encountered while fetching a composition.",
			args: args{
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
					MockStatusUpdate: WantComposite(t, NewComposite(func(cr *composite.Unstructured) {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						cr.SetConditions(v1.WatchCircuitClosed(), xpv2.ReconcileError(errors.Wrap(errBoom, errFetchComp)))
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
				r:   reconcile.Result{},
				err: cmpopts.AnyError,
			},
		},
		"InvalidPipelineError": {
			reason: "We should return an error if the CompositionRevision does not have a valid pipeline.",
			args: args{
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
					MockStatusUpdate: WantComposite(t, NewComposite(func(cr *composite.Unstructured) {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						cr.SetConditions(v1.WatchCircuitClosed(), xpv2.ReconcileError(errors.New("selected CompositionRevision test-revision does not have a valid function pipeline: pipeline status unknown")))
					})),
				},
				opts: []ReconcilerOption{
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionRevisionFetcher(CompositionRevisionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.CompositionRevision, error) {
						rev := &v1.CompositionRevision{}
						rev.SetName("test-revision")
						// No ValidPipeline condition set, so it will be Unknown by default
						return rev, nil
					})),
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: cmpopts.AnyError,
			},
		},
		"InvalidPipelineWithMessageError": {
			reason: "We should return an error with the condition message if the CompositionRevision has an invalid pipeline with a detailed message.",
			args: args{
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
					MockStatusUpdate: WantComposite(t, NewComposite(func(cr *composite.Unstructured) {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						cr.SetConditions(v1.WatchCircuitClosed(), xpv2.ReconcileError(errors.New("selected CompositionRevision test-revision does not have a valid function pipeline: function foo-function does not have the required composition capability")))
					})),
				},
				opts: []ReconcilerOption{
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionRevisionFetcher(CompositionRevisionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.CompositionRevision, error) {
						rev := &v1.CompositionRevision{}
						rev.SetName("test-revision")
						rev.SetConditions(v1.MissingCapabilities("function foo-function does not have the required composition capability"))
						return rev, nil
					})),
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: cmpopts.AnyError,
			},
		},
		"ConfigureCompositeError": {
			reason: "We should return any error encountered while configuring the composite resource.",
			args: args{
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
					MockStatusUpdate: WantComposite(t, NewComposite(func(cr *composite.Unstructured) {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						cr.SetConditions(v1.WatchCircuitClosed(), xpv2.ReconcileError(errors.Wrap(errBoom, errConfigure)))
					})),
				},
				opts: []ReconcilerOption{
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionRevisionFetcher(CompositionRevisionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.CompositionRevision, error) {
						return NewCompositionRevision(), nil
					})),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.CompositionRevision) error {
						return errBoom
					})),
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: cmpopts.AnyError,
			},
		},
		"ComposeResourcesError": {
			reason: "We should return any error encountered while composing resources.",
			args: args{
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
					MockStatusUpdate: WantComposite(t, NewComposite(func(cr *composite.Unstructured) {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						cr.SetConditions(v1.WatchCircuitClosed(), xpv2.ReconcileError(errors.Wrap(errBoom, errCompose)))
					})),
				},
				opts: []ReconcilerOption{
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionRevisionFetcher(CompositionRevisionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.CompositionRevision, error) {
						return NewCompositionRevision(), nil
					})),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.CompositionRevision) error {
						return nil
					})),
					WithComposer(ComposerFn(func(_ context.Context, _ *composite.Unstructured, _ CompositionRequest) (CompositionResult, error) {
						return CompositionResult{}, errBoom
					})),
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: cmpopts.AnyError,
			},
		},
		"PublishConnectionDetailsError": {
			reason: "We should return any error encountered while publishing connection details.",
			args: args{
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
					MockStatusUpdate: WantComposite(t, NewComposite(func(cr *composite.Unstructured) {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						cr.SetConditions(v1.WatchCircuitClosed(), xpv2.ReconcileError(errors.Wrap(errBoom, errPublish)))
					})),
				},
				opts: []ReconcilerOption{
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionRevisionFetcher(CompositionRevisionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.CompositionRevision, error) {
						return NewCompositionRevision(), nil
					})),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.CompositionRevision) error {
						return nil
					})),
					WithComposer(ComposerFn(func(_ context.Context, _ *composite.Unstructured, _ CompositionRequest) (CompositionResult, error) {
						return CompositionResult{}, nil
					})),
					WithConnectionPublishers(ConnectionPublisherFn(func(_ context.Context, _ ConnectionSecretOwner, _ managed.ConnectionDetails) (published bool, err error) {
						return false, errBoom
					})),
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: cmpopts.AnyError,
			},
		},
		"CompositionWarnings": {
			reason: "We should not requeue if our Composer returned warning events.",
			args: args{
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
					MockStatusUpdate: WantComposite(t, NewComposite(func(xr *composite.Unstructured) {
						xr.SetCompositionReference(&corev1.ObjectReference{})
						xr.SetConditions(v1.WatchCircuitClosed(), xpv2.ReconcileSuccess(), xpv2.Available())
					})),
				},
				opts: []ReconcilerOption{
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionRevisionFetcher(CompositionRevisionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.CompositionRevision, error) {
						return NewCompositionRevision(), nil
					})),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.CompositionRevision) error {
						return nil
					})),
					WithComposer(ComposerFn(func(_ context.Context, _ *composite.Unstructured, _ CompositionRequest) (CompositionResult, error) {
						return CompositionResult{
							Events: []TargetedEvent{
								{
									Event:  event.Warning("Warning", errBoom),
									Target: CompositionTargetComposite,
								},
							},
						}, nil
					})),
					WithConnectionPublishers(ConnectionPublisherFn(func(_ context.Context, _ ConnectionSecretOwner, _ managed.ConnectionDetails) (published bool, err error) {
						return false, nil
					})),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"ComposedResourcesNotReady": {
			reason: "We should requeue if any of our composed resources are not yet ready.",
			args: args{
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
					MockStatusUpdate: WantComposite(t, NewComposite(func(cr *composite.Unstructured) {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						cr.SetConditions(v1.WatchCircuitClosed(), xpv2.ReconcileSuccess(), xpv2.Creating().WithMessage("Unready resources: cat, cow, elephant, and 1 more"))
					})),
				},
				opts: []ReconcilerOption{
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionRevisionFetcher(CompositionRevisionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.CompositionRevision, error) {
						return NewCompositionRevision(), nil
					})),
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
					WithConnectionPublishers(ConnectionPublisherFn(func(_ context.Context, _ ConnectionSecretOwner, _ managed.ConnectionDetails) (published bool, err error) {
						return false, nil
					})),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: true},
			},
		},
		"ComposedResourcesReady": {
			reason: "We should requeue after our poll interval if all of our composed resources are ready.",
			args: args{
				c: &test.MockClient{
					MockGet: WithComposite(t, NewComposite(func(cr *composite.Unstructured) {
						cr.SetResourceReferences([]corev1.ObjectReference{{
							APIVersion: "example.org/v1",
							Kind:       "ComposedResource",
						}})
					})),
					MockStatusUpdate: WantComposite(t, NewComposite(func(cr *composite.Unstructured) {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						cr.SetResourceReferences([]corev1.ObjectReference{{
							APIVersion: "example.org/v1",
							Kind:       "ComposedResource",
						}})
						cr.SetConditions(v1.WatchCircuitClosed(), xpv2.ReconcileSuccess(), xpv2.Available())
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
						return NewCompositionRevision(), nil
					})),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.CompositionRevision) error {
						return nil
					})),
					WithComposer(ComposerFn(func(_ context.Context, _ *composite.Unstructured, _ CompositionRequest) (CompositionResult, error) {
						return CompositionResult{ConnectionDetails: cd}, nil
					})),
					WithConnectionPublishers(ConnectionPublisherFn(func(_ context.Context, _ ConnectionSecretOwner, got managed.ConnectionDetails) (published bool, err error) {
						want := cd
						if diff := cmp.Diff(want, got); diff != "" {
							t.Errorf("PublishConnection(...): -want, +got:\n%s", diff)
						}
						return true, nil
					})),
					WithWatchStarter("cool-controller", nil, WatchStarterFn(func(_ context.Context, _ string, ws ...engine.Watch) error {
						cd := &kunstructured.Unstructured{}
						cd.SetAPIVersion("example.org/v1")
						cd.SetKind("ComposedResource")
						want := []engine.Watch{engine.WatchFor(cd, engine.WatchTypeDependency, nil)}

						if diff := cmp.Diff(want, ws, cmp.AllowUnexported(engine.Watch{})); diff != "" {
							t.Errorf("StartWatches(...): -want, +got:\n%s", diff)
						}

						return nil
					}), func() dependency.Tracker {
						// The XR depends on the composed resource kind, so the
						// reconciler should start a watch for it.
						tr := dependency.NewInMemory()
						tr.Track(client.ObjectKey{Name: "cool"}, []dependency.Reference{{
							GVK: schema.GroupVersionKind{Group: "example.org", Version: "v1", Kind: "ComposedResource"},
						}}, nil)
						return tr
					}()),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"ReconciliationPausedSuccessful": {
			reason: `If a composite resource has the pause annotation with value "true", there should be no further requeue requests.`,
			args: args{
				c: &test.MockClient{
					MockGet: WithComposite(t, NewComposite(func(cr *composite.Unstructured) {
						cr.SetAnnotations(map[string]string{meta.AnnotationKeyReconciliationPaused: "true"})
					})),
					MockStatusUpdate: WantComposite(t, NewComposite(func(cr *composite.Unstructured) {
						cr.SetAnnotations(map[string]string{meta.AnnotationKeyReconciliationPaused: "true"})
						cr.SetConditions(v1.WatchCircuitClosed(), xpv2.ReconcilePaused().WithMessage(reconcilePausedMsg))
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
				c: &test.MockClient{
					MockGet: WithComposite(t, NewComposite(func(cr *composite.Unstructured) {
						cr.SetAnnotations(map[string]string{meta.AnnotationKeyReconciliationPaused: "true"})
					})),
					MockStatusUpdate: test.NewMockSubResourceUpdateFn(errBoom),
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: cmpopts.AnyError,
			},
		},
		"ReconciliationResumes": {
			reason: `If a composite resource has the pause annotation with some value other than "true" and the Synced=False/ReconcilePaused status condition, reconciliation should resume with requeuing.`,
			args: args{
				c: &test.MockClient{
					MockGet: WithComposite(t, NewComposite(func(cr *composite.Unstructured) {
						cr.SetAnnotations(map[string]string{meta.AnnotationKeyReconciliationPaused: ""})
						cr.SetConditions(v1.WatchCircuitClosed(), xpv2.ReconcilePaused())
					})),
					MockStatusUpdate: WantComposite(t, NewComposite(func(cr *composite.Unstructured) {
						cr.SetAnnotations(map[string]string{meta.AnnotationKeyReconciliationPaused: ""})
						cr.SetConditions(v1.WatchCircuitClosed(), xpv2.ReconcileSuccess(), xpv2.Available())
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
						return NewCompositionRevision(), nil
					})),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.CompositionRevision) error {
						return nil
					})),
					WithComposer(ComposerFn(func(_ context.Context, _ *composite.Unstructured, _ CompositionRequest) (CompositionResult, error) {
						return CompositionResult{}, nil
					})),
					WithConnectionPublishers(ConnectionPublisherFn(func(_ context.Context, _ ConnectionSecretOwner, _ managed.ConnectionDetails) (published bool, err error) {
						return true, nil
					})),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"ReconciliationResumesAfterAnnotationRemoval": {
			reason: `If a composite resource has the pause annotation removed and the Synced=False/ReconcilePaused status condition, reconciliation should resume with requeuing.`,
			args: args{
				c: &test.MockClient{
					MockGet: WithComposite(t, NewComposite(func(cr *composite.Unstructured) {
						// no annotation atm
						// (but reconciliations were already paused)
						cr.SetConditions(v1.WatchCircuitClosed(), xpv2.ReconcilePaused())
					})),
					MockStatusUpdate: WantComposite(t, NewComposite(func(cr *composite.Unstructured) {
						cr.SetConditions(v1.WatchCircuitClosed(), xpv2.ReconcileSuccess(), xpv2.Available())
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
						return NewCompositionRevision(), nil
					})),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.CompositionRevision) error {
						return nil
					})),
					WithComposer(ComposerFn(func(_ context.Context, _ *composite.Unstructured, _ CompositionRequest) (CompositionResult, error) {
						return CompositionResult{}, nil
					})),
					WithConnectionPublishers(ConnectionPublisherFn(func(_ context.Context, _ ConnectionSecretOwner, _ managed.ConnectionDetails) (published bool, err error) {
						return true, nil
					})),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"CustomEventsAndConditions": {
			reason: "We should emit custom events and set custom conditions that were returned by the composer on both the composite resource and the claim.",
			args: args{
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						if xr, ok := obj.(*composite.Unstructured); ok {
							// non-nil claim ref to trigger claim Get()
							xr.SetClaimReference(&reference.Claim{})
							return nil
						}
						if cm, ok := obj.(*claim.Unstructured); ok {
							claim.New(claim.WithGroupVersionKind(schema.GroupVersionKind{})).DeepCopyInto(cm)
							return nil
						}
						return nil
					}),
					MockStatusUpdate: WantComposite(t, NewComposite(func(cr *composite.Unstructured) {
						cr.Schema = composite.SchemaLegacy
						cr.SetCompositionReference(&corev1.ObjectReference{})
						cr.SetConditions(
							v1.WatchCircuitClosed(),
							xpv2.Condition{
								Type:    "DatabaseReady",
								Status:  corev1.ConditionTrue,
								Reason:  "Available",
								Message: "This is a condition for database availability.",
							},
							xpv2.Condition{
								Type:               "InternalSync",
								Status:             corev1.ConditionTrue,
								LastTransitionTime: metav1.Time{},
								Reason:             "SyncSuccess",
								Message:            "This is a condition representing an internal sync process.",
								ObservedGeneration: 0,
							},
							xpv2.ReconcileSuccess(),
							xpv2.Available(),
						)
						cr.SetClaimConditionTypes("DatabaseReady")
						cr.SetClaimReference(&reference.Claim{})
					})),
				},
				opts: []ReconcilerOption{
					// Only legacy XRs support claims.
					WithCompositeSchema(composite.SchemaLegacy),
					WithRecorder(newTestRecorder(
						eventArgs{
							Kind: compositeKind,
							Event: event.Event{
								Type:        event.Type(corev1.EventTypeNormal),
								Reason:      "SelectComposition",
								Message:     "Successfully selected composition: ",
								Annotations: map[string]string{},
							},
						},
						eventArgs{
							Kind: compositeKind,
							Event: event.Event{
								Type:   event.TypeNormal,
								Reason: "DatabaseAvailable",
								// The composite should have the "Pipeline step" prefix.
								Message:     "Pipeline step \"some-function\": This is an event for database availability.",
								Annotations: map[string]string{},
							},
						},
						eventArgs{
							Kind: claimKind,
							Event: event.Event{
								Type:   event.TypeNormal,
								Reason: "DatabaseAvailable",
								// The claim should not have the "Pipeline step" prefix.
								Message:     "This is an event for database availability.",
								Annotations: map[string]string{},
							},
						},
						eventArgs{
							Kind: compositeKind,
							Event: event.Event{
								Type:        event.TypeNormal,
								Reason:      "SyncSuccess",
								Message:     "Pipeline step \"some-function\": Internal sync was successful.",
								Annotations: map[string]string{},
							},
						},
					)),
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionRevisionFetcher(CompositionRevisionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.CompositionRevision, error) {
						return NewCompositionRevision(), nil
					})),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.CompositionRevision) error {
						return nil
					})),
					WithComposer(ComposerFn(func(_ context.Context, _ *composite.Unstructured, _ CompositionRequest) (CompositionResult, error) {
						return CompositionResult{
							Composed:          []ComposedResource{},
							ConnectionDetails: cd,
							Events: []TargetedEvent{
								{
									Event: event.Event{
										Type:        event.TypeNormal,
										Reason:      "DatabaseAvailable",
										Message:     "This is an event for database availability.",
										Annotations: map[string]string{},
									},
									Detail: "Pipeline step \"some-function\"",
									Target: CompositionTargetCompositeAndClaim,
								},
								{
									Event: event.Event{
										Type:        event.TypeNormal,
										Reason:      "SyncSuccess",
										Message:     "Internal sync was successful.",
										Annotations: map[string]string{},
									},
									Detail: "Pipeline step \"some-function\"",
									Target: CompositionTargetComposite,
								},
							},
							Conditions: []TargetedCondition{
								{
									Condition: xpv2.Condition{
										Type:    "DatabaseReady",
										Status:  corev1.ConditionTrue,
										Reason:  "Available",
										Message: "This is a condition for database availability.",
									},
									Target: CompositionTargetCompositeAndClaim,
								},
								{
									Condition: xpv2.Condition{
										Type:               "InternalSync",
										Status:             corev1.ConditionTrue,
										LastTransitionTime: metav1.Time{},
										Reason:             "SyncSuccess",
										Message:            "This is a condition representing an internal sync process.",
										ObservedGeneration: 0,
									},
									Target: CompositionTargetComposite,
								},
							},
						}, nil
					})),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"CustomEventsAndConditionFatal": {
			reason: "In the case of a fatal result from the composer, we should set all custom conditions that were seen. If any custom conditions were not seen, they should be marked as Unknown. The error message should be emitted as an event to the composite but not the claim.",
			args: args{
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						if xr, ok := obj.(*composite.Unstructured); ok {
							// non-nil claim ref to trigger claim Get()
							xr.SetClaimReference(&reference.Claim{})
							xr.SetConditions(xpv2.Condition{
								Type:    "DatabaseReady",
								Status:  corev1.ConditionTrue,
								Reason:  "Available",
								Message: "This is a condition for database availability.",
							})
							xr.SetClaimConditionTypes("DatabaseReady")
							return nil
						}
						if cm, ok := obj.(*claim.Unstructured); ok {
							claim.New(claim.WithGroupVersionKind(schema.GroupVersionKind{})).DeepCopyInto(cm)
							return nil
						}
						return nil
					}),
					MockStatusUpdate: WantComposite(t, NewComposite(func(cr *composite.Unstructured) {
						cr.Schema = composite.SchemaLegacy
						cr.SetCompositionReference(&corev1.ObjectReference{})
						cr.SetConditions(
							xpv2.Condition{
								Type:    "DatabaseReady",
								Status:  corev1.ConditionUnknown,
								Reason:  "FatalError",
								Message: "A fatal error occurred before the status of this condition could be determined.",
							},
							v1.WatchCircuitClosed(),
							xpv2.ReconcileError(fmt.Errorf("cannot compose resources: %w", errBoom)),
							xpv2.Condition{
								Type:               "InternalSync",
								Status:             corev1.ConditionTrue,
								LastTransitionTime: metav1.Time{},
								Reason:             "SyncSuccess",
								Message:            "This is a condition representing an internal sync process.",
								ObservedGeneration: 0,
							},
							xpv2.Condition{
								Type:               "BucketReady",
								Status:             corev1.ConditionTrue,
								LastTransitionTime: metav1.Time{},
								Reason:             "Available",
								Message:            "This is a condition for bucket availability.",
								ObservedGeneration: 0,
							},
						)

						cr.SetClaimConditionTypes(
							"DatabaseReady",
							"BucketReady",
						)
						cr.SetClaimReference(&reference.Claim{})
					})),
				},
				opts: []ReconcilerOption{
					// Only legacy XRs support claims.
					WithCompositeSchema(composite.SchemaLegacy),
					WithRecorder(newTestRecorder(
						eventArgs{
							Kind: compositeKind,
							Event: event.Event{
								Type:        event.Type(corev1.EventTypeNormal),
								Reason:      "SelectComposition",
								Message:     "Successfully selected composition: ",
								Annotations: map[string]string{},
							},
						},
						eventArgs{
							Kind:  compositeKind,
							Event: event.Warning("ComposeResources", fmt.Errorf("cannot compose resources: %w", errBoom)),
						},
						eventArgs{
							Kind: compositeKind,
							Event: event.Event{
								Type:    event.TypeNormal,
								Reason:  "DatabaseAvailable",
								Message: "Pipeline step \"some-function\": This is an event for database availability.",
							},
						},
						eventArgs{
							Kind: claimKind,
							Event: event.Event{
								Type:   event.TypeNormal,
								Reason: "DatabaseAvailable",
								// The claim should not have the "Pipeline step" prefix.
								Message: "This is an event for database availability.",
							},
						},
						eventArgs{
							Kind: compositeKind,
							Event: event.Event{
								Type:   event.TypeNormal,
								Reason: "SyncSuccess",
								// The composite should have the "Pipeline step" prefix.
								Message:     "Pipeline step \"some-function\": Internal sync was successful.",
								Annotations: map[string]string{},
							},
						},
						eventArgs{
							Kind: compositeKind,
							Event: event.Event{
								Type:   event.TypeNormal,
								Reason: "EventNoDetail",
								// The composite should not have the prefix as it had an empty
								// detail.
								Message:     "This event should not contain a detail prefix.",
								Annotations: map[string]string{},
							},
						},
					)),
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionRevisionFetcher(CompositionRevisionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.CompositionRevision, error) {
						return NewCompositionRevision(), nil
					})),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.CompositionRevision) error {
						return nil
					})),
					WithComposer(ComposerFn(func(_ context.Context, _ *composite.Unstructured, _ CompositionRequest) (CompositionResult, error) {
						return CompositionResult{
							Composed:          []ComposedResource{},
							ConnectionDetails: cd,
							Events: []TargetedEvent{
								{
									Event: event.Event{
										Type:    event.TypeNormal,
										Reason:  "DatabaseAvailable",
										Message: "This is an event for database availability.",
									},
									Detail: "Pipeline step \"some-function\"",
									Target: CompositionTargetCompositeAndClaim,
								},
								{
									Event: event.Event{
										Type:        event.TypeNormal,
										Reason:      "SyncSuccess",
										Message:     "Internal sync was successful.",
										Annotations: map[string]string{},
									},
									Detail: "Pipeline step \"some-function\"",
									Target: CompositionTargetComposite,
								},
								{
									Event: event.Event{
										Type:        event.TypeNormal,
										Reason:      "EventNoDetail",
										Message:     "This event should not contain a detail prefix.",
										Annotations: map[string]string{},
									},
									Target: CompositionTargetComposite,
								},
							},
							Conditions: []TargetedCondition{
								{
									Condition: xpv2.Condition{
										Type:               "InternalSync",
										Status:             corev1.ConditionTrue,
										LastTransitionTime: metav1.Time{},
										Reason:             "SyncSuccess",
										Message:            "This is a condition representing an internal sync process.",
										ObservedGeneration: 0,
									},
									Target: CompositionTargetComposite,
								},
								{
									Condition: xpv2.Condition{
										Type:               "BucketReady",
										Status:             corev1.ConditionTrue,
										LastTransitionTime: metav1.Time{},
										Reason:             "Available",
										Message:            "This is a condition for bucket availability.",
										ObservedGeneration: 0,
									},
									Target: CompositionTargetCompositeAndClaim,
								},
							},
						}, errBoom
					})),
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: cmpopts.AnyError,
			},
		},
		"CustomConditionUpdate": {
			reason: "Custom conditions should be updated if they already exist. Additionally, if a condition already exists in the status but was not included in the response, it should remain in the status.",
			args: args{
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						if xr, ok := obj.(*composite.Unstructured); ok {
							// non-nil claim ref to trigger claim Get()
							xr.SetClaimReference(&reference.Claim{})
							// The database condition already exists on the XR.
							xr.SetConditions(xpv2.Condition{
								Type:    "DatabaseReady",
								Status:  corev1.ConditionTrue,
								Reason:  "Available",
								Message: "This is a condition for database availability.",
							})
							// The bucket began in a non-ready state.
							xr.SetConditions(xpv2.Condition{
								Type:    "BucketReady",
								Status:  corev1.ConditionFalse,
								Reason:  "Creating",
								Message: "Waiting for bucket to be created.",
							})

							xr.SetClaimConditionTypes("DatabaseReady", "BucketReady")
							return nil
						}
						if cm, ok := obj.(*claim.Unstructured); ok {
							claim.New(claim.WithGroupVersionKind(schema.GroupVersionKind{})).DeepCopyInto(cm)
							return nil
						}
						return nil
					}),
					MockStatusUpdate: WantComposite(t, NewComposite(func(cr *composite.Unstructured) {
						cr.Schema = composite.SchemaLegacy
						cr.SetCompositionReference(&corev1.ObjectReference{})
						cr.SetConditions(
							// The database condition should exist even though it was not seen
							// during this reconcile.
							xpv2.Condition{
								Type:    "DatabaseReady",
								Status:  corev1.ConditionTrue,
								Reason:  "Available",
								Message: "This is a condition for database availability.",
							},
							// The bucket condition should be updated to reflect the latest
							// condition which is available.
							xpv2.Condition{
								Type:               "BucketReady",
								Status:             corev1.ConditionTrue,
								LastTransitionTime: metav1.Time{},
								Reason:             "Available",
								Message:            "This is a condition for bucket availability.",
								ObservedGeneration: 0,
							},
							v1.WatchCircuitClosed(),
							xpv2.Condition{
								Type:               "InternalSync",
								Status:             corev1.ConditionTrue,
								LastTransitionTime: metav1.Time{},
								Reason:             "SyncSuccess",
								Message:            "This is a condition representing an internal sync process.",
								ObservedGeneration: 0,
							},
							xpv2.ReconcileSuccess(),
							xpv2.Available(),
						)
						cr.SetClaimConditionTypes(
							// The database claim condition should exist even though it was
							// not seen during this reconcile.
							"DatabaseReady",
							"BucketReady",
						)
						cr.SetClaimReference(&reference.Claim{})
					})),
				},
				opts: []ReconcilerOption{
					// Only legacy XRs support claims.
					WithCompositeSchema(composite.SchemaLegacy),
					WithRecorder(newTestRecorder(
						eventArgs{
							Kind: compositeKind,
							Event: event.Event{
								Type:        event.Type(corev1.EventTypeNormal),
								Reason:      "SelectComposition",
								Message:     "Successfully selected composition: ",
								Annotations: map[string]string{},
							},
						},
					)),
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionRevisionFetcher(CompositionRevisionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.CompositionRevision, error) {
						return NewCompositionRevision(), nil
					})),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.CompositionRevision) error {
						return nil
					})),
					WithComposer(ComposerFn(func(_ context.Context, _ *composite.Unstructured, _ CompositionRequest) (CompositionResult, error) {
						return CompositionResult{
							Composed:          []ComposedResource{},
							ConnectionDetails: cd,
							Events:            []TargetedEvent{},
							Conditions: []TargetedCondition{
								// The database condition is not added to the XR again.
								{
									Condition: xpv2.Condition{
										Type:               "InternalSync",
										Status:             corev1.ConditionTrue,
										LastTransitionTime: metav1.Time{},
										Reason:             "SyncSuccess",
										Message:            "This is a condition representing an internal sync process.",
										ObservedGeneration: 0,
									},
									Target: CompositionTargetComposite,
								},
								// The bucket is now ready.
								{
									Condition: xpv2.Condition{
										Type:               "BucketReady",
										Status:             corev1.ConditionTrue,
										LastTransitionTime: metav1.Time{},
										Reason:             "Available",
										Message:            "This is a condition for bucket availability.",
										ObservedGeneration: 0,
									},
									Target: CompositionTargetCompositeAndClaim,
								},
							},
						}, nil
					})),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"SystemConditionUpdate": {
			reason: "A system condition should be updated if it is explicitly allowed to do so",
			args: args{
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						if xr, ok := obj.(*composite.Unstructured); ok {
							// non-nil claim ref to trigger claim Get()
							xr.SetClaimReference(&reference.Claim{})
							return nil
						}
						if cm, ok := obj.(*claim.Unstructured); ok {
							claim.New(claim.WithGroupVersionKind(schema.GroupVersionKind{})).DeepCopyInto(cm)
							return nil
						}
						return nil
					}),
					MockStatusUpdate: WantComposite(t, NewComposite(func(cr *composite.Unstructured) {
						cr.Schema = composite.SchemaLegacy
						cr.SetCompositionReference(&corev1.ObjectReference{})
						cr.SetConditions(v1.WatchCircuitClosed(), xpv2.ReconcileSuccess(), xpv2.Creating())
						cr.SetClaimReference(&reference.Claim{})
					})),
				},
				opts: []ReconcilerOption{
					// Only legacy XRs support claims.
					WithCompositeSchema(composite.SchemaLegacy),
					WithRecorder(newTestRecorder(
						eventArgs{
							Kind: compositeKind,
							Event: event.Event{
								Type:        event.Type(corev1.EventTypeNormal),
								Reason:      "SelectComposition",
								Message:     "Successfully selected composition: ",
								Annotations: map[string]string{},
							},
						},
					)),
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionRevisionFetcher(CompositionRevisionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.CompositionRevision, error) {
						return NewCompositionRevision(), nil
					})),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.CompositionRevision) error {
						return nil
					})),
					WithComposer(ComposerFn(func(_ context.Context, _ *composite.Unstructured, _ CompositionRequest) (CompositionResult, error) {
						return CompositionResult{
							Composed:          []ComposedResource{},
							ConnectionDetails: cd,
							Ready:             new(false),
							Events:            []TargetedEvent{},
							Conditions:        []TargetedCondition{},
						}, nil
					})),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"CustomEventsFailToGetClaim": {
			reason: "We should emit custom events that were returned by the composer. If we cannot get the claim, we should just emit events for the composite and continue as normal.",
			args: args{
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
						if xr, ok := obj.(*composite.Unstructured); ok {
							// non-nil claim ref to trigger claim Get()
							xr.SetClaimReference(&reference.Claim{})
							return nil
						}
						if _, ok := obj.(*claim.Unstructured); ok {
							// something went wrong when getting the claim
							return errBoom
						}
						return nil
					}),
					MockStatusUpdate: WantComposite(t, NewComposite(func(cr *composite.Unstructured) {
						cr.Schema = composite.SchemaLegacy
						cr.SetCompositionReference(&corev1.ObjectReference{})
						cr.SetConditions(v1.WatchCircuitClosed(), xpv2.ReconcileSuccess(), xpv2.Available())
						cr.SetClaimReference(&reference.Claim{})
					})),
				},
				opts: []ReconcilerOption{
					// Only legacy XRs support claims.
					WithCompositeSchema(composite.SchemaLegacy),
					WithRecorder(newTestRecorder(
						eventArgs{
							Kind: compositeKind,
							Event: event.Event{
								Type:        event.Type(corev1.EventTypeNormal),
								Reason:      "SelectComposition",
								Message:     "Successfully selected composition: ",
								Annotations: map[string]string{},
							},
						},
						eventArgs{
							Kind: compositeKind,
							Event: event.Event{
								Type:        event.TypeNormal,
								Reason:      "DatabaseAvailable",
								Message:     "Pipeline step \"some-function\": This is an event for database availability.",
								Annotations: map[string]string{},
							},
						},
					)),
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionRevisionFetcher(CompositionRevisionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.CompositionRevision, error) {
						return NewCompositionRevision(), nil
					})),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.CompositionRevision) error {
						return nil
					})),
					WithComposer(ComposerFn(func(_ context.Context, _ *composite.Unstructured, _ CompositionRequest) (CompositionResult, error) {
						return CompositionResult{
							Composed:          []ComposedResource{},
							ConnectionDetails: cd,
							Events: []TargetedEvent{
								{
									Event: event.Event{
										Type:        event.TypeNormal,
										Reason:      "DatabaseAvailable",
										Message:     "This is an event for database availability.",
										Annotations: map[string]string{},
									},
									Detail: "Pipeline step \"some-function\"",
									Target: CompositionTargetCompositeAndClaim,
								},
							},
						}, nil
					})),
				},
			},
			want: want{
				r: reconcile.Result{},
			},
		},
		"CircuitBreakerOpen": {
			reason: "When the circuit breaker is open, we should set the CircuitOpen condition and continue with reconciliation.",
			args: args{
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
					MockStatusUpdate: WantComposite(t, NewComposite(func(cr *composite.Unstructured) {
						cr.SetConditions(v1.WatchCircuitOpen("ConfigMap/test-config (default)"), xpv2.ReconcileError(errors.Wrap(errBoom, errAddFinalizer)))
					})),
				},
				opts: []ReconcilerOption{
					// Use a failing finalizer to make reconcile return early,
					// avoiding the need to mock the entire composition flow.
					WithCompositeFinalizer(resource.FinalizerFns{
						AddFinalizerFn: func(_ context.Context, _ resource.Object) error {
							return errBoom
						},
					}),
					WithCircuitBreaker(&MockCircuitBreaker{
						MockGetState: func(_ context.Context, _ types.NamespacedName) circuit.State {
							return circuit.State{
								IsOpen:      true,
								TriggeredBy: "ConfigMap/test-config (default)",
							}
						},
					}),
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: cmpopts.AnyError,
			},
		},
		"CircuitBreakerClosed": {
			reason: "When the circuit breaker is closed, we should set the CircuitClosed condition and continue with reconciliation.",
			args: args{
				c: &test.MockClient{
					MockGet: test.NewMockGetFn(nil),
					MockStatusUpdate: WantComposite(t, NewComposite(func(cr *composite.Unstructured) {
						cr.SetConditions(v1.WatchCircuitClosed(), xpv2.ReconcileError(errors.Wrap(errBoom, errAddFinalizer)))
					})),
				},
				opts: []ReconcilerOption{
					// Use a failing finalizer to make reconcile return early,
					// avoiding the need to mock the entire composition flow.
					WithCompositeFinalizer(resource.FinalizerFns{
						AddFinalizerFn: func(_ context.Context, _ resource.Object) error {
							return errBoom
						},
					}),
					WithCircuitBreaker(&MockCircuitBreaker{
						MockGetState: func(_ context.Context, _ types.NamespacedName) circuit.State {
							return circuit.State{
								IsOpen: false,
							}
						},
					}),
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: cmpopts.AnyError,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewReconciler(tc.args.c, tc.args.of, tc.args.opts...)

			got, err := r.Reconcile(context.Background(), reconcile.Request{})
			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.r, got); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want, +got:\n%s", tc.reason, diff)
			}

			if tr, ok := r.record.(*testRecorder); ok {
				if diff := cmp.Diff(tr.Want, tr.Got); diff != "" {
					t.Errorf("\n%s\nr.Reconcile(...): -want events, +got events:\n%s", tc.reason, diff)
				}
			}
		})
	}
}

func TestEffectivePollInterval(t *testing.T) {
	cases := map[string]struct {
		reason          string
		pollInterval    time.Duration
		minPollInterval time.Duration
		annotation      string
		want            time.Duration
	}{
		"NoAnnotationReturnsDefault": {
			reason:          "When no annotation is set, the default poll interval is returned.",
			pollInterval:    10 * time.Minute,
			minPollInterval: 1 * time.Second,
			annotation:      "",
			want:            10 * time.Minute,
		},
		"ValidAnnotationAboveMinimumReturnsAnnotation": {
			reason:          "When the annotation is at or above the minimum, it overrides the default.",
			pollInterval:    10 * time.Minute,
			minPollInterval: 1 * time.Second,
			annotation:      "24h",
			want:            24 * time.Hour,
		},
		"ValidAnnotationAtMinimumReturnsAnnotation": {
			reason:          "When the annotation equals the minimum exactly, it is returned as-is.",
			pollInterval:    10 * time.Minute,
			minPollInterval: 30 * time.Second,
			annotation:      "30s",
			want:            30 * time.Second,
		},
		"ValidAnnotationBelowMinimumReturnsMinimum": {
			reason:          "When the annotation is below the minimum, the minimum is returned rather than the default.",
			pollInterval:    10 * time.Minute,
			minPollInterval: 30 * time.Second,
			annotation:      "1s",
			want:            30 * time.Second,
		},
		"InvalidAnnotationReturnsDefault": {
			reason:          "When the annotation cannot be parsed, the default poll interval is returned.",
			pollInterval:    5 * time.Minute,
			minPollInterval: 1 * time.Second,
			annotation:      "not-a-duration",
			want:            5 * time.Minute,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := &Reconciler{
				pollInterval:    tc.pollInterval,
				minPollInterval: tc.minPollInterval,
			}

			xr := NewComposite(func(cr *composite.Unstructured) {
				if tc.annotation != "" {
					cr.SetAnnotations(map[string]string{
						meta.AnnotationKeyPollInterval: tc.annotation,
					})
				}
			})

			got := r.effectivePollInterval(xr)
			if got != tc.want {
				t.Errorf("\n%s\neffectivePollInterval(...): want %v, got %v", tc.reason, tc.want, got)
			}
		})
	}
}

func TestReconcilePollIntervalAnnotation(t *testing.T) {
	now := metav1.Now()

	cases := map[string]struct {
		reason        string
		pollInterval  time.Duration
		annotation    string
		wantApprox    time.Duration
		wantTolerance time.Duration
	}{
		"AnnotationOverridesPollInterval": {
			reason:        "When a valid poll interval annotation is set, it should override the controller-level poll interval.",
			pollInterval:  1 * time.Minute,
			annotation:    "24h",
			wantApprox:    24 * time.Hour,
			wantTolerance: 3 * time.Hour,
		},
		"InvalidAnnotationFallsBack": {
			reason:        "When an invalid poll interval annotation is set, the controller-level poll interval should be used.",
			pollInterval:  5 * time.Minute,
			annotation:    "not-a-duration",
			wantApprox:    5 * time.Minute,
			wantTolerance: 1 * time.Minute,
		},
		"BelowMinimumClampedToMinimum": {
			reason:        "When the poll interval annotation is below the minimum, the minimum poll interval should be used.",
			pollInterval:  5 * time.Minute,
			annotation:    "1ms",
			wantApprox:    1 * time.Second,
			wantTolerance: 500 * time.Millisecond,
		},
		"NoAnnotationUsesDefault": {
			reason:        "When no poll interval annotation is set, the controller-level poll interval should be used.",
			pollInterval:  10 * time.Minute,
			annotation:    "",
			wantApprox:    10 * time.Minute,
			wantTolerance: 2 * time.Minute,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			annotations := map[string]string{}
			if tc.annotation != "" {
				annotations[meta.AnnotationKeyPollInterval] = tc.annotation
			}

			c := &test.MockClient{
				MockGet: WithComposite(t, NewComposite(func(cr *composite.Unstructured) {
					if len(annotations) > 0 {
						cr.SetAnnotations(annotations)
					}
				})),
				MockStatusUpdate: WantComposite(t, NewComposite(func(cr *composite.Unstructured) {
					cr.SetCompositionReference(&corev1.ObjectReference{})
					if len(annotations) > 0 {
						cr.SetAnnotations(annotations)
					}
					cr.SetConditions(v1.WatchCircuitClosed(), xpv2.ReconcileSuccess(), xpv2.Available())
					cr.SetConnectionDetailsLastPublishedTime(&now)
				})),
			}

			opts := []ReconcilerOption{
				WithPollInterval(tc.pollInterval),
				WithMinPollInterval(1 * time.Second),
				WithCompositeFinalizer(resource.NewNopFinalizer()),
				WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
					cr.SetCompositionReference(&corev1.ObjectReference{})
					return nil
				})),
				WithCompositionRevisionFetcher(CompositionRevisionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.CompositionRevision, error) {
					return NewCompositionRevision(), nil
				})),
				WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.CompositionRevision) error {
					return nil
				})),
				WithComposer(ComposerFn(func(_ context.Context, _ *composite.Unstructured, _ CompositionRequest) (CompositionResult, error) {
					return CompositionResult{}, nil
				})),
				WithConnectionPublishers(ConnectionPublisherFn(func(_ context.Context, _ ConnectionSecretOwner, _ managed.ConnectionDetails) (published bool, err error) {
					return true, nil
				})),
			}

			r := NewReconciler(c, schema.GroupVersionKind{}, opts...)
			got, err := r.Reconcile(context.Background(), reconcile.Request{})
			if err != nil {
				t.Errorf("\n%s\nr.Reconcile(...): unexpected error: %v", tc.reason, err)
			}

			diff := got.RequeueAfter - tc.wantApprox
			if diff < 0 {
				diff = -diff
			}
			if diff > tc.wantTolerance {
				t.Errorf("\n%s\nr.Reconcile(...): want RequeueAfter ~%v (±%v), got %v",
					tc.reason, tc.wantApprox, tc.wantTolerance, got.RequeueAfter)
			}
		})
	}
}

func TestReconcileRequestAnnotation(t *testing.T) {
	now := metav1.Now()

	type args struct {
		c    client.Client
		of   schema.GroupVersionKind
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
		"NewReconcileRequestRecordsToken": {
			reason: "A new reconcile-requested-at token should be recorded in status.lastHandledReconcileAt.",
			args: args{
				c: &test.MockClient{
					MockGet: WithComposite(t, NewComposite(func(cr *composite.Unstructured) {
						cr.SetAnnotations(map[string]string{
							meta.AnnotationKeyReconcileRequestedAt: "1705312200",
						})
					})),
					MockStatusUpdate: WantComposite(t, NewComposite(func(cr *composite.Unstructured) {
						cr.SetAnnotations(map[string]string{
							meta.AnnotationKeyReconcileRequestedAt: "1705312200",
						})
						cr.SetCompositionReference(&corev1.ObjectReference{})
						cr.SetConditions(v1.WatchCircuitClosed(), xpv2.ReconcileSuccess(), xpv2.Available())
						cr.SetConnectionDetailsLastPublishedTime(&now)
						cr.SetLastHandledReconcileAt("1705312200")
					})),
				},
				opts: []ReconcilerOption{
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionRevisionFetcher(CompositionRevisionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.CompositionRevision, error) {
						return NewCompositionRevision(), nil
					})),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.CompositionRevision) error {
						return nil
					})),
					WithComposer(ComposerFn(func(_ context.Context, _ *composite.Unstructured, _ CompositionRequest) (CompositionResult, error) {
						return CompositionResult{}, nil
					})),
					WithConnectionPublishers(ConnectionPublisherFn(func(_ context.Context, _ ConnectionSecretOwner, _ managed.ConnectionDetails) (published bool, err error) {
						return true, nil
					})),
					WithRecorder(newTestRecorder(
						eventArgs{
							Kind: compositeKind,
							Event: event.Event{
								Type:        event.TypeNormal,
								Reason:      "ReconcileRequestHandled",
								Message:     "Handling reconcile request",
								Annotations: map[string]string{"token": "1705312200"},
							},
						},
						eventArgs{
							Kind: compositeKind,
							Event: event.Event{
								Type:        event.TypeNormal,
								Reason:      "SelectComposition",
								Message:     "Successfully selected composition: ",
								Annotations: map[string]string{},
							},
						},
						eventArgs{
							Kind: compositeKind,
							Event: event.Event{
								Type:        event.TypeNormal,
								Reason:      "PublishConnectionSecret",
								Message:     "Successfully published connection details",
								Annotations: map[string]string{},
							},
						},
					)),
				},
			},
		},
		"AlreadyHandledReconcileRequestIsNoOp": {
			reason: "When the token matches lastHandledReconcileAt, status should remain unchanged.",
			args: args{
				c: &test.MockClient{
					MockGet: WithComposite(t, NewComposite(func(cr *composite.Unstructured) {
						cr.SetAnnotations(map[string]string{
							meta.AnnotationKeyReconcileRequestedAt: "already-handled",
						})
						cr.SetLastHandledReconcileAt("already-handled")
					})),
					MockStatusUpdate: WantComposite(t, NewComposite(func(cr *composite.Unstructured) {
						cr.SetAnnotations(map[string]string{
							meta.AnnotationKeyReconcileRequestedAt: "already-handled",
						})
						cr.SetCompositionReference(&corev1.ObjectReference{})
						cr.SetConditions(v1.WatchCircuitClosed(), xpv2.ReconcileSuccess(), xpv2.Available())
						cr.SetConnectionDetailsLastPublishedTime(&now)
						cr.SetLastHandledReconcileAt("already-handled")
					})),
				},
				opts: []ReconcilerOption{
					WithCompositeFinalizer(resource.NewNopFinalizer()),
					WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
						cr.SetCompositionReference(&corev1.ObjectReference{})
						return nil
					})),
					WithCompositionRevisionFetcher(CompositionRevisionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.CompositionRevision, error) {
						return NewCompositionRevision(), nil
					})),
					WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.CompositionRevision) error {
						return nil
					})),
					WithComposer(ComposerFn(func(_ context.Context, _ *composite.Unstructured, _ CompositionRequest) (CompositionResult, error) {
						return CompositionResult{}, nil
					})),
					WithConnectionPublishers(ConnectionPublisherFn(func(_ context.Context, _ ConnectionSecretOwner, _ managed.ConnectionDetails) (published bool, err error) {
						return true, nil
					})),
					WithRecorder(newTestRecorder(
						eventArgs{
							Kind: compositeKind,
							Event: event.Event{
								Type:        event.TypeNormal,
								Reason:      "SelectComposition",
								Message:     "Successfully selected composition: ",
								Annotations: map[string]string{},
							},
						},
						eventArgs{
							Kind: compositeKind,
							Event: event.Event{
								Type:        event.TypeNormal,
								Reason:      "PublishConnectionSecret",
								Message:     "Successfully published connection details",
								Annotations: map[string]string{},
							},
						},
					)),
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewReconciler(tc.args.c, tc.args.of, tc.args.opts...)
			got, err := r.Reconcile(context.Background(), reconcile.Request{})

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.r, got); diff != "" {
				t.Errorf("\n%s\nr.Reconcile(...): -want, +got:\n%s", tc.reason, diff)
			}

			if tr, ok := r.record.(*testRecorder); ok {
				if diff := cmp.Diff(tr.Want, tr.Got); diff != "" {
					t.Errorf("\n%s\nr.Reconcile(...): -want events, +got events:\n%s", tc.reason, diff)
				}
			}
		})
	}
}

// MockCircuitBreaker is a mock implementation of circuit.Breaker for testing.
type MockCircuitBreaker struct {
	MockGetState    func(ctx context.Context, target types.NamespacedName) circuit.State
	MockRecordEvent func(ctx context.Context, target types.NamespacedName, es circuit.EventSource, et circuit.EventType)
	MockResetTarget func(ctx context.Context, target types.NamespacedName)
}

// GetState calls MockGetState if set, otherwise returns a closed circuit.
func (m *MockCircuitBreaker) GetState(ctx context.Context, target types.NamespacedName) circuit.State {
	if m.MockGetState != nil {
		return m.MockGetState(ctx, target)
	}
	return circuit.State{IsOpen: false}
}

// RecordEvent calls MockRecordEvent if set.
func (m *MockCircuitBreaker) RecordEvent(ctx context.Context, target types.NamespacedName, es circuit.EventSource, et circuit.EventType) {
	if m.MockRecordEvent != nil {
		m.MockRecordEvent(ctx, target, es, et)
	}
}

// ResetTarget calls MockResetTarget if set.
func (m *MockCircuitBreaker) ResetTarget(ctx context.Context, target types.NamespacedName) {
	if m.MockResetTarget != nil {
		m.MockResetTarget(ctx, target)
	}
}

func NewCompositionRevision() *v1.CompositionRevision {
	rev := &v1.CompositionRevision{}
	rev.SetConditions(v1.ValidPipeline())
	return rev
}

type CompositeModifier func(cr *composite.Unstructured)

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

// Test types.
const (
	compositeKind = "Composite"
	claimKind     = "Claim"
)

// testRecorder allows asserting event creation.
type testRecorder struct {
	Want []eventArgs
	Got  []eventArgs
}

type eventArgs struct {
	Kind  string
	Event event.Event
}

func (r *testRecorder) Event(obj runtime.Object, e event.Event) {
	var kind string

	switch obj.(type) {
	case *composite.Unstructured:
		kind = compositeKind
	case *claim.Unstructured:
		kind = claimKind
	}

	r.Got = append(r.Got, eventArgs{Kind: kind, Event: e})
}

func (r *testRecorder) WithAnnotations(_ ...string) event.Recorder {
	return r
}

func newTestRecorder(expected ...eventArgs) *testRecorder {
	return &testRecorder{
		Want: expected,
	}
}

// newDeletionPolicyReconciler returns a Reconciler for the deletion policy
// tests. The XR's Composition resolves to the supplied revision.
func newDeletionPolicyReconciler(c client.Client, f *feature.Flags, rev func() (*v1.CompositionRevision, error), compose ComposerFn, o ...ReconcilerOption) *Reconciler {
	return NewReconciler(c, schema.GroupVersionKind{}, append([]ReconcilerOption{
		WithFeatures(f),
		WithPollInterval(time.Minute),
		WithCompositionSelector(CompositionSelectorFn(func(_ context.Context, cr resource.Composite) error {
			cr.SetCompositionReference(&corev1.ObjectReference{})
			return nil
		})),
		WithCompositionRevisionFetcher(CompositionRevisionFetcherFn(func(_ context.Context, _ resource.Composite) (*v1.CompositionRevision, error) {
			return rev()
		})),
		WithConfigurator(ConfiguratorFn(func(_ context.Context, _ resource.Composite, _ *v1.CompositionRevision) error {
			return nil
		})),
		WithComposer(compose),
	}, o...)...)
}

func TestReconcileDeleteFollowsDeletionPolicy(t *testing.T) {
	now := metav1.Now()
	errBoom := errors.New("boom")

	type args struct {
		finalizers []string
		recorded   v1.CompositionDeletionPolicy // What the XR recorded while live.
		policy     v1.CompositionDeletionPolicy // The revision's policy.
		noRevision bool                         // Is the revision gone?
		invalid    bool                         // Does the revision have an invalid pipeline?
		disabled   bool                         // Is function-controlled deletion disabled?
		compose    func() (CompositionResult, error)
	}

	type want struct {
		// Which path the delete took. Background, and anything that defers to
		// Kubernetes, only removes the finalizer.
		ranPipeline           bool
		ranForegroundDeletion bool
		removedFinalizer      bool

		requeue  bool                         // Did we requeue?
		syncErr  bool                         // Did the Synced condition report an error?
		recorded v1.CompositionDeletionPolicy // The policy in the last status update.
		err      bool
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NoRecordedPolicy": {
			reason: "An XR that never recorded a policy uses Background, so it finishes deleting immediately.",
			want: want{
				removedFinalizer: true,
			},
		},
		"Foreground": {
			reason: "An XR with the Foreground policy is deleted in the foreground, without running its pipeline.",
			args: args{
				recorded: v1.CompositionDeletionPolicyForeground,
			},
			want: want{
				ranForegroundDeletion: true,
				requeue:               true,
				recorded:              v1.CompositionDeletionPolicyForeground,
			},
		},
		"Pipeline": {
			reason: "An XR with the Pipeline policy runs its pipeline, and waits while composed resources still exist.",
			args: args{
				recorded: v1.CompositionDeletionPolicyPipeline,
				policy:   v1.CompositionDeletionPolicyPipeline,
				compose: func() (CompositionResult, error) {
					return CompositionResult{Existing: 1}, nil
				},
			},
			want: want{
				ranPipeline: true,
				requeue:     true,
				recorded:    v1.CompositionDeletionPolicyPipeline,
			},
		},
		"PipelineDeletedEverything": {
			reason: "Once the pipeline desires nothing and nothing exists, the XR finishes deleting.",
			args: args{
				recorded: v1.CompositionDeletionPolicyPipeline,
				policy:   v1.CompositionDeletionPolicyPipeline,
				compose: func() (CompositionResult, error) {
					return CompositionResult{}, nil
				},
			},
			want: want{
				ranPipeline:      true,
				removedFinalizer: true,
				recorded:         v1.CompositionDeletionPolicyPipeline,
			},
		},
		"PipelineFails": {
			reason: "An XR with the Pipeline policy whose pipeline fails is held.",
			args: args{
				recorded: v1.CompositionDeletionPolicyPipeline,
				policy:   v1.CompositionDeletionPolicyPipeline,
				compose: func() (CompositionResult, error) {
					return CompositionResult{}, errBoom
				},
			},
			want: want{
				ranPipeline: true,
				syncErr:     true,
				recorded:    v1.CompositionDeletionPolicyPipeline,
				err:         true,
			},
		},
		"PipelineWithoutComposition": {
			reason: "An XR with the Pipeline policy whose Composition is gone can't run its pipeline, so it's held.",
			args: args{
				recorded:   v1.CompositionDeletionPolicyPipeline,
				noRevision: true,
			},
			want: want{
				syncErr:  true,
				recorded: v1.CompositionDeletionPolicyPipeline,
				err:      true,
			},
		},
		"PipelineNowForeground": {
			reason: "If an XR's Composition changed from Pipeline to Foreground, the XR records Foreground and requeues to use it. It doesn't need a valid pipeline to do so.",
			args: args{
				recorded: v1.CompositionDeletionPolicyPipeline,
				policy:   v1.CompositionDeletionPolicyForeground,
				invalid:  true,
			},
			want: want{
				requeue:  true,
				recorded: v1.CompositionDeletionPolicyForeground,
			},
		},
		"PipelineNowBackground": {
			reason: "If an XR's Composition changed from Pipeline to Background, the XR records Background and requeues to use it.",
			args: args{
				recorded: v1.CompositionDeletionPolicyPipeline,
			},
			want: want{
				requeue:  true,
				recorded: v1.CompositionDeletionPolicyBackground,
			},
		},
		"OrphanPropagation": {
			reason: "An XR deleted with orphan propagation leaves its composed resources alone, whatever its policy.",
			args: args{
				finalizers: []string{metav1.FinalizerOrphanDependents},
				recorded:   v1.CompositionDeletionPolicyPipeline,
			},
			want: want{
				removedFinalizer: true,
				recorded:         v1.CompositionDeletionPolicyPipeline,
			},
		},
		"ForegroundPropagation": {
			reason: "An XR deleted with foreground propagation defers to Kubernetes garbage collection, whatever its policy.",
			args: args{
				finalizers: []string{metav1.FinalizerDeleteDependents},
				recorded:   v1.CompositionDeletionPolicyPipeline,
			},
			want: want{
				removedFinalizer: true,
				recorded:         v1.CompositionDeletionPolicyPipeline,
			},
		},
		"Disabled": {
			reason: "An XR finishes deleting immediately when function-controlled deletion is disabled, whatever its policy.",
			args: args{
				recorded: v1.CompositionDeletionPolicyPipeline,
				disabled: true,
			},
			want: want{
				removedFinalizer: true,
				recorded:         v1.CompositionDeletionPolicyPipeline,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			xr := NewComposite(func(cr *composite.Unstructured) {
				cr.SetDeletionTimestamp(&now)
				cr.SetFinalizers(tc.args.finalizers)
				cr.SetDeletionPolicy(tc.args.recorded)
			})

			f := &feature.Flags{}
			if !tc.args.disabled {
				f.Enable(features.EnableAlphaFunctionControlledDeletion)
			}

			got := want{}

			var synced xpv2.Condition

			c := &test.MockClient{
				MockGet: WithComposite(t, xr),
				MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(obj client.Object) error {
					if xr, ok := obj.(*composite.Unstructured); ok {
						got.recorded = xr.GetDeletionPolicy()
						synced = xr.GetCondition(xpv2.TypeSynced)
					}

					return nil
				}),
			}

			rev := func() (*v1.CompositionRevision, error) {
				if tc.args.noRevision {
					return nil, errBoom
				}

				rev := NewCompositionRevision()
				rev.Spec.DeletionPolicy = tc.args.policy

				if tc.args.invalid {
					rev.SetConditions(xpv2.Condition{Type: v1.TypeValidPipeline, Status: corev1.ConditionFalse, Reason: "InvalidPipeline"})
				}

				return rev, nil
			}

			compose := ComposerFn(func(_ context.Context, _ *composite.Unstructured, _ CompositionRequest) (CompositionResult, error) {
				got.ranPipeline = true

				if tc.args.compose == nil {
					t.Errorf("\n%s\nReconcile(...): ran the function pipeline, which shouldn't run", tc.reason)
					return CompositionResult{}, nil
				}

				return tc.args.compose()
			})

			r := newDeletionPolicyReconciler(c, f, rev, compose,
				WithCompositeFinalizer(resource.FinalizerFns{
					RemoveFinalizerFn: func(_ context.Context, _ resource.Object) error {
						got.removedFinalizer = true
						return nil
					},
				}),
				WithForegroundDeletion(
					ComposedResourceObserverFn(func(_ context.Context, _ resource.Composite) (ComposedResourceStates, error) {
						return ComposedResourceStates{"first": state("Thing", "xr-first")}, nil
					}),
					ComposedResourceGarbageCollectorFn(func(_ context.Context, _ metav1.Object, _, _ ComposedResourceStates) error {
						got.ranForegroundDeletion = true
						return nil
					}),
				),
			)

			res, err := r.Reconcile(context.Background(), reconcile.Request{})
			got.err = err != nil
			got.requeue = !res.IsZero()
			got.syncErr = synced.Reason == xpv2.ReasonReconcileError

			if diff := cmp.Diff(tc.want, got, cmp.AllowUnexported(want{})); diff != "" {
				t.Errorf("\n%s\nReconcile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestDeleteInForeground(t *testing.T) {
	errBoom := errors.New("boom")

	orphaned := state("Thing", "xr-orphaned")
	orphaned.Resource.SetOwnerReferences(nil)

	type args struct {
		observed   ComposedResourceStates
		observeErr error
		gcErr      error
	}

	type want struct {
		deleted          []string // What did we garbage collect?
		requeue          bool     // Did we requeue?
		removedFinalizer bool     // Was our finalizer removed?
		err              bool
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ComposedResourcesExist": {
			reason: "We should delete the XR's composed resources, and keep its finalizer until they're gone.",
			args: args{
				observed: ComposedResourceStates{
					"first":  state("Thing", "xr-first"),
					"second": state("Thing", "xr-second"),
				},
			},
			want: want{
				deleted: []string{"first", "second"},
				requeue: true,
			},
		},
		"NothingLeft": {
			reason: "We should remove the XR's finalizer once its composed resources are gone.",
			args: args{
				observed: ComposedResourceStates{},
			},
			want: want{
				removedFinalizer: true,
			},
		},
		"OrphanedResourcesDontBlock": {
			reason: "We shouldn't wait for composed resources we don't control, because we'd never delete them.",
			args: args{
				observed: ComposedResourceStates{"orphaned": orphaned},
			},
			want: want{
				removedFinalizer: true,
			},
		},
		"ObserveError": {
			reason: "We should return any error encountered observing the XR's composed resources.",
			args: args{
				observeErr: errBoom,
			},
			want: want{
				err: true,
			},
		},
		"GarbageCollectError": {
			reason: "We should return any error encountered deleting the XR's composed resources.",
			args: args{
				observed: ComposedResourceStates{"first": state("Thing", "xr-first")},
				gcErr:    errBoom,
			},
			want: want{
				deleted: []string{"first"},
				err:     true,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := want{}

			r := NewReconciler(&test.MockClient{
				MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
			}, schema.GroupVersionKind{},
				WithPollInterval(time.Minute),
				WithCompositeFinalizer(resource.FinalizerFns{
					RemoveFinalizerFn: func(_ context.Context, _ resource.Object) error {
						got.removedFinalizer = true
						return nil
					},
				}),
				WithForegroundDeletion(
					ComposedResourceObserverFn(func(_ context.Context, _ resource.Composite) (ComposedResourceStates, error) {
						return tc.args.observed, tc.args.observeErr
					}),
					ComposedResourceGarbageCollectorFn(func(_ context.Context, _ metav1.Object, observed, _ ComposedResourceStates) error {
						for n := range observed {
							got.deleted = append(got.deleted, string(n))
						}

						return tc.args.gcErr
					}),
				),
			)

			xr := NewComposite()

			res, err := r.deleteInForeground(context.Background(), context.Background(), logging.NewNopLogger(), r.conditions.For(xr), xr)
			got.err = err != nil
			got.requeue = !res.IsZero()

			slices.Sort(got.deleted)

			if diff := cmp.Diff(tc.want, got, cmp.AllowUnexported(want{})); diff != "" {
				t.Errorf("\n%s\nr.deleteInForeground(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestReconcileRecordsDeletionPolicy(t *testing.T) {
	type args struct {
		enabled bool
		legacy  bool
		policy  v1.CompositionDeletionPolicy
		fails   bool
	}

	type want struct {
		recorded              v1.CompositionDeletionPolicy
		recordedBeforeCompose v1.CompositionDeletionPolicy
		err                   bool
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"Pipeline": {
			reason: "A Composition with the Pipeline policy is recorded as such.",
			args: args{
				enabled: true,
				policy:  v1.CompositionDeletionPolicyPipeline,
			},
			want: want{
				recorded:              v1.CompositionDeletionPolicyPipeline,
				recordedBeforeCompose: v1.CompositionDeletionPolicyPipeline,
			},
		},
		"PipelineThatFails": {
			reason: "A Composition with the Pipeline policy is recorded before composing, so the XR still runs its pipeline when it's deleted even if the pipeline is failing.",
			args: args{
				enabled: true,
				policy:  v1.CompositionDeletionPolicyPipeline,
				fails:   true,
			},
			want: want{
				recorded:              v1.CompositionDeletionPolicyPipeline,
				recordedBeforeCompose: v1.CompositionDeletionPolicyPipeline,
				err:                   true,
			},
		},
		"Foreground": {
			reason: "A Composition with the Foreground policy is recorded as such.",
			args: args{
				enabled: true,
				policy:  v1.CompositionDeletionPolicyForeground,
			},
			want: want{
				recorded:              v1.CompositionDeletionPolicyForeground,
				recordedBeforeCompose: v1.CompositionDeletionPolicyForeground,
			},
		},
		"Background": {
			reason: "A Composition with the Background policy is recorded as such.",
			args: args{
				enabled: true,
				policy:  v1.CompositionDeletionPolicyBackground,
			},
			want: want{
				recorded:              v1.CompositionDeletionPolicyBackground,
				recordedBeforeCompose: v1.CompositionDeletionPolicyBackground,
			},
		},
		"Unset": {
			reason: "A Composition that sets no policy is recorded as Background.",
			args: args{
				enabled: true,
			},
			want: want{
				recorded:              v1.CompositionDeletionPolicyBackground,
				recordedBeforeCompose: v1.CompositionDeletionPolicyBackground,
			},
		},
		"Legacy": {
			reason: "A legacy XR's schema has nowhere to record a policy, so nothing is recorded.",
			args: args{
				enabled: true,
				legacy:  true,
				policy:  v1.CompositionDeletionPolicyPipeline,
			},
			want: want{},
		},
		"Disabled": {
			reason: "Nothing is recorded unless function-controlled deletion is enabled.",
			args: args{
				policy: v1.CompositionDeletionPolicyPipeline,
			},
			want: want{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := &feature.Flags{}
			if tc.args.enabled {
				f.Enable(features.EnableAlphaFunctionControlledDeletion)
			}

			xr := NewComposite()
			if tc.args.legacy {
				xr.Schema = composite.SchemaLegacy
			}

			got := want{}

			c := &test.MockClient{
				MockGet: WithComposite(t, xr),
				MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(obj client.Object) error {
					if xr, ok := obj.(*composite.Unstructured); ok {
						got.recorded = xr.GetDeletionPolicy()
					}

					return nil
				}),
			}

			rev := func() (*v1.CompositionRevision, error) {
				rev := NewCompositionRevision()
				rev.Spec.DeletionPolicy = tc.args.policy

				return rev, nil
			}

			compose := ComposerFn(func(_ context.Context, _ *composite.Unstructured, _ CompositionRequest) (CompositionResult, error) {
				got.recordedBeforeCompose = got.recorded

				if tc.args.fails {
					return CompositionResult{}, errors.New("boom")
				}

				return CompositionResult{}, nil
			})

			r := newDeletionPolicyReconciler(c, f, rev, compose, WithCompositeFinalizer(resource.NewNopFinalizer()))

			_, err := r.Reconcile(context.Background(), reconcile.Request{})
			got.err = err != nil

			if diff := cmp.Diff(tc.want, got, cmp.AllowUnexported(want{})); diff != "" {
				t.Errorf("\n%s\nReconcile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

// state returns an observed composed resource that the XR controls.
func state(kind, name string) ComposedResourceState {
	cd := composed.New()
	cd.SetAPIVersion("example.org/v1")
	cd.SetKind(kind)
	cd.SetName(name)
	cd.SetOwnerReferences([]metav1.OwnerReference{{Controller: new(true)}})

	return ComposedResourceState{Resource: cd}
}
