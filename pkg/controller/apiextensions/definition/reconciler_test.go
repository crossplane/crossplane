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
	"github.com/pkg/errors"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	kcontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	runtimev1alpha1 "github.com/crossplane/crossplane-runtime/apis/core/v1alpha1"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/crossplane/crossplane/apis/apiextensions/v1alpha1"
)

type MockEngine struct {
	ControllerEngine
	MockStart func(name string, o kcontroller.Options, w ...controller.Watch) error
}

func (m *MockEngine) Start(name string, o kcontroller.Options, w ...controller.Watch) error {
	return m.MockStart(name, o, w...)
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
		"InfrastructureDefinitionNotFound": {
			reason: "We should not return an error if the InfrastructureDefinition was not found.",
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
		"GetInfrastructureDefinitionError": {
			reason: "We should return any other error encountered while getting an InfrastructureDefinition.",
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
				err: errors.Wrap(errBoom, errGetInfraDef),
			},
		},
		"RenderCustomResourceDefinitionError": {
			reason: "We should record any error encountered while rendering a CRD as a status condition.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1alpha1.InfrastructureDefinition{}
								want.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(errBoom, errRenderCRD)))

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
						},
					}),
					WithCRDRenderer(CRDRenderFn(func(_ *v1alpha1.InfrastructureDefinition) (*v1beta1.CustomResourceDefinition, error) {
						return nil, errBoom
					})),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"GetCustomResourceDefinitionError": {
			reason: "We should record any error encountered while getting a CRD as a status condition.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								switch v := o.(type) {
								case *v1alpha1.InfrastructureDefinition:
									d := v1alpha1.InfrastructureDefinition{}
									d.SetDeletionTimestamp(&now)
									*v = d
								case *v1beta1.CustomResourceDefinition:
									return errBoom
								}
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1alpha1.InfrastructureDefinition{}
								want.SetDeletionTimestamp(&now)
								want.Status.SetConditions(v1alpha1.Deleting())
								want.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(errBoom, errGetCRD)))

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
						},
					}),
					WithCRDRenderer(CRDRenderFn(func(_ *v1alpha1.InfrastructureDefinition) (*v1beta1.CustomResourceDefinition, error) {
						return &v1beta1.CustomResourceDefinition{}, nil
					})),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"RemoveFinalizerError": {
			reason: "We should record any error encountered while removing a finalizer as a status condition.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								if v, ok := o.(*v1alpha1.InfrastructureDefinition); ok {
									d := v1alpha1.InfrastructureDefinition{}
									d.SetDeletionTimestamp(&now)
									*v = d
								}
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1alpha1.InfrastructureDefinition{}
								want.SetDeletionTimestamp(&now)
								want.Status.SetConditions(v1alpha1.Deleting())
								want.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(errBoom, errRemoveFinalizer)))

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
						},
					}),
					WithCRDRenderer(CRDRenderFn(func(_ *v1alpha1.InfrastructureDefinition) (*v1beta1.CustomResourceDefinition, error) {
						return &v1beta1.CustomResourceDefinition{}, nil
					})),
					WithFinalizer(resource.FinalizerFns{RemoveFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return errBoom
					}}),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"SuccessfulDelete": {
			reason: "We should not requeue when deleted if we successfully cleaned up our CRD and removed our finalizer.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								if v, ok := o.(*v1alpha1.InfrastructureDefinition); ok {
									d := v1alpha1.InfrastructureDefinition{}
									d.SetDeletionTimestamp(&now)
									*v = d
								}
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil),
						},
					}),
					WithCRDRenderer(CRDRenderFn(func(_ *v1alpha1.InfrastructureDefinition) (*v1beta1.CustomResourceDefinition, error) {
						return &v1beta1.CustomResourceDefinition{}, nil
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
			reason: "We should record any error encountered while deleting all defined resources as a status condition.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								switch v := o.(type) {
								case *v1alpha1.InfrastructureDefinition:
									d := v1alpha1.InfrastructureDefinition{}
									d.SetUID(owner)
									d.SetDeletionTimestamp(&now)
									*v = d
								case *v1beta1.CustomResourceDefinition:
									crd := v1beta1.CustomResourceDefinition{}
									crd.SetCreationTimestamp(now)
									crd.SetOwnerReferences([]metav1.OwnerReference{{UID: owner, Controller: &ctrlr}})
									*v = crd
								}
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1alpha1.InfrastructureDefinition{}
								want.SetUID(owner)
								want.SetDeletionTimestamp(&now)
								want.Status.SetConditions(v1alpha1.Deleting())
								want.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(errBoom, errDeleteCRs)))

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
							MockDeleteAllOf: test.NewMockDeleteAllOfFn(errBoom),
						},
					}),
					WithCRDRenderer(CRDRenderFn(func(_ *v1alpha1.InfrastructureDefinition) (*v1beta1.CustomResourceDefinition, error) {
						return &v1beta1.CustomResourceDefinition{}, nil
					})),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"ListCustomResourcesError": {
			reason: "We should record any error encountered while listing all defined resources as a status condition.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								switch v := o.(type) {
								case *v1alpha1.InfrastructureDefinition:
									d := v1alpha1.InfrastructureDefinition{}
									d.SetUID(owner)
									d.SetDeletionTimestamp(&now)
									*v = d
								case *v1beta1.CustomResourceDefinition:
									crd := v1beta1.CustomResourceDefinition{}
									crd.SetCreationTimestamp(now)
									crd.SetOwnerReferences([]metav1.OwnerReference{{UID: owner, Controller: &ctrlr}})
									*v = crd
								}
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1alpha1.InfrastructureDefinition{}
								want.SetUID(owner)
								want.SetDeletionTimestamp(&now)
								want.Status.SetConditions(v1alpha1.Deleting())
								want.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(errBoom, errListCRs)))

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
							MockDeleteAllOf: test.NewMockDeleteAllOfFn(nil),
							MockList:        test.NewMockListFn(errBoom),
						},
					}),
					WithCRDRenderer(CRDRenderFn(func(_ *v1alpha1.InfrastructureDefinition) (*v1beta1.CustomResourceDefinition, error) {
						return &v1beta1.CustomResourceDefinition{}, nil
					})),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"WaitForDeleteAllOf": {
			reason: "We should record the pending deletion of defined resources as a status condition.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								switch v := o.(type) {
								case *v1alpha1.InfrastructureDefinition:
									d := v1alpha1.InfrastructureDefinition{}
									d.SetUID(owner)
									d.SetDeletionTimestamp(&now)
									*v = d
								case *v1beta1.CustomResourceDefinition:
									crd := v1beta1.CustomResourceDefinition{}
									crd.SetCreationTimestamp(now)
									crd.SetOwnerReferences([]metav1.OwnerReference{{UID: owner, Controller: &ctrlr}})
									*v = crd
								}
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1alpha1.InfrastructureDefinition{}
								want.SetUID(owner)
								want.SetDeletionTimestamp(&now)
								want.Status.SetConditions(v1alpha1.Deleting())
								want.Status.SetConditions(runtimev1alpha1.ReconcileSuccess().WithMessage(waitCRDelete))

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
							MockDeleteAllOf: test.NewMockDeleteAllOfFn(nil),
							MockList: test.NewMockListFn(nil, func(o runtime.Object) error {
								v := o.(*unstructured.UnstructuredList)
								*v = unstructured.UnstructuredList{
									Items: []unstructured.Unstructured{{}, {}},
								}
								return nil
							}),
						},
					}),
					WithCRDRenderer(CRDRenderFn(func(_ *v1alpha1.InfrastructureDefinition) (*v1beta1.CustomResourceDefinition, error) {
						return &v1beta1.CustomResourceDefinition{}, nil
					})),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: tinyWait},
			},
		},
		"DeleteCustomResourceDefinitionError": {
			reason: "We should record any error encountered while deleting the CRD we created as a status condition.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								switch v := o.(type) {
								case *v1alpha1.InfrastructureDefinition:
									d := v1alpha1.InfrastructureDefinition{}
									d.SetUID(owner)
									d.SetDeletionTimestamp(&now)
									*v = d
								case *v1beta1.CustomResourceDefinition:
									crd := v1beta1.CustomResourceDefinition{}
									crd.SetCreationTimestamp(now)
									crd.SetOwnerReferences([]metav1.OwnerReference{{UID: owner, Controller: &ctrlr}})
									*v = crd
								}
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1alpha1.InfrastructureDefinition{}
								want.SetUID(owner)
								want.SetDeletionTimestamp(&now)
								want.Status.SetConditions(v1alpha1.Deleting())
								want.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(errBoom, errDeleteCRD)))

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
							MockDeleteAllOf: test.NewMockDeleteAllOfFn(nil),
							MockList:        test.NewMockListFn(nil),
							MockDelete:      test.NewMockDeleteFn(errBoom),
						},
					}),
					WithCRDRenderer(CRDRenderFn(func(_ *v1alpha1.InfrastructureDefinition) (*v1beta1.CustomResourceDefinition, error) {
						return &v1beta1.CustomResourceDefinition{}, nil
					})),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"SuccessfulCleanup": {
			reason: "We should requeue after a tiny wait to remove our finalizer once we've cleaned up our defined resources and CRD.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil, func(o runtime.Object) error {
								switch v := o.(type) {
								case *v1alpha1.InfrastructureDefinition:
									d := v1alpha1.InfrastructureDefinition{}
									d.SetUID(owner)
									d.SetDeletionTimestamp(&now)
									*v = d
								case *v1beta1.CustomResourceDefinition:
									crd := v1beta1.CustomResourceDefinition{}
									crd.SetCreationTimestamp(now)
									crd.SetOwnerReferences([]metav1.OwnerReference{{UID: owner, Controller: &ctrlr}})
									*v = crd
								}
								return nil
							}),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1alpha1.InfrastructureDefinition{}
								want.SetUID(owner)
								want.SetDeletionTimestamp(&now)
								want.Status.SetConditions(v1alpha1.Deleting())
								want.Status.SetConditions(runtimev1alpha1.ReconcileSuccess())

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
							MockDeleteAllOf: test.NewMockDeleteAllOfFn(nil),
							MockList:        test.NewMockListFn(nil),
							MockDelete:      test.NewMockDeleteFn(nil),
						},
					}),
					WithCRDRenderer(CRDRenderFn(func(_ *v1alpha1.InfrastructureDefinition) (*v1beta1.CustomResourceDefinition, error) {
						return &v1beta1.CustomResourceDefinition{}, nil
					})),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: tinyWait},
			},
		},
		"AddFinalizerError": {
			reason: "We should record any error encountered while adding a finalizer as a status condition.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1alpha1.InfrastructureDefinition{}
								want.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(errBoom, errAddFinalizer)))

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
						},
					}),
					WithCRDRenderer(CRDRenderFn(func(_ *v1alpha1.InfrastructureDefinition) (*v1beta1.CustomResourceDefinition, error) {
						return &v1beta1.CustomResourceDefinition{}, nil
					})),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return errBoom
					}}),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"ApplyCustomResourceDefinitionError": {
			reason: "We should record any error encountered while applying our CRD as a status condition.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1alpha1.InfrastructureDefinition{}
								want.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(errBoom, errApplyCRD)))

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
						},
						Applicator: resource.ApplyFn(func(_ context.Context, _ runtime.Object, _ ...resource.ApplyOption) error {
							return errBoom
						}),
					}),
					WithCRDRenderer(CRDRenderFn(func(_ *v1alpha1.InfrastructureDefinition) (*v1beta1.CustomResourceDefinition, error) {
						return &v1beta1.CustomResourceDefinition{}, nil
					})),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return nil
					}}),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"CustomResourceDefinitionIsNotEstablished": {
			reason: "We should requeue after a tiny wait if we're waiting for a newly created CRD to become established.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1alpha1.InfrastructureDefinition{}
								want.Status.SetConditions(runtimev1alpha1.ReconcileSuccess().WithMessage(waitCRDEstablish))

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
						},
						Applicator: resource.ApplyFn(func(_ context.Context, _ runtime.Object, _ ...resource.ApplyOption) error {
							return nil
						}),
					}),
					WithCRDRenderer(CRDRenderFn(func(_ *v1alpha1.InfrastructureDefinition) (*v1beta1.CustomResourceDefinition, error) {
						return &v1beta1.CustomResourceDefinition{}, nil
					})),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return nil
					}}),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: tinyWait},
			},
		},
		"StartControllerError": {
			reason: "We should record any error encountered while starting our controller as a status condition.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1alpha1.InfrastructureDefinition{}
								want.Status.SetConditions(runtimev1alpha1.ReconcileError(errors.Wrap(errBoom, errStartController)))

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
						},
						Applicator: resource.ApplyFn(func(_ context.Context, _ runtime.Object, _ ...resource.ApplyOption) error {
							return nil
						}),
					}),
					WithCRDRenderer(CRDRenderFn(func(_ *v1alpha1.InfrastructureDefinition) (*v1beta1.CustomResourceDefinition, error) {
						return &v1beta1.CustomResourceDefinition{
							Status: v1beta1.CustomResourceDefinitionStatus{
								Conditions: []v1beta1.CustomResourceDefinitionCondition{
									{Type: v1beta1.Established, Status: v1beta1.ConditionTrue},
								},
							},
						}, nil
					})),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return nil
					}}),
					WithControllerEngine(&MockEngine{MockStart: func(_ string, _ kcontroller.Options, _ ...controller.Watch) error {
						return errBoom
					}}),
				},
			},
			want: want{
				r: reconcile.Result{RequeueAfter: shortWait},
			},
		},
		"SuccessfulStart": {
			reason: "We should not requeue if we successfully ensured our CRD exists and controller is started.",
			args: args{
				mgr: &fake.Manager{},
				opts: []ReconcilerOption{
					WithClientApplicator(resource.ClientApplicator{
						Client: &test.MockClient{
							MockGet: test.NewMockGetFn(nil),
							MockStatusUpdate: test.NewMockStatusUpdateFn(nil, func(o runtime.Object) error {
								want := &v1alpha1.InfrastructureDefinition{}
								want.Status.SetConditions(v1alpha1.Started())
								want.Status.SetConditions(runtimev1alpha1.ReconcileSuccess())

								if diff := cmp.Diff(want, o); diff != "" {
									t.Errorf("-want, +got:\n%s", diff)
								}
								return nil
							}),
						},
						Applicator: resource.ApplyFn(func(_ context.Context, _ runtime.Object, _ ...resource.ApplyOption) error {
							return nil
						}),
					}),
					WithCRDRenderer(CRDRenderFn(func(_ *v1alpha1.InfrastructureDefinition) (*v1beta1.CustomResourceDefinition, error) {
						return &v1beta1.CustomResourceDefinition{
							Status: v1beta1.CustomResourceDefinitionStatus{
								Conditions: []v1beta1.CustomResourceDefinitionCondition{
									{Type: v1beta1.Established, Status: v1beta1.ConditionTrue},
								},
							},
						}, nil
					})),
					WithFinalizer(resource.FinalizerFns{AddFinalizerFn: func(_ context.Context, _ resource.Object) error {
						return nil
					}}),
					WithControllerEngine(&MockEngine{MockStart: func(_ string, _ kcontroller.Options, _ ...controller.Watch) error {
						return nil
					}}),
				},
			},
			want: want{
				r: reconcile.Result{Requeue: false},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewReconciler(tc.args.mgr, tc.args.opts...)
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
