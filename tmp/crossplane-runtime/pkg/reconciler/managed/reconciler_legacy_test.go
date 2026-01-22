/*
Copyright 2019 The Crossplane Authors.

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

package managed

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/v2/apis/changelogs/proto/v1alpha1"
	xpv1 "github.com/crossplane/crossplane/apis/v2/core"
	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
)

var _ reconcile.Reconciler = &Reconciler{}

func TestReconciler(t *testing.T) {
	type args struct {
		m  manager.Manager
		mg resource.ManagedKind
		o  []ReconcilerOption
	}

	type want struct {
		result        reconcile.Result
		resultCmpOpts []cmp.Option
		err           error
	}

	errBoom := errors.New("boom")
	now := metav1.Now()

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"GetManagedError": {
			reason: "Any error (except not found) encountered while getting the resource under reconciliation should be returned.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{MockGet: test.NewMockGetFn(errBoom)},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
			},
			want: want{err: errors.Wrap(errBoom, errGetManaged)},
		},
		"ManagedNotFound": {
			reason: "Not found errors encountered while getting the resource under reconciliation should be ignored.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, ""))},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
			},
			want: want{result: reconcile.Result{}},
		},
		"UnpublishConnectionDetailsDeletionPolicyDeleteOrpahn": {
			reason: "Errors unpublishing connection details should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							mg := asLegacyManaged(obj, 42)
							mg.SetDeletionTimestamp(&now)
							mg.SetDeletionPolicy(xpv1.DeletionOrphan)

							return nil
						}),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							want.SetDeletionTimestamp(&now)
							want.SetDeletionPolicy(xpv1.DeletionOrphan)
							want.SetConditions(xpv1.Deleting().WithObservedGeneration(42))
							want.SetConditions(xpv1.ReconcileError(errBoom).WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Errors unpublishing connection details should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					withConnectionPublishers(ConnectionPublisherFns{
						UnpublishConnectionFn: func(_ context.Context, _ resource.ConnectionSecretOwner, _ ConnectionDetails) error { return errBoom },
					}),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"RemoveFinalizerErrorDeletionPolicyOrphan": {
			reason: "Errors removing the managed resource finalizer should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							mg := asLegacyManaged(obj, 42)
							mg.SetDeletionTimestamp(&now)
							mg.SetDeletionPolicy(xpv1.DeletionOrphan)

							return nil
						}),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							want.SetDeletionTimestamp(&now)
							want.SetDeletionPolicy(xpv1.DeletionOrphan)
							want.SetConditions(xpv1.Deleting().WithObservedGeneration(42))
							want.SetConditions(xpv1.ReconcileError(errBoom).WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Errors removing the managed resource finalizer should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithFinalizer(resource.FinalizerFns{RemoveFinalizerFn: func(_ context.Context, _ resource.Object) error { return errBoom }}),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"DeleteSuccessfulDeletionPolicyOrphan": {
			reason: "Successful managed resource deletion with deletion policy Orphan should not trigger a requeue or status update.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							mg := asLegacyManaged(obj, 42)
							mg.SetDeletionTimestamp(&now)
							mg.SetDeletionPolicy(xpv1.DeletionOrphan)

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithFinalizer(resource.FinalizerFns{RemoveFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{Requeue: false}},
		},
		"InitializeError": {
			reason: "Errors initializing the managed resource should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: legacyManagedMockGetFn(nil, 42),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							want.SetConditions(xpv1.ReconcileError(errBoom).WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Errors initializing the managed resource should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(InitializerFn(func(_ context.Context, _ resource.Managed) error {
						return errBoom
					})),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"ExtraFinalizersDelayDelete": {
			reason: "The existence of multiple finalizers should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							mg := asLegacyManaged(obj, 42)
							mg.SetDeletionTimestamp(&now)
							mg.SetDeletionPolicy(xpv1.DeletionDelete)
							mg.SetFinalizers([]string{FinalizerName, "finalizer2"})

							return nil
						}),
						MockUpdate: test.NewMockUpdateFn(nil),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnector(ExternalConnectorFn(func(_ context.Context, _ resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: true}, nil
							},
							DisconnectFn: func(_ context.Context) error {
								return nil
							},
						}

						return c, nil
					})),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"ExternalCreatePending": {
			reason: "We should return early if the managed resource appears to be pending creation. We might have leaked a resource and don't want to create another.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							asLegacyManaged(obj, 42)
							meta.SetExternalCreatePending(obj, now.Time)

							return nil
						}),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							meta.SetExternalCreatePending(want, now.Time)
							want.SetConditions(
								xpv1.Creating().WithObservedGeneration(42),
								xpv1.ReconcileError(errors.New(errCreateIncomplete)).WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "We should update our status when we're asked to reconcile a managed resource that is pending creation."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(InitializerFn(func(_ context.Context, _ resource.Managed) error { return nil })),
				},
			},
			want: want{result: reconcile.Result{Requeue: false}},
		},
		"ResolveReferencesError": {
			reason: "Errors during reference resolution references should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: legacyManagedMockGetFn(nil, 42),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							want.SetConditions(xpv1.ReconcileError(errBoom).WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Errors during reference resolution should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error {
						return errBoom
					})),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"ExternalConnectError": {
			reason: "Errors connecting to the provider should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: legacyManagedMockGetFn(nil, 42),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, got client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							want.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errReconcileConnect)).WithObservedGeneration(42))

							if diff := cmp.Diff(want, got, test.EquateConditions()); diff != "" {
								reason := "Errors connecting to the provider should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithExternalConnector(ExternalConnectorFn(func(_ context.Context, _ resource.Managed) (ExternalClient, error) {
						return nil, errBoom
					})),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"ExternalDisconnectError": {
			reason: "Error disconnecting from the provider should not trigger requeue.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: legacyManagedMockGetFn(nil, 42),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							want.SetConditions(xpv1.ReconcileSuccess().WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "A successful no-op reconcile should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnector(ExternalConnectorFn(func(_ context.Context, _ resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, nil
							},
							DisconnectFn: func(_ context.Context) error {
								return errBoom
							},
						}

						return c, nil
					})),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultPollInterval}},
		},
		"ExternalObserveError": {
			reason: "Errors observing the external resource should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: legacyManagedMockGetFn(nil, 42),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							want.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errReconcileObserve)).WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Errors observing the managed resource should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithExternalConnector(ExternalConnectorFn(func(_ context.Context, _ resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{}, errBoom
							},
							DisconnectFn: func(_ context.Context) error {
								return nil
							},
						}

						return c, nil
					})),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"CreationGracePeriod": {
			reason: "If our resource appears not to exist during the creation grace period we should return early.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							meta.SetExternalCreateSucceeded(obj, time.Now())
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithCreationGracePeriod(1 * time.Minute),
					WithExternalConnector(ExternalConnectorFn(func(_ context.Context, _ resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: false}, nil
							},
							DisconnectFn: func(_ context.Context) error {
								return nil
							},
						}

						return c, nil
					})),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"ExternalDeleteError": {
			reason: "Errors deleting the external resource should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							mg := asLegacyManaged(obj, 42)
							mg.SetDeletionTimestamp(&now)

							return nil
						}),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							want.SetDeletionTimestamp(&now)
							want.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errReconcileDelete)).WithObservedGeneration(42))
							want.SetConditions(xpv1.Deleting().WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "An error deleting an external resource should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnector(ExternalConnectorFn(func(_ context.Context, _ resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: true}, nil
							},
							DeleteFn: func(_ context.Context, _ resource.Managed) (ExternalDelete, error) {
								return ExternalDelete{}, errBoom
							},
							DisconnectFn: func(_ context.Context) error {
								return nil
							},
						}

						return c, nil
					})),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"ExternalDeleteSuccessful": {
			reason: "A deleted managed resource with the 'delete' reclaim policy should delete its external resource then requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							mg := asLegacyManaged(obj, 42)
							mg.SetDeletionTimestamp(&now)

							return nil
						}),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							want.SetDeletionTimestamp(&now)
							want.SetConditions(xpv1.ReconcileSuccess().WithObservedGeneration(42))
							want.SetConditions(xpv1.Deleting().WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "A deleted external resource should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnector(ExternalConnectorFn(func(_ context.Context, _ resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: true}, nil
							},
							DeleteFn: func(_ context.Context, _ resource.Managed) (ExternalDelete, error) {
								return ExternalDelete{}, nil
							},
							DisconnectFn: func(_ context.Context) error {
								return nil
							},
						}

						return c, nil
					})),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"UnpublishConnectionDetailsDeletionPolicyDeleteError": {
			reason: "Errors unpublishing connection details should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							mg := asLegacyManaged(obj, 42)
							mg.SetDeletionTimestamp(&now)

							return nil
						}),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							want.SetDeletionTimestamp(&now)
							want.SetConditions(xpv1.Deleting().WithObservedGeneration(42))
							want.SetConditions(xpv1.ReconcileError(errBoom).WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Errors unpublishing connection details should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnector(ExternalConnectorFn(func(_ context.Context, _ resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: false}, nil
							},
							DisconnectFn: func(_ context.Context) error {
								return nil
							},
						}

						return c, nil
					})),
					withConnectionPublishers(ConnectionPublisherFns{
						UnpublishConnectionFn: func(_ context.Context, _ resource.ConnectionSecretOwner, _ ConnectionDetails) error { return errBoom },
					}),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"RemoveFinalizerErrorDeletionPolicyDelete": {
			reason: "Errors removing the managed resource finalizer should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							mg := asLegacyManaged(obj, 42)
							mg.SetDeletionTimestamp(&now)

							return nil
						}),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							want.SetDeletionTimestamp(&now)
							want.SetConditions(xpv1.Deleting().WithObservedGeneration(42))
							want.SetConditions(xpv1.ReconcileError(errBoom).WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Errors removing the managed resource finalizer should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnector(ExternalConnectorFn(func(_ context.Context, _ resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: false}, nil
							},
							DisconnectFn: func(_ context.Context) error {
								return nil
							},
						}

						return c, nil
					})),
					WithFinalizer(resource.FinalizerFns{RemoveFinalizerFn: func(_ context.Context, _ resource.Object) error { return errBoom }}),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"DeleteSuccessfulDeletionPolicyDelete": {
			reason: "Successful managed resource deletion with deletion policy Delete should not trigger a requeue or status update.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							mg := asLegacyManaged(obj, 42)
							mg.SetDeletionTimestamp(&now)

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnector(ExternalConnectorFn(func(_ context.Context, _ resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: false}, nil
							},
							DisconnectFn: func(_ context.Context) error {
								return nil
							},
						}

						return c, nil
					})),
					WithFinalizer(resource.FinalizerFns{RemoveFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{Requeue: false}},
		},
		"PublishObservationConnectionDetailsError": {
			reason: "Errors publishing connection details after observation should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: legacyManagedMockGetFn(nil, 42),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							want.SetConditions(xpv1.ReconcileError(errBoom).WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Errors publishing connection details after observation should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnector(&NopConnector{}),
					withConnectionPublishers(ConnectionPublisherFns{
						PublishConnectionFn: func(_ context.Context, _ resource.ConnectionSecretOwner, _ ConnectionDetails) (bool, error) {
							return false, errBoom
						},
					}),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"AddFinalizerError": {
			reason: "Errors adding a finalizer should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: legacyManagedMockGetFn(nil, 42),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							want.SetConditions(xpv1.ReconcileError(errBoom).WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Errors adding a finalizer should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnector(&NopConnector{}),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return errBoom }}),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"UpdateCreatePendingError": {
			reason: "Errors while updating our external-create-pending annotation should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet:    legacyManagedMockGetFn(nil, 42),
						MockUpdate: test.NewMockUpdateFn(errBoom),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							meta.SetExternalCreatePending(want, time.Now())
							want.SetConditions(
								xpv1.Creating().WithObservedGeneration(42),
								xpv1.ReconcileError(errors.Wrap(errBoom, errUpdateManaged)).WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions(), cmpopts.EquateApproxTime(1*time.Second)); diff != "" {
								reason := "Errors while creating an external resource should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnector(ExternalConnectorFn(func(_ context.Context, _ resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: false}, nil
							},
							CreateFn: func(_ context.Context, _ resource.Managed) (ExternalCreation, error) {
								return ExternalCreation{}, errBoom
							},
							DisconnectFn: func(_ context.Context) error {
								return nil
							},
						}

						return c, nil
					})),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"CreateExternalError": {
			reason: "Errors while creating an external resource should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet:    legacyManagedMockGetFn(nil, 42),
						MockUpdate: test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							meta.SetExternalCreatePending(want, time.Now())
							meta.SetExternalCreateFailed(want, time.Now())
							want.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errReconcileCreate)).WithObservedGeneration(42))
							want.SetConditions(xpv1.Creating().WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions(), cmpopts.EquateApproxTime(1*time.Second)); diff != "" {
								reason := "Errors while creating an external resource should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnector(ExternalConnectorFn(func(_ context.Context, _ resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: false}, nil
							},
							CreateFn: func(_ context.Context, _ resource.Managed) (ExternalCreation, error) {
								return ExternalCreation{}, errBoom
							},
							DisconnectFn: func(_ context.Context) error {
								return nil
							},
						}

						return c, nil
					})),
					// We simulate our critical annotation update failing too here.
					// This is mostly just to exercise the code, which just creates a log and an event.
					WithCriticalAnnotationUpdater(CriticalAnnotationUpdateFn(func(_ context.Context, _ client.Object) error { return errBoom })),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"UpdateCriticalAnnotationsError": {
			reason: "Errors updating critical annotations after creation should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet:    legacyManagedMockGetFn(nil, 42),
						MockUpdate: test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							meta.SetExternalCreatePending(want, time.Now())
							meta.SetExternalCreateSucceeded(want, time.Now())
							want.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errUpdateManagedAnnotations)).WithObservedGeneration(42))
							want.SetConditions(xpv1.Creating().WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions(), cmpopts.EquateApproxTime(1*time.Second)); diff != "" {
								reason := "Errors updating critical annotations after creation should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnector(ExternalConnectorFn(func(_ context.Context, _ resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: false}, nil
							},
							CreateFn: func(_ context.Context, _ resource.Managed) (ExternalCreation, error) {
								return ExternalCreation{}, nil
							},
							DisconnectFn: func(_ context.Context) error {
								return nil
							},
						}

						return c, nil
					})),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
					WithCriticalAnnotationUpdater(CriticalAnnotationUpdateFn(func(_ context.Context, _ client.Object) error { return errBoom })),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"PublishCreationConnectionDetailsError": {
			reason: "Errors publishing connection details after creation should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet:    legacyManagedMockGetFn(nil, 42),
						MockUpdate: test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							meta.SetExternalCreatePending(want, time.Now())
							meta.SetExternalCreateSucceeded(want, time.Now())
							want.SetConditions(xpv1.ReconcileError(errBoom).WithObservedGeneration(42))
							want.SetConditions(xpv1.Creating().WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions(), cmpopts.EquateApproxTime(1*time.Second)); diff != "" {
								reason := "Errors publishing connection details after creation should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnector(ExternalConnectorFn(func(_ context.Context, _ resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: false}, nil
							},
							CreateFn: func(_ context.Context, _ resource.Managed) (ExternalCreation, error) {
								cd := ConnectionDetails{"create": []byte{}}
								return ExternalCreation{ConnectionDetails: cd}, nil
							},
							DisconnectFn: func(_ context.Context) error {
								return nil
							},
						}

						return c, nil
					})),
					WithCriticalAnnotationUpdater(CriticalAnnotationUpdateFn(func(_ context.Context, _ client.Object) error { return nil })),
					withConnectionPublishers(ConnectionPublisherFns{
						PublishConnectionFn: func(_ context.Context, _ resource.ConnectionSecretOwner, cd ConnectionDetails) (bool, error) {
							// We're called after observe, create, and update
							// but we only want to fail when publishing details
							// after a creation.
							if _, ok := cd["create"]; ok {
								return false, errBoom
							}

							return true, nil
						},
					}),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"CreateSuccessful": {
			reason: "Successful managed resource creation should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet:    legacyManagedMockGetFn(nil, 42),
						MockUpdate: test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							meta.SetExternalCreatePending(want, time.Now())
							meta.SetExternalCreateSucceeded(want, time.Now())
							want.SetConditions(xpv1.ReconcileSuccess().WithObservedGeneration(42))
							want.SetConditions(xpv1.Creating().WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions(), cmpopts.EquateApproxTime(1*time.Second)); diff != "" {
								reason := "Successful managed resource creation should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnector(&NopConnector{}),
					WithCriticalAnnotationUpdater(CriticalAnnotationUpdateFn(func(_ context.Context, _ client.Object) error { return nil })),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"CreateSuccessfulAfterExternalCreatePendingAndDeterministicName": {
			reason: "Successful managed resource creation which was previously pending and has a deterministic external name should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							asLegacyManaged(obj, 42)
							meta.SetExternalCreatePending(obj, now.Time)

							return nil
						}),
						MockUpdate: test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							meta.SetExternalCreatePending(want, time.Now())
							meta.SetExternalCreateSucceeded(want, time.Now())
							want.SetConditions(xpv1.ReconcileSuccess().WithObservedGeneration(42))
							want.SetConditions(xpv1.Creating().WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions(), cmpopts.EquateApproxTime(1*time.Second)); diff != "" {
								reason := "Successful managed resource creation should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnector(&NopConnector{}),
					WithCriticalAnnotationUpdater(CriticalAnnotationUpdateFn(func(_ context.Context, _ client.Object) error { return nil })),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
					WithDeterministicExternalName(true),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"LateInitializeUpdateError": {
			reason: "Errors updating a managed resource to persist late initialized fields should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet:    legacyManagedMockGetFn(nil, 42),
						MockUpdate: test.NewMockUpdateFn(errBoom),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							want.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errUpdateManaged)).WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Errors updating a managed resource should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnector(ExternalConnectorFn(func(_ context.Context, _ resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: true, ResourceUpToDate: true, ResourceLateInitialized: true}, nil
							},
							DisconnectFn: func(_ context.Context) error {
								return nil
							},
						}

						return c, nil
					})),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"ExternalResourceUpToDate": {
			reason: "When the external resource exists and is up to date a requeue should be triggered after a long wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: legacyManagedMockGetFn(nil, 42),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							want.SetConditions(xpv1.ReconcileSuccess().WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "A successful no-op reconcile should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnector(ExternalConnectorFn(func(_ context.Context, _ resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, nil
							},
							DisconnectFn: func(_ context.Context) error {
								return nil
							},
						}

						return c, nil
					})),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultPollInterval}},
		},
		"ExternalResourceUpToDateWithJitter": {
			reason: "When the external resource exists and is up to date a requeue should be triggered after a long wait with jitter added.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: legacyManagedMockGetFn(nil, 42),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, _ client.Object, _ ...client.SubResourceUpdateOption) error {
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnector(ExternalConnectorFn(func(_ context.Context, _ resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, nil
							},
							DisconnectFn: func(_ context.Context) error {
								return nil
							},
						}

						return c, nil
					})),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
					WithPollJitterHook(time.Second),
				},
			},
			want: want{
				result: reconcile.Result{RequeueAfter: defaultPollInterval},
				resultCmpOpts: []cmp.Option{cmp.Comparer(func(l, r time.Duration) bool {
					diff := l - r
					if diff < 0 {
						diff = -diff
					}

					return diff < time.Second
				})},
			},
		},
		"ExternalResourceUpToDateWithPollIntervalHook": {
			reason: "When the external resource exists and is up to date a requeue should be triggered after a long wait processed by the interval hook.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: legacyManagedMockGetFn(nil, 42),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, _ client.Object, _ ...client.SubResourceUpdateOption) error {
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnector(ExternalConnectorFn(func(_ context.Context, _ resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, nil
							},
							DisconnectFn: func(_ context.Context) error {
								return nil
							},
						}

						return c, nil
					})),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
					WithPollIntervalHook(func(_ resource.Managed, pollInterval time.Duration) time.Duration {
						return 2 * pollInterval
					}),
				},
			},
			want: want{
				result: reconcile.Result{RequeueAfter: 2 * defaultPollInterval},
			},
		},
		"ExternalResourceUpToDateWithMultiplePollIntervalHooks": {
			reason: "When the external resource exists and is up to date a requeue should be triggered after a long wait processed by the latest interval hook.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: legacyManagedMockGetFn(nil, 42),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, _ client.Object, _ ...client.SubResourceUpdateOption) error {
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnector(ExternalConnectorFn(func(_ context.Context, _ resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, nil
							},
							DisconnectFn: func(_ context.Context) error {
								return nil
							},
						}

						return c, nil
					})),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
					WithPollJitterHook(time.Second),
					WithPollIntervalHook(func(_ resource.Managed, pollInterval time.Duration) time.Duration {
						return 2 * pollInterval
					}),
					WithPollIntervalHook(func(_ resource.Managed, pollInterval time.Duration) time.Duration {
						return 3 * pollInterval
					}),
				},
			},
			want: want{
				result: reconcile.Result{RequeueAfter: 3 * defaultPollInterval},
			},
		},
		"UpdateExternalError": {
			reason: "Errors while updating an external resource should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: legacyManagedMockGetFn(nil, 42),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							want.SetConditions(xpv1.ReconcileError(errors.Wrap(errBoom, errReconcileUpdate)).WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Errors while updating an external resource should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnector(ExternalConnectorFn(func(_ context.Context, _ resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: true, ResourceUpToDate: false}, nil
							},
							UpdateFn: func(_ context.Context, _ resource.Managed) (ExternalUpdate, error) {
								return ExternalUpdate{}, errBoom
							},
							DisconnectFn: func(_ context.Context) error {
								return nil
							},
						}

						return c, nil
					})),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"PublishUpdateConnectionDetailsError": {
			reason: "Errors publishing connection details after an update should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: legacyManagedMockGetFn(nil, 42),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							want.SetConditions(xpv1.ReconcileError(errBoom).WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Errors publishing connection details after an update should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnector(ExternalConnectorFn(func(_ context.Context, _ resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: true, ResourceUpToDate: false}, nil
							},
							UpdateFn: func(_ context.Context, _ resource.Managed) (ExternalUpdate, error) {
								cd := ConnectionDetails{"update": []byte{}}
								return ExternalUpdate{ConnectionDetails: cd}, nil
							},
							DisconnectFn: func(_ context.Context) error {
								return nil
							},
						}

						return c, nil
					})),
					withConnectionPublishers(ConnectionPublisherFns{
						PublishConnectionFn: func(_ context.Context, _ resource.ConnectionSecretOwner, cd ConnectionDetails) (bool, error) {
							// We're called after observe, create, and update
							// but we only want to fail when publishing details
							// after an update.
							if _, ok := cd["update"]; ok {
								return false, errBoom
							}

							return false, nil
						},
					}),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"UpdateSuccessful": {
			reason: "A successful managed resource update should trigger a requeue after a long wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: legacyManagedMockGetFn(nil, 42),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							want.SetConditions(xpv1.ReconcileSuccess().WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "A successful managed resource update should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnector(ExternalConnectorFn(func(_ context.Context, _ resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: true, ResourceUpToDate: false}, nil
							},
							UpdateFn: func(_ context.Context, _ resource.Managed) (ExternalUpdate, error) {
								return ExternalUpdate{}, nil
							},
							DisconnectFn: func(_ context.Context) error {
								return nil
							},
						}

						return c, nil
					})),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultPollInterval}},
		},
		"TypedReconcilerUpdateSuccessful": {
			reason: "A successful managed resource update should trigger a requeue after a long wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: legacyManagedMockGetFn(nil, 42),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							want.SetConditions(xpv1.ReconcileSuccess().WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "A successful managed resource update should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithTypedExternalConnector(TypedExternalConnectorFn[*fake.LegacyManaged](func(_ context.Context, _ *fake.LegacyManaged) (TypedExternalClient[*fake.LegacyManaged], error) {
						c := &TypedExternalClientFns[*fake.LegacyManaged]{
							ObserveFn: func(_ context.Context, _ *fake.LegacyManaged) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: true, ResourceUpToDate: false}, nil
							},
							UpdateFn: func(_ context.Context, _ *fake.LegacyManaged) (ExternalUpdate, error) {
								return ExternalUpdate{}, nil
							},
							DisconnectFn: func(_ context.Context) error {
								return nil
							},
						}

						return c, nil
					})),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
				},
			},
			want: want{
				result: reconcile.Result{RequeueAfter: defaultPollInterval},
			},
		},
		"ReconciliationPausedSuccessful": {
			reason: `If a managed resource has the pause annotation with value "true", there should be no further requeue requests.`,
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							mg := asLegacyManaged(obj, 42)
							mg.SetAnnotations(map[string]string{meta.AnnotationKeyReconciliationPaused: "true"})

							return nil
						}),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							want.SetAnnotations(map[string]string{meta.AnnotationKeyReconciliationPaused: "true"})
							want.SetConditions(xpv1.ReconcilePaused().WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := `If managed resource has the pause annotation with value "true", it should acquire "Synced" status condition with the status "False" and the reason "ReconcilePaused".`
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
			},
			want: want{result: reconcile.Result{}},
		},
		"ManagementPolicyReconciliationPausedSuccessful": {
			reason: `If a managed resource has the pause annotation with value "true", there should be no further requeue requests.`,
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							mg := asLegacyManaged(obj, 42)
							mg.SetManagementPolicies(xpv1.ManagementPolicies{})

							return nil
						}),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							want.SetManagementPolicies(xpv1.ManagementPolicies{})
							want.SetConditions(xpv1.ReconcilePaused().WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := `If managed resource has the pause annotation with value "true", it should acquire "Synced" status condition with the status "False" and the reason "ReconcilePaused".`
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithManagementPolicies(),
					WithInitializers(),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{}},
		},
		"ReconciliationResumes": {
			reason: `If a managed resource has the pause annotation with some value other than "true" and the Synced=False/ReconcilePaused status condition, reconciliation should resume with requeueing.`,
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							mg := asLegacyManaged(obj, 42)
							mg.SetAnnotations(map[string]string{meta.AnnotationKeyReconciliationPaused: "false"})
							mg.SetConditions(xpv1.ReconcilePaused())

							return nil
						}),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							want.SetAnnotations(map[string]string{meta.AnnotationKeyReconciliationPaused: "false"})
							want.SetConditions(xpv1.ReconcileSuccess().WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := `Managed resource should acquire Synced=False/ReconcileSuccess status condition after a resume.`
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnector(ExternalConnectorFn(func(_ context.Context, _ resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, nil
							},
							DisconnectFn: func(_ context.Context) error {
								return nil
							},
						}

						return c, nil
					})),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultPollInterval}},
		},
		"ReconciliationPausedError": {
			reason: `If a managed resource has the pause annotation with value "true" and the status update due to reconciliation being paused fails, error should be reported causing an exponentially backed-off requeue.`,
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							mg := asLegacyManaged(obj, 42)
							mg.SetAnnotations(map[string]string{meta.AnnotationKeyReconciliationPaused: "true"})

							return nil
						}),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, _ client.Object, _ ...client.SubResourceUpdateOption) error {
							return errBoom
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
			},
			want: want{err: errors.Wrap(errBoom, errUpdateManagedStatus)},
		},
		"ManagementPoliciesUsedButNotEnabled": {
			reason: `If management policies tried to be used without enabling the feature, we should throw an error.`,
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							mg := asLegacyManaged(obj, 42)
							mg.SetManagementPolicies(xpv1.ManagementPolicies{xpv1.ManagementActionCreate})

							return nil
						}),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							want.SetManagementPolicies(xpv1.ManagementPolicies{xpv1.ManagementActionCreate})
							want.SetConditions(xpv1.ReconcileError(fmt.Errorf(errFmtManagementPolicyNonDefault, xpv1.ManagementPolicies{xpv1.ManagementActionCreate})).WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := `If managed resource has a non default management policy but feature not enabled, it should return a proper error.`
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
			},
			want: want{result: reconcile.Result{}},
		},
		"ManagementPolicyNotSupported": {
			reason: `If an unsupported management policy is used, we should throw an error.`,
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							mg := asLegacyManaged(obj, 42)
							mg.SetManagementPolicies(xpv1.ManagementPolicies{xpv1.ManagementActionCreate})

							return nil
						}),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							want.SetManagementPolicies(xpv1.ManagementPolicies{xpv1.ManagementActionCreate})
							want.SetConditions(xpv1.ReconcileError(fmt.Errorf(errFmtManagementPolicyNotSupported, xpv1.ManagementPolicies{xpv1.ManagementActionCreate})).WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := `If managed resource has non supported management policy, it should return a proper error.`
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithManagementPolicies(),
				},
			},
			want: want{result: reconcile.Result{}},
		},
		"CustomManagementPolicyNotSupported": {
			reason: `If a custom unsupported management policy is used, we should throw an error.`,
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							mg := asLegacyManaged(obj, 42)
							mg.SetManagementPolicies(xpv1.ManagementPolicies{xpv1.ManagementActionAll})

							return nil
						}),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							want.SetManagementPolicies(xpv1.ManagementPolicies{xpv1.ManagementActionAll})
							want.SetConditions(xpv1.ReconcileError(fmt.Errorf(errFmtManagementPolicyNotSupported, xpv1.ManagementPolicies{xpv1.ManagementActionAll})).WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := `If managed resource has non supported management policy, it should return a proper error.`
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithManagementPolicies(),
					WithReconcilerSupportedManagementPolicies([]sets.Set[xpv1.ManagementAction]{sets.New(xpv1.ManagementActionObserve)}),
				},
			},
			want: want{result: reconcile.Result{}},
		},
		"ObserveOnlyResourceDoesNotExist": {
			reason: "With only Observe management action, observing a resource that does not exist should be reported as a conditioned status error.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							mg := asLegacyManaged(obj, 42)
							mg.SetManagementPolicies(xpv1.ManagementPolicies{xpv1.ManagementActionObserve})

							return nil
						}),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							want.SetManagementPolicies(xpv1.ManagementPolicies{xpv1.ManagementActionObserve})
							want.SetConditions(xpv1.ReconcileError(errors.Wrap(errors.New(errExternalResourceNotExist), errReconcileObserve)).WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Resource does not exist should be reported as a conditioned status when ObserveOnly."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithManagementPolicies(),
					WithExternalConnector(ExternalConnectorFn(func(_ context.Context, _ resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: false}, nil
							},
							DisconnectFn: func(_ context.Context) error {
								return nil
							},
						}

						return c, nil
					})),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"ObserveOnlyPublishConnectionDetailsError": {
			reason: "With Observe, errors publishing connection details after observation should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							mg := asLegacyManaged(obj, 42)
							mg.SetManagementPolicies(xpv1.ManagementPolicies{xpv1.ManagementActionObserve})

							return nil
						}),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							want.SetManagementPolicies(xpv1.ManagementPolicies{xpv1.ManagementActionObserve})
							want.SetConditions(xpv1.ReconcileError(errBoom).WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Errors publishing connection details after observation should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithManagementPolicies(),
					WithExternalConnector(ExternalConnectorFn(func(_ context.Context, _ resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: true}, nil
							},
							DisconnectFn: func(_ context.Context) error {
								return nil
							},
						}

						return c, nil
					})),
					withConnectionPublishers(ConnectionPublisherFns{
						PublishConnectionFn: func(_ context.Context, _ resource.ConnectionSecretOwner, _ ConnectionDetails) (bool, error) {
							return false, errBoom
						},
					}),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"ObserveOnlySuccessfulObserve": {
			reason: "With Observe, a successful managed resource observe should trigger a requeue after a long wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							mg := asLegacyManaged(obj, 42)
							mg.SetManagementPolicies(xpv1.ManagementPolicies{xpv1.ManagementActionObserve})

							return nil
						}),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							want.SetManagementPolicies(xpv1.ManagementPolicies{xpv1.ManagementActionObserve})
							want.SetConditions(xpv1.ReconcileSuccess().WithObservedGeneration(42).WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "With ObserveOnly, a successful managed resource observation should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithManagementPolicies(),
					WithExternalConnector(ExternalConnectorFn(func(_ context.Context, _ resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: true}, nil
							},
							DisconnectFn: func(_ context.Context) error {
								return nil
							},
						}

						return c, nil
					})),
					withConnectionPublishers(ConnectionPublisherFns{
						PublishConnectionFn: func(_ context.Context, _ resource.ConnectionSecretOwner, _ ConnectionDetails) (bool, error) {
							return false, nil
						},
					}),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultPollInterval}},
		},
		"ManagementPolicyAllCreateSuccessful": {
			reason: "Successful managed resource creation using management policy all should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							mg := asLegacyManaged(obj, 42)
							mg.SetManagementPolicies(xpv1.ManagementPolicies{xpv1.ManagementActionAll})

							return nil
						}),
						MockUpdate: test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							want.SetManagementPolicies(xpv1.ManagementPolicies{xpv1.ManagementActionAll})
							meta.SetExternalCreatePending(want, time.Now())
							meta.SetExternalCreateSucceeded(want, time.Now())
							want.SetConditions(xpv1.ReconcileSuccess().WithObservedGeneration(42))
							want.SetConditions(xpv1.Creating().WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions(), cmpopts.EquateApproxTime(1*time.Second)); diff != "" {
								reason := "Successful managed resource creation should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithManagementPolicies(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnector(&NopConnector{}),
					WithCriticalAnnotationUpdater(CriticalAnnotationUpdateFn(func(_ context.Context, _ client.Object) error { return nil })),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"ManagementPolicyCreateCreateSuccessful": {
			reason: "Successful managed resource creation using management policy Create should trigger a requeue after a short wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							mg := asLegacyManaged(obj, 42)
							mg.SetManagementPolicies(xpv1.ManagementPolicies{xpv1.ManagementActionAll})

							return nil
						}),
						MockUpdate: test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							want.SetManagementPolicies(xpv1.ManagementPolicies{xpv1.ManagementActionAll})
							meta.SetExternalCreatePending(want, time.Now())
							meta.SetExternalCreateSucceeded(want, time.Now())
							want.SetConditions(xpv1.ReconcileSuccess().WithObservedGeneration(42))
							want.SetConditions(xpv1.Creating().WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions(), cmpopts.EquateApproxTime(1*time.Second)); diff != "" {
								reason := "Successful managed resource creation should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithManagementPolicies(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnector(&NopConnector{}),
					WithCriticalAnnotationUpdater(CriticalAnnotationUpdateFn(func(_ context.Context, _ client.Object) error { return nil })),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{Requeue: true}},
		},
		"ManagementPolicyImmutable": {
			reason: "Successful reconciliation skipping update should trigger a requeue after a long wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							mg := asLegacyManaged(obj, 42)
							mg.SetManagementPolicies(xpv1.ManagementPolicies{xpv1.ManagementActionObserve, xpv1.ManagementActionLateInitialize, xpv1.ManagementActionCreate, xpv1.ManagementActionDelete})

							return nil
						}),
						MockUpdate: test.NewMockUpdateFn(errBoom),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							want.SetManagementPolicies(xpv1.ManagementPolicies{xpv1.ManagementActionObserve, xpv1.ManagementActionLateInitialize, xpv1.ManagementActionCreate, xpv1.ManagementActionDelete})
							want.SetConditions(xpv1.ReconcileSuccess().WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := `Managed resource should acquire Synced=False/ReconcileSuccess status condition.`
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithManagementPolicies(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnector(ExternalConnectorFn(func(_ context.Context, _ resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: true, ResourceUpToDate: false}, nil
							},
							UpdateFn: func(_ context.Context, _ resource.Managed) (ExternalUpdate, error) {
								return ExternalUpdate{}, errBoom
							},
							DisconnectFn: func(_ context.Context) error {
								return nil
							},
						}

						return c, nil
					})),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultPollInterval}},
		},
		"ManagementPolicyAllUpdateSuccessful": {
			reason: "A successful managed resource update using management policies should trigger a requeue after a long wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							mg := asLegacyManaged(obj, 42)
							mg.SetManagementPolicies(xpv1.ManagementPolicies{xpv1.ManagementActionAll})

							return nil
						}),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							want.SetManagementPolicies(xpv1.ManagementPolicies{xpv1.ManagementActionAll})
							want.SetConditions(xpv1.ReconcileSuccess().WithObservedGeneration(42).WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "A successful managed resource update should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithManagementPolicies(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnector(ExternalConnectorFn(func(_ context.Context, _ resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: true, ResourceUpToDate: false}, nil
							},
							UpdateFn: func(_ context.Context, _ resource.Managed) (ExternalUpdate, error) {
								return ExternalUpdate{}, nil
							},
							DisconnectFn: func(_ context.Context) error {
								return nil
							},
						}

						return c, nil
					})),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultPollInterval}},
		},
		"ManagementPolicyUpdateUpdateSuccessful": {
			reason: "A successful managed resource update using management policies should trigger a requeue after a long wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							mg := asLegacyManaged(obj, 42)
							mg.SetManagementPolicies(xpv1.ManagementPolicies{xpv1.ManagementActionAll})

							return nil
						}),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							want.SetManagementPolicies(xpv1.ManagementPolicies{xpv1.ManagementActionAll})
							want.SetConditions(xpv1.ReconcileSuccess().WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "A successful managed resource update should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithManagementPolicies(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnector(ExternalConnectorFn(func(_ context.Context, _ resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: true, ResourceUpToDate: false}, nil
							},
							UpdateFn: func(_ context.Context, _ resource.Managed) (ExternalUpdate, error) {
								return ExternalUpdate{}, nil
							},
							DisconnectFn: func(_ context.Context) error {
								return nil
							},
						}

						return c, nil
					})),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultPollInterval}},
		},
		"ManagementPolicySkipLateInitialize": {
			reason: "Should skip updating a managed resource to persist late initialized fields and should trigger a requeue after a long wait.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							mg := asLegacyManaged(obj, 42)
							mg.SetManagementPolicies(xpv1.ManagementPolicies{xpv1.ManagementActionObserve, xpv1.ManagementActionUpdate, xpv1.ManagementActionCreate, xpv1.ManagementActionDelete})

							return nil
						}),
						MockUpdate: test.NewMockUpdateFn(errBoom),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
							want := newLegacyManaged(42)
							want.SetManagementPolicies(xpv1.ManagementPolicies{xpv1.ManagementActionObserve, xpv1.ManagementActionUpdate, xpv1.ManagementActionCreate, xpv1.ManagementActionDelete})
							want.SetConditions(xpv1.ReconcileSuccess().WithObservedGeneration(42))

							if diff := cmp.Diff(want, obj, test.EquateConditions()); diff != "" {
								reason := "Errors updating a managed resource should be reported as a conditioned status."
								t.Errorf("\nReason: %s\n-want, +got:\n%s", reason, diff)
							}

							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithInitializers(),
					WithManagementPolicies(),
					WithReferenceResolver(ReferenceResolverFn(func(_ context.Context, _ resource.Managed) error { return nil })),
					WithExternalConnector(ExternalConnectorFn(func(_ context.Context, _ resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								return ExternalObservation{ResourceExists: true, ResourceUpToDate: true, ResourceLateInitialized: true}, nil
							},
							DisconnectFn: func(_ context.Context) error {
								return nil
							},
						}

						return c, nil
					})),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error { return nil }}),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultPollInterval}},
		},
		"ObserveAndLateInitializePolicy": {
			reason: "If management policy is set to Observe and LateInitialize, reconciliation should proceed",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							mg := asLegacyManaged(obj, 42)
							mg.SetManagementPolicies(xpv1.ManagementPolicies{xpv1.ManagementActionObserve, xpv1.ManagementActionLateInitialize})

							return nil
						}),
						MockUpdate: test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, _ client.Object, _ ...client.SubResourceUpdateOption) error {
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithManagementPolicies(),
					WithReconcilerSupportedManagementPolicies(defaultSupportedManagementPolicies()),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultPollInterval}},
		},
		"ObserveUpdateAndLateInitializePolicy": {
			reason: "If management policy is set to Observe, Update and LateInitialize, reconciliation should proceed",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							mg := asLegacyManaged(obj, 42)
							mg.SetManagementPolicies(xpv1.ManagementPolicies{
								xpv1.ManagementActionObserve,
								xpv1.ManagementActionUpdate,
								xpv1.ManagementActionLateInitialize,
							})

							return nil
						}),
						MockUpdate: test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, _ client.Object, _ ...client.SubResourceUpdateOption) error {
							return nil
						}),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithManagementPolicies(),
					WithReconcilerSupportedManagementPolicies(defaultSupportedManagementPolicies()),
				},
			},
			want: want{result: reconcile.Result{RequeueAfter: defaultPollInterval}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewReconciler(tc.args.m, tc.args.mg, tc.args.o...)

			got, err := r.Reconcile(context.Background(), reconcile.Request{})
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\nReason: %s\nr.Reconcile(...): -want error, +got error:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.result, got, tc.want.resultCmpOpts...); diff != "" {
				t.Errorf("\nReason: %s\nr.Reconcile(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestTestLegacyManagementPoliciesResolverIsPaused(t *testing.T) {
	type args struct {
		enabled bool
		policy  xpv1.ManagementPolicies
	}

	cases := map[string]struct {
		reason string
		args   args
		want   bool
	}{
		"Disabled": {
			reason: "Should return false if management policies are disabled",
			args: args{
				enabled: false,
				policy:  xpv1.ManagementPolicies{},
			},
			want: false,
		},
		"EnabledEmptyPolicies": {
			reason: "Should return true if the management policies are enabled and empty",
			args: args{
				enabled: true,
				policy:  xpv1.ManagementPolicies{},
			},
			want: true,
		},
		"EnabledNonEmptyPolicies": {
			reason: "Should return true if the management policies are enabled and non empty",
			args: args{
				enabled: true,
				policy:  xpv1.ManagementPolicies{xpv1.ManagementActionAll},
			},
			want: false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewLegacyManagementPoliciesResolver(tc.args.enabled, tc.args.policy, xpv1.DeletionDelete)
			if diff := cmp.Diff(tc.want, r.IsPaused()); diff != "" {
				t.Errorf("\nReason: %s\nIsPaused(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestLegacyManagementPoliciesResolverValidate(t *testing.T) {
	type args struct {
		enabled bool
		policy  xpv1.ManagementPolicies
	}

	cases := map[string]struct {
		reason string
		args   args
		want   error
	}{
		"Enabled": {
			reason: "Should return nil if the management policy is enabled.",
			args: args{
				enabled: true,
				policy:  xpv1.ManagementPolicies{},
			},
			want: nil,
		},
		"DisabledNonDefault": {
			reason: "Should return error if the management policy is non-default and disabled.",
			args: args{
				enabled: false,
				policy:  xpv1.ManagementPolicies{xpv1.ManagementActionCreate},
			},
			want: fmt.Errorf(errFmtManagementPolicyNonDefault, []xpv1.ManagementAction{xpv1.ManagementActionCreate}),
		},
		"DisabledDefault": {
			reason: "Should return nil if the management policy is default and disabled.",
			args: args{
				enabled: false,
				policy:  xpv1.ManagementPolicies{xpv1.ManagementActionAll},
			},
			want: nil,
		},
		"EnabledSupported": {
			reason: "Should return nil if the management policy is supported.",
			args: args{
				enabled: true,
				policy:  xpv1.ManagementPolicies{xpv1.ManagementActionAll},
			},
			want: nil,
		},
		"EnabledNotSupported": {
			reason: "Should return err if the management policy is not supported.",
			args: args{
				enabled: true,
				policy:  xpv1.ManagementPolicies{xpv1.ManagementActionDelete},
			},
			want: fmt.Errorf(errFmtManagementPolicyNotSupported, []xpv1.ManagementAction{xpv1.ManagementActionDelete}),
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewLegacyManagementPoliciesResolver(tc.args.enabled, tc.args.policy, xpv1.DeletionDelete)
			if diff := cmp.Diff(tc.want, r.Validate(), test.EquateErrors()); diff != "" {
				t.Errorf("\nReason: %s\nIsNonDefault(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestLegacyManagementPoliciesResolverShouldCreate(t *testing.T) {
	type args struct {
		managementPoliciesEnabled bool
		policy                    xpv1.ManagementPolicies
	}

	cases := map[string]struct {
		reason string
		args   args
		want   bool
	}{
		"ManagementPoliciesDisabled": {
			reason: "Should return true if management policies are disabled",
			args: args{
				managementPoliciesEnabled: false,
			},
			want: true,
		},
		"ManagementPoliciesEnabledHasCreate": {
			reason: "Should return true if management policies are enabled and managementPolicies has action Create",
			args: args{
				managementPoliciesEnabled: true,
				policy:                    xpv1.ManagementPolicies{xpv1.ManagementActionCreate},
			},
			want: true,
		},
		"ManagementPoliciesEnabledHasCreateAll": {
			reason: "Should return true if management policies are enabled and managementPolicies has action All",
			args: args{
				managementPoliciesEnabled: true,
				policy:                    xpv1.ManagementPolicies{xpv1.ManagementActionAll},
			},
			want: true,
		},
		"ManagementPoliciesEnabledActionNotAllowed": {
			reason: "Should return false if management policies are enabled and managementPolicies does not have Create",
			args: args{
				managementPoliciesEnabled: true,
				policy:                    xpv1.ManagementPolicies{xpv1.ManagementActionObserve},
			},
			want: false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewLegacyManagementPoliciesResolver(tc.args.managementPoliciesEnabled, tc.args.policy, xpv1.DeletionOrphan)
			if diff := cmp.Diff(tc.want, r.ShouldCreate()); diff != "" {
				t.Errorf("\nReason: %s\nShouldCreate(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestLegacyManagementPoliciesResolverShouldUpdate(t *testing.T) {
	type args struct {
		managementPoliciesEnabled bool
		policy                    xpv1.ManagementPolicies
	}

	cases := map[string]struct {
		reason string
		args   args
		want   bool
	}{
		"ManagementPoliciesDisabled": {
			reason: "Should return true if management policies are disabled",
			args: args{
				managementPoliciesEnabled: false,
			},
			want: true,
		},
		"ManagementPoliciesEnabledHasUpdate": {
			reason: "Should return true if management policies are enabled and managementPolicies has action Update",
			args: args{
				managementPoliciesEnabled: true,
				policy:                    xpv1.ManagementPolicies{xpv1.ManagementActionUpdate},
			},
			want: true,
		},
		"ManagementPoliciesEnabledHasUpdateAll": {
			reason: "Should return true if management policies are enabled and managementPolicies has action All",
			args: args{
				managementPoliciesEnabled: true,
				policy:                    xpv1.ManagementPolicies{xpv1.ManagementActionAll},
			},
			want: true,
		},
		"ManagementPoliciesEnabledActionNotAllowed": {
			reason: "Should return false if management policies are enabled and managementPolicies does not have Update",
			args: args{
				managementPoliciesEnabled: true,
				policy:                    xpv1.ManagementPolicies{xpv1.ManagementActionObserve},
			},
			want: false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewLegacyManagementPoliciesResolver(tc.args.managementPoliciesEnabled, tc.args.policy, xpv1.DeletionOrphan)
			if diff := cmp.Diff(tc.want, r.ShouldUpdate()); diff != "" {
				t.Errorf("\nReason: %s\nShouldUpdate(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestLegacyManagementPoliciesResolverShouldLateInitialize(t *testing.T) {
	type args struct {
		managementPoliciesEnabled bool
		policy                    xpv1.ManagementPolicies
	}

	cases := map[string]struct {
		reason string
		args   args
		want   bool
	}{
		"ManagementPoliciesDisabled": {
			reason: "Should return true if management policies are disabled",
			args: args{
				managementPoliciesEnabled: false,
			},
			want: true,
		},
		"ManagementPoliciesEnabledHasLateInitialize": {
			reason: "Should return true if management policies are enabled and managementPolicies has action LateInitialize",
			args: args{
				managementPoliciesEnabled: true,
				policy:                    xpv1.ManagementPolicies{xpv1.ManagementActionLateInitialize},
			},
			want: true,
		},
		"ManagementPoliciesEnabledHasLateInitializeAll": {
			reason: "Should return true if management policies are enabled and managementPolicies has action All",
			args: args{
				managementPoliciesEnabled: true,
				policy:                    xpv1.ManagementPolicies{xpv1.ManagementActionAll},
			},
			want: true,
		},
		"ManagementPoliciesEnabledActionNotAllowed": {
			reason: "Should return false if management policies are enabled and managementPolicies does not have LateInitialize",
			args: args{
				managementPoliciesEnabled: true,
				policy:                    xpv1.ManagementPolicies{xpv1.ManagementActionObserve},
			},
			want: false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewLegacyManagementPoliciesResolver(tc.args.managementPoliciesEnabled, tc.args.policy, xpv1.DeletionOrphan)
			if diff := cmp.Diff(tc.want, r.ShouldLateInitialize()); diff != "" {
				t.Errorf("\nReason: %s\nShouldLateInitialize(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestLegacyManagementPoliciesResolverOnlyObserve(t *testing.T) {
	type args struct {
		managementPoliciesEnabled bool
		policy                    xpv1.ManagementPolicies
	}

	cases := map[string]struct {
		reason string
		args   args
		want   bool
	}{
		"ManagementPoliciesDisabled": {
			reason: "Should return false if management policies are disabled",
			args: args{
				managementPoliciesEnabled: false,
			},
			want: false,
		},
		"ManagementPoliciesEnabledHasOnlyObserve": {
			reason: "Should return true if management policies are enabled and managementPolicies has action LateInitialize",
			args: args{
				managementPoliciesEnabled: true,
				policy:                    xpv1.ManagementPolicies{xpv1.ManagementActionObserve},
			},
			want: true,
		},
		"ManagementPoliciesEnabledHasMultipleActions": {
			reason: "Should return false if management policies are enabled and managementPolicies has multiple actions",
			args: args{
				managementPoliciesEnabled: true,
				policy:                    xpv1.ManagementPolicies{xpv1.ManagementActionLateInitialize, xpv1.ManagementActionObserve},
			},
			want: false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewLegacyManagementPoliciesResolver(tc.args.managementPoliciesEnabled, tc.args.policy, xpv1.DeletionOrphan)
			if diff := cmp.Diff(tc.want, r.ShouldOnlyObserve()); diff != "" {
				t.Errorf("\nReason: %s\nShouldOnlyObserve(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestLegacyShouldDelete(t *testing.T) {
	type args struct {
		managementPoliciesEnabled bool
		managed                   resource.LegacyManaged
	}

	type want struct {
		delete bool
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"DeletionOrphan": {
			reason: "Should orphan if management policies are disabled and deletion policy is set to Orphan.",
			args: args{
				managementPoliciesEnabled: false,
				managed: &fake.LegacyManaged{
					Orphanable: fake.Orphanable{
						Policy: xpv1.DeletionOrphan,
					},
				},
			},
			want: want{delete: false},
		},
		"DeletionDelete": {
			reason: "Should delete if management policies are disabled and deletion policy is set to Delete.",
			args: args{
				managementPoliciesEnabled: false,
				managed: &fake.LegacyManaged{
					Orphanable: fake.Orphanable{
						Policy: xpv1.DeletionDelete,
					},
				},
			},
			want: want{delete: true},
		},
		"DeletionDeleteManagementActionAll": {
			reason: "Should delete if management policies are enabled and deletion policy is set to Delete and management policy is set to All.",
			args: args{
				managementPoliciesEnabled: true,
				managed: &fake.LegacyManaged{
					Orphanable: fake.Orphanable{
						Policy: xpv1.DeletionDelete,
					},
					Manageable: fake.Manageable{
						Policy: xpv1.ManagementPolicies{xpv1.ManagementActionAll},
					},
				},
			},
			want: want{delete: true},
		},
		"DeletionOrphanManagementActionAll": {
			reason: "Should orphan if management policies are enabled and deletion policy is set to Orphan and management policy is set to All.",
			args: args{
				managementPoliciesEnabled: true,
				managed: &fake.LegacyManaged{
					Orphanable: fake.Orphanable{
						Policy: xpv1.DeletionOrphan,
					},
					Manageable: fake.Manageable{
						Policy: xpv1.ManagementPolicies{xpv1.ManagementActionAll},
					},
				},
			},
			want: want{delete: false},
		},
		"DeletionDeleteManagementActionDelete": {
			reason: "Should delete if management policies are enabled and deletion policy is set to Delete and management policy has action Delete.",
			args: args{
				managementPoliciesEnabled: true,
				managed: &fake.LegacyManaged{
					Orphanable: fake.Orphanable{
						Policy: xpv1.DeletionDelete,
					},
					Manageable: fake.Manageable{
						Policy: xpv1.ManagementPolicies{xpv1.ManagementActionDelete},
					},
				},
			},
			want: want{delete: true},
		},
		"DeletionOrphanManagementActionDelete": {
			reason: "Should delete if management policies are enabled and deletion policy is set to Orphan and management policy has action Delete.",
			args: args{
				managementPoliciesEnabled: true,
				managed: &fake.LegacyManaged{
					Orphanable: fake.Orphanable{
						Policy: xpv1.DeletionOrphan,
					},
					Manageable: fake.Manageable{
						Policy: xpv1.ManagementPolicies{xpv1.ManagementActionDelete},
					},
				},
			},
			want: want{delete: true},
		},
		"DeletionDeleteManagementActionNoDelete": {
			reason: "Should orphan if management policies are enabled and deletion policy is set to Delete and management policy does not have action Delete.",
			args: args{
				managementPoliciesEnabled: true,
				managed: &fake.LegacyManaged{
					Orphanable: fake.Orphanable{
						Policy: xpv1.DeletionDelete,
					},
					Manageable: fake.Manageable{
						Policy: xpv1.ManagementPolicies{xpv1.ManagementActionObserve},
					},
				},
			},
			want: want{delete: false},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewLegacyManagementPoliciesResolver(tc.args.managementPoliciesEnabled, tc.args.managed.GetManagementPolicies(), tc.args.managed.GetDeletionPolicy())
			if diff := cmp.Diff(tc.want.delete, r.ShouldDelete()); diff != "" {
				t.Errorf("\nReason: %s\nShouldDelete(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestLegacyReconcilerChangeLogs(t *testing.T) {
	type args struct {
		m  manager.Manager
		mg resource.ManagedKind
		o  []ReconcilerOption
		c  *changeLogServiceClient
	}

	type want struct {
		callCount  int
		opType     v1alpha1.OperationType
		errMessage string
	}

	now := metav1.Now()
	errBoom := errors.New("boom")

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"CreateSuccessfulWithChangeLogs": {
			reason: "Successful managed resource creation should send a create change log entry when change logs are enabled.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet:          legacyManagedMockGetFn(nil, 42),
						MockUpdate:       test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, _ client.Object, _ ...client.SubResourceUpdateOption) error { return nil }),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithExternalConnector(ExternalConnectorFn(func(_ context.Context, _ resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								// resource doesn't exist, which should trigger a create operation
								return ExternalObservation{ResourceExists: false, ResourceUpToDate: false}, nil
							},
							CreateFn: func(_ context.Context, _ resource.Managed) (ExternalCreation, error) {
								return ExternalCreation{}, nil
							},
							DisconnectFn: func(_ context.Context) error {
								return nil
							},
						}

						return c, nil
					})),
				},
				c: &changeLogServiceClient{},
			},
			want: want{
				callCount:  1,
				opType:     v1alpha1.OperationType_OPERATION_TYPE_CREATE,
				errMessage: "",
			},
		},
		"CreateFailureWithChangeLogs": {
			reason: "Failed managed resource creation should send a create change log entry with the error when change logs are enabled.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet:          legacyManagedMockGetFn(nil, 42),
						MockUpdate:       test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, _ client.Object, _ ...client.SubResourceUpdateOption) error { return nil }),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithExternalConnector(ExternalConnectorFn(func(_ context.Context, _ resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								// resource doesn't exist, which should trigger a create operation
								return ExternalObservation{ResourceExists: false, ResourceUpToDate: false}, nil
							},
							CreateFn: func(_ context.Context, _ resource.Managed) (ExternalCreation, error) {
								// return an error from Create to simulate a failed creation
								return ExternalCreation{}, errBoom
							},
							DisconnectFn: func(_ context.Context) error {
								return nil
							},
						}

						return c, nil
					})),
				},
				c: &changeLogServiceClient{},
			},
			want: want{
				callCount:  1,
				opType:     v1alpha1.OperationType_OPERATION_TYPE_CREATE,
				errMessage: errBoom.Error(),
			},
		},
		"UpdateSuccessfulWithChangeLogs": {
			reason: "Successful managed resource update should send an update change log entry when change logs are enabled.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet:          legacyManagedMockGetFn(nil, 42),
						MockUpdate:       test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, _ client.Object, _ ...client.SubResourceUpdateOption) error { return nil }),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithExternalConnector(ExternalConnectorFn(func(_ context.Context, _ resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								// resource exists but isn't up to date, which should trigger an update operation
								return ExternalObservation{ResourceExists: true, ResourceUpToDate: false}, nil
							},
							UpdateFn: func(_ context.Context, _ resource.Managed) (ExternalUpdate, error) {
								return ExternalUpdate{}, nil
							},
							DisconnectFn: func(_ context.Context) error {
								return nil
							},
						}

						return c, nil
					})),
				},
				c: &changeLogServiceClient{},
			},
			want: want{
				callCount:  1,
				opType:     v1alpha1.OperationType_OPERATION_TYPE_UPDATE,
				errMessage: "",
			},
		},
		"UpdateFailureWithChangeLogs": {
			reason: "Failed managed resource update should send an update change log entry with the error when change logs are enabled.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet:          legacyManagedMockGetFn(nil, 42),
						MockUpdate:       test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, _ client.Object, _ ...client.SubResourceUpdateOption) error { return nil }),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithExternalConnector(ExternalConnectorFn(func(_ context.Context, _ resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								// resource exists but isn't up to date, which should trigger an update operation
								return ExternalObservation{ResourceExists: true, ResourceUpToDate: false}, nil
							},
							UpdateFn: func(_ context.Context, _ resource.Managed) (ExternalUpdate, error) {
								// return an error from Update to simulate a failed update
								return ExternalUpdate{}, errBoom
							},
							DisconnectFn: func(_ context.Context) error {
								return nil
							},
						}

						return c, nil
					})),
				},
				c: &changeLogServiceClient{},
			},
			want: want{
				callCount:  1,
				opType:     v1alpha1.OperationType_OPERATION_TYPE_UPDATE,
				errMessage: errBoom.Error(),
			},
		},
		"DeleteSuccessfulWithChangeLogs": {
			reason: "Successful managed resource delete should send a delete change log entry when change logs are enabled.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							// set a deletion timestamp, which should trigger a delete operation
							mg := asLegacyManaged(obj, 42)
							mg.SetDeletionTimestamp(&now)

							return nil
						}),
						MockUpdate:       test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, _ client.Object, _ ...client.SubResourceUpdateOption) error { return nil }),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithExternalConnector(ExternalConnectorFn(func(_ context.Context, _ resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								// resource exists but we set a deletion timestamp above, which should trigger a delete operation
								return ExternalObservation{ResourceExists: true}, nil
							},
							DeleteFn: func(_ context.Context, _ resource.Managed) (ExternalDelete, error) {
								return ExternalDelete{}, nil
							},
							DisconnectFn: func(_ context.Context) error {
								return nil
							},
						}

						return c, nil
					})),
				},
				c: &changeLogServiceClient{},
			},
			want: want{
				callCount:  1,
				opType:     v1alpha1.OperationType_OPERATION_TYPE_DELETE,
				errMessage: "",
			},
		},
		"DeleteFailureWithChangeLogs": {
			reason: "Failed managed resource delete should send a delete change log entry with the error when change logs are enabled.",
			args: args{
				m: &fake.Manager{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							// set a deletion timestamp, which should trigger a delete operation
							mg := asLegacyManaged(obj, 42)
							mg.SetDeletionTimestamp(&now)

							return nil
						}),
						MockUpdate:       test.NewMockUpdateFn(nil),
						MockStatusUpdate: test.MockSubResourceUpdateFn(func(_ context.Context, _ client.Object, _ ...client.SubResourceUpdateOption) error { return nil }),
					},
					Scheme: fake.SchemeWith(&fake.LegacyManaged{}),
				},
				mg: resource.ManagedKind(fake.GVK(&fake.LegacyManaged{})),
				o: []ReconcilerOption{
					WithExternalConnector(ExternalConnectorFn(func(_ context.Context, _ resource.Managed) (ExternalClient, error) {
						c := &ExternalClientFns{
							ObserveFn: func(_ context.Context, _ resource.Managed) (ExternalObservation, error) {
								// resource exists but we set a deletion timestamp above, which should trigger a delete operation
								return ExternalObservation{ResourceExists: true}, nil
							},
							DeleteFn: func(_ context.Context, _ resource.Managed) (ExternalDelete, error) {
								// return an error from Delete to simulate a failed delete
								return ExternalDelete{}, errBoom
							},
							DisconnectFn: func(_ context.Context) error {
								return nil
							},
						}

						return c, nil
					})),
				},
				c: &changeLogServiceClient{},
			},
			want: want{
				callCount:  1,
				opType:     v1alpha1.OperationType_OPERATION_TYPE_DELETE,
				errMessage: errBoom.Error(),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			tc.args.o = append(tc.args.o, WithChangeLogger(NewGRPCChangeLogger(tc.args.c, WithProviderVersion("provider-cool:v9.99.999"))))
			r := NewReconciler(tc.args.m, tc.args.mg, tc.args.o...)
			r.Reconcile(context.Background(), reconcile.Request{})

			if diff := cmp.Diff(tc.want.callCount, len(tc.args.c.requests)); diff != "" {
				t.Errorf("\nReason: %s\nr.Reconcile(...): -want callCount, +got callCount:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.opType, tc.args.c.requests[0].GetEntry().GetOperation()); diff != "" {
				t.Errorf("\nReason: %s\nr.Reconcile(...): -want opType, +got opType:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.errMessage, tc.args.c.requests[0].GetEntry().GetErrorMessage()); diff != "" {
				t.Errorf("\nReason: %s\nr.Reconcile(...): -want errMessage, +got errMessage:\n%s", tc.reason, diff)
			}
		})
	}
}

func asLegacyManaged(obj client.Object, generation int64) *fake.LegacyManaged {
	mg := obj.(*fake.LegacyManaged)
	mg.Generation = generation

	return mg
}

func newLegacyManaged(generation int64) *fake.LegacyManaged {
	mg := &fake.LegacyManaged{}
	mg.Generation = generation

	return mg
}

func legacyManagedMockGetFn(err error, generation int64) test.MockGetFn {
	return test.NewMockGetFn(err, func(obj client.Object) error {
		asLegacyManaged(obj, generation)
		return nil
	})
}
