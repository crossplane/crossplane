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

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/config"
	kcontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	apiextensionscontroller "github.com/crossplane/crossplane/internal/controller/apiextensions/controller"
	"github.com/crossplane/crossplane/internal/features"
)

type MockEngine struct {
	ControllerEngine
	MockIsRunning func(name string) bool
	MockCreate    func(name string, o kcontroller.Options, w ...controller.Watch) (controller.NamedController, error)
	MockStart     func(name string, o kcontroller.Options, w ...controller.Watch) error
	MockStop      func(name string)
	MockErr       func(name string) error
}

func (m *MockEngine) IsRunning(name string) bool {
	return m.MockIsRunning(name)
}

func (m *MockEngine) Create(name string, o kcontroller.Options, w ...controller.Watch) (controller.NamedController, error) {
	return m.MockCreate(name, o, w...)
}

func (m *MockEngine) Start(name string, o kcontroller.Options, w ...controller.Watch) error {
	return m.MockStart(name, o, w...)
}

func (m *MockEngine) Stop(name string) {
	m.MockStop(name)
}

func (m *MockEngine) Err(name string) error {
	return m.MockErr(name)
}

func TestReconcile(t *testing.T) {
	errBoom := errors.New("boom")
	now := metav1.Now()
	owner := types.UID("definitely-a-uuid")
	ctrlr := true

	type args struct {
		mgr  manager.Manager
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
		"GetCompositeResourceDefinitionError": {
			reason: "We should return any other error encountered while getting a CompositeResourceDefinition.",
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
				err: errors.Wrap(errBoom, errGetXRD),
			},
		},
		"RenderCustomResourceDefinitionError": {
			reason: "We should return any error we encounter rendering a CRD.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil),
						},
					}),
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
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o client.Object) error {
								d := o.(*v1.CompositeResourceDefinition)
								d.SetDeletionTimestamp(&now)
								return nil
							}),
							MockStatusUpdate: test.NewMockSubResourceUpdateFn(errBoom),
						},
					}),
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
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
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
					}),
					WithCRDRenderer(CRDRenderFn(func(_ *v1.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
						return &extv1.CustomResourceDefinition{}, nil
					})),
				},
			},
			want: want{
				err: errors.Wrap(errBoom, errGetCRD),
			},
		},
		"RemoveFinalizerError": {
			reason: "We should return any error we encounter while removing a finalizer.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
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
					}),
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
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
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
					}),
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
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
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
					}),
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
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
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
					}),
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
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
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
					}),
					WithCRDRenderer(CRDRenderFn(func(_ *v1.CompositeResourceDefinition) (*extv1.CustomResourceDefinition, error) {
						return &extv1.CustomResourceDefinition{}, nil
					})),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: true},
			},
		},
		"DeleteCustomResourceDefinitionError": {
			reason: "We should return any error we encounter while deleting the CRD we created.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
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
					}),
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
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
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
					}),
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
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil),
						},
					}),
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
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil),
						},
						Applicator: resource.ApplyFn(func(_ context.Context, _ client.Object, _ ...resource.ApplyOption) error {
							return errBoom
						}),
					}),
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
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil),
						},
						Applicator: resource.ApplyFn(func(_ context.Context, _ client.Object, _ ...resource.ApplyOption) error {
							return nil
						}),
					}),
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
		"CreateControllerError": {
			reason: "We should return any error we encounter while starting our controller.",
			args: args{
				mgr: &mockManager{
					GetCacheFn: func() cache.Cache {
						return &mockCache{
							ListFn: func(_ context.Context, _ client.ObjectList, _ ...client.ListOption) error { return nil },
						}
					},
					GetClientFn: func() client.Client {
						return &test.MockClient{MockList: test.NewMockListFn(nil)}
					},
					GetSchemeFn: runtime.NewScheme,
					GetRESTMapperFn: func() meta.RESTMapper {
						return meta.NewDefaultRESTMapper([]schema.GroupVersion{v1.SchemeGroupVersion})
					},
				},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil),
						},
						Applicator: resource.ApplyFn(func(_ context.Context, _ client.Object, _ ...resource.ApplyOption) error {
							return nil
						}),
					}),
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
						MockErr:       func(_ string) error { return nil },
						MockCreate: func(_ string, _ kcontroller.Options, _ ...controller.Watch) (controller.NamedController, error) {
							return nil, errBoom
						},
					}),
				},
			},
			want: want{
				r:   reconcile.Result{},
				err: errors.Wrap(errBoom, errStartController),
			},
		},
		"SuccessfulStart": {
			reason: "We should return without requeueing if we successfully ensured our CRD exists and controller is started.",
			args: args{
				mgr: &mockManager{
					GetCacheFn: func() cache.Cache {
						return &mockCache{
							ListFn: func(_ context.Context, _ client.ObjectList, _ ...client.ListOption) error { return nil },
						}
					},
					GetClientFn: func() client.Client {
						return &test.MockClient{MockList: test.NewMockListFn(nil)}
					},
					GetSchemeFn: runtime.NewScheme,
					GetRESTMapperFn: func() meta.RESTMapper {
						return meta.NewDefaultRESTMapper([]schema.GroupVersion{v1.SchemeGroupVersion})
					},
				},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
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
					}),
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
						MockErr:       func(_ string) error { return errBoom }, // This error should only be logged.
						MockCreate: func(_ string, _ kcontroller.Options, _ ...controller.Watch) (controller.NamedController, error) {
							return mockNamedController{
								MockStart: func(_ context.Context) error { return nil },
								MockGetCache: func() cache.Cache {
									return &mockCache{
										IndexFieldFn: func(_ context.Context, _ client.Object, _ string, _ client.IndexerFunc) error {
											return nil
										},
										WaitForCacheSyncFn: func(_ context.Context) bool {
											return true
										},
									}
								},
							}, nil
						},
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
				mgr: &mockManager{
					GetCacheFn: func() cache.Cache {
						return &mockCache{
							ListFn: func(_ context.Context, _ client.ObjectList, _ ...client.ListOption) error { return nil },
						}
					},
					GetClientFn: func() client.Client {
						return &test.MockClient{MockList: test.NewMockListFn(nil)}
					},
					GetSchemeFn: runtime.NewScheme,
					GetRESTMapperFn: func() meta.RESTMapper {
						return meta.NewDefaultRESTMapper([]schema.GroupVersion{v1.SchemeGroupVersion})
					},
				},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
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
					}),
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
						MockErr: func(_ string) error { return nil },
						MockCreate: func(_ string, _ kcontroller.Options, _ ...controller.Watch) (controller.NamedController, error) {
							return mockNamedController{
								MockStart: func(_ context.Context) error { return nil },
								MockGetCache: func() cache.Cache {
									return &mockCache{
										IndexFieldFn: func(_ context.Context, _ client.Object, _ string, _ client.IndexerFunc) error {
											return nil
										},
										WaitForCacheSyncFn: func(_ context.Context) bool {
											return true
										},
									}
								},
							}, nil
						},
						MockStop: func(_ string) {},
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
				mgr: &mockManager{
					GetCacheFn: func() cache.Cache {
						return &mockCache{
							ListFn: func(_ context.Context, _ client.ObjectList, _ ...client.ListOption) error { return nil },
						}
					},
					GetClientFn: func() client.Client {
						return &test.MockClient{MockList: test.NewMockListFn(nil)}
					},
					GetSchemeFn: runtime.NewScheme,
					GetRESTMapperFn: func() meta.RESTMapper {
						return meta.NewDefaultRESTMapper([]schema.GroupVersion{v1.SchemeGroupVersion})
					},
				},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
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
					}),
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
						MockErr:       func(_ string) error { return errBoom }, // This error should only be logged.
						MockCreate: func(_ string, _ kcontroller.Options, _ ...controller.Watch) (controller.NamedController, error) {
							t.Errorf("MockCreate should not be called")
							return nil, nil
						},
					}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
	}

	// Run every test with and without the realtime compositions feature.
	rtc := apiextensionscontroller.Options{Options: controller.DefaultOptions()}
	rtc.Features.Enable(features.EnableAlphaRealtimeCompositions)

	type mode struct {
		name  string
		extra []ReconcilerOption
	}
	for _, m := range []mode{
		{name: "Default"},
		{name: string(features.EnableAlphaRealtimeCompositions), extra: []ReconcilerOption{WithOptions(rtc)}},
	} {
		t.Run(m.name, func(t *testing.T) {
			for name, tc := range cases {
				t.Run(name, func(t *testing.T) {
					r := NewReconciler(tc.args.mgr, append(tc.args.opts, m.extra...)...)
					got, err := r.Reconcile(context.Background(), reconcile.Request{})

					if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
						t.Errorf("\n%s\nr.Reconcile(...): -want error, +got error:\n%s", tc.reason, diff)
					}
					if diff := cmp.Diff(tc.want.r, got, test.EquateErrors()); diff != "" {
						t.Errorf("\n%s\nr.Reconcile(...): -want, +got:\n%s", tc.reason, diff)
					}
				})
			}
		})
	}
}

