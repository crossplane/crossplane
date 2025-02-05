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

package definition

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/crossplane/internal/engine"
)

var (
	_ ControllerEngine = &MockEngine{}
	_ ControllerEngine = &NopEngine{}
)

type MockEngine struct {
	MockStart           func(name string, o ...engine.ControllerOption) error
	MockStop            func(ctx context.Context, name string) error
	MockIsRunning       func(name string) bool
	MockGetWatches      func(name string) ([]engine.WatchID, error)
	MockStartWatches    func(name string, ws ...engine.Watch) error
	MockStopWatches     func(ctx context.Context, name string, ws ...engine.WatchID) (int, error)
	MockGetCached       func() client.Client
	MockGetUncached     func() client.Client
	MockGetFieldIndexer func() client.FieldIndexer
}

func (m *MockEngine) IsRunning(name string) bool {
	return m.MockIsRunning(name)
}

func (m *MockEngine) Start(name string, o ...engine.ControllerOption) error {
	return m.MockStart(name, o...)
}

func (m *MockEngine) Stop(ctx context.Context, name string) error {
	return m.MockStop(ctx, name)
}

func (m *MockEngine) GetWatches(name string) ([]engine.WatchID, error) {
	return m.MockGetWatches(name)
}

func (m *MockEngine) StartWatches(name string, ws ...engine.Watch) error {
	return m.MockStartWatches(name, ws...)
}

func (m *MockEngine) StopWatches(ctx context.Context, name string, ws ...engine.WatchID) (int, error) {
	return m.MockStopWatches(ctx, name, ws...)
}

func (m *MockEngine) GetCached() client.Client {
	return m.MockGetCached()
}

func (m *MockEngine) GetUncached() client.Client {
	return m.MockGetUncached()
}

func (m *MockEngine) GetFieldIndexer() client.FieldIndexer {
	return m.MockGetFieldIndexer()
}

