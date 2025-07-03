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

package offered

import (
	"context"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/crossplane/apis/apiextensions/shared"
	v2 "github.com/crossplane/crossplane/apis/apiextensions/v2"
	"github.com/crossplane/crossplane/internal/engine"
)

type MockEngine struct {
	MockStart        func(name string, o ...engine.ControllerOption) error
	MockStop         func(ctx context.Context, name string) error
	MockIsRunning    func(name string) bool
	MockStartWatches func(ctx context.Context, name string, ws ...engine.Watch) error
	MockGetClient    func() client.Client
}

var (
	_ ControllerEngine = &MockEngine{}
	_ ControllerEngine = &NopEngine{}
)

func (m *MockEngine) Start(name string, o ...engine.ControllerOption) error {
	return m.MockStart(name, o...)
}

func (m *MockEngine) Stop(ctx context.Context, name string) error {
	return m.MockStop(ctx, name)
}

func (m *MockEngine) IsRunning(name string) bool {
	return m.MockIsRunning(name)
}

func (m *MockEngine) StartWatches(ctx context.Context, name string, ws ...engine.Watch) error {
	return m.MockStartWatches(ctx, name, ws...)
}

func (m *MockEngine) GetCached() client.Client {
	return m.MockGetClient()
}

func TestReconcile(t *testing.T) {
	errBoom := errors.New("boom")
	testLog := logging.NewLogrLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(io.Discard)).WithName("testlog"))
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
		"RenderCompositeResourceDefinitionError": {
			reason: "We should return any error we encounter while rendering a CRD.",
			args: args{
				ca: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
					},
				},
				opts: []ReconcilerOption{
					WithCRDRenderer(CRDRenderFn(func(_ *v2.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
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
							d := o.(*v2.CompositeResourceDefinition)
							d.SetDeletionTimestamp(&now)
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(errBoom),
					},
				},
				opts: []ReconcilerOption{
					WithCRDRenderer(CRDRenderFn(func(_ *v2.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
						return &extv1.CustomResourceDefinition{}, nil
					})),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errUpdateStatus),
			},
		},
		"GetCustomResourceDefinitionError": {
			reason: "We should return any error we encounter while getting a CRD.",
			args: args{
				ca: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							switch v := o.(type) {
							case *v2.CompositeResourceDefinition:
								d := v2.CompositeResourceDefinition{}
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
					WithCRDRenderer(CRDRenderFn(func(_ *v2.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
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
							if v, ok := o.(*v2.CompositeResourceDefinition); ok {
								d := v2.CompositeResourceDefinition{}
								d.SetDeletionTimestamp(&now)
								*v = d
							}
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
				},
				opts: []ReconcilerOption{
					WithCRDRenderer(CRDRenderFn(func(_ *v2.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
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
							if v, ok := o.(*v2.CompositeResourceDefinition); ok {
								d := v2.CompositeResourceDefinition{}
								d.SetDeletionTimestamp(&now)
								*v = d
							}
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
				},
				opts: []ReconcilerOption{
					WithCRDRenderer(CRDRenderFn(func(_ *v2.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
						return &extv1.CustomResourceDefinition{}, nil
					})),
					WithControllerEngine(&MockEngine{
						MockStop: func(_ context.Context, _ string) error { return nil },
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
		"SuccessfulDelete": {
			reason: "We should not requeue when deleted if we successfully cleaned up our CRD and removed our finalizer.",
			args: args{
				ca: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							if v, ok := o.(*v2.CompositeResourceDefinition); ok {
								d := v2.CompositeResourceDefinition{}
								d.SetDeletionTimestamp(&now)
								*v = d
							}
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
				},
				opts: []ReconcilerOption{
					WithCRDRenderer(CRDRenderFn(func(_ *v2.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
						return &extv1.CustomResourceDefinition{}, nil
					})),
					WithControllerEngine(&MockEngine{
						MockStop: func(_ context.Context, _ string) error { return nil },
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
		"ListCustomResourcesError": {
			reason: "We should return any error we encounter while listing all defined resources.",
			args: args{
				ca: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							switch v := o.(type) {
							case *v2.CompositeResourceDefinition:
								d := v2.CompositeResourceDefinition{}
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
						MockList:         test.NewMockListFn(errBoom),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
				},
				opts: []ReconcilerOption{
					WithCRDRenderer(CRDRenderFn(func(_ *v2.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
						return &extv1.CustomResourceDefinition{}, nil
					})),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errListCRs),
			},
		},
		"DeleteCustomResourcesError": {
			reason: "We should return any error we encounter while deleting defined resources.",
			args: args{
				ca: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							switch v := o.(type) {
							case *v2.CompositeResourceDefinition:
								d := v2.CompositeResourceDefinition{}
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
						MockList: test.NewMockListFn(nil, func(o client.ObjectList) error {
							v := o.(*unstructured.UnstructuredList)
							*v = unstructured.UnstructuredList{
								Items: []unstructured.Unstructured{{}, {}},
							}
							return nil
						}),
						MockDelete:       test.NewMockDeleteFn(errBoom),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
				},
				opts: []ReconcilerOption{
					WithCRDRenderer(CRDRenderFn(func(_ *v2.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
						return &extv1.CustomResourceDefinition{}, nil
					})),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errDeleteCR),
			},
		},
		"SuccessfulDeleteCustomResources": {
			reason: "We should requeue to ensure our defined resources are gone before we remove our CRD.",
			args: args{
				ca: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
							switch v := o.(type) {
							case *v2.CompositeResourceDefinition:
								d := v2.CompositeResourceDefinition{}
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
						MockList: test.NewMockListFn(nil, func(o client.ObjectList) error {
							v := o.(*unstructured.UnstructuredList)
							*v = unstructured.UnstructuredList{
								Items: []unstructured.Unstructured{{}, {}},
							}
							return nil
						}),
						MockDelete:       test.NewMockDeleteFn(nil),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
				},
				opts: []ReconcilerOption{
					WithCRDRenderer(CRDRenderFn(func(_ *v2.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
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
							case *v2.CompositeResourceDefinition:
								d := v2.CompositeResourceDefinition{}
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
					WithCRDRenderer(CRDRenderFn(func(_ *v2.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
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
							case *v2.CompositeResourceDefinition:
								d := v2.CompositeResourceDefinition{
									Spec: v2.CompositeResourceDefinitionSpec{},
								}
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
						MockList:         test.NewMockListFn(nil),
						MockDelete:       test.NewMockDeleteFn(errBoom),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil),
					},
				},
				opts: []ReconcilerOption{
					WithCRDRenderer(CRDRenderFn(func(_ *v2.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
						return &extv1.CustomResourceDefinition{}, nil
					})),
					WithControllerEngine(&MockEngine{
						MockStop: func(_ context.Context, _ string) error { return nil },
					}),
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
							case *v2.CompositeResourceDefinition:
								d := v2.CompositeResourceDefinition{}
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
						MockList:   test.NewMockListFn(nil),
						MockDelete: test.NewMockDeleteFn(nil),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(got client.Object) error {
							want := &v2.CompositeResourceDefinition{}
							want.SetUID(owner)
							want.SetDeletionTimestamp(&now)
							want.Status.SetConditions(shared.TerminatingClaim())

							if diff := cmp.Diff(want, got); diff != "" {
								t.Errorf("MockStatusUpdate: -want, +got:\n%s\n", diff)
							}

							return nil
						}),
					},
				},
				opts: []ReconcilerOption{
					WithCRDRenderer(CRDRenderFn(func(_ *v2.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
						return &extv1.CustomResourceDefinition{}, nil
					})),
					WithControllerEngine(&MockEngine{
						MockStop: func(_ context.Context, _ string) error { return nil },
					}),
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
					WithCRDRenderer(CRDRenderFn(func(_ *v2.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
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
		"ApplyCRDError": {
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
					WithCRDRenderer(CRDRenderFn(func(_ *v2.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
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
					WithCRDRenderer(CRDRenderFn(func(_ *v2.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
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
			reason: "We should return any error we encounter while stopping our controller because the XRD's referenceable version changed.",
			args: args{
				ca: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							xrd := &v2.CompositeResourceDefinition{
								Spec: v2.CompositeResourceDefinitionSpec{
									Group: "example.org",
									ClaimNames: &extv1.CustomResourceDefinitionNames{
										Kind: "Claim",
									},
									Versions: []v2.CompositeResourceDefinitionVersion{
										{
											Name:          "v2",
											Referenceable: true,
										},
										{
											Name: "v1",
										},
									},
								},
								Status: v2.CompositeResourceDefinitionStatus{
									Controllers: v2.CompositeResourceDefinitionControllerStatus{
										CompositeResourceClaimTypeRef: v2.TypeReference{
											APIVersion: "example.org/v1",
											Kind:       "Claim",
										},
									},
								},
							}

							*obj.(*v2.CompositeResourceDefinition) = *xrd
							return nil
						}),
					},
					Applicator: resource.ApplyFn(func(_ context.Context, _ client.Object, _ ...resource.ApplyOption) error {
						return nil
					}),
				},
				opts: []ReconcilerOption{
					WithCRDRenderer(CRDRenderFn(func(_ *v2.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
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
					WithCRDRenderer(CRDRenderFn(func(_ *v2.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
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
						MockStart:     func(_ string, _ ...engine.ControllerOption) error { return errBoom },
						MockGetClient: func() client.Client { return test.NewMockClient() },
					}),
				},
			},
			want: want{
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
					WithCRDRenderer(CRDRenderFn(func(_ *v2.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
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
						MockStartWatches: func(_ context.Context, _ string, _ ...engine.Watch) error {
							return errBoom
						},
						MockGetClient: func() client.Client { return test.NewMockClient() },
					}),
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: errors.Wrap(errBoom, errStartWatches),
			},
		},
		"SuccessfulStart": {
			reason: "We should not requeue if we successfully ensured our CRD exists and controller is started.",
			args: args{
				ca: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
							want := &v2.CompositeResourceDefinition{}
							want.Status.SetConditions(shared.WatchingClaim())

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
					WithCRDRenderer(CRDRenderFn(func(_ *v2.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
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
						MockStartWatches: func(_ context.Context, _ string, _ ...engine.Watch) error { return nil },
						MockGetClient:    func() client.Client { return test.NewMockClient() },
					},
					),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"SuccessfulUpdateControllerVersion": {
			reason: "We should not requeue if we successfully ensured our CRD exists, the old controller stopped, and the new one started.",
			args: args{
				ca: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil, func(obj client.Object) error {
							d := obj.(*v2.CompositeResourceDefinition)
							d.Spec.ClaimNames = &extv1.CustomResourceDefinitionNames{} //nolint:staticcheck // we are still supporting v1 XRD
							d.Spec.Versions = []v2.CompositeResourceDefinitionVersion{
								{Name: "old", Referenceable: false},
								{Name: "new", Referenceable: true},
							}
							d.Status.Controllers.CompositeResourceClaimTypeRef = v2.TypeReference{APIVersion: "old"}
							return nil
						}),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
							want := &v2.CompositeResourceDefinition{}
							want.Spec.ClaimNames = &extv1.CustomResourceDefinitionNames{} //nolint:staticcheck // we are still supporting v1 XRD
							want.Spec.Versions = []v2.CompositeResourceDefinitionVersion{
								{Name: "old", Referenceable: false},
								{Name: "new", Referenceable: true},
							}
							want.Status.Controllers.CompositeResourceClaimTypeRef = v2.TypeReference{APIVersion: "new"}
							want.Status.SetConditions(shared.WatchingClaim())

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
					WithCRDRenderer(CRDRenderFn(func(_ *v2.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
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
						MockStartWatches: func(_ context.Context, _ string, _ ...engine.Watch) error { return nil },
						MockGetClient:    func() client.Client { return test.NewMockClient() },
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
		"NotRestartingWithoutVersionChange": {
			reason: "We should return without requeuing if we successfully ensured our CRD exists and controller is started.",
			args: args{
				ca: resource.ClientApplicator{
					Client: &test.MockClient{
						MockGet: test.NewMockGetFn(nil),
						MockStatusUpdate: test.NewMockSubResourceUpdateFn(nil, func(o client.Object) error {
							want := &v2.CompositeResourceDefinition{}
							want.Status.SetConditions(shared.WatchingClaim())

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
					WithCRDRenderer(CRDRenderFn(func(_ *v2.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
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
			r := NewReconciler(tc.args.ca, append(tc.args.opts, WithLogger(testLog))...)

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