type mockNamedController struct {
	MockStart    func(_ context.Context) error
	MockGetCache func() cache.Cache
}

func (m mockNamedController) Start(ctx context.Context) error {
	return m.MockStart(ctx)
}

func (m mockNamedController) GetCache() cache.Cache {
	return m.MockGetCache()
}

type mockManager struct {
	manager.Manager

	GetCacheFn             func() cache.Cache
	GetClientFn            func() client.Client
	GetSchemeFn            func() *runtime.Scheme
	GetRESTMapperFn        func() meta.RESTMapper
	GetConfigFn            func() *rest.Config
	GetLoggerFn            func() logr.Logger
	GetControllerOptionsFn func() ctrlconfig.Controller
}

func (m *mockManager) GetCache() cache.Cache {
	return m.GetCacheFn()
}

func (m *mockManager) GetClient() client.Client {
	return m.GetClientFn()
}

func (m *mockManager) GetScheme() *runtime.Scheme {
	return m.GetSchemeFn()
}

func (m *mockManager) GetRESTMapper() meta.RESTMapper {
	return m.GetRESTMapperFn()
}

func (m *mockManager) GetConfig() *rest.Config {
	return m.GetConfigFn()
}

func (m *mockManager) GetLogger() logr.Logger {
	return m.GetLoggerFn()
}

func (m *mockManager) GetControllerOptions() ctrlconfig.Controller {
	return m.GetControllerOptionsFn()
}

type mockCache struct {
	cache.Cache

	ListFn             func(_ context.Context, list client.ObjectList, opts ...client.ListOption) error
	IndexFieldFn       func(_ context.Context, obj client.Object, field string, extractValue client.IndexerFunc) error
	WaitForCacheSyncFn func(_ context.Context) bool
}

func (m *mockCache) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return m.ListFn(ctx, list, opts...)
}

func (m *mockCache) IndexField(ctx context.Context, obj client.Object, field string, extractValue client.IndexerFunc) error {
	return m.IndexFieldFn(ctx, obj, field, extractValue)
}

func (m *mockCache) WaitForCacheSync(ctx context.Context) bool {
	return m.WaitForCacheSyncFn(ctx)
}