func TestReconcile(t *testing.T) {
	errBoom := errors.New("boom")
	now := metav1.Now()
	owner := types.UID("definitely-a-uuid")
	ctrlr := true

	type args struct {
		ca   resource.ClientApplicator
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
		"CompositeResourceDefinitionNotFound": {
			reason: "We should not return an error if the CompositeResourceDefinition was not found.",
			args: args{
				ca: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(kerrors.NewNotFound(schema.GroupResource{}, "")),
					},
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"GetCompositeResourceDefinitionError": {
			reason: "We should return any other error encountered while getting a CompositeResourceDefinition.",
			args: args{
				ca: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(errBoom),
					},
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetXRD),
			},
		},
		"RenderCustomResourceDefinitionError": {
			reason: "We should return any error we encounter rendering a CRD.",
			args: args{
				ca: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
					},
				},
				opts: []ReconcilerOption{
					WithCRDRenderer(CRDRenderFn(func(_ *v1.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
						return nil, errBoom
					})),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errRenderCRD),
			},
		},
		"SetTerminatingConditionError": {
			reason: "We should return any error we encounter while setting the terminating status condition.",
			args: args{
				ca: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							d := o.(*v1.CompositeResourceDefinition)
							d.SetDeletionTimestamp(&now)
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(errBoom),
					},
				},
				opts: []ReconcilerOption{
					WithCRDRenderer(CRDRenderFn(func(_ *v1.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
						return &extv1.CustomResourceDefinition{}, nil
					})),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateStatus),
			},
		},
		"GetCustomResourceDefinitionError": {
			reason: "We should return any error we encounter getting a CRD.",
			args: args{
				ca: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							switch v := o.(type) {
							case *v1.CompositeResourceDefinition:
								d := v1.CompositeResourceDefinition{}
								d.SetDeletionTimestamp(&now)
								*v = d
							case *extv1.CustomResourceDefinition:
								return errBoom
							}
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
				},
				opts: []ReconcilerOption{
					WithCRDRenderer(CRDRenderFn(func(_ *v1.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
						return &extv1.CustomResourceDefinition{}, nil
					})),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetCRD),
			},
		},
		"CustomResourceDefinitionNotFoundStopControllerError": {
			reason: "We should return any error we encounter while stopping our controller (just in case) when the CRD doesn't exist.",
			args: args{
				ca: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							if v, ok := o.(*v1.CompositeResourceDefinition); ok {
								d := v1.CompositeResourceDefinition{}
								d.SetDeletionTimestamp(&now)
								*v = d
							}
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
				},
				opts: []ReconcilerOption{
					WithCRDRenderer(CRDRenderFn(func(_ *v1.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
						return &extv1.CustomResourceDefinition{}, nil
					})),
					WithControllerEngine(&MockEngine{
						MockStop: func(_ context.Context, _ string) error {
							return errBoom
						},
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errStopController),
			},
		},
		"RemoveFinalizerError": {
			reason: "We should return any error we encounter while removing a finalizer.",
			args: args{
				ca: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							if v, ok := o.(*v1.CompositeResourceDefinition); ok {
								d := v1.CompositeResourceDefinition{}
								d.SetDeletionTimestamp(&now)
								*v = d
							}
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
				},
				opts: []ReconcilerOption{
					WithCRDRenderer(CRDRenderFn(func(_ *v1.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
						return &extv1.CustomResourceDefinition{}, nil
					})),
					WithFinalizer(resource.FinalizerFns{RemoveFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return errBoom
					}}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errRemoveFinalizer),
			},
		},
		"SuccessfulDelete": {
			reason: "We should not requeue when deleted if we successfully cleaned up our CRD and removed our finalizer.",
			args: args{
				ca: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							if v, ok := o.(*v1.CompositeResourceDefinition); ok {
								d := v1.CompositeResourceDefinition{}
								d.SetDeletionTimestamp(&now)
								*v = d
							}
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
				},
				opts: []ReconcilerOption{
					WithCRDRenderer(CRDRenderFn(func(_ *v1.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
						return &extv1.CustomResourceDefinition{}, nil
					})),
					WithFinalizer(resource.FinalizerFns{RemoveFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return nil
					}}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"DeleteAllCustomResourcesError": {
			reason: "We should return any error we encounter while deleting all defined resources.",
			args: args{
				ca: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							switch v := o.(type) {
							case *v1.CompositeResourceDefinition:
								d := v1.CompositeResourceDefinition{}
								d.SetUID(owner)
								d.SetDeletionTimestamp(&now)
								*v = d
							case *extv1.CustomResourceDefinition:
								crd := extv1.CustomResourceDefinition{}
								crd.SetCreationTimestamp(now)
								crd.SetOwnerReferences([]metav1.OwnerReference{{UID: owner, Controller: &ctrlr}})
								*v = crd
							}
							return nil
						}),
						MockDeleteAllOf:  test.NewMockDeleteAllOfFn(errBoom),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
				},
				opts: []ReconcilerOption{
					WithCRDRenderer(CRDRenderFn(func(_ *v1.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
						return &extv1.CustomResourceDefinition{}, nil
					})),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errDeleteCRs),
			},
		},
		"ListCustomResourcesError": {
			reason: "We should return any error we encounter while listing all defined resources.",
			args: args{
				ca: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							switch v := o.(type) {
							case *v1.CompositeResourceDefinition:
								d := v1.CompositeResourceDefinition{}
								d.SetUID(owner)
								d.SetDeletionTimestamp(&now)
								*v = d
							case *extv1.CustomResourceDefinition:
								crd := extv1.CustomResourceDefinition{}
								crd.SetCreationTimestamp(now)
								crd.SetOwnerReferences([]metav1.OwnerReference{{UID: owner, Controller: &ctrlr}})
								*v = crd
							}
							return nil
						}),
						MockDeleteAllOf:  test.NewMockDeleteAllOfFn(nil),
						MockList:         test.NewMockListFn(errBoom),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
				},
				opts: []ReconcilerOption{
					WithCRDRenderer(CRDRenderFn(func(_ *v1.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
						return &extv1.CustomResourceDefinition{}, nil
					})),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errListCRs),
			},
		},
		"WaitForDeleteAllOf": {
			reason: "We should record the pending deletion of defined resources.",
			args: args{
				ca: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							switch v := o.(type) {
							case *v1.CompositeResourceDefinition:
								d := v1.CompositeResourceDefinition{}
								d.SetUID(owner)
								d.SetDeletionTimestamp(&now)
								*v = d
							case *extv1.CustomResourceDefinition:
								crd := extv1.CustomResourceDefinition{}
								crd.SetCreationTimestamp(now)
								crd.SetOwnerReferences([]metav1.OwnerReference{{UID: owner, Controller: &ctrlr}})
								*v = crd
							}
							return nil
						}),
						MockDeleteAllOf: test.NewMockDeleteAllOfFn(nil),
						MockList: test.NewMockListFn(nil, func(o client.ObjectList) error {
							v := o.(*unstructured.UnstructuredList)
							*v = unstructured.UnstructuredList{
								Items: []unstructured.Unstructured{{}, {}},
							}
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
				},
				opts: []ReconcilerOption{
					WithCRDRenderer(CRDRenderFn(func(_ *v1.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
						return &extv1.CustomResourceDefinition{}, nil
					})),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: true},
			},
		},
		"StopControllerError": {
			reason: "We should return any error we encounter while stopping our controller.",
			args: args{
				ca: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							switch v := o.(type) {
							case *v1.CompositeResourceDefinition:
								d := v1.CompositeResourceDefinition{}
								d.SetUID(owner)
								d.SetDeletionTimestamp(&now)
								*v = d
							case *extv1.CustomResourceDefinition:
								crd := extv1.CustomResourceDefinition{}
								crd.SetCreationTimestamp(now)
								crd.SetOwnerReferences([]metav1.OwnerReference{{UID: owner, Controller: &ctrlr}})
								*v = crd
							}
							return nil
						}),
						MockDeleteAllOf:  test.NewMockDeleteAllOfFn(nil),
						MockList:         test.NewMockListFn(nil),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
				},
				opts: []ReconcilerOption{
					WithCRDRenderer(CRDRenderFn(func(_ *v1.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
						return &extv1.CustomResourceDefinition{}, nil
					})),
					WithControllerEngine(&MockEngine{
						MockStop: func(_ context.Context, _ string) error {
							return errBoom
						},
					}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errStopController),
			},
		},
		"DeleteCustomResourceDefinitionError": {
			reason: "We should return any error we encounter while deleting the CRD we created.",
			args: args{
				ca: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							switch v := o.(type) {
							case *v1.CompositeResourceDefinition:
								d := v1.CompositeResourceDefinition{}
								d.SetUID(owner)
								d.SetDeletionTimestamp(&now)
								*v = d
							case *extv1.CustomResourceDefinition:
								crd := extv1.CustomResourceDefinition{}
								crd.SetCreationTimestamp(now)
								crd.SetOwnerReferences([]metav1.OwnerReference{{UID: owner, Controller: &ctrlr}})
								*v = crd
							}
							return nil
						}),
						MockDeleteAllOf:  test.NewMockDeleteAllOfFn(nil),
						MockList:         test.NewMockListFn(nil),
						MockDelete:       test.NewMockDeleteFn(errBoom),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
				},
				opts: []ReconcilerOption{
					WithCRDRenderer(CRDRenderFn(func(_ *v1.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
						return &extv1.CustomResourceDefinition{}, nil
					})),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errDeleteCRD),
			},
		},
		"SuccessfulCleanup": {
			reason: "We should requeue to remove our finalizer once we've cleaned up our defined resources and CRD.",
			args: args{
				ca: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							switch v := o.(type) {
							case *v1.CompositeResourceDefinition:
								d := v1.CompositeResourceDefinition{}
								d.SetUID(owner)
								d.SetDeletionTimestamp(&now)
								*v = d
							case *extv1.CustomResourceDefinition:
								crd := extv1.CustomResourceDefinition{}
								crd.SetCreationTimestamp(now)
								crd.SetOwnerReferences([]metav1.OwnerReference{{UID: owner, Controller: &ctrlr}})
								*v = crd
							}
							return nil
						}),
						MockDeleteAllOf: test.NewMockDeleteAllOfFn(nil),
						MockList:        test.NewMockListFn(nil),
						MockDelete:      test.NewMockDeleteFn(nil),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(got client.Object) error {
							want := &v1.CompositeResourceDefinition{}
							want.SetUID(owner)
							want.SetDeletionTimestamp(&now)
							want.Status.SetConditions(v1.TerminatingComposite())

							if diff := cmp.Diff(want, got); diff != "" {
								t.Errorf("MockStatusUpdate: -want, +got:\n%s\n", diff)
							}

							return nil
						}),
					},
				},
				opts: []ReconcilerOption{
					WithCRDRenderer(CRDRenderFn(func(_ *v1.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
						return &extv1.CustomResourceDefinition{}, nil
					})),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: true},
			},
		},
		"AddFinalizerError": {
			reason: "We should return any error we encounter while adding a finalizer.",
			args: args{
				ca: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
					},
				},
				opts: []ReconcilerOption{
					WithCRDRenderer(CRDRenderFn(func(_ *v1.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
						return &extv1.CustomResourceDefinition{}, nil
					})),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return errBoom
					}}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errAddFinalizer),
			},
		},
		"ApplyCustomResourceDefinitionError": {
			reason: "We should return any error we encounter while applying our CRD.",
			args: args{
				ca: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
					},
					Applicator: resource.ApplyFn(func(_ context.Context, _ client.Object, _ ...resource.ApplyOption) error {
						return errBoom
					}),
				},
				opts: []ReconcilerOption{
					WithCRDRenderer(CRDRenderFn(func(_ *v1.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
						return &extv1.CustomResourceDefinition{}, nil
					})),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return nil
					}}),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errApplyCRD),
			},
		},
		"CustomResourceDefinitionIsNotEstablished": {
			reason: "We should requeue if we're waiting for a newly created CRD to become established.",
			args: args{
				ca: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
					},
					Applicator: resource.ApplyFn(func(_ context.Context, _ client.Object, _ ...resource.ApplyOption) error {
						return nil
					}),
				},
				opts: []ReconcilerOption{
					WithCRDRenderer(CRDRenderFn(func(_ *v1.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
						return &extv1.CustomResourceDefinition{}, nil
					})),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return nil
					}}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: true},
			},
		},
		"VersionChangedStopControllerError": {
			reason: "We should return any error we encounter while stopping our controller because the XRD's referencable version changed.",
			args: args{
				ca: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							xrd := &v1.CompositeResourceDefinition{
								Spec: v1.CompositeResourceDefinitionSpec{
									Group: "example.org",
									Names: extv1.CustomResourceDefinitionNames{
										Kind: "XR",
									},
									Versions: []v1.CompositeResourceDefinitionVersion{
										{
											Name:          "v2",
											Referenceable: true,
										},
										{
											Name: "v1",
										},
									},
								},
								Status: v1.CompositeResourceDefinitionStatus{
									Controllers: v1.CompositeResourceDefinitionControllerStatus{
										CompositeResourceTypeRef: v1.TypeReference{
											APIVersion: "example.org/v1",
											Kind:       "XR",
										},
									},
								},
							}

							*obj.(*v1.CompositeResourceDefinition) = *xrd
							return nil
						}),
					},
					Applicator: resource.ApplyFn(func(_ context.Context, _ client.Object, _ ...resource.ApplyOption) error {
						return nil
					}),
				},
				opts: []ReconcilerOption{
					WithCRDRenderer(CRDRenderFn(func(_ *v1.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
						return &extv1.CustomResourceDefinition{
							Status: extv1.CustomResourceDefinitionStatus{
								Conditions: []extv1.CustomResourceDefinitionCondition{
									{Type: extv1.Established, Status: extv1.ConditionTrue},
								},
							},
						}, nil
					})),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return nil
					}}),
					WithControllerEngine(&MockEngine{
						MockIsRunning: func(_ string) bool { return false },
						MockStop: func(_ context.Context, _ string) error {
							return errBoom
						},
					}),
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: errors.Wrap(errBoom, errStopController),
			},
		},
		"StartControllerError": {
			reason: "We should return any error we encounter while starting our controller.",
			args: args{
				ca: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
					},
					Applicator: resource.ApplyFn(func(_ context.Context, _ client.Object, _ ...resource.ApplyOption) error {
						return nil
					}),
				},
				opts: []ReconcilerOption{
					WithCRDRenderer(CRDRenderFn(func(_ *v1.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
						return &extv1.CustomResourceDefinition{
							Status: extv1.CustomResourceDefinitionStatus{
								Conditions: []extv1.CustomResourceDefinitionCondition{
									{Type: extv1.Established, Status: extv1.ConditionTrue},
								},
							},
						}, nil
					})),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return nil
					}}),
					WithControllerEngine(&MockEngine{
						MockIsRunning: func(_ string) bool { return false },
						MockStart: func(_ string, _ ...engine.ControllerOption) error {
							return errBoom
						},
						MockGetCached:   func() client.Client { return test.NewMockClient() },
						MockGetUncached: func() client.Client { return test.NewMockClient() },
					}),
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: errors.Wrap(errBoom, errStartController),
			},
		},
		"StartWatchesError": {
			reason: "We should return any error we encounter while starting watches.",
			args: args{
				ca: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
					},
					Applicator: resource.ApplyFn(func(_ context.Context, _ client.Object, _ ...resource.ApplyOption) error {
						return nil
					}),
				},
				opts: []ReconcilerOption{
					WithCRDRenderer(CRDRenderFn(func(_ *v1.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
						return &extv1.CustomResourceDefinition{
							Status: extv1.CustomResourceDefinitionStatus{
								Conditions: []extv1.CustomResourceDefinitionCondition{
									{Type: extv1.Established, Status: extv1.ConditionTrue},
								},
							},
						}, nil
					})),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return nil
					}}),
					WithControllerEngine(&MockEngine{
						MockIsRunning: func(_ string) bool { return false },
						MockStart: func(_ string, _ ...engine.ControllerOption) error {
							return nil
						},
						MockStartWatches: func(_ string, _ ...engine.Watch) error {
							return errBoom
						},
						MockGetCached:   func() client.Client { return test.NewMockClient() },
						MockGetUncached: func() client.Client { return test.NewMockClient() },
					}),
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: errors.Wrap(errBoom, errStartWatches),
			},
		},
		"SuccessfulStart": {
			reason: "We should return without requeueing if we successfully ensured our CRD exists and controller is started.",
			args: args{
				ca: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
							want := &v1.CompositeResourceDefinition{}
							want.Status.SetConditions(v1.WatchingComposite())

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
				opts: []ReconcilerOption{
					WithCRDRenderer(CRDRenderFn(func(_ *v1.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
						return &extv1.CustomResourceDefinition{
							Status: extv1.CustomResourceDefinitionStatus{
								Conditions: []extv1.CustomResourceDefinitionCondition{
									{Type: extv1.Established, Status: extv1.ConditionTrue},
								},
							},
						}, nil
					})),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return nil
					}}),
					WithControllerEngine(&MockEngine{
						MockIsRunning:    func(_ string) bool { return false },
						MockStart:        func(_ string, _ ...engine.ControllerOption) error { return nil },
						MockStartWatches: func(_ string, _ ...engine.Watch) error { return nil },
						MockGetCached:    func() client.Client { return test.NewMockClient() },
						MockGetUncached:  func() client.Client { return test.NewMockClient() },
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"SuccessfulUpdateControllerVersion": {
			reason: "We should return without requeueing if we successfully ensured our CRD exists, the old controller stopped, and the new one started.",
			args: args{
				ca: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							d := obj.(*v1.CompositeResourceDefinition)
							d.Spec.Versions = []v1.CompositeResourceDefinitionVersion{
								{Name: "old", Referenceable: false},
								{Name: "new", Referenceable: true},
							}
							d.Status.Controllers.CompositeResourceTypeRef = v1.TypeReference{APIVersion: "old"}
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
							want := &v1.CompositeResourceDefinition{}
							want.Spec.Versions = []v1.CompositeResourceDefinitionVersion{
								{Name: "old", Referenceable: false},
								{Name: "new", Referenceable: true},
							}
							want.Status.Controllers.CompositeResourceTypeRef = v1.TypeReference{APIVersion: "new"}
							want.Status.SetConditions(v1.WatchingComposite())

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
				opts: []ReconcilerOption{
					WithCRDRenderer(CRDRenderFn(func(_ *v1.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
						return &extv1.CustomResourceDefinition{
							Status: extv1.CustomResourceDefinitionStatus{
								Conditions: []extv1.CustomResourceDefinitionCondition{
									{Type: extv1.Established, Status: extv1.ConditionTrue},
								},
							},
						}, nil
					})),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return nil
					}}),
					WithControllerEngine(&MockEngine{
						MockStart:        func(_ string, _ ...engine.ControllerOption) error { return nil },
						MockStop:         func(_ context.Context, _ string) error { return nil },
						MockIsRunning:    func(_ string) bool { return false },
						MockStartWatches: func(_ string, _ ...engine.Watch) error { return nil },
						MockGetCached:    func() client.Client { return test.NewMockClient() },
						MockGetUncached:  func() client.Client { return test.NewMockClient() },
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"NotRestartingWithoutVersionChange": {
			reason: "We should return without requeueing if we successfully ensured our CRD exists and controller is started.",
			args: args{
				ca: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
							want := &v1.CompositeResourceDefinition{}
							want.Status.SetConditions(v1.WatchingComposite())

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
				opts: []ReconcilerOption{
					WithCRDRenderer(CRDRenderFn(func(_ *v1.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
						return &extv1.CustomResourceDefinition{
							Status: extv1.CustomResourceDefinitionStatus{
								Conditions: []extv1.CustomResourceDefinitionCondition{
									{Type: extv1.Established, Status: extv1.ConditionTrue},
								},
							},
						}, nil
					})),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return nil
					}}),
					WithControllerEngine(&MockEngine{
						MockIsRunning: func(_ string) bool { return true },
						MockStart: func(_ string, _ ...engine.ControllerOption) error {
							t.Errorf("MockStart should not be called")
							return nil
						},
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewReconciler(tc.args.ca, tc.args.opts...)
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
